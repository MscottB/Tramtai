package adaptadores

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// Claude ejecuta Claude Code con "claude -p" en streaming (stream-json), aislado de la configuración del usuario.
type Claude struct {
	Programa      string // nombre o ruta del ejecutable (comandos_agentes.claude)
	Autenticacion *Modo  // clave_api (predeterminada) o suscripcion; se puede cambiar con el servicio en marcha
}

// Detectar implementa Detectable.
func (c Claude) Detectar(ctx context.Context) Estado {
	e := Estado{Agente: "claude", Autenticacion: c.Autenticacion.Valor()}
	if c.Autenticacion.Valor() == AutenticacionSuscripcion {
		e.Terminos = idioma.T("términos de consumidor de Anthropic: solo uso personal y local")
	} else {
		e.Terminos = idioma.T("términos comerciales de Anthropic (clave de API de Console)")
		if claveEntorno("ANTHROPIC_API_KEY") == "" {
			e.Problema = idioma.T("falta ANTHROPIC_API_KEY")
		}
	}
	ruta, err := resolverPrograma(c.Programa)
	if err != nil {
		e.Problema = err.Error()
		return e
	}
	e.Ruta, e.Version = ruta, version(ctx, ruta)
	return e
}

// Ejecutar implementa Adaptador.
func (c Claude) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(Evento) error) (Resultado, error) {
	ruta, err := resolverPrograma(c.Programa)
	if err != nil {
		return Resultado{}, err
	}
	var clave string
	var extra []string
	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--setting-sources", "", "--strict-mcp-config", "--tools", "", "--no-session-persistence", "--disable-slash-commands"}
	switch c.Autenticacion.Valor() {
	case AutenticacionSuscripcion:
	case AutenticacionClaveAPI, "":
		if clave = claveEntorno("ANTHROPIC_API_KEY"); clave == "" {
			return Resultado{}, nucleo.NuevoError(nucleo.CodigoAutenticacionAgente,
				idioma.T("falta ANTHROPIC_API_KEY; o configura adaptadores.claude.autenticacion: suscripcion para uso personal local"))
		}
		extra = append(extra, "ANTHROPIC_API_KEY="+clave)
		if !consciente(s) {
			args = append(args, "--bare") // solo admite la clave de API: nunca recurre a la sesión de suscripción
		}
	default:
		return Resultado{}, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("autenticación desconocida: %s", c.Autenticacion.Valor()))
	}
	raiz, espacio, err := espacioTemporal()
	if err != nil {
		return Resultado{}, err
	}
	defer os.RemoveAll(raiz)
	if consciente(s) && clave != "" {
		// --bare omitiría el CLAUDE.md del proyecto; un directorio de configuración privado y vacío evita las reglas
		// del usuario y cualquier sesión guardada, así que solo vale la clave de API (sección 6.3).
		configuracion := filepath.Join(raiz, "claude-config")
		if err := os.Mkdir(configuracion, 0o700); err != nil {
			return Resultado{}, err
		}
		extra = append(extra, "CLAUDE_CONFIG_DIR="+configuracion)
	}

	// El sistema propio sustituye al de Claude Code (vacío si no hay): passthrough y ~650 tokens fijos en vez de ~24 000.
	sistema, prompt := separar(s)
	archivoSistema := filepath.Join(raiz, "sistema.txt")
	if err := os.WriteFile(archivoSistema, []byte(sistema), 0o600); err != nil {
		return Resultado{}, err
	}
	args = append(args, "--system-prompt-file", archivoSistema)
	if consciente(s) {
		args[slices.Index(args, "--setting-sources")+1] = "project" // reglas del proyecto, nunca las del usuario
	}
	if s.Modelo != "" {
		args = append(args, "--model", s.Modelo)
	}
	if v := valorEsfuerzo("claude", s.Esfuerzo); v != "" {
		args = append(args, "--effort", v)
	}
	if len(s.Salida.Esquema) > 0 {
		// Salida estructurada nativa: el resultado llega en structured_output (validado en M0).
		var compacto bytes.Buffer
		if err := json.Compact(&compacto, s.Salida.Esquema); err != nil {
			return Resultado{}, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("esquema no válido"))
		}
		args = append(args, "--json-schema", compacto.String())
	}

	var final *resultadoClaude
	r, err := procesos.Ejecutar(ctx, procesos.Orden{Programa: ruta, Argumentos: args, Directorio: directorioTrabajo(s, espacio),
		Entorno: entornoLimpio(extra...), Entrada: prompt}, func(linea string) error {
		var e eventoClaude
		if json.Unmarshal([]byte(linea), &e) != nil {
			return nil // línea informativa no JSON
		}
		switch e.Type {
		case "stream_event":
			if e.Event.Type == "content_block_delta" && e.Event.Delta.Type == "text_delta" && e.Event.Delta.Text != "" {
				return emitir(Evento{Tipo: "texto", Datos: e.Event.Delta.Text})
			}
		case "result":
			final = &e.resultadoClaude
		}
		return nil // los demás tipos son informativos (sección 9.4)
	})
	if err != nil {
		return Resultado{}, err
	}
	if final == nil {
		texto := redactar(r.Errores, clave)
		if texto == "" {
			return Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorProtocoloAgente, idioma.T("claude terminó sin evento result"))
		}
		return Resultado{}, nucleo.NuevoError(clasificar(texto, 0), texto)
	}
	if final.IsError {
		texto := redactar(final.Result, clave)
		return Resultado{}, nucleo.NuevoError(clasificar(texto, final.APIErrorStatus), texto)
	}
	salida := final.Result
	if len(final.StructuredOutput) > 0 && string(final.StructuredOutput) != "null" {
		salida = string(final.StructuredOutput) // el campo estructurado prevalece sobre el texto libre (sección 11)
	}
	return Resultado{Salida: salida, ModeloEfectivo: final.modeloEfectivo(), Uso: nucleo.Uso{
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
		OutputTokens int `json:"outputTokens"`
	} `json:"modelUsage"`
}

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
