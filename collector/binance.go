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
	"github.com/gtoxlili/echoAlpha/config"
	"github.com/gtoxlili/echoAlpha/entity"
	"github.com/gtoxlili/echoAlpha/utils"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

const (
	usdtSuffix = "USDT"
)

type binanceProvider struct {
	client    *futures.Client
	coins     []string
	createdAt time.Time
	// <--- 新增字段 ---
	// 存储历史账户总价值，用于计算 Pct 和 Sharpe
	initialAccountValue     float64
	historicalAccountValues []float64
	historicalMu            sync.RWMutex // 用于保护 slice 的读写
}

func newBinanceProvider(apiKey, secretKey string, coins []string) *binanceProvider {
	provider := &binanceProvider{
		client: binance.NewFuturesClient(apiKey, secretKey),
		coins: lo.Map(coins, func(coin string, _ int) string {
			return strings.ToUpper(coin) + usdtSuffix
		}),
	}

	res, err := utils.RetryWithBackoff(func() (*futures.Account, error) {
		return provider.client.NewGetAccountService().Do(context.Background())
	}, 3)
	if err != nil {
		panic(err)
	}

	initialAmount, err := strconv.ParseFloat(res.TotalMarginBalance, 64)
	if err != nil {
		panic(err)
	}
	provider.historicalAccountValues = []float64{initialAmount}
	provider.initialAccountValue = initialAmount
	provider.createdAt = time.Now().Truncate(3 * time.Minute).Add(3 * time.Minute)

	return provider
}

func (b *binanceProvider) GetStartingCapital() float64 {
	return b.initialAccountValue
}

