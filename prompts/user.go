package prompts

import (
	"fmt"
	"strings"

	"github.com/gtoxlili/echoAlpha/entity"
)

// promptTemplate (主模板)
// 关键变更：所有币种的静态部分被替换为 {all_coins_data_block}
const promptTemplate = `It has been {minutes_elapsed} minutes since you started trading.

Below, we are providing you with a variety of collector data, price data, and predictive signals so you can discover alpha. Below that is your current account information, value, performance, positions, etc.

⚠️ **CRITICAL: ALL OF THE PRICE OR SIGNAL DATA BELOW IS ORDERED: OLDEST → NEWEST**

**Timeframes note:** Unless stated otherwise in a section title, intraday series are provided at **3-minute intervals**. If a coin uses a different interval, it is explicitly stated in that coin's section.

---

## CURRENT MARKET STATE FOR ALL COINS

{all_coins_data_block}

## HERE IS YOUR ACCOUNT INFORMATION & PERFORMANCE

**Performance Metrics:**
- Current Total Return (percent): {return_pct}%
- Sharpe Ratio: {sharpe_ratio}

**Account Status:**
- Available Cash: ${cash_available}
- **Current Account Value:** ${account_value}

**Current Live Positions & Performance:**

` + "```json" + `
{positions_block}
` + "```" + `

Based on the above data, provide your trading decision in the required JSON format.
`

// coinDataTemplate (新增的子模板)
// 这是用于在循环中为每个币种生成内容的模板。
// 注意占位符是通用的（例如 {price}, {ema20}）
const coinDataTemplate = `### ALL {symbol} DATA

**Current Snapshot:**
- current_price = {price}
- current_ema20 = {ema20}
- current_macd = {macd}
- current_rsi (7 period) = {rsi7}

**Perpetual Futures Metrics:**
- Open Interest: Latest: {oi_latest} | Average: {oi_avg}
- Funding Rate: {funding_rate}

**Intraday Series (3-minute intervals, oldest → latest):**

Mid prices: [{prices_3m}]

EMA indicators (20-period): [{ema20_3m}]

MACD indicators: [{macd_3m}]

RSI indicators (7-Period): [{rsi7_3m}]

RSI indicators (14-Period): [{rsi14_3m}]

**Longer-term Context (4-hour timeframe):**

20-Period EMA: {ema20_4h} vs. 50-Period EMA: {ema50_4h}

3-Period ATR: {atr3_4h} vs. 14-Period ATR: {atr14_4h}

Current Volume: {volume_current} vs. Average Volume: {volume_avg}

MACD indicators (4h): [{macd_4h}]

RSI indicators (14-Period, 4h): [{rsi14_4h}]

---
`

// --- 3. 辅助函数 ---

// formatFloatSlice 将 []float64 转换为 "val1, val2, val3" 格式的字符串
func formatFloatSlice(slice []float64) string {
	var b strings.Builder
	for i, v := range slice {
		b.WriteString(fmt.Sprintf("%.4f", v))
		if i < len(slice)-1 {
			b.WriteString(", ")
		}
	}
	return b.String()
}

func formatPositions(positions []entity.PositionData) string {
	if len(positions) == 0 {
		return "[]"
	}

	var b strings.Builder
	b.WriteString("[\n")
	for i, p := range positions {
		b.WriteString(fmt.Sprintf(
			`  {
    'symbol': '%s',
    'quantity': %f,
    'entry_price': %f,
    'current_price': %f,
    'liquidation_price': %f,
    'unrealized_pnl': %f,
    'leverage': %d,
    'exit_plan': {
      'profit_target': %f,
      'stop_loss': %f,
      'invalidation_condition': '%s'
    },
    'confidence': %f,
    'risk_usd': %f,
    'notional_usd': %f
  }`,
			p.Symbol, p.Quantity, p.EntryPrice, p.CurrentPrice, p.LiqPrice,
			p.UnrealizedPNL, p.Leverage, p.ExitPlan.ProfitTarget, p.ExitPlan.StopLoss,
			p.ExitPlan.InvalidCond, p.Confidence, p.RiskUSD, p.NotionalUSD,
		))

		if i < len(positions)-1 {
			b.WriteString(",\n")
		}
	}
	b.WriteString("\n]")
	return b.String()
}

