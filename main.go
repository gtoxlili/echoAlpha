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

		// 暂不考虑 "僵尸" 持仓 的情况
		for idx, positions := range data.Positions {
			meta, exists := tradeManager.Get(positions.Symbol)
			if !exists {
				continue
			}
			data.Positions[idx].ExitPlan.ProfitTarget = meta.ProfitTarget
			data.Positions[idx].ExitPlan.StopLoss = meta.StopLoss
			data.Positions[idx].ExitPlan.InvalidCond = meta.InvalidationCondition
			data.Positions[idx].Confidence = meta.Confidence
			data.Positions[idx].RiskUSD = meta.RiskUSD
			data.Positions[idx].AgeInMinutes = time.Since(meta.EntryTime).Minutes()
		}

		actions, err := agent.RunAnalysis(ctx, data)
		if err != nil {
			panic(err)
		}

		for _, action := range actions {
			switch action.Signal {
			case "buy_to_enter", "sell_to_enter":
				log.Printf("AI decision: Open %s %s", action.Signal, action.Coin)
				execErr := tradeExecutor.Order(ctx, action)
				if execErr == nil {
					tradeManager.Add(action)
				} else {
					log.Printf("Failed to execute open order for %s: %v", action.Coin, execErr)
				}
			case "close":
				log.Printf("AI decision: Close %s", action.Coin)
				execErr := tradeExecutor.CloseOrder(ctx, action.Coin)
				if execErr == nil {
					tradeManager.Remove(action.Coin)
				} else {
					log.Printf("Failed to execute close order for %s: %v", action.Coin, execErr)
				}
			}
		}
	}
}
