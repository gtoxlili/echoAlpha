package trade

import (
	"log"
	"sync"
	"time"

	"github.com/gtoxlili/echoAlpha/entity"
)

type Manager struct {
	mu sync.RWMutex
	// openPositions 的 key 是 symbol (例如 "BTC"), value 是我们存储的元数据
	openPositions map[string]entity.TradeMetadata
}

func NewManager() *Manager {
	return &Manager{
		openPositions: make(map[string]entity.TradeMetadata),
	}
}

// Add 在 AI 决定开仓并且订单 *成功执行* 后被调用
func (tm *Manager) Add(decision entity.TradeSignal) {
	if decision.Signal != "buy_to_enter" && decision.Signal != "sell_to_enter" {
		return
	}

	metadata := entity.TradeMetadata{
		Symbol:                decision.Coin,
		EntryTime:             time.Now(), // <-- 关键：在执行时记录当前时间
		ProfitTarget:          decision.ProfitTarget,
		StopLoss:              decision.StopLoss,
		InvalidationCondition: decision.InvalidationCondition,
		Confidence:            decision.Confidence,
		RiskUSD:               decision.RiskUSD,
		Justification:         decision.Justification,
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.openPositions[decision.Coin] = metadata
	log.Printf("Manager: Added new position %s", decision.Coin)
}

// Remove 在 AI 决定平仓并且订单 *成功执行* 后被调用
func (tm *Manager) Remove(symbol string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if _, ok := tm.openPositions[symbol]; ok {
		delete(tm.openPositions, symbol)
		log.Printf("Manager: Removed position %s", symbol)
	}
}

// Get 返回单个持仓的元数据
func (tm *Manager) Get(symbol string) (entity.TradeMetadata, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	meta, ok := tm.openPositions[symbol]
	return meta, ok
}

// GetAll 返回所有当前持仓的元数据
func (tm *Manager) GetAll() map[string]entity.TradeMetadata {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// 返回一个副本以保证线程安全
	clone := make(map[string]entity.TradeMetadata, len(tm.openPositions))
	for k, v := range tm.openPositions {
		clone[k] = v
	}
	return clone
}
