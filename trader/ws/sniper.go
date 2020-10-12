package ws

import (
	"context"
	"errors"
	"fmt"
	"github.com/huobirdcenter/huobi_golang/pkg/model/market"
	"github.com/huobirdcenter/huobi_golang/pkg/model/order"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/hs/exchange/huobi"
	"github.com/xyths/hs/logger"
	"github.com/xyths/qtr/trader/rest"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"log"
	"strings"
	"time"
)

type SniperConfig struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Strategy SniperStrategyConf
	Log      hs.LogConf
	Robots   []hs.BroadcastConf
}

type SniperStrategyConf struct {
	Total    float64
	Interval string
}

type SniperTrader struct {
	config   SniperConfig
	interval time.Duration
	maxTotal decimal.Decimal
	dry      bool

	Sugar  *zap.SugaredLogger
	db     *mongo.Database
	ex     exchange.Exchange
	symbol exchange.Symbol
	fee    exchange.Fee
	robots []broadcast.Broadcaster

	candle     hs.Candle
	uniqueId   int64
	position   int64 // 1 long (full), -1 short (clear)
	LongTimes  int64
	ShortTimes int64
}

func NewSniperTrader(ctx context.Context, configFilename string) (*SniperTrader, error) {
	cfg := SniperConfig{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		return nil, err
	}
	interval, err := time.ParseDuration(cfg.Strategy.Interval)
	if err != nil {
		logger.Sugar.Fatalf("error interval format: %s", cfg.Strategy.Interval)
	}
	s := &SniperTrader{
		config:   cfg,
		interval: interval,
		maxTotal: decimal.NewFromFloat(cfg.Strategy.Total),
	}
	err = s.Init(ctx)
	return s, err
}

func (s *SniperTrader) Init(ctx context.Context) error {
	if err := s.initLogger(); err != nil {
		return err
	}
	db, err := hs.ConnectMongo(ctx, s.config.Mongo)
	if err != nil {
		return err
	}
	s.db = db
	s.candle = hs.NewCandle(2000)
	if err := s.initEx(); err != nil {
		return err
	}
	s.initRobots(ctx)
	s.Sugar.Info("SuperTrendTrader initialized")
	return nil
}

func (s *SniperTrader) Close(ctx context.Context) {
	if s.db != nil {
		_ = s.db.Client().Disconnect(ctx)
	}
	if s.Sugar != nil {
		s.Sugar.Info("SuperTrendTrader stopped")
		s.Sugar.Sync()
	}
}

func (s *SniperTrader) Print(ctx context.Context) error {
	s.loadState(ctx)
	log.Printf(`State
	Position: %d
	Unique Id: %d
	Long times: %d
	Short times: %d`,
		s.position,
		s.uniqueId,
		s.LongTimes,
		s.ShortTimes,
	)
	//log.Printf(`Sell-stop order
	//Id: %d / %s
	//Price: %s
	//Amount: %s
	//Create Time: %s`,
	//	s.sellStopOrder.Id, s.sellStopOrder.ClientOrderId,
	//	s.sellStopOrder.Price,
	//	s.sellStopOrder.Amount,
	//	s.sellStopOrder.Time,
	//)

	return nil
}

func (s *SniperTrader) Clear(ctx context.Context) error {
	s.clearState(ctx)
	return nil
}

func (s *SniperTrader) Start(ctx context.Context, dry bool) {
	s.loadState(ctx)
	s.checkState(ctx)
	s.dry = dry
	if s.dry {
		s.Sugar.Info("This is dry-run")
	}

	// setup subscriber
	s.ex.SubscribeOrder(s.Symbol(), "sniper-order", s.OrderUpdateHandler)

	{
		to := time.Now()
		from := to.Add(-2000 * s.interval)
		candle, err := s.ex.CandleFrom(s.Symbol(), "sniper-candle", s.interval, from, to)
		if err != nil {
			s.Sugar.Fatalf("get candle error: %s", err)
		}
		s.candle.Add(candle)
	}
	s.ex.SubscribeCandlestick(s.Symbol(), "sniper-tick", s.interval, s.tickerHandler)
}
func (s *SniperTrader) Stop() {
	s.ex.UnsubscribeCandlestick(s.Symbol(), "sniper-tick", s.interval)
	s.ex.UnsubscribeOrder(s.Symbol(), "sniper-order")
}

func (s *SniperTrader) tickerHandler(resp interface{}) {
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
			}
		}
	} else {
		s.Sugar.Warn("Unknown response: %v", resp)
	}
}

