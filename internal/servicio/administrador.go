package servicio

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/totp"
)

// El administrador del servidor: su contraseña (solo la huella argon2id, en datos/administrador), los códigos de
// recuperación (solo sus huellas, en datos/administrador-recuperacion) y el secreto TOTP, en el almacén cifrado con el
// nombre TRAMTAI_TOTP.
const (
	archivoAdministrador = "administrador"
	archivoRecuperacion  = "administrador-recuperacion"
	nombreTOTP           = "TRAMTAI_TOTP"
	largoMinimo          = 12
)

// totpSoloCLI: en Linux el servicio no puede escribir en systemd-creds, así que el TOTP se activa y se quita con la CLI
// (servidor.GuardarClave, como root); las pruebas lo cambian.
var totpSoloCLI = runtime.GOOS == "linux"

// muRecuperacion: un código de recuperación se gasta una sola vez aunque lleguen dos a la vez.
var muRecuperacion sync.Mutex

// FijarContrasena guarda la huella argon2id (m=64 MiB, t=3, p=1, sal de 16 bytes, clave de 32) de la contraseña del
// administrador, en formato PHC. Pide 12 caracteres como mínimo.
func FijarContrasena(datos, c string) error {
	if utf8.RuneCountInString(c) < largoMinimo {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("la contraseña necesita al menos %d caracteres", largoMinimo))
	}
	sal := make([]byte, 16)
	if _, err := rand.Read(sal); err != nil {
		return err
	}
	huella := argon2.IDKey([]byte(c), sal, 3, 64*1024, 1, 32)
	linea := fmt.Sprintf("$argon2id$v=%d$m=65536,t=3,p=1$%s$%s\n", argon2.Version, base64.RawStdEncoding.EncodeToString(sal), base64.RawStdEncoding.EncodeToString(huella))
	return configuracion.EscribirAtomico(filepath.Join(datos, archivoAdministrador), []byte(linea), 0o600)
}

// leerAdministrador es la línea PHC de la contraseña ("" si no hay administrador o no se puede leer).
func leerAdministrador(datos string) string {
	linea, err := os.ReadFile(filepath.Join(datos, archivoAdministrador))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(linea))
}

// ComprobarContrasena compara la contraseña con la huella guardada, en tiempo constante. Sin administrador: false, nil.
func ComprobarContrasena(datos, c string) (bool, error) {
	linea, err := os.ReadFile(filepath.Join(datos, archivoAdministrador))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return comprobarLinea(datos, strings.TrimSpace(string(linea)), c)
}

// comprobarLinea compara la contraseña con una línea PHC de argon2id, con sus parámetros.
func comprobarLinea(datos, linea, c string) (bool, error) {
	partes := strings.Split(linea, "$")
	var memoria, pasadas uint32
	var hilos uint8
	var sal, huella []byte
	var err error
	if len(partes) == 6 && partes[1] == "argon2id" && partes[2] == fmt.Sprintf("v=%d", argon2.Version) {
		if _, err = fmt.Sscanf(partes[3], "m=%d,t=%d,p=%d", &memoria, &pasadas, &hilos); err == nil {
			if sal, err = base64.RawStdEncoding.DecodeString(partes[4]); err == nil {
				huella, err = base64.RawStdEncoding.DecodeString(partes[5])
			}
		}
	} else {
		err = errors.New("formato")
	}
	if err != nil || len(huella) == 0 || pasadas == 0 || hilos == 0 {
		return false, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("el archivo del administrador (%s) está dañado: vuelve a poner la contraseña con «tramtai --servidor administrador contraseña»",
			filepath.Join(datos, archivoAdministrador)))
	}
	calculada := argon2.IDKey([]byte(c), sal, pasadas, memoria, hilos, uint32(len(huella)))
	return subtle.ConstantTimeCompare(calculada, huella) == 1, nil
}

// normalizarCodigo deja un código de recuperación como se genera (XXXXX-XXXXX en mayúsculas): se acepta en minúsculas,
// sin guion o con espacios.
func normalizarCodigo(c string) string {
	limpio := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(c))
	if len(limpio) != 10 {
		return limpio
	}
	return limpio[:5] + "-" + limpio[5:]
}

// NuevosCodigosRecuperacion crea 10 códigos de recuperación de un solo uso (XXXXX-XXXXX en base32) y guarda solo sus
// huellas; los anteriores dejan de valer. Esta es la única vez que se ven.
func NuevosCodigosRecuperacion(datos string) ([]string, error) {
	codigos := make([]string, 10)
	var huellas strings.Builder
	for i := range codigos {
		bytes := make([]byte, 7) // 56 bits; se usan 50, 10 caracteres de base32
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		codigos[i] = normalizarCodigo(base32.StdEncoding.EncodeToString(bytes)[:10])
		huellas.WriteString(api.HuellaToken(codigos[i]) + "\n")
	}
	muRecuperacion.Lock()
	defer muRecuperacion.Unlock()
	if err := configuracion.EscribirAtomico(filepath.Join(datos, archivoRecuperacion), []byte(huellas.String()), 0o600); err != nil {
		return nil, err
	}
	return codigos, nil
}

