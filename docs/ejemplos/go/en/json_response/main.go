// A JSON response that follows a schema: Tramtai validates it before returning it.
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
	schema := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{"summary": map[string]any{"type": "string"}},
		"required":             []string{"summary"},
		"additionalProperties": false,
	}
	response, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model:    model,
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage("Summarize in one sentence what Tramtai is.")},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
			JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{Name: "summary", Schema: schema},
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	var data struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(response.Choices[0].Message.Content), &data); err != nil {
		log.Fatal(err)
	}
	fmt.Println("summary:", data.Summary)
}
