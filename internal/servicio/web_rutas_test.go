package servicio

import (
	"testing"

	"tramtai/internal/adaptadores"
)

// TestPermisosDeRutasUI recorre la tabla: sin sesión, ninguna ruta que pide sesión o sesión local se atiende.
func TestPermisosDeRutasUI(t *testing.T) {
	_, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	for _, ruta := range rutasDeUI {
		if ruta.permiso == publica {
			continue
		}
		if r, cuerpo := pedir(t, ts.URL, ruta.metodo, ruta.ruta, "", "{}"); r.StatusCode != 401 {
			t.Errorf("%s %s sin sesión: %d %s", ruta.metodo, ruta.ruta, r.StatusCode, cuerpo)
		}
	}
}
