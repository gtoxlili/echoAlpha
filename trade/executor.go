package trade

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/gtoxlili/echoAlpha/entity"
)

const (
	usdtSuffix = "USDT"
)

type Executor struct {
	client *futures.Client
}

// NewExecutor 必须接收 API key 和 secret key 才能执行交易。
// 你的主程序 main.go 需要修改为:
// tradeExecutor := trade.NewExecutor(os.Getenv("BINANCE_API_KEY"), os.Getenv("BINANCE_SECRET_KEY"))
func NewExecutor(apiKey, secretKey string) *Executor {
	if apiKey == "" || secretKey == "" {
		log.Println("⚠️ [Executor] 警告: APIKey 或 SecretKey 为空。交易执行将失败。")
	}
	return &Executor{
		client: futures.NewClient(apiKey, secretKey),
	}
}

func (te *Executor) Order(ctx context.Context, action entity.TradeSignal) error {
	symbol := action.Coin + usdtSuffix
	var entrySide, closeSide futures.SideType

	if action.Signal == "buy_to_enter" {
		entrySide = futures.SideTypeBuy
		closeSide = futures.SideTypeSell
	} else if action.Signal == "sell_to_enter" {
		entrySide = futures.SideTypeSell
		closeSide = futures.SideTypeBuy
	} else {
		return fmt.Errorf("[Executor] 收到无效的开仓信号: %s", action.Signal)
	}

	// --- 1. 设置杠杆 ---
	log.Printf("[Executor] 正在为 %s 设置 %dx 杠杆...", symbol, action.Leverage)
	_, err := te.client.NewChangeLeverageService().
		Symbol(symbol).
		Leverage(action.Leverage).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("设置杠杆失败 for %s: %w", symbol, err)
	}
	log.Printf("[Executor] %s 杠杆设置成功。", symbol)

	// --- 2. 准备批量订单 ---
	// 使用 FormatFloat 'f', -1 来自动匹配精度
	quantityStr := strconv.FormatFloat(action.Quantity, 'f', -1, 64)
	stopLossStr := strconv.FormatFloat(action.StopLoss, 'f', -1, 64)
	profitTargetStr := strconv.FormatFloat(action.ProfitTarget, 'f', -1, 64)

	orderServices := make([]*futures.CreateOrderService, 0, 3)

	// 订单 1: 市价入场单
	entryOrder := te.client.NewCreateOrderService().
		Symbol(symbol).
		Side(entrySide).
		Type(futures.OrderTypeMarket).
		Quantity(quantityStr)

	orderServices = append(orderServices, entryOrder)

	// 订单 2: 止损单 (STOP_MARKET)
	stopLossOrder := te.client.NewCreateOrderService().
		Symbol(symbol).
		Side(closeSide).
		Type(futures.OrderTypeStopMarket).
		StopPrice(stopLossStr).                    // 止损触发价
		WorkingType(futures.WorkingTypeMarkPrice). // 使用标记价格防止插针
		ClosePosition(true)                        // 关键：表明这是一个平仓单

	orderServices = append(orderServices, stopLossOrder)

	takeProfitOrder := te.client.NewCreateOrderService().
		Symbol(symbol).
		Side(closeSide).
		Type(futures.OrderTypeTakeProfitMarket).
		StopPrice(profitTargetStr). // 止盈触发价
		WorkingType(futures.WorkingTypeMarkPrice).
		ClosePosition(true) // 关键：表明这是一个平仓单

	orderServices = append(orderServices, takeProfitOrder)

	// --- 3. 批量执行 ---
	log.Printf("[Executor] 正在为 %s 批量提交开仓、止损、止盈订单...", symbol)
	_, err = te.client.NewCreateBatchOrdersService().
		OrderList(orderServices).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("批量下单失败 for %s: %w", symbol, err)
	}

	log.Printf("[Executor] %s 批量下单成功 (开仓, 止损, 止盈)。", symbol)
	return nil
}

