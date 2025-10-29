package collector

import (
	"context"

	"github.com/gtoxlili/echoAlpha/entity"
)

type StateProvider interface {
	GetPromptData(ctx context.Context) (entity.PromptData, error)
}
