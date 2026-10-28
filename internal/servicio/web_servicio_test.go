package servicio

import (
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
)

// TestAutenticacionRemotaProhibida: en el servidor, cambiar la autenticación de un agente desde una sesión remota (la
// de la contraseña) es 403. Desde este equipo con el servicio expuesto, pasar a suscripción es 409 con la orden de la
// suscripción personal; pasar a clave de API sigue valiendo.
func TestAutenticacionRemotaProhibida(t *testing.T) {
	s, ts, seguro := conAdministrador(t, "", "")
	modo := adaptadores.NuevoModo(adaptadores.AutenticacionClaveAPI)
	s.opc.Registro["claude"] = conModo{modo: modo}
	n := navegadorSeguro(t, seguro)
	n.acceder(contrasenaPrueba, "")
	for _, cabeceras := range [][]string{{"X-CSRF-Token", n.csrf}, nil} {
		if r, cuerpo := n.pedir("POST", "/ui/autenticacion", `{"agente":"claude","modo":"suscripcion"}`, cabeceras...); r.StatusCode != 403 || modo.Valor() != adaptadores.AutenticacionClaveAPI {
			t.Fatalf("desde una sesión remota: %d %s", r.StatusCode, cuerpo)
		}
	}
	s.opc.Expuesto = true // p. ej. detrás de un proxy (servidor.Expuesto, al arrancar)
	ruta := "/api/v1/servicio/agentes/claude/autenticacion"
	if r, cuerpo := admin(t, s, ts.URL, "PUT", ruta, `{"modo":"suscripcion"}`); r.StatusCode != 409 || modo.Valor() != adaptadores.AutenticacionClaveAPI ||
		!strings.Contains(cuerpo, "agentes suscripcion claude --personal") {
		t.Fatalf("expuesto, a suscripción: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := admin(t, s, ts.URL, "PUT", ruta, `{"modo":"clave_api"}`); r.StatusCode != 204 {
		t.Fatalf("expuesto, a clave de API: %d %s", r.StatusCode, cuerpo)
	}
	// En el escritorio expuesto tampoco: el error de siempre (no hay suscripción personal que ofrecer).
	s.opc.Entorno = "escritorio"
	if r, cuerpo := admin(t, s, ts.URL, "PUT", ruta, `{"modo":"suscripcion"}`); r.StatusCode != 400 || !strings.Contains(cuerpo, "configuracion_invalida") ||
		modo.Valor() != adaptadores.AutenticacionClaveAPI {
		t.Fatalf("escritorio expuesto: %d %s", r.StatusCode, cuerpo)
	}
}
