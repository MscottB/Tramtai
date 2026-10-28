package actualizacion

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMasNueva(t *testing.T) {
	casos := []struct {
		publicada, actual string
		esperado          bool
	}{
		{"v0.26.0+16102026", "v0.25.1+15102026", true},
		{"v0.25.1+16102026", "v0.25.1+15102026", false}, // la misma versión con otra fecha no es nueva
		{"v0.25.0+16102026", "v0.25.1+15102026", false},
		{"v1.0.0+01012027", "v0.99.9+31122026", true},
		{"v0.10.0+1", "v0.9.0+1", true}, // compara números, no texto
		{"v0.26.0+1", "dev", false},     // una compilación sin versión no se da por desactualizada
		{"", "v0.1.0+1", false},
	}
	for _, c := range casos {
		if r := MasNueva(c.publicada, c.actual); r != c.esperado {
			t.Errorf("MasNueva(%q, %q) = %v", c.publicada, c.actual, r)
		}
	}
}

func TestComprobadorNoRepiteLaConsulta(t *testing.T) {
	var llamadas atomic.Int32
	c := &Comprobador{Actual: "v0.25.0+1", Consultar: func(context.Context) (Version, error) {
		llamadas.Add(1)
		return Version{Etiqueta: "v0.26.0+2"}, nil
	}}
	v, nueva := c.Disponible(context.Background(), true)
	if !nueva || v.Etiqueta != "v0.26.0+2" {
		t.Fatalf("primera consulta: %+v %v", v, nueva)
	}
	c.Disponible(context.Background(), true)
	c.Disponible(context.Background(), false)
	if n := llamadas.Load(); n != 1 {
		t.Fatalf("se consultó %d veces en la misma hora", n)
	}
}

// Firmas de la versión falsa: la buena, la de otro par, ninguna, unas sumas cambiadas después de firmar o el trío
// (instalador, sumas y firma, todo válido) de una versión vieja subido a la última.
const (
	firmaValida = iota
	firmaDeOtro
	sinFirma
	sumasCambiadas
	versionVieja
)

// servidor publica una versión falsa con su instalador, sus sumas y su firma (con un par de la prueba, que pasa a ser
// la clave pública de confianza); alterado sirve otro contenido que el sumado.
func servidor(t *testing.T, alterado bool, firma int) {
	t.Helper()
	contenido := []byte("instalador de prueba")
	suma := sha256.Sum256(contenido)
	nombre := "Tramtai-v0.26.0+2-instalador-windows-x64.exe"
	if firma == versionVieja {
		nombre = "Tramtai-v0.20.0+1-instalador-windows-x64.exe"
	}
	sumas := fmt.Sprintf("%s  %s\n", hex.EncodeToString(suma[:]), nombre)
	publica, privada, _ := ed25519.GenerateKey(nil)
	_, otra, _ := ed25519.GenerateKey(nil)
	clave := privada
	if firma == firmaDeOtro {
		clave = otra
	}
	firmada := base64.StdEncoding.EncodeToString(ed25519.Sign(clave, []byte(sumas)))
	if firma == sumasCambiadas {
		sumas += "0000  otro.exe\n"
	}
	anterior := clavesPublicas
	clavesPublicas = []ed25519.PublicKey{publica}
	t.Cleanup(func() { clavesPublicas = anterior })
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/ultima", func(w http.ResponseWriter, _ *http.Request) {
		archivoDeFirma := ""
		if firma != sinFirma {
			archivoDeFirma = fmt.Sprintf(`,{"name":"SHA256SUMS.txt.sig","browser_download_url":"%s/descargas/SHA256SUMS.txt.sig"}`, srv.URL)
		}
		fmt.Fprintf(w, `{"tag_name":"v0.26.0+2","html_url":"%[1]s/version","body":"Tramtai v0.26.0\n\n- novedad","assets":[`+
			`{"name":"%[2]s","browser_download_url":"%[1]s/descargas/%[2]s"},`+
			`{"name":"SHA256SUMS.txt","browser_download_url":"%[1]s/descargas/SHA256SUMS.txt"}%[3]s]}`, srv.URL, nombre, archivoDeFirma)
	})
	mux.HandleFunc("/descargas/SHA256SUMS.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, sumas)
	})
	mux.HandleFunc("/descargas/SHA256SUMS.txt.sig", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, firmada)
	})
	mux.HandleFunc("/descargas/"+nombre, func(w http.ResponseWriter, _ *http.Request) {
		if alterado {
			w.Write([]byte("otro contenido"))
			return
		}
		w.Write(contenido)
	})
	antes := [2]string{urlUltima, prefijoDescarga}
	urlUltima, prefijoDescarga = srv.URL+"/ultima", srv.URL+"/descargas/"
	t.Cleanup(func() { urlUltima, prefijoDescarga = antes[0], antes[1] })
}

