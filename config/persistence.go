package config

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/gtoxlili/echoAlpha/entity"
)

type Persistence struct {
	PortfolioAnalysis string                          `json:"portfolio_analysis"`
	OpenPositions     map[string]entity.TradeMetadata `json:"open_positions"`
}

var (
	AppPersistence *Persistence
	mu             sync.Mutex
)

func init() {
	// 从配置文件中获取
	file, err := os.Open(PersistencePath)
	if err != nil {
		AppPersistence = defaultPersistence()
		return
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&AppPersistence); err != nil {
		AppPersistence = defaultPersistence()
	}
}

func defaultPersistence() *Persistence {
	return &Persistence{
		PortfolioAnalysis: "No positions are open and no prior analysis exists. The market is a blank slate. My immediate goal is to analyze the full dataset provided, establish a market baseline, and find a single, high-quality entry point that aligns with the risk management protocol.",
		OpenPositions:     make(map[string]entity.TradeMetadata),
	}
}

func SavePortfolioPersistence(portfolioAnalysis string) error {
	mu.Lock()
	defer mu.Unlock()
	AppPersistence.PortfolioAnalysis = portfolioAnalysis
	file, err := os.OpenFile(PersistencePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(AppPersistence)
}

func SaveOpenPositions(openPositions map[string]entity.TradeMetadata) error {
	mu.Lock()
	defer mu.Unlock()
	AppPersistence.OpenPositions = openPositions
	file, err := os.OpenFile(PersistencePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(AppPersistence)
}
