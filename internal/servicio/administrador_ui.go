package servicio

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tramtai/internal/api"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/totp"
)

// Lo que la Web UI pide al administrador del servidor: la entrada con contraseña y Ajustes → Seguridad. Todo lo que
// compara una contraseña pasa por los frenos (frenos.go); cada acceso, fallo y bloqueo queda en el registro con la
// dirección del cliente (ClienteReal).

// soloServidor responde 404 fuera del modo servidor: el administrador es del servidor.
func (s *Servicio) soloServidor(w http.ResponseWriter, r *http.Request) bool {
	if s.entorno() == "servidor" {
		return true
	}
	api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.T("el administrador solo existe en el modo servidor")))
	return false
}

// entradaUI dice a la página de entrada si hay administrador, si pide código y si la petición llegó por HTTPS.
func (s *Servicio) entradaUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	e := contrato.EntradaRemota{Contrasena: leerAdministrador(s.opc.Datos) != "", HTTPS: EsHTTPS(r, s.opc.Confiables)}
	if e.Contrasena { // sin administrador (el escritorio) no se pregunta al almacén
		e.TOTP, _ = hayTOTP()
	}
	api.EscribirJSON(w, http.StatusOK, e)
}

// bloqueada responde 429 a una dirección que espera y lo anota.
func (s *Servicio) bloqueada(w http.ResponseWriter, r *http.Request, ip netip.Addr, espera time.Duration) {
	fmt.Fprintf(s.opc.Registros, "web ui: intento bloqueado desde %s en %s (espera %s)\n", ip, r.URL.Path, espera.Round(time.Second))
	demasiados(w, r, espera, idioma.Tf("demasiados intentos fallidos: vuelve a probar dentro de %d minutos", int(espera.Minutes())+1))
}

// conFrenos calcula con argon2id bajo los frenos: 429 si la dirección (o todas) espera, comprobado otra vez ya con el
// hueco tomado porque las peticiones a la vez pasan la primera comprobación antes de que se cuenten sus fallos; 503 con
// la cola llena. Devuelve false si ya respondió o la petición se canceló.
func (s *Servicio) conFrenos(w http.ResponseWriter, r *http.Request, ip netip.Addr, calcular func()) bool {
	if espera := s.frenos.Permitido(ip, time.Now()); espera > 0 {
		s.bloqueada(w, r, ip, espera)
		return false
	}
	ocupado, ok := s.frenos.tomar(r.Context())
	if ocupado {
		fmt.Fprintf(s.opc.Registros, "web ui: entrada ocupada (cola de argon2id llena); rechazada %s\n", ip)
		w.Header().Set("Retry-After", "5")
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoEntradaOcupada, idioma.T("hay demasiados intentos de entrada a la vez: vuelve a probar en unos segundos")))
		return false
	}
	if !ok {
		return false
	}
	defer s.frenos.soltar()
	if espera := s.frenos.Permitido(ip, time.Now()); espera > 0 {
		s.bloqueada(w, r, ip, espera)
		return false
	}
	calcular()
	return true
}

// anotarFallo cuenta un fallo en los frenos y lo anota en el registro, con el bloqueo si lo provoca.
func (s *Servicio) anotarFallo(ip netip.Addr, que string) {
	if s.frenos.Fallo(ip, time.Now()) {
		fmt.Fprintf(s.opc.Registros, "web ui: %s fallido desde %s; queda bloqueada\n", que, ip)
		return
	}
	fmt.Fprintf(s.opc.Registros, "web ui: %s fallido desde %s\n", que, ip)
}

// errorAdministrador responde un error al comprobar la contraseña; si el archivo está dañado, su ruta va solo al registro.
func (s *Servicio) errorAdministrador(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errAdministradorDanado) {
		fmt.Fprintf(s.opc.Registros, "web ui: %s está dañado\n", filepath.Join(s.opc.Datos, archivoAdministrador))
	}
	api.EscribirError(w, r, err)
}

