package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestOpenAPICubreLasRutas falla si una ruta /api/v1 de Rutas no está descrita en openapi.json, o al revés.
func TestOpenAPICubreLasRutas(t *testing.T) {
	fuente, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(documentoOpenAPI, &doc); err != nil {
		t.Fatalf("openapi.json no es JSON válido: %v", err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.1") {
		t.Fatalf("openapi = %q", doc.OpenAPI)
	}
	enCodigo := map[string]bool{}
	for _, m := range regexp.MustCompile(`HandleFunc\("(GET|POST|DELETE) /api/v1(/[^"]*)"`).FindAllStringSubmatch(string(fuente), -1) {
		enCodigo[strings.ToLower(m[1])+" "+m[2]] = true
	}
	if len(enCodigo) < 10 {
		t.Fatalf("solo se encontraron %d rutas en api.go", len(enCodigo))
	}
	enDocumento := map[string]bool{}
	for ruta, ops := range doc.Paths {
		for metodo := range ops {
			if metodo != "parameters" {
				enDocumento[metodo+" "+ruta] = true
			}
		}
	}
	for r := range enCodigo {
		if !enDocumento[r] {
			t.Errorf("%s está en api.go pero no en openapi.json", r)
		}
	}
	for r := range enDocumento {
		if !enCodigo[r] {
			t.Errorf("%s está en openapi.json pero no en api.go", r)
		}
	}
}

func servidores(t *testing.T, cuerpo string) (string, map[string]any) {
	t.Helper()
	var doc struct {
		Servers []struct{ URL string }    `json:"servers"`
		Paths   map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal([]byte(cuerpo), &doc); err != nil || len(doc.Servers) != 1 {
		t.Fatalf("documento %v: %.200s", err, cuerpo)
	}
	return doc.Servers[0].URL, doc.Paths["/trabajos"]
}

func TestOpenAPIServers(t *testing.T) {
	ts := servidor(t, simulado(trozos{}), nil)
	sinClave, err := http.Get(ts.URL + "/api/v1/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	sinClave.Body.Close()
	if sinClave.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sin clave: %d", sinClave.StatusCode)
	}
	r, cuerpo := pedir(t, ts, "GET", "/api/v1/openapi.json", "")
	if r.StatusCode != http.StatusOK || r.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("%d %s", r.StatusCode, r.Header.Get("Content-Type"))
	}
	if url, trabajos := servidores(t, cuerpo); url != ts.URL+"/api/v1" || trabajos["delete"] == nil {
		t.Fatalf("instancia: %s, delete %v", url, trabajos["delete"] != nil)
	}

	// Un punto con ruta /p/<nombre>: servers lleva el prefijo y no describe lo que solo tiene la instancia.
	s := nuevoServidor(t, simulado(trozos{}), nil, &Punto{Nombre: "demo", Contratos: map[string]bool{"nativa": true}})
	mux := http.NewServeMux()
	mux.Handle("/p/demo/", http.StripPrefix("/p/demo", s.Handler()))
	tp := httptest.NewServer(mux)
	defer tp.Close()
	r, cuerpo = pedir(t, tp, "GET", "/p/demo/api/v1/openapi.json?x=1", "")
	if r.StatusCode != http.StatusOK {
		t.Fatalf("punto: %d %s", r.StatusCode, cuerpo)
	}
	if url, trabajos := servidores(t, cuerpo); url != tp.URL+"/p/demo/api/v1" || trabajos["delete"] != nil || trabajos["post"] == nil {
		t.Fatalf("punto: %s, delete %v", url, trabajos["delete"] != nil)
	}
}
