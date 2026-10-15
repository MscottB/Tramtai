package adaptadores

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// baseGemini es la Gemini API pública (generativelanguage); Vertex AI queda fuera del MVP inicial.
const baseGemini = "https://generativelanguage.googleapis.com/v1beta"

// modeloGeminiPredeterminado se usa si la solicitud no fija modelo ni perfil.
const modeloGeminiPredeterminado = "gemini-flash-latest"

// Gemini llama directamente a la Gemini API con la clave del usuario: sin CLI ni sesión OAuth.
type Gemini struct {
	Base    string       // URL base; vacía = baseGemini (las pruebas la sustituyen)
	Cliente *http.Client // nil = http.DefaultClient
}

// Detectar implementa Detectable sin consumir cuota.
func (g Gemini) Detectar(context.Context) Estado {
	e := Estado{Agente: "gemini", Autenticacion: AutenticacionClaveAPI, Ruta: g.base(),
		Terminos: idioma.T("términos de la Gemini API de Google (clave de API propia)")}
	if claveEntorno("GEMINI_API_KEY", "GOOGLE_API_KEY") == "" {
		e.Problema = idioma.T("falta GEMINI_API_KEY (tramtai claves guardar GEMINI_API_KEY)")
	}
	return e
}

func (g Gemini) base() string {
	if g.Base != "" {
		return strings.TrimSuffix(g.Base, "/")
	}
	return baseGemini
}

// Ejecutar implementa Adaptador con streamGenerateContent (SSE): el texto llega por fragmentos.
func (g Gemini) Ejecutar(ctx context.Context, s nucleo.SolicitudEjecucion, emitir func(Evento) error) (Resultado, error) {
	clave := claveEntorno("GEMINI_API_KEY", "GOOGLE_API_KEY")
	if clave == "" {
		return Resultado{}, nucleo.NuevoError(nucleo.CodigoAutenticacionAgente, idioma.T("falta GEMINI_API_KEY (tramtai claves guardar GEMINI_API_KEY)"))
	}
	if consciente(s) {
		return Resultado{}, nucleo.NuevoError(nucleo.CodigoContextoNoPermitido, idioma.T("la Gemini API no tiene proyecto: usa modo_contexto aislado"))
	}
	modelo := strings.TrimPrefix(s.Modelo, "models/")
	if modelo == "" {
		modelo = modeloGeminiPredeterminado
	}
	cuerpo, err := cuerpoGemini(s)
	if err != nil {
		return Resultado{}, err
	}
	destino := g.base() + "/models/" + url.PathEscape(modelo) + ":streamGenerateContent?alt=sse"
	peticion, err := http.NewRequestWithContext(ctx, http.MethodPost, destino, bytes.NewReader(cuerpo))
	if err != nil {
		return Resultado{}, err
	}
	peticion.Header.Set("Content-Type", "application/json")
	peticion.Header.Set("x-goog-api-key", clave) // en cabecera: nunca en la URL
	cliente := g.Cliente
	if cliente == nil {
		cliente = http.DefaultClient
	}
	respuesta, err := cliente.Do(peticion)
	if err != nil {
		if ctx.Err() != nil {
			return Resultado{}, ctx.Err()
		}
		return Resultado{}, nucleo.NuevoError(nucleo.CodigoAgenteNoDisponible, idioma.Tf("no se pudo contactar con la Gemini API: %s", redactar(err.Error(), clave)))
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusOK {
		datos, _ := io.ReadAll(io.LimitReader(respuesta.Body, 16<<10))
		texto := redactar(mensajeGemini(datos), clave)
		codigo := clasificar(texto, respuesta.StatusCode)
		if respuesta.StatusCode == http.StatusNotFound {
			codigo = nucleo.CodigoModeloNoDisponible
		}
		return Resultado{}, nucleo.NuevoError(codigo, "Gemini API ("+respuesta.Status+"): "+texto)
	}

	var salida strings.Builder
	var r Resultado
	lector := bufio.NewScanner(respuesta.Body)
	lector.Buffer(make([]byte, 64<<10), 8<<20)
	for lector.Scan() {
		datos, esDato := strings.CutPrefix(lector.Text(), "data:")
		if !esDato {
			continue
		}
		var f fragmentoGemini
		if err := json.Unmarshal([]byte(strings.TrimSpace(datos)), &f); err != nil {
			return Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorProtocoloAgente, idioma.T("fragmento SSE de Gemini no válido"))
		}
		if f.Error.Message != "" {
			return Resultado{}, nucleo.NuevoError(clasificar(f.Error.Message, f.Error.Code), "Gemini API: "+redactar(f.Error.Message, clave))
		}
		for _, c := range f.Candidates {
			for _, p := range c.Content.Parts {
				if p.Text == "" || p.Thought {
					continue
				}
				salida.WriteString(p.Text)
				if err := emitir(Evento{Tipo: "texto", Datos: p.Text}); err != nil {
					return Resultado{}, err
				}
			}
			if c.FinishReason == "SAFETY" || c.FinishReason == "PROHIBITED_CONTENT" {
				return Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorAgente, idioma.Tf("Gemini bloqueó la respuesta (%s)", c.FinishReason))
			}
		}
		if f.UsageMetadata.PromptTokenCount > 0 || f.UsageMetadata.CandidatesTokenCount > 0 {
			r.Uso = nucleo.Uso{TokensEntrada: f.UsageMetadata.PromptTokenCount,
				TokensSalida: f.UsageMetadata.CandidatesTokenCount + f.UsageMetadata.ThoughtsTokenCount}
		}
		if f.ModelVersion != "" {
			r.ModeloEfectivo = f.ModelVersion
		}
	}
	if err := lector.Err(); err != nil {
		if ctx.Err() != nil {
			return Resultado{}, ctx.Err()
		}
		return Resultado{}, nucleo.NuevoError(nucleo.CodigoErrorProtocoloAgente, idioma.Tf("flujo de Gemini interrumpido: %s", redactar(err.Error(), clave)))
	}
	r.Salida = salida.String()
	return r, nil
}

