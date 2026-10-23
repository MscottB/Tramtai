package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"tramtai/internal/adaptadores"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// propiosClaude son los prefijos de la configuración de Claude Code que su CLI sí recibe.
var propiosClaude = []string{"CLAUDE_", "ANTHROPIC_"}

// Claude ejecuta Claude Code con "claude -p" en streaming (stream-json), aislado de la configuración del usuario.
type Claude struct {
	Programa      string            // nombre o ruta del ejecutable (comandos_agentes.claude)
	Autenticacion *adaptadores.Modo // clave_api (predeterminada) o suscripcion; se puede cambiar con el servicio en marcha
}

func (c Claude) Detectar(ctx context.Context) adaptadores.Estado {
	e := adaptadores.Estado{Agente: "claude", Autenticacion: c.Autenticacion.Valor()}
	if c.Autenticacion.Valor() == adaptadores.AutenticacionSuscripcion {
		e.Terminos = idioma.T("términos de consumidor de Anthropic: solo uso personal y local")
	} else {
		e.Terminos = idioma.T("términos comerciales de Anthropic (clave de API de Console)")
		if adaptadores.ClaveEntorno("ANTHROPIC_API_KEY") == "" {
			e.Problema, e.ProblemaCodigo = idioma.T("falta ANTHROPIC_API_KEY"), adaptadores.ProblemaFaltaClave
		}
	}
	ruta, err := adaptadores.ResolverPrograma(c.Programa)
	if err != nil {
		e.Problema, e.ProblemaCodigo = err.Error(), adaptadores.ProblemaNoInstalado
		return e
	}
	e.Ruta, e.Version = ruta, adaptadores.VersionDe(ctx, ruta)
	if c.Autenticacion.Valor() == adaptadores.AutenticacionSuscripcion {
		adaptadores.AplicarSesion(&e, c.ComprobarSesion(ctx), "Claude Code", "claude auth login")
	}
	return e
}

// ComprobarSesion usa claude auth status --json, que lee su configuración y no
// envía ningún mensaje: loggedIn falso es sin sesión y un authMethod distinto de claude.ai, una clave de API.
func (c Claude) ComprobarSesion(ctx context.Context) adaptadores.SesionCLI {
	ruta, err := adaptadores.ResolverPrograma(c.Programa)
	if err != nil {
		return adaptadores.SesionCLI{Estado: adaptadores.SesionDesconocida}
	}
	salida, ok := adaptadores.SalidaEstado(ctx, ruta, "auth", "status", "--json")
	var estado struct {
		LoggedIn   bool   `json:"loggedIn"`
		AuthMethod string `json:"authMethod"`
		Email      string `json:"email"`
	}
	if !ok || json.Unmarshal([]byte(salida), &estado) != nil {
		return adaptadores.SesionCLI{Estado: adaptadores.SesionDesconocida}
	}
	switch {
	case !estado.LoggedIn:
		return adaptadores.SesionCLI{Estado: adaptadores.SesionSin}
	case estado.AuthMethod != "claude.ai":
		return adaptadores.SesionCLI{Estado: adaptadores.SesionUsaClave, Cuenta: estado.Email}
	}
	return adaptadores.SesionCLI{Estado: adaptadores.SesionOK, Cuenta: estado.Email}
}

