package servidor

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tramtai/internal/actualizacion"
	"tramtai/internal/version"
)

// versionPublicada publica en un servidor HTTP de prueba la v0.99.0+1 con el paquete del servidor de este sistema (el
// .tar.gz de Linux, con la carpeta tramtai, o el .zip portable de Windows), con archivos dentro, sus sumas y la firma de
// las sumas con un par de la prueba, que pasa a ser la clave de confianza; con firmaMala, firmadas con otro par. Este
// programa pasa a ser la v0.1.0+1.
func versionPublicada(t *testing.T, archivos map[string]string, firmaMala bool) {
	t.Helper()
	const etiqueta = "v0.99.0+1"
	nombre, err := actualizacion.Asset(etiqueta, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	var paquete bytes.Buffer
	if strings.HasSuffix(nombre, ".zip") {
		z := zip.NewWriter(&paquete)
		for n, contenido := range archivos {
			w, _ := z.Create(n)
			w.Write([]byte(contenido))
		}
		z.Close()
	} else {
		gz := gzip.NewWriter(&paquete)
		tw := tar.NewWriter(gz)
		tw.WriteHeader(&tar.Header{Name: "tramtai/", Mode: 0o755, Typeflag: tar.TypeDir})
		for n, contenido := range archivos {
			tw.WriteHeader(&tar.Header{Name: "tramtai/" + n, Mode: 0o755, Size: int64(len(contenido)), Typeflag: tar.TypeReg})
			tw.Write([]byte(contenido))
		}
		tw.Close()
		gz.Close()
	}
	suma := sha256.Sum256(paquete.Bytes())
	sumas := fmt.Sprintf("%x  %s\n", suma, nombre)
	publica, privada, _ := ed25519.GenerateKey(nil)
	if firmaMala {
		_, privada, _ = ed25519.GenerateKey(nil)
	}
	publicados := map[string][]byte{nombre: paquete.Bytes(), "SHA256SUMS.txt": []byte(sumas),
		"SHA256SUMS.txt.sig": []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(privada, []byte(sumas))))}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/ultima", func(w http.ResponseWriter, _ *http.Request) {
		var lista []string
		for n := range publicados {
			lista = append(lista, fmt.Sprintf(`{"name":%q,"browser_download_url":"%s/descargas/%s"}`, n, srv.URL, n))
		}
		fmt.Fprintf(w, `{"tag_name":%q,"assets":[%s]}`, etiqueta, strings.Join(lista, ","))
	})
	mux.HandleFunc("/descargas/", func(w http.ResponseWriter, r *http.Request) {
		datos, ok := publicados[strings.TrimPrefix(r.URL.Path, "/descargas/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(datos)
	})
	actualizacion.ParaPruebas(t, srv.URL+"/ultima", srv.URL+"/descargas/", publica)
	anterior := version.Version
	version.Version = "v0.1.0+1"
	t.Cleanup(func() { version.Version = anterior })
}

// programaNuevo es el contenido del paquete publicado: el programa de este sistema en la versión nueva, con algo más
// que no se instala (la app de escritorio del .zip portable).
func programaNuevo() map[string]string {
	archivos := map[string]string{"LICENCIA.md": "licencia"}
	for _, n := range programas {
		archivos[n] = n + " nuevo"
	}
	return archivos
}

// programaInstalado deja el programa de este sistema, en la v1, en una carpeta de prueba que pasa a ser la del
// programa, y sustituye detener y arrancar el servicio por unos que anotan en ordenes.
func programaInstalado(t *testing.T, ordenes *[]string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range programas {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(n+" v1"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	rutas0, detener0, arrancar0 := rutas, detenerServicio, arrancarServicio
	t.Cleanup(func() { rutas, detenerServicio, arrancarServicio = rutas0, detener0, arrancar0 })
	rutas = func() Sitios { return Sitios{Programa: dir} }
	detenerServicio = func() error { *ordenes = append(*ordenes, "detener"); return nil }
	arrancarServicio = func() error { *ordenes = append(*ordenes, "arrancar"); return nil }
	return dir
}

// intacto comprueba que la carpeta del programa tiene solo el programa de la v1: nada nuevo, apartado ni a medias.
func intacto(t *testing.T, dir string) {
	t.Helper()
	entradas, _ := os.ReadDir(dir)
	if len(entradas) != len(programas) {
		t.Errorf("la carpeta del programa cambió: %v", entradas)
	}
	for _, n := range programas {
		if b, _ := os.ReadFile(filepath.Join(dir, n)); string(b) != n+" v1" {
			t.Errorf("%s: %q, se esperaba la v1", n, b)
		}
	}
}

// TestActualizarPaquete: lo que instaló el paquete .deb o .rpm se actualiza con apt o dnf; Actualizar no descarga ni
// toca nada.
func TestActualizarPaquete(t *testing.T) {
	var ordenes []string
	dir := programaInstalado(t, &ordenes)
	versionPublicada(t, programaNuevo(), false)
	if err := os.WriteFile(filepath.Join(dir, ".paquete"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Actualizar(t.Context(), actualizacion.PaqueteServidor); !errors.Is(err, ErrInstaladoPorPaquete) || !strings.Contains(err.Error(), "apt") {
		t.Fatalf("instalado por el paquete: %v", err)
	}
	os.Remove(filepath.Join(dir, ".paquete"))
	intacto(t, dir)
	if len(ordenes) > 0 {
		t.Errorf("no se toca el servicio: %q", ordenes)
	}
}

// TestActualizarFirmaMala: con las sumas firmadas por otra clave no se instala nada y el servicio sigue como estaba.
func TestActualizarFirmaMala(t *testing.T) {
	var ordenes []string
	dir := programaInstalado(t, &ordenes)
	versionPublicada(t, programaNuevo(), true)
	if err := Actualizar(t.Context(), actualizacion.PaqueteServidor); err == nil || !strings.Contains(err.Error(), "firma") {
		t.Fatalf("se aceptó una firma de otra clave: %v", err)
	}
	intacto(t, dir)
	if len(ordenes) > 0 {
		t.Errorf("no se toca el servicio: %q", ordenes)
	}
}

// TestActualizarAlDia: si la última versión publicada no es más nueva que este programa, no se descarga ni se toca nada.
func TestActualizarAlDia(t *testing.T) {
	var ordenes []string
	dir := programaInstalado(t, &ordenes)
	versionPublicada(t, programaNuevo(), false)
	version.Version = "v0.99.0+1"
	if err := Actualizar(t.Context(), actualizacion.PaqueteServidor); !errors.Is(err, actualizacion.ErrAlDia) {
		t.Fatalf("con la misma versión: %v", err)
	}
	intacto(t, dir)
	if len(ordenes) > 0 {
		t.Errorf("no se toca el servicio: %q", ordenes)
	}
}

// TestActualizarPaqueteIncompleto: un paquete firmado sin uv no se instala a medias.
func TestActualizarPaqueteIncompleto(t *testing.T) {
	var ordenes []string
	dir := programaInstalado(t, &ordenes)
	archivos := programaNuevo()
	delete(archivos, programas[1])
	versionPublicada(t, archivos, false)
	if err := Actualizar(t.Context(), actualizacion.PaqueteServidor); err == nil || !strings.Contains(err.Error(), programas[1]) {
		t.Fatalf("sin %s: %v", programas[1], err)
	}
	intacto(t, dir)
	if len(ordenes) > 0 {
		t.Errorf("no se toca el servicio: %q", ordenes)
	}
}

// TestActualizarVuelveAtras: si cambiar el programa falla con el servicio ya detenido, lo que se apartó vuelve a su
// sitio y el servicio arranca con el programa de antes.
func TestActualizarVuelveAtras(t *testing.T) {
	var ordenes []string
	dir := programaInstalado(t, &ordenes)
	versionPublicada(t, programaNuevo(), false)
	// Una carpeta con algo dentro donde iría uv apartado: no se puede renombrar uv encima.
	estorbo := filepath.Join(dir, programas[1]+".anterior")
	if err := os.MkdirAll(filepath.Join(estorbo, "algo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Actualizar(t.Context(), actualizacion.PaqueteServidor); err == nil {
		t.Fatal("se dio por actualizado sin poder apartar uv")
	}
	os.RemoveAll(estorbo)
	intacto(t, dir)
	if strings.Join(ordenes, " ") != "detener arrancar" {
		t.Errorf("el servicio vuelve a arrancar con el programa de antes: %q", ordenes)
	}
}
