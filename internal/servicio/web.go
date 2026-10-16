package servicio

import (
	"cmp"
	"context"
	"crypto/subtle"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/api"
	"tramtai/internal/autoarranque"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/version"
)

// sesion es una sesión de la Web UI enlazada a una única instancia: cookie HttpOnly con
// SameSite estricto y un token CSRF obligatorio en las peticiones que modifican.
type sesion struct {
	instancia string
	csrf      string
	vence     time.Time
	// local: entró sin token por ser del mismo usuario del sistema (esLocal); puede cambiar de instancia.
	local bool
}

type codigoEntrada struct {
	instancia string
	vence     time.Time
}

const (
	cookieSesion   = "tramtai_sesion"
	duracionSesion = 12 * time.Hour
	duracionCodigo = time.Minute
)

func (s *Servicio) sesionDe(r *http.Request) (string, *sesion) {
	cookie, err := r.Cookie(cookieSesion)
	if err != nil {
		return "", nil
	}
	s.muSesiones.Lock()
	defer s.muSesiones.Unlock()
	ses := s.sesiones[cookie.Value]
	if ses == nil || time.Now().After(ses.vence) {
		delete(s.sesiones, cookie.Value)
		return "", nil
	}
	ses.vence = time.Now().Add(duracionSesion)
	return cookie.Value, ses
}

func (s *Servicio) crearSesion(w http.ResponseWriter, instancia string, local bool) *sesion {
	id := aleatorio(32)
	ses := &sesion{instancia: instancia, csrf: aleatorio(16), vence: time.Now().Add(duracionSesion), local: local}
	s.muSesiones.Lock()
	for clave, otra := range s.sesiones {
		if time.Now().After(otra.vence) {
			delete(s.sesiones, clave)
		}
	}
	s.sesiones[id] = ses
	s.muSesiones.Unlock()
	http.SetCookie(w, &http.Cookie{Name: cookieSesion, Value: id, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: int(duracionSesion.Seconds())})
	return ses
}

// CodigoSesion crea un código de un solo uso, válido un minuto, que abre la Web UI de una instancia sin escribir
// su token. Lo piden la CLI, la terminal interactiva y la aplicación de escritorio con el token de servicio; el
// token de la instancia nunca viaja en la URL.
func (s *Servicio) CodigoSesion(instancia string) (string, error) {
	s.mu.RLock()
	_, activa := s.instancias[instancia]
	s.mu.RUnlock()
	if !activa {
		return "", nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("la instancia %s no está activa", instancia))
	}
	codigo := aleatorio(16)
	s.muSesiones.Lock()
	for clave, c := range s.codigos {
		if time.Now().After(c.vence) {
			delete(s.codigos, clave)
		}
	}
	s.codigos[codigo] = codigoEntrada{instancia: instancia, vence: time.Now().Add(duracionCodigo)}
	s.muSesiones.Unlock()
	return codigo, nil
}

