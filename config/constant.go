package config

import "time"

const (
	KlineInterval       = 3 * time.Minute
	KlineIntervalLonger = 4 * time.Hour

	SeriesLength        = 16
	KlineLimit          = 256
	OiPeriod            = "5m"
	OiLimit             = 288
	MaxHistoricalValues = 1 << 10 // 最多存储 1024 个历史账户总价值数据点

	DecisionFrequency = "Every 6-12 minutes (mid-to-low frequency trading)"
	MinLeverage       = 1
	MaxLeverage       = 20

	LLMTemperature = 0.6

	PersistencePath = ".echo-alpha-persistence.json"
)

var (
	AssetUniverse = []string{"BTC", "ETH", "AERO", "BNB", "SOL", "XRP"}
)
