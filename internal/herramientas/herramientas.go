// Package herramientas emula las herramientas (tools) de la aplicación para los agentes CLI, que no las tienen. Le
// describe al agente las funciones, le pide la respuesta como un sobre JSON {"content", "tool_calls"} (con la salida
// estructurada nativa de cada CLI) y lo traduce a llamadas. El agente nunca ejecuta nada: solo pide la llamada, y la
// aplicación devuelve el resultado en el siguiente turno. Lo usan las tres APIs: la propia y las fachadas OpenAI y
// Anthropic.
package herramientas

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/salidas"
)

// Activas indica si el agente puede pedir herramientas: las hay y la elección no es «ninguna».
func Activas(sol nucleo.SolicitudEjecucion) bool {
	return len(sol.Herramientas) > 0 && sol.EleccionHerramienta.Modo != "ninguna"
}

func conParametros(h nucleo.Herramienta) bool {
	return len(h.Parametros) > 0 && string(h.Parametros) != "null"
}

// Topes de las herramientas de una petición (los de OpenAI): cuántas, su nombre y el tamaño de todos sus esquemas, que
// se compilan al aceptar y van en las instrucciones del agente.
const maximoHerramientas = 128

var reNombre = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Validar comprueba las herramientas, la elección y que cada resultado responda a una llamada anterior.
func Validar(sol nucleo.SolicitudEjecucion) error {
	invalida := func(texto string) error { return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, texto) }
	if len(sol.Herramientas) > maximoHerramientas {
		return invalida(idioma.Tf("demasiadas herramientas (máximo %d)", maximoHerramientas))
	}
	total := 0
	for _, h := range sol.Herramientas {
		total += len(h.Parametros)
	}
	if total > nucleo.MaximoEsquemaPeticion {
		return invalida(idioma.T("los parámetros de las herramientas superan 256 KB"))
	}
	e := sol.EleccionHerramienta
	if len(sol.Herramientas) == 0 && (e.Modo == "obligatoria" || e.Modo == "una") {
		return invalida(idioma.T("se exige llamar a una herramienta, pero no se ofreció ninguna"))
	}
	var nombres []string
	for i, h := range sol.Herramientas {
		if h.Nombre == "" {
			return invalida(idioma.Tf("la herramienta %d no tiene nombre", i))
		}
		if !reNombre.MatchString(h.Nombre) {
			return invalida(idioma.Tf("nombre de herramienta no válido: %s (letras, números, _ y -, hasta 64)", h.Nombre))
		}
		if slices.Contains(nombres, h.Nombre) {
			return invalida(idioma.Tf("herramienta repetida: %s", h.Nombre))
		}
		if conParametros(h) {
			if _, err := salidas.Compilar(h.Parametros); err != nil {
				return invalida(idioma.Tf("los parámetros de la herramienta %s no son un JSON Schema válido", h.Nombre))
			}
		}
		nombres = append(nombres, h.Nombre)
	}
	if e.Modo == "una" && !slices.Contains(nombres, e.Nombre) {
		return invalida(idioma.Tf("se pide una herramienta que no se ofreció: %s", e.Nombre))
	}
	llamadas := map[string]bool{}
	for i, m := range sol.Mensajes {
		for _, l := range m.Llamadas {
			llamadas[l.ID] = true
		}
		if m.Rol == "herramienta" && !llamadas[m.IDLlamada] {
			return invalida(idioma.Tf("mensaje %d: id_llamada %s no corresponde a ninguna llamada anterior", i, m.IDLlamada))
		}
	}
	if Activas(sol) && (sol.Salida.Formato == "json" || len(sol.Salida.Esquema) > 0) {
		return nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.T("no se pueden combinar herramientas con una respuesta en JSON"))
	}
	return nil
}

