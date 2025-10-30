package main

import (
	"context"
	"fmt"

	"github.com/gtoxlili/echoAlpha/collector"
	"github.com/gtoxlili/echoAlpha/prompts"
)

var assetUniverse = []string{"BTC", "ETH", "AERO"}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := collector.ResolveCollector("Binance", assetUniverse)

	//agent, err := llm.NewAgent("Binance", assetUniverse, "doubao-seed-1-6-251015")
	//if err != nil {
	//	panic(err)
	//}

	data, err := provider.AssemblePromptData(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println(prompts.BuildUserPrompt(data))

	//analysis, err := agent.RunAnalysis(ctx, data)
	//if err != nil {
	//	panic(err)
	//}

	//analysis.Print()
}
