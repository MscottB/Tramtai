// Una respuesta en JSON que cumple un esquema: Tramtai la valida antes de devolverla.
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

func main() {
	url := entorno("TRAMTAI_URL", "http://127.0.0.1:17373")
	cliente := openai.NewClient(option.WithBaseURL(url+"/openai/v1"), option.WithAPIKey(entorno("TRAMTAI_CLAVE", "<clave de Mis APIs>")))
	modelo := entorno("TRAMTAI_MODELO", "claude/haiku")
	esquema := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{"resumen": map[string]any{"type": "string"}},
		"required":             []string{"resumen"},
		"additionalProperties": false,
	}
	respuesta, err := cliente.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model:    modelo,
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("Resume en una frase qué es Tramtai.")},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
			JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{Name: "resumen", Schema: esquema},
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	var datos struct {
		Resumen string `json:"resumen"`
	}
	if err := json.Unmarshal([]byte(respuesta.Choices[0].Message.Content), &datos); err != nil {
		log.Fatal(err)
	}
	fmt.Println("resumen:", datos.Resumen)
}
