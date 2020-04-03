package types

import "time"

// for mongo
//	orderid: 订单id
//	pair: 交易对
//	type: 买卖类型
//	rate: 买卖价格
//	amount: 订单买卖币种数量
//	time: 订单时间
//	time_unix: 订单unix时间戳
type Trade struct {
	TradeId     uint64    `bson:"_id"`
	OrderNumber uint64    `bson:"orderNumber"`
	Label       string    `bson:"label"`
	Pair        string    `bson:"pair"`
	Type        string    `bson:"type"`
	Rate        float64   `bson:"rate"` //成交价格
	Amount      float64   `bson:"amount"`
	Total       float64   `bson:"total"`
	DateString  string    `bson:"dateString"` // for log
	Date        time.Time `bson:"date"`
	TimeUnix    int64     `bson:"timeUnix"`
	Role        string    `bson:"role"`
	Fee         float64   `bson:"fee"`
	FeeCoin     string    `bson:"feeCoin"`
	GtFee       float64   `bson:"gtFee"`
	PointFee    float64   `bson:"pointFee"`
}

type Balance struct {
	Label string
	SERO  float64 `bson:"SERO"`
	USDT  float64 `bson:"USDT"`
	GT    float64 `bson:"GT"`
	Time  time.Time
}

type Order struct {
	OrderNumber   uint64
	CurrencyPair  string
	Type          string
	InitialRate   float64 // 下单价格
	InitialAmount float64 // 下单数量

	Status string

	Rate         float64
	Amount       float64
	FilledRate   float64
	FilledAmount float64

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
	PercentChange float64 //涨跌百分比
	BaseVolume    float64 //交易量
	QuoteVolume   float64 // 兑换货币交易量
	High24hr      float64 // 24小时最高价
	Low24hr       float64 // 24小时最低价
}
