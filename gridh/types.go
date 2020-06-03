package gridh

import "github.com/shopspring/decimal"

type Asset struct {
	Quote decimal.Decimal // quote currency, USDT
	Long  decimal.Decimal // long etf, btc3l
	Short decimal.Decimal // short etf btc3s
	Fee   decimal.Decimal // fee reserve
}

type GridStatus struct {
	Id         int
	Base       bool
	Symbol     string
	Total      string
	Price      string
	AmountBuy  string `bson:"amountBuy"`
	AmountSell string `bson:"amountSell"`
	Order      uint64
}