// CloseOrder 负责执行 AI 的 "close" 平仓信号
// 它的逻辑是:
// 1. 获取当前持仓
// 2. 取消该币种所有挂单 (即 SL/TP)
// 3. 提交一个反向的市价单来平仓
func (te *Executor) CloseOrder(ctx context.Context, symbol string) error {
	symbolWithSuffix := symbol + usdtSuffix
	log.Printf("[Executor] 正在为 %s 准备平仓...", symbol)

	// --- 1. 获取当前持仓信息 ---
	// 我们必须先查询持仓，以确定平仓的 方向(Side) 和 数量(Quantity)
	positions, err := te.client.NewGetPositionRiskService().
		Symbol(symbolWithSuffix).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("平仓失败: 无法获取 %s 的持仓信息: %w", symbol, err)
	}
	if len(positions) == 0 {
		return fmt.Errorf("平仓失败: %s 没有返回持仓信息", symbol)
	}

	position := positions[0] // 我们只查询了一个 symbol
	quantity, err := strconv.ParseFloat(position.PositionAmt, 64)
	if err != nil {
		return fmt.Errorf("平仓失败: 无法解析持仓数量 '%s': %w", position.PositionAmt, err)
	}

	if quantity == 0 {
		log.Printf("[Executor] %s 持仓已为0，无需平仓。但仍将尝试取消挂单。", symbol)
		return te.cancelAllOrders(ctx, symbolWithSuffix)
	}

	// 确定平仓方向
	var closeSide futures.SideType
	if quantity > 0 { // 当前是多头 (Long)
		closeSide = futures.SideTypeSell // 平仓需要 "Sell"
	} else { // 当前是空头 (Short)
		closeSide = futures.SideTypeBuy // 平仓需要 "Buy"
	}

	// --- 2. 取消所有相关挂单 (SL/TP) ---
	// 必须在提交平仓单 *之前* 执行，否则可能导致SL/TP单被触发
	log.Printf("[Executor] 正在取消 %s 的所有挂单 (SL/TP)...", symbol)
	if err := te.cancelAllOrders(ctx, symbolWithSuffix); err != nil {
		return err // 错误已在辅助函数中格式化
	}
	log.Printf("[Executor] %s 挂单取消成功。", symbol)

	// --- 3. 提交市价平仓单 ---
	// 数量必须是正数（绝对值）
	closeQuantityStr := strconv.FormatFloat(math.Abs(quantity), 'f', -1, 64)

	log.Printf("[Executor] 正在提交 %s 的市价平仓单 (Side: %s, Qty: %s)...", symbol, closeSide, closeQuantityStr)
	_, err = te.client.NewCreateOrderService().
		Symbol(symbolWithSuffix).
		Side(closeSide).
		Type(futures.OrderTypeMarket).
		Quantity(closeQuantityStr).
		ReduceOnly(true). // 关键：确保此订单只平仓，不会反向开仓
		Do(ctx)

	if err != nil {
		return fmt.Errorf("市价平仓单提交失败 for %s: %w", symbol, err)
	}

	log.Printf("[Executor] %s 市价平仓单提交成功。", symbol)
	return nil
}

// cancelAllOrders 是一个辅助函数，用于取消指定 symbol 的所有挂单
func (te *Executor) cancelAllOrders(ctx context.Context, symbolWithSuffix string) error {
	err := te.client.NewCancelAllOpenOrdersService().
		Symbol(symbolWithSuffix).
		Do(ctx)
	if err != nil {
		// 注意: 如果本身没有挂单，币安 API 也会返回一个错误 (e.g., code -2011 "Unknown order sent.")
		// 我们可以选择忽略特定错误码，但为了安全起见，我们暂时记录所有错误
		// 在生产环境中，你可能需要解析 'err' 并忽略 "no open orders" 类型的错误。
		log.Printf("⚠️ [Executor] 取消 %s 的挂单时遇到问题: %v (如果无挂单，此错误可忽略)", symbolWithSuffix, err)
		// 暂不返回
	}
	return nil
}
