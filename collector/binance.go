package collector

import (
	"context"
	"errors"
	"strconv"

	"github.com/adshao/go-binance/v2"
	"github.com/cinar/indicator"
	"github.com/gtoxlili/echoAlpha/entity"
)

// 获取指定交易对指定周期的历史K线收盘价
func fetchClosePrices(client *binance.Client, symbol, interval string, limit int) ([]float64, error) {
	klines, err := client.NewKlinesService().Symbol(symbol).Interval(interval).Limit(limit).Do(context.Background())
	if err != nil {
		return nil, err
	}
	prices := make([]float64, 0, len(klines))
	for _, k := range klines {
		p, err := strconv.ParseFloat(k.Close, 64)
		if err != nil {
			return nil, err
		}
		prices = append(prices, p)
	}
	return prices, nil
}

// 计算EMA20指标，返回最新EMA20值
func calculateEMA20(prices []float64) float64 {
	emaSeries := indicator.Ema(20, prices)
	if len(emaSeries) == 0 {
		return 0
	}
	return emaSeries[len(emaSeries)-1]
}

// 计算MACD指标，返回最新MACD和Signal值以及两者差值Hist
func calculateMACD(prices []float64) (macd float64, signal float64, hist float64) {
	macdSeries, signalSeries := indicator.Macd(prices)
	if len(macdSeries) == 0 || len(signalSeries) == 0 {
		return 0, 0, 0
	}
	macd = macdSeries[len(macdSeries)-1]
	signal = signalSeries[len(signalSeries)-1]
	hist = macd - signal
	return macd, signal, hist
}

// 计算RSI指标，返回最新RSI值
func calculateRSI(prices []float64, period int) float64 {
	_, rsiSeries := indicator.RsiPeriod(period, prices)
	if len(rsiSeries) == 0 {
		return 0
	}
	return rsiSeries[len(rsiSeries)-1]
}

func fetchCurrentPrice(client *binance.Client, symbol string) (float64, error) {
	prices, err := client.NewListPricesService().Do(context.Background())
	if err != nil {
		return 0, err
	}
	for _, p := range prices {
		if p.Symbol == symbol {
			price, err := strconv.ParseFloat(p.Price, 64)
			if err != nil {
				return 0, err
			}
			return price, nil
		}
	}
	return 0, errors.New("symbol not found in price list")
}

// AssemblePromptData 汇总数据示例，需根据完整结构继续填充
func AssemblePromptData(apiKey, secretKey string, symbol string) (*entity.PromptData, error) {
	client := binance.NewClient(apiKey, secretKey)

	// 拉取现价
	currentPrice, err := fetchCurrentPrice(client, symbol)
	if err != nil {
		return nil, err
	}

	// 拉取过去K线作为计算指标基础，示例取30条1分钟K线
	closePrices, err := fetchClosePrices(client, symbol, "1m", 30)
	if err != nil {
		return nil, err
	}

	// 计算指标
	ema20 := calculateEMA20(closePrices)
	macd, _, _ := calculateMACD(closePrices)
	rsi7 := calculateRSI(closePrices, 7)

	// 组织数据结构，示例填充部分CoinData
	coinData := entity.CoinData{
		Price: currentPrice,
		EMA20: ema20,
		MACD:  macd,
		RSI7:  rsi7,
		// OIFunding, Intraday, LongTerm 另需实现
	}

	coins := make(map[string]entity.CoinData)
	coins[symbol] = coinData

	// TODO: 获取账户数据，仓位数据，构建 AccountData 和 Positions

	promptData := &entity.PromptData{
		MinutesElapsed: 0, // 调用时刻计算或传入
		Coins:          coins,
		Account:        entity.AccountData{},    // 需填充
		Positions:      []entity.PositionData{}, // 需填充
	}

	return promptData, nil
}