func TestDescargar(t *testing.T) {
	servidor(t, false, firmaValida)
	v, err := Ultima(context.Background())
	if err != nil || !v.TieneInstalador() || v.Notas != "Tramtai v0.26.0\n\n- novedad" {
		t.Fatalf("Ultima: %+v %v", v, err)
	}
	var recibidos, total int64
	ruta, err := Descargar(context.Background(), v, t.TempDir(), func(r, todo int64) { recibidos, total = r, todo })
	if err != nil {
		t.Fatal(err)
	}
	if datos, _ := os.ReadFile(ruta); string(datos) != "instalador de prueba" {
		t.Fatalf("contenido descargado: %q", datos)
	}
	if n := int64(len("instalador de prueba")); recibidos != n || total != n {
		t.Fatalf("avance al terminar: %d de %d, se esperaba %d de %d", recibidos, total, n, n)
	}

	// Una dirección fuera del repositorio de Tramtai no se descarga.
	ajena := v
	for _, u := range []string{"https://ejemplo.com/instalador.exe", prefijoDescarga + "../../otro/instalador.exe", prefijoDescarga + "%2e%2e/otro.exe"} {
		ajena.urlInstalador = u
		if _, err := Descargar(context.Background(), ajena, t.TempDir(), nil); err == nil {
			t.Fatalf("se aceptó una dirección de descarga ajena: %s", u)
		}
	}
	if v.Pagina != "" {
		t.Fatalf("una página de novedades fuera del repositorio no se publica: %q", v.Pagina)
	}
}

