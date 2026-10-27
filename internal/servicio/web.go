package servicio

import (
	"crypto/subtle"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"tramtai/internal/api"
	"tramtai/internal/credenciales"
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
	// local: entró sin token por ser del mismo usuario del sistema (esLocal) o con un código canjeado en este
	// equipo (porCodigo); puede cambiar de instancia y usar los controles del servicio.
	local     bool
	porCodigo bool
	// generacion es la del servicio al crearla: si el servicio la sube (cambiar la contraseña, «Cerrar todas las
	// sesiones»), la sesión deja de valer.
	generacion uint64
	// totpPendiente es el secreto TOTP que Ajustes → Seguridad enseñó y aún no se confirmó con un código.
	totpPendiente string
}

// derivarSesion crea la sesión local de otra instancia a partir de la actual, conservando cómo se entró.
func (s *Servicio) derivarSesion(w http.ResponseWriter, r *http.Request, instancia string, actual *sesion) *sesion {
	nueva := s.crearSesion(w, r, instancia, true)
	nueva.porCodigo = actual.porCodigo
	return nueva
}

// sesionLocal indica que la sesión es local y que la petición sigue llegando desde este equipo: del mismo usuario
// (esLocal) o, en una sesión abierta con código, desde loopback sin proxy (cuando no se puede comprobar el usuario).
func (s *Servicio) sesionLocal(r *http.Request, ses *sesion) bool {
	return ses != nil && ses.local && (s.esLocal(r) || (ses.porCodigo && desdeEsteEquipo(r)))
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

// nombreCookie lleva el puerto: las cookies valen para todo 127.0.0.1, sin distinguir puerto, y dos Tramtai a la vez
// (la instalada y una portable) en el mismo navegador se echarían la una a la otra. Por HTTPS lleva además el prefijo
// __Host-: el navegador solo la acepta con Secure, Path=/ y sin Domain, así que ni un subdominio ni HTTP plano la pisan.
func (s *Servicio) nombreCookie(r *http.Request) string {
	nombre := cookieSesion + "_" + strconv.Itoa(s.opc.Puerto)
	if EsHTTPS(r, s.opc.Confiables) {
		return "__Host-" + nombre
	}
	return nombre
}

func (s *Servicio) sesionDe(r *http.Request) (string, *sesion) {
	cookie, err := r.Cookie(s.nombreCookie(r))
	if err != nil {
		return "", nil
	}
	linea := leerAdministrador(s.opc.Datos)
	s.muSesiones.Lock()
	defer s.muSesiones.Unlock()
	s.vigilarAdministrador(linea)
	ses := s.sesiones[cookie.Value]
	if ses == nil || time.Now().After(ses.vence) || ses.generacion != s.generacion {
		delete(s.sesiones, cookie.Value)
		return "", nil
	}
	ses.vence = time.Now().Add(duracionSesion)
	return cookie.Value, ses
}

// vigilarAdministrador sube la generación si la contraseña del administrador cambió desde la última vez que se miró,
// también desde la CLI (que escribe datos/administrador directamente): cambiarla cierra todas las sesiones, también las
// abiertas con código. Se llama con muSesiones tomado.
func (s *Servicio) vigilarAdministrador(linea string) {
	if linea != s.lineaAdministrador {
		s.lineaAdministrador = linea
		s.generacion++
	}
}

func (s *Servicio) crearSesion(w http.ResponseWriter, r *http.Request, instancia string, local bool) *sesion {
	id := aleatorio(32)
	ses := &sesion{instancia: instancia, csrf: aleatorio(16), vence: time.Now().Add(duracionSesion), local: local}
	linea := leerAdministrador(s.opc.Datos)
	s.muSesiones.Lock()
	s.vigilarAdministrador(linea)
	for clave, otra := range s.sesiones {
		if time.Now().After(otra.vence) || otra.generacion != s.generacion {
			delete(s.sesiones, clave)
		}
	}
	ses.generacion = s.generacion
	s.sesiones[id] = ses
	s.muSesiones.Unlock()
	if instancia != "" {
		s.marcarUso(instancia) // una sesión nueva vuelve al último espacio usado
	}
	http.SetCookie(w, &http.Cookie{Name: s.nombreCookie(r), Value: id, Path: "/", HttpOnly: true, Secure: EsHTTPS(r, s.opc.Confiables),
		SameSite: http.SameSiteStrictMode, MaxAge: int(duracionSesion.Seconds())})
	return ses
}

// cerrarSesiones cierra todas las sesiones de la Web UI; las locales se vuelven a abrir solas desde este equipo.
func (s *Servicio) cerrarSesiones() {
	s.muSesiones.Lock()
	s.generacion++
	s.muSesiones.Unlock()
}

// CodigoSesion crea un código de un solo uso, válido un minuto, que abre la Web UI de una instancia sin escribir
// su token. Lo piden la CLI, la terminal interactiva y la aplicación de escritorio con el token de servicio; el
// token de la instancia nunca viaja en la URL.
func (s *Servicio) CodigoSesion(instancia string) (string, error) {
	s.mu.RLock()
	_, activa := s.instancias[instancia]
	s.mu.RUnlock()
	if !activa {
		return "", nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("el espacio %s no está activo", instancia))
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
		nombre = nombreProyecto(inst.Raiz, inst.Config)
	}
	return instanciaResumida{ID: inst.ID, Ambito: inst.Ambito, Raiz: inst.Raiz, Nombre: nombre}
}

