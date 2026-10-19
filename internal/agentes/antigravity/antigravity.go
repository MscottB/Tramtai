package antigravity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"tramtai/internal/adaptadores"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// valoresEsfuerzo son los de --effort de agy, que llega a max.
var valoresEsfuerzo = []string{"low", "medium", "high", "xhigh", "max"}

// Antigravity ejecuta agy con el mensaje por stdin (--input-format stream-json), solo con la Gemini API y la clave del
// usuario: los términos de Antigravity no admiten herramientas de terceros con la sesión de una cuenta personal.
// Un hook PreToolUse niega todas sus herramientas (comprobado con el CLI real: lectura, órdenes y web) y, por si una
// versión dejara de aplicarlo, Tramtai corta la ejecución si alguna llega a ejecutarse.
type Antigravity struct {
	Programa      string // nombre o ruta del ejecutable (comandos_agentes.antigravity)
	Autenticacion string // solo clave_api
}

// Detectar implementa Detectable.
func (a Antigravity) Detectar(ctx context.Context) adaptadores.Estado {
	e := adaptadores.Estado{Agente: "antigravity", Autenticacion: a.Autenticacion,
		Terminos: idioma.T("Antigravity con la Gemini API y tu clave (modelProvider gemini); nunca la sesión personal")}
	if err := a.autenticacion(); err != nil {
		e.Problema = nucleo.MensajeDe(err)
	} else if adaptadores.ClaveEntorno("GEMINI_API_KEY") == "" {
		e.Problema, e.ProblemaCodigo = idioma.T("falta GEMINI_API_KEY (tramtai claves guardar GEMINI_API_KEY)"), adaptadores.ProblemaFaltaClave
	}
	ruta, err := adaptadores.ResolverPrograma(a.Programa)
	if err != nil {
		e.Problema, e.ProblemaCodigo = err.Error(), adaptadores.ProblemaNoInstalado
		return e
	}
	e.Ruta, e.Version = ruta, adaptadores.VersionDe(ctx, ruta)
	return e
}

func (a Antigravity) autenticacion() error {
	if a.Autenticacion != "" && a.Autenticacion != adaptadores.AutenticacionClaveAPI {
		return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida,
			idioma.T("antigravity solo admite clave_api: los términos de Antigravity prohíben usar la sesión personal desde otra herramienta"))
	}
	return nil
}

// Ejecutar implementa Adaptador.
func (a Antigravity) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	if err := a.autenticacion(); err != nil {
		return adaptadores.Resultado{}, err
	}
	clave := adaptadores.ClaveEntorno("GEMINI_API_KEY")
	if clave == "" {
		return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoAutenticacionAgente, idioma.T("falta GEMINI_API_KEY (tramtai claves guardar GEMINI_API_KEY)"))
	}
	ruta, err := adaptadores.ResolverPrograma(a.Programa)
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	if adaptadores.Consciente(s) {
		return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoContextoNoPermitido, idioma.T("antigravity no admite consciente_proyecto: usa modo_contexto aislado"))
	}
	raiz, espacio, err := adaptadores.EspacioTemporal()
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	defer os.RemoveAll(raiz)
	home := filepath.Join(raiz, "home")
	if err := prepararCarpetas(raiz, home, espacio); err != nil {
		return adaptadores.Resultado{}, err
	}

	// Con --json-schema agy entrega el JSON con su herramienta «finish», que el hook niega: el formato va en las
	// instrucciones y Tramtai valida la respuesta, como con Copilot.
	sistema, prompt := adaptadores.Separar(s)
	formato := adaptadores.FormatoEnInstrucciones(s.Salida)
	if formato != "" {
		sistema = strings.TrimSpace(sistema + "\n\n" + formato)
	}
	entrada, err := json.Marshal(map[string]any{"event": "user", "message": map[string]string{"role": "user", "content": adaptadores.EnmarcarSistema(sistema, prompt)}})
	if err != nil {
		return adaptadores.Resultado{}, err
	}

	ctxCLI, cancelar := context.WithCancel(ctx)
	defer cancelar()
	e := estado{detener: cancelar}
	r, err := procesos.Ejecutar(ctxCLI, procesos.Orden{Programa: ruta, Argumentos: argumentos(s), Directorio: espacio,
		Entorno: adaptadores.EntornoLimpio([]string{"GEMINI_"}, "USERPROFILE="+home, "HOME="+home, "APPDATA="+filepath.Join(home, "AppData", "Roaming"),
			"LOCALAPPDATA="+filepath.Join(home, "AppData", "Local"), "XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
			"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"), "XDG_CACHE_HOME="+filepath.Join(home, ".cache"),
			"XDG_STATE_HOME="+filepath.Join(home, ".local", "state"), "GEMINI_API_KEY="+clave),
		Entrada: string(entrada) + "\n"}, func(linea string) error { return e.leer(linea, emitir) })
	if e.herramienta && ctx.Err() == nil {
		return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorAgente,
			idioma.T("antigravity usó una herramienta y se detuvo: Tramtai lo ejecuta sin herramientas"))
	}
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	if e.final == nil {
		texto := adaptadores.RedactarCon(strings.TrimSpace(r.Errores), clave)
		if texto == "" {
			texto = idioma.T("agy terminó sin resultado")
		}
		return adaptadores.Resultado{}, nucleo.NuevoError(adaptadores.Clasificar(texto, 0), texto)
	}
	if e.final.Status != "SUCCESS" {
		texto := adaptadores.RedactarCon(strings.TrimSpace(e.final.Status+": "+e.final.Error+" "+e.final.Response+" "+r.Errores), clave)
		return adaptadores.Resultado{}, nucleo.NuevoError(adaptadores.Clasificar(texto, 0), texto)
	}
	salida := strings.TrimSpace(e.final.Response)
	if formato != "" {
		salida = adaptadores.SinBloqueDeCodigo(salida)
	}
	return adaptadores.Resultado{Salida: salida, Uso: nucleo.Uso{
		TokensEntrada: e.final.Usage.InputTokens + e.final.Usage.CacheRead,
		TokensSalida:  e.final.Usage.OutputTokens,
	}}, nil
}

