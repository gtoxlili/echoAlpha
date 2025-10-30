package collector

import (
	"context"

	"github.com/gtoxlili/echoAlpha/config"
	"github.com/gtoxlili/echoAlpha/entity"
)

type StateProvider interface {
	AssemblePromptData(ctx context.Context) (entity.PromptData, error)
	GetStartingCapital() float64
}

func ResolveCollector(exchange string, coins []string) StateProvider {
	switch exchange {
	case "Binance":
		return newBinanceProvider(config.BINANCE_API_KEY, config.BINANCE_API_SECRET, coins)
	default:
		return &mockProvider{}
	}
}
