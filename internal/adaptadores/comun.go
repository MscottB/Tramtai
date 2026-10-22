package adaptadores

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"tramtai/internal/credenciales"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// Autenticación admitida por los adaptadores CLI.
const (
	AutenticacionClaveAPI    = "clave_api"
	AutenticacionSuscripcion = "suscripcion"
)

// FormatoSerializacion identifica el enmarcado de conversaciones de varios turnos;
// se publicará en /api/v1/capacidades.
const FormatoSerializacion = "tramtai-turnos-v2"

// variablesCredenciales se retiran del entorno heredado: cada CLI recibe solo la suya.
var variablesCredenciales = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN",
	"CODEX_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY", "COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"}

// Estado es el resultado de la detección de un adaptador para el diagnóstico.
type Estado struct {
	Agente        string
	Ruta          string
	Version       string
	Autenticacion string
	Terminos      string
	Problema      string
	// ProblemaCodigo clasifica Problema para la Web UI sin depender del idioma del texto.
	ProblemaCodigo string
	// Cuenta es la cuenta de la suscripción, si el CLI la informa (solo se muestra en este equipo).
	Cuenta string
}

// Códigos de ProblemaCodigo; vacío con Problema es otro problema que hay que revisar.
const (
	ProblemaNoInstalado = "no_instalado"
	ProblemaFaltaClave  = "falta_clave"
	// ProblemaAviso es algo que conviene saber pero no impide usar el agente.
	ProblemaAviso = "aviso"
	// ProblemaUsaClave: el agente está en suscripción pero su CLI pasó a una clave de API.
	ProblemaUsaClave = "usa_clave_api"
	// ProblemaSesion: una petición real falló por autenticación y aún no volvió a funcionar.
	ProblemaSesion = "sesion_no_responde"
)

// Estados de SesionCLI.
const (
	SesionOK          = "ok"
	SesionSin         = "sin_sesion"
	SesionUsaClave    = "usa_clave_api"
	SesionDesconocida = "desconocido"
)

// SesionCLI es lo que sabe un CLI de su propia sesión: si hay, si es de la suscripción o de una clave de API, y la
// cuenta si la informa.
type SesionCLI struct {
	Estado string
	Cuenta string
}

// ComprobadorSesion lo cumplen los agentes con suscripción que saben comprobar su sesión sin gastar mensajes (su
// comando de estado o su archivo de sesión; ver docs/equipo/m0-agentes.md).
type ComprobadorSesion interface {
	ComprobarSesion(ctx context.Context) SesionCLI
}

// SalidaEstado ejecuta un comando de estado del CLI (sin claves en el entorno y con 5 s como mucho) y devuelve lo que
// escribió, también si terminó con error (codex login status sale con 1 sin sesión); ok es falso si no llegó a correr.
func SalidaEstado(ctx context.Context, ruta string, args ...string) (salida string, ok bool) {
	ctx, cancelar := context.WithTimeout(ctx, 5*time.Second)
	defer cancelar()
	cmd := exec.CommandContext(ctx, ruta, args...)
	cmd.Env = EntornoLimpio(nil)
	procesos.SinVentana(cmd)
	datos, err := cmd.Output()
	var salidaError *exec.ExitError
	if err != nil && !errors.As(err, &salidaError) {
		return "", false
	}
	return string(datos), ctx.Err() == nil
}

// AplicarSesion pasa a e lo que dijo ComprobarSesion: sin sesión es como una clave que falta (la tarjeta enseña cómo
// iniciarla), con clave de API se avisa, y con sesión se anota la cuenta. cli es el nombre que ve el usuario.
func AplicarSesion(e *Estado, s SesionCLI, cli, iniciar string) {
	switch s.Estado {
	case SesionSin:
		e.Problema, e.ProblemaCodigo = idioma.Tf("%s no tiene sesión iniciada: inicia sesión con %s o pasa a clave de API", cli, iniciar), ProblemaFaltaClave
	case SesionUsaClave:
		e.Problema, e.ProblemaCodigo = idioma.Tf("%s usa ahora una clave de API, no tu suscripción", cli), ProblemaUsaClave
	case SesionOK:
		e.Cuenta = s.Cuenta
	}
}

// Detectable lo implementan los adaptadores que pueden informar su estado sin consumir cuota.
type Detectable interface {
	Detectar(ctx context.Context) Estado
}

