// Tu primer mensaje por la URL compatible con OpenAI, con la respuesta en vivo.
package main

import (
	"context"
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
	flujo := cliente.Chat.Completions.NewStreaming(context.Background(), openai.ChatCompletionNewParams{
		Model:    modelo,
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("Hola, ¿quién eres?")},
	})
	for flujo.Next() {
		if trozo := flujo.Current(); len(trozo.Choices) > 0 {
			fmt.Print(trozo.Choices[0].Delta.Content)
		}
	}
	if err := flujo.Err(); err != nil {
		log.Fatal(err)
	}
	fmt.Println()
}