// buildAllCoinsBlock (新增的辅助函数)
// 动态构建所有币种的数据块
func buildAllCoinsBlock(coins map[string]entity.CoinData) string {
	var b strings.Builder

	// 按照 assetUniverse 中定义的顺序循环
	for symbol, coinData := range coins {
		prices3mStr := formatFloatSlice(coinData.Prices3m)
		ema20_3mStr := formatFloatSlice(coinData.EMA20_3m)
		macd3mStr := formatFloatSlice(coinData.MACD_3m)
		rsi7_3mStr := formatFloatSlice(coinData.RSI7_3m)
		rsi14_3mStr := formatFloatSlice(coinData.RSI14_3m)
		macd4hStr := formatFloatSlice(coinData.MACD_4h)
		rsi14_4hStr := formatFloatSlice(coinData.RSI14_4h)

		r := strings.NewReplacer(
			"{symbol}", symbol,
			"{price}", fmt.Sprintf("%.4f", coinData.Price),
			"{ema20}", fmt.Sprintf("%.4f", coinData.EMA20),
			"{macd}", fmt.Sprintf("%.4f", coinData.MACD),
			"{rsi7}", fmt.Sprintf("%.4f", coinData.RSI7),
			"{oi_latest}", fmt.Sprintf("%.4f", coinData.OILatest),
			"{oi_avg}", fmt.Sprintf("%.4f", coinData.OIAvg),
			"{funding_rate}", fmt.Sprintf("%s", coinData.FundRate),
			"{prices_3m}", prices3mStr,
			"{ema20_3m}", ema20_3mStr,
			"{macd_3m}", macd3mStr,
			"{rsi7_3m}", rsi7_3mStr,
			"{rsi14_3m}", rsi14_3mStr,
			"{ema20_4h}", fmt.Sprintf("%.4f", coinData.EMA20_4h),
			"{ema50_4h}", fmt.Sprintf("%.4f", coinData.EMA50_4h),
			"{atr3_4h}", fmt.Sprintf("%.4f", coinData.ATR3_4h),
			"{atr14_4h}", fmt.Sprintf("%.4f", coinData.ATR14_4h),
			"{volume_current}", fmt.Sprintf("%.4f", coinData.VolCurr),
			"{volume_avg}", fmt.Sprintf("%.4f", coinData.VolAvg),
			"{macd_4h}", macd4hStr,
			"{rsi14_4h}", rsi14_4hStr,
		)

		// 将币种模板应用替换并附加到主构建器
		b.WriteString(r.Replace(coinDataTemplate))
	}

	return b.String()
}

func BuildUserPrompt(data entity.PromptData) string {

	allCoinsBlockStr := buildAllCoinsBlock(data.Coins)

	positionsStr := formatPositions(data.Positions)

	r := strings.NewReplacer(
		"{minutes_elapsed}", fmt.Sprintf("%.0f", data.MinutesElapsed),
		"{all_coins_data_block}", allCoinsBlockStr, // 替换整个币种块

		// --- 账户字段 ---
		"{return_pct}", fmt.Sprintf("%.4f", data.Account.ReturnPct),
		"{sharpe_ratio}", fmt.Sprintf("%.4f", data.Account.SharpeRatio),
		"{cash_available}", fmt.Sprintf("%.4f", data.Account.CashAvailable),
		"{account_value}", fmt.Sprintf("%.4f", data.Account.AccountValue),

		// --- 仓位块 ---
		"{positions_block}", positionsStr,
	)

	// 4. 执行替换并返回
	return r.Replace(promptTemplate)
}
