package llm

import (
	"context"
	"fmt"

	"github.com/gtoxlili/echoAlpha/entity"
	"github.com/gtoxlili/echoAlpha/prompts"
	"github.com/gtoxlili/echoAlpha/utils"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
	"github.com/samber/lo"
)

type Agent struct {
	client       openai.Client
	model        string
	systemPrompt string
}

func NewAgent(exchange string, coins []string, modelName string) (*Agent, error) {
	systemPrompt := prompts.BuildSystemPrompt(exchange, coins, modelName)

	client, err := resolveClient(modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
	}

	return &Agent{
		client:       client,
		model:        modelName,
		systemPrompt: systemPrompt,
	}, nil
}

func (a *Agent) RunAnalysis(
	ctx context.Context,
	data entity.PromptData,
) (entity.TradeSignals, error) {
	userPrompt := prompts.BuildUserPrompt(data)
	param := openai.ChatCompletionNewParams{
		Model: a.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(a.systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Temperature: openai.Float(0.5),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: lo.ToPtr(shared.NewResponseFormatJSONObjectParam()),
		},
		ReasoningEffort: openai.ReasoningEffortHigh,
	}

	completion, err := a.client.Chat.Completions.New(ctx, param)
	if err != nil {
		return lo.Empty[entity.TradeSignals](), fmt.Errorf("failed to get completion: %w", err)
	}
	return utils.ParseResult[entity.TradeSignals](completion)
}
