package collector

import (
	"context"
	"echoAlpha/entity"
)

type StateProvider interface {
	GetPromptData(ctx context.Context) (entity.PromptData, error)
}
