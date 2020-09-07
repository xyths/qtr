package rest

import (
	"context"
	"fmt"
	"github.com/google/martian/log"
	"github.com/huobirdcenter/huobi_golang/pkg/model/market"
	"github.com/shopspring/decimal"
	"github.com/xyths/go-indicators"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/hs/exchange/gateio"
	"github.com/xyths/hs/exchange/huobi"
	"github.com/xyths/hs/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"strconv"
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
	dry      bool

	db     *mongo.Database
	ex     hs.Exchange
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

	candle hs.Candle

	lastBuyTime  int64
	lastSellTime int64

	orderId    string
	position   int
	LongTimes  int
	ShortTimes int

	//balance   map[string]decimal.Decimal
}

const collNameState = "state"

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
		maxTotal: decimal.NewFromFloat(cfg.Strategy.Total / 2),
	}
}

func (s *SuperTrendTrader) Init(ctx context.Context) {
	db, err := hs.ConnectMongo(ctx, s.config.Mongo)
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	s.db = db
	s.candle = hs.NewCandle(2000)
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

func (s *SuperTrendTrader) Start(ctx context.Context, dry bool) error {
	s.loadState(ctx)

	// setup subscriber
	s.dry = dry
	{
		to := time.Now()
		from := to.Add(-2000 * s.interval)
		candle, err := s.ex.CandleFrom(s.baseSymbol, "super", s.interval, from, to)
		if err != nil {
			logger.Sugar.Fatalf("get candle error: %s", err)
		}
		s.candle = candle
	}
	go s.ex.SubscribeCandlestick(ctx, s.baseSymbol, "super", s.interval, s.tickerHandler)
	// wait for candle
	<-ctx.Done()
	return nil
}

func (s *SuperTrendTrader) initEx(ctx context.Context) {
	switch s.config.Exchange.Name {
	case "gate":
		s.initGate(ctx)
	case "huobi":
		s.initHuobi(ctx)
	default:
		logger.Sugar.Fatalf("unsupported exchange")
	}
	s.initLimits()
}

func (s *SuperTrendTrader) initGate(ctx context.Context) {
	//s.ex = gateio.New(s.config.Exchange.Key, s.config.Exchange.Secret, s.config.Exchange.Host)
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
}

func (s *SuperTrendTrader) initHuobi(ctx context.Context) {
	s.ex = huobi.New(s.config.Exchange.Label, s.config.Exchange.Key, s.config.Exchange.Secret, s.config.Exchange.Host)
	switch s.config.Exchange.Symbols[0] {
	case "btc_usdt":
		s.baseSymbol = huobi.BTC_USDT
		s.quoteCurrency = huobi.USDT
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
	case "btc_usdt":
		s.longSymbol = huobi.BTC_USDT
		s.longCurrency = huobi.BTC
	default:
		s.longSymbol = huobi.BTC_USDT
		s.longCurrency = huobi.BTC
	}
	switch shortSymbol {
	case "btc_usdt":
		s.shortSymbol = huobi.BTC_USDT
		s.shortCurrency = huobi.BTC
	default:
		s.shortSymbol = huobi.BTC_USDT
		s.shortCurrency = huobi.BTC
	}
}

func (s *SuperTrendTrader) initLimits() {
	s.longPricePrecision, s.longAmountPrecision, s.longMinAmount, s.longMinTotal = s.getLimits(s.longSymbol)
	logger.Sugar.Debugf("init long symbol %s, pricePrecision = %d, amountPrecision = %d, minAmount = %s, minTotal = %s",
		s.longSymbol, s.longPricePrecision, s.longAmountPrecision, s.longMinAmount.String(), s.longMinTotal.String())
	s.shortPricePrecision, s.shortAmountPrecision, s.shortMinAmount, s.shortMinTotal = s.getLimits(s.shortSymbol)
	logger.Sugar.Debugf("init short symbol %s, pricePrecision = %d, amountPrecision = %d, minAmount = %s, minTotal = %s",
		s.shortSymbol, s.shortPricePrecision, s.shortAmountPrecision, s.shortMinAmount.String(), s.shortMinTotal.String())
}

func (s SuperTrendTrader) getLimits(symbol string) (pricePrecision, amountPrecision int32, minAmount, minTotal decimal.Decimal) {
	return s.ex.PricePrecision(symbol), s.ex.AmountPrecision(symbol), s.ex.MinAmount(symbol), s.ex.MinTotal(symbol)
}

func (s *SuperTrendTrader) initRobots(ctx context.Context) {
	for _, conf := range s.config.Robots {
		s.robots = append(s.robots, broadcast.New(conf))
	}
}

func (s *SuperTrendTrader) buy(symbol string, amountPrecision int32, minAmount, minTotal decimal.Decimal) {
	balance, err := s.ex.SpotAvailableBalance()
	if err != nil {
		logger.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	if s.position == 1 {
		logger.Sugar.Infof("position full: %v", balance)
		return
	}
	maxTotal := balance[s.quoteCurrency]
	if maxTotal.GreaterThan(s.maxTotal) {
		maxTotal = s.maxTotal
	}
	total := maxTotal
	if total.LessThan(minTotal) {
		logger.Sugar.Infof("total too small: %s", total)
		//full
		s.position = 1
		return
	}
	clientId := fmt.Sprintf("c-0-%d", s.LongTimes+1)
	orderId, err := s.ex.BuyMarket(symbol, clientId, total)
	if err != nil {
		logger.Sugar.Errorf("buy error: %s", err)
		return
	}
	//logger.Sugar.Infof("买入，订单号: %d / %s, price: %s, amount: %s", orderId, clientId, price, amount)
	logger.Sugar.Infof("市价买入，订单号: %d / %s, total: %s", orderId, clientId, total)
	//s.Broadcast(symbol, "buy", price.String(), amount.String(), total.String())

	s.position = 1
	s.LongTimes++
}

func (s *SuperTrendTrader) sell(symbol, currency string, amountPrecision int32, minAmount, minTotal decimal.Decimal) {
	balance, err := s.ex.SpotAvailableBalance()
	if err != nil {
		logger.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	if s.position == -1 {
		logger.Sugar.Infof("position clear: %v", balance)
		return
	}
	// sell all balance
	amount := balance[currency].Round(amountPrecision)
	if amount.GreaterThan(balance[currency]) {
		amount = amount.Sub(minAmount)
	}
	//logger.Sugar.Debugf("try to sell %s, balance: %v, amount: %s, price: %s", symbol, balance, amount, price)
	if amount.LessThan(minAmount) {
		logger.Sugar.Infof("amount too small: %s", amount)
		s.position = -1
		return
	}
	clientId := fmt.Sprintf("c-%d-0", s.ShortTimes+1)
	orderId, err := s.ex.SellMarket(symbol, clientId, amount)
	if err != nil {
		logger.Sugar.Errorf("sell error: %s", err)
		return
	}
	logger.Sugar.Infof("市价清仓，订单号: %d / %s, amount: %s", orderId, clientId, amount)
	//s.Broadcast(symbol, "sell", price.String(), amount.String(), total.String())

	s.position = -1
	s.ShortTimes++
}

func (s *SuperTrendTrader) long() {
	// sell short currency
	if s.shortSymbol != s.baseSymbol {
		s.sell(s.shortSymbol, s.shortCurrency, s.shortAmountPrecision, s.shortMinAmount, s.shortMinTotal)
	}
	// buy long currency
	s.buy(s.longSymbol, s.longAmountPrecision, s.longMinAmount, s.longMinTotal)
}

func (s *SuperTrendTrader) short() {
	// sell long currency
	s.sell(s.longSymbol, s.longCurrency, s.longAmountPrecision, s.longMinAmount, s.longMinTotal)
	// buy short currency
	if s.shortSymbol != s.baseSymbol {
		s.buy(s.shortSymbol, s.shortAmountPrecision, s.shortMinAmount, s.shortMinTotal)
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

func (s *SuperTrendTrader) tickerHandler(resp interface{}) {
	candlestickResponse, ok := resp.(market.SubscribeCandlestickResponse)
	if ok {
		if &candlestickResponse != nil {
			if candlestickResponse.Tick != nil {
				tick := candlestickResponse.Tick
				logger.Sugar.Infof("Tick, id: %d, count: %v, amount: %v, volume: %v, OHLC[%v, %v, %v, %v]",
					tick.Id, tick.Count, tick.Amount, tick.Vol, tick.Open, tick.High, tick.Low, tick.Close)
				ticker := hs.Ticker{
					Timestamp: tick.Id,
				}
				ticker.Open, _ = tick.Open.Float64()
				ticker.High, _ = tick.High.Float64()
				ticker.Low, _ = tick.Low.Float64()
				ticker.Close, _ = tick.Close.Float64()
				ticker.Volume, _ = tick.Vol.Float64()
				if s.candle.Length() > 0 {
					s.candle.Append(ticker)
					s.onTick(s.dry)
				} else {
					logger.Sugar.Info("candle is not ready for append")
				}
			}

			if candlestickResponse.Data != nil {
				logger.Sugar.Infof("Candlestick(candle) update, last timestamp: %d", candlestickResponse.Data[len(candlestickResponse.Data)-1].Id)
				for _, tick := range candlestickResponse.Data {
					//logger.Sugar.Infof("Candlestick data[%d], id: %d, count: %v, volume: %v, OHLC[%v, %v, %v, %v]",
					//	i, tick.Id, tick.Count, tick.Vol, tick.Open, tick.High, tick.Low, tick.Close)
					ticker := hs.Ticker{
						Timestamp: tick.Id,
					}
					ticker.Open, _ = tick.Open.Float64()
					ticker.High, _ = tick.High.Float64()
					ticker.Low, _ = tick.Low.Float64()
					ticker.Close, _ = tick.Close.Float64()
					ticker.Volume, _ = tick.Vol.Float64()
					s.candle.Append(ticker)
				}
				s.onTick(s.dry)
			}
		}
	} else {
		logger.Sugar.Warn("Unknown response: %v", resp)
	}
}

func (s *SuperTrendTrader) onTick(dry bool) {
	tsl, trend := indicator.SuperTrend(s.config.Strategy.Factor, s.config.Strategy.Period, s.candle.High, s.candle.Low, s.candle.Close)
	for i := s.candle.Length() - 2; i < s.candle.Length(); i++ {
		logger.Sugar.Debugf("[%d] %s %f %f %f %f, %f, %v",
			i, timestampToDate(s.candle.Timestamp[i]), s.candle.Open[i], s.candle.High[i], s.candle.Low[i], s.candle.Close[i], tsl[i], trend[i])
	}
	l := len(trend)
	t := s.candle.Timestamp[l-1]
	if l < 2 {
		return
	}
	logger.Sugar.Debugf("SuperTrend = [..., (%f, %v), (%f, %v)", tsl[l-2], trend[l-2], tsl[l-1], trend[l-1])

	if trend[l-1] && !trend[l-2] && s.lastBuyTime != t {
		// false -> true, buy/long
		logger.Sugar.Info("[Signal] BUY")
		s.saveLastBuyTime(context.Background())
		if !dry {
			go s.long()
		}
		s.lastBuyTime = t
	} else if !trend[l-1] && trend[l-2] && s.lastSellTime != t {
		// true -> false, sell/short
		logger.Sugar.Info("[Signal] SELL")
		s.saveLastSellTime(context.Background())
		if !dry {
			go s.short()
		}
		s.lastSellTime = t
	}
}

func (s *SuperTrendTrader) saveLastBuyTime(ctx context.Context) {
	if err := s.saveInt64(ctx, "lastBuyTime", s.lastBuyTime); err != nil {
		logger.Sugar.Errorf("save lastBuyTime error: %s", err)
	}
}
func (s *SuperTrendTrader) saveLastSellTime(ctx context.Context) {
	if err := s.saveInt64(ctx, "lastSellTime", s.lastSellTime); err != nil {
		logger.Sugar.Errorf("save lastSellTime error: %s", err)
	}
}
func (s *SuperTrendTrader) saveInt64(ctx context.Context, key string, value int64) error {
	coll := s.db.Collection(collNameState)
	option := &options.UpdateOptions{}
	option.SetUpsert(true)

	_, err := coll.UpdateOne(context.Background(),
		bson.D{
			{"key", key},
		},
		bson.D{
			{"$set", bson.D{
				{"value", value},
			}},
			{"$currentDate", bson.D{
				{"lastModified", true},
			}},
		},
		option,
	)
	return err
}
func (s *SuperTrendTrader) loadLastBuyTime(ctx context.Context) (int64, error) {
	return s.loadInt64(ctx, "lastBuyTime")
}
func (s *SuperTrendTrader) loadLastSellTime(ctx context.Context) (int64, error) {
	return s.loadInt64(ctx, "lastSellTime")
}
func (s *SuperTrendTrader) loadInt64(ctx context.Context, key string) (int64, error) {
	coll := s.db.Collection(collNameState)
	var state = struct {
		Key   string
		Value string
	}{}
	if err := coll.FindOne(ctx, bson.D{}).Decode(&state); err == mongo.ErrNoDocuments {
		//logger.Sugar.Errorf("load Position error: %s",err)
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	return strconv.ParseInt(state.Value, 10, 64)
}

func (s *SuperTrendTrader) loadState(ctx context.Context) {
	if buy, err := s.loadLastBuyTime(ctx); err != nil {
		logger.Sugar.Fatalf("loadBuyTime error: %s", err)
	} else if buy != 0 {
		s.lastBuyTime = buy
		logger.Sugar.Infof("loaded loadBuyTime: %s", timestampToDate(buy))
	}
	if sell, err := s.loadLastSellTime(ctx); err != nil {
		logger.Sugar.Fatalf("loadSellTime error: %s", err)
	} else if sell != 0 {
		s.lastSellTime = sell
		logger.Sugar.Infof("loaded lastSellTime: %s", timestampToDate(sell))
	}
}

func timestampToDate(timestamp int64) string {
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	return time.Unix(timestamp, 0).In(beijing).Format(layout)
}
