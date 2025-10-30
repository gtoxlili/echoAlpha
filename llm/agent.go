package llm

import (
	"context"
	"fmt"

	"github.com/gtoxlili/echoAlpha/config"
	"github.com/gtoxlili/echoAlpha/entity"
	"github.com/gtoxlili/echoAlpha/prompts"
	"github.com/gtoxlili/echoAlpha/utils"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/shared"
	"github.com/samber/lo"
)

type Agent struct {
	client                openai.Client
	model                 string
	systemPrompt          string
	lastPortfolioAnalysis string
}

func NewAgent(exchange string, coins []string, modelName string, startingCapital float64) (*Agent, error) {
	systemPrompt := prompts.BuildSystemPrompt(
		exchange,
		coins,
		modelName,
		startingCapital,
		config.DecisionFrequency,
		config.MinLeverage,
		config.MaxLeverage,
	)

	client, err := resolveClient(modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
	}

	return &Agent{
		client:                client,
		model:                 modelName,
		systemPrompt:          systemPrompt,
		lastPortfolioAnalysis: "No positions are open and no prior analysis exists. The market is a blank slate. My immediate goal is to analyze the full dataset provided, establish a market baseline, and find a single, high-quality entry point that aligns with the risk management protocol.",
	}, nil
}

func (a *Agent) RunAnalysis(
	ctx context.Context,
	data entity.PromptData,
) (entity.AgentDecision, error) {
	userPrompt := prompts.BuildUserPrompt(data, a.lastPortfolioAnalysis)

	param := openai.ChatCompletionNewParams{
		Model: a.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(a.systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Temperature: openai.Float(config.LLMTemperature),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: lo.ToPtr(shared.NewResponseFormatJSONObjectParam()),
		},
		ReasoningEffort: openai.ReasoningEffortHigh,
	}

	completion, err := a.client.Chat.Completions.New(ctx, param)
	if err != nil {
		return lo.Empty[entity.AgentDecision](), fmt.Errorf("failed to get completion: %w", err)
	}
	decision, err := utils.ParseResult[entity.AgentDecision](completion)
	if err != nil {
		return lo.Empty[entity.AgentDecision](), fmt.Errorf("failed to parse completion: %w", err)
	}

	// 更新最后的组合分析
	a.lastPortfolioAnalysis = decision.PortfolioAnalysis

	return decision, nil
}
