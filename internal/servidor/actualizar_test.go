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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

// TestActualizarNuevoNoArranca: si el programa nuevo ni siquiera se puede ejecutar (aquí no es un ejecutable), el de
// antes vuelve a su sitio y el servicio arranca con él.
func TestActualizarNuevoNoArranca(t *testing.T) {
	var ordenes []string
	dir := programaInstalado(t, &ordenes)
	versionPublicada(t, programaNuevo(), false)
	if err := Actualizar(t.Context(), actualizacion.PaqueteServidor); err == nil {
		t.Fatal("se dio por actualizado con un programa que no se ejecuta")
	}
	intacto(t, dir)
	if strings.Join(ordenes, " ") != "detener arrancar" {
		t.Errorf("el servicio vuelve a arrancar con el programa de antes: %q", ordenes)
	}
}

// TestActualizarReinicioFalla: si el programa nuevo se ejecuta pero «servidor reiniciar» falla, el nuevo se queda en su
// sitio y el de antes en .anterior (no se vuelve atrás ni se arranca el viejo), y el error trae lo que dijo y la ruta
// del .anterior para volver a mano.
func TestActualizarReinicioFalla(t *testing.T) {
	var ordenes []string
	dir := programaInstalado(t, &ordenes)
	versionPublicada(t, programaNuevo(), false)
	salida := exec.Command(os.Args[0], "-test.run=^$", "-flag-que-no-existe").Run() // un *exec.ExitError de verdad
	if !errors.As(salida, new(*exec.ExitError)) {
		t.Fatalf("se esperaba un ExitError: %v", salida)
	}
	ejecutar0 := ejecutar
	t.Cleanup(func() { ejecutar = ejecutar0 })
	ejecutar = func(string, ...string) ([]byte, error) { return []byte("la unidad no arrancó\n"), salida }
	err := Actualizar(t.Context(), actualizacion.PaqueteServidor)
	anterior := filepath.Join(dir, programas[0]) + ".anterior"
	if err == nil || !strings.Contains(err.Error(), "la unidad no arrancó") || !strings.Contains(err.Error(), anterior) {
		t.Fatalf("el error trae la salida y la ruta del .anterior: %v", err)
	}
	for _, n := range programas {
		if b, _ := os.ReadFile(filepath.Join(dir, n)); string(b) != n+" nuevo" {
			t.Errorf("%s: %q, el nuevo sigue en su sitio", n, b)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, n+".anterior")); string(b) != n+" v1" {
			t.Errorf("%s.anterior: %q, el de antes queda apartado", n, b)
		}
	}
	if strings.Join(ordenes, " ") != "detener" {
		t.Errorf("no se arranca el programa de antes: %q", ordenes)
	}
}

// TestExtraerZipConBarras: Compress-Archive puede escribir los nombres con \ (.\tramtai.exe): valen igual.
func TestExtraerZipConBarras(t *testing.T) {
	dir := t.TempDir()
	paquete := filepath.Join(dir, "Tramtai-v0.99.0+1-portable-windows-x64.zip")
	f, err := os.Create(paquete)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, n := range programas {
		w, _ := z.Create(`.\` + n)
		w.Write([]byte(n + " nuevo"))
	}
	z.Close()
	f.Close()
	if err := extraerPrograma(paquete, dir); err != nil {
		t.Fatal(err)
	}
	for _, n := range programas {
		if b, _ := os.ReadFile(filepath.Join(dir, n+".nuevo")); string(b) != n+" nuevo" {
			t.Errorf("%s.nuevo: %q", n, b)
		}
	}
}

// TestExtraerTarSinEnlaces: un enlace con el nombre del programa no se toma por el programa.
func TestExtraerTarSinEnlaces(t *testing.T) {
	dir := t.TempDir()
	paquete := filepath.Join(dir, "tramtai-v0.99.0+1-linux-amd64.tar.gz")
	f, err := os.Create(paquete)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "tramtai/" + programas[0], Linkname: "../../otro", Mode: 0o777, Typeflag: tar.TypeSymlink})
	tw.WriteHeader(&tar.Header{Name: "tramtai/" + programas[1], Mode: 0o755, Size: 2, Typeflag: tar.TypeReg})
	tw.Write([]byte("uv"))
	tw.Close()
	gz.Close()
	f.Close()
	if err := extraerPrograma(paquete, dir); err == nil || !strings.Contains(err.Error(), "no trae "+programas[0]) {
		t.Fatalf("un enlace no es el programa: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, programas[0]+".nuevo")); err == nil {
		t.Error("quedó algo con el nombre del programa")
	}
}

// TestActualizarConElInstalado: desde otra copia (la del escritorio, por ejemplo), la actualización la hace el programa
// instalado, que compara con su propia versión; desde el instalado, no se delega; sin servidor, el error de siempre.
func TestActualizarConElInstalado(t *testing.T) {
	var ordenes []string
	dir := programaInstalado(t, &ordenes)
	instalado := filepath.Join(dir, programas[0])
	otra := filepath.Join(t.TempDir(), programas[0])
	if err := os.WriteFile(otra, []byte("otra copia"), 0o755); err != nil {
		t.Fatal(err)
	}
	actual0, lanzar0 := ejecutableActual, lanzarInstalado
	t.Cleanup(func() { ejecutableActual, lanzarInstalado = actual0, lanzar0 })
	var lanzados []string
	lanzarInstalado = func(programa string, _, _ io.Writer, args ...string) error {
		lanzados = append(lanzados, strings.Join(append([]string{programa}, args...), " "))
		return nil
	}
	ejecutableActual = func() (string, error) { return instalado, nil }
	if delegado, err := ActualizarConElInstalado(io.Discard, io.Discard); delegado || err != nil || len(lanzados) > 0 {
		t.Fatalf("desde el instalado no se delega: %v %v %q", delegado, err, lanzados)
	}
	ejecutableActual = func() (string, error) { return otra, nil }
	if delegado, err := ActualizarConElInstalado(io.Discard, io.Discard); !delegado || err != nil ||
		strings.Join(lanzados, "|") != instalado+" --servidor servidor actualizar" {
		t.Fatalf("desde otra copia se delega en el instalado: %v %v %q", delegado, err, lanzados)
	}
	// Si no se puede mirar el ejecutable actual, no se delega (falla cerrado).
	lanzados = nil
	ejecutableActual = func() (string, error) { return filepath.Join(t.TempDir(), "no-existe"), nil }
	if delegado, err := ActualizarConElInstalado(io.Discard, io.Discard); !delegado || err == nil || len(lanzados) > 0 {
		t.Fatalf("sin identificar el ejecutable actual: %v %v %q", delegado, err, lanzados)
	}
	os.Remove(instalado)
	if delegado, err := ActualizarConElInstalado(io.Discard, io.Discard); !delegado || err == nil || !strings.Contains(err.Error(), "no está instalado") {
		t.Fatalf("sin servidor instalado: %v %v", delegado, err)
	}
}
