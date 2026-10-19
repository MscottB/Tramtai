package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/salidas"
)

// Los agentes CLI no tienen herramientas del cliente: Tramtai las emula. Le describe al agente las herramientas, le
// pide la respuesta como un sobre JSON {"content", "tool_calls"} (con la salida estructurada nativa de cada CLI) y lo
// traduce a tool_calls (OpenAI) o tool_use (Anthropic). El agente nunca ejecuta nada: solo pide la llamada, y la
// aplicación devuelve el resultado en el siguiente turno.

// herramienta es una función que ofrece la aplicación.
type herramienta struct {
	Nombre      string
	Descripcion string
	Parametros  json.RawMessage
}

// eleccion es tool_choice: modo "auto", "ninguna", "obligatoria" o "una" (la de Nombre), y si cabe solo una llamada.
type eleccion struct {
	Modo    string
	Nombre  string
	SoloUna bool
}

// llamada es una petición de herramienta del agente, con los argumentos como objeto JSON compacto.
type llamada struct {
	ID         string `json:"id,omitempty"`
	Nombre     string `json:"name"`
	Argumentos string `json:"arguments"`
}

// herramientas es el uso de herramientas de una solicitud; nil si el agente no puede pedirlas.
type herramientas struct {
	lista    []herramienta
	eleccion eleccion
}

func conParametros(h herramienta) bool {
	return len(h.Parametros) > 0 && string(h.Parametros) != "null"
}

// prepararHerramientas valida las herramientas y la elección y, si el agente debe poder pedirlas, añade su descripción
// a las instrucciones y fija el sobre JSON como esquema de salida.
func prepararHerramientas(sol *nucleo.SolicitudEjecucion, lista []herramienta, e eleccion) (*herramientas, error) {
	invalida := func(texto string) error { return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, texto) }
	if e.Modo == "" {
		e.Modo = "auto"
	}
	if len(lista) == 0 {
		if e.Modo == "obligatoria" || e.Modo == "una" {
			return nil, invalida(idioma.T("tool_choice exige una herramienta, pero tools está vacío"))
		}
		return nil, nil
	}
	nombres := make([]string, 0, len(lista))
	for i, h := range lista {
		if h.Nombre == "" {
			return nil, invalida(idioma.Tf("la herramienta %d no tiene nombre", i))
		}
		if contiene(nombres, h.Nombre) {
			return nil, invalida(idioma.Tf("herramienta repetida: %s", h.Nombre))
		}
		if conParametros(h) {
			if _, err := salidas.Compilar(h.Parametros); err != nil {
				return nil, invalida(idioma.Tf("los parámetros de la herramienta %s no son un JSON Schema válido", h.Nombre))
			}
		}
		nombres = append(nombres, h.Nombre)
	}
	if e.Modo == "una" && !contiene(nombres, e.Nombre) {
		return nil, invalida(idioma.Tf("tool_choice pide una herramienta que no está en tools: %s", e.Nombre))
	}
	if e.Modo == "ninguna" {
		return nil, nil // la conversación puede traer llamadas anteriores; el agente solo responde con texto
	}
	if sol.Salida.Formato == "json" || len(sol.Salida.Esquema) > 0 {
		return nil, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.T("no se pueden combinar herramientas con una respuesta en JSON"))
	}
	if e.Modo == "una" {
		nombres = []string{e.Nombre}
	}
	// Sobre compatible con el modo estricto de Codex: todo obligatorio, sin propiedades extra y los argumentos como
	// texto, porque el esquema de cada herramienta puede no ser estricto. Tramtai los valida después.
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
	sol.Instrucciones = strings.TrimSpace(sol.Instrucciones + "\n\n" + instruccionesHerramientas(lista, e))
	return &herramientas{lista: lista, eleccion: e}, nil
}

