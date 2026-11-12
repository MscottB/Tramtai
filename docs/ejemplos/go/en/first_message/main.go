// Your first message through the OpenAI-compatible URL, with the response streamed live.
package main

import (
	"context"
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
	stream := client.Chat.Completions.NewStreaming(context.Background(), openai.ChatCompletionNewParams{
		Model:    model,
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("Hi, who are you?")},
	})
	for stream.Next() {
		if chunk := stream.Current(); len(chunk.Choices) > 0 {
			fmt.Print(chunk.Choices[0].Delta.Content)
		}
	}
	if err := stream.Err(); err != nil {
		log.Fatal(err)
	}
	fmt.Println()
}
