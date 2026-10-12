package servicio

import (
	"crypto/subtle"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"tramtai/internal/api"
	"tramtai/internal/nucleo"
	"tramtai/internal/version"
)

// sesion es una sesión de la Web UI enlazada a una única instancia (secciones 7.4 y 13.3): cookie HttpOnly con
// SameSite estricto y un token CSRF obligatorio en las peticiones que modifican.
type sesion struct {
	instancia string
	csrf      string
	vence     time.Time
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

func (s *Servicio) crearSesion(w http.ResponseWriter, instancia string) *sesion {
	id := aleatorio(32)
	ses := &sesion{instancia: instancia, csrf: aleatorio(16), vence: time.Now().Add(duracionSesion)}
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
// token de la instancia nunca viaja en la URL (sección 7.4).
func (s *Servicio) CodigoSesion(instancia string) (string, error) {
	s.mu.RLock()
	_, activa := s.instancias[instancia]
	s.mu.RUnlock()
	if !activa {
		return "", nucleo.NuevoError(nucleo.CodigoNoEncontrado, "la instancia "+instancia+" no está activa")
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

// rutasUI gestiona la sesión: GET la consulta, POST entra con el token de una instancia, DELETE sale y
// /ui/entrar canjea un código de un solo uso.
func (s *Servicio) rutasUI(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/ui/sesion" && r.Method == http.MethodGet:
		_, ses := s.sesionDe(r)
		if ses == nil {
			api.NoAutorizado(w, r)
			return
		}
		s.responderSesion(w, r, ses)
	case r.URL.Path == "/ui/sesion" && r.Method == http.MethodPost:
		var peticion struct {
			Token string `json:"token"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		inst := s.instanciaPorToken(strings.TrimSpace(peticion.Token))
		if inst == nil {
			api.NoAutorizado(w, r)
			return
		}
		s.responderSesion(w, r, s.crearSesion(w, inst.ID))
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
		s.crearSesion(w, entrada.instancia)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, "ruta no encontrada: "+r.Method+" "+r.URL.Path))
	}
}

func (s *Servicio) responderSesion(w http.ResponseWriter, r *http.Request, ses *sesion) {
	s.mu.RLock()
	inst := s.instancias[ses.instancia]
	s.mu.RUnlock()
	if inst == nil {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, "la instancia de la sesión ("+ses.instancia+") no está activa"))
		return
	}
	api.EscribirJSON(w, http.StatusOK, map[string]any{"instancia": inst.ID, "ambito": inst.Ambito, "raiz": inst.Raiz,
		"csrf": ses.csrf, "version": version.Texto()})
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
	h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
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
		api.EscribirJSON(w, http.StatusOK, estadoConBase{inst.estado(), s.Base()})
		return true
	}
	if !strings.HasPrefix(ruta, "/api/v1/instancia/") && ruta != "/api/v1/puntos" && !strings.HasPrefix(ruta, "/api/v1/puntos/") {
		return false
	}
	if !privilegiada {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoAutorizado, "requiere una sesión de la Web UI o el token de servicio"))
		return true
	}
	switch {
	case ruta == "/api/v1/instancia/configuracion" && r.Method == http.MethodGet:
		entradas := []EntradaConfiguracion{}
		for _, e := range inst.Config.Efectiva() {
			entradas = append(entradas, EntradaConfiguracion{Clave: e.Clave, Valor: e.Valor, Origen: e.Origen.String()})
		}
		api.EscribirJSON(w, http.StatusOK, map[string]any{"configuracion": entradas})
	case ruta == "/api/v1/puntos" && r.Method == http.MethodGet:
		api.EscribirJSON(w, http.StatusOK, map[string]any{"puntos_acceso": s.puntosDe(inst.ID)})
	case ruta == "/api/v1/puntos" && r.Method == http.MethodPost:
		s.crearPuntoUI(w, r, inst)
	case strings.HasPrefix(ruta, "/api/v1/puntos/"):
		s.puntoUI(w, r, inst, strings.TrimPrefix(ruta, "/api/v1/puntos/"))
	default:
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, "ruta no encontrada: "+r.Method+" "+ruta))
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
	err = EditarPuntos(ruta, origen, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		if slices.ContainsFunc(lista, func(c ConfigPunto) bool { return c.Nombre == p.Nombre }) {
			return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "el punto de acceso "+p.Nombre+" ya existe")
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

// puntoUI activa o desactiva (PATCH), elimina (DELETE) o muestra el token (GET <nombre>/token) de un punto de la
// instancia de la sesión.
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
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, "punto de acceso no encontrado en esta instancia: "+nombre))
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
		var cambio struct {
			Activo bool `json:"activo"`
		}
		if err := api.LeerJSON(w, r, &cambio); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		if editar(func(lista []ConfigPunto) ([]ConfigPunto, error) {
			for i := range lista {
				if lista[i].Nombre == nombre {
					lista[i].Activo = &cambio.Activo
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
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, "ruta no encontrada: "+r.Method+" "+r.URL.Path))
	}
}

// csrfValido compara en tiempo constante la cabecera X-CSRF-Token con la de la sesión.
func csrfValido(r *http.Request, ses *sesion) bool {
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(ses.csrf)) == 1
}