// Preparar arma lo que recibe el agente: los turnos con llamadas y resultados anteriores como texto que entiende y,
// si puede pedir herramientas, su descripción en las instrucciones y el sobre JSON como esquema de salida. No
// modifica la solicitud recibida.
func Preparar(sol nucleo.SolicitudEjecucion) nucleo.SolicitudEjecucion {
	conTurnos := slices.ContainsFunc(sol.Mensajes, func(m nucleo.Mensaje) bool { return len(m.Llamadas) > 0 || m.Rol == "herramienta" })
	if conTurnos {
		nombres := map[string]string{} // id de llamada → herramienta, para nombrar sus resultados
		mensajes := make([]nucleo.Mensaje, len(sol.Mensajes))
		for i, m := range sol.Mensajes {
			switch {
			case len(m.Llamadas) > 0:
				for _, l := range m.Llamadas {
					nombres[l.ID] = l.Nombre
				}
				m = nucleo.Mensaje{Rol: m.Rol, Contenido: sobreLlamadas(m.Contenido, m.Llamadas)}
			case m.Rol == "herramienta":
				m = nucleo.Mensaje{Rol: "usuario", Contenido: turnoResultado(nombres[m.IDLlamada], m.IDLlamada, m.Contenido, m.EsError)}
			}
			mensajes[i] = m
		}
		sol.Mensajes = mensajes
	}
	if !Activas(sol) {
		return sol
	}
	nombres := make([]string, 0, len(sol.Herramientas))
	for _, h := range sol.Herramientas {
		nombres = append(nombres, h.Nombre)
	}
	if sol.EleccionHerramienta.Modo == "una" {
		nombres = []string{sol.EleccionHerramienta.Nombre}
	}
	// Sobre compatible con el modo estricto de Codex: todo obligatorio, sin propiedades extra y los argumentos como
	// texto, porque el esquema de cada herramienta puede no ser estricto. Tramtai los valida después (Interpretar).
	sobre, _ := json.Marshal(map[string]any{
		"type": "object", "additionalProperties": false, "required": []string{"content", "tool_calls"},
		"properties": map[string]any{
			"content": map[string]any{"type": "string"},
			"tool_calls": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "additionalProperties": false, "required": []string{"name", "arguments"},
				"properties": map[string]any{
					"name":      map[string]any{"type": "string", "enum": nombres},
					"arguments": map[string]any{"type": "string"},
				}}},
		}})
	sol.Salida.Formato, sol.Salida.Esquema = "json", sobre
	sol.Instrucciones = strings.TrimSpace(sol.Instrucciones + "\n\n" + instrucciones(sol))
	return sol
}

// soloUna indica si cabe solo una llamada por respuesta (llamadas_paralelas en false).
func soloUna(sol nucleo.SolicitudEjecucion) bool {
	return sol.LlamadasParalelas != nil && !*sol.LlamadasParalelas
}

// instrucciones explica al agente cómo pedir herramientas. Va en inglés, que los modelos siguen mejor, y no depende
// del idioma de la interfaz.
func instrucciones(sol nucleo.SolicitudEjecucion) string {
	var b strings.Builder
	b.WriteString("The application calling you can run tools for you. You cannot run them yourself: you request them, " +
		"and the application sends back their results in a later turn.\n\nAvailable tools:\n")
	for _, h := range sol.Herramientas {
		parametros := `{"type":"object","properties":{}}`
		if conParametros(h) {
			var compacto bytes.Buffer
			if json.Compact(&compacto, h.Parametros) == nil {
				parametros = compacto.String()
			}
		}
		fmt.Fprintf(&b, "- %s: %s\n  Arguments (JSON Schema): %s\n", h.Nombre, strings.TrimSpace(h.Descripcion), parametros)
	}
	b.WriteString("\nReply ONLY with a JSON object with two fields:\n" +
		`- "content": your message for the user ("" if you only call tools).` + "\n" +
		`- "tool_calls": the tools to run, each {"name": tool name, "arguments": a JSON object encoded as a string that satisfies the tool's schema}; [] if you need none.` + "\n")
	switch sol.EleccionHerramienta.Modo {
	case "obligatoria":
		b.WriteString("You must call at least one tool.\n")
	case "una":
		fmt.Fprintf(&b, "You must call the tool %s.\n", sol.EleccionHerramienta.Nombre)
	}
	if soloUna(sol) {
		b.WriteString("Call at most one tool per reply.\n")
	}
	b.WriteString(`Earlier tool calls appear in assistant turns with this same format, and their results in turns that start with "Tool result". ` +
		`Call a tool only when you still need information you don't have: when the results are already in the conversation, ` +
		`use them to answer and never repeat those calls.`)
	return b.String()
}

