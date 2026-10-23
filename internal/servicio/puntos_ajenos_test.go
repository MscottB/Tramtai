package servicio

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
)

// Ninguna ruta de un trabajo ({id}) deja a una API ver ni tocar el trabajo de otra. Recorre todas las rutas de
// openapi.json (que TestOpenAPICubreLasRutas mantiene igual a api.go): una ruta nueva que se olvide de comprobar a qué
// API pertenece el trabajo falla aquí.
func TestNingunaRutaDejaVerTrabajosDeOtraAPI(t *testing.T) {
	s, ts, _ := entorno(t, puntosOrigen, "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	tienda, _ := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, "tienda"))
	soporte, _ := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, "soporte"))
	s.Recargar()
	r, cuerpo := pedir(t, ts.URL, "POST", "/p/soporte/api/v1/trabajos", soporte, hola)
	if r.StatusCode != http.StatusAccepted {
		t.Fatalf("crear en soporte: %d %s", r.StatusCode, cuerpo)
	}
	var trabajo struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(cuerpo), &trabajo)

	documento, err := os.ReadFile("../api/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(documento, &doc); err != nil {
		t.Fatal(err)
	}
	cliente := &http.Client{Timeout: 5 * time.Second} // un flujo SSE ajeno que no se cortara colgaría la prueba
	rutas := 0
	for ruta, operaciones := range doc.Paths {
		if !strings.Contains(ruta, "{id}") {
			continue
		}
		for metodo := range operaciones {
			if metodo == "parameters" {
				continue
			}
			rutas++
			req, _ := http.NewRequest(strings.ToUpper(metodo), ts.URL+"/p/tienda/api/v1"+strings.ReplaceAll(ruta, "{id}", trabajo.ID), strings.NewReader("{}"))
			req.Header.Set("Authorization", "Bearer "+tienda)
			req.Header.Set("Content-Type", "application/json")
			resp, err := cliente.Do(req)
			if err != nil {
				t.Errorf("%s %s: %v", metodo, ruta, err)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("%s %s con el trabajo de otra API: %d, se esperaba 404", metodo, ruta, resp.StatusCode)
			}
		}
	}
	if rutas < 5 {
		t.Fatalf("solo se recorrieron %d rutas con {id}", rutas)
	}
	if r, _ := pedir(t, ts.URL, "GET", "/p/soporte/api/v1/trabajos/"+trabajo.ID, soporte, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("su propia API sí lo ve: %d", r.StatusCode)
	}
}
