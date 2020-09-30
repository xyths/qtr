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
	TradeId uint64 `bson:"_id" gorm:"primary_key"`
	OrderId uint64 `bson:"orderId"`
	Symbol  string
	Type    string
	Price   string
	Amount  string
	Total   string
	Date    string
	Role    string
	Fee     map[string]string
}

type GateBalance struct {
	Label     string
	SERO      float64 `gorm:"Column:sero"`
	USDT      float64 `gorm:"Column:usdt"`
	GT        float64 `gorm:"Column:gt"`
	SeroPrice float64 `gorm:"Column:sero_price"`
	GtPrice   float64 `gorm:"Column:gt_price"`
	Time      time.Time
}

func TimestampToDate(timestamp int64) string {
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	return time.Unix(timestamp, 0).In(beijing).Format(layout)
}

