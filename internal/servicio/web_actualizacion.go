package servicio

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tramtai/internal/actualizacion"
	"tramtai/internal/api"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Rutas de /ui/ de las actualizaciones y de Acerca de (rutasDeUI). La administración (rutasAdmin) las atiende igual, sin
// sesión, para la terminal.

// actualizacionUI consulta el estado de las actualizaciones (GET) o activa o desactiva su búsqueda (POST o PUT).
func (s *Servicio) actualizacionUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if r.Method != http.MethodGet {
		var peticion struct {
			Buscar bool `json:"buscar"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if err := s.fijarBusqueda(peticion.Buscar); err != nil {
			api.EscribirError(w, r, err)
			return
		}
	}
	ctx, cancelar := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancelar()
	api.EscribirJSON(w, http.StatusOK, s.estadoActualizacion(ctx, r.Method == http.MethodGet && r.URL.Query().Get("ahora") == "1"))
}

func (s *Servicio) actualizarUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	ctx, cancelar := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancelar()
	v, err := s.IniciarActualizacion(ctx)
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	api.EscribirJSON(w, http.StatusAccepted, map[string]any{"version": v.Etiqueta})
}

// decidirInstalacionUI instala ya (…/actualizar/instalar) o cancela la cuenta atrás (…/actualizar/cancelar).
func (s *Servicio) decidirInstalacionUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if err := s.decidirInstalacion(strings.HasSuffix(r.URL.Path, "/instalar")); err != nil {
		api.EscribirJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"codigo": nucleo.CodigoDe(err), "mensaje": nucleo.MensajeDe(err)}})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servicio) acercaUI(w http.ResponseWriter, _ *http.Request, ses *sesion) {
	api.EscribirJSON(w, http.StatusOK, s.acerca(ses.local))
}

func (s *Servicio) progresoUI(w http.ResponseWriter, _ *http.Request, _ *sesion) {
	api.EscribirJSON(w, http.StatusOK, s.progresoActualizacion())
}

// novedadesUI es lo que trajo cada versión desde la última vista en este navegador (o lo que trae cada una hasta la
// nueva); sin búsqueda de actualizaciones no se habla con GitHub.
func (s *Servicio) novedadesUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if !s.buscar.Load() {
		api.EscribirJSON(w, http.StatusOK, map[string]any{"versiones": []actualizacion.Version{}})
		return
	}
	ctx, cancelar := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancelar()
	// hasta (opcional) es la versión nueva, para leer antes de actualizar lo que trae cada una que se salta.
	versiones, err := s.novedades(ctx, r.URL.Query().Get("desde"), r.URL.Query().Get("hasta"))
	if err != nil {
		fmt.Fprintf(s.opc.Registros, "novedades: %v\n", err) // el detalle (red, proxy) queda aquí, no en la respuesta
		api.EscribirJSON(w, http.StatusBadGateway, map[string]any{"error": map[string]any{"codigo": "github_no_disponible", "mensaje": idioma.T("GitHub no está disponible")}})
		return
	}
	api.EscribirJSON(w, http.StatusOK, map[string]any{"versiones": versiones})
}
