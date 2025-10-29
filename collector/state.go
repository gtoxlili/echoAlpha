package collector

import (
	"context"
	"os"

	"github.com/gtoxlili/echoAlpha/entity"
)

type StateProvider interface {
	AssemblePromptData(ctx context.Context) (entity.PromptData, error)
}

func ResolveCollector(exchange string, coins []string) StateProvider {
	switch exchange {
	case "Binance":
		return newBinanceProvider(os.Getenv("BINANCE_API_KEY"), os.Getenv("BINANCE_API_SECRET"), coins)
	default:
		return &mockProvider{}
	}
}