func (s *SniperTrader) OrderUpdateHandler(response interface{}) {
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
		if o.ClientOrderId == "" {
			s.Sugar.Debugf("no clientOrderId, not my order %d", o.OrderId)
			return
		}
		//s.Sugar.Debugf("Order update, event: %s, symbol: %s, type: %s, id: %d, clientId: %s, status: %s",
		//	o.EventType, o.Symbol, o.Type, o.OrderId, o.ClientOrderId, o.OrderStatus)
		o2 := rest.Order{
			Id:            uint64(o.OrderId),
			ClientOrderId: o.ClientOrderId,
			Type:          o.Type,
			Status:        o.OrderStatus,
			Updated:       time.Now(),
		}
		switch o.EventType {
		case "creation":
			s.Sugar.Debugf("order created, orderId: %d, clientOrderId: %s", o.OrderId, o.ClientOrderId)
			o2.Price = o.OrderPrice
			switch o.Type {
			case "buy-market":
				//order2.Total = o.order
			case "sell-market":
				o2.Amount = o.OrderSize
			case "buy-limit":
				o2.Amount = o.OrderSize
			case "sell-limit":
				o2.Amount = o.OrderSize
				//o2.StopPrice = o.st
			default:
				s.Sugar.Infof("unprocessed order type: %s", o.Type)
			}
			//go s.createOrder(context.Background(), o2)
		case "cancellation":
			s.Sugar.Debugf("order cancelled, orderId: %d, clientOrderId: %s", o.OrderId, o.ClientOrderId)
			//go s.cancelOrder(context.Background(), o2)
		case "trade":
			s.Sugar.Debugf("order filled, orderId: %d, clientOrderId: %s, fill type: %s",
				o.OrderId, o.ClientOrderId, o.OrderStatus)
			t := rest.Trade{
				Id:     uint64(o.TradeId),
				Price:  o.TradePrice,
				Amount: o.TradeVolume,
				Remain: o.RemainAmt,
				Time:   time.Unix(o.TradeTime/1000, o.TradeTime%1000),
			}
			if p, err1 := decimal.NewFromString(o.TradePrice); err1 == nil {
				if a, err2 := decimal.NewFromString(o.TradeVolume); err2 == nil {
					t.Total = p.Mul(a).String()
				}
			}
			//go s.fillOrder(context.Background(), o2, t)
		case "deletion":
			s.Sugar.Debugf("order deleted, orderId: %d, clientOrderId: %s, fill type: %s",
				o.OrderId, o.ClientOrderId, o.OrderStatus)
			//go s.deleteOrder(context.Background(), o2)
		default:
			s.Sugar.Warnf("unknown eventType, should never happen, orderId: %d, clientOrderId: %s, eventType: %s",
				o.OrderId, o.ClientOrderId, o.EventType)
		}
	}
}

func (s *SniperTrader) onTick(dry bool) {
	//tsl, trend := indicator.SuperTrend(s.config.Strategy.Factor, s.config.Strategy.Period, s.candle.High, s.candle.Low, s.candle.Close)
	//l := s.candle.Length()
	//if l < 3 {
	//	return
	//}
	//for i := l - 3; i < l-1; i++ {
	//	s.Sugar.Debugf(
	//		"[%d] %s %f %f %f %f, %f %v",
	//		i, TimestampToDate(s.candle.Timestamp[i]),
	//		s.candle.Open[i], s.candle.High[i], s.candle.Low[i], s.candle.Close[i],
	//		tsl[i], trend[i],
	//	)
	//}
	//s.Sugar.Debugf("SuperTrend = [..., (%f, %v), (%f, %v)", tsl[l-3], trend[l-3], tsl[l-2], trend[l-2])
	//price := decimal.NewFromFloat(math.Min(s.candle.Close[l-1], s.candle.Close[l-2]))
	//stop := decimal.NewFromFloat(tsl[l-2]).Round(s.PricePrecision())
	//
	//if trend[l-2] && !trend[l-3] {
	//	// false -> true, buy/long
	//	s.Sugar.Info("[Signal] BUY")
	//	s.trend = 1
	//	if !dry {
	//		go s.long(price, stop)
	//	}
	//} else if !trend[l-2] && trend[l-3] {
	//	// true -> false, sell/short
	//	s.Sugar.Info("[Signal] SELL")
	//	s.trend = -1
	//	if !dry {
	//		go s.short(price, stop)
	//	}
	//} else if s.StopLoss() && s.position == 1 {
	//	// update sell-stop-limit order
	//	go s.updateSellStop(stop, false)
	//}
}

func (s *SniperTrader) initLogger() error {
	l, err := hs.NewZapLogger(s.config.Log.Level, s.config.Log.Outputs, s.config.Log.Errors)
	if err != nil {
		return err
	}
	s.Sugar = l.Sugar()
	s.Sugar.Info("Logger initialized")
	return nil
}

func (s *SniperTrader) initEx() error {
	switch s.config.Exchange.Name {
	case "gate":
		s.initGate()
	case "huobi":
		if err := s.initHuobi(); err != nil {
			return err
		}
	default:
		return errors.New("unsupported exchange")
	}
	s.Sugar.Info("Exchange initialized")
	s.Sugar.Infof(
		"Symbol: %s, PricePrecision: %d, AmountPrecision: %d, MinAmount: %s, MinTotal: %s",
		s.Symbol(),
		s.PricePrecision(), s.AmountPrecision(),
		s.MinAmount(), s.MinTotal(),
	)
	s.Sugar.Infof(
		"BaseMakerFee: %s, BaseTakerFee: %s, ActualMakerFee: %s, ActualTakerFee: %s",
		s.BaseMakerFee(), s.BaseTakerFee(),
		s.ActualMakerFee(), s.ActualTakerFee(),
	)
	return nil
}