// rutasUI gestiona la sesión: GET la consulta (y, si la petición es local, entra sin token), POST entra con el
// token de una instancia o, en una sesión local, cambia de instancia, DELETE sale y /ui/entrar canjea un código de
// un solo uso. /ui/instancias activa la instancia global, /ui/proyectos abre la de un proyecto (y lo prepara si
// hace falta) y /ui/autoarranque cambia el arranque con el sistema, todos desde una sesión local.
func (s *Servicio) rutasUI(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/ui/sesion" && r.Method == http.MethodGet:
		_, ses := s.sesionDe(r)
		if ses == nil && s.esLocal(r) {
			ses = s.crearSesion(w, s.instanciaPredeterminada(), true)
		}
		if ses == nil {
			api.NoAutorizado(w, r)
			return
		}
		s.responderSesion(w, r, ses)
	case r.URL.Path == "/ui/sesion" && r.Method == http.MethodPost:
		var peticion struct {
			Token     string `json:"token"`
			Instancia string `json:"instancia"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if peticion.Instancia != "" { // cambio de instancia: solo en una sesión local y con su CSRF
			_, ses := s.sesionDe(r)
			if ses == nil || !ses.local || !csrfValido(r, ses) || !s.esLocal(r) {
				api.NoAutorizado(w, r)
				return
			}
			if s.instancia(peticion.Instancia) == nil {
				api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("la instancia %s no está activa", peticion.Instancia)))
				return
			}
			s.responderSesion(w, r, s.crearSesion(w, peticion.Instancia, true))
			return
		}
		inst := s.instanciaPorToken(strings.TrimSpace(peticion.Token))
		if inst == nil {
			api.NoAutorizado(w, r)
			return
		}
		s.responderSesion(w, r, s.crearSesion(w, inst.ID, false))
	case r.URL.Path == "/ui/instancias" && r.Method == http.MethodPost:
		_, ses := s.sesionDe(r)
		if ses == nil || !ses.local || !csrfValido(r, ses) || !s.esLocal(r) {
			api.NoAutorizado(w, r)
			return
		}
		inst, err := s.ActivarGlobal()
		if err == nil {
			err = s.guardarActivas()
		}
		if err != nil {
			api.EscribirError(w, r, err)
			return
		}
		s.responderSesion(w, r, s.crearSesion(w, inst.ID, true))
	case r.URL.Path == "/ui/proyectos" && r.Method == http.MethodPost:
		_, ses := s.sesionDe(r)
		if ses == nil || !ses.local || !csrfValido(r, ses) || !s.esLocal(r) {
			api.NoAutorizado(w, r)
			return
		}
		var peticion struct {
			Raiz string `json:"raiz"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		inst, err := s.AbrirProyecto(peticion.Raiz)
		if err == nil {
			err = s.guardarActivas()
		}
		if err != nil {
			api.EscribirError(w, r, err)
			return
		}
		s.responderSesion(w, r, s.crearSesion(w, inst.ID, true))
	case r.URL.Path == "/ui/autoarranque" && r.Method == http.MethodPost:
		_, ses := s.sesionDe(r)
		if ses == nil || !ses.local || !csrfValido(r, ses) || !s.esLocal(r) {
			api.NoAutorizado(w, r)
			return
		}
		var peticion struct {
			Activo bool `json:"activo"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		err := autoarranque.Desactivar()
		if peticion.Activo {
			var escritorio string
			if escritorio, err = autoarranque.Escritorio(); err == nil {
				err = autoarranque.Activar(escritorio)
			}
		}
		if err != nil {
			api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, err.Error()))
			return
		}
		s.responderSesion(w, r, ses)
	case r.URL.Path == "/ui/autenticacion" && r.Method == http.MethodPost:
		_, ses := s.sesionDe(r)
		if ses == nil || !ses.local || !csrfValido(r, ses) || !s.esLocal(r) {
			api.NoAutorizado(w, r)
			return
		}
		var peticion struct {
			Agente string `json:"agente"`
			Modo   string `json:"modo"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if err := s.cambiarAutenticacion(peticion.Agente, peticion.Modo); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case r.URL.Path == "/ui/servicio/puerto" && r.Method == http.MethodPost:
		_, ses := s.sesionDe(r)
		if ses == nil || !ses.local || !csrfValido(r, ses) || !s.esLocal(r) {
			api.NoAutorizado(w, r)
			return
		}
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
		s.responderSesion(w, r, ses)
	case r.URL.Path == "/ui/servicio/reiniciar" && r.Method == http.MethodPost:
		_, ses := s.sesionDe(r)
		if ses == nil || !ses.local || !csrfValido(r, ses) || !s.esLocal(r) {
			api.NoAutorizado(w, r)
			return
		}
		if s.opc.Reiniciar == nil {
			api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("este servicio no se puede reiniciar desde la Web UI")))
			return
		}
		if err := s.opc.Reiniciar(); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		s.mu.RLock()
		puerto := cmp.Or(s.opc.PuertoConfigurado, s.opc.Puerto)
		s.mu.RUnlock()
		// El servicio nuevo espera a que este termine y escucha en el puerto configurado (o en otro libre).
		api.EscribirJSON(w, http.StatusAccepted, map[string]any{"url": "http://" + net.JoinHostPort(s.opc.Host, strconv.Itoa(puerto))})
		s.detenerUna.Do(func() { close(s.detener) })
	case r.URL.Path == "/ui/servicio/detener" && r.Method == http.MethodPost:
		// Solo desde este equipo: detener corta también a las aplicaciones conectadas.
		_, ses := s.sesionDe(r)
		if ses == nil || !ses.local || !csrfValido(r, ses) || !s.esLocal(r) {
			api.NoAutorizado(w, r)
			return
		}
		api.EscribirJSON(w, http.StatusAccepted, map[string]any{"estado": "deteniendo"})
		s.detenerUna.Do(func() { close(s.detener) })
	case r.URL.Path == "/ui/actualizacion" && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		// Consultar es de cualquier sesión (?ahora=1 va a GitHub en el momento); activar o desactivar la búsqueda,
		// solo desde este equipo.
		_, ses := s.sesionDe(r)
		if ses == nil || (r.Method == http.MethodPost && (!ses.local || !csrfValido(r, ses) || !s.esLocal(r))) {
			api.NoAutorizado(w, r)
			return
		}
		if r.Method == http.MethodPost {
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
	case r.URL.Path == "/ui/actualizar" && r.Method == http.MethodPost:
		_, ses := s.sesionDe(r)
		if ses == nil || !ses.local || !csrfValido(r, ses) || !s.esLocal(r) {
			api.NoAutorizado(w, r)
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancelar()
		v, err := s.IniciarActualizacion(ctx)
		if err != nil {
			api.EscribirError(w, r, err)
			return
		}
		api.EscribirJSON(w, http.StatusAccepted, map[string]any{"version": v.Etiqueta})
	case r.URL.Path == "/ui/actualizacion/progreso" && r.Method == http.MethodGet:
		if _, ses := s.sesionDe(r); ses == nil {
			api.NoAutorizado(w, r)
			return
		}
		api.EscribirJSON(w, http.StatusOK, s.progresoActualizacion())
	case r.URL.Path == "/ui/sesion" && r.Method == http.MethodDelete:
		if id, _ := s.sesionDe(r); id != "" {
			s.muSesiones.Lock()
			delete(s.sesiones, id)
			s.muSesiones.Unlock()
		}
		http.SetCookie(w, &http.Cookie{Name: cookieSesion, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		w.WriteHeader(http.StatusNoContent)
	case r.URL.Path == "/ui/entrar" && r.Method == http.MethodGet:
		codigo := r.URL.Query().Get("codigo")
		s.muSesiones.Lock()
		entrada, existe := s.codigos[codigo]
		delete(s.codigos, codigo)
		s.muSesiones.Unlock()
		if !existe || time.Now().After(entrada.vence) {
			http.Redirect(w, r, "/?error=codigo", http.StatusSeeOther)
			return
		}
		s.crearSesion(w, entrada.instancia, s.esLocal(r)) // con tramtai abrir en este equipo, la sesión es local
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("ruta no encontrada: %s %s", r.Method, r.URL.Path)))
	}
}

// instanciaResumida es lo que el selector de instancias de la Web UI necesita.
type instanciaResumida struct {
	ID     string `json:"id"`
	Ambito string `json:"ambito"`
	Raiz   string `json:"raiz,omitempty"`
	Nombre string `json:"nombre"`
}

func resumir(inst *Instancia) instanciaResumida {
	nombre := "Global"
	if inst.Ambito == "proyecto" {
		nombre = filepath.Base(inst.Raiz)
	}
	return instanciaResumida{ID: inst.ID, Ambito: inst.Ambito, Raiz: inst.Raiz, Nombre: nombre}
}

func (s *Servicio) instancia(id string) *Instancia {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instancias[id]
}

// instanciaPredeterminada es la global si está activa; si no, la primera por nombre ("" si no hay ninguna).
func (s *Servicio) instanciaPredeterminada() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.instancias["global"] != nil {
		return "global"
	}
	ids := make([]string, 0, len(s.instancias))
	for id := range s.instancias {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	if len(ids) == 0 {
		return ""
	}
	return ids[0]
}

// responderSesion devuelve la sesión. Una sesión local ve además todas las instancias activas para cambiar entre
// ellas; si no hay ninguna, la Web UI ofrece activar la global.
func (s *Servicio) responderSesion(w http.ResponseWriter, r *http.Request, ses *sesion) {
	respuesta := map[string]any{"csrf": ses.csrf, "version": version.Version, "local": ses.local, "instancia": ""}
	if ses.local {
		s.mu.RLock()
		lista := []instanciaResumida{}
		for _, inst := range s.instancias {
			lista = append(lista, resumir(inst))
		}
		s.mu.RUnlock()
		slices.SortFunc(lista, func(a, b instanciaResumida) int {
			if a.Ambito != b.Ambito {
				return strings.Compare(a.Ambito, b.Ambito) // global antes que proyecto
			}
			return strings.Compare(a.Nombre, b.Nombre)
		})
		respuesta["instancias"] = lista
		s.mu.RLock()
		respuesta["servicio"] = map[string]any{"puerto": s.opc.Puerto, "puerto_configurado": cmp.Or(s.opc.PuertoConfigurado, s.opc.Puerto),
			"puerto_automatico": s.opc.PuertoAutomatico, "reiniciable": s.opc.Reiniciar != nil}
		s.mu.RUnlock()
		if autoarranque.Disponible {
			respuesta["autoarranque"], _ = autoarranque.Activo()
		}
	}
	inst := s.instancia(ses.instancia)
	if inst == nil && !ses.local {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("la instancia de la sesión (%s) no está activa", ses.instancia)))
		return
	}
	if inst != nil {
		datos := resumir(inst)
		respuesta["instancia"], respuesta["ambito"], respuesta["raiz"], respuesta["nombre"] = datos.ID, datos.Ambito, datos.Raiz, datos.Nombre
	}
	api.EscribirJSON(w, http.StatusOK, respuesta)
}

