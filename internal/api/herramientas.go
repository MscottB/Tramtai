package api

import (
	"encoding/json"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Las herramientas las emula internal/herramientas para las tres APIs; aquí solo se traduce el formato de OpenAI y
// Anthropic a los campos del núcleo (herramientas, elección, llamadas y resultados) y de vuelta.

// llamadaDe lee una llamada anterior del asistente; los argumentos llegan como texto con un objeto JSON.
func llamadaDe(id, nombre string, argumentos json.RawMessage) (nucleo.Llamada, error) {
	objeto, err := nucleo.ObjetoJSON(argumentos)
	if err != nil {
		return nucleo.Llamada{}, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("los argumentos de %s no son un objeto JSON", nombre))
	}
	return nucleo.Llamada{ID: id, Nombre: nombre, Argumentos: json.RawMessage(objeto)}, nil
}

// contenidoDe lee el contenido de un mensaje: texto, partes de texto, archivos (file, input_file y document) y los
// bloques tool_use y tool_result de Anthropic, que devuelve como llamadas y como mensajes «herramienta». mensaje
// (desde 1) y n sitúan y numeran los archivos.
func contenidoDe(crudo json.RawMessage, mensaje int, n *int) (string, []nucleo.Llamada, []nucleo.Mensaje, []archivoEnLinea, error) {
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
	var textos []string
	var resultados []nucleo.Mensaje
	var llamadas []nucleo.Llamada
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
			l, err := llamadaDe(p.ID, p.Name, p.Input)
			if err != nil {
				return "", nil, nil, nil, err
			}
			llamadas = append(llamadas, l)
		case "tool_result":
			contenido, err := textoDe(p.Content)
			if err != nil {
				return "", nil, nil, nil, err
			}
			resultados = append(resultados, nucleo.Mensaje{Rol: "herramienta", IDLlamada: p.ToolUseID, Contenido: contenido, EsError: p.IsError})
		default:
			return "", nil, nil, nil, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("contenido de tipo %s no soportado: solo texto y archivos", p.Type))
		}
	}
	return strings.Join(textos, "\n"), llamadas, resultados, archivos, nil
}

// eleccionOpenAI traduce tool_choice de chat ("auto", "none", "required", {"type":"function","function":{"name"}})
// y de responses ({"type":"function","name"}).
func eleccionOpenAI(crudo json.RawMessage) (nucleo.EleccionHerramienta, error) {
	var e nucleo.EleccionHerramienta
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
