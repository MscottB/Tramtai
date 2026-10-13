package adaptadores

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"tramtai/internal/credenciales"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// Autenticación admitida por los adaptadores CLI (sección 2.4).
const (
	AutenticacionClaveAPI    = "clave_api"
	AutenticacionSuscripcion = "suscripcion"
)

// FormatoSerializacion identifica el enmarcado de conversaciones de varios turnos (sección 8.2);
// se publicará en /api/v1/capacidades.
const FormatoSerializacion = "tramtai-turnos-v1"

// variablesCredenciales se retiran del entorno heredado: cada CLI recibe solo la suya.
var variablesCredenciales = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN",
	"CODEX_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}

// Estado es el resultado de la detección de un adaptador para el diagnóstico.
type Estado struct {
	Agente        string
	Ruta          string
	Version       string
	Autenticacion string
	Terminos      string
	Problema      string
}

// Detectable lo implementan los adaptadores que pueden informar su estado sin consumir cuota.
type Detectable interface {
	Detectar(ctx context.Context) Estado
}

// separar reparte la solicitud en el texto de sistema y el prompt (sección 8.2). Un único mensaje de usuario
// se envía sin enmarcar; una conversación se delimita por turnos con su rol original y el contenido intacto.
func separar(s nucleo.SolicitudEjecucion) (sistema, prompt string) {
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
	var b strings.Builder
	for _, m := range turnos {
		b.WriteString("<<<turno rol=" + m.Rol + ">>>\n" + m.Contenido + "\n<<<fin turno>>>\n")
	}
	return sistema, b.String()
}

// enmarcarSistema antepone el sistema como primer bloque delimitado, para CLIs sin mecanismo nativo.
func enmarcarSistema(sistema, prompt string) string {
	if sistema == "" {
		return prompt
	}
	return "<<<turno rol=sistema>>>\n" + sistema + "\n<<<fin turno>>>\n" + prompt
}

// entornoLimpio hereda el entorno sin credenciales y añade las variables indicadas.
func entornoLimpio(extra ...string) []string {
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
		if nombre, _, _ := strings.Cut(par, "="); !quitar[strings.ToUpper(nombre)] {
			entorno = append(entorno, par)
		}
	}
	return append(entorno, extra...)
}

// claveEntorno devuelve la primera clave definida: la variable de entorno tiene prioridad y, si falta, se busca en el
// almacén de credenciales del sistema (tramtai claves guardar, sección 16.1).
func claveEntorno(nombres ...string) string {
	for _, nombre := range nombres {
		if valor := os.Getenv(nombre); valor != "" {
			return valor
		}
	}
	for _, nombre := range nombres {
		if valor, err := credenciales.Leer(nombre); err == nil && valor != "" {
			return valor
		}
	}
	return ""
}

// resolverPrograma localiza el ejecutable. Rechaza los envoltorios .cmd/.bat: pasarían los argumentos por
// cmd.exe (sección 17.2), y en M0 el claude.cmd de npm resultó estar roto.
func resolverPrograma(nombre string) (string, error) {
	ruta, err := exec.LookPath(nombre)
	if err != nil {
		return "", nucleo.NuevoError(nucleo.CodigoAgenteNoDisponible, "no se encontró el ejecutable "+nombre+"; instálalo o configura comandos_agentes")
	}
	if extension := strings.ToLower(filepath.Ext(ruta)); extension == ".cmd" || extension == ".bat" {
		return "", nucleo.NuevoError(nucleo.CodigoAgenteNoDisponible, ruta+" es un envoltorio "+extension+"; configura comandos_agentes con la ruta al .exe real")
	}
	return ruta, nil
}

// directorioTrabajo es donde se ejecuta el agente: el espacio vacío (aislado) o la raíz del proyecto confiable.
func directorioTrabajo(s nucleo.SolicitudEjecucion, espacio string) string {
	if consciente(s) {
		return s.RaizProyecto
	}
	return espacio
}

// consciente indica el modo consciente_proyecto ya autorizado por el planificador.
func consciente(s nucleo.SolicitudEjecucion) bool {
	return s.ModoContexto == "consciente_proyecto" && s.RaizProyecto != ""
}

// espacioTemporal crea el directorio privado de una ejecución con un espacio de trabajo vacío (modo aislado).
func espacioTemporal() (raiz, espacio string, err error) {
	raiz, err = os.MkdirTemp("", "tramtai-")
	if err != nil {
		return "", "", err
	}
	espacio = filepath.Join(raiz, "espacio")
	return raiz, espacio, os.Mkdir(espacio, 0o700)
}

// redactar elimina los secretos de un texto que puede acabar en un registro o en la respuesta.
func redactar(texto string, secretos ...string) string {
	for _, secreto := range secretos {
		if secreto != "" {
			texto = strings.ReplaceAll(texto, secreto, "[REDACTADO]")
		}
	}
	return texto
}

// clasificar traduce un error del proveedor a un código estable (sección 19: errores diferenciados).
func clasificar(texto string, estadoHTTP int) nucleo.CodigoError {
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

// version ejecuta "<programa> --version" sin consumir cuota.
func version(ctx context.Context, ruta string) string {
	cmd := exec.CommandContext(ctx, ruta, "--version")
	procesos.SinVentana(cmd)
	salida, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(salida), "\n", 2)[0])
}
