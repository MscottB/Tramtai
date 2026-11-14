// A file with the message: it travels as base64 inside the request and Tramtai turns it into text for the agent.
// Run it from docs/ejemplos/go (go run ./en/file), where ../sample.txt is.
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

// env reads the URL, the key and the model (TRAMTAI_URL, TRAMTAI_KEY, TRAMTAI_MODEL).
func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func main() {
	url := env("TRAMTAI_URL", "http://127.0.0.1:17373")
	client := openai.NewClient(option.WithBaseURL(url+"/openai/v1"), option.WithAPIKey(env("TRAMTAI_KEY", "<key from My APIs>")))
	model := env("TRAMTAI_MODEL", "claude/haiku")
	content, err := os.ReadFile("../sample.txt")
	if err != nil {
		log.Fatal(err)
	}
	file := openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
		Filename: openai.String("sample.txt"),
		FileData: openai.String("data:text/plain;base64," + base64.StdEncoding.EncodeToString(content)),
	})
	response, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: model,
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
			openai.TextContentPart("What does this file say?"), file,
		})},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response.Choices[0].Message.Content)
}
