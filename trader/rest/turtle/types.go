package turtle

import (
	"github.com/xyths/hs"
	"time"
)

type RestTurtleStrategyConf struct {
	Total       float64
	Interval    string
	MaxPosition float64 `json:"maxPosition"` // max total can buy for one time
	MaxTimes    int     `json:"maxTimes"`    // max times for buy

	PeriodATR   int `json:"periodATR"`   //periodATR   = 14
	PeriodUpper int `json:"periodUpper"` //periodUpper = 20
	PeriodLower int `json:"periodLower"` //periodLower = 10
}

type Config struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Strategy RestTurtleStrategyConf
	Robots   []hs.BroadcastConf
}

type state struct {
	Position     int     `bson:"position"` // 0: empty, 1: open
	BuyTimes     int     `bson:"buyTimes"`
	SellTimes    int     `bson:"sellTimes"`
	LastBuyPrice float64 `bson:"lastBuyPrice"`

	LastModified time.Time `bson:"lastModified"`
}