// UsarCodigoRecuperacion comprueba un código de recuperación y, si vale, lo borra: no vuelve a valer.
func UsarCodigoRecuperacion(datos, c string) (bool, error) {
	muRecuperacion.Lock()
	defer muRecuperacion.Unlock()
	ruta := filepath.Join(datos, archivoRecuperacion)
	contenido, err := os.ReadFile(ruta)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	buscada := []byte(api.HuellaToken(normalizarCodigo(c)))
	var quedan strings.Builder
	encontrado := false
	for linea := range strings.Lines(string(contenido)) {
		linea = strings.TrimSpace(linea)
		if linea == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(linea), buscada) == 1 && !encontrado {
			encontrado = true
			continue
		}
		quedan.WriteString(linea + "\n")
	}
	if !encontrado {
		return false, nil
	}
	return true, configuracion.EscribirAtomico(ruta, []byte(quedan.String()), 0o600)
}

// BorrarCodigosRecuperacion quita los códigos de recuperación (al quitar el TOTP ya no sirven).
func BorrarCodigosRecuperacion(datos string) error {
	if err := os.Remove(filepath.Join(datos, archivoRecuperacion)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// ComprobarQuitarTOTP rechaza quitar el TOTP mientras un agente use la suscripción personal: en remoto la exige (spec §5).
func ComprobarQuitarTOTP(c *configuracion.Configuracion) error {
	for _, f := range agentes.Todos() {
		if c.Booleano("adaptadores." + f.Nombre + ".suscripcion_personal") {
			return nucleo.NuevoError(nucleo.CodigoAccionNoPermitida, idioma.Tf("no se puede quitar el TOTP mientras %s usa tu suscripción personal en el servidor: pásalo antes a clave de API (adaptadores.%s.autenticacion: clave_api)", f.Nombre, f.Nombre))
		}
	}
	return nil
}

// errTOTPSoloCLI es el 409 de Linux: el servicio no puede escribir en systemd-creds.
func errTOTPSoloCLI() error {
	return nucleo.NuevoError(nucleo.CodigoAccionNoPermitida, idioma.T("en Linux el servicio no puede escribir en systemd-creds: el TOTP se activa con «sudo tramtai-servidor administrador totp activar» y se quita con «sudo tramtai-servidor administrador totp quitar»"))
}

// fijarTOTP guarda el secreto TOTP en el almacén del propio servicio (Windows: el de NT SERVICE\Tramtai).
func (s *Servicio) fijarTOTP(secreto string) error {
	if _, err := totp.Codigo(secreto, time.Now()); err != nil {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("el secreto TOTP no es base32"))
	}
	if totpSoloCLI {
		return errTOTPSoloCLI()
	}
	return guardarCredencial(nombreTOTP, strings.ToUpper(strings.ReplaceAll(secreto, " ", "")))
}

// quitarTOTP borra el secreto TOTP y los códigos de recuperación, si no hay suscripción personal.
func (s *Servicio) quitarTOTP() error {
	if err := ComprobarQuitarTOTP(s.opc.Config); err != nil {
		return err
	}
	if totpSoloCLI {
		return errTOTPSoloCLI()
	}
	if err := borrarCredencial(nombreTOTP); err != nil {
		return err
	}
	return BorrarCodigosRecuperacion(s.opc.Datos)
}

// hayTOTP dice si el administrador tiene TOTP; si el almacén no responde, cuenta como sí (para no entrar sin él).
func hayTOTP() (bool, error) {
	secreto, err := leerCredencial(nombreTOTP)
	return secreto != "" || err != nil, err
}

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
	if !EsHTTPS(r, s.opc.Confiables) {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoEntradaRemotaNoDisponible, idioma.T("con contraseña solo se entra por HTTPS: con el TLS propio del servicio (servicio.tls.modo) o desde un proxy de confianza (servicio.proxy.confiables) que envíe X-Forwarded-Proto: https")))
		return
	}
	linea := leerAdministrador(s.opc.Datos)
	if linea == "" {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoEntradaRemotaNoDisponible, idioma.T("este servidor aún no tiene administrador: ponle contraseña con «tramtai --servidor administrador contraseña» o entra con el código de «tramtai --servidor abrir»")))
		return
	}
	ip, ahora := ClienteReal(r, s.opc.Confiables), time.Now()
	if espera := s.frenos.Permitido(ip, ahora); espera > 0 {
		fmt.Fprintf(s.opc.Registros, "web ui: intento de acceso bloqueado desde %s (espera %s)\n", ip, espera.Round(time.Second))
		demasiados(w, r, espera, idioma.Tf("demasiados intentos fallidos: vuelve a probar dentro de %d minutos", int(espera.Minutes())+1))
		return
	}
	var ok bool
	var err error
	if !s.frenos.conArgon(r, func() { ok, err = comprobarLinea(s.opc.Datos, linea, peticion.Contrasena) }) {
		return
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	// El código solo se mira con la contraseña buena: una mala no gasta un paso TOTP ni un código de recuperación.
	if ok = ok && s.segundoFactor(strings.TrimSpace(peticion.Codigo), ahora); !ok {
		if s.frenos.Fallo(ip, ahora) {
			fmt.Fprintf(s.opc.Registros, "web ui: acceso fallido desde %s; queda bloqueada\n", ip)
		} else {
			fmt.Fprintf(s.opc.Registros, "web ui: acceso fallido desde %s\n", ip)
		}
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoAutorizado, idioma.T("contraseña o código incorrectos")))
		return
	}
	s.frenos.Exito(ip)
	fmt.Fprintf(s.opc.Registros, "web ui: acceso correcto del administrador desde %s\n", ip)
	ses := s.crearSesion(w, r, "global", false)
	ses.administrador = linea // la cookie aún no ha salido: nadie más usa esta sesión
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
	ip, ahora := ClienteReal(r, s.opc.Confiables), time.Now()
	if espera := s.frenos.Permitido(ip, ahora); espera > 0 {
		demasiados(w, r, espera, idioma.Tf("demasiados intentos fallidos: vuelve a probar dentro de %d minutos", int(espera.Minutes())+1))
		return
	}
	var ok bool
	var err error
	if !s.frenos.conArgon(r, func() {
		if ok, err = ComprobarContrasena(s.opc.Datos, peticion.Actual); ok && err == nil {
			err = FijarContrasena(s.opc.Datos, peticion.Nueva)
		}
	}) {
		return
	}
	switch {
	case err != nil:
		api.EscribirError(w, r, err)
		return
	case !ok:
		s.frenos.Fallo(ip, ahora)
		fmt.Fprintf(s.opc.Registros, "web ui: cambio de contraseña con la actual mal desde %s\n", ip)
		// 400 y no 401: con 401 la Web UI renueva la sesión y repite la petición, y contaría dos fallos.
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("la contraseña actual no es esa")))
		return
	}
	s.cerrarSesiones()
	fmt.Fprintf(s.opc.Registros, "web ui: contraseña del administrador cambiada desde %s; sesiones cerradas\n", ip)
	w.WriteHeader(http.StatusNoContent)
}

