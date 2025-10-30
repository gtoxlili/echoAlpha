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
	"golang.org/x/sync/errgroup"
)

const (
	usdtSuffix = "USDT"
	// seriesLength 是我们为提示词中的时间序列数据（如MACD, RSI）保留的长度
	seriesLength = 30
	// klineInterval3m 3分钟K线
	klineInterval3m = "3m"
	// klineInterval4h 4小时K线
	klineInterval4h = "4h"
	// klineLimit 是获取K线的数量，需要足够长以计算EMA50等指标
	klineLimit = 100
	// oiPeriod OI 历史数据的周期
	oiPeriod = "5m"
	// oiLimit OI 历史数据的数量 (288 * 5分钟 = 24小时)
	oiLimit = 288
)

type binanceProvider struct {
	client    *futures.Client
	coins     []string
	createdAt time.Time
}

func newBinanceProvider(apiKey, secretKey string, coins []string) *binanceProvider {
	futuresClient := binance.NewFuturesClient(apiKey, secretKey) // USDT-M Futures
	return &binanceProvider{
		client: futuresClient,
		coins: lo.Map(coins, func(coin string, _ int) string {
			// 使用常量
			return strings.ToUpper(coin) + usdtSuffix
		}),
		createdAt: time.Now(),
	}
}

func (b *binanceProvider) AssemblePromptData(ctx context.Context) (entity.PromptData, error) {
	var (
		mu          sync.Mutex
		coinDataMap = make(map[string]entity.CoinData, len(b.coins))
		g, gctx     = errgroup.WithContext(ctx) // 'eg' -> 'g'
	)

	// 1. 并行获取所有代币数据
	for _, symbol := range b.coins {
		local := symbol
		g.Go(func() error {
			coinData, err := b.fetchCoinData(gctx, local)
			if err != nil {
				// 非致命错误：只记录日志，不中断整个组
				log.Printf("error fetching data for %s: %v", local, err)
				return nil
			}

			// 使用互斥锁安全地写入 map
			mu.Lock()
			coinDataMap[strings.TrimSuffix(local, usdtSuffix)] = coinData
			mu.Unlock()
			return nil
		})
	}

	// 2. TODO: 并行获取账户和仓位数据
	// 可以在同一个 errgroup 中添加更多的 Go 协程
	// var accountData entity.AccountData
	// g.Go(func() error { ... fetch account ... })
	// var positionsData []entity.PositionData
	// g.Go(func() error { ... fetch positions ... })

	if err := g.Wait(); err != nil {
		return lo.Empty[entity.PromptData](), err
	}

	return entity.PromptData{
		MinutesElapsed: time.Since(b.createdAt).Minutes(),
		Coins:          coinDataMap,
		Account:        lo.Empty[entity.AccountData](),    // 需填充
		Positions:      lo.Empty[[]entity.PositionData](), // 需填充
	}, nil
}

// fetchAndParseKlines 获取K线数据并将其解析为 float64 切片
func (b *binanceProvider) fetchAndParseKlines(ctx context.Context, symbol, interval string, limit int) (
	klines []*futures.Kline, high, low, close, volume []float64, err error,
) {
	klines, err = b.client.NewKlinesService().Symbol(symbol).Interval(interval).Limit(limit).Do(ctx)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	high = make([]float64, len(klines))
	low = make([]float64, len(klines))
	close = make([]float64, len(klines))
	volume = make([]float64, len(klines))

	for i, kline := range klines { // k -> kline
		h, _ := strconv.ParseFloat(kline.High, 64)
		l, _ := strconv.ParseFloat(kline.Low, 64)
		c, _ := strconv.ParseFloat(kline.Close, 64)
		v, _ := strconv.ParseFloat(kline.Volume, 64)

		high[i] = h
		low[i] = l
		close[i] = c
		volume[i] = v
	}
	return klines, high, low, close, volume, nil
}

