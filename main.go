package main

import (
	"context"
	"fmt"

	"github.com/gtoxlili/echoAlpha/collector"
	"github.com/gtoxlili/echoAlpha/prompts"
)

var assetUniverse = []string{"BTC", "ETH", "SOL", "BNB"}

func main() {
	//provider := collector.ResolveCollector("Mock", assetUniverse)
	//
	//agent, err := llm.NewAgent("Hyperliquid", assetUniverse, "doubao-seed-1-6-251015")
	//if err != nil {
	//	panic(err)
	//}
	//
	//data, err := provider.GetPromptData(context.Background())
	//if err != nil {
	//	panic(err)
	//}
	//analysis, err := agent.RunAnalysis(context.Background(), data)
	//if err != nil {
	//	panic(err)
	//}
	//
	//analysis.Print()

	provider := collector.ResolveCollector("Binance", assetUniverse)

	data, _ := provider.AssemblePromptData(context.Background())
	fmt.Println(prompts.BuildUserPrompt(data))
}
