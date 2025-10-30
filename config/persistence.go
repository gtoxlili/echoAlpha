package config

import (
	"encoding/json"
	"os"

	"github.com/gtoxlili/echoAlpha/entity"
)

type Persistence struct {
	PortfolioAnalysis string                          `json:"portfolio_analysis"`
	OpenPositions     map[string]entity.TradeMetadata `json:"open_positions"`
}

var AppPersistence *Persistence

func init() {
	// 从配置文件中获取
	file, err := os.Open(".echo-alpha-persistence.json")
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
