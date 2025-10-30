package llm

import (
	"strings"

	"github.com/gtoxlili/echoAlpha/config"
	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

func resolveClient(modelName string) (openai.Client, error) {

	switch {
	case strings.HasPrefix(modelName, "doubao-"):
		return openai.NewClient(
			option.WithAPIKey(config.VOLC_API_KEY),
			option.WithBaseURL(config.VOLC_API_KEY),
		), nil
	default:
		panic("unimplemented model provider resolver")
	}
}