func TestDescargarRechazaSumaDistinta(t *testing.T) {
	servidor(t, true, firmaValida)
	v, err := Ultima(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := Descargar(context.Background(), v, dir, nil); err == nil {
		t.Fatal("se aceptó un instalador cuya suma no coincide")
	}
	if archivos, _ := os.ReadDir(dir); len(archivos) != 0 {
		t.Fatalf("quedó el archivo dañado: %v", archivos)
	}
}

// Sin una firma válida de las sumas no se descarga el instalador: ni firmadas con otra clave, ni cambiadas después de
// firmar, ni sin firma (entonces la versión no es instalable).
func TestDescargarExigeLaFirma(t *testing.T) {
	for nombre, firma := range map[string]int{"otra clave": firmaDeOtro, "sumas cambiadas": sumasCambiadas} {
		t.Run(nombre, func(t *testing.T) {
			servidor(t, false, firma)
			v, err := Ultima(context.Background())
			if err != nil || !v.TieneInstalador() {
				t.Fatalf("Ultima: %+v %v", v, err)
			}
			dir := t.TempDir()
			if _, err := Descargar(context.Background(), v, dir, nil); err == nil || !strings.Contains(err.Error(), "firma") {
				t.Fatalf("se aceptó una firma inválida: %v", err)
			}
			if archivos, _ := os.ReadDir(dir); len(archivos) != 0 {
				t.Fatalf("se descargó el instalador sin firma válida: %v", archivos)
			}
		})
	}
	servidor(t, false, sinFirma)
	v, err := Ultima(context.Background())
	if err != nil || v.TieneInstalador() {
		t.Fatalf("una versión sin firma no es instalable: %+v %v", v, err)
	}
	if _, err := Descargar(context.Background(), v, t.TempDir(), nil); err == nil {
		t.Fatal("se descargó una versión sin firma")
	}
}

// La firma vale para las sumas de una versión, que nombran su instalador con la etiqueta: el trío de una versión vieja
// subido a la última no se instala.
func TestVersionViejaNoSeInstala(t *testing.T) {
	servidor(t, false, versionVieja)
	v, err := Ultima(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.TieneInstalador() {
		t.Fatalf("el instalador de otra versión se dio por bueno: %+v", v)
	}
	if _, err := Descargar(context.Background(), v, t.TempDir(), nil); err == nil {
		t.Fatal("se descargó el instalador de una versión vieja")
	}
}

// TestAssetPorSistema: el servidor se actualiza con el .tar.gz de Linux de su arquitectura o con el .zip portable de
// Windows x64 (los nombres de Publicar); para otro sistema no hay paquete.
func TestAssetPorSistema(t *testing.T) {
	for _, c := range []struct{ goos, goarch, nombre string }{
		{"linux", "amd64", "tramtai-v0.58.0+02112026-linux-amd64.tar.gz"},
		{"linux", "arm64", "tramtai-v0.58.0+02112026-linux-arm64.tar.gz"},
		{"windows", "amd64", "Tramtai-v0.58.0+02112026-portable-windows-x64.zip"},
		{"windows", "arm64", ""},
		{"darwin", "arm64", ""},
	} {
		nombre, err := Asset("v0.58.0+02112026", c.goos, c.goarch)
		if nombre != c.nombre || (err == nil) != (c.nombre != "") {
			t.Errorf("%s/%s: %q %v, se esperaba %q", c.goos, c.goarch, nombre, err, c.nombre)
		}
	}
}

func TestComprobadorRenovar(t *testing.T) {
	var veces atomic.Int32
	c := &Comprobador{Actual: "v1.0.0", Consultar: func(context.Context) (Version, error) {
		veces.Add(1)
		return Version{Etiqueta: "v1.1.0"}, nil
	}}
	if _, nueva := c.Disponible(context.Background(), true); !nueva {
		t.Fatal("la v1.1.0 es más nueva que la v1.0.0")
	}
	c.Renovar() // recién consultada: no vuelve a GitHub
	c.Disponible(context.Background(), true)
	if n := veces.Load(); n != 1 {
		t.Fatalf("se consultó %d veces seguidas", n)
	}
	c.mu.Lock()
	c.hecha = time.Now().Add(-11 * time.Second)
	c.mu.Unlock()
	c.Renovar()
	c.Disponible(context.Background(), true)
	if hecha, fallo := c.Comprobado(); veces.Load() != 2 || fallo || time.Since(hecha) > time.Second {
		t.Fatalf("Buscar ahora: %d consultas, fallo %v, hecha %v", veces.Load(), fallo, hecha)
	}
}

func TestLimpiarDescargas(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	t.Setenv("TMPDIR", tmp)
	vieja, reciente, ajena := filepath.Join(tmp, "tramtai-actualizacion111"), filepath.Join(tmp, "tramtai-actualizacion222"), filepath.Join(tmp, "otra-cosa")
	for _, d := range []string{vieja, reciente, ajena} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "instalador.exe"), []byte("x"), 0o644)
	}
	hace := time.Now().Add(-time.Hour)
	os.Chtimes(vieja, hace, hace)
	os.Chtimes(ajena, hace, hace)
	LimpiarDescargas(time.Now().Add(-10 * time.Minute))
	if _, err := os.Stat(vieja); !os.IsNotExist(err) {
		t.Errorf("la descarga vieja debía borrarse: %v", err)
	}
	for _, d := range []string{reciente, ajena} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("%s debía seguir: %v", filepath.Base(d), err)
		}
	}
}

// servidorVersiones publica la lista de versiones de GitHub con las etiquetas dadas; estado distinto de 200 simula
// un fallo (límite de consultas).
func servidorVersiones(t *testing.T, estado int, versiones string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(estado)
		fmt.Fprint(w, versiones)
	}))
	t.Cleanup(srv.Close)
	antes := urlVersiones
	urlVersiones = srv.URL
	reiniciarCache()
	t.Cleanup(func() { urlVersiones = antes; reiniciarCache() })
}

func ficha(etiqueta, pagina string, borrador, previa bool) string {
	return fmt.Sprintf(`{"tag_name":%q,"html_url":%q,"body":"notas de %s","draft":%t,"prerelease":%t}`, etiqueta, pagina, etiqueta, borrador, previa)
}

func etiquetas(vs []Version) string {
	var e []string
	for _, v := range vs {
		e = append(e, v.Etiqueta)
	}
	return strings.Join(e, ",")
}

