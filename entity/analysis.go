package entity

import (
	"fmt"
	"time"

	json "github.com/bytedance/sonic"
)

type TradeMetadata struct {
	Symbol                string    `json:"symbol"`
	EntryTime             time.Time `json:"entry_time"` // <-- 我们在这里添加了持仓时间
	ProfitTarget          float64   `json:"profit_target"`
	StopLoss              float64   `json:"stop_loss"`
	InvalidationCondition string    `json:"invalidation_condition"`
	Confidence            float64   `json:"confidence"`
	RiskUSD               float64   `json:"risk_usd"`
	Justification         string    `json:"justification"`
}

type TradeSignal struct {
	Signal                string  `json:"signal"`
	Coin                  string  `json:"coin"`
	Quantity              float64 `json:"quantity"`
	Leverage              int     `json:"leverage"`
	ProfitTarget          float64 `json:"profit_target"`
	StopLoss              float64 `json:"stop_loss"`
	InvalidationCondition string  `json:"invalidation_condition"`
	Confidence            float64 `json:"confidence"`
	RiskUSD               float64 `json:"risk_usd"`
	Justification         string  `json:"justification"`
}

type AIResponse struct {
	PortfolioAnalysis string        `json:"portfolio_analysis"`
	Actions           []TradeSignal `json:"actions"`
}

func (ar AIResponse) Print() {
	display, _ := json.MarshalIndent(ar, "", "  ")
	fmt.Printf("%s\n", display)
}