func (s *SniperTrader) initGate() {
	//s.ex = gateio.New(s.config.Exchange.Key, s.config.Exchange.Secret, s.config.Exchange.Host)
}

func (s *SniperTrader) initHuobi() (err error) {
	s.ex, err = huobi.New(s.config.Exchange.Label, s.config.Exchange.Key, s.config.Exchange.Secret, s.config.Exchange.Host)
	if err != nil {
		return err
	}
	s.symbol, err = s.ex.GetSymbol(s.config.Exchange.Symbols[0])
	if err != nil {
		return err
	}
	s.fee, err = s.ex.GetFee(s.Symbol())
	return err
}

func (s *SniperTrader) initRobots(ctx context.Context) {
	for _, conf := range s.config.Robots {
		s.robots = append(s.robots, broadcast.New(conf))
	}
	s.Sugar.Info("Broadcasters initialized")
}

func (s *SniperTrader) Symbol() string {
	return s.symbol.Symbol
}
func (s *SniperTrader) QuoteCurrency() string {
	return s.symbol.QuoteCurrency
}
func (s *SniperTrader) BaseCurrency() string {
	return s.symbol.BaseCurrency
}
func (s *SniperTrader) PricePrecision() int32 {
	return s.symbol.PricePrecision
}
func (s *SniperTrader) AmountPrecision() int32 {
	return s.symbol.AmountPrecision
}
func (s *SniperTrader) MinAmount() decimal.Decimal {
	return s.symbol.MinAmount
}
func (s *SniperTrader) MinTotal() decimal.Decimal {
	return s.symbol.MinTotal
}
func (s *SniperTrader) BaseMakerFee() decimal.Decimal {
	return s.fee.BaseMaker
}
func (s *SniperTrader) BaseTakerFee() decimal.Decimal {
	return s.fee.BaseTaker
}
func (s *SniperTrader) ActualMakerFee() decimal.Decimal {
	return s.fee.ActualMaker
}
func (s *SniperTrader) ActualTakerFee() decimal.Decimal {
	return s.fee.ActualTaker
}

func (s *SniperTrader) Broadcast(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	labels := []string{s.config.Exchange.Name, s.config.Exchange.Label}
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	timeStr := time.Now().In(beijing).Format(layout)

	msg := fmt.Sprintf("%s [%s] [%s] %s", timeStr, strings.Join(labels, "] ["), s.Symbol(), message)
	for _, robot := range s.robots {
		if err := robot.SendText(msg); err != nil {
			s.Sugar.Infof("broadcast error: %s", err)
		}
	}
}

func (s *SniperTrader) loadState(ctx context.Context) {
	coll := s.db.Collection(collNameState)
	if uniqueId, err := hs.LoadInt64(ctx, coll, "uniqueId"); err != nil {
		s.Sugar.Fatalf("load UniqueId error: %s", err)
	} else if uniqueId != 0 {
		s.uniqueId = uniqueId
		s.Sugar.Infof("loaded UniqueId: %d", uniqueId)
	}
	if position, err := hs.LoadInt64(ctx, coll, "position"); err != nil {
		s.Sugar.Fatalf("load position error: %s", err)
	} else if position != 0 {
		s.position = position
		s.Sugar.Infof("loaded position: %d", position)
	}
	if longTimes, err := hs.LoadInt64(ctx, coll, "longTimes"); err != nil {
		s.Sugar.Fatalf("load longTimes error: %s", err)
	} else if longTimes != 0 {
		s.LongTimes = longTimes
		s.Sugar.Infof("loaded longTimes: %d", longTimes)
	}
	if shortTimes, err := hs.LoadInt64(ctx, coll, "shortTimes"); err != nil {
		s.Sugar.Fatalf("load shortTimes error: %s", err)
	} else if shortTimes != 0 {
		s.ShortTimes = shortTimes
		s.Sugar.Infof("loaded shortTimes: %d", shortTimes)
	}
}

func (s *SniperTrader) clearState(ctx context.Context) {
	coll := s.db.Collection(collNameState)
	if err := hs.DeleteInt64(ctx, coll, "longTimes"); err != nil {
		s.Sugar.Errorf("delete longTimes error: %s", err)
	} else {
		s.Sugar.Info("delete longTimes from database")
	}
	if err := hs.DeleteInt64(ctx, coll, "shortTimes"); err != nil {
		s.Sugar.Errorf("delete shortTimes error: %s", err)
	} else {
		s.Sugar.Info("delete shortTimes from database")
	}
	if err := hs.DeleteInt64(ctx, coll, "sellStopOrder"); err != nil {
		s.Sugar.Errorf("delete sellStopOrder error: %s", err)
	} else {
		s.Sugar.Info("delete sellStopOrder from database")
	}
}

func (s *SniperTrader) checkState(ctx context.Context) {
	// check sell-stop order
	//s.checkSellStopOrder(ctx)
}
