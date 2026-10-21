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
	mux.HandleFunc("GET /api/v1/servicio", func(w http.ResponseWriter, r *http.Request) {
		api.EscribirJSON(w, http.StatusOK, s.Estado(r.Context()))
	})
	mux.HandleFunc("POST /api/v1/servicio/instancias", s.adminActivar)
	mux.HandleFunc("DELETE /api/v1/servicio/instancias/{id}", s.adminDesactivar)
	mux.HandleFunc("GET /api/v1/servicio/instancias/{id}/configuracion", s.adminConfiguracion)
	mux.HandleFunc("POST /api/v1/servicio/sesiones", s.adminSesion)
	mux.HandleFunc("GET /api/v1/servicio/espacios", func(w http.ResponseWriter, r *http.Request) {
		api.EscribirJSON(w, http.StatusOK, map[string]any{"espacios": s.Espacios(r.Context())})
	})
	mux.HandleFunc("DELETE /api/v1/servicio/espacios/{id}", func(w http.ResponseWriter, r *http.Request) {
		var peticion struct {
			Nombre string `json:"nombre"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if err := s.Olvidar(r.PathValue("id"), peticion.Nombre); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v1/servicio/puntos", s.adminListarPuntos)
	mux.HandleFunc("POST /api/v1/servicio/puntos", s.adminCrearPunto)
	mux.HandleFunc("POST /api/v1/servicio/puntos/activo", s.adminActivarPuntos)
	mux.HandleFunc("PUT /api/v1/servicio/puntos/{nombre}", s.adminGuardarPunto)
	mux.HandleFunc("DELETE /api/v1/servicio/puntos/{nombre}", s.adminEliminarPunto)
	mux.HandleFunc("POST /api/v1/servicio/puntos/{nombre}/token", s.adminRotarToken)
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
	if err := s.guardarActivas(); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	_ = s.actualizarDescriptor()
	api.EscribirJSON(w, http.StatusOK, ActivacionInstancia{Instancia: inst.estado(r.Context()), Token: s.tokenDe(inst)})
}

func (s *Servicio) adminDesactivar(w http.ResponseWriter, r *http.Request) {
	if err := s.Desactivar(r.PathValue("id")); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if err := s.guardarActivas(); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	_ = s.actualizarDescriptor()
	w.WriteHeader(http.StatusNoContent)
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