// instruccionesHerramientas explica al agente cómo pedir herramientas. Va en inglés, que los modelos siguen mejor,
// y no depende del idioma de la interfaz.
func instruccionesHerramientas(lista []herramienta, e eleccion) string {
	var b strings.Builder
	b.WriteString("The application calling you can run tools for you. You cannot run them yourself: you request them, " +
		"and the application sends back their results in a later turn.\n\nAvailable tools:\n")
	for _, h := range lista {
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
	switch e.Modo {
	case "obligatoria":
		b.WriteString("You must call at least one tool.\n")
	case "una":
		fmt.Fprintf(&b, "You must call the tool %s.\n", e.Nombre)
	}
	if e.SoloUna {
		b.WriteString("Call at most one tool per reply.\n")
	}
	b.WriteString(`Earlier tool calls appear in assistant turns with this same format, and their results in turns that start with "Tool result". ` +
		`Call a tool only when you still need information you don't have: when the results are already in the conversation, ` +
		`use them to answer and never repeat those calls.`)
	return b.String()
}

// respuesta da el texto y las llamadas de un trabajo terminado; sin herramientas, su salida tal cual.
func (h *herramientas) respuesta(final nucleo.Trabajo, prefijo string) (string, []llamada, error) {
	if h == nil {
		return final.Salida, nil, nil
	}
	return h.interpretar(final.Salida, prefijo)
}

// interpretar traduce el sobre del agente. prefijo da los identificadores de las llamadas (call_…, toolu_…).
func (h *herramientas) interpretar(salida, prefijo string) (string, []llamada, error) {
	noCumple := func(texto string) error { return nucleo.NuevoError(nucleo.CodigoSalidaNoCumpleEsquema, texto) }
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
	var llamadas []llamada
	for i, c := range sobre.ToolCalls {
		var pedida *herramienta
		for j := range h.lista {
			if h.lista[j].Nombre == c.Name {
				pedida = &h.lista[j]
			}
		}
		if pedida == nil {
			return "", nil, noCumple(idioma.Tf("el agente pidió una herramienta que no existe: %s", c.Name))
		}
		argumentos, err := objetoJSON(c.Arguments)
		if err != nil {
			return "", nil, noCumple(idioma.Tf("los argumentos de %s no son un objeto JSON", c.Name))
		}
		if conParametros(*pedida) {
			if err := salidas.Validar(pedida.Parametros, argumentos); err != nil {
				return "", nil, noCumple(idioma.Tf("los argumentos de %s no cumplen su esquema: %s", c.Name, nucleo.MensajeDe(err)))
			}
		}
		llamadas = append(llamadas, llamada{ID: fmt.Sprintf("%s_%d", prefijo, i), Nombre: c.Name, Argumentos: argumentos})
	}
	if len(llamadas) == 0 && (h.eleccion.Modo == "obligatoria" || h.eleccion.Modo == "una") {
		return "", nil, noCumple(idioma.T("tool_choice exigía llamar a una herramienta y el agente no lo hizo"))
	}
	if h.eleccion.SoloUna && len(llamadas) > 1 {
		llamadas = llamadas[:1]
	}
	return sobre.Content, llamadas, nil
}

// objetoJSON acepta los argumentos como texto con un objeto JSON (lo pedido) o como el objeto mismo, y los compacta.
func objetoJSON(crudo json.RawMessage) (string, error) {
	var texto string
	if json.Unmarshal(crudo, &texto) == nil {
		crudo = json.RawMessage(texto)
	}
	if strings.TrimSpace(string(crudo)) == "" {
		crudo = json.RawMessage("{}")
	}
	var objeto map[string]json.RawMessage
	if err := json.Unmarshal(crudo, &objeto); err != nil || objeto == nil {
		return "", errors.New("no es un objeto JSON")
	}
	var compacto bytes.Buffer
	if err := json.Compact(&compacto, crudo); err != nil {
		return "", err
	}
	return compacto.String(), nil
}

// contenidoDe lee el contenido de un mensaje: texto, partes de texto, archivos (file, input_file y document) y los
// bloques tool_use y tool_result de Anthropic, que devuelve como llamadas y como turnos de resultado. nombres recuerda
// la herramienta de cada llamada; mensaje (desde 1) y n sitúan y numeran los archivos.
func contenidoDe(crudo json.RawMessage, nombres map[string]string, mensaje int, n *int) (string, []llamada, []string, []archivoEnLinea, error) {
	var texto string
	if len(crudo) == 0 || json.Unmarshal(crudo, &texto) == nil {
		return texto, nil, nil, nil, nil
	}
	var partes []struct {
		parteArchivo
		Type      string          `json:"type"`
		Text      string          `json:"text"`
		ID        string          `json:"id"`
		Name      string          `json:"name"`
		Input     json.RawMessage `json:"input"`
		ToolUseID string          `json:"tool_use_id"`
		Content   json.RawMessage `json:"content"`
		IsError   bool            `json:"is_error"`
	}
	if err := json.Unmarshal(crudo, &partes); err != nil {
		return "", nil, nil, nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("contenido de mensaje no válido"))
	}
	var textos, resultados []string
	var llamadas []llamada
	var archivos []archivoEnLinea
	for i, p := range partes {
		switch p.Type {
		case "file", "input_file", "document":
			a, err := archivoDeParte(p.Type, p.parteArchivo, mensaje, i+1, n)
			if err != nil {
				return "", nil, nil, nil, err
			}
			archivos = append(archivos, a)
		case "image_url", "input_image", "image":
			return "", nil, nil, nil, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado,
				idioma.Tf("mensaje %d, parte %d: Tramtai no admite imágenes todavía", mensaje, i+1))
		case "text", "input_text", "output_text":
			textos = append(textos, p.Text)
		case "tool_use":
			argumentos, err := objetoJSON(p.Input)
			if err != nil {
				return "", nil, nil, nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("los argumentos de %s no son un objeto JSON", p.Name))
			}
			nombres[p.ID] = p.Name
			llamadas = append(llamadas, llamada{ID: p.ID, Nombre: p.Name, Argumentos: argumentos})
		case "tool_result":
			contenido, err := textoDe(p.Content)
			if err != nil {
				return "", nil, nil, nil, err
			}
			resultados = append(resultados, turnoResultado(nombres[p.ToolUseID], p.ToolUseID, contenido, p.IsError))
		default:
			return "", nil, nil, nil, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("contenido de tipo %s no soportado: solo texto y archivos", p.Type))
		}
	}
	return strings.Join(textos, "\n"), llamadas, resultados, archivos, nil
}

