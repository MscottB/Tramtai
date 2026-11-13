package api

import (
	"context"
	"errors"
	"net/http"

	"tramtai/internal/agentes"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/preparar"
)

// Preparacion instala, actualiza y desinstala los CLI de los agentes e inicia su sesión (preparar.Preparador).
type Preparacion interface {
	Estado(agente string) (contrato.EstadoPreparacion, bool)
	Iniciar(agente string, pedido contrato.PedidoPreparacion) (contrato.EstadoPreparacion, error)
	Cancelar(agente string) error
	Codigo(agente, codigo string) error
}

// agentePreparable es el id de la ruta si es de un agente del catálogo con receta; si no, responde 404 sin tocar nada.
func agentePreparable(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if f, ok := agentes.Buscar(id); !ok || f.Receta == nil {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("no hay ningún agente %s", id)))
		return "", false
	}
	return id, true
}

// errorPreparacion responde un fallo de la preparación: 409 con su código (con ya_en_curso, también el estado de la que
// está en marcha); uno del servicio (servicio_detenido tras Cerrar), con el suyo; uno sin código es de la entrada
// (acción desconocida, código pegado que no vale), 400. Ninguno repite lo pegado.
func errorPreparacion(w http.ResponseWriter, r *http.Request, estado contrato.EstadoPreparacion, err error) {
	var e *preparar.Error
	switch {
	case nucleo.CodigoDe(err) != "":
		EscribirError(w, r, err)
		return
	case !errors.As(err, &e):
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, err.Error()))
		return
	}
	cuerpo := map[string]any{"error": map[string]any{"codigo": e.Codigo, "mensaje": e.Mensaje}}
	if e.Codigo == preparar.CodigoYaEnCurso {
		cuerpo["preparacion"] = estado
	}
	EscribirJSON(w, http.StatusConflict, cuerpo)
}

// estadoPreparacion responde la preparación en marcha o la última del agente (null si no hubo ninguna).
func (s *Servidor) estadoPreparacion(w http.ResponseWriter, r *http.Request) {
	id, ok := agentePreparable(w, r)
	if !ok {
		return
	}
	var estado *contrato.EstadoPreparacion
	if e, hay := s.Preparacion.Estado(id); hay {
		estado = &e
	}
	EscribirJSON(w, http.StatusOK, estado)
}

// iniciarPreparacion lanza la acción pedida en segundo plano: 202 con su estado.
func (s *Servidor) iniciarPreparacion(w http.ResponseWriter, r *http.Request) {
	id, ok := agentePreparable(w, r)
	if !ok {
		return
	}
	var pedido contrato.PedidoPreparacion
	if err := LeerJSON(w, r, &pedido); err != nil {
		EscribirError(w, r, err)
		return
	}
	e, err := s.Preparacion.Iniciar(id, pedido)
	if err != nil {
		errorPreparacion(w, r, e, err)
		return
	}
	EscribirJSON(w, http.StatusAccepted, e)
}

// cancelarPreparacion corta la preparación en marcha del agente (sin ninguna, no hace nada).
func (s *Servidor) cancelarPreparacion(w http.ResponseWriter, r *http.Request) {
	id, ok := agentePreparable(w, r)
	if !ok {
		return
	}
	if err := s.Preparacion.Cancelar(id); err != nil {
		errorPreparacion(w, r, contrato.EstadoPreparacion{}, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// codigoPreparacion pasa al inicio de sesión en marcha lo que el usuario pegó. Lo pegado no vuelve en la respuesta.
func (s *Servidor) codigoPreparacion(w http.ResponseWriter, r *http.Request) {
	id, ok := agentePreparable(w, r)
	if !ok {
		return
	}
	var cuerpo struct {
		Codigo string `json:"codigo"`
	}
	if LeerJSON(w, r, &cuerpo) != nil { // un mensaje fijo: el del decodificador podría citar lo pegado
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("cuerpo no válido: envía el código pegado en el campo codigo")))
		return
	}
	if err := s.Preparacion.Codigo(id, cuerpo.Codigo); err != nil {
		errorPreparacion(w, r, contrato.EstadoPreparacion{}, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// versionesAgentes responde las versiones de los CLI: la última comprobación guardada o, con ?forzar=1, una nueva. No se
// corta si el cliente se va: el resultado se guarda para el siguiente.
func (s *Servidor) versionesAgentes(w http.ResponseWriter, r *http.Request) {
	EscribirJSON(w, http.StatusOK, s.VersionesAgentes(context.WithoutCancel(r.Context()), r.URL.Query().Get("forzar") == "1"))
}
