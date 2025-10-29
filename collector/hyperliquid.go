package collector

import (
	"context"

	"github.com/gtoxlili/echoAlpha/entity"
)

type hyperliquidProvider struct{}

func (hlp *hyperliquidProvider) GetPromptData(ctx context.Context) (entity.PromptData, error) {
	panic("not implemented")
}