// Separar reparte la solicitud en el texto de sistema y el prompt. Un único mensaje de usuario
// se envía sin enmarcar; una conversación se delimita por turnos con su rol original y el contenido intacto.
func Separar(s nucleo.SolicitudEjecucion) (sistema, prompt string) {
	var partesSistema []string
	if s.Instrucciones != "" {
		partesSistema = append(partesSistema, s.Instrucciones)
	}
	var turnos []nucleo.Mensaje
	for _, m := range s.Mensajes {
		if m.Rol == "sistema" {
			partesSistema = append(partesSistema, m.Contenido)
		} else {
			turnos = append(turnos, m)
		}
	}
	sistema = strings.Join(partesSistema, "\n\n")
	if len(turnos) == 1 && turnos[0].Rol == "usuario" {
		return sistema, turnos[0].Contenido
	}
	// Los delimitadores llevan un código por petición: un mensaje que escriba «<<<fin turno>>>» no cierra su turno.
	codigo := codigoDelimitador()
	var b strings.Builder
	for _, m := range turnos {
		b.WriteString("<<<turno-" + codigo + " rol=" + m.Rol + ">>>\n" + m.Contenido + "\n<<<fin turno-" + codigo + ">>>\n")
	}
	return sistema, b.String()
}

// codigoDelimitador es un código aleatorio para los delimitadores de una petición.
func codigoDelimitador() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Secretos son las claves de proveedor disponibles (entorno o almacén del sistema), para taparlas en la salida.
func Secretos() []string {
	var lista []string
	for _, nombre := range variablesCredenciales {
		// Las que el almacén no admite (GH_TOKEN, GITHUB_TOKEN…) solo pueden venir del entorno: no se pregunta al almacén,
		// que en macOS y Linux lanza un proceso por consulta.
		clave := os.Getenv(nombre)
		if clave == "" && credenciales.Admitido(nombre) {
			clave = ClaveEntorno(nombre)
		}
		if len(clave) >= 8 {
			lista = append(lista, clave)
		}
	}
	return lista
}

// Redactar tapa los secretos dados en un texto que llega a la respuesta, al historial o a la Web UI.
func Redactar(texto string, secretos []string) string { return RedactarCon(texto, secretos...) }

// EnmarcarSistema antepone el sistema como primer bloque delimitado, para CLIs sin mecanismo nativo.
func EnmarcarSistema(sistema, prompt string) string {
	if sistema == "" {
		return prompt
	}
	codigo := codigoDelimitador()
	return "<<<turno-" + codigo + " rol=sistema>>>\n" + sistema + "\n<<<fin turno-" + codigo + ">>>\n" + prompt
}

// EntornoLimpio hereda el entorno sin credenciales y añade las variables indicadas. Quita además las TRAMTAI_*
// (el token del servicio) y cualquier otro secreto (GITHUB_TOKEN, AWS_*…), salvo la configuración propia de esa
// CLI (propios: por ejemplo CLAUDE_ y ANTHROPIC_ para Claude); los secretos de otro proveedor no le llegan.
func EntornoLimpio(propios []string, extra ...string) []string {
	quitar := map[string]bool{}
	for _, nombre := range variablesCredenciales {
		quitar[nombre] = true
	}
	for _, par := range extra {
		nombre, _, _ := strings.Cut(par, "=")
		quitar[strings.ToUpper(nombre)] = true
	}
	var entorno []string
	for _, par := range os.Environ() {
		nombre, _, _ := strings.Cut(par, "=")
		nombre = strings.ToUpper(nombre)
		propia := slices.ContainsFunc(propios, func(p string) bool { return strings.HasPrefix(nombre, p) })
		if quitar[nombre] || strings.HasPrefix(nombre, "TRAMTAI_") || (nucleo.EntornoSecreto(par) && !propia) {
			continue
		}
		entorno = append(entorno, par)
	}
	return append(entorno, extra...)
}

// ClaveEntorno devuelve la primera clave definida: la variable de entorno tiene prioridad y, si falta, se busca en el
// almacén de credenciales del sistema (tramtai claves guardar). Un almacén que no responde cuenta como sin clave;
// al ejecutar, ClaveAlmacen dice por qué.
func ClaveEntorno(nombres ...string) string {
	clave, _ := ClaveAlmacen(nombres...)
	return clave
}

