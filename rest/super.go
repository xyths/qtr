package rest

import (
	"context"
	"fmt"
	"github.com/google/martian/log"
	"github.com/shopspring/decimal"
	"github.com/xyths/go-indicators"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/hs/logger"
	"github.com/xyths/qtr/gateio"
	"go.mongodb.org/mongo-driver/mongo"
	"strings"
	"time"
)

type SuperTrendConfig struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Strategy SuperTrendStrategyConf
	Robots   []hs.BroadcastConf
}

type SuperTrendStrategyConf struct {
	Total    float64
	Interval string
	Factor   float64
	Period   int
}

type SuperTrendTrader struct {
	config   SuperTrendConfig
	interval time.Duration

	db     *mongo.Database
	ex     *gateio.GateIO
	robots []broadcast.Broadcaster

	quoteCurrency string // cash, eg. USDT
	baseSymbol    string

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

func NewSuperTrendTrader(configFilename string) *SuperTrendTrader {
	cfg := SuperTrendConfig{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		logger.Sugar.Fatal(err)
	}
	interval, err := time.ParseDuration(cfg.Strategy.Interval)
	if err != nil {
		logger.Sugar.Fatalf("error interval format: %s", cfg.Strategy.Interval)
	}
	return &SuperTrendTrader{
		config:   cfg,
		interval: interval,
	}
}

func (s *SuperTrendTrader) Init(ctx context.Context) {
	db, err := hs.ConnectMongo(ctx, s.config.Mongo)
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	s.db = db
	s.initEx(ctx)
	s.initRobots(ctx)
}

func (s *SuperTrendTrader) Close(ctx context.Context) {
	if s.db != nil {
		_ = s.db.Client().Disconnect(ctx)
	}
}

func (s *SuperTrendTrader) Print(ctx context.Context) error {
	//s.getPosition(ctx)
	return nil
}

func (s *SuperTrendTrader) Clear(ctx context.Context) error {
	//s.clearState(ctx)
	return nil
}

func (s *SuperTrendTrader) Start(ctx context.Context) error {
	//if !s.loadState(ctx) {
	//	s.getPosition(ctx)
	//	s.saveState(ctx)
	//}

	s.doWork(ctx)
	for {
		select {
		case <-ctx.Done():
			logger.Sugar.Info("SuperTrend trader stopped")
			return nil
		case <-time.After(s.interval):
			s.doWork(ctx)
		}
	}
}

func (s *SuperTrendTrader) initEx(ctx context.Context) {
	s.ex = gateio.New(s.config.Exchange.Key, s.config.Exchange.Secret, s.config.Exchange.Host)
	switch s.config.Exchange.Symbols[0] {
	case "btc3l_usdt":
		s.baseSymbol = gateio.BTC3L_USDT
		//s.baseCurrency = gateio.BTC3L
		s.quoteCurrency = gateio.USDT
	case "btc_usdt":
		s.baseSymbol = gateio.BTC_USDT
		//s.baseCurrency = gateio.BTC
		s.quoteCurrency = gateio.USDT
	case "sero_usdt":
		s.baseSymbol = gateio.SERO_USDT
		//s.baseCurrency = gateio.SERO
		s.quoteCurrency = gateio.USDT
	default:
		s.baseSymbol = "btc_usdt"
	}

	longSymbol := s.config.Exchange.Symbols[0]
	shortSymbol := s.config.Exchange.Symbols[0]
	if len(s.config.Exchange.Symbols) >= 2 {
		longSymbol = s.config.Exchange.Symbols[1]
	}
	if len(s.config.Exchange.Symbols) >= 3 {
		shortSymbol = s.config.Exchange.Symbols[2]
	}
	switch longSymbol {
	case "btc3l_usdt":
		s.longSymbol = gateio.BTC3L_USDT
		s.longCurrency = gateio.BTC3L
	case "btc_usdt":
		s.longSymbol = gateio.BTC_USDT
		s.longCurrency = gateio.BTC
	case "sero_usdt":
		s.longSymbol = gateio.SERO_USDT
		s.longCurrency = gateio.SERO
	default:
		s.longSymbol = gateio.BTC_USDT
		s.longCurrency = gateio.BTC
	}
	switch shortSymbol {
	case "btc3l_usdt":
		s.shortSymbol = gateio.BTC3L_USDT
		s.shortCurrency = gateio.BTC3L
	case "btc3s_usdt":
		s.shortSymbol = gateio.BTC3S_USDT
		s.shortCurrency = gateio.BTC3S
	case "btc_usdt":
		s.shortSymbol = gateio.BTC_USDT
		s.shortCurrency = gateio.BTC
	case "sero_usdt":
		s.shortSymbol = gateio.SERO_USDT
		s.shortCurrency = gateio.SERO
	default:
		s.shortSymbol = gateio.BTC_USDT
		s.shortCurrency = gateio.BTC
	}

	s.longPricePrecision = int32(gateio.PricePrecision[s.longSymbol])
	s.longAmountPrecision = int32(gateio.AmountPrecision[s.longSymbol])
	s.longMinAmount = decimal.NewFromFloat(gateio.MinAmount[s.longSymbol])
	s.longMinTotal = decimal.NewFromInt(gateio.MinTotal[s.longSymbol])
	logger.Sugar.Debugf("init long symbol, pricePrecision = %d, amountPrecision = %d, minAmount = %s, minTotal = %s",
		s.longPricePrecision, s.longAmountPrecision, s.longMinAmount.String(), s.longMinTotal.String())

	s.shortPricePrecision = int32(gateio.PricePrecision[s.shortSymbol])
	s.shortAmountPrecision = int32(gateio.AmountPrecision[s.shortSymbol])
	s.shortMinAmount = decimal.NewFromFloat(gateio.MinAmount[s.shortSymbol])
	s.shortMinTotal = decimal.NewFromInt(gateio.MinTotal[s.shortSymbol])
	logger.Sugar.Debugf("init short symbol, pricePrecision = %d, amountPrecision = %d, minAmount = %s, minTotal = %s",
		s.shortPricePrecision, s.shortAmountPrecision, s.shortMinAmount.String(), s.shortMinTotal.String())

}
func (s *SuperTrendTrader) initRobots(ctx context.Context) {
	for _, conf := range s.config.Robots {
		s.robots = append(s.robots, broadcast.New(conf))
	}
}

func (s *SuperTrendTrader) doWork(ctx context.Context) {
	candle, err := s.ex.GetCandle(s.baseSymbol, int(s.interval)/1000000000, int(300*s.interval/time.Hour)-1)
	if err != nil {
		logger.Sugar.Errorf("get candle error: %s", err)
		return
	}

	_, trend := indicator.SuperTrend(s.config.Strategy.Factor, s.config.Strategy.Period, candle.High, candle.Low, candle.Close)
	l := len(trend)
	if l < 2 {
		return
	}
	if trend[l-1] && !trend[l-2] {
		// false -> true, buy/long
		s.long(ctx)
	} else if !trend[l-1] && trend[l-2] {
		// true -> false, sell/short
		s.short(ctx)
	} else if s.LongTimes == 0 && s.ShortTimes == 0 && trend[l-1] {
		// first time, try buy
		s.long(ctx)
	} else if s.LongTimes == 0 && s.ShortTimes == 0 && !trend[l-1] {
		// first time, try sell
		s.short(ctx)
	}
}

//func (t *SuperTrendTrader) getPosition(ctx context.Context) {
//	balance, err := t.ex.AvailableBalance()
//	if err != nil {
//		logger.Sugar.Errorf("get available balance error: %s", err)
//		return
//	}
//	t.balance = balance
//	logger.Sugar.Debugf("balance: %v", t.balance)
//
//	if t.balance[t.quoteCurrency].LessThan(decimal.NewFromInt(1)) {
//		// full
//		t.state.Position = 3
//	} else if balance[t.baseCurrency].GreaterThan(t.minAmount) {
//		t.state.Position = 1
//	}
//
//	t.state.LastModified = time.Now()
//}

func (s *SuperTrendTrader) buy(ctx context.Context, symbol string, amountPrecision int32, minAmount, minTotal decimal.Decimal) {
	balance, err := s.ex.AvailableBalance()
	if err != nil {
		logger.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	if s.position == 1 {
		logger.Sugar.Infof("position full: %v", balance)
		return
	}
	ob, err := s.ex.OrderBook(symbol)
	if err != nil {
		logger.Sugar.Errorf("get order book error: %s", err)
		return
	}
	// taker
	s1, _ := gateio.Sell1(ob.Asks)
	price := decimal.NewFromFloat(s1)
	// sell all balance
	amount := balance[s.quoteCurrency].Div(price).Round(amountPrecision)
	total := price.Mul(amount)
	for total.GreaterThan(balance[s.quoteCurrency]) {
		amount = amount.Sub(minAmount)
		total = price.Mul(amount)
	}
	logger.Sugar.Debugf("try to buy %s, balance: %v, amount: %s, price: %s, total: %s", symbol, balance, amount, price, total)
	if amount.LessThan(minAmount) {
		logger.Sugar.Infof("amount too small: %s", amount)
		//full
		s.position = 1
		return
	}
	if total.LessThan(minTotal) {
		logger.Sugar.Infof("total too small: %s", total)
		//full
		s.position = 1
		return
	}
	clientId := fmt.Sprintf("c-0-%d", s.LongTimes+1)
	orderId, err := s.ex.Buy(symbol, price, amount, gateio.OrderTypeNormal, clientId)
	if err != nil {
		logger.Sugar.Errorf("buy error: %s", err)
		return
	}
	logger.Sugar.Infof("买入，订单号: %d / %s, price: %s, amount: %s", orderId, clientId, price, amount)
	s.Broadcast(symbol, "buy", price.String(), amount.String(), total.String())

	s.position = 1
	s.LongTimes++
}

func (s *SuperTrendTrader) sell(ctx context.Context, symbol, currency string, amountPrecision int32, minAmount, minTotal decimal.Decimal) {
	balance, err := s.ex.AvailableBalance()
	if err != nil {
		logger.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	if s.position == -1 {
		logger.Sugar.Infof("position clear: %v", balance)
		return
	}
	ob, err := s.ex.OrderBook(symbol)
	if err != nil {
		logger.Sugar.Errorf("get order book error: %s", err)
		return
	}
	// taker
	b1, _ := gateio.Buy1(ob.Bids)
	price := decimal.NewFromFloat(b1)
	// sell all balance
	amount := balance[currency].Round(amountPrecision)
	if amount.GreaterThan(balance[currency]) {
		amount = amount.Sub(minAmount)
	}
	logger.Sugar.Debugf("try to sell %s, balance: %v, amount: %s, price: %s", symbol, balance, amount, price)
	if amount.LessThan(minAmount) {
		logger.Sugar.Infof("amount too small: %s", amount)
		s.position = -1
		return
	}
	total := amount.Mul(price)
	if total.LessThan(minTotal) {
		logger.Sugar.Infof("total too small: %s", total)
		//full
		s.position = 1
		return
	}
	clientId := fmt.Sprintf("c-%d-0", s.ShortTimes+1)
	orderId, err := s.ex.Sell(symbol, price, amount, gateio.OrderTypeNormal, clientId)
	if err != nil {
		logger.Sugar.Errorf("sell error: %s", err)
		return
	}
	logger.Sugar.Infof("清仓，订单号: %d / %s, price: %s, amount: %s", orderId, clientId, price, amount)
	s.Broadcast(symbol, "sell", price.String(), amount.String(), total.String())

	s.position = -1
	s.ShortTimes++
}

func (s *SuperTrendTrader) long(ctx context.Context) {
	// sell short currency
	if s.shortSymbol != s.baseSymbol {
		s.sell(ctx, s.shortSymbol, s.shortCurrency, s.shortAmountPrecision, s.shortMinAmount, s.shortMinTotal)
	}
	// buy long currency
	s.buy(ctx, s.longSymbol, s.longAmountPrecision, s.longMinAmount, s.longMinTotal)
}

func (s *SuperTrendTrader) short(ctx context.Context) {
	// sell long currency
	s.sell(ctx, s.longSymbol, s.longCurrency, s.longAmountPrecision, s.longMinAmount, s.longMinTotal)
	// buy short currency
	if s.shortSymbol != s.baseSymbol {
		s.buy(ctx, s.shortSymbol, s.shortAmountPrecision, s.shortMinAmount, s.shortMinTotal)
	}
}

func (s *SuperTrendTrader) Broadcast(symbol, direction, price, amount, total string) {
	labels := []string{s.config.Exchange.Name, s.config.Exchange.Label}
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	timeStr := time.Now().In(beijing).Format(layout)

	var title string
	switch direction {
	case "buy":
		title = "买入"
	case "sell":
		title = "卖出"
	}
	msg := fmt.Sprintf(`%s [%s]
[%s] [%s]
成交均价 %s, 成交量 %s, 成交额 %s`, timeStr, title, strings.Join(labels, "] ["), symbol, price, amount, total)
	for _, robot := range s.robots {
		if err := robot.SendText(msg); err != nil {
			log.Infof("broadcast error: %s", err)
		}
	}
}