// argumentos: el mensaje llega por stdin, así que ningún argumento depende de lo que manda la app.
func argumentos(s nucleo.SolicitudEjecucion) []string {
	args := []string{"--input-format", "stream-json", "--output-format", "stream-json", "--disable-slash-commands"}
	if s.Modelo != "" {
		args = append(args, "--model", s.Modelo)
	}
	if v := adaptadores.ValorEsfuerzo(valoresEsfuerzo, s.Esfuerzo); v != "" {
		args = append(args, "--effort", v)
	}
	// -p vacío al final: con un valor aparte, agy tomaría la opción siguiente como mensaje.
	return append(args, "-p=")
}

// ordenDelHook es la ruta del guion tal como la ejecutará agy: sin comillas (llegan mal escapadas a cmd) y, por eso,
// solo con letras, números y _ - . : ~ \ /, sin espacios ni nada que cmd o sh interpreten (& ^ % ! ( ) …). En Windows se
// usa su nombre corto (C:\Users\ANAMAR~1\…) si la carpeta temporal tiene espacios. Si aun así queda algo fuera de lo
// seguro, no se ejecuta: sin el hook, Antigravity podría usar herramientas.
func ordenDelHook(guion string) (string, error) {
	orden := rutaCorta(guion)
	if strings.IndexFunc(orden, func(r rune) bool { return !segura(r) }) >= 0 {
		return "", nucleo.NuevoError(nucleo.CodigoAgenteNoDisponible, idioma.Tf(
			"la carpeta temporal tiene espacios (%s) y Antigravity no ejecutaría el hook que le niega las herramientas: define TMP con una carpeta sin espacios", filepath.Dir(orden)))
	}
	return orden, nil
}

// segura dice si una letra de la ruta del hook llega igual a cmd y a sh sin comillas.
func segura(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(`_-.:~\/`, r)
}

// prepararCarpetas crea el HOME privado con modelProvider gemini y el hook que niega todas las herramientas. El hook
// está en la carpeta de trabajo (.agents/hooks.json, que agy carga aunque no sea de confianza) y su orden es un
// archivo fuera de ella: una orden con comillas llega mal escapada a cmd en Windows.
func prepararCarpetas(raiz, home, espacio string) error {
	ajustes := filepath.Join(home, ".gemini", "antigravity-cli")
	agentes := filepath.Join(espacio, ".agents")
	for _, dir := range []string{ajustes, agentes} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(ajustes, "settings.json"), []byte(`{"modelProvider": "gemini"}`), 0o600); err != nil {
		return err
	}
	decision := `{"decision":"deny","reason":"Tramtai ejecuta Antigravity sin herramientas"}`
	guion, contenido := filepath.Join(raiz, "denegar.sh"), "#!/bin/sh\nprintf '%s\\n' '"+decision+"'\n"
	if runtime.GOOS == "windows" {
		guion, contenido = filepath.Join(raiz, "denegar.cmd"), "@echo "+decision+"\r\n"
	}
	if err := os.WriteFile(guion, []byte(contenido), 0o700); err != nil {
		return err
	}
	orden, err := ordenDelHook(guion)
	if err != nil {
		return err
	}
	hooks, err := json.Marshal(map[string]any{"tramtai-sin-herramientas": map[string]any{"PreToolUse": []any{
		map[string]any{"matcher": "*", "hooks": []any{map[string]any{"type": "command", "command": orden, "timeout": 10}}}}}})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(agentes, "hooks.json"), hooks, 0o600)
}

// estado es lo que se va leyendo del stream-json de agy.
type estado struct {
	final       *resultadoAntigravity
	herramienta bool
	detener     func()
}

func (e *estado) leer(linea string, emitir func(adaptadores.Evento) error) error {
	if e.herramienta {
		return nil // tras una herramienta no se emite nada: el texto podría llevar lo que obtuvo
	}
	var ev struct {
		Event string `json:"event"`
		Paso  struct {
			Tipo      string `json:"step_type"`
			Estado    string `json:"state"`
			TextDelta string `json:"text_delta"`
		} `json:"step_update"`
		Result *resultadoAntigravity `json:"result"`
	}
	if json.Unmarshal([]byte(linea), &ev) != nil {
		return nil // línea informativa no JSON
	}
	switch ev.Event {
	case "step_update":
		switch {
		case ev.Paso.Tipo == "tool" && ev.Paso.Estado == "DONE":
			// El hook la niega (estado ERROR): si una llega a completarse, la ejecución se corta.
			e.herramienta = true
			e.detener()
		case ev.Paso.Tipo == "agent_response" && ev.Paso.TextDelta != "":
			return emitir(adaptadores.Evento{Tipo: "texto", Datos: ev.Paso.TextDelta})
		}
	case "result":
		e.final = ev.Result
	}
	return nil
}

type resultadoAntigravity struct {
	Status   string `json:"status"`
	Response string `json:"response"`
	Error    string `json:"error"`
	Usage    struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
		CacheRead    int `json:"cache_read_tokens"`
	} `json:"usage"`
}
