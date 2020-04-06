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
	TradeId     uint64    `bson:"_id" gorm:"primary_key"`
	OrderNumber uint64    `bson:"orderNumber"`
	Label       string    `bson:"label"`
	Pair        string    `bson:"pair"`
	Type        string    `bson:"type"`
	Rate        float64   `bson:"rate"` //成交价格
	Amount      float64   `bson:"amount"`
	Total       float64   `bson:"total"`
	Date        time.Time `bson:"date"`
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
