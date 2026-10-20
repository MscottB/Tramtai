// Herramientas (tools): tu app ofrece funciones, el agente pide una, tu app la ejecuta y le devuelve el resultado.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// entorno lee la URL, la clave y el modelo (TRAMTAI_URL, TRAMTAI_CLAVE, TRAMTAI_MODELO).
func entorno(nombre, predeterminado string) string {
	if v := os.Getenv(nombre); v != "" {
		return v
	}
	return predeterminado
}

// clima es la función de tu app: aquí consultarías un servicio real.
func clima(ciudad string) string { return "18 °C y despejado en " + ciudad }

func main() {
	url := entorno("TRAMTAI_URL", "http://127.0.0.1:17373")
	cliente := openai.NewClient(option.WithBaseURL(url+"/openai/v1"), option.WithAPIKey(entorno("TRAMTAI_CLAVE", "<clave de Mis APIs>")))
	modelo := entorno("TRAMTAI_MODELO", "claude/haiku")
	herramientas := []openai.ChatCompletionToolUnionParam{openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "clima",
		Description: openai.String("El tiempo actual de una ciudad"),
		Parameters: openai.FunctionParameters{
			"type":       "object",
			"properties": map[string]any{"ciudad": map[string]any{"type": "string"}},
			"required":   []string{"ciudad"},
		},
	})}
	mensajes := []openai.ChatCompletionMessageParamUnion{openai.UserMessage("¿Qué tiempo hace en Santiago?")}
	for {
		respuesta, err := cliente.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
			Model: modelo, Messages: mensajes, Tools: herramientas,
		})
		if err != nil {
			log.Fatal(err)
		}
		mensaje := respuesta.Choices[0].Message
		if len(mensaje.ToolCalls) == 0 {
			fmt.Println(mensaje.Content)
			return
		}
		// El turno del asistente con sus llamadas, y un resultado por cada una.
		mensajes = append(mensajes, mensaje.ToParam())
		for _, llamada := range mensaje.ToolCalls {
			var argumentos struct {
				Ciudad string `json:"ciudad"`
			}
			if err := json.Unmarshal([]byte(llamada.Function.Arguments), &argumentos); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("El agente pide %s(%s)\n", llamada.Function.Name, llamada.Function.Arguments)
			mensajes = append(mensajes, openai.ToolMessage(clima(argumentos.Ciudad), llamada.ID))
		}
	}
}
