package executor

import "github.com/shopspring/decimal"

type Signal struct {
	Direction int // 1: buy, -1: sell
	Price     decimal.Decimal
	Amount    decimal.Decimal
}