// ResolverModelo: Claude Code informa el modelo concreto en su evento system/init, antes de
// enviar la petición, y en cuanto llega se le corta. shortcut: si el CLI tarda en morir puede llegar a enviar un
// mensaje de un carácter; si alguna vez cobra, pedirlo solo al abrir el selector.
func (c Claude) ResolverModelo(ctx context.Context, modelo string) (string, error) {
	ruta, err := adaptadores.ResolverPrograma(c.Programa)
	if err != nil {
		return "", err
	}
	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--setting-sources", "", "--strict-mcp-config",
		"--tools", "", "--no-session-persistence", "--disable-slash-commands"}
	var extra []string
	if c.Autenticacion.Valor() != adaptadores.AutenticacionSuscripcion {
		clave, err := adaptadores.ClaveAlmacen("ANTHROPIC_API_KEY")
		if err != nil {
			return "", err
		}
		if clave == "" {
			return "", nucleo.NuevoError(nucleo.CodigoAutenticacionAgente, idioma.T("falta ANTHROPIC_API_KEY"))
		}
		extra = append(extra, "ANTHROPIC_API_KEY="+clave)
		args = append(args, "--bare")
	}
	if modelo != "" {
		args = append(args, "--model", modelo)
	}
	raiz, espacio, err := adaptadores.EspacioTemporal()
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(raiz)
	ctx, cortar := context.WithCancel(ctx)
	defer cortar()
	var concreto string
	_, err = procesos.Ejecutar(ctx, procesos.Orden{Programa: ruta, Argumentos: args, Directorio: espacio,
		Entorno: adaptadores.EntornoLimpio(propiosClaude, extra...), Entrada: "."}, func(linea string) error {
		var e struct{ Type, Subtype, Model string }
		if json.Unmarshal([]byte(linea), &e) == nil && e.Type == "system" && e.Subtype == "init" {
			concreto = e.Model
			cortar()
		}
		return nil
	})
	if concreto != "" {
		return concreto, nil
	}
	return "", err
}

func (c Claude) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	ruta, err := adaptadores.ResolverPrograma(c.Programa)
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	var clave string
	var extra []string
	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--setting-sources", "", "--strict-mcp-config", "--tools", "", "--no-session-persistence", "--disable-slash-commands"}
	switch c.Autenticacion.Valor() {
	case adaptadores.AutenticacionSuscripcion:
	case adaptadores.AutenticacionClaveAPI, "":
		var err error
		if clave, err = adaptadores.ClaveAlmacen("ANTHROPIC_API_KEY"); err != nil {
			return adaptadores.Resultado{}, err
		}
		if clave == "" {
			return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoAutenticacionAgente,
				idioma.T("falta ANTHROPIC_API_KEY; o configura adaptadores.claude.autenticacion: suscripcion para uso personal local"))
		}
		extra = append(extra, "ANTHROPIC_API_KEY="+clave)
		if !adaptadores.Consciente(s) {
			args = append(args, "--bare") // solo admite la clave de API: nunca recurre a la sesión de suscripción
		}
	default:
		return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("autenticación desconocida: %s", c.Autenticacion.Valor()))
	}
	raiz, espacio, err := adaptadores.EspacioTemporal()
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	defer os.RemoveAll(raiz)
	if adaptadores.Consciente(s) && clave != "" {
		// --bare omitiría el CLAUDE.md del proyecto; un directorio de configuración privado y vacío evita las reglas
		// del usuario y cualquier sesión guardada, así que solo vale la clave de API.
		configuracion := filepath.Join(raiz, "claude-config")
		if err := os.Mkdir(configuracion, 0o700); err != nil {
			return adaptadores.Resultado{}, err
		}
		extra = append(extra, "CLAUDE_CONFIG_DIR="+configuracion)
	}

	// El sistema propio sustituye al de Claude Code (vacío si no hay): passthrough y ~650 tokens fijos en vez de ~24 000.
	sistema, prompt := adaptadores.Separar(s)
	archivoSistema := filepath.Join(raiz, "sistema.txt")
	if err := os.WriteFile(archivoSistema, []byte(sistema), 0o600); err != nil {
		return adaptadores.Resultado{}, err
	}
	args = append(args, "--system-prompt-file", archivoSistema)
	if adaptadores.Consciente(s) {
		args[slices.Index(args, "--setting-sources")+1] = "project" // reglas del proyecto, nunca las del usuario
	}
	if s.Modelo != "" {
		args = append(args, "--model", s.Modelo)
	}
	if v := adaptadores.ValorEsfuerzo(valoresEsfuerzo, s.Esfuerzo); v != "" {
		args = append(args, "--effort", v)
	}
	if len(s.Salida.Esquema) > 0 {
		// Salida estructurada nativa: el resultado llega en structured_output (comprobado con el CLI real).
		var compacto bytes.Buffer
		if err := json.Compact(&compacto, s.Salida.Esquema); err != nil {
			return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("esquema no válido"))
		}
		args = append(args, "--json-schema", compacto.String())
	}

	var final *resultadoClaude
	r, err := procesos.Ejecutar(ctx, procesos.Orden{Programa: ruta, Argumentos: args, Directorio: adaptadores.DirectorioTrabajo(s, espacio),
		Entorno: adaptadores.EntornoLimpio(propiosClaude, extra...), Entrada: prompt}, func(linea string) error {
		var e eventoClaude
		if json.Unmarshal([]byte(linea), &e) != nil {
			return nil // línea informativa no JSON
		}
		switch e.Type {
		case "stream_event":
			if e.Event.Type == "content_block_delta" && e.Event.Delta.Type == "text_delta" && e.Event.Delta.Text != "" {
				return emitir(adaptadores.Evento{Tipo: "texto", Datos: e.Event.Delta.Text})
			}
		case "result":
			final = &e.resultadoClaude
		}
		return nil // los demás tipos son informativos
	})
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	if final == nil {
		texto := adaptadores.RedactarCon(r.Errores, clave)
		if texto == "" {
			return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorProtocoloAgente, idioma.T("claude terminó sin evento result"))
		}
		return adaptadores.Resultado{}, nucleo.NuevoError(adaptadores.Clasificar(texto, 0), texto)
	}
	if final.IsError {
		texto := adaptadores.RedactarCon(final.Result, clave)
		return adaptadores.Resultado{}, nucleo.NuevoError(adaptadores.Clasificar(texto, final.APIErrorStatus), texto)
	}
	salida := final.Result
	if len(final.StructuredOutput) > 0 && string(final.StructuredOutput) != "null" {
		salida = string(final.StructuredOutput) // el campo estructurado prevalece sobre el texto libre
	}
	return adaptadores.Resultado{Salida: salida, ModeloEfectivo: final.modeloEfectivo(), ContextoTokens: final.contexto(), Uso: nucleo.Uso{
		TokensEntrada: final.Usage.InputTokens + final.Usage.CacheCreation + final.Usage.CacheRead,
		TokensSalida:  final.Usage.OutputTokens,
		CostoUSD:      final.TotalCostUSD,
	}}, nil
}