// ClaveAlmacen es ClaveEntorno con el fallo del almacén: si no hay clave porque el almacén no responde, el trabajo lo
// dice en lugar de «falta la clave».
func ClaveAlmacen(nombres ...string) (string, error) {
	for _, nombre := range nombres {
		if valor := os.Getenv(nombre); valor != "" {
			return valor, nil
		}
	}
	var fallo error
	for _, nombre := range nombres {
		valor, err := credenciales.Leer(nombre)
		if err != nil && fallo == nil {
			fallo = nucleo.NuevoError(nucleo.CodigoErrorInterno, idioma.Tf("%s no respondió al leer %s: %s", credenciales.Almacen(), nombre, err.Error()))
		}
		if err == nil && valor != "" {
			return valor, nil
		}
	}
	return "", fallo
}

// ResolverPrograma localiza el ejecutable. Rechaza los envoltorios .cmd/.bat: pasarían los argumentos por
// cmd.exe, y el claude.cmd de npm resultó estar roto.
func ResolverPrograma(nombre string) (string, error) {
	ruta, err := exec.LookPath(nombre)
	if err != nil {
		return "", nucleo.NuevoError(nucleo.CodigoAgenteNoDisponible, idioma.Tf("no se encontró el ejecutable %s; instálalo o configura comandos_agentes", nombre))
	}
	if extension := strings.ToLower(filepath.Ext(ruta)); extension == ".cmd" || extension == ".bat" {
		return "", nucleo.NuevoError(nucleo.CodigoAgenteNoDisponible, idioma.Tf("%s es un envoltorio %s; configura comandos_agentes con la ruta al .exe real", ruta, extension))
	}
	return ruta, nil
}

// DirectorioTrabajo es donde se ejecuta el agente: el espacio vacío (aislado) o la raíz del proyecto confiable.
func DirectorioTrabajo(s nucleo.SolicitudEjecucion, espacio string) string {
	if Consciente(s) {
		return s.RaizProyecto
	}
	return espacio
}

// Consciente indica el modo consciente_proyecto ya autorizado por el planificador.
func Consciente(s nucleo.SolicitudEjecucion) bool {
	return s.ModoContexto == "consciente_proyecto" && s.RaizProyecto != ""
}

// EspacioTemporal crea el directorio privado de una ejecución con un espacio de trabajo vacío (modo aislado).
func EspacioTemporal() (raiz, espacio string, err error) {
	raiz, err = os.MkdirTemp("", "tramtai-")
	if err != nil {
		return "", "", err
	}
	espacio = filepath.Join(raiz, "espacio")
	return raiz, espacio, os.Mkdir(espacio, 0o700)
}

// RedactarCon elimina los secretos de un texto que puede acabar en un registro o en la respuesta.
func RedactarCon(texto string, secretos ...string) string {
	for _, secreto := range secretos {
		if secreto != "" {
			texto = strings.ReplaceAll(texto, secreto, "[REDACTADO]")
		}
	}
	return texto
}

// Clasificar traduce un error del proveedor a un código estable, distinto para cada tipo de error.
func Clasificar(texto string, estadoHTTP int) nucleo.CodigoError {
	t := strings.ToLower(texto)
	switch {
	case estadoHTTP == 429 || strings.Contains(t, "rate limit") || strings.Contains(t, "usage limit") || strings.Contains(t, "quota"):
		return nucleo.CodigoLimiteUsoProveedor
	case estadoHTTP == 401 || estadoHTTP == 403 || strings.Contains(t, "unauthorized") || strings.Contains(t, "api key") ||
		strings.Contains(t, "not logged in") || strings.Contains(t, "authentication"):
		return nucleo.CodigoAutenticacionAgente
	case strings.Contains(t, "model") && (strings.Contains(t, "not found") || strings.Contains(t, "does not exist") ||
		strings.Contains(t, "not supported") || strings.Contains(t, "invalid") || strings.Contains(t, "not available")):
		return nucleo.CodigoModeloNoDisponible
	}
	return nucleo.CodigoErrorAgente
}

// VersionDe ejecuta "<programa> --version" sin consumir cuota.
func VersionDe(ctx context.Context, ruta string) string {
	cmd := exec.CommandContext(ctx, ruta, "--version")
	cmd.Env = EntornoLimpio(nil) // consultar la versión no necesita ninguna clave
	procesos.SinVentana(cmd)
	salida, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(salida), "\n", 2)[0])
}
