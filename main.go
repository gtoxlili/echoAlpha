package main

import (
	"context"

	"github.com/gtoxlili/echoAlpha/collector"
	"github.com/gtoxlili/echoAlpha/llm"
)

var assetUniverse = []string{"BTC", "ETH", "SOL", "BNB"}

func main() {
	provider := collector.ResolveCollector("Binance", assetUniverse)

	agent, err := llm.NewAgent("Hyperliquid", assetUniverse, "doubao-seed-1-6-251015")
	if err != nil {
		panic(err)
	}

	data, err := provider.AssemblePromptData(context.Background())
	if err != nil {
		panic(err)
	}
	analysis, err := agent.RunAnalysis(context.Background(), data)
	if err != nil {
		panic(err)
	}

	analysis.Print()
}
