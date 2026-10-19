package api

import (
	"encoding/json"
	"net/http"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// estadoConversor responde el estado del conversor de archivos al momento (para seguir una preparación).
func (s *Servidor) estadoConversor(w http.ResponseWriter, r *http.Request) {
	if s.Conversor == nil {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.T("esta instancia no tiene conversor de archivos")))
		return
	}
	EscribirJSON(w, http.StatusOK, s.Conversor.Estado())
}

// prepararConversor lanza la preparación (o devuelve la que está en curso).
func (s *Servidor) prepararConversor(w http.ResponseWriter, r *http.Request) {
	if s.Conversor == nil {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.T("esta instancia no tiene conversor de archivos")))
		return
	}
	EscribirJSON(w, http.StatusAccepted, s.Conversor.Preparar())
}

// ajustesConversor guarda conversion.preparar_al_iniciar.
func (s *Servidor) ajustesConversor(w http.ResponseWriter, r *http.Request) {
	var cuerpo struct {
		PrepararAlIniciar *bool `json:"preparar_al_iniciar"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&cuerpo); err != nil || cuerpo.PrepararAlIniciar == nil {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("envía {\"preparar_al_iniciar\": true o false}")))
		return
	}
	if s.FijarPrepararAlIniciar == nil || s.Conversor == nil {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.T("esta instancia no tiene conversor de archivos")))
		return
	}
	if err := s.FijarPrepararAlIniciar(*cuerpo.PrepararAlIniciar); err != nil {
		EscribirError(w, r, err)
		return
	}
	s.Conversor.FijarAlIniciar(*cuerpo.PrepararAlIniciar)
	w.WriteHeader(http.StatusNoContent)
}
