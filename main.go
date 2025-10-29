package main

import (
	"fmt"

	"github.com/gtoxlili/echoAlpha/collector"
)

var assetUniverse = []string{"BTC", "ETH"}

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

	fmt.Println(collector.AssemblePromptData())
}
