package servicio

import (
	"encoding/json"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
	"tramtai/internal/credenciales"
)

// almacenFalso sustituye el almacén de credenciales del sistema durante la prueba.
func almacenFalso(t *testing.T) map[string]string {
	t.Helper()
	almacen := map[string]string{}
	g, b, l := guardarCredencial, borrarCredencial, leerCredencial
	t.Cleanup(func() { guardarCredencial, borrarCredencial, leerCredencial = g, b, l })
	guardarCredencial = func(nombre, valor string) error { almacen[nombre] = valor; return nil }
	borrarCredencial = func(nombre string) error { delete(almacen, nombre); return nil }
	leerCredencial = func(nombre string) (string, error) { return almacen[nombre], nil }
	for _, nombre := range credenciales.Nombres { // las del entorno de quien prueba no cuentan
		t.Setenv(nombre, "")
	}
	return almacen
}

// Las claves se guardan con el token de servicio y además solo desde este equipo: una petición que llega por un proxy
// (aunque traiga el token) se rechaza con 403 y no toca el almacén.
func TestAdminClavesSoloLocal(t *testing.T) {
	almacen := almacenFalso(t)
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	ruta := "/api/v1/servicio/claves/ANTHROPIC_API_KEY"
	for _, metodo := range []string{"PUT", "DELETE", "GET"} {
		destino := ruta
		if metodo == "GET" {
			destino = "/api/v1/servicio/claves"
		}
		if r, cuerpo := pedir(t, ts.URL, metodo, destino, s.tokenAdmin, `{"valor":"sk-prueba-123"}`, "X-Forwarded-For", "203.0.113.7"); r.StatusCode != 403 {
			t.Errorf("%s por un proxy: %d %s", metodo, r.StatusCode, cuerpo)
		}
	}
	if len(almacen) > 0 {
		t.Fatalf("por un proxy no se guarda nada: %v", almacen)
	}
	if r, cuerpo := admin(t, s, ts.URL, "PUT", ruta, `{"valor":"sk-prueba-123"}`); r.StatusCode != 204 || almacen["ANTHROPIC_API_KEY"] != "sk-prueba-123" {
		t.Fatalf("desde este equipo: %d %s %v", r.StatusCode, cuerpo, almacen)
	}
	if r, _ := pedir(t, ts.URL, "PUT", ruta, "", `{"valor":"sk-otra-123"}`); r.StatusCode != 401 || almacen["ANTHROPIC_API_KEY"] != "sk-prueba-123" {
		t.Fatalf("sin el token de servicio: %d", r.StatusCode)
	}
	for _, mal := range []struct{ ruta, cuerpo string }{
		{"/api/v1/servicio/claves/CLAUDE_CODE_OAUTH_TOKEN", `{"valor":"sk-prueba-123"}`},
		{"/api/v1/servicio/claves/TRAMTAI_TOTP", `{"valor":"ABCDEFGH"}`},
		{ruta, `{"valor":"con espacio"}`},
	} {
		if r, cuerpo := admin(t, s, ts.URL, "PUT", mal.ruta, mal.cuerpo); r.StatusCode != 400 {
			t.Errorf("%s %s: %d %s", mal.ruta, mal.cuerpo, r.StatusCode, cuerpo)
		}
	}
	if r, cuerpo := admin(t, s, ts.URL, "DELETE", ruta, ""); r.StatusCode != 204 || len(almacen) > 0 {
		t.Fatalf("borrar: %d %s %v", r.StatusCode, cuerpo, almacen)
	}
}

// La lista dice qué claves tiene el servicio, sin valores y sin las internas (el secreto TOTP del administrador).
func TestClavesListaSinInternos(t *testing.T) {
	almacen := almacenFalso(t)
	almacen["OPENAI_API_KEY"], almacen["TRAMTAI_TOTP"] = "sk-prueba-123", "ABCDEFGH"
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	r, cuerpo := admin(t, s, ts.URL, "GET", "/api/v1/servicio/claves", "")
	var lista []contrato.ClaveGuardada
	if r.StatusCode != 200 || json.Unmarshal([]byte(cuerpo), &lista) != nil {
		t.Fatalf("%d %s", r.StatusCode, cuerpo)
	}
	guardadas := map[string]bool{}
	for _, c := range lista {
		guardadas[c.Nombre] = c.Guardada
	}
	if _, ok := guardadas["TRAMTAI_TOTP"]; ok || !guardadas["OPENAI_API_KEY"] || guardadas["ANTHROPIC_API_KEY"] {
		t.Fatalf("lista: %s", cuerpo)
	}
	if len(lista) != len(credenciales.Nombres) {
		t.Fatalf("todas las claves admitidas, guardadas o no: %s", cuerpo)
	}
}
