package adaptadores

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// maximoPromptAntigravity acota el prompt, que agy recibe como argumento y Windows limita la línea de órdenes
// (32.767 unidades UTF-16): se cuenta ya escapado, con margen para el resto de la orden.
// shortcut: si agy acepta el prompt por stdin (--input-format stream-json), pasar a stdin y quitar el límite.
const maximoPromptAntigravity = 28000

// Antigravity ejecuta "agy -p" solo con la Gemini API y la clave del usuario: los términos de
// Antigravity no admiten herramientas de terceros con la sesión OAuth de una cuenta personal. Es experimental.
// shortcut: no restringe las herramientas de agy (permissions.deny/toolPermission sin validar con el CLI real);
// añadirlas cuando M0b las compruebe (docs/equipo/pendientes.md).
type Antigravity struct {
	Programa      string // nombre o ruta del ejecutable (comandos_agentes.antigravity)
	Autenticacion string // solo clave_api
}

// Detectar implementa Detectable.
func (a Antigravity) Detectar(ctx context.Context) Estado {
	e := Estado{Agente: "antigravity", Autenticacion: a.Autenticacion,
		Terminos: idioma.T("experimental: solo modelProvider gemini con clave de API propia; nunca la sesión personal")}
	if err := a.autenticacion(); err != nil {
		e.Problema = nucleo.MensajeDe(err)
	} else if ClaveEntorno("GEMINI_API_KEY") == "" {
		e.Problema, e.ProblemaCodigo = idioma.T("falta GEMINI_API_KEY (tramtai claves guardar GEMINI_API_KEY)"), ProblemaFaltaClave
	}
	ruta, err := ResolverPrograma(a.Programa)
	if err != nil {
		e.Problema, e.ProblemaCodigo = err.Error(), ProblemaNoInstalado
		return e
	}
	e.Ruta, e.Version = ruta, VersionDe(ctx, ruta)
	return e
}

func (a Antigravity) autenticacion() error {
	if a.Autenticacion != "" && a.Autenticacion != AutenticacionClaveAPI {
		return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida,
			idioma.T("antigravity solo admite clave_api: los términos de Antigravity prohíben usar la sesión personal desde otra herramienta"))
	}
	return nil
}

