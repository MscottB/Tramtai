package servicio

import (
	"net/http"
	"os"

	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/credenciales"
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
	mux.HandleFunc("POST /api/v1/servicio/agentes/{agente}/conexion", func(w http.ResponseWriter, r *http.Request) {
		var peticion struct {
			Conectado bool `json:"conectado"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if err := s.Conectar(r.PathValue("agente"), peticion.Conectado); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
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
	mux.HandleFunc("POST /api/v1/servicio/espacios/{id}/activar", func(w http.ResponseWriter, r *http.Request) {
		inst, err := s.ActivarEspacio(r.PathValue("id"))
		s.responderEspacio(w, r, inst, err)
	})
	mux.HandleFunc("POST /api/v1/servicio/espacios/{id}/detener", func(w http.ResponseWriter, r *http.Request) {
		if err := s.DetenerEspacio(r.PathValue("id")); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/v1/servicio/espacios/{id}/confianza", func(w http.ResponseWriter, r *http.Request) {
		var peticion struct {
			Confiar *bool `json:"confiar"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if err := s.confiarEspacio(r.PathValue("id"), peticion.Confiar); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/v1/servicio/espacios/{id}/ubicacion", func(w http.ResponseWriter, r *http.Request) {
		var peticion struct {
			Ruta string `json:"ruta"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		inst, err := s.Ubicar(r.PathValue("id"), peticion.Ruta)
		s.responderEspacio(w, r, inst, err)
	})
	mux.HandleFunc("POST /api/v1/servicio/proyectos", func(w http.ResponseWriter, r *http.Request) {
		var peticion struct {
			Raiz      string `json:"raiz"`
			Confiar   *bool  `json:"confiar"`
			CrearAqui bool   `json:"crear_aqui"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		inst, err := s.abrirYGuardar(peticion.Raiz, AbrirOpciones{Confiar: peticion.Confiar, CrearAqui: peticion.CrearAqui})
		if !escribirErrorAbrir(w, r, err) {
			s.responderEspacio(w, r, inst, nil)
		}
	})
	mux.HandleFunc("PUT /api/v1/servicio/agentes/{agente}/autenticacion", func(w http.ResponseWriter, r *http.Request) {
		var peticion struct {
			Modo string `json:"modo"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if err := s.cambiarAutenticacion(r.PathValue("agente"), peticion.Modo); err != nil {
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
	// El puerto se cambia aquí con el servicio en marcha: así la CLI no edita config.yaml a la vez que el servicio.
	mux.HandleFunc("PUT /api/v1/servicio/puerto", func(w http.ResponseWriter, r *http.Request) {
		var peticion struct {
			Puerto     int  `json:"puerto"`
			Automatico bool `json:"automatico"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if err := s.fijarPuerto(peticion.Puerto, peticion.Automatico); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// Ajustes en la terminal: el puerto y el arranque con el sistema, las actualizaciones y Acerca de, con las mismas
	// respuestas que /ui/ (quien tiene el token de servicio es el dueño del equipo: ve también las rutas).
	mux.HandleFunc("GET /api/v1/servicio/ajustes", func(w http.ResponseWriter, _ *http.Request) {
		api.EscribirJSON(w, http.StatusOK, s.ajustesServicio())
	})
	mux.HandleFunc("PUT /api/v1/servicio/autoarranque", func(w http.ResponseWriter, r *http.Request) {
		var peticion peticionArranque
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if err := s.fijarAutoarranque(peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	sinSesion := func(atender func(*Servicio, http.ResponseWriter, *http.Request, *sesion)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { atender(s, w, r, nil) }
	}
	mux.HandleFunc("GET /api/v1/servicio/actualizacion", sinSesion((*Servicio).actualizacionUI))
	mux.HandleFunc("PUT /api/v1/servicio/actualizacion", sinSesion((*Servicio).actualizacionUI))
	mux.HandleFunc("POST /api/v1/servicio/actualizar", sinSesion((*Servicio).actualizarUI))
	mux.HandleFunc("POST /api/v1/servicio/actualizar/instalar", sinSesion((*Servicio).decidirInstalacionUI))
	mux.HandleFunc("POST /api/v1/servicio/actualizar/cancelar", sinSesion((*Servicio).decidirInstalacionUI))
	mux.HandleFunc("GET /api/v1/servicio/actualizacion/progreso", sinSesion((*Servicio).progresoUI))
	mux.HandleFunc("GET /api/v1/servicio/actualizacion/novedades", sinSesion((*Servicio).novedadesUI))
	// «Reiniciar ahora» tras cambiar el puerto: lanza el servicio nuevo y responde dónde escuchará (202).
	mux.HandleFunc("POST /api/v1/servicio/reiniciar", sinSesion((*Servicio).reiniciarUI))
	// Las claves de proveedor del almacén del servicio (tramtai-servidor claves en Windows, donde el almacén es el de la
	// cuenta del servicio): con el token de servicio y además solo desde este equipo, nunca por un proxy.
	mux.HandleFunc("GET /api/v1/servicio/claves", soloDesdeEsteEquipo(func(w http.ResponseWriter, _ *http.Request) {
		lista := []contrato.ClaveGuardada{}
		for _, nombre := range credenciales.Nombres {
			valor, _ := leerCredencial(nombre)
			lista = append(lista, contrato.ClaveGuardada{Nombre: nombre, Guardada: valor != "" || os.Getenv(nombre) != ""})
		}
		api.EscribirJSON(w, http.StatusOK, lista)
	}))
	mux.HandleFunc("PUT /api/v1/servicio/claves/{nombre}", soloDesdeEsteEquipo(func(w http.ResponseWriter, r *http.Request) {
		var peticion struct {
			Valor string `json:"valor"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		nombre := r.PathValue("nombre")
		err := credenciales.Rechazo(nombre)
		if err == nil {
			err = credenciales.Comprobar(nombre, peticion.Valor)
		}
		if err != nil {
			api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, err.Error()))
			return
		}
		if err := guardarCredencial(nombre, peticion.Valor); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("DELETE /api/v1/servicio/claves/{nombre}", soloDesdeEsteEquipo(func(w http.ResponseWriter, r *http.Request) {
		if err := credenciales.Rechazo(r.PathValue("nombre")); err != nil {
			api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, err.Error()))
			return
		}
		if err := borrarCredencial(r.PathValue("nombre")); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.HandleFunc("GET /api/v1/servicio/acerca", func(w http.ResponseWriter, _ *http.Request) {
		api.EscribirJSON(w, http.StatusOK, s.acerca(true))
	})
	mux.HandleFunc("POST /api/v1/servicio/recargar", func(w http.ResponseWriter, _ *http.Request) {
		puntos := s.Recargar()
		_ = s.persistirEstado() // anotado; los puntos ya están en su archivo
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

// El almacén de credenciales del sistema; las pruebas lo sustituyen.
var (
	guardarCredencial = credenciales.Guardar
	borrarCredencial  = credenciales.Borrar
	leerCredencial    = credenciales.Leer
)

// soloDesdeEsteEquipo rechaza con 403 lo que no llega por loopback sin proxy (desdeEsteEquipo).
func soloDesdeEsteEquipo(atender http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !desdeEsteEquipo(r) {
			api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoOrigenNoPermitido, idioma.T("las claves del servicio solo se administran desde este equipo")))
			return
		}
		atender(w, r)
	}
}

// responderEspacio responde con el espacio de inst tal como lo lista GET /espacios, o con el error.
func (s *Servicio) responderEspacio(w http.ResponseWriter, r *http.Request, inst *Instancia, err error) {
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	for _, e := range s.Espacios(r.Context()) {
		if e.ID == inst.ID {
			api.EscribirJSON(w, http.StatusOK, e)
			return
		}
	}
	api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("no existe el espacio %s", inst.ID)))
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
		// Adoptar la marca de un espacio cuya carpeta ya no está (un repositorio clonado puede traerla) pide la misma
		// confirmación que AbrirProyecto: confiar en la carpeta.
		g, errGlobal := configuracion.CargarConfiguracion("", agentes.Internos())
		if movido := s.rutaMovida(peticion.Raiz); movido != "" && (errGlobal != nil || !g.ProyectoConfiable(peticion.Raiz)) {
			err = &ErrorConfianza{Raiz: peticion.Raiz, Movido: movido}
		} else {
			inst, err = s.ActivarProyecto(peticion.Raiz)
		}
	default:
		err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("ámbito desconocido %q: usa global o proyecto", peticion.Ambito))
	}
	if escribirErrorAbrir(w, r, err) {
		return
	}
	if err := s.persistirEstado(); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	api.EscribirJSON(w, http.StatusOK, ActivacionInstancia{Instancia: inst.estado(r.Context())})
}

func (s *Servicio) adminDesactivar(w http.ResponseWriter, r *http.Request) {
	if err := s.Desactivar(r.PathValue("id")); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if err := s.persistirEstado(); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servicio) adminConfiguracion(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	inst := s.instancias[r.PathValue("id")]
	s.mu.RUnlock()
	if inst == nil {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.T("el espacio no está activo")))
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
	api.EscribirJSON(w, http.StatusOK, map[string]any{"url": s.BasePublica() + "/ui/entrar?codigo=" + codigo})
}