// comprobarActual pide otra vez la contraseña del administrador (con los frenos) antes de un cambio de seguridad y, si
// vale, hace siVale con el mismo hueco de argon2id. Si no vale, cuenta el fallo y responde 400 (con 401 la Web UI
// renueva la sesión y repite la petición, y contaría dos fallos). Devuelve false si ya respondió.
func (s *Servicio) comprobarActual(w http.ResponseWriter, r *http.Request, ip netip.Addr, actual string, siVale func() error) bool {
	var ok bool
	var err error
	if !s.conFrenos(w, r, ip, func() {
		ok, err = ComprobarContrasena(s.opc.Datos, actual)
		switch {
		case err != nil:
		case !ok:
			s.anotarFallo(ip, "contraseña actual en "+r.URL.Path) // con el hueco aún tomado, como en accederUI
		case siVale != nil:
			err = siVale()
		}
	}) {
		return false
	}
	switch {
	case err != nil:
		s.errorAdministrador(w, r, err)
		return false
	case !ok:
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("la contraseña actual no es esa")))
		return false
	}
	return true
}

// segundoFactor comprueba el código: TOTP (6 dígitos, sin reutilizar un paso) o uno de recuperación, que se gasta. Sin
// TOTP no hace falta código.
func (s *Servicio) segundoFactor(codigo string, ahora time.Time) bool {
	secreto, err := leerCredencial(nombreTOTP)
	if err != nil {
		fmt.Fprintf(s.opc.Registros, "web ui: el almacén no da el secreto TOTP (%v): no se entra\n", err)
		return false
	}
	if secreto == "" {
		return true
	}
	codigo = strings.TrimSpace(codigo)
	if len(codigo) == 6 && strings.Trim(codigo, "0123456789") == "" {
		s.muSesiones.Lock()
		defer s.muSesiones.Unlock()
		paso, ok := totp.Valido(secreto, codigo, ahora, s.ultimoPasoTOTP)
		if ok {
			s.ultimoPasoTOTP = paso
		}
		return ok
	}
	ok, err := UsarCodigoRecuperacion(s.opc.Datos, codigo)
	if err != nil {
		fmt.Fprintf(s.opc.Registros, "web ui: no se pudieron leer los códigos de recuperación: %v\n", err)
	}
	return ok
}