// sobreLlamadas escribe un turno anterior del asistente con el mismo formato que se le pide al agente.
func sobreLlamadas(texto string, llamadas []llamada) string {
	datos, _ := json.Marshal(map[string]any{"content": texto, "tool_calls": llamadas})
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

// eleccionOpenAI traduce tool_choice de chat ("auto", "none", "required", {"type":"function","function":{"name"}})
// y de responses ({"type":"function","name"}). paralelas es parallel_tool_calls.
func eleccionOpenAI(crudo json.RawMessage, paralelas *bool) (eleccion, error) {
	e := eleccion{SoloUna: paralelas != nil && !*paralelas}
	if len(crudo) == 0 || string(crudo) == "null" {
		return e, nil
	}
	var modo string
	if json.Unmarshal(crudo, &modo) == nil {
		switch modo {
		case "auto":
			e.Modo = "auto"
		case "none":
			e.Modo = "ninguna"
		case "required":
			e.Modo = "obligatoria"
		default:
			return e, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("tool_choice desconocido: %s", modo))
		}
		return e, nil
	}
	var objeto struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Function *struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(crudo, &objeto); err != nil || objeto.Type != "function" {
		return e, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.T("tool_choice solo admite auto, none, required o una función"))
	}
	e.Modo, e.Nombre = "una", objeto.Name
	if objeto.Function != nil {
		e.Nombre = objeto.Function.Name
	}
	return e, nil
}

// soloFunciones rechaza las herramientas de los proveedores (búsqueda web, archivos, código…): no existen en los
// agentes CLI.
func soloFunciones(tipo string, validos ...string) error {
	if contiene(validos, tipo) {
		return nil
	}
	return nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("herramienta de tipo %s no soportada: solo funciones", tipo))
}
