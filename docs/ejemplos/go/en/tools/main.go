// Tool calling: your app offers functions, the agent asks for one, your app runs it and sends back the result.
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

// getWeather is your app's function: here you would call a real service.
func getWeather(city string) string { return "18 °C and clear in " + city }

func main() {
	url := env("TRAMTAI_URL", "http://127.0.0.1:17373")
	client := openai.NewClient(option.WithBaseURL(url+"/openai/v1"), option.WithAPIKey(env("TRAMTAI_KEY", "<key from My APIs>")))
	model := env("TRAMTAI_MODEL", "claude/haiku")
	tools := []openai.ChatCompletionToolUnionParam{openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "get_weather",
		Description: openai.String("Current weather for a city"),
		Parameters: openai.FunctionParameters{
			"type":       "object",
			"properties": map[string]any{"city": map[string]any{"type": "string"}},
			"required":   []string{"city"},
		},
	})}
	messages := []openai.ChatCompletionMessageParamUnion{openai.UserMessage("What's the weather like in Santiago?")}
	for round := 0; round < 5; round++ { // a cap: if the agent keeps asking for tools, quota isn't spent forever
		response, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
			Model: model, Messages: messages, Tools: tools,
		})
		if err != nil {
			log.Fatal(err)
		}
		message := response.Choices[0].Message
		if len(message.ToolCalls) == 0 {
			fmt.Println(message.Content)
			return
		}
		// The assistant's turn with its calls, and one result for each.
		messages = append(messages, message.ToParam())
		for _, call := range message.ToolCalls {
			var args struct {
				City string `json:"city"`
			}
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				log.Fatal(err)
			}
			fmt.Printf("The agent asks for %s(%s)\n", call.Function.Name, call.Function.Arguments)
			messages = append(messages, openai.ToolMessage(getWeather(args.City), call.ID))
		}
	}
	log.Fatal("the agent kept asking for tools after 5 rounds")
}