func (s *Servicio) instancia(id string) *Instancia {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.instancias[id]
}

// instanciaPredeterminada es el espacio activo que se usó por última vez; si ninguno se ha usado, la global si está
// activa; si no, el primero por nombre ("" si no hay ninguno).
func (s *Servicio) instanciaPredeterminada() string {
	s.mu.RLock()
	ids := make([]string, 0, len(s.instancias))
	for id := range s.instancias {
		ids = append(ids, id)
	}
	s.mu.RUnlock()
	slices.Sort(ids)
	elegido, ultimo := "", time.Time{}
	for _, id := range ids {
		if e, ok := s.registro().buscar(id); ok && e.UltimoUso.After(ultimo) {
			elegido, ultimo = id, e.UltimoUso
		}
	}
	switch {
	case elegido != "":
		return elegido
	case slices.Contains(ids, "global"):
		return "global"
	case len(ids) > 0:
		return ids[0]
	}
	return ""
}

// responderSesion devuelve la sesión. Una sesión local ve además todas las instancias activas para cambiar entre
// ellas; si no hay ninguna, la Web UI ofrece activar la global.
func (s *Servicio) responderSesion(w http.ResponseWriter, r *http.Request, ses *sesion) {
	respuesta := map[string]any{"csrf": ses.csrf, "version": version.Version, "local": ses.local, "instancia": "",
		// almacen: dónde guarda este sistema las claves de proveedor (cifradas, fuera de Tramtai), para Agentes.
		"almacen": map[string]any{"disponible": credenciales.Disponible(), "nombre": credenciales.Almacen()}}
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
		// Los mismos que Ajustes en la terminal; la sesión lleva el arranque fuera de servicio.
		ajustes := s.ajustesServicio()
		if ajustes.Autoarranque != nil {
			respuesta["autoarranque"], respuesta["modo_arranque"] = *ajustes.Autoarranque, ajustes.ModoArranque
		}
		ajustes.Autoarranque, ajustes.ModoArranque = nil, ""
		respuesta["servicio"] = ajustes
	}
	inst := s.instancia(ses.instancia)
	if inst == nil && !ses.local {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("el espacio de la sesión (%s) no está activo", ses.instancia)))
		return
	}
	if inst != nil {
		datos := resumir(inst)
		respuesta["instancia"], respuesta["ambito"], respuesta["raiz"], respuesta["nombre"] = datos.ID, datos.Ambito, datos.Raiz, datos.Nombre
	}
	api.EscribirJSON(w, http.StatusOK, respuesta)
}

// csrfValido compara en tiempo constante la cabecera X-CSRF-Token con la de la sesión.
func csrfValido(r *http.Request, ses *sesion) bool {
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(ses.csrf)) == 1
}

// permisoUI es lo que exige una ruta de la Web UI antes de atenderla.
type permisoUI int

const (
	publica          permisoUI = iota // la propia ruta decide (entrar, salir y consultar la sesión)
	conSesion                         // cualquier sesión
	conSesionYCSRF                    // cualquier sesión, con su CSRF (sin él, 403 csrf_invalido)
	soloLocal                         // una sesión local, desde este equipo y con su CSRF
	soloLocalLectura                  // una sesión local, para consultar (sin CSRF: no cambia nada)
)

