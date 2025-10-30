package main

import (
	"context"
	"log"
	"time"

	"github.com/gtoxlili/echoAlpha/collector"
	"github.com/gtoxlili/echoAlpha/llm"
	"github.com/gtoxlili/echoAlpha/trade"
)

var assetUniverse = []string{"BTC", "ETH", "AERO"}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := collector.ResolveCollector("Binance", assetUniverse)
	startingCapital := provider.GetStartingCapital()

	agent, err := llm.NewAgent("Binance", assetUniverse, "doubao-seed-1-6-251015", startingCapital)
	if err != nil {
		panic(err)
	}

	tradeManager := trade.NewManager()
	tradeExecutor := trade.NewExecutor()

	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		data, err := provider.AssemblePromptData(ctx)
		if err != nil {
			panic(err)
		}

		actions, err := agent.RunAnalysis(ctx, data)
		if err != nil {
			panic(err)
		}

		for _, action := range actions {
			switch action.Signal {
			case "buy_to_enter", "sell_to_enter":
				log.Printf("AI decision: Open %s %s", action.Signal, action.Coin)
				execErr := tradeExecutor.Order(action)
				if execErr == nil {
					tradeManager.Add(action)
				} else {
					log.Printf("Failed to execute open order for %s: %v", action.Coin, execErr)
				}
			case "close":
				log.Printf("AI decision: Close %s", action.Coin)
				execErr := tradeExecutor.CloseOrder(action.Coin)
				if execErr == nil {
					// 2. 交易成功, *从我们的状态管理器中移除*
					tradeManager.Remove(action.Coin)
				} else {
					log.Printf("Failed to execute close order for %s: %v", action.Coin, execErr)
				}
			}
		}
	}
}
