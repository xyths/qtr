package exchange

import (
	"github.com/shopspring/decimal"
	"time"
)

type Order struct {
	OrderNumber   uint64
	CurrencyPair  string
	Type          string
	InitialRate   decimal.Decimal // 下单价格
	InitialAmount decimal.Decimal // 下单数量

	Status string

	Rate         decimal.Decimal
	Amount       decimal.Decimal
	FilledRate   decimal.Decimal
	FilledAmount decimal.Decimal

	FeePercentage float64
	FeeValue      float64
	FeeCurrency   string
	Fee           float64

	Timestamp int64
}

type Ticker struct {
	Last          float64 // 最新成交价
	LowestAsk     float64 // 卖1，卖方最低价
	HighestBid    float64 // 买1，买方最高价
	PercentChange float64 // 涨跌百分比
	BaseVolume    float64 // 交易量
	QuoteVolume   float64 // 兑换货币交易量
	High24hr      float64 // 24小时最高价
	Low24hr       float64 // 24小时最低价
}

type Candle struct {
	Timestamp uint64 `gorm:"primary_key"`
	Open      float64
	Close     float64
	High      float64
	Low       float64
	Volume    float64
}

type Balance struct {
	Time     time.Time
	Exchange string
	Label    string
	Currency string
	Amount   float64
}
