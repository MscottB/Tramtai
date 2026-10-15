package actualizacion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
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
	ruta, err := Descargar(context.Background(), v, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if datos, _ := os.ReadFile(ruta); string(datos) != "instalador de prueba" {
		t.Fatalf("contenido descargado: %q", datos)
	}

	// Una dirección fuera del repositorio de Tramtai no se descarga.
	ajena := v
	ajena.urlInstalador = "https://ejemplo.com/instalador.exe"
	if _, err := Descargar(context.Background(), ajena, t.TempDir()); err == nil {
		t.Fatal("se aceptó una dirección de descarga ajena")
	}
}

func TestDescargarRechazaSumaDistinta(t *testing.T) {
	servidor(t, true)
	v, err := Ultima(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := Descargar(context.Background(), v, dir); err == nil {
		t.Fatal("se aceptó un instalador cuya suma no coincide")
	}
	if archivos, _ := os.ReadDir(dir); len(archivos) != 0 {
		t.Fatalf("quedó el archivo dañado: %v", archivos)
	}
}