// esSitio indica una petición de navegación o de archivos de la Web UI, que no exige token.
func (s *Servicio) esSitio(r *http.Request) bool {
	if s.opc.Web == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return false
	}
	for _, prefijo := range []string{"/api/", "/openai/", "/anthropic/"} {
		if strings.HasPrefix(r.URL.Path, prefijo) {
			return false
		}
	}
	return true
}

// sitio sirve los archivos compilados de la Web UI y, para cualquier otra ruta de navegación, index.html.
func (s *Servicio) sitio(w http.ResponseWriter, r *http.Request) {
	ruta := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if ruta == "" {
		ruta = "index.html"
	}
	if info, err := fs.Stat(s.opc.Web, ruta); err != nil || info.IsDir() {
		ruta = "index.html"
	}
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	// connect-src admite otros puertos de este equipo para seguir al servicio si un reinicio le cambia el puerto.
	h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; "+
		"connect-src 'self' http://127.0.0.1:* http://localhost:*; frame-ancestors 'none'")
	if strings.HasPrefix(ruta, "assets/") {
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "no-cache")
	}
	http.ServeFileFS(w, r, s.opc.Web, ruta)
}

// estadoConBase es el estado de la instancia con la URL del servicio, para el panel de la Web UI.
type estadoConBase struct {
	EstadoInstancia
	Base string `json:"base"`
}