func TestNovedades(t *testing.T) {
	repo := "https://github.com/MscottB/Tramtai/releases/tag/"
	servidorVersiones(t, http.StatusOK, "["+strings.Join([]string{
		ficha("v0.37.0+1", repo+"v0.37.0", false, false),
		ficha("v0.36.3+1", repo+"v0.36.3", false, true),
		ficha("v0.36.2+1", repo+"v0.36.2", true, false),
		ficha("v0.36.1+1", repo+"v0.36.1", false, false),
		ficha("v0.36.0+1", "https://otro.sitio/v0.36.0", false, false),
		ficha("v0.35.1+1", repo+"v0.35.1", false, false),
	}, ",")+"]")
	ctx := context.Background()
	vs, err := Novedades(ctx, "v0.35.1+9", "v0.36.1+9")
	if err != nil || etiquetas(vs) != "v0.36.1+1,v0.36.0+1" {
		t.Fatalf("de la 0.35.1 a la 0.36.1, sin borrador, previa ni futuras: %q %v", etiquetas(vs), err)
	}
	if vs[0].Notas != "notas de v0.36.1+1" || vs[0].Pagina != repo+"v0.36.1" || vs[1].Pagina != "" {
		t.Fatalf("notas y solo páginas del repositorio: %+v", vs)
	}
	if vs, _ := Novedades(ctx, "v0.37.0+1", "v0.36.1+1"); len(vs) != 0 {
		t.Fatalf("al volver a una versión anterior no hay novedades: %q", etiquetas(vs))
	}
	if vs, _ := Novedades(ctx, "v0.36.1+16102026", "v0.36.1+17102026"); len(vs) != 0 {
		t.Fatalf("la misma versión con otra fecha no es nueva: %q", etiquetas(vs))
	}
	if vs, _ := Novedades(ctx, "v0.35.1+1", "dev"); len(vs) != 0 {
		t.Fatalf("una compilación sin versión no tiene novedades: %q", etiquetas(vs))
	}
	if vs, _ := Novedades(ctx, "", "v0.36.1+1"); etiquetas(vs) != "v0.36.1+1" {
		t.Fatalf("sin desde, solo la instalada: %q", etiquetas(vs))
	}
}

func TestNovedadesTope(t *testing.T) {
	var fichas []string
	for menor := 34; menor >= 20; menor-- {
		fichas = append(fichas, ficha(fmt.Sprintf("v0.%d.0+1", menor), "", false, false))
	}
	servidorVersiones(t, http.StatusOK, "["+strings.Join(fichas, ",")+"]")
	vs, err := Novedades(context.Background(), "v0.1.0", "v0.34.0")
	if err != nil || len(vs) != 10 || vs[0].Etiqueta != "v0.34.0+1" {
		t.Fatalf("como mucho 10, la más nueva primero: %q %v", etiquetas(vs), err)
	}
}

func TestNovedadesFallo(t *testing.T) {
	servidorVersiones(t, http.StatusForbidden, `{"message":"API rate limit exceeded"}`)
	if _, err := Novedades(context.Background(), "v0.35.0", "v0.36.0"); err == nil {
		t.Fatal("un 403 de GitHub debe devolver error")
	}
}

func TestNovedadesCache(t *testing.T) {
	var consultas atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		consultas.Add(1)
		fmt.Fprint(w, "["+ficha("v0.36.1+1", "", false, false)+"]")
	}))
	t.Cleanup(srv.Close)
	antes := urlVersiones
	urlVersiones = srv.URL
	reiniciarCache()
	t.Cleanup(func() { urlVersiones = antes; reiniciarCache() })
	Novedades(context.Background(), "v0.35.0", "v0.36.1")
	Novedades(context.Background(), "v0.36.0", "v0.36.1")
	if consultas.Load() != 1 {
		t.Fatalf("dentro de 10 minutos la lista se reutiliza: %d consultas", consultas.Load())
	}
	reiniciarCache()
	Novedades(context.Background(), "v0.35.0", "v0.36.1")
	if consultas.Load() != 2 {
		t.Fatalf("sin caché vuelve a consultar: %d", consultas.Load())
	}
}

// Las notas llevan el bloque inglés al final: el recorte deja sitio para las dos partes.
func TestArmarNotasEnDosIdiomas(t *testing.T) {
	notas := "Tramtai v0.44.0\n\n" + strings.Repeat("cambio ", 1100) + "\n\n## English\n\nTramtai v0.44.0\n\nSummary."
	if v := armar("v0.44.0", "", notas); !strings.HasSuffix(v.Notas, "Summary.") {
		t.Fatalf("se perdió el bloque inglés: …%q", v.Notas[len(v.Notas)-40:])
	}
	if v := armar("v0.44.0", "", strings.Repeat("x", 12001)); len([]rune(v.Notas)) != 12001 || !strings.HasSuffix(v.Notas, "…") {
		t.Fatalf("recorte: %d runas", len([]rune(v.Notas)))
	}
}