// Ejecutar implementa Adaptador. agy entrega el resultado al final (sin streaming de tokens).
func (a Antigravity) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(Evento) error) (Resultado, error) {
	if err := a.autenticacion(); err != nil {
		return Resultado{}, err
	}
	clave := ClaveEntorno("GEMINI_API_KEY")
	if clave == "" {
		return Resultado{}, nucleo.NuevoError(nucleo.CodigoAutenticacionAgente, idioma.T("falta GEMINI_API_KEY (tramtai claves guardar GEMINI_API_KEY)"))
	}
	ruta, err := ResolverPrograma(a.Programa)
	if err != nil {
		return Resultado{}, err
	}
	if Consciente(s) {
		// Sin restricción de herramientas comprobada, no trabaja en la carpeta del proyecto.
		return Resultado{}, nucleo.NuevoError(nucleo.CodigoContextoNoPermitido, idioma.T("antigravity no admite consciente_proyecto: usa modo_contexto aislado"))
	}
	sistema, prompt := Separar(s)
	prompt = EnmarcarSistema(sistema, prompt)
	if strings.HasPrefix(prompt, "-") {
		prompt = "\n" + prompt // tras -p, un argumento que empieza por «-» se leería como una opción de agy
	}
	if unidadesLineaOrdenes(prompt) > maximoPromptAntigravity {
		return Resultado{}, nucleo.NuevoError(nucleo.CodigoEntradaDemasiadoGrande, idioma.T("la entrada supera lo que admite antigravity por línea de órdenes; usa gemini"))
	}
	raiz, espacio, err := EspacioTemporal()
	if err != nil {
		return Resultado{}, err
	}
	defer os.RemoveAll(raiz)
	// HOME privado: agy no carga las reglas ni la sesión del usuario y su estado (conversaciones) se borra al
	// terminar; modelProvider gemini obliga a usar la clave (comprobado con una clave falsa).
	home := filepath.Join(raiz, "home")
	ajustes := filepath.Join(home, ".gemini", "antigravity-cli")
	if err := os.MkdirAll(ajustes, 0o700); err != nil {
		return Resultado{}, err
	}
	if err := os.WriteFile(filepath.Join(ajustes, "settings.json"), []byte(`{"modelProvider": "gemini"}`), 0o600); err != nil {
		return Resultado{}, err
	}
	args := []string{"-p", prompt, "--output-format", "json", "--disable-slash-commands"}
	if s.Modelo != "" {
		args = append(args, "--model", s.Modelo)
	}
	if len(s.Salida.Esquema) > 0 {
		archivo := filepath.Join(raiz, "esquema.json")
		if err := os.WriteFile(archivo, s.Salida.Esquema, 0o600); err != nil {
			return Resultado{}, err
		}
		args = append(args, "--json-schema", archivo)
	}
	var lineas []string
	r, err := procesos.Ejecutar(ctx, procesos.Orden{Programa: ruta, Argumentos: args, Directorio: DirectorioTrabajo(s, espacio),
		Entorno: EntornoLimpio([]string{"GEMINI_"}, "USERPROFILE="+home, "HOME="+home, "APPDATA="+filepath.Join(home, "AppData", "Roaming"),
			"LOCALAPPDATA="+filepath.Join(home, "AppData", "Local"), "XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
			"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"), "XDG_CACHE_HOME="+filepath.Join(home, ".cache"),
			"XDG_STATE_HOME="+filepath.Join(home, ".local", "state"), "GEMINI_API_KEY="+clave)}, func(linea string) error {
		lineas = append(lineas, linea)
		return nil
	})
	if err != nil {
		return Resultado{}, err
	}
	var final resultadoAntigravity
	if json.Unmarshal([]byte(strings.Join(lineas, "\n")), &final) != nil || final.Status == "" {
		texto := RedactarCon(strings.TrimSpace(r.Errores+"\n"+strings.Join(lineas, "\n")), clave)
		if texto == "" {
			texto = idioma.T("agy terminó sin resultado JSON")
		}
		return Resultado{}, nucleo.NuevoError(Clasificar(texto, 0), texto)
	}
	if final.Status != "SUCCESS" {
		texto := RedactarCon(strings.TrimSpace(final.Status+": "+final.Response+" "+r.Errores), clave)
		return Resultado{}, nucleo.NuevoError(Clasificar(texto, 0), texto)
	}
	salida := strings.TrimSpace(final.Response)
	if len(final.StructuredOutput) > 0 && string(final.StructuredOutput) != "null" {
		salida = string(final.StructuredOutput) // con texto libre añadía campos fuera del esquema
	}
	if err := emitir(Evento{Tipo: "texto", Datos: salida}); err != nil {
		return Resultado{}, err
	}
	return Resultado{Salida: salida, Uso: nucleo.Uso{
		TokensEntrada: final.Usage.InputTokens + final.Usage.CacheRead,
		TokensSalida:  final.Usage.OutputTokens, // incluye thinking_tokens (total = entrada + salida)
	}}, nil
}

type resultadoAntigravity struct {
	Status           string          `json:"status"`
	Response         string          `json:"response"`
	StructuredOutput json.RawMessage `json:"structured_output"`
	Usage            struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		CacheRead    int `json:"cache_read_tokens"`
	} `json:"usage"`
}

// unidadesLineaOrdenes cuenta las unidades UTF-16 que ocupa un argumento en la línea de órdenes de Windows tras
// escaparlo como syscall.EscapeArg: comillas si hay espacios, \" por cada comilla y las barras previas dobladas.
func unidadesLineaOrdenes(arg string) int {
	especial, espacio := false, false
	for _, r := range arg {
		switch r {
		case '"', '\\':
			especial = true
		case ' ', '\t':
			espacio = true
		}
	}
	n := 0
	for _, r := range arg {
		n += utf16.RuneLen(r)
	}
	if espacio {
		n += 2
	}
	if !especial {
		return n
	}
	barras := 0
	for _, r := range arg {
		switch r {
		case '\\':
			barras++
		case '"':
			n += barras + 1
			barras = 0
		default:
			barras = 0
		}
	}
	if espacio {
		n += barras // las barras finales se doblan antes de la comilla de cierre
	}
	return n
}
