package llm

import (
	"strings"

	"github.com/gtoxlili/echoAlpha/constant"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func resolveClient(modelName string) (openai.Client, error) {

	switch {
	case strings.HasPrefix(modelName, "gpt-"):
		panic("OpenAI client not implemented yet")
	default:
		apiKey := constant.VOLC_API_KEY
		baseURL := constant.VOLC_BASE_URL

		return openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseURL),
		), nil
	}
}
