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
	"github.com/gtoxlili/echoAlpha/utils"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

const (
	usdtSuffix      = "USDT"
	seriesLength    = 30
	klineInterval3m = "3m"
	klineInterval4h = "4h"
	klineLimit      = 100
	oiPeriod        = "5m"
	oiLimit         = 288
)

type binanceProvider struct {
	client    *futures.Client
	coins     []string
	createdAt time.Time
}

func newBinanceProvider(apiKey, secretKey string, coins []string) *binanceProvider {
	futuresClient := binance.NewFuturesClient(apiKey, secretKey)
	return &binanceProvider{
		client: futuresClient,
		coins: lo.Map(coins, func(coin string, _ int) string {
			return strings.ToUpper(coin) + usdtSuffix
		}),
		createdAt: time.Now(),
	}
}

func (b *binanceProvider) AssemblePromptData(ctx context.Context) (entity.PromptData, error) {
	var (
		mu          sync.Mutex
		coinDataMap = make(map[string]entity.CoinData, len(b.coins))
		g, gctx     = errgroup.WithContext(ctx)
	)

	for _, symbol := range b.coins {
		local := symbol
		g.Go(func() error {
			coinData, err := utils.RetryWithBackoff(func() (entity.CoinData, error) {
				return b.fetchCoinData(gctx, local)
			}, 5)
			if err != nil {
				log.Printf("error fetching data for %s: %v", local, err)
				return nil
			}

			mu.Lock()
			coinDataMap[strings.TrimSuffix(local, usdtSuffix)] = coinData
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return lo.Empty[entity.PromptData](), err
	}

	return entity.PromptData{
		MinutesElapsed: time.Since(b.createdAt).Minutes(),
		Coins:          coinDataMap,
		Account:        lo.Empty[entity.AccountData](), Positions: lo.Empty[[]entity.PositionData]()}, nil
}

func (b *binanceProvider) fetchCoinData(ctx context.Context, symbol string) (entity.CoinData, error) {
	var data entity.CoinData
	var g, gctx = errgroup.WithContext(ctx)

	g.Go(func() error {
		price, err := b.fetchCurrentPrice(gctx, symbol)
		if err != nil {
			return fmt.Errorf("failed to fetch current price for %s: %w", symbol, err)
		}
		data.Price = price
		return nil
	})

	g.Go(func() error {
		oiFunding, err := b.fetchOIFundingData(gctx, symbol)
		if err != nil {
			return fmt.Errorf("failed to fetch OIFunding for %s: %w", symbol, err)
		}
		data.OIFunding = oiFunding
		return nil
	})

	g.Go(func() error {
		_, _, _, close3m, _, err := b.fetchAndParseKlines(gctx, symbol, klineInterval3m, klineLimit)
		if err != nil {
			return fmt.Errorf("failed to fetch 3m klines for %s: %w", symbol, err)
		}

		ema203m := indicator.Ema(20, close3m)
		macd3m, _ := indicator.Macd(close3m)
		_, rsi73m := indicator.RsiPeriod(7, close3m)
		_, rsi143m := indicator.RsiPeriod(14, close3m)

		data.Intraday.Prices3m = lo.Subset(close3m, -seriesLength, uint(seriesLength))
		data.Intraday.Ema203m = lo.Subset(ema203m, -seriesLength, uint(seriesLength))
		data.Intraday.MACD3m = lo.Subset(macd3m, -seriesLength, uint(seriesLength))
		data.Intraday.Rsi73m = lo.Subset(rsi73m, -seriesLength, uint(seriesLength))
		data.Intraday.Rsi143m = lo.Subset(rsi143m, -seriesLength, uint(seriesLength))

		data.EMA20 = lo.LastOrEmpty(ema203m)
		data.MACD = lo.LastOrEmpty(macd3m)
		data.RSI7 = lo.LastOrEmpty(rsi73m)

		return nil
	})

	g.Go(func() error {
		_, high4h, low4h, close4h, vol4h, err := b.fetchAndParseKlines(gctx, symbol, klineInterval4h, klineLimit)
		if err != nil {
			return fmt.Errorf("failed to fetch 4h klines for %s: %w", symbol, err)
		}

		ema204h := indicator.Ema(20, close4h)
		ema504h := indicator.Ema(50, close4h)
		_, atr34h := indicator.Atr(3, high4h, low4h, close4h)
		_, atr144h := indicator.Atr(14, high4h, low4h, close4h)
		macd4h, _ := indicator.Macd(close4h)
		_, rsi144h := indicator.RsiPeriod(14, close4h)

		data.LongTerm.Ema204h = lo.LastOrEmpty(ema204h)
		data.LongTerm.Ema504h = lo.LastOrEmpty(ema504h)
		data.LongTerm.Atr34h = lo.LastOrEmpty(atr34h)
		data.LongTerm.Atr144h = lo.LastOrEmpty(atr144h)
		data.LongTerm.VolCurr = lo.LastOrEmpty(vol4h)
		data.LongTerm.VolAvg = lo.Sum(vol4h) / float64(len(vol4h))

		data.LongTerm.MACD4h = lo.Subset(macd4h, -seriesLength, uint(seriesLength))
		data.LongTerm.Rsi144h = lo.Subset(rsi144h, -seriesLength, uint(seriesLength))

		return nil
	})

	if err := g.Wait(); err != nil {
		return lo.Empty[entity.CoinData](), err
	}

	return data, nil
}

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

	for i, kline := range klines {
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

func (b *binanceProvider) fetchOIFundingData(ctx context.Context, symbol string) (entity.OIFunding, error) {
	var result entity.OIFunding
	var g errgroup.Group
	g.Go(func() error {
		res, err := b.client.NewPremiumIndexService().Symbol(symbol).Do(ctx)
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

	g.Go(func() error {
		hist, err := b.client.NewOpenInterestStatisticsService().Symbol(symbol).Period(oiPeriod).Limit(oiLimit).Do(ctx)
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

	if err := g.Wait(); err != nil {
		return result, err
	}
	return result, nil
}

func (b *binanceProvider) fetchCurrentPrice(ctx context.Context, symbol string) (float64, error) {
	prices, err := b.client.NewListPricesService().Symbol(symbol).Do(ctx)
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
