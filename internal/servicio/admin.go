package servicio

import (
	"net/http"

	"tramtai/internal/api"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// rutasAdmin es la administración del servicio; solo se atiende en el puerto principal con el token de servicio.
func (s *Servicio) rutasAdmin() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/servicio", func(w http.ResponseWriter, _ *http.Request) {
		api.EscribirJSON(w, http.StatusOK, s.Estado())
	})
	mux.HandleFunc("POST /api/v1/servicio/instancias", s.adminActivar)
	mux.HandleFunc("DELETE /api/v1/servicio/instancias/{id}", s.adminDesactivar)
	mux.HandleFunc("GET /api/v1/servicio/instancias/{id}/configuracion", s.adminConfiguracion)
	mux.HandleFunc("POST /api/v1/servicio/sesiones", s.adminSesion)
	mux.HandleFunc("POST /api/v1/servicio/recargar", func(w http.ResponseWriter, _ *http.Request) {
		puntos := s.Recargar()
		_ = s.actualizarDescriptor()
		api.EscribirJSON(w, http.StatusOK, map[string]any{"puntos_acceso": puntos})
	})
	mux.HandleFunc("POST /api/v1/servicio/detener", func(w http.ResponseWriter, _ *http.Request) {
		api.EscribirJSON(w, http.StatusAccepted, map[string]any{"estado": "deteniendo"})
		s.detenerUna.Do(func() { close(s.detener) })
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("ruta no encontrada: %s %s", r.Method, r.URL.Path)))
	})
	return mux
}

// ActivacionInstancia es la respuesta a la activación de una instancia.
type ActivacionInstancia struct {
	Instancia EstadoInstancia `json:"instancia"`
	Token     string          `json:"token"`
}

func (s *Servicio) adminActivar(w http.ResponseWriter, r *http.Request) {
	var peticion struct {
		Ambito string `json:"ambito"`
		Raiz   string `json:"raiz"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	var inst *Instancia
	var err error
	switch peticion.Ambito {
	case "global":
		inst, err = s.ActivarGlobal()
	case "proyecto":
		inst, err = s.ActivarProyecto(peticion.Raiz)
	default:
		err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("ámbito desconocido %q: usa global o proyecto", peticion.Ambito))
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	_ = s.guardarActivas()
	_ = s.actualizarDescriptor()
	api.EscribirJSON(w, http.StatusOK, ActivacionInstancia{Instancia: inst.estado(), Token: inst.token})
}

func (s *Servicio) adminDesactivar(w http.ResponseWriter, r *http.Request) {
	if err := s.Desactivar(r.PathValue("id")); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	_ = s.guardarActivas()
	_ = s.actualizarDescriptor()
	w.WriteHeader(http.StatusNoContent)
}

// EntradaConfiguracion es una fila de la configuración efectiva de una instancia.
type EntradaConfiguracion struct {
	Clave  string `json:"clave"`
	Valor  any    `json:"valor"`
	Origen string `json:"origen"`
}

func (s *Servicio) adminConfiguracion(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	inst := s.instancias[r.PathValue("id")]
	s.mu.RUnlock()
	if inst == nil {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.T("la instancia no está activa")))
		return
	}
	entradas := []EntradaConfiguracion{}
	for _, e := range inst.Config.Efectiva() {
		entradas = append(entradas, EntradaConfiguracion{Clave: e.Clave, Valor: e.Valor, Origen: e.Origen.String()})
	}
	api.EscribirJSON(w, http.StatusOK, map[string]any{"configuracion": entradas})
}

// adminSesion devuelve una URL de un solo uso que abre la Web UI de una instancia (tramtai global abrir).
func (s *Servicio) adminSesion(w http.ResponseWriter, r *http.Request) {
	var peticion struct {
		Instancia string `json:"instancia"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	codigo, err := s.CodigoSesion(peticion.Instancia)
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	api.EscribirJSON(w, http.StatusOK, map[string]any{"url": s.Base() + "/ui/entrar?codigo=" + codigo})
}
