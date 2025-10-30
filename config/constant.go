package config

import "time"

const (
	KlineInterval       = 3 * time.Minute
	KlineIntervalLonger = 4 * time.Hour

	SeriesLength        = 10
	KlineLimit          = 100
	OiPeriod            = "5m"
	OiLimit             = 288
	MaxHistoricalValues = 1 >> 10 // 最多存储 1024 个历史账户总价值数据点
)

var (
	AssetUniverse = []string{"BTC", "ETH", "AERO", "BNB", "SOL"}
)