// rutaUI es una ruta de /ui/: método, ruta exacta, permiso y quién la atiende (ses es nil en las públicas).
type rutaUI struct {
	metodo, ruta string
	permiso      permisoUI
	atender      func(s *Servicio, w http.ResponseWriter, r *http.Request, ses *sesion)
}

// rutasDeUI son las rutas de la sesión y de los controles del servicio. GET /ui/sesion la consulta (y, si la petición
// es local, entra sin token), POST cambia de instancia en una sesión local, DELETE sale y /ui/entrar canjea un código
// de un solo uso. Lo que cambia el servicio o el equipo (instancias, proyectos, arranque, puerto, reinicio,
// actualizaciones) solo se hace desde una sesión local.
var rutasDeUI = []rutaUI{
	{http.MethodGet, "/ui/sesion", publica, (*Servicio).consultarSesionUI},
	{http.MethodPost, "/ui/sesion", publica, (*Servicio).cambiarSesionUI},
	{http.MethodDelete, "/ui/sesion", publica, (*Servicio).salirUI},
	{http.MethodGet, "/ui/entrar", publica, (*Servicio).entrarUI},
	{http.MethodPost, "/ui/instancias", soloLocal, (*Servicio).activarGlobalUI},
	{http.MethodPost, "/ui/proyectos", soloLocal, (*Servicio).abrirProyectoUI},
	// Espacios: la lista, y activar, detener, confiar, reasociar u olvidar uno (el id va en el cuerpo).
	{http.MethodGet, "/ui/espacios", soloLocalLectura, (*Servicio).listarEspaciosUI},
	{http.MethodPost, "/ui/espacios/activar", soloLocal, (*Servicio).accionEspacioUI},
	{http.MethodPost, "/ui/espacios/detener", soloLocal, (*Servicio).accionEspacioUI},
	{http.MethodPost, "/ui/espacios/confiar", soloLocal, (*Servicio).accionEspacioUI},
	{http.MethodPost, "/ui/espacios/ubicar", soloLocal, (*Servicio).accionEspacioUI},
	{http.MethodPost, "/ui/espacios/olvidar", soloLocal, (*Servicio).accionEspacioUI},
	{http.MethodPost, "/ui/autoarranque", soloLocal, (*Servicio).autoarranqueUI},
	{http.MethodPost, "/ui/autenticacion", soloLocal, (*Servicio).autenticacionUI},
	{http.MethodPost, "/ui/agentes/conexion", soloLocal, (*Servicio).conexionUI},
	{http.MethodPost, "/ui/servicio/puerto", soloLocal, (*Servicio).puertoUI},
	{http.MethodPost, "/ui/servicio/reiniciar", soloLocal, (*Servicio).reiniciarUI},
	// Solo desde este equipo: detener corta también a las aplicaciones conectadas.
	{http.MethodPost, "/ui/servicio/detener", soloLocal, (*Servicio).detenerUI},
	// Consultar es de cualquier sesión (?ahora=1 va a GitHub en el momento); activar o desactivar la búsqueda, solo
	// desde este equipo.
	{http.MethodGet, "/ui/actualizacion", conSesion, (*Servicio).actualizacionUI},
	{http.MethodPost, "/ui/actualizacion", soloLocal, (*Servicio).actualizacionUI},
	{http.MethodPost, "/ui/actualizar", soloLocal, (*Servicio).actualizarUI},
	{http.MethodPost, "/ui/actualizar/instalar", soloLocal, (*Servicio).decidirInstalacionUI},
	{http.MethodPost, "/ui/actualizar/cancelar", soloLocal, (*Servicio).decidirInstalacionUI},
	{http.MethodGet, "/ui/acerca", conSesion, (*Servicio).acercaUI},
	// Los términos de uso: los ve y los acepta cualquier sesión (el POST con su CSRF); lo aceptado queda en datos/terminos.json.
	{http.MethodGet, "/ui/terminos", conSesion, (*Servicio).terminosUI},
	{http.MethodPost, "/ui/terminos", conSesionYCSRF, (*Servicio).aceptarTerminosUI},
	{http.MethodGet, "/ui/actualizacion/progreso", conSesion, (*Servicio).progresoUI},
	{http.MethodGet, "/ui/actualizacion/novedades", conSesion, (*Servicio).novedadesUI},
	// El administrador del servidor (la Web UI remota): entrar con contraseña es público y lo frena frenos.go; Ajustes →
	// Seguridad es de cualquier sesión, también la remota, con su CSRF.
	{http.MethodGet, "/ui/entrada", publica, (*Servicio).entradaUI},
	{http.MethodPost, "/ui/acceder", publica, (*Servicio).accederUI},
	{http.MethodPost, "/ui/administrador/contrasena", conSesionYCSRF, (*Servicio).contrasenaUI},
	{http.MethodPost, "/ui/administrador/totp", conSesionYCSRF, (*Servicio).activarTOTPUI},
	{http.MethodDelete, "/ui/administrador/totp", conSesionYCSRF, (*Servicio).quitarTOTPUI},
	{http.MethodPost, "/ui/administrador/recuperacion", conSesionYCSRF, (*Servicio).recuperacionUI},
	{http.MethodPost, "/ui/sesiones/cerrar", conSesionYCSRF, (*Servicio).cerrarSesionesUI},
}

