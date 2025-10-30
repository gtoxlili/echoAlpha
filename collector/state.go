package collector

import (
	"context"

	"github.com/gtoxlili/echoAlpha/constant"
	"github.com/gtoxlili/echoAlpha/entity"
)

type StateProvider interface {
	AssemblePromptData(ctx context.Context) (entity.PromptData, error)
	GetStartingCapital() float64
}

func ResolveCollector(exchange string, coins []string) StateProvider {
	switch exchange {
	case "Binance":
		return newBinanceProvider(constant.BINANCE_API_KEY, constant.BINANCE_API_SECRET, coins)
	default:
		return &mockProvider{}
	}
}
