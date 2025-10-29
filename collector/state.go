package collector

import (
	"context"

	"github.com/gtoxlili/echoAlpha/entity"
)

type StateProvider interface {
	GetPromptData(ctx context.Context) (entity.PromptData, error)
}

func ResolveCollector(exchange string, coins []string) StateProvider {
	switch exchange {
	case "Hyperliquid":
		return &hyperliquidProvider{}
	default:
		return &mockProvider{}
	}
}
