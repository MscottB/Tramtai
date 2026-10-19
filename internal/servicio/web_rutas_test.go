package servicio

import (
	"net/http/httptest"
	"testing"

	"tramtai/internal/adaptadores"
)

// TestPermisosDeRutasUI recorre la tabla: sin sesión no se atiende ninguna ruta con permiso; con una sesión que no es
// local, ninguna soloLocal; y con una local, ninguna soloLocal sin su CSRF.
func TestPermisosDeRutasUI(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	sesion := func(local bool) string {
		w := httptest.NewRecorder()
		s.crearSesion(w, "global", local)
		return w.Result().Cookies()[0].String()
	}
	remota, local := sesion(false), sesion(true)
	for _, ruta := range rutasDeUI {
		if ruta.permiso == publica {
			continue
		}
		if r, cuerpo := pedir(t, ts.URL, ruta.metodo, ruta.ruta, "", "{}"); r.StatusCode != 401 {
			t.Errorf("%s %s sin sesión: %d %s", ruta.metodo, ruta.ruta, r.StatusCode, cuerpo)
		}
		if ruta.permiso != soloLocal {
			continue
		}
		if r, cuerpo := pedir(t, ts.URL, ruta.metodo, ruta.ruta, "", "{}", "Cookie", remota); r.StatusCode != 401 {
			t.Errorf("%s %s con una sesión que no es local: %d %s", ruta.metodo, ruta.ruta, r.StatusCode, cuerpo)
		}
		if r, cuerpo := pedir(t, ts.URL, ruta.metodo, ruta.ruta, "", "{}", "Cookie", local); r.StatusCode != 401 {
			t.Errorf("%s %s con una sesión local sin CSRF: %d %s", ruta.metodo, ruta.ruta, r.StatusCode, cuerpo)
		}
	}
}
