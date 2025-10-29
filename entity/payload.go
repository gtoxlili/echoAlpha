package entity

// PromptData 是包含所有上下文的顶级结构
type PromptData struct {
	MinutesElapsed int                 `json:"minutes_elapsed"`
	Coins          map[string]CoinData `json:"coins"`
	Account        AccountData         `json:"account"`
	Positions      []PositionData      `json:"positions"`
}

// CoinData 包含特定加密货币的市场数据
type CoinData struct {
	Price float64 `json:"price"`
	EMA20 float64 `json:"ema_20"`
	MACD  float64 `json:"macd"`
	RSI7  float64 `json:"rsi_7"`
	OIFunding
	Intraday
	LongTerm
}

// OIFunding 包含持仓量和资金费率数据
type OIFunding struct {
	OILatest int64   `json:"oi_latest"`
	OIAvg    int64   `json:"oi_avg"`
	FundRate float64 `json:"fund_rate"`
}

// Intraday 包含短线（3分钟）时间框架数据
type Intraday struct {
	Prices3m []float64 `json:"prices_3m"`
	EMA20_3m []float64 `json:"ema_20_3m"`
	MACD_3m  []float64 `json:"macd_3m"`
	RSI7_3m  []float64 `json:"rsi_7_3m"`
	RSI14_3m []float64 `json:"rsi_14_3m"`
}

// LongTerm 包含长线（4小时）时间框架数据
type LongTerm struct {
	EMA20_4h float64   `json:"ema_20_4h"`
	EMA50_4h float64   `json:"ema_50_4h"`
	ATR3_4h  float64   `json:"atr_3_4h"`
	ATR14_4h float64   `json:"atr_14_4h"`
	VolCurr  int64     `json:"vol_curr"`
	VolAvg   int64     `json:"vol_avg"`
	MACD_4h  []float64 `json:"macd_4h"`
	RSI14_4h []float64 `json:"rsi_14_4h"`
}

// AccountData 包含账户绩效和余额
type AccountData struct {
	ReturnPct     float64 `json:"return_pct"`
	SharpeRatio   float64 `json:"sharpe_ratio"`
	CashAvailable float64 `json:"cash_available"`
	AccountValue  float64 `json:"account_value"`
}

// PositionData 包含单个持仓的详细信息
type PositionData struct {
	Symbol        string       `json:"symbol"`
	Quantity      float64      `json:"quantity"`
	EntryPrice    float64      `json:"entry_price"`
	CurrentPrice  float64      `json:"current_price"`
	LiqPrice      float64      `json:"liq_price"`
	UnrealizedPNL float64      `json:"unrealized_pnl"`
	Leverage      int          `json:"leverage"`
	ExitPlan      ExitPlanData `json:"exit_plan"`
	Confidence    float64      `json:"confidence"`
	RiskUSD       float64      `json:"risk_usd"`
	NotionalUSD   float64      `json:"notional_usd"`
}

// ExitPlanData 包含仓位的退出策略
type ExitPlanData struct {
	ProfitTarget float64 `json:"profit_target"`
	StopLoss     float64 `json:"stop_loss"`
	InvalidCond  string  `json:"invalid_cond"`
}