// cuerpoGemini traduce la solicitud: los mensajes van como contents con sus roles y el sistema como
// systemInstruction, sin reescribir el contenido. El esquema usa la salida estructurada nativa.
func cuerpoGemini(s nucleo.SolicitudEjecucion) ([]byte, error) {
	type parte struct {
		Text string `json:"text"`
	}
	type contenido struct {
		Role  string  `json:"role,omitempty"`
		Parts []parte `json:"parts"`
	}
	var sistema []string
	if s.Instrucciones != "" {
		sistema = append(sistema, s.Instrucciones)
	}
	var contenidos []contenido
	for _, m := range s.Mensajes {
		switch m.Rol {
		case "sistema":
			sistema = append(sistema, m.Contenido)
		case "asistente":
			contenidos = append(contenidos, contenido{Role: "model", Parts: []parte{{m.Contenido}}})
		default:
			contenidos = append(contenidos, contenido{Role: "user", Parts: []parte{{m.Contenido}}})
		}
	}
	cuerpo := map[string]any{"contents": contenidos}
	if len(sistema) > 0 {
		cuerpo["systemInstruction"] = contenido{Parts: []parte{{strings.Join(sistema, "\n\n")}}}
	}
	configuracion := map[string]any{}
	if len(s.Salida.Esquema) > 0 {
		if !json.Valid(s.Salida.Esquema) {
			return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("esquema no válido"))
		}
		configuracion["responseMimeType"] = "application/json"
		configuracion["responseJsonSchema"] = s.Salida.Esquema
	} else if s.Salida.Formato == "json" {
		configuracion["responseMimeType"] = "application/json"
	}
	if len(configuracion) > 0 {
		cuerpo["generationConfig"] = configuracion
	}
	return json.Marshal(cuerpo)
}

type fragmentoGemini struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text    string `json:"text"`
				Thought bool   `json:"thought"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		ThoughtsTokenCount   int `json:"thoughtsTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion"`
	Error        struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// mensajeGemini extrae error.message de una respuesta de error; si no es JSON, devuelve el texto recortado.
func mensajeGemini(datos []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(datos, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return strings.TrimSpace(string(datos))
}
