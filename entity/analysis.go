package entity

import (
	"fmt"

	json "github.com/bytedance/sonic"
)

type SignalEnum string

const (
	BuyToEnter  SignalEnum = "buy_to_enter"
	SellToEnter SignalEnum = "sell_to_enter"
	Close       SignalEnum = "close"
)

type TradeSignal struct {
	Signal                SignalEnum `json:"signal"`
	Coin                  string     `json:"coin"`
	Quantity              float64    `json:"quantity"`
	Leverage              int        `json:"leverage"`
	ProfitTarget          float64    `json:"profit_target"`
	StopLoss              float64    `json:"stop_loss"`
	InvalidationCondition string     `json:"invalidation_condition"`
	Confidence            float64    `json:"confidence"`
	RiskUSD               float64    `json:"risk_usd"`
	Justification         string     `json:"justification"`
}

type TradeSignals []TradeSignal

func (tss TradeSignals) Print() {
	display, _ := json.MarshalIndent(tss, "", "  ")
	fmt.Printf("%s\n", display)
}
