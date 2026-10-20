// Un archivo junto al mensaje: va en base64 dentro de la petición y Tramtai lo convierte a texto para el agente.
package main

import (
	"context"
	"encoding/base64"
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
	contenido, err := os.ReadFile("../muestra.txt")
	if err != nil {
		log.Fatal(err)
	}
	archivo := openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
		Filename: openai.String("muestra.txt"),
		FileData: openai.String("data:text/plain;base64," + base64.StdEncoding.EncodeToString(contenido)),
	})
	respuesta, err := cliente.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: modelo,
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
			openai.TextContentPart("¿Qué dice este archivo?"), archivo,
		})},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(respuesta.Choices[0].Message.Content)
}