// activarTOTPUI activa el TOTP en dos pasos: sin código, enseña un secreto nuevo (que queda pendiente en la sesión); con
// el código de ese secreto, lo guarda y da los 10 códigos de recuperación. Así nadie lo activa sin haberlo apuntado.
func (s *Servicio) activarTOTPUI(w http.ResponseWriter, r *http.Request, ses *sesion) {
	if !s.soloServidor(w, r) {
		return
	}
	var peticion struct {
		Codigo string `json:"codigo"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if totpSoloCLI {
		api.EscribirError(w, r, errTOTPSoloCLI())
		return
	}
	s.muSesiones.Lock()
	pendiente := ses.totpPendiente
	s.muSesiones.Unlock()
	if peticion.Codigo == "" {
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
	fmt.Fprintf(s.opc.Registros, "web ui: TOTP del administrador activado desde %s\n", ClienteReal(r, s.opc.Confiables))
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

func (s *Servicio) quitarTOTPUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if !s.soloServidor(w, r) {
		return
	}
	if err := s.quitarTOTP(); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	fmt.Fprintf(s.opc.Registros, "web ui: TOTP del administrador quitado desde %s\n", ClienteReal(r, s.opc.Confiables))
	w.WriteHeader(http.StatusNoContent)
}

// recuperacionUI crea 10 códigos de recuperación nuevos (los anteriores dejan de valer); sin TOTP no sirven.
func (s *Servicio) recuperacionUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if !s.soloServidor(w, r) {
		return
	}
	if conTOTP, _ := hayTOTP(); !conTOTP {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoAccionNoPermitida, idioma.T("los códigos de recuperación sustituyen al TOTP: actívalo antes")))
		return
	}
	codigos, err := NuevosCodigosRecuperacion(s.opc.Datos)
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	api.EscribirJSON(w, http.StatusOK, contrato.CodigosRecuperacion{Codigos: codigos})
}

// cerrarSesionesUI es «Cerrar todas las sesiones»: también la de quien lo pide.
func (s *Servicio) cerrarSesionesUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	s.cerrarSesiones()
	fmt.Fprintf(s.opc.Registros, "web ui: todas las sesiones cerradas desde %s\n", ClienteReal(r, s.opc.Confiables))
	w.WriteHeader(http.StatusNoContent)
}