func (b *binanceProvider) AssemblePromptData(ctx context.Context) (entity.PromptData, error) {
	var (
		mu          sync.Mutex
		accountData entity.AccountData
		coinDataMap = make(map[string]entity.CoinData, len(b.coins))
		positions   []entity.PositionData
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

	// --- 2. 获取账户数据 ---
	g.Go(func() error {
		var err error
		// 同样使用 RetryWithBackoff
		accountData, err = utils.RetryWithBackoff(func() (entity.AccountData, error) {
			return b.fetchAccountData(gctx)
		}, 5)

		if err != nil {
			log.Printf("error fetching account data: %v", err)
			return nil // 同上，记录日志但不中断
		}
		return nil
	})

	g.Go(func() error {
		var err error
		// 同样使用 RetryWithBackoff
		positions, err = utils.RetryWithBackoff(func() ([]entity.PositionData, error) {
			return b.fetchPositionsData(gctx)
		}, 5)

		if err != nil {
			log.Printf("error fetching positions data: %v", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return lo.Empty[entity.PromptData](), err
	}

	return entity.PromptData{
		MinutesElapsed: time.Since(b.createdAt).Minutes(),
		Coins:          coinDataMap,
		Account:        accountData,
		Positions: lo.Map(positions, func(p entity.PositionData, _ int) entity.PositionData {
			if coinData, exists := coinDataMap[p.Symbol]; exists {
				p.CurrentPrice = coinData.Price
			}
			return p
		}),
	}, nil
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
		interval := fmt.Sprintf("%.0fm", config.KlineInterval.Minutes())
		_, _, _, close3m, _, err := b.fetchAndParseKlines(gctx, symbol, interval, config.KlineLimit)
		if err != nil {
			return fmt.Errorf("failed to fetch 3m klines for %s: %w", symbol, err)
		}

		ema203m := indicator.Ema(20, close3m)
		macd3m, _ := indicator.Macd(close3m)
		_, rsi73m := indicator.RsiPeriod(7, close3m)
		_, rsi143m := indicator.RsiPeriod(14, close3m)

		data.Intraday.Prices3m = lo.Subset(close3m, -config.SeriesLength, uint(config.SeriesLength))
		data.Intraday.Ema203m = lo.Subset(ema203m, -config.SeriesLength, uint(config.SeriesLength))
		data.Intraday.MACD3m = lo.Subset(macd3m, -config.SeriesLength, uint(config.SeriesLength))
		data.Intraday.Rsi73m = lo.Subset(rsi73m, -config.SeriesLength, uint(config.SeriesLength))
		data.Intraday.Rsi143m = lo.Subset(rsi143m, -config.SeriesLength, uint(config.SeriesLength))

		data.EMA20 = lo.LastOrEmpty(ema203m)
		data.MACD = lo.LastOrEmpty(macd3m)
		data.RSI7 = lo.LastOrEmpty(rsi73m)

		return nil
	})

	g.Go(func() error {
		interval := fmt.Sprintf("%.0fh", config.KlineIntervalLonger.Hours())
		_, high4h, low4h, close4h, vol4h, err := b.fetchAndParseKlines(gctx, symbol, interval, config.KlineLimit)
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

		data.LongTerm.MACD4h = lo.Subset(macd4h, -config.SeriesLength, uint(config.SeriesLength))
		data.LongTerm.Rsi144h = lo.Subset(rsi144h, -config.SeriesLength, uint(config.SeriesLength))

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
		hist, err := b.client.NewOpenInterestStatisticsService().Symbol(symbol).Period(config.OiPeriod).Limit(config.OiLimit).Do(ctx)
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

func (b *binanceProvider) fetchAccountData(ctx context.Context) (entity.AccountData, error) {
	res, err := b.client.NewGetAccountService().Do(ctx)
	if err != nil {
		return lo.Empty[entity.AccountData](), fmt.Errorf("failed to fetch account info: %w", err)
	}

	var data entity.AccountData
	var parseErr error

	// 1. 解析当前账户总价值
	currentValue, parseErr := strconv.ParseFloat(res.TotalMarginBalance, 64)
	if parseErr != nil {
		return lo.Empty[entity.AccountData](), fmt.Errorf("failed to parse TotalMarginBalance: %w", parseErr)
	}
	data.AccountValue = currentValue

	// CashAvailable 对应 USDT 资产的可用余额
	// 我们使用 lo.Find 在资产列表中查找 "USDT"
	usdtAsset, found := lo.Find(res.Assets, func(asset *futures.AccountAsset) bool {
		return asset.Asset == "USDT"
	})

	if found {
		data.CashAvailable, parseErr = strconv.ParseFloat(usdtAsset.AvailableBalance, 64)
		if parseErr != nil {
			return lo.Empty[entity.AccountData](), fmt.Errorf("error parsing USDT AvailableBalance: %v", parseErr)
		}
	} else {
		data.CashAvailable = 0.0
	}

	// 3. 锁定、更新历史数据并执行计算
	b.historicalMu.Lock()
	defer b.historicalMu.Unlock()

	b.historicalAccountValues = append(b.historicalAccountValues, currentValue)
	if len(b.historicalAccountValues) > config.MaxHistoricalValues {
		b.historicalAccountValues = b.historicalAccountValues[1:]
	}

	if b.initialAccountValue > 0 {
		data.ReturnPct = (currentValue - b.initialAccountValue) / b.initialAccountValue
	} else {
		data.ReturnPct = 0.0
	}

	// 6. 计算夏普比率 (SharpeRatio)
	data.SharpeRatio = b.calculateSharpeRatio() // 使用下面的新辅助函数

	return data, nil
}

// calculateSharpeRatio 是一个 binanceProvider 的方法
// 注意：此函数假定在调用它之前已经获取了 b.historicalMu 的锁！
func (b *binanceProvider) calculateSharpeRatio() float64 {
	values := b.historicalAccountValues

	// 至少需要3个数据点才能计算2个回报率，从而计算标准差
	if len(values) < 3 {
		return 0.0
	}

	// 1. 计算回报率序列
	// (P1-P0)/P0, (P2-P1)/P1, ...
	returns := make([]float64, len(values)-1)
	for i := 1; i < len(values); i++ {
		if values[i-1] == 0 { // 避免除以零
			returns[i-1] = 0.0
			continue
		}
		returns[i-1] = (values[i] - values[i-1]) / values[i-1]
	}

	// 2. 计算回报率的平均值
	avgReturn := utils.Avg(returns)

	// 3. 计算回报率的标准差
	stdDevReturn := utils.StdDev(returns)

	// 4. 计算夏普比率
	// Sharpe = (Average Return - Risk-Free Rate) / Standard Deviation
	// 我们假设 Risk-Free Rate (无风险利率) 为 0，这在短周期交易中很常见
	if stdDevReturn == 0 {
		return 0.0 // 避免除以零
	}

	return avgReturn / stdDevReturn
}

func (b *binanceProvider) fetchPositionsData(ctx context.Context) ([]entity.PositionData, error) {
	// NewGetPositionRiskService 会返回所有交易对的风险和持仓信息
	res, err := b.client.NewGetPositionRiskService().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch position risk: %w", err)
	}

	positions := make([]entity.PositionData, 0)
	for _, p := range res {
		quantity, _ := strconv.ParseFloat(p.PositionAmt, 64)

		// 过滤掉没有持仓的 (PositionAmt == 0)
		if quantity == 0 {
			continue
		}

		// 解析 API 返回的字符串
		entryPrice, _ := strconv.ParseFloat(p.EntryPrice, 64)
		liqPrice, _ := strconv.ParseFloat(p.LiquidationPrice, 64)
		pnl, _ := strconv.ParseFloat(p.UnRealizedProfit, 64)
		leverage, _ := strconv.ParseInt(p.Leverage, 10, 64)
		notional, _ := strconv.ParseFloat(p.Notional, 64)

		positions = append(positions, entity.PositionData{
			Symbol:        strings.TrimSuffix(p.Symbol, usdtSuffix), // 移除USDT后缀, 与 coinDataMap 统一
			Quantity:      quantity,
			EntryPrice:    entryPrice,
			LiqPrice:      liqPrice,
			UnrealizedPNL: pnl,
			Leverage:      int(leverage),
			NotionalUSD:   notional,
		})
	}

	return positions, nil
}
