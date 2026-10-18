package api

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// documentoOpenAPI describe /api/v1; TestOpenAPICubreLasRutas comprueba que sigue al día con Rutas.
//
//go:embed openapi.json
var documentoOpenAPI []byte

// openapi devuelve la descripción con servers apuntando a la URL base de la petición: en un punto con ruta,
// http.StripPrefix quita /p/<nombre> de URL.Path, pero RequestURI lo conserva.
func (s *Servidor) openapi(w http.ResponseWriter, r *http.Request) {
	ruta := r.URL.Path
	if u, err := url.ParseRequestURI(r.RequestURI); err == nil {
		ruta = u.Path
	}
	prefijo, _, _ := strings.Cut(ruta, "/api/v1/")
	esquema := "http"
	if r.TLS != nil {
		esquema = "https"
	}
	var doc map[string]any
	_ = json.Unmarshal(documentoOpenAPI, &doc) // incluido en el ejecutable y validado por las pruebas
	doc["servers"] = []map[string]string{{"url": esquema + "://" + r.Host + prefijo + "/api/v1"}}
	if s.Punto != nil {
		// Un punto de acceso no registra lo que solo existe en la instancia (DELETE /trabajos).
		for _, ops := range doc["paths"].(map[string]any) {
			for metodo, op := range ops.(map[string]any) {
				if o, ok := op.(map[string]any); ok && o["x-tramtai-solo-instancia"] == true {
					delete(ops.(map[string]any), metodo)
				}
			}
		}
	}
	EscribirJSON(w, http.StatusOK, doc)
}
