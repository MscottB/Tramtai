package servicio

import (
	"fmt"
	"net/http"

	"tramtai/internal/api"
	"tramtai/internal/contrato"
	"tramtai/internal/terminos"
)

// terminosUI dice si este entorno aún tiene que aceptar los términos de uso vigentes. Si no se puede leer terminos.json
// cuenta como pendiente (se pide la aceptación otra vez, no se la salta) y queda una línea en el registro del servicio.
func (s *Servicio) terminosUI(w http.ResponseWriter, _ *http.Request, _ *sesion) {
	pendiente, err := terminos.Pendiente(s.opc.Datos)
	if err != nil {
		fmt.Fprintf(s.opc.Registros, "los términos de uso cuentan como pendientes: %s\n", err)
	}
	servidor := s.entorno() == "servidor"
	api.EscribirJSON(w, http.StatusOK, contrato.EstadoTerminos{Version: terminos.Version, Pendiente: pendiente, Riesgos: terminos.Riesgos(servidor), Servidor: servidor})
}

// aceptarTerminosUI guarda la aceptación hecha en la Web UI. Cambia datos del entorno, así que la ruta pide el CSRF de la
// sesión (conSesionYCSRF).
func (s *Servicio) aceptarTerminosUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if err := terminos.Aceptar(s.opc.Datos, terminos.ComoInterfaz); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
