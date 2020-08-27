package rest

import (
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/qtr/gateio"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type SqueezeMomentumConfig struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Strategy SqueezeMomentumStrategyConf
	Robots   []hs.BroadcastConf
}

type SqueezeMomentumStrategyConf struct {
	Total    float64
	Interval string
	Factor   float64
	Period   int
}

type SqueezeMomentumTrader struct {
	config   SqueezeMomentumConfig
	interval time.Duration

	db     *mongo.Database
	ex     *gateio.GateIO
	robots []broadcast.Broadcaster

	quoteCurrency string // cash, eg. USDT
	baseSymbol    string
	maxTotal      decimal.Decimal // max total for buy order, half total in config

	longSymbol           string
	longCurrency         string
	longPricePrecision   int32
	longAmountPrecision  int32
	longMinAmount        decimal.Decimal
	longMinTotal         decimal.Decimal
	shortSymbol          string
	shortCurrency        string
	shortPricePrecision  int32
	shortAmountPrecision int32
	shortMinAmount       decimal.Decimal
	shortMinTotal        decimal.Decimal

	orderId    string
	position   int
	LongTimes  int
	ShortTimes int

	//balance   map[string]decimal.Decimal
}
