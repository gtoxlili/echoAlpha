package main

import (
	"context"
	"echoAlpha/entity"
	"echoAlpha/llm"
)

var assetUniverse = []string{"BTC", "ETH"}

func main() {
	mockData := entity.PromptData{
		MinutesElapsed: 180,
		Coins: map[string]entity.CoinData{
			"BTC": {
				Price: 65500.00,
				EMA20: 65320.0,
				MACD:  25.0,
				RSI7:  58.0,
				OIFunding: entity.OIFunding{
					OILatest: 1250000000,
					OIAvg:    1200000000,
					FundRate: 0.005,
				},
				Intraday: entity.Intraday{
					Prices3m: []float64{65100.5, 65050.0, 65000.0, 65150.5, 65200.0, 65300.0, 65250.5, 65350.0, 65450.0, 65500.00},
					EMA20_3m: []float64{65150.0, 65130.0, 65100.0, 65110.0, 65130.0, 65160.0, 65180.0, 65220.0, 65270.0, 65320.0},
					MACD_3m:  []float64{-50.5, -60.0, -55.0, -40.0, -30.0, -10.0, -5.0, 5.0, 15.0, 25.0},
					RSI7_3m:  []float64{25.0, 22.0, 20.0, 30.0, 35.0, 45.0, 42.0, 50.0, 55.0, 58.0},
					RSI14_3m: []float64{30.0, 28.0, 27.0, 32.0, 36.0, 42.0, 40.0, 45.0, 48.0, 51.0},
				},
				LongTerm: entity.LongTerm{
					EMA20_4h: 64000.0,
					EMA50_4h: 63500.0,
					ATR3_4h:  800.0,
					ATR14_4h: 1000.0,
					VolCurr:  150000000,
					VolAvg:   120000000,
					MACD_4h:  []float64{50.0, 20.0, -10.0, -5.0, 15.0, 30.0, 45.0, 60.0, 70.0, 75.0},
					RSI14_4h: []float64{50.0, 48.0, 45.0, 47.0, 51.0, 53.0, 55.0, 58.0, 60.0, 61.0},
				},
			},
			"ETH": {
				Price: 3520.00,
				EMA20: 3516.0,
				MACD:  2.0,
				RSI7:  51.0,
				OIFunding: entity.OIFunding{
					OILatest: 850000000,
					OIAvg:    860000000,
					FundRate: -0.002,
				},
				Intraday: entity.Intraday{
					Prices3m: []float64{3510.0, 3505.0, 3500.0, 3502.0, 3508.0, 3512.0, 3510.0, 3514.0, 3518.0, 3520.00},
					EMA20_3m: []float64{3515.0, 3514.0, 3512.0, 3511.0, 3511.0, 3511.5, 3511.0, 3512.0, 3514.0, 3516.0},
					MACD_3m:  []float64{-5.0, -6.0, -5.5, -4.0, -3.0, -1.0, -0.5, 0.5, 1.5, 2.0},
					RSI7_3m:  []float64{30.0, 28.0, 25.0, 28.0, 35.0, 40.0, 38.0, 44.0, 48.0, 51.0},
					RSI14_3m: []float64{35.0, 33.0, 30.0, 32.0, 37.0, 40.0, 39.0, 42.0, 45.0, 47.0},
				},
				LongTerm: entity.LongTerm{
					EMA20_4h: 3450.0,
					EMA50_4h: 3460.0,
					ATR3_4h:  50.0,
					ATR14_4h: 65.0,
					VolCurr:  90000000,
					VolAvg:   100000000,
					MACD_4h:  []float64{5.0, 2.0, -1.0, -3.0, -5.0, -4.0, -3.0, -2.0, -1.0, -0.5},
					RSI14_4h: []float64{51.0, 49.0, 47.0, 46.0, 44.0, 45.0, 46.0, 47.0, 47.5, 48.0},
				},
			},
		},
		Account: entity.AccountData{
			ReturnPct:     -1.5,
			SharpeRatio:   -0.25,
			CashAvailable: 8000.00,
			AccountValue:  9850.00,
		},
		Positions: []entity.PositionData{
			{
				Symbol:        "ETH",
				Quantity:      -2.0,
				EntryPrice:    3505.00,
				CurrentPrice:  3520.00,
				LiqPrice:      3820.45,
				UnrealizedPNL: -30.00,
				Leverage:      10,
				ExitPlan: entity.ExitPlanData{
					ProfitTarget: 3450.00,
					StopLoss:     3525.00,
					InvalidCond:  "Price breaks above 3525 or 3-min MACD crosses positive.",
				},
				Confidence:  0.6,
				RiskUSD:     40.00,
				NotionalUSD: 7010.00,
			},
		},
	}

	agent, err := llm.NewAgent("OKX", assetUniverse, "doubao-seed-1-6-251015")
	if err != nil {
		panic(err)
	}

	analysis, err := agent.RunAnalysis(context.Background(), mockData)
	if err != nil {
		panic(err)
	}

	analysis.Print()
}
