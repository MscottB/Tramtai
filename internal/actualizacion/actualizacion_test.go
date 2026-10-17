package actualizacion

import (
	"context"
	"crypto/sha256"
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

// servidor publica una versión falsa con su instalador y sus sumas; alterado sirve otro contenido que el sumado.
func servidor(t *testing.T, alterado bool) {
	t.Helper()
	contenido := []byte("instalador de prueba")
	suma := sha256.Sum256(contenido)
	const nombre = "Tramtai-v0.26.0+2-instalador-windows-x64.exe"
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/ultima", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v0.26.0+2","html_url":"%[1]s/version","body":"Tramtai v0.26.0\n\n- novedad","assets":[`+
			`{"name":"%[2]s","browser_download_url":"%[1]s/descargas/%[2]s"},`+
			`{"name":"SHA256SUMS.txt","browser_download_url":"%[1]s/descargas/SHA256SUMS.txt"}]}`, srv.URL, nombre)
	})
	mux.HandleFunc("/descargas/SHA256SUMS.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(suma[:]), nombre)
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
	servidor(t, false)
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
	servidor(t, true)
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
	t.Cleanup(func() { urlVersiones = antes })
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