// rutaInstancia atiende lo que la Web UI necesita además de la API de la instancia: su estado y, con una sesión
// de la Web UI o el token de servicio (no con el token que usan las aplicaciones), su configuración efectiva y sus
// puntos de acceso. Devuelve false si la ruta no es suya.
func (s *Servicio) rutaInstancia(w http.ResponseWriter, r *http.Request, inst *Instancia, privilegiada bool) bool {
	ruta := r.URL.Path
	if ruta == "/api/v1/instancia" && r.Method == http.MethodGet {
		api.EscribirJSON(w, http.StatusOK, estadoConBase{inst.estado(r.Context()), s.Base()})
		return true
	}
	if !strings.HasPrefix(ruta, "/api/v1/instancia/") && ruta != "/api/v1/puntos" && !strings.HasPrefix(ruta, "/api/v1/puntos/") {
		return false
	}
	if !privilegiada {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoAutorizado, idioma.T("requiere una sesión de la Web UI o el token de servicio")))
		return true
	}
	switch {
	case ruta == "/api/v1/instancia/token" && r.Method == http.MethodGet:
		// Para mostrar en «Conectar» la clave que usan las aplicaciones; solo con sesión o token de servicio.
		api.EscribirJSON(w, http.StatusOK, map[string]any{"token": inst.token, "archivo": inst.ArchivoToken})
	case ruta == "/api/v1/instancia/configuracion" && r.Method == http.MethodGet:
		entradas := []EntradaConfiguracion{}
		for _, e := range inst.Config.Efectiva() {
			entradas = append(entradas, EntradaConfiguracion{Clave: e.Clave, Valor: e.Valor, Origen: e.Origen.String()})
		}
		api.EscribirJSON(w, http.StatusOK, map[string]any{"configuracion": entradas})
	case ruta == "/api/v1/instancia/predeterminado" && r.Method == http.MethodPost:
		var peticion struct {
			Modelo string `json:"modelo"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		if err := s.fijarPredeterminado(inst, peticion.Modelo); err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
	case ruta == "/api/v1/puntos" && r.Method == http.MethodGet:
		uso, _ := inst.Planificador.UsoPorPunto(r.Context()) // sin uso, la lista se muestra igual
		api.EscribirJSON(w, http.StatusOK, map[string]any{"puntos_acceso": s.puntosDe(inst.ID), "uso": uso})
	case ruta == "/api/v1/puntos" && r.Method == http.MethodPost:
		s.crearPuntoUI(w, r, inst)
	case strings.HasPrefix(ruta, "/api/v1/puntos/"):
		s.puntoUI(w, r, inst, strings.TrimPrefix(ruta, "/api/v1/puntos/"))
	default:
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("ruta no encontrada: %s %s", r.Method, ruta)))
	}
	return true
}

func (s *Servicio) puntosDe(instancia string) []EstadoPunto {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lista := []EstadoPunto{}
	for _, p := range s.estadoPuntos() {
		if p.Instancia == instancia {
			lista = append(lista, p)
		}
	}
	return lista
}

// archivoPuntos es donde se guardan los puntos nuevos de la instancia: la configuración global para la global y
// .tramtai/local.yaml para un proyecto (nunca la versionada).
func archivoPuntos(inst *Instancia) (string, nucleo.Origen, error) {
	if inst.Ambito == "global" {
		ruta, err := nucleo.RutaGlobal()
		return ruta, nucleo.OrigenGlobal, err
	}
	return filepath.Join(inst.Raiz, ".tramtai", "local.yaml"), nucleo.OrigenLocal, nil
}

func origenDeArchivo(ruta string) nucleo.Origen {
	if strings.HasSuffix(filepath.ToSlash(ruta), ".tramtai/local.yaml") {
		return nucleo.OrigenLocal
	}
	return nucleo.OrigenGlobal
}

func (s *Servicio) crearPuntoUI(w http.ResponseWriter, r *http.Request, inst *Instancia) {
	var p ConfigPunto
	if err := api.LeerJSON(w, r, &p); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	ruta, origen, err := archivoPuntos(inst)
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	p.Instancia, p.Activo = "", nil
	if origen == nucleo.OrigenGlobal && inst.ID != "global" {
		p.Instancia = inst.ID
	}
	if _, err := p.Normalizar(inst.ID, s.opc.Puerto); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if err := s.validarRespaldo(p); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	err = EditarPuntos(ruta, origen, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		if slices.ContainsFunc(lista, func(c ConfigPunto) bool { return c.Nombre == p.Nombre }) {
			return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("el punto de acceso %s ya existe", p.Nombre))
		}
		return append(lista, p), nil
	})
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.Recargar()
	_ = s.actualizarDescriptor()
	token, err := LeerOCrearToken(RutaTokenPunto(s.opc.Datos, p.Nombre))
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	for _, e := range s.puntosDe(inst.ID) {
		if e.Nombre == p.Nombre {
			api.EscribirJSON(w, http.StatusCreated, map[string]any{"punto_acceso": e, "token": token})
			return
		}
	}
	api.EscribirJSON(w, http.StatusCreated, map[string]any{"token": token})
}

// puntoUI cambia los ajustes (PATCH: activo, instrucciones, esfuerzo y modelo), elimina (DELETE) o muestra el
// token (GET <nombre>/token) de un punto de la instancia de la sesión.
func (s *Servicio) puntoUI(w http.ResponseWriter, r *http.Request, inst *Instancia, resto string) {
	nombre, accion, _ := strings.Cut(resto, "/")
	var punto *EstadoPunto
	for _, p := range s.puntosDe(inst.ID) {
		if p.Nombre == nombre {
			punto = &p
			break
		}
	}
	if punto == nil {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("punto de acceso no encontrado en esta instancia: %s", nombre)))
		return
	}
	editar := func(f func([]ConfigPunto) ([]ConfigPunto, error)) bool {
		if err := EditarPuntos(punto.Origen, origenDeArchivo(punto.Origen), f); err != nil {
			api.EscribirError(w, r, err)
			return false
		}
		s.Recargar()
		_ = s.actualizarDescriptor()
		return true
	}
	switch {
	case accion == "token" && r.Method == http.MethodGet:
		token, err := LeerOCrearToken(RutaTokenPunto(s.opc.Datos, nombre))
		if err != nil {
			api.EscribirError(w, r, err)
			return
		}
		api.EscribirJSON(w, http.StatusOK, map[string]any{"token": token})
	case accion == "" && r.Method == http.MethodPatch:
		var cambio struct { // solo cambia lo que viene en el cuerpo
			Activo        *bool     `json:"activo"`
			Instrucciones *string   `json:"instrucciones"`
			Esfuerzo      *string   `json:"esfuerzo"` // "" vuelve al del agente
			Modelo        *string   `json:"modelo"`   // identificador del catálogo; "" vuelve al del espacio
			Forzar        *bool     `json:"forzar_modelo"`
			Permitidos    *[]string `json:"modelos_permitidos"`
			// Esquema: ausente no cambia; null o {} lo quita; un objeto lo fija.
			Esquema  json.RawMessage `json:"esquema"`
			Respaldo *[]string       `json:"respaldo"`
		}
		if err := api.LeerJSON(w, r, &cambio); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if cambio.Esfuerzo != nil && *cambio.Esfuerzo != "" && !slices.Contains(nucleo.NivelesEsfuerzo, *cambio.Esfuerzo) {
			api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("esfuerzo desconocido: %s", *cambio.Esfuerzo)))
			return
		}
		if editar(func(lista []ConfigPunto) ([]ConfigPunto, error) {
			for i := range lista {
				if lista[i].Nombre != nombre {
					continue
				}
				if cambio.Activo != nil {
					lista[i].Activo = cambio.Activo
				}
				if cambio.Instrucciones != nil {
					lista[i].Instrucciones = strings.TrimSpace(*cambio.Instrucciones)
				}
				if cambio.Esfuerzo != nil {
					lista[i].Esfuerzo = *cambio.Esfuerzo
				}
				if cambio.Modelo != nil {
					if err := s.modeloDePunto(inst, &lista[i], strings.TrimSpace(*cambio.Modelo)); err != nil {
						return nil, err
					}
				}
				if cambio.Forzar != nil {
					lista[i].ForzarModelo = *cambio.Forzar
				}
				if cambio.Permitidos != nil {
					lista[i].ModelosPermitidos = slices.DeleteFunc(*cambio.Permitidos, func(m string) bool { return strings.TrimSpace(m) == "" })
				}
				if len(cambio.Esquema) > 0 {
					lista[i].Esquema = nil // null y {} lo quitan
					if err := json.Unmarshal(cambio.Esquema, &lista[i].Esquema); err != nil {
						return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("el esquema debe ser un objeto JSON"))
					}
				}
				if cambio.Respaldo != nil {
					lista[i].Respaldo = *cambio.Respaldo
				}
				// La receta se valida antes de escribir: un cambio no válido deja la configuración como estaba. Solo si
				// cambia, para no impedir desactivar un punto que ya tenía otro error.
				if len(cambio.Esquema) > 0 || cambio.Respaldo != nil || cambio.Permitidos != nil {
					if _, err := lista[i].Normalizar(inst.ID, s.opc.Puerto); err != nil {
						return nil, err
					}
					if err := s.validarRespaldo(lista[i]); err != nil {
						return nil, err
					}
				}
			}
			return lista, nil
		}) {
			api.EscribirJSON(w, http.StatusOK, map[string]any{"puntos_acceso": s.puntosDe(inst.ID)})
		}
	case accion == "" && r.Method == http.MethodDelete:
		if editar(func(lista []ConfigPunto) ([]ConfigPunto, error) {
			return slices.DeleteFunc(lista, func(c ConfigPunto) bool { return c.Nombre == nombre }), nil
		}) {
			w.WriteHeader(http.StatusNoContent)
		}
	default:
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("ruta no encontrada: %s %s", r.Method, r.URL.Path)))
	}
}

// modeloDePunto fija el modelo de un punto desde un identificador del catálogo (agente, agente/modelo o
// perfil/nombre). "" lo quita para que valga el del espacio y deja de forzarlo, porque ya no hay qué forzar.
func (s *Servicio) modeloDePunto(inst *Instancia, c *ConfigPunto, id string) error {
	c.Agente, c.Modelo, c.PerfilModelo = "", "", ""
	agente, modelo, _ := strings.Cut(id, "/")
	switch {
	case id == "":
		c.ForzarModelo = false
	case agente == "perfil":
		if _, existe := inst.Config.Perfiles()[modelo]; !existe {
			return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("perfil de modelo desconocido: %s", modelo))
		}
		c.PerfilModelo = modelo
	case s.opc.Registro[agente] == nil:
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("agente desconocido: %s", agente))
	default:
		c.Agente, c.Modelo = agente, modelo
	}
	return nil
}

// csrfValido compara en tiempo constante la cabecera X-CSRF-Token con la de la sesión.
func csrfValido(r *http.Request, ses *sesion) bool {
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(ses.csrf)) == 1
}

// cambiarAutenticacion pasa claude o codex a suscripción o clave de API: lo guarda en la configuración global y
// lo aplica al servicio en marcha, sin reiniciarlo.
func (s *Servicio) cambiarAutenticacion(agente, modo string) error {
	if agente != "claude" && agente != "codex" {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("solo claude y codex admiten la suscripción"))
	}
	if modo != adaptadores.AutenticacionClaveAPI && modo != adaptadores.AutenticacionSuscripcion {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("modo de autenticación desconocido: %s", modo))
	}
	if ip := net.ParseIP(s.opc.Host); modo == adaptadores.AutenticacionSuscripcion && s.opc.Host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.T("la suscripción no se permite con el servicio abierto a otros equipos"))
	}
	ruta, err := nucleo.RutaGlobal()
	if err != nil {
		return err
	}
	if err := FijarValor(ruta, nucleo.OrigenGlobal, "adaptadores."+agente+".autenticacion", modo); err != nil {
		return err
	}
	return adaptadores.CambiarAutenticacion(s.opc.Registro, agente, modo)
}

// fijarPredeterminado guarda el modelo que se usa cuando la solicitud no indica agente ni modelo (un identificador
// del catálogo: agente, agente/modelo o perfil/nombre) y lo aplica en marcha. En la instancia global va a la
// configuración global y alcanza también a los proyectos que no tengan el suyo; en un proyecto, a su local.yaml.
func (s *Servicio) fijarPredeterminado(inst *Instancia, id string) error {
	valores := map[string]any{"predeterminados.modelo": "", "predeterminados.perfil_modelo": ""}
	agente, modelo, _ := strings.Cut(id, "/")
	switch {
	case agente == "perfil":
		if _, existe := inst.Config.Perfiles()[modelo]; !existe {
			return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("perfil de modelo desconocido: %s", modelo))
		}
		valores["predeterminados.perfil_modelo"] = modelo
	case s.opc.Registro[agente] == nil:
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("agente desconocido: %s", agente))
	default:
		valores["predeterminados.agente"], valores["predeterminados.modelo"] = agente, modelo
	}
	ruta, origen, err := archivoPuntos(inst)
	if err != nil {
		return err
	}
	for clave, valor := range valores {
		if err := FijarValor(ruta, origen, clave, valor); err != nil {
			return err
		}
	}
	if inst.Ambito != "global" {
		inst.Config.Fijar(origen, valores)
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, otra := range s.instancias {
		otra.Config.Fijar(origen, valores)
	}
	return nil
}

// fijarPuerto guarda servicio.puerto y servicio.puerto_automatico en la configuración global si el puerto está
// libre (o es el que ya usa este servicio). Se aplica al reiniciar.
func (s *Servicio) fijarPuerto(puerto int, automatico bool) error {
	if puerto < 1024 || puerto > 65535 {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("el puerto debe estar entre 1024 y 65535"))
	}
	if puerto != s.opc.Puerto && !PuertoLibre(s.opc.Host, puerto) {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("el puerto %d está ocupado por otro programa: elige otro", puerto))
	}
	ruta, err := nucleo.RutaGlobal()
	if err != nil {
		return err
	}
	if err := FijarValor(ruta, nucleo.OrigenGlobal, "servicio.puerto", puerto); err != nil {
		return err
	}
	if err := FijarValor(ruta, nucleo.OrigenGlobal, "servicio.puerto_automatico", automatico); err != nil {
		return err
	}
	s.mu.Lock()
	s.opc.PuertoConfigurado, s.opc.PuertoAutomatico = puerto, automatico
	s.mu.Unlock()
	return nil
}
