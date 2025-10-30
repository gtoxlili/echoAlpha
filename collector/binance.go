package collector

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/cinar/indicator"
	"github.com/gtoxlili/echoAlpha/entity"
	"github.com/samber/lo"
	lop "github.com/samber/lo/parallel"
	"golang.org/x/sync/errgroup"
)

const seriesLength = 30

type binanceProvider struct {
	client        *binance.Client
	futuresClient *futures.Client
	coins         []string
	createdAt     time.Time
}

func newBinanceProvider(apiKey, secretKey string, coins []string) *binanceProvider {
	client := binance.NewClient(apiKey, secretKey)
	futuresClient := binance.NewFuturesClient(apiKey, secretKey) // USDT-M Futures
	return &binanceProvider{
		client:        client,
		futuresClient: futuresClient,
		coins: lo.Map(coins, func(c string, _ int) string {
			return strings.ToUpper(c) + "USDT"
		}),
		createdAt: time.Now(),
	}
}

// AssemblePromptData (重构后)
func (b *binanceProvider) AssemblePromptData(ctx context.Context) (entity.PromptData, error) {
	var liveSymbolPrices sync.Map

	// lop.ForEach 会并行执行
	lop.ForEach(b.coins, func(symbol string, _ int) {
		coinData, err := b.fetchCoinData(ctx, symbol)
		if err != nil {
			log.Printf("error fetching data for %s: %v", symbol, err)
			return // 跳过这个代币
		}
		liveSymbolPrices.Store(symbol, coinData)
	})

	// TODO: 获取账户数据，仓位数据，构建 AccountData 和 Positions
	// accountData, err := b.fetchAccountData(ctx)
	// positionsData, err := b.fetchPositionsData(ctx)

	promptData := &entity.PromptData{
		MinutesElapsed: time.Since(b.createdAt).Minutes(),
		Coins:          make(map[string]entity.CoinData, len(b.coins)),
		Account:        entity.AccountData{},    // 需填充
		Positions:      []entity.PositionData{}, // 需填充
	}

	liveSymbolPrices.Range(func(key, value any) bool {
		symbol := strings.TrimRight(key.(string), "USDT")
		coinData := value.(entity.CoinData)
		promptData.Coins[symbol] = coinData
		return true
	})

	return *promptData, nil
}

// fetchFullKlines 获取完整的K线数据，用于计算ATR和Volume
func (b *binanceProvider) fetchFullKlines(ctx context.Context, symbol, interval string, limit int) (
	klines []*binance.Kline, high, low, close, volume []float64, err error,
) {
	klines, err = b.client.NewKlinesService().Symbol(symbol).Interval(interval).Limit(limit).Do(ctx)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	high = make([]float64, len(klines))
	low = make([]float64, len(klines))
	close = make([]float64, len(klines))
	volume = make([]float64, len(klines))

	for i, k := range klines {
		h, _ := strconv.ParseFloat(k.High, 64)
		l, _ := strconv.ParseFloat(k.Low, 64)
		c, _ := strconv.ParseFloat(k.Close, 64)
		// 币安返回的 Volume 是 float 字符串（例如 "1234.56"），我们先解析为 float 再转为 int64
		v, _ := strconv.ParseFloat(k.Volume, 64)

		high[i] = h
		low[i] = l
		close[i] = c
		volume[i] = v // 四舍五入为整数
	}
	return klines, high, low, close, volume, nil
}

// fetchOIFundingData 获取持仓量和资金费率
// 注意：这会发起3次API调用
func (b *binanceProvider) fetchOIFundingData(ctx context.Context, symbol string) (entity.OIFunding, error) {
	var result entity.OIFunding
	var eg errgroup.Group

	// 1. 获取最新资金费率
	eg.Go(func() error {
		// 资金费率和溢价指数
		res, err := b.futuresClient.NewPremiumIndexService().Symbol(symbol).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch premium index (funding rate): %w", err)
		}
		for _, r := range res {
			if r.Symbol == symbol {
				if err == nil {
					result.FundRate = r.LastFundingRate
				}
				break
			}
		}
		return nil
	})

	// 2. 获取最新持仓量
	eg.Go(func() error {
		res, err := b.futuresClient.NewGetOpenInterestService().Symbol(symbol).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch open interest: %w", err)
		}
		oi, err := strconv.ParseFloat(res.OpenInterest, 64)
		if err == nil {
			result.OILatest = oi
		}
		return nil
	})

	// 3. 获取持仓量历史数据（用于计算平均值）
	// 我们获取过去24小时的数据（288 * 5min = 24h）
	eg.Go(func() error {
		hist, err := b.futuresClient.NewOpenInterestStatisticsService().Symbol(symbol).Period("5m").Limit(288).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch open interest history: %w", err)
		}
		if len(hist) == 0 {
			return nil
		}

		result.OIAvg = lo.SumBy(hist, func(h *futures.OpenInterestStatistic) float64 {
			oi, _ := strconv.ParseFloat(h.SumOpenInterest, 64)
			return oi
		}) / float64(len(hist))

		return nil
	})

	if err := eg.Wait(); err != nil {
		return result, err
	}
	return result, nil
}

