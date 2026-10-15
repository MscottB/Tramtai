package adaptadores

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// Codex ejecuta "codex exec --json". Solo entrega el resultado final (sin streaming por tokens, comprobado con el CLI real)
// y no informa el modelo efectivo: el trabajo queda como no verificable salvo con un identificador explícito.
type Codex struct {
	Programa      string // comandos_agentes.codex
	Autenticacion *Modo  // clave_api (predeterminada) o suscripcion (cuenta ChatGPT); cambiable en marcha
}

// Detectar implementa Detectable.
func (c Codex) Detectar(ctx context.Context) Estado {
	e := Estado{Agente: "codex", Autenticacion: c.Autenticacion.Valor()}
	if c.Autenticacion.Valor() == AutenticacionSuscripcion {
		e.Terminos = idioma.T("Terms of Use de OpenAI con cuenta ChatGPT: solo uso personal y local; aislamiento parcial")
		if inicio, err := os.UserHomeDir(); err == nil {
			if _, err := os.Stat(filepath.Join(inicio, ".codex", "AGENTS.md")); err == nil {
				e.Problema = idioma.T("existe ~/.codex/AGENTS.md: Codex lo cargará en cada ejecución")
			}
		}
	} else {
		e.Terminos = idioma.T("términos de la API de OpenAI (clave de API)")
		if claveEntorno("CODEX_API_KEY", "OPENAI_API_KEY") == "" {
			e.Problema, e.ProblemaCodigo = idioma.T("falta CODEX_API_KEY u OPENAI_API_KEY"), ProblemaFaltaClave
		}
	}
	ruta, err := resolverPrograma(c.Programa)
	if err != nil {
		e.Problema, e.ProblemaCodigo = err.Error(), ProblemaNoInstalado
		return e
	}
	e.Ruta, e.Version = ruta, version(ctx, ruta)
	return e
}

// Ejecutar implementa Adaptador.
func (c Codex) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(Evento) error) (Resultado, error) {
	ruta, err := resolverPrograma(c.Programa)
	if err != nil {
		return Resultado{}, err
	}
	raiz, espacio, err := espacioTemporal()
	if err != nil {
		return Resultado{}, err
	}
	defer os.RemoveAll(raiz)

	var clave string
	var extra []string
	switch c.Autenticacion.Valor() {
	case AutenticacionSuscripcion:
	case AutenticacionClaveAPI, "":
		if clave = claveEntorno("CODEX_API_KEY", "OPENAI_API_KEY"); clave == "" {
			return Resultado{}, nucleo.NuevoError(nucleo.CodigoAutenticacionAgente,
				idioma.T("falta CODEX_API_KEY u OPENAI_API_KEY; o configura adaptadores.codex.autenticacion: suscripcion para uso personal local"))
		}
		// Un CODEX_HOME privado y vacío evita cargar ~/.codex/AGENTS.md: aislamiento completo.
		home := filepath.Join(raiz, "codex-home")
		if err := os.Mkdir(home, 0o700); err != nil {
			return Resultado{}, err
		}
		extra = append(extra, "CODEX_HOME="+home, "CODEX_API_KEY="+clave)
	default:
		return Resultado{}, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("autenticación desconocida: %s", c.Autenticacion.Valor()))
	}

	trabajo := directorioTrabajo(s, espacio)
	args := []string{"exec", "--json", "--ephemeral", "--skip-git-repo-check", "-C", trabajo, "-s", "read-only",
		"-c", `approval_policy="never"`, "-c", "features.shell_tool=false", "-c", `web_search="disabled"`, "-c", `history.persistence="none"`}
	if !consciente(s) {
		args = append(args, "--ignore-rules") // en modo consciente carga el AGENTS.md del proyecto
	}
	if s.Modelo != "" {
		args = append(args, "-m", s.Modelo)
	}
	if v := valorEsfuerzo("codex", s.Esfuerzo); v != "" {
		args = append(args, "-c", `model_reasoning_effort="`+v+`"`)
	}
	if len(s.Salida.Esquema) > 0 {
		archivo := filepath.Join(raiz, "esquema.json")
		if err := os.WriteFile(archivo, s.Salida.Esquema, 0o600); err != nil {
			return Resultado{}, err
		}
		args = append(args, "--output-schema", archivo)
	}
	args = append(args, "-") // el prompt llega por stdin
	sistema, prompt := separar(s)

	var mensajes []string
	var uso nucleo.Uso
	var errorAgente string
	terminado := false
	r, err := procesos.Ejecutar(ctx, procesos.Orden{Programa: ruta, Argumentos: args, Directorio: trabajo,
		Entorno: entornoLimpio(extra...), Entrada: enmarcarSistema(sistema, prompt)}, func(linea string) error {
		var e eventoCodex
		if json.Unmarshal([]byte(linea), &e) != nil {
			return nil
		}
		switch e.Type {
		case "item.completed":
			if e.Item.Type == "agent_message" && e.Item.Text != "" {
				mensajes = append(mensajes, e.Item.Text)
				return emitir(Evento{Tipo: "texto", Datos: e.Item.Text})
			}
		case "turn.completed":
			terminado = true
			uso = nucleo.Uso{TokensEntrada: e.Usage.InputTokens, TokensSalida: e.Usage.OutputTokens}
		case "turn.failed":
			errorAgente = e.Error.Message
		case "error":
			errorAgente = e.Message
		}
		return nil
	})
	if err != nil {
		return Resultado{}, err
	}
	if errorAgente != "" || !terminado {
		texto := redactar(strings.TrimSpace(errorAgente+"\n"+r.Errores), clave)
		if texto == "" {
			return Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorProtocoloAgente, idioma.T("codex terminó sin turn.completed"))
		}
		return Resultado{}, nucleo.NuevoError(clasificar(texto, 0), texto)
	}
	return Resultado{Salida: strings.Join(mensajes, "\n\n"), Uso: uso}, nil
}

type eventoCodex struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Item    struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}