// accederUI abre una sesión remota (no local) de la global con la contraseña del administrador y, si lo tiene, su código
// TOTP o uno de recuperación. Solo por HTTPS y con administrador; siempre el mismo mensaje si algo no coincide; responde
// JSON (la sesión) y nunca redirige.
func (s *Servicio) accederUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	var peticion struct {
		Contrasena string `json:"contrasena"`
		Codigo     string `json:"codigo"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	ip := ClienteReal(r, s.opc.Confiables)
	if !EsHTTPS(r, s.opc.Confiables) {
		fmt.Fprintf(s.opc.Registros, "web ui: intento de acceso sin HTTPS desde %s\n", ip)
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoEntradaRemotaNoDisponible, idioma.T("con contraseña solo se entra por HTTPS: con el TLS propio del servicio (servicio.tls.modo) o desde un proxy de confianza (servicio.proxy.confiables) que envíe X-Forwarded-Proto: https")))
		return
	}
	// La línea y la generación, juntas: si la contraseña cambia antes de crear la sesión, la generación ya no es esta.
	s.muSesiones.Lock()
	s.vigilarAdministrador(true)
	linea, generacion := s.lineaAdministrador, s.generacion
	s.muSesiones.Unlock()
	if linea == "" {
		fmt.Fprintf(s.opc.Registros, "web ui: intento de acceso sin administrador desde %s\n", ip)
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoEntradaRemotaNoDisponible, idioma.T("este servidor aún no tiene administrador: ponle contraseña con «tramtai --servidor administrador contraseña» o entra con el código de «tramtai --servidor abrir»")))
		return
	}
	var ok bool
	var err error
	// El fallo se cuenta con el hueco aún tomado: así ninguna petición a la vez pasa los frenos antes de que cuente.
	if !s.conFrenos(w, r, ip, func() {
		if ok, err = comprobarLinea(linea, peticion.Contrasena); err != nil {
			return
		}
		// El código solo se mira con la contraseña buena: una mala no gasta un paso TOTP ni un código de recuperación.
		if ok = ok && s.segundoFactor(peticion.Codigo, time.Now()); !ok {
			s.anotarFallo(ip, "acceso")
		}
	}) {
		return
	}
	if err != nil {
		s.errorAdministrador(w, r, err)
		return
	}
	if !ok {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoAutorizado, idioma.T("contraseña o código incorrectos")))
		return
	}
	ses := s.crearSesion(w, r, "global", false)
	s.muSesiones.Lock()
	cambio := ses.generacion != generacion
	if cambio {
		ses.vence = time.Time{} // la contraseña cambió mientras se comprobaba: la sesión no vale (sin contar fallo)
	}
	s.muSesiones.Unlock()
	if cambio {
		fmt.Fprintf(s.opc.Registros, "web ui: acceso desde %s descartado: la contraseña cambió mientras se comprobaba\n", ip)
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoAutorizado, idioma.T("contraseña o código incorrectos")))
		return
	}
	s.frenos.Exito(ip, time.Now())
	fmt.Fprintf(s.opc.Registros, "web ui: acceso correcto del administrador desde %s\n", ip)
	s.responderSesion(w, r, ses)
}

// contrasenaUI cambia la contraseña con la actual (que cuenta en los frenos como un intento de entrada) y cierra todas
// las sesiones.
func (s *Servicio) contrasenaUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if !s.soloServidor(w, r) {
		return
	}
	var peticion struct {
		Actual string `json:"actual"`
		Nueva  string `json:"nueva"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	ip := ClienteReal(r, s.opc.Confiables)
	if !s.comprobarActual(w, r, ip, peticion.Actual, func() error { return FijarContrasena(s.opc.Datos, peticion.Nueva) }) {
		return
	}
	s.cerrarSesiones()
	fmt.Fprintf(s.opc.Registros, "web ui: contraseña del administrador cambiada desde %s; sesiones cerradas\n", ip)
	w.WriteHeader(http.StatusNoContent)
}

