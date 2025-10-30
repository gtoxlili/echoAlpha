package main

import (
	"context"
	"log"
	"time"

	"github.com/gtoxlili/echoAlpha/collector"
	"github.com/gtoxlili/echoAlpha/constant"
	"github.com/gtoxlili/echoAlpha/llm"
	"github.com/gtoxlili/echoAlpha/trade"
)

const klineInterval = 3 * time.Minute

// 假设 assetUniverse 在这里定义，或者从配置加载
var assetUniverse = []string{"BTC", "ETH", "AERO", "BNB"}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("🤖 交易机器人启动...")
	provider := collector.ResolveCollector("Binance", assetUniverse)
	startingCapital := provider.GetStartingCapital()

	agent, err := llm.NewAgent("Binance", assetUniverse, "doubao-seed-1-6-251015", startingCapital)
	if err != nil {
		log.Panicf("❌ [初始化] 致命错误: 无法创建 AI Agent: %v", err)
	}

	tradeManager := trade.NewManager()
	tradeExecutor, err := trade.NewExecutor(constant.BINANCE_API_KEY, constant.BINANCE_API_SECRET)
	if err != nil {
		log.Panicf("❌ [初始化] 致命错误: 无法创建 Trade Executor: %v", err)
	}

	log.Printf("... 交易所: Binance, 模型: %s", "doubao-seed-1-6-251015")
	log.Printf("... 初始资本: $%.2f", startingCapital)
	log.Printf("... 决策周期: 3 分钟")

	now := time.Now()
	nextTickTime := now.Truncate(klineInterval).Add(klineInterval)
	durationToWait := time.Until(nextTickTime)
	log.Printf("... 当前时间: %s", now.Format("2006-01-02 15:04:05"))
	log.Printf("... K线对齐: 等待 %v, 将在 %s 执行首次分析...", durationToWait.Round(time.Second), nextTickTime.Format("15:04:05"))

	// 启动主循环
	for {
		if err := delay(ctx); err != nil {
			log.Printf("❌ 主循环延迟错误: %v", err)
			return
		}
		runDecisionCycle(ctx, provider, agent, tradeManager, tradeExecutor)
	}
}

/**
 * runDecisionCycle 封装了单次决策的完整流程
 */
func runDecisionCycle(
	ctx context.Context,
	provider collector.StateProvider,
	agent *llm.Agent,
	tradeManager *trade.Manager,
	tradeExecutor *trade.Executor,
) {
	log.Println("----------- 决策周期开始 -----------")
	defer log.Println("----------- 决策周期结束 -----------")

	// --- 步骤 1: 数据采集 ---
	log.Println("🔄 1. [数据采集] 正在从 Binance 获取最新市场数据...")
	data, err := provider.AssemblePromptData(ctx)
	if err != nil {
		log.Printf("❌ [数据采集] 错误: %v", err)
		return // 非致命错误，等待下个周期
	}
	log.Printf("✅ 1. [数据采集] 完成。账户价值: $%.2f", data.Account.AccountValue)

	// --- 步骤 2: 状态合并 ---
	log.Println("🔄 2. [状态合并] 正在合并本地元数据与交易所持仓...")
	mergedPositions := 0
	// 暂不考虑 "僵尸" 持仓 的情况
	for idx, position := range data.Positions {
		meta, exists := tradeManager.Get(position.Symbol)
		if !exists {
			// 这是 API 有持仓，但我们本地没有元数据的情况 (僵尸持仓)
			// 根据你的要求，我们暂不处理，直接跳过
			continue
		}

		// 假设 data.Positions[idx] 的结构体中已经包含了 ExitPlan, Confidence 等字段
		// 注意：这要求 entity.PositionData 结构体被修改过，以包含这些字段
		data.Positions[idx].ExitPlan.ProfitTarget = meta.ProfitTarget
		data.Positions[idx].ExitPlan.StopLoss = meta.StopLoss
		data.Positions[idx].ExitPlan.InvalidCond = meta.InvalidationCondition
		data.Positions[idx].Confidence = meta.Confidence
		data.Positions[idx].RiskUSD = meta.RiskUSD
		data.Positions[idx].AgeInMinutes = time.Since(meta.EntryTime).Minutes()

		log.Printf("   ... 合并持仓 %s (已持仓 %.0f 分钟)", position.Symbol, data.Positions[idx].AgeInMinutes)
		mergedPositions++
	}
	log.Printf("✅ 2. [状态合并] 完成。共合并 %d 个持仓的元数据。", mergedPositions)

	// --- 步骤 3: AI 分析 ---
	log.Println("🧠 3. [AI分析] 正在将数据提交给 LLM 进行分析...")
	actions, err := agent.RunAnalysis(ctx, data)
	if err != nil {
		log.Printf("❌ [AI分析] 错误: %v", err)
		return // AI 分析失败，等待下个周期
	}

	// --- 步骤 4: 决策与执行 ---
	log.Printf("🤖 4. [AI决策] 收到 %d 个决策。", len(actions))
	if len(actions) == 0 {
		log.Println("   ... 决策为空: AI 决定 [持有/无操作]。")
		return
	}

	log.Println("📈 5. [交易执行] 正在处理决策...")
	for _, action := range actions {
		switch action.Signal {
		case "buy_to_enter", "sell_to_enter":
			// --- 修改后的日志 ---
			log.Printf("   ... 🟩 [开仓] 信号: %s, 币种: %s, 数量: %f, 杠杆: %d",
				action.Signal, action.Coin, action.Quantity, action.Leverage)
			// 【修改点】将 止盈/止损 替换为 InvalidationCondition
			log.Printf("   ...    ├─ 失效条件: %s, 信心: %.2f",
				action.InvalidationCondition, action.Confidence)
			log.Printf("   ...    └─ 理由: %s", action.Justification)
			// --- 日志结束 ---

			execErr := tradeExecutor.Order(ctx, action)
			if execErr == nil {
				tradeManager.Add(action) // 交易成功, *更新本地状态*
				log.Printf("   ... ✅ [开仓] 订单执行成功，已添加 %s 到持仓管理器。", action.Coin)
			} else {
				log.Printf("   ... ❗ [开仓] 订单执行失败: %s, 错误: %v", action.Coin, execErr)
			}
		case "close":
			log.Printf("   ... 🟥 [平仓] 信号: %s, 币种: %s", action.Signal, action.Coin)
			log.Printf("   ...    └─ 理由: %s", action.Justification)

			execErr := tradeExecutor.CloseOrder(ctx, action.Coin)
			if execErr == nil {
				tradeManager.Remove(action.Coin) // 交易成功, *更新本地状态*
				log.Printf("   ... ✅ [平仓] 订单执行成功，已从持仓管理器移除 %s。", action.Coin)
			} else {
				log.Printf("   ... ❗ [平仓] 订单执行失败: %s, 错误: %v", action.Coin, execErr)
			}
		}
	}
}

func delay(ctx context.Context) error {
	next := time.Now().Truncate(klineInterval).Add(klineInterval)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Until(next)):
		return nil
	}
}
