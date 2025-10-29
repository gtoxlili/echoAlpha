package collector

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/cinar/indicator"
	"github.com/gtoxlili/echoAlpha/entity"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
)

type binanceProvider struct {
	client    *binance.Client
	coins     []string
	createdAt time.Time
}

func newBinanceProvider(apiKey, secretKey string, coins []string) *binanceProvider {
	client := binance.NewClient(apiKey, secretKey)
	return &binanceProvider{
		client: client,
		coins: lo.Map(coins, func(c string, _ int) string {
			return strings.ToUpper(c) + "USDT"
		}),
		createdAt: time.Now(),
	}
}

// AssemblePromptData 汇总数据示例，需根据完整结构继续填充
func (b *binanceProvider) AssemblePromptData(ctx context.Context) (entity.PromptData, error) {
	var coinDatas sync.Map

	lop.ForEach(b.coins, func(symbol string, _ int) {
		currentPrice, err := b.fetchCurrentPrice(ctx, symbol)
		if err != nil {
			return
		}

		closePrices, err := b.fetchClosePrices(ctx, symbol, "1m", 30)
		if err != nil {
			return
		}

		ema20 := calculateEMA20(closePrices)
		macd, _, _ := calculateMACD(closePrices)
		rsi7 := calculateRSI(closePrices, 7)

		coinData := entity.CoinData{
			Price: currentPrice,
			EMA20: ema20,
			MACD:  macd,
			RSI7:  rsi7,
			// OIFunding, Intraday, LongTerm 另需实现
		}

		coinDatas.Store(symbol, coinData)
	})

	// TODO: 获取账户数据，仓位数据，构建 AccountData 和 Positions

	promptData := &entity.PromptData{
		MinutesElapsed: time.Since(b.createdAt).Minutes(),
		Coins:          make(map[string]entity.CoinData, len(b.coins)),
		Account:        entity.AccountData{},    // 需填充
		Positions:      []entity.PositionData{}, // 需填充
	}

	coinDatas.Range(func(key, value any) bool {
		symbol := key.(string)
		coinData := value.(entity.CoinData)
		promptData.Coins[symbol] = coinData
		return true
	})

	return *promptData, nil
}

func (b *binanceProvider) fetchCurrentPrice(ctx context.Context, symbol string) (float64, error) {
	prices, err := b.client.NewListSymbolTickerService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, err
	}
	for _, p := range prices {
		if p.Symbol == symbol {
			price, err := strconv.ParseFloat(p.LastPrice, 64)
			if err != nil {
				return 0, err
			}
			return price, nil
		}
	}
	return 0, errors.New("symbol not found in price list")
}

// 获取指定交易对指定周期的历史K线收盘价
func (b *binanceProvider) fetchClosePrices(ctx context.Context, symbol, interval string, limit int) ([]float64, error) {
	klines, err := b.client.NewKlinesService().Symbol(symbol).Interval(interval).Limit(limit).Do(ctx)
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
