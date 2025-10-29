package llm

import (
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

const (
	envVolcAPIKey  = "VOLC_API_KEY"
	envVolcBaseURL = "VOLC_BASE_URL"

	envOpenAIAPIKey = "OPENAI_API_KEY"
)

func resolveClient(modelName string) (openai.Client, error) {

	switch {
	case strings.HasPrefix(modelName, "gpt-"):
		apiKey := os.Getenv(envOpenAIAPIKey)
		if apiKey == "" {
			return openai.Client{}, fmt.Errorf("env var %s not set for model %s", envOpenAIAPIKey, modelName)
		}
		return openai.NewClient(option.WithAPIKey(apiKey)), nil
	default:
		apiKey := os.Getenv(envVolcAPIKey)
		baseURL := os.Getenv(envVolcBaseURL)

		if apiKey == "" || baseURL == "" {
			return openai.Client{}, fmt.Errorf("env vars %s or %s not set for model %s", envVolcAPIKey, envVolcBaseURL, modelName)
		}

		return openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseURL),
		), nil
	}
}
