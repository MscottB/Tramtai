package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"tramtai/internal/adaptadores"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// Codex ejecuta "codex exec --json". Solo entrega el resultado final (sin streaming por tokens, comprobado con el CLI real)
// y no informa el modelo efectivo: el trabajo queda como no verificable salvo con un identificador explícito.
type Codex struct {
	Programa      string            // comandos_agentes.codex
	Autenticacion *adaptadores.Modo // clave_api (predeterminada) o suscripcion (cuenta ChatGPT); cambiable en marcha
}

// Detectar implementa Detectable.
func (c Codex) Detectar(ctx context.Context) adaptadores.Estado {
	e := adaptadores.Estado{Agente: "codex", Autenticacion: c.Autenticacion.Valor()}
	if c.Autenticacion.Valor() == adaptadores.AutenticacionSuscripcion {
		e.Terminos = idioma.T("Terms of Use de OpenAI con cuenta ChatGPT: solo uso personal y local; aislamiento parcial")
		if inicio, err := os.UserHomeDir(); err == nil {
			if _, err := os.Stat(filepath.Join(inicio, ".codex", "AGENTS.md")); err == nil {
				e.Problema, e.ProblemaCodigo = idioma.T("existe ~/.codex/AGENTS.md: Codex lo cargará en cada ejecución"), adaptadores.ProblemaAviso
			}
		}
	} else {
		e.Terminos = idioma.T("términos de la API de OpenAI (clave de API)")
		if adaptadores.ClaveEntorno("CODEX_API_KEY", "OPENAI_API_KEY") == "" {
			e.Problema, e.ProblemaCodigo = idioma.T("falta CODEX_API_KEY u OPENAI_API_KEY"), adaptadores.ProblemaFaltaClave
		}
	}
	ruta, err := adaptadores.ResolverPrograma(c.Programa)
	if err != nil {
		e.Problema, e.ProblemaCodigo = err.Error(), adaptadores.ProblemaNoInstalado
		return e
	}
	e.Ruta, e.Version = ruta, adaptadores.VersionDe(ctx, ruta)
	if c.Autenticacion.Valor() == adaptadores.AutenticacionSuscripcion {
		adaptadores.AplicarSesion(&e, c.ComprobarSesion(ctx), "Codex", "codex login")
	}
	return e
}

// ComprobarSesion implementa adaptadores.ComprobadorSesion con codex login status, que no envía ningún mensaje ni dice
// la cuenta: «Logged in using ChatGPT» es la suscripción, «… an API key» una clave y «Not logged in», sin sesión.
func (c Codex) ComprobarSesion(ctx context.Context) adaptadores.SesionCLI {
	ruta, err := adaptadores.ResolverPrograma(c.Programa)
	if err != nil {
		return adaptadores.SesionCLI{Estado: adaptadores.SesionDesconocida}
	}
	salida, ok := adaptadores.SalidaEstado(ctx, ruta, "login", "status")
	switch {
	case !ok:
		return adaptadores.SesionCLI{Estado: adaptadores.SesionDesconocida}
	case strings.Contains(salida, "Not logged in"):
		return adaptadores.SesionCLI{Estado: adaptadores.SesionSin}
	case strings.Contains(salida, "API key"):
		return adaptadores.SesionCLI{Estado: adaptadores.SesionUsaClave}
	case strings.Contains(salida, "ChatGPT"):
		return adaptadores.SesionCLI{Estado: adaptadores.SesionOK}
	}
	return adaptadores.SesionCLI{Estado: adaptadores.SesionDesconocida}
}

// Ejecutar implementa Adaptador.
func (c Codex) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(adaptadores.Evento) error) (adaptadores.Resultado, error) {
	ruta, err := adaptadores.ResolverPrograma(c.Programa)
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	raiz, espacio, err := adaptadores.EspacioTemporal()
	if err != nil {
		return adaptadores.Resultado{}, err
	}
	defer os.RemoveAll(raiz)

	var clave string
	var extra []string
	switch c.Autenticacion.Valor() {
	case adaptadores.AutenticacionSuscripcion:
	case adaptadores.AutenticacionClaveAPI, "":
		var err error
		if clave, err = adaptadores.ClaveAlmacen("CODEX_API_KEY", "OPENAI_API_KEY"); err != nil {
			return adaptadores.Resultado{}, err
		}
		if clave == "" {
			return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoAutenticacionAgente,
				idioma.T("falta CODEX_API_KEY u OPENAI_API_KEY; o configura adaptadores.codex.autenticacion: suscripcion para uso personal local"))
		}
		// Un CODEX_HOME privado y vacío evita cargar ~/.codex/AGENTS.md: aislamiento completo.
		home := filepath.Join(raiz, "codex-home")
		if err := os.Mkdir(home, 0o700); err != nil {
			return adaptadores.Resultado{}, err
		}
		extra = append(extra, "CODEX_HOME="+home, "CODEX_API_KEY="+clave)
	default:
		return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("autenticación desconocida: %s", c.Autenticacion.Valor()))
	}

	trabajo := adaptadores.DirectorioTrabajo(s, espacio)
	args := []string{"exec", "--json", "--ephemeral", "--skip-git-repo-check", "-C", trabajo, "-s", "read-only",
		"-c", `approval_policy="never"`, "-c", "features.shell_tool=false", "-c", `web_search="disabled"`, "-c", `history.persistence="none"`}
	if !adaptadores.Consciente(s) {
		args = append(args, "--ignore-rules") // en modo consciente carga el AGENTS.md del proyecto
	}
	if s.Modelo != "" {
		args = append(args, "-m", s.Modelo)
	}
	if v := adaptadores.ValorEsfuerzo(valoresEsfuerzo, s.Esfuerzo); v != "" {
		args = append(args, "-c", `model_reasoning_effort="`+v+`"`)
	}
	if len(s.Salida.Esquema) > 0 {
		archivo := filepath.Join(raiz, "esquema.json")
		if err := os.WriteFile(archivo, s.Salida.Esquema, 0o600); err != nil {
			return adaptadores.Resultado{}, err
		}
		args = append(args, "--output-schema", archivo)
	}
	args = append(args, "-") // el prompt llega por stdin
	sistema, prompt := adaptadores.Separar(s)

	var mensajes []string
	var uso nucleo.Uso
	var errorAgente string
	terminado := false
	r, err := procesos.Ejecutar(ctx, procesos.Orden{Programa: ruta, Argumentos: args, Directorio: trabajo,
		Entorno: adaptadores.EntornoLimpio([]string{"CODEX_", "OPENAI_"}, extra...), Entrada: adaptadores.EnmarcarSistema(sistema, prompt)}, func(linea string) error {
		var e eventoCodex
		if json.Unmarshal([]byte(linea), &e) != nil {
			return nil
		}
		switch e.Type {
		case "item.completed":
			if e.Item.Type == "agent_message" && e.Item.Text != "" {
				mensajes = append(mensajes, e.Item.Text)
				return emitir(adaptadores.Evento{Tipo: "texto", Datos: e.Item.Text})
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
		return adaptadores.Resultado{}, err
	}
	if errorAgente != "" || !terminado {
		texto := adaptadores.RedactarCon(strings.TrimSpace(errorAgente+"\n"+r.Errores), clave)
		if texto == "" {
			return adaptadores.Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorProtocoloAgente, idioma.T("codex terminó sin turn.completed"))
		}
		return adaptadores.Resultado{}, nucleo.NuevoError(adaptadores.Clasificar(texto, 0), texto)
	}
	return adaptadores.Resultado{Salida: strings.Join(mensajes, "\n\n"), Uso: uso}, nil
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