// rutasUI busca la ruta en rutasDeUI, comprueba su permiso y la atiende.
func (s *Servicio) rutasUI(w http.ResponseWriter, r *http.Request) {
	for _, ruta := range rutasDeUI {
		if ruta.metodo != r.Method || ruta.ruta != r.URL.Path {
			continue
		}
		var ses *sesion
		if ruta.permiso != publica {
			_, ses = s.sesionDe(r)
		}
		if (ruta.permiso == conSesion || ruta.permiso == conSesionYCSRF) && ses == nil || ruta.permiso == soloLocal && (!s.sesionLocal(r, ses) || !csrfValido(r, ses)) ||
			ruta.permiso == soloLocalLectura && !s.sesionLocal(r, ses) {
			api.NoAutorizado(w, r)
			return
		}
		if ruta.permiso == conSesionYCSRF && !csrfValido(r, ses) {
			api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoCSRFInvalido, idioma.T("falta la cabecera X-CSRF-Token de la sesión")))
			return
		}
		ruta.atender(s, w, r, ses)
		return
	}
	api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("ruta no encontrada: %s %s", r.Method, r.URL.Path)))
}

func (s *Servicio) consultarSesionUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	_, ses := s.sesionDe(r)
	if ses == nil && s.esLocal(r) {
		ses = s.crearSesion(w, r, s.instanciaPredeterminada(), true)
	}
	if ses == nil {
		api.NoAutorizado(w, r)
		return
	}
	s.responderSesion(w, r, ses)
}

// cambiarSesionUI cambia de instancia en una sesión local; con un token no se entra.
func (s *Servicio) cambiarSesionUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
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
		if !s.sesionLocal(r, ses) || !csrfValido(r, ses) {
			api.NoAutorizado(w, r)
			return
		}
		if s.instancia(peticion.Instancia) == nil {
			api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("el espacio %s no está activo", peticion.Instancia)))
			return
		}
		s.responderSesion(w, r, s.derivarSesion(w, r, peticion.Instancia, ses))
		return
	}
	// La clave de las aplicaciones no abre la Web UI: quien la tuviera podría crear APIs y cambiar la
	// configuración. Se entra en el mismo equipo y usuario o con el código de un solo uso.
	api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoAutorizado,
		idioma.T("la clave de las aplicaciones no abre la Web UI: ábrela desde la app de Tramtai o con «tramtai abrir»")))
}

func (s *Servicio) salirUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if id, _ := s.sesionDe(r); id != "" {
		s.muSesiones.Lock()
		delete(s.sesiones, id)
		s.muSesiones.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: s.nombreCookie(r), Value: "", Path: "/", HttpOnly: true, Secure: EsHTTPS(r, s.opc.Confiables),
		SameSite: http.SameSiteStrictMode, MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

// entrarUI canjea un código de un solo uso por una sesión.
func (s *Servicio) entrarUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	codigo := r.URL.Query().Get("codigo")
	s.muSesiones.Lock()
	entrada, existe := s.codigos[codigo]
	delete(s.codigos, codigo)
	s.muSesiones.Unlock()
	if !existe || time.Now().After(entrada.vence) {
		http.Redirect(w, r, "/?error=codigo", http.StatusSeeOther)
		return
	}
	// Con un código canjeado en este equipo la sesión es local: solo el dueño del servicio puede pedirlo (con el
	// token de servicio). Así funciona también cuando no se puede comprobar el usuario de la conexión.
	ses := s.crearSesion(w, r, entrada.instancia, s.esLocal(r) || desdeEsteEquipo(r))
	ses.porCodigo = true // la cookie aún no ha salido: nadie más usa esta sesión
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
