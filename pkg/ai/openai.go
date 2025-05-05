package ai

import (
	"context"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

// OpenAIClient is the global client for interacting with OpenAI
var OpenAIClient *openai.Client

// InitOpenAI initializes the OpenAI client using the environment variable API key.
func InitOpenAI() {
	OpenAIClient = openai.NewClient(os.Getenv("OPENAI_API_KEY"))
}

// GenerateReport sends a chat completion request to the OpenAI API with the given prompt.
// Returns the assistant's response or an error.
func GenerateReport(prompt string) (string, error) {
	resp, err := OpenAIClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    openai.GPT4Turbo,
			Messages: []openai.ChatCompletionMessage{{Role: "user", Content: prompt}},
		},
	)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Message.Content, nil
}
