package rest

import (
	"context"
	"fmt"
	"github.com/huobirdcenter/huobi_golang/pkg/model/market"
	"github.com/huobirdcenter/huobi_golang/pkg/model/order"
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
	"go.uber.org/zap"
	"log"
	"strings"
	"time"
)

type SuperTrendConfig struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Strategy SuperTrendStrategyConf
	Log      hs.LogConf
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

	Sugar  *zap.SugaredLogger
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

	candle        hs.Candle
	uniqueId      int64
	sellStopOrder SellStopOrder
	position      int64
	LongTimes     int64
	ShortTimes    int64
}

const (
	collNameState = "state"
	collNameOrder = "order"
)

var emptySellStopOrder = SellStopOrder{Name: "sellStopOrder"}

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
		config:        cfg,
		interval:      interval,
		sellStopOrder: emptySellStopOrder,
		maxTotal:      decimal.NewFromFloat(cfg.Strategy.Total / 2),
	}
}

func (s *SuperTrendTrader) Init(ctx context.Context) {
	s.initLogger()
	db, err := hs.ConnectMongo(ctx, s.config.Mongo)
	if err != nil {
		s.Sugar.Fatal(err)
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
	s.Sugar.Sync()
}

func (s *SuperTrendTrader) Print(ctx context.Context) error {
	s.loadState(ctx)
	log.Printf("Position: %d", s.position)
	log.Printf("Unique Id: %d", s.uniqueId)
	log.Printf("Long times: %d", s.LongTimes)
	log.Printf("Short times: %d", s.ShortTimes)
	log.Printf(`Sell-stop order
	Id: %d / %s
	Price: %s
	Amount: %s
	Create Time: %s`,
		s.sellStopOrder.Id, s.sellStopOrder.ClientId,
		s.sellStopOrder.Price,
		s.sellStopOrder.Amount,
		s.sellStopOrder.Time,
	)

	return nil
}

func (s *SuperTrendTrader) Clear(ctx context.Context) error {
	s.clearState(ctx)
	return nil
}

func (s *SuperTrendTrader) Start(ctx context.Context, dry bool) error {
	s.loadState(ctx)
	s.checkState(ctx)
	s.dry = dry

	// setup subscriber
	go s.ex.SubscribeOrder(ctx, s.baseSymbol, "super-order", s.OrderUpdateHandler)

	{
		to := time.Now()
		from := to.Add(-2000 * s.interval)
		candle, err := s.ex.CandleFrom(s.baseSymbol, "super-candle", s.interval, from, to)
		if err != nil {
			s.Sugar.Fatalf("get candle error: %s", err)
		}
		s.candle.Add(candle)
	}
	go s.ex.SubscribeCandlestick(ctx, s.baseSymbol, "super-tick", s.interval, s.tickerHandler)
	// wait for candle
	<-ctx.Done()
	return nil
}

func (s *SuperTrendTrader) initLogger() {
	logger, err := hs.NewZapLogger(s.config.Log.Level, s.config.Log.Outputs, s.config.Log.Errors)
	if err != nil {
		log.Fatalf("setup log error: %s", err)
	}
	s.Sugar = logger.Sugar()
}

func (s *SuperTrendTrader) initEx(ctx context.Context) {
	switch s.config.Exchange.Name {
	case "gate":
		s.initGate(ctx)
	case "huobi":
		s.initHuobi(ctx)
	default:
		s.Sugar.Fatalf("unsupported exchange")
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
	s.Sugar.Debugf("init long symbol %s, pricePrecision = %d, amountPrecision = %d, minAmount = %s, minTotal = %s",
		s.longSymbol, s.longPricePrecision, s.longAmountPrecision, s.longMinAmount.String(), s.longMinTotal.String())
	s.shortPricePrecision, s.shortAmountPrecision, s.shortMinAmount, s.shortMinTotal = s.getLimits(s.shortSymbol)
	s.Sugar.Debugf("init short symbol %s, pricePrecision = %d, amountPrecision = %d, minAmount = %s, minTotal = %s",
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
		s.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	s.Sugar.Infof("balance: %v", balance)
	if s.position == 1 {
		s.Sugar.Infof("position full: %v", balance)
		return
	}
	maxTotal := balance[s.quoteCurrency]
	if maxTotal.GreaterThan(s.maxTotal) {
		maxTotal = s.maxTotal
	}
	total := maxTotal
	if total.LessThan(minTotal) {
		s.Sugar.Infof("total too small: %s", total)
		//full
		s.position = 1
		if err := s.saveInt64(context.Background(), "position", s.position); err != nil {
			s.Sugar.Infof("save position error: %s", err)
		}
		return
	}
	clientId := fmt.Sprintf("c-%d-%d-%d", s.ShortTimes, s.LongTimes+1, s.GetUniqueId())
	orderId, err := s.ex.BuyMarket(symbol, clientId, total)
	if err != nil {
		s.Sugar.Errorf("buy error: %s", err)
		return
	}
	o := Order{Id: int64(orderId), ClientOrderId: clientId}
	s.addOrder(context.Background(), o)
	s.Sugar.Infof("市价买入，订单号: %d / %s, total: %s", orderId, clientId, total)
	s.Broadcast("市价买入，订单号: %d / %s, 买入金额: %s", orderId, clientId, total)

	s.position = 1
	if err := s.saveInt64(context.Background(), "position", s.position); err != nil {
		s.Sugar.Infof("save position error: %s", err)
	}
	s.LongTimes++
	if err := s.saveInt64(context.Background(), "longTimes", s.LongTimes); err != nil {
		s.Sugar.Infof("save longTimes error: %s", err)
	}
}

func (s *SuperTrendTrader) sell(symbol, currency string, amountPrecision int32, minAmount, minTotal decimal.Decimal) {
	balance, err := s.ex.SpotAvailableBalance()
	if err != nil {
		s.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	s.Sugar.Infof("balance: %v", balance)
	if s.position == -1 {
		s.Sugar.Infof("position clear: %v", balance)
		return
	}
	// sell all balance
	amount := balance[currency].Round(amountPrecision)
	if amount.GreaterThan(balance[currency]) {
		amount = amount.Sub(minAmount)
	}
	//logger.Sugar.Debugf("try to sell %s, balance: %v, amount: %s, price: %s", symbol, balance, amount, price)
	if amount.LessThan(minAmount) {
		s.Sugar.Infof("amount too small: %s", amount)
		s.position = -1
		if err := s.saveInt64(context.Background(), "position", s.position); err != nil {
			s.Sugar.Infof("save position error: %s", err)
		}
		return
	}
	clientId := fmt.Sprintf("s-%d-%d-%d", s.ShortTimes+1, s.LongTimes, s.GetUniqueId())
	orderId, err := s.ex.SellMarket(symbol, clientId, amount)
	if err != nil {
		s.Sugar.Errorf("sell error: %s", err)
		return
	}
	o := Order{Id: int64(orderId), ClientOrderId: clientId}
	s.addOrder(context.Background(), o)
	s.Sugar.Infof("市价清仓，订单号: %d / %s, amount: %s", orderId, clientId, amount)
	s.Broadcast("市价清仓，订单号: %d / %s, 卖出数量: %s", orderId, clientId, amount)

	s.position = -1
	if err := s.saveInt64(context.Background(), "position", s.position); err != nil {
		s.Sugar.Infof("save position error: %s", err)
	}
	s.ShortTimes++
	if err := s.saveInt64(context.Background(), "shortTimes", s.ShortTimes); err != nil {
		s.Sugar.Infof("save shortTimes error: %s", err)
	}
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

func (s *SuperTrendTrader) sellStopLimit(symbol, currency string, price decimal.Decimal,
	amountPrecision int32, minAmount, minTotal decimal.Decimal) (*SellStopOrder, error) {
	if s.position == -1 {
		s.Sugar.Info("position clear, no need sell-stop")
		return nil, nil
	}
	balance, err := s.ex.SpotAvailableBalance()
	if err != nil {
		s.Sugar.Errorf("get available balance error: %s", err)
		return nil, err
	}
	s.Sugar.Infof("balance: %v", balance)
	// sell all balance
	amount := balance[currency].Round(amountPrecision)
	if amount.GreaterThan(balance[currency]) {
		amount = amount.Sub(minAmount)
	}
	s.Sugar.Debugf("try to sell %s, balance: %v, amount: %s, price: %s, total: %s", symbol, balance, amount, price)
	if amount.LessThan(minAmount) {
		s.Sugar.Infof("amount too small: %s (need %s)", amount, minAmount)
		s.position = -1
		if err := s.saveInt64(context.Background(), "position", s.position); err != nil {
			s.Sugar.Infof("save position error: %s", err)
		}
		s.Sugar.Info("update position to `clear`")
		return nil, nil
	}
	total := price.Mul(amount)
	if total.LessThan(minTotal) {
		s.Sugar.Infof("total too small: %s (need %s)", total, minTotal)
		s.position = -1
		if err := s.saveInt64(context.Background(), "position", s.position); err != nil {
			s.Sugar.Infof("save position error: %s", err)
		}
		s.Sugar.Info("update position to `clear`")
		return nil, nil
	}
	clientId := fmt.Sprintf("stl-%d-%d-%d", s.ShortTimes, s.LongTimes, s.GetUniqueId())
	orderId, err := s.ex.SellStopLimit(symbol, clientId, price, amount, price)
	if err != nil {
		s.Sugar.Errorf("sell-stop error: %s", err)
		return nil, err
	}
	o := Order{
		Id:            int64(orderId),
		ClientOrderId: clientId,
		Type:          "sell-stop-limit",
	}
	s.addOrder(context.Background(), o)
	s.Sugar.Infof("止损单，订单号: %d / %s, amount: %s", orderId, clientId, amount)
	sso := &SellStopOrder{
		Name:      emptySellStopOrder.Name,
		Id:        int64(orderId),
		ClientId:  clientId,
		Price:     price.String(),
		StopPrice: price.String(),
		Amount:    amount.String(),
		Total:     total.String(),
		Time:      time.Now().String(),
		Status:    "created",
	}
	return sso, nil
}

func (s *SuperTrendTrader) updateSellStop(price decimal.Decimal) {
	if s.sellStopOrder.Price != "" {
		oldStopPrice := decimal.RequireFromString(s.sellStopOrder.Price)
		if price.Equal(oldStopPrice) {
			s.Sugar.Debug("same stop price, no need to update")
			return
		}
	}
	s.Sugar.Debugf("try to place sell-stop-limit order, price %s", price)
	// clear old order
	if s.sellStopOrder.Id != 0 {
		if err := s.ex.CancelOrder(s.longSymbol, uint64(s.sellStopOrder.Id)); err != nil {
			s.Sugar.Errorf("cancel sell-stop-limit order error: %s", err)
		} else {
			s.Sugar.Infof("cancelled sell-stop-limit order %d", s.sellStopOrder.Id)
			s.sellStopOrder = emptySellStopOrder
			if err := s.saveKey(context.Background(), s.sellStopOrder.Name, s.sellStopOrder); err != nil {
				s.Sugar.Errorf("save sellStopOrder error: %s", err)
			}
		}
	}

	// place new order
	if sso, err := s.sellStopLimit(s.longSymbol, s.longCurrency, price,
		s.longAmountPrecision, s.longMinAmount, s.longMinTotal); err != nil {
		s.Sugar.Errorf("sell-stop error: %s", err)
	} else if sso != nil {
		s.sellStopOrder = *sso
		if err := s.saveKey(context.Background(), s.sellStopOrder.Name, s.sellStopOrder); err != nil {
			s.Sugar.Errorf("save sellStopOrder error: %s", err)
		}
	}
}

func (s *SuperTrendTrader) Broadcast(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	labels := []string{s.config.Exchange.Name, s.config.Exchange.Label}
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	timeStr := time.Now().In(beijing).Format(layout)

	msg := fmt.Sprintf("%s [%s] %s", timeStr, strings.Join(labels, "] ["), message)
	for _, robot := range s.robots {
		if err := robot.SendText(msg); err != nil {
			s.Sugar.Infof("broadcast error: %s", err)
		}
	}
}

func (s *SuperTrendTrader) tickerHandler(resp interface{}) {
	candlestickResponse, ok := resp.(market.SubscribeCandlestickResponse)
	if ok {
		if &candlestickResponse != nil {
			if candlestickResponse.Tick != nil {
				tick := candlestickResponse.Tick
				//s.Sugar.Debugf("Tick, id: %d, count: %v, amount: %v, volume: %v, OHLC[%v, %v, %v, %v]",
				//	tick.Id, tick.Count, tick.Amount, tick.Vol, tick.Open, tick.High, tick.Low, tick.Close)
				ticker := hs.Ticker{
					Timestamp: tick.Id,
				}
				ticker.Open, _ = tick.Open.Float64()
				ticker.High, _ = tick.High.Float64()
				ticker.Low, _ = tick.Low.Float64()
				ticker.Close, _ = tick.Close.Float64()
				ticker.Volume, _ = tick.Vol.Float64()
				if s.candle.Length() > 0 {
					oldTime := s.candle.Timestamp[s.candle.Length()-1]
					newTime := ticker.Timestamp
					s.candle.Append(ticker)
					if oldTime != newTime {
						s.onTick(s.dry)
					}
				} else {
					s.Sugar.Info("candle is not ready for append")
				}
			}

			if candlestickResponse.Data != nil {
				//s.Sugar.Debugf("Candlestick(candle) update, last timestamp: %d", candlestickResponse.Data[len(candlestickResponse.Data)-1].Id)
				for _, tick := range candlestickResponse.Data {
					//s.Sugar.Infof("Candlestick data[%d], id: %d, count: %v, volume: %v, OHLC[%v, %v, %v, %v]",
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
				s.onCandle(s.dry)
			}
		}
	} else {
		s.Sugar.Warn("Unknown response: %v", resp)
	}
}

func (s *SuperTrendTrader) onCandle(dry bool) {
	// do nothing for now
}

func (s *SuperTrendTrader) OrderUpdateHandler(response interface{}) {
	subOrderResponse, ok := response.(order.SubscribeOrderV2Response)
	if !ok {
		s.Sugar.Warnf("Received unknown response: %v", response)
	}
	//log.Printf("subOrderResponse = %#v", subOrderResponse)
	if subOrderResponse.Action == "sub" {
		if subOrderResponse.IsSuccess() {
			s.Sugar.Infof("Subscribe topic %s successfully", subOrderResponse.Ch)
		} else {
			s.Sugar.Fatalf("Subscribe topic %s error, code: %d, message: %s",
				subOrderResponse.Ch, subOrderResponse.Code, subOrderResponse.Message)
		}
	} else if subOrderResponse.Action == "push" {
		if subOrderResponse.Data == nil {
			s.Sugar.Infof("SubscribeOrderV2Response has no data: %#v", subOrderResponse)
			return
		}
		o := subOrderResponse.Data
		//s.Sugar.Debugf("Order update, event: %s, symbol: %s, type: %s, id: %d, clientId: %s, status: %s",
		//	o.EventType, o.Symbol, o.Type, o.OrderId, o.ClientOrderId, o.OrderStatus)
		switch o.EventType {
		case "creation":
			s.Sugar.Debugf("order created, orderId: %d, clientOrderId: %s", o.OrderId, o.ClientOrderId)
			order2 := Order{
				Id:            o.OrderId,
				ClientOrderId: o.ClientOrderId,
				Type:          o.Type,
				Price:         o.OrderPrice,
				Status:        o.OrderStatus,
				Updated:       time.Now(),
			}
			switch o.Type {
			case "buy-market":
				// TODO: Total = o.OrderValue
				//order2.Total = o.order
			case "sell-market":
				order2.Amount = o.OrderSize
			case "buy-limit":
				order2.Amount = o.OrderSize
			case "sell-limit":
				order2.Amount = o.OrderSize
			default:
				s.Sugar.Infof("unprocessed order type: %s", o.Type)
			}
			go s.createOrder(context.Background(), order2)
		case "cancellation":
			s.Sugar.Debugf("order cancelled, orderId: %d, clientOrderId: %s", o.OrderId, o.ClientOrderId)
			o2 := Order{
				Id:      o.OrderId,
				Status:  o.OrderStatus,
				Updated: time.Now(),
			}
			go s.cancelOrder(context.Background(), o2)
		case "trade":
			s.Sugar.Debugf("order filled, orderId: %d, clientOrderId: %s, fill type: %s",
				o.OrderId, o.ClientOrderId, o.OrderStatus)
			o2 := Order{
				Id:            o.OrderId,
				ClientOrderId: o.ClientOrderId,
				Status:        o.OrderStatus,
				Updated:       time.Now(),
			}
			t := Trade{
				Id:     o.TradeId,
				Price:  o.TradePrice,
				Amount: o.TradeVolume,
				Remain: o.RemainAmt,
			}
			go s.fillOrder(context.Background(), o2, t)
		case "deletion":
			s.Sugar.Debugf("order deleted, orderId: %d, clientOrderId: %s, fill type: %s",
				o.OrderId, o.ClientOrderId, o.OrderStatus)
			o2 := Order{
				Id:      o.OrderId,
				Status:  o.OrderStatus,
				Updated: time.Now(),
			}
			go s.deleteOrder(context.Background(), o2)
		default:
			s.Sugar.Warnf("unknown eventType, should never happen, orderId: %d, clientOrderId: %s, eventType: %s",
				o.OrderId, o.ClientOrderId, o.EventType)
		}
	}
}

func (s *SuperTrendTrader) onTick(dry bool) {
	tsl, trend := indicator.SuperTrend(s.config.Strategy.Factor, s.config.Strategy.Period, s.candle.High, s.candle.Low, s.candle.Close)
	l := s.candle.Length()
	if l < 3 {
		return
	}
	for i := l - 3; i < l-1; i++ {
		s.Sugar.Debugf("[%d] %s %f %f %f %f, %f %v",
			i, timestampToDate(s.candle.Timestamp[i]), s.candle.Open[i], s.candle.High[i], s.candle.Low[i], s.candle.Close[i], tsl[i], trend[i])
	}
	s.Sugar.Debugf("SuperTrend = [..., (%f, %v), (%f, %v)", tsl[l-3], trend[l-3], tsl[l-2], trend[l-2])

	if trend[l-2] && !trend[l-3] {
		// false -> true, buy/long
		s.Sugar.Info("[Signal] BUY")
		if !dry {
			go s.long()
		}
	} else if !trend[l-2] && trend[l-3] {
		// true -> false, sell/short
		s.Sugar.Info("[Signal] SELL")
		if !dry {
			go s.short()
		}
	} else if s.position == 1 {
		// update sell-stop-limit order
		stop := decimal.NewFromFloat(tsl[l-2]).Round(s.longPricePrecision)
		go s.updateSellStop(stop)
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
func (s *SuperTrendTrader) loadInt64(ctx context.Context, key string) (int64, error) {
	coll := s.db.Collection(collNameState)
	var state = struct {
		Key          string
		Value        int64
		LastModified time.Time
	}{}
	if err := coll.FindOne(ctx, bson.D{
		{"key", key},
	}).Decode(&state); err == mongo.ErrNoDocuments {
		//s.Sugar.Errorf("load Position error: %s",err)
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	s.Sugar.Infof("load %s: %d, lastModified: %s", key, state.Value, state.LastModified.String())
	return state.Value, nil
}
func (s *SuperTrendTrader) deleteInt64(ctx context.Context, key string) error {
	coll := s.db.Collection(collNameState)
	_, err := coll.DeleteOne(ctx, bson.D{
		{"key", key},
	})
	return err
}
func (s *SuperTrendTrader) deleteKey(ctx context.Context, key string) error {
	coll := s.db.Collection(collNameState)
	_, err := coll.DeleteOne(ctx, bson.D{
		{"name", key},
	})
	return err
}
func (s *SuperTrendTrader) saveKey(ctx context.Context, key string, value interface{}) error {
	coll := s.db.Collection(collNameState)
	option := &options.UpdateOptions{}
	option.SetUpsert(true)

	_, err := coll.UpdateOne(context.Background(),
		bson.D{
			{"name", key},
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
func (s *SuperTrendTrader) loadKey(ctx context.Context, key string, value interface{}) error {
	coll := s.db.Collection(collNameState)
	if err := coll.FindOne(ctx, bson.D{
		{"name", key},
	}).Decode(value); err == mongo.ErrNoDocuments {
		return nil
	} else if err != nil {
		return err
	}
	s.Sugar.Debugf("load %s: %v", key, value)
	return nil
}

func (s *SuperTrendTrader) loadState(ctx context.Context) {
	if uniqueId, err := s.loadInt64(ctx, "uniqueId"); err != nil {
		s.Sugar.Fatalf("load UniqueId error: %s", err)
	} else if uniqueId != 0 {
		s.uniqueId = uniqueId
		s.Sugar.Infof("loaded UniqueId: %d", uniqueId)
	}
	if position, err := s.loadInt64(ctx, "position"); err != nil {
		s.Sugar.Fatalf("load position error: %s", err)
	} else if position != 0 {
		s.position = position
		s.Sugar.Infof("loaded position: %d", position)
	}
	if longTimes, err := s.loadInt64(ctx, "longTimes"); err != nil {
		s.Sugar.Fatalf("load longTimes error: %s", err)
	} else if longTimes != 0 {
		s.LongTimes = longTimes
		s.Sugar.Infof("loaded longTimes: %d", longTimes)
	}
	if shortTimes, err := s.loadInt64(ctx, "shortTimes"); err != nil {
		s.Sugar.Fatalf("load shortTimes error: %s", err)
	} else if shortTimes != 0 {
		s.ShortTimes = shortTimes
		s.Sugar.Infof("loaded shortTimes: %d", shortTimes)
	}
	sellStopOrder := SellStopOrder{}
	if err := s.loadKey(ctx, "sellStopOrder", &sellStopOrder); err != nil {
		s.Sugar.Fatalf("load sellStopOrder error: %s", err)
	} else if sellStopOrder.Id != 0 {
		s.sellStopOrder = sellStopOrder
		s.Sugar.Infof("loaded sellStopOrderId: %d", sellStopOrder)
	}
}

func (s *SuperTrendTrader) GetUniqueId() int64 {
	s.uniqueId = (s.uniqueId + 1) % 10000

	if err := s.saveInt64(context.Background(), "uniqueId", s.uniqueId); err != nil {
		s.Sugar.Errorf("save uniqueId error: %s", err)
	}
	return s.uniqueId
}

func (s *SuperTrendTrader) clearState(ctx context.Context) {
	if err := s.deleteInt64(ctx, "longTimes"); err != nil {
		s.Sugar.Errorf("delete longTimes error: %s", err)
	} else {
		s.Sugar.Info("delete longTimes from database")
	}
	if err := s.deleteInt64(ctx, "shortTimes"); err != nil {
		s.Sugar.Errorf("delete shortTimes error: %s", err)
	} else {
		s.Sugar.Info("delete shortTimes from database")
	}
	if err := s.deleteInt64(ctx, "sellStopOrder"); err != nil {
		s.Sugar.Errorf("delete sellStopOrder error: %s", err)
	} else {
		s.Sugar.Info("delete sellStopOrder from database")
	}
}

func (s *SuperTrendTrader) checkState(ctx context.Context) {
	// check sell-stop order
	s.checkSellStopOrder(ctx)
}

func (s *SuperTrendTrader) addOrder(ctx context.Context, o Order) {
	coll := s.db.Collection(collNameOrder)
	if _, err := coll.InsertOne(
		ctx,
		bson.D{
			{"_id", o.Id},
			{"clientOrderId", o.ClientOrderId},
			{"updated", o.Updated},
		},
	); err != nil {
		s.Sugar.Errorf("add order error: %s", err)
	}
}
func (s *SuperTrendTrader) createOrder(ctx context.Context, o Order) {
	coll := s.db.Collection(collNameOrder)
	option := &options.FindOneAndUpdateOptions{}
	option.SetUpsert(true)
	if r := coll.FindOneAndUpdate(
		ctx,
		bson.D{
			{"_id", o.Id},
		},
		bson.D{
			{"$set", bson.D{
				{"type", o.Type},
				{"price", o.Price},
				{"amount", o.Amount},
				{"total", o.Total},
				{"status", o.Status},
				{"updated", o.Updated},
			}},
		},
		option,
	); r.Err() != nil {
		s.Sugar.Errorf("create order error: %s", r.Err())
	}
}
func (s *SuperTrendTrader) fillOrder(ctx context.Context, o Order, t Trade) {
	s.Broadcast("订单成交(%s), 订单号: %d / %s, 价格: %s, 数量: %s, 交易额: %s",
		o.Status, o.Id, o.ClientOrderId, t.Price, t.Amount, t.Total)
	if o.Status == "filled" && strings.HasPrefix(o.ClientOrderId, "stl-") {
		s.position = -1
		if err := s.saveInt64(context.Background(), "position", s.position); err != nil {
			s.Sugar.Infof("save position error: %s", err)
		}
		s.Sugar.Infof("sell-stop order %d is filled, change position to -1 (clear)", o.Id)
	}
	coll := s.db.Collection(collNameOrder)
	option := &options.FindOneAndUpdateOptions{}
	option.SetUpsert(true)
	if r := coll.FindOneAndUpdate(
		ctx,
		bson.D{
			{"_id", o.Id},
		},
		bson.D{
			{"$push", bson.D{
				{"trades", t},
			}},
			{"$set", bson.D{
				{"status", o.Status},
				{"updated", time.Now()},
			}},
		},
		option,
	); r.Err() != nil {
		s.Sugar.Errorf("fill order error: %s", r.Err())
	}
}
func (s *SuperTrendTrader) cancelOrder(ctx context.Context, o Order) {
	s.updateOrderStatus(ctx, o)
}
func (s *SuperTrendTrader) deleteOrder(ctx context.Context, o Order) {
	s.updateOrderStatus(ctx, o)
}
func (s *SuperTrendTrader) updateOrderStatus(ctx context.Context, o Order) {
	coll := s.db.Collection(collNameOrder)
	option := &options.FindOneAndUpdateOptions{}
	option.SetUpsert(true)
	if r := coll.FindOneAndUpdate(
		ctx,
		bson.D{
			{"_id", o.Id},
		},
		bson.D{
			{"$set", bson.D{
				{"status", o.Status},
				{"updated", o.Updated},
			}},
		},
		option,
	); r.Err() != nil {
		s.Sugar.Errorf("create order error: %s", r.Err())
	}
}

func (s *SuperTrendTrader) checkSellStopOrder(ctx context.Context) {
	if s.sellStopOrder.Id == 0 {
		s.Sugar.Info("no sell-stop order")
		return
	}
	o, err := s.ex.GetOrderById(uint64(s.sellStopOrder.Id), s.longSymbol)
	if err != nil {
		s.Sugar.Infof("get sell-stop order error: %s", err)
		return
	}
	if o.Status == "filled" || o.Status == "cancelled" {
		s.sellStopOrder = emptySellStopOrder
		if err := s.saveKey(context.Background(), s.sellStopOrder.Name, s.sellStopOrder); err != nil {
			s.Sugar.Errorf("save sellStopOrderId error: %s", err)
		}
	} else {
		s.Sugar.Infof("sell-stop order id: %s/%s, price: %s, amount: %s, total: %s, status: %s, filled: %s",
			o.Id, o.ClientOrderId, o.InitialPrice, o.InitialAmount, o.InitialPrice.Mul(o.InitialAmount), o.Status, o.FilledAmount)
	}
}

func timestampToDate(timestamp int64) string {
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	return time.Unix(timestamp, 0).In(beijing).Format(layout)
}