// Interpretar lee el sobre del agente: el texto para el usuario y las llamadas, con los argumentos validados contra el
// esquema de su herramienta. prefijo da los identificadores (call_…, toolu_…, llamada_…). Sin herramientas activas,
// devuelve la salida tal cual. Un sobre que no cumple es salida_no_cumple_esquema con sus problemas, para que la
// corrección se los pase al agente.
func Interpretar(sol nucleo.SolicitudEjecucion, salida, prefijo string) (string, []nucleo.Llamada, error) {
	if !Activas(sol) {
		return salida, nil, nil
	}
	// La forma del sobre primero, con los problemas detallados para la corrección (los CLI que no fuerzan el esquema
	// pueden responder otra cosa); después, cada llamada.
	if err := salidas.Validar(formaSobre, salida); err != nil {
		return "", nil, err
	}
	noCumple := func(texto string) error {
		return salidas.NoCumple([]salidas.Problema{{Problema: "otro", Texto: texto}})
	}
	var sobre struct {
		Content   string `json:"content"`
		ToolCalls []struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(salida), &sobre); err != nil {
		return "", nil, noCumple(idioma.T("el agente no respondió con el formato de herramientas"))
	}
	var llamadas []nucleo.Llamada
	for i, c := range sobre.ToolCalls {
		indice := slices.IndexFunc(sol.Herramientas, func(h nucleo.Herramienta) bool { return h.Nombre == c.Name })
		if indice < 0 {
			return "", nil, noCumple(idioma.Tf("el agente pidió una herramienta que no existe: %s", c.Name))
		}
		argumentos, err := nucleo.ObjetoJSON(c.Arguments)
		if err != nil {
			return "", nil, noCumple(idioma.Tf("los argumentos de %s no son un objeto JSON", c.Name))
		}
		if pedida := sol.Herramientas[indice]; conParametros(pedida) {
			if err := salidas.Validar(pedida.Parametros, argumentos); err != nil {
				return "", nil, noCumple(idioma.Tf("los argumentos de %s no cumplen su esquema: %s", c.Name, nucleo.MensajeDe(err)))
			}
		}
		llamadas = append(llamadas, nucleo.Llamada{ID: fmt.Sprintf("%s_%d", prefijo, i), Nombre: c.Name, Argumentos: json.RawMessage(argumentos)})
	}
	if modo := sol.EleccionHerramienta.Modo; len(llamadas) == 0 && (modo == "obligatoria" || modo == "una") {
		return "", nil, noCumple(idioma.T("se exigía llamar a una herramienta y el agente no lo hizo"))
	}
	if soloUna(sol) && len(llamadas) > 1 {
		llamadas = llamadas[:1]
	}
	return sobre.Content, llamadas, nil
}

// formaSobre es la forma que se exige al sobre del agente: más flexible que el esquema estricto de Preparar (admite los
// argumentos como objeto y campos de más), porque no todos los CLI fuerzan ese esquema.
var formaSobre = []byte(`{"type":"object","required":["content","tool_calls"],"properties":{"content":{"type":"string"},` +
	`"tool_calls":{"type":"array","items":{"type":"object","required":["name","arguments"],` +
	`"properties":{"name":{"type":"string"},"arguments":{"type":["string","object"]}}}}}}`)

// llamadaSobre es una llamada con la forma del sobre que conoce el agente: los argumentos como texto.
type llamadaSobre struct {
	ID         string `json:"id,omitempty"`
	Nombre     string `json:"name"`
	Argumentos string `json:"arguments"`
}

// sobreLlamadas escribe un turno anterior del asistente con el mismo formato que se le pide al agente.
func sobreLlamadas(texto string, llamadas []nucleo.Llamada) string {
	lista := make([]llamadaSobre, len(llamadas))
	for i, l := range llamadas {
		lista[i] = llamadaSobre{ID: l.ID, Nombre: l.Nombre, Argumentos: string(l.Argumentos)}
	}
	datos, _ := json.Marshal(map[string]any{"content": texto, "tool_calls": lista})
	return string(datos)
}

// turnoResultado escribe el resultado de una herramienta como turno del usuario.
func turnoResultado(nombre, id, contenido string, esError bool) string {
	cabecera := "Tool result"
	if esError {
		cabecera = "Tool result (error)"
	}
	if nombre != "" {
		cabecera += " for " + nombre
	}
	if id != "" {
		cabecera += " (call " + id + ")"
	}
	return cabecera + ":\n" + contenido
}