// fetchCoinData 为单个代币获取所有需要的数据
// 它在内部并行执行所有网络请求
func (b *binanceProvider) fetchCoinData(ctx context.Context, symbol string) (entity.CoinData, error) {
	var data entity.CoinData
	var eg, gctx = errgroup.WithContext(ctx)

	// 组 1: 获取当前价格
	eg.Go(func() error {
		price, err := b.fetchCurrentPrice(gctx, symbol)
		if err != nil {
			return fmt.Errorf("failed to fetch current price for %s: %w", symbol, err)
		}
		data.Price = price
		return nil
	})

	// 组 2: 获取 OIFunding 数据
	eg.Go(func() error {
		oiFunding, err := b.fetchOIFundingData(gctx, symbol)
		if err != nil {
			log.Printf("warning: failed to fetch OIFunding for %s: %v", symbol, err)
			return nil // 暂时忽略错误，避免整个数据失败
		}
		data.OIFunding = oiFunding
		return nil
	})

	// 组 3: 获取 Intraday (3m) 数据
	eg.Go(func() error {
		// 根据我们之前的讨论，100条数据足够预热和计算
		prices3m, err := b.fetchClosePrices(gctx, symbol, "3m", 100)
		if err != nil {
			return fmt.Errorf("failed to fetch 3m klines for %s: %w", symbol, err)
		}

		// 计算指标
		ema203m := indicator.Ema(20, prices3m)
		macd3m, _ := indicator.Macd(prices3m)
		_, rsi73m := indicator.RsiPeriod(7, prices3m)
		_, rsi143m := indicator.RsiPeriod(14, prices3m)

		const seriesLength = 30

		// 填充 Intraday 结构
		data.Intraday.Prices3m = lo.Subset(prices3m, -seriesLength, uint(seriesLength))
		data.Intraday.Ema203m = lo.Subset(ema203m, -seriesLength, uint(seriesLength))
		data.Intraday.Macd3m = lo.Subset(macd3m, -seriesLength, uint(seriesLength))
		data.Intraday.Rsi73m = lo.Subset(rsi73m, -seriesLength, uint(seriesLength))
		data.Intraday.Rsi143m = lo.Subset(rsi143m, -seriesLength, uint(seriesLength))

		// 填充 Snapshot 数据 (使用 3m 数据的最新值)
		data.EMA20 = lo.LastOrEmpty(ema203m)
		data.MACD = lo.LastOrEmpty(macd3m)
		data.RSI7 = lo.LastOrEmpty(rsi73m)

		return nil
	})

	// 组 4: 获取 LongTerm (4h) 数据
	eg.Go(func() error {
		_, high4h, low4h, close4h, vol4h, err := b.fetchFullKlines(gctx, symbol, "4h", 100)
		if err != nil {
			return fmt.Errorf("failed to fetch 4h klines for %s: %w", symbol, err)
		}

		// 计算指标
		ema204h := indicator.Ema(20, close4h)
		ema504h := indicator.Ema(50, close4h)
		_, atr34h := indicator.Atr(3, high4h, low4h, close4h)   // 假设 indicator.Atr 存在
		_, atr144h := indicator.Atr(14, high4h, low4h, close4h) // 假设 indicator.Atr 存在
		macd4h, _ := indicator.Macd(close4h)
		_, rsi144h := indicator.RsiPeriod(14, close4h)

		// 填充 LongTerm 结构
		data.LongTerm.Ema204h = lo.LastOrEmpty(ema204h)
		data.LongTerm.Ema504h = lo.LastOrEmpty(ema504h)
		data.LongTerm.Atr34h = lo.LastOrEmpty(atr34h)
		data.LongTerm.Atr144h = lo.LastOrEmpty(atr144h)
		data.LongTerm.VolCurr = lo.LastOrEmpty(vol4h)
		data.LongTerm.VolAvg = lo.Sum(vol4h) / float64(len(vol4h))

		// 只修改序列数据
		data.LongTerm.Macd4h = lo.Subset(macd4h, -seriesLength, uint(seriesLength))
		data.LongTerm.Rsi144h = lo.Subset(rsi144h, -seriesLength, uint(seriesLength))

		return nil
	})

	// 等待所有 goroutine 完成
	if err := eg.Wait(); err != nil {
		return lo.Empty[entity.CoinData](), err
	}

	return data, nil
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
