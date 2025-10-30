package trade

import "github.com/gtoxlili/echoAlpha/entity"

type Executor struct {
}

func NewExecutor() *Executor {
	return &Executor{}
}

func (te *Executor) Order(action entity.TradeSignal) error {
	panic("not implemented")
}

func (te *Executor) CloseOrder(symbol string) error {
	panic("not implemented")
}
