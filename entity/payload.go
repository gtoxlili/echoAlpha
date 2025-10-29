package entity

type PromptData struct {
	MinutesElapsed int
	Coins          map[string]CoinData // 关键变更：使用 Map
	Account        AccountData
	Positions      []PositionData
}

type AccountData struct {
	ReturnPct     float64
	SharpeRatio   float64
	CashAvailable float64
	AccountValue  float64
}

type CoinData struct {
	Price float64
	EMA20 float64
	MACD  float64
	RSI7  float64
	OIFunding
	Intraday
	LongTerm
}

type OIFunding struct {
	OILatest float64
	OIAvg    float64
	FundRate float64
}

type Intraday struct {
	Prices3m []float64
	EMA20_3m []float64
	MACD_3m  []float64
	RSI7_3m  []float64
	RSI14_3m []float64
}

type LongTerm struct {
	EMA20_4h float64
	EMA50_4h float64
	ATR3_4h  float64
	ATR14_4h float64
	VolCurr  float64
	VolAvg   float64
	MACD_4h  []float64
	RSI14_4h []float64
}

type PositionData struct {
	Symbol        string
	Quantity      float64
	EntryPrice    float64
	CurrentPrice  float64
	LiqPrice      float64
	UnrealizedPNL float64
	Leverage      int
	ExitPlan      ExitPlanData
	Confidence    float64
	RiskUSD       float64
	NotionalUSD   float64
}

type ExitPlanData struct {
	ProfitTarget float64
	StopLoss     float64
	InvalidCond  string
}