// activarTOTPUI activa el TOTP en dos pasos. Primero, con la contraseña actual (y, si ya hay TOTP, un código del secreto
// actual o uno de recuperación), enseña un secreto nuevo, que queda pendiente en la sesión; después, con un código de ese
// secreto, lo guarda y da los 10 códigos de recuperación. Así nadie lo activa sin haberlo apuntado, y una sesión robada o
// abierta con código no lo cambia sin la contraseña.
func (s *Servicio) activarTOTPUI(w http.ResponseWriter, r *http.Request, ses *sesion) {
	if !s.soloServidor(w, r) {
		return
	}
	var peticion struct {
		Actual       string `json:"actual"`
		CodigoActual string `json:"codigo_actual"`
		Codigo       string `json:"codigo"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if totpSoloCLI {
		api.EscribirError(w, r, errTOTPSoloCLI())
		return
	}
	ip := ClienteReal(r, s.opc.Confiables)
	if peticion.Codigo == "" {
		if !s.comprobarActual(w, r, ip, peticion.Actual, nil) {
			return
		}
		if !s.segundoFactor(peticion.CodigoActual, time.Now()) {
			s.anotarFallo(ip, "código TOTP actual")
			api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("ya hay TOTP: escribe un código de la app (o uno de recuperación) para cambiarlo")))
			return
		}
		secreto, err := totp.Generar()
		if err != nil {
			api.EscribirError(w, r, err)
			return
		}
		s.muSesiones.Lock()
		ses.totpPendiente = secreto
		s.muSesiones.Unlock()
		api.EscribirJSON(w, http.StatusOK, contrato.SecretoTOTP{Secreto: secreto, URI: totp.URI(secreto, s.cuentaTOTP())})
		return
	}
	s.muSesiones.Lock()
	pendiente := ses.totpPendiente
	s.muSesiones.Unlock()
	paso, valido := totp.Valido(pendiente, strings.TrimSpace(peticion.Codigo), time.Now(), 0)
	if pendiente == "" || !valido {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("el código no coincide con el secreto nuevo: revisa que el equipo y el móvil estén en hora y vuelve a probar")))
		return
	}
	if err := s.fijarTOTP(pendiente); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.muSesiones.Lock()
	ses.totpPendiente = ""
	s.ultimoPasoTOTP = max(s.ultimoPasoTOTP, paso) // el código de la confirmación ya está visto
	s.muSesiones.Unlock()
	codigos, err := NuevosCodigosRecuperacion(s.opc.Datos)
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	fmt.Fprintf(s.opc.Registros, "web ui: TOTP del administrador activado desde %s\n", ip)
	api.EscribirJSON(w, http.StatusOK, contrato.CodigosRecuperacion{Codigos: codigos})
}

// cuentaTOTP es el nombre con el que la app de autenticación muestra este servidor: su dominio o el nombre del equipo.
func (s *Servicio) cuentaTOTP() string {
	if len(s.opc.Dominios) > 0 {
		return s.opc.Dominios[0]
	}
	if nombre, err := os.Hostname(); err == nil {
		return nombre
	}
	return "servidor"
}

// leerActual lee {actual} del cuerpo: la contraseña que piden quitar el TOTP y regenerar los códigos.
func leerActual(w http.ResponseWriter, r *http.Request) (string, bool) {
	var peticion struct {
		Actual string `json:"actual"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return "", false
	}
	return peticion.Actual, true
}

// quitarTOTPUI quita el TOTP con la contraseña actual (409 con suscripción personal o en Linux).
func (s *Servicio) quitarTOTPUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if !s.soloServidor(w, r) {
		return
	}
	actual, ok := leerActual(w, r)
	if !ok {
		return
	}
	ip := ClienteReal(r, s.opc.Confiables)
	if !s.comprobarActual(w, r, ip, actual, nil) {
		return
	}
	if err := s.quitarTOTP(); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	fmt.Fprintf(s.opc.Registros, "web ui: TOTP del administrador quitado desde %s\n", ip)
	w.WriteHeader(http.StatusNoContent)
}

// recuperacionUI crea 10 códigos de recuperación nuevos con la contraseña actual (los anteriores dejan de valer); sin
// TOTP no sirven.
func (s *Servicio) recuperacionUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if !s.soloServidor(w, r) {
		return
	}
	actual, ok := leerActual(w, r)
	if !ok {
		return
	}
	if conTOTP, _ := hayTOTP(); !conTOTP {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoAccionNoPermitida, idioma.T("los códigos de recuperación sustituyen al TOTP: actívalo antes")))
		return
	}
	ip := ClienteReal(r, s.opc.Confiables)
	if !s.comprobarActual(w, r, ip, actual, nil) {
		return
	}
	codigos, err := NuevosCodigosRecuperacion(s.opc.Datos)
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	fmt.Fprintf(s.opc.Registros, "web ui: códigos de recuperación nuevos desde %s\n", ip)
	api.EscribirJSON(w, http.StatusOK, contrato.CodigosRecuperacion{Codigos: codigos})
}

// cerrarSesionesUI es «Cerrar todas las sesiones»: también la de quien lo pide.
func (s *Servicio) cerrarSesionesUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	s.cerrarSesiones()
	fmt.Fprintf(s.opc.Registros, "web ui: todas las sesiones cerradas desde %s\n", ClienteReal(r, s.opc.Confiables))
	w.WriteHeader(http.StatusNoContent)
}