type eventoClaude struct {
	Type  string `json:"type"`
	Event struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
	resultadoClaude
}

type resultadoClaude struct {
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
	IsError          bool            `json:"is_error"`
	APIErrorStatus   int             `json:"api_error_status"`
	TotalCostUSD     float64         `json:"total_cost_usd"`
	Usage            struct {
		InputTokens   int `json:"input_tokens"`
		OutputTokens  int `json:"output_tokens"`
		CacheCreation int `json:"cache_creation_input_tokens"`
		CacheRead     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	ModelUsage map[string]struct {
		OutputTokens  int `json:"outputTokens"`
		ContextWindow int `json:"contextWindow"`
	} `json:"modelUsage"`
}

// contexto es el contexto que Claude Code informa para el modelo efectivo; 0 si no lo informa.
func (r resultadoClaude) contexto() int { return r.ModelUsage[r.modeloEfectivo()].ContextWindow }

// modeloEfectivo es el modelo que más salida produjo; Claude Code puede usar uno auxiliar en paralelo.
func (r resultadoClaude) modeloEfectivo() string {
	modelos := make([]string, 0, len(r.ModelUsage))
	for m := range r.ModelUsage {
		modelos = append(modelos, m)
	}
	sort.Slice(modelos, func(i, j int) bool {
		a, b := r.ModelUsage[modelos[i]].OutputTokens, r.ModelUsage[modelos[j]].OutputTokens
		return a > b || (a == b && modelos[i] < modelos[j])
	})
	if len(modelos) == 0 {
		return ""
	}
	return modelos[0]
}
