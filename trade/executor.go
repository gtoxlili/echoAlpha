package trade

import (
	"context"

	"github.com/gtoxlili/echoAlpha/entity"
)

type Executor struct {
}

func NewExecutor() *Executor {
	return &Executor{}
}

func (te *Executor) Order(ctx context.Context, action entity.TradeSignal) error {
	panic("not implemented")
}

func (te *Executor) CloseOrder(ctx context.Context, symbol string) error {
	panic("not implemented")
}