// fetchOIFundingData 获取持仓量和资金费率
func (b *binanceProvider) fetchOIFundingData(ctx context.Context, symbol string) (entity.OIFunding, error) {
	var result entity.OIFunding
	var g errgroup.Group // 'eg' -> 'g'

	// 1. 获取最新资金费率
	g.Go(func() error {
		// 资金费率和溢价指数
		res, err := b.client.NewPremiumIndexService().Symbol(symbol).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch premium index (funding rate): %w", err)
		}
		// go-binance 即使指定了 symbol 也可能返回一个切片，遍历是安全的
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
	g.Go(func() error {
		res, err := b.client.NewGetOpenInterestService().Symbol(symbol).Do(ctx)
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
	g.Go(func() error {
		// --- 优化点 1: 使用常量 ---
		hist, err := b.client.NewOpenInterestStatisticsService().Symbol(symbol).Period(oiPeriod).Limit(oiLimit).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch open interest history: %w", err)
		}
		if len(hist) == 0 {
			return nil // 没有历史数据，不是一个错误
		}

		result.OIAvg = lo.SumBy(hist, func(h *futures.OpenInterestStatistic) float64 {
			oi, _ := strconv.ParseFloat(h.SumOpenInterest, 64)
			return oi
		}) / float64(len(hist))

		return nil
	})

	if err := g.Wait(); err != nil {
		return result, err
	}
	return result, nil
}

// fetchCoinData 为单个代币获取所有需要的数据
func (b *binanceProvider) fetchCoinData(ctx context.Context, symbol string) (entity.CoinData, error) {
	var data entity.CoinData
	var g, gctx = errgroup.WithContext(ctx) // 'eg' -> 'g'

	// 组 1: 获取当前价格
	g.Go(func() error {
		price, err := b.fetchCurrentPrice(gctx, symbol)
		if err != nil {
			return fmt.Errorf("failed to fetch current price for %s: %w", symbol, err)
		}
		data.Price = price
		return nil
	})

	// 组 2: 获取 OIFunding 数据
	g.Go(func() error {
		oiFunding, err := b.fetchOIFundingData(gctx, symbol)
		if err != nil {
			log.Printf("warning: failed to fetch OIFunding for %s: %v", symbol, err)
			return nil // 暂时忽略错误，避免整个数据失败
		}
		data.OIFunding = oiFunding
		return nil
	})

	// 组 3: 获取 Intraday (3m) 数据
	g.Go(func() error {
		// --- 优化点 1 & 3: 使用常量和新函数名 ---
		_, _, _, close3m, _, err := b.fetchAndParseKlines(gctx, symbol, klineInterval3m, klineLimit)
		if err != nil {
			return fmt.Errorf("failed to fetch 3m klines for %s: %w", symbol, err)
		}

		// 计算指标
		ema203m := indicator.Ema(20, close3m)
		macd3m, _ := indicator.Macd(close3m)
		_, rsi73m := indicator.RsiPeriod(7, close3m)
		_, rsi143m := indicator.RsiPeriod(14, close3m)

		// --- 优化点 4: 移除重复的 const seriesLength = 30 ---

		// 填充 Intraday 结构 (使用包级别的 seriesLength)
		data.Intraday.Prices3m = lo.Subset(close3m, -seriesLength, uint(seriesLength))
		data.Intraday.Ema203m = lo.Subset(ema203m, -seriesLength, uint(seriesLength))
		data.Intraday.MACD3m = lo.Subset(macd3m, -seriesLength, uint(seriesLength))
		data.Intraday.Rsi73m = lo.Subset(rsi73m, -seriesLength, uint(seriesLength))
		data.Intraday.Rsi143m = lo.Subset(rsi143m, -seriesLength, uint(seriesLength))

		// 填充 Snapshot 数据 (使用 3m 数据的最新值)
		data.EMA20 = lo.LastOrEmpty(ema203m)
		data.MACD = lo.LastOrEmpty(macd3m)
		data.RSI7 = lo.LastOrEmpty(rsi73m)

		return nil
	})

	// 组 4: 获取 LongTerm (4h) 数据
	g.Go(func() error {
		// --- 优化点 1 & 3: 使用常量和新函数名 ---
		_, high4h, low4h, close4h, vol4h, err := b.fetchAndParseKlines(gctx, symbol, klineInterval4h, klineLimit)
		if err != nil {
			return fmt.Errorf("failed to fetch 4h klines for %s: %w", symbol, err)
		}

		// 计算指标
		ema204h := indicator.Ema(20, close4h)
		ema504h := indicator.Ema(50, close4h)
		_, atr34h := indicator.Atr(3, high4h, low4h, close4h)
		_, atr144h := indicator.Atr(14, high4h, low4h, close4h)
		macd4h, _ := indicator.Macd(close4h)
		_, rsi144h := indicator.RsiPeriod(14, close4h)

		// 填充 LongTerm 结构
		data.LongTerm.Ema204h = lo.LastOrEmpty(ema204h)
		data.LongTerm.Ema504h = lo.LastOrEmpty(ema504h)
		data.LongTerm.Atr34h = lo.LastOrEmpty(atr34h)
		data.LongTerm.Atr144h = lo.LastOrEmpty(atr144h)
		data.LongTerm.VolCurr = lo.LastOrEmpty(vol4h)
		data.LongTerm.VolAvg = lo.Sum(vol4h) / float64(len(vol4h))

		// 只修改序列数据 (使用包级别的 seriesLength)
		data.LongTerm.MACD4h = lo.Subset(macd4h, -seriesLength, uint(seriesLength))
		data.LongTerm.Rsi144h = lo.Subset(rsi144h, -seriesLength, uint(seriesLength))

		return nil
	})

	// 等待所有 goroutine 完成
	if err := g.Wait(); err != nil {
		return lo.Empty[entity.CoinData](), err
	}

	return data, nil
}

func (b *binanceProvider) fetchCurrentPrice(ctx context.Context, symbol string) (float64, error) {
	prices, err := b.client.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, err
	}
	// 即使指定了 symbol，API 仍然返回一个切片，遍历是正确的
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
