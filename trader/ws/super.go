package ws

import (
	"context"
	"errors"
	"fmt"
	"github.com/huobirdcenter/huobi_golang/pkg/model/market"
	"github.com/huobirdcenter/huobi_golang/pkg/model/order"
	"github.com/shopspring/decimal"
	"github.com/xyths/go-indicators"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/hs/exchange/huobi"
	"github.com/xyths/hs/logger"
	"github.com/xyths/qtr/executor"
	"github.com/xyths/qtr/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"log"
	"math"
	"strings"
	"sync"
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
	Total     float64
	Interval  string
	Factor    float64
	Period    int
	StopLoss  bool `json:"stopLoss"`
	Reinforce float64
}

type SuperTrendTrader struct {
	config   SuperTrendConfig
	interval time.Duration
	dry      bool

	Sugar  *zap.SugaredLogger
	db     *mongo.Database
	ex     exchange.Exchange
	symbol exchange.Symbol
	fee    exchange.Fee
	robots []broadcast.Broadcaster

	maxTotal decimal.Decimal // max total for buy order, half total in config

	candle     hs.Candle
	uniqueId   int64
	position   int64 // 1 long (full), -1 short (clear)
	trend      int64 // 1: long, -1 short
	LongTimes  int64
	ShortTimes int64

	sellStopLock  sync.Mutex
	sellStopOrder executor.SellStopOrder

	reinforceLock      sync.Mutex
	reinforceBuyOrder  executor.ReinforceOrder
	reinforceSellOrder executor.ReinforceOrder
}

const (
	collNameState = "state"
	collNameOrder = "order"

	sep                      = "-"
	prefixBuyMarketOrder     = "bm"
	prefixBuyLimitOrder      = "bl"
	prefixBuyStopOrder       = "bs"
	prefixBuyReinforceOrder  = "br"
	prefixSellMarketOrder    = "sm"
	prefixSellLimitOrder     = "sl"
	prefixSellStopOrder      = "ss"
	prefixSellReinforceOrder = "sr"
)

var (
	emptySellStopOrder      = executor.SellStopOrder{Name: "sellStopOrder"}
	emptyReinforceBuyOrder  = executor.SellStopOrder{Name: "reinforceBuyOrder"}
	emptyReinforceSellOrder = executor.SellStopOrder{Name: "reinforceSellOrder"}
)

func NewSuperTrendTrader(ctx context.Context, configFilename string) (*SuperTrendTrader, error) {
	cfg := SuperTrendConfig{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		return nil, err
	}
	interval, err := time.ParseDuration(cfg.Strategy.Interval)
	if err != nil {
		logger.Sugar.Fatalf("error interval format: %s", cfg.Strategy.Interval)
	}
	s := &SuperTrendTrader{
		config:   cfg,
		interval: interval,
		maxTotal: decimal.NewFromFloat(cfg.Strategy.Total / 2),

		sellStopOrder:      emptySellStopOrder,
		reinforceBuyOrder:  emptyReinforceBuyOrder,
		reinforceSellOrder: emptyReinforceSellOrder,
	}
	err = s.Init(ctx)
	return s, err
}

func (s *SuperTrendTrader) Init(ctx context.Context) error {
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

func (s *SuperTrendTrader) Close(ctx context.Context) {
	if s.db != nil {
		_ = s.db.Client().Disconnect(ctx)
	}
	if s.Sugar != nil {
		s.Sugar.Info("SuperTrendTrader stopped")
		s.Sugar.Sync()
	}
}

func (s *SuperTrendTrader) Print(ctx context.Context) error {
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
	log.Printf(`Sell-stop order
	Id: %d / %s
	Price: %s
	Amount: %s
	Create Time: %s`,
		s.sellStopOrder.Id, s.sellStopOrder.ClientOrderId,
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

func (s *SuperTrendTrader) Start(ctx context.Context, dry bool) {
	s.loadState(ctx)
	s.checkState(ctx)
	s.dry = dry
	if s.dry {
		s.Sugar.Info("This is dry-run")
	}

	// setup subscriber
	s.ex.SubscribeOrder(s.Symbol(), "super-order", s.OrderUpdateHandler)

	{
		candle, err := s.ex.CandleBySize(s.Symbol(), s.interval, 2000)
		if err != nil {
			s.Sugar.Fatalf("get candle error: %s", err)
		}
		s.candle.Add(candle)
		s.onTick(s.dry)
	}
	s.ex.SubscribeCandlestick(s.Symbol(), "super-tick", s.interval, s.tickerHandler)
}
func (s *SuperTrendTrader) Stop() {
	s.ex.UnsubscribeCandlestick(s.Symbol(), "super-tick", s.interval)
	s.ex.UnsubscribeOrder(s.Symbol(), "super-order")
}

func (s *SuperTrendTrader) initLogger() error {
	l, err := hs.NewZapLogger(s.config.Log)
	if err != nil {
		return err
	}
	s.Sugar = l.Sugar()
	s.Sugar.Info("Logger initialized")
	return nil
}

func (s *SuperTrendTrader) initEx() error {
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

func (s *SuperTrendTrader) initGate() {
	//s.ex = gateio.New(s.config.Exchange.Key, s.config.Exchange.Secret, s.config.Exchange.Host)
}

func (s *SuperTrendTrader) initHuobi() (err error) {
	s.ex, err = huobi.New(s.config.Exchange.Label, s.config.Exchange.Key, s.config.Exchange.Secret, s.config.Exchange.Host)
	if err != nil {
		return err
	}
	s.symbol, err = s.ex.GetSymbol(context.Background(), s.config.Exchange.Symbols[0])
	if err != nil {
		return err
	}
	s.fee, err = s.ex.GetFee(s.Symbol())
	return err
}

func (s *SuperTrendTrader) initRobots(ctx context.Context) {
	for _, conf := range s.config.Robots {
		s.robots = append(s.robots, broadcast.New(conf))
	}
	s.Sugar.Info("Broadcasters initialized")
}

func (s *SuperTrendTrader) StopLoss() bool {
	return s.config.Strategy.StopLoss
}
func (s *SuperTrendTrader) Reinforce() float64 {
	return s.config.Strategy.Reinforce
}

func (s *SuperTrendTrader) Symbol() string {
	return s.symbol.Symbol
}
func (s *SuperTrendTrader) QuoteCurrency() string {
	return s.symbol.QuoteCurrency
}
func (s *SuperTrendTrader) BaseCurrency() string {
	return s.symbol.BaseCurrency
}
func (s *SuperTrendTrader) PricePrecision() int32 {
	return s.symbol.PricePrecision
}
func (s *SuperTrendTrader) AmountPrecision() int32 {
	return s.symbol.AmountPrecision
}
func (s *SuperTrendTrader) MinAmount() decimal.Decimal {
	return s.symbol.MinAmount
}
func (s *SuperTrendTrader) MinTotal() decimal.Decimal {
	return s.symbol.MinTotal
}
func (s *SuperTrendTrader) BaseMakerFee() decimal.Decimal {
	return s.fee.BaseMaker
}
func (s *SuperTrendTrader) BaseTakerFee() decimal.Decimal {
	return s.fee.BaseTaker
}
func (s *SuperTrendTrader) ActualMakerFee() decimal.Decimal {
	return s.fee.ActualMaker
}
func (s *SuperTrendTrader) ActualTakerFee() decimal.Decimal {
	return s.fee.ActualTaker
}

func (s *SuperTrendTrader) buy(symbol string, price, stopPrice decimal.Decimal, amountPrecision int32, minAmount, minTotal decimal.Decimal) {
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
	maxTotal := balance[s.QuoteCurrency()]
	if maxTotal.GreaterThan(s.maxTotal) {
		maxTotal = s.maxTotal
	}
	amount := maxTotal.DivRound(price, amountPrecision)
	total := amount.Mul(price)
	if amount.LessThan(minAmount) { //|| total.LessThan(minTotal) or total (%s / %s), total, minTotal
		s.Sugar.Infof("amount (%s / %s) is too small", amount, minAmount)
		//full
		s.SetPosition(1)
		return
	}
	clientId := GetClientOrderId(sep, prefixBuyLimitOrder, s.ShortTimes, s.LongTimes+1, s.GetUniqueId())
	orderId, err := s.ex.BuyLimit(symbol, clientId, price, amount)
	if err != nil {
		s.Sugar.Errorf("buy error: %s", err)
		return
	}
	o := executor.Order{
		Id:            orderId,
		ClientOrderId: clientId,
		StopPrice:     stopPrice.String(),
		Total:         total.String(),
		Updated:       time.Now(),
	}
	s.addOrder(context.Background(), o)
	s.Sugar.Infof("限价买入，订单号: %d / %s, price: %s, amount: %s, total: %s", orderId, clientId, price, amount, total)
	s.Broadcast("限价买入，订单号: %d / %s, 价格: %s, 数量: %s, 买入总金额: %s", orderId, clientId, price, amount, total)

	s.SetPosition(1)
	s.LongTimes++
	if err := hs.SaveInt64(context.Background(), s.db.Collection(collNameState), "longTimes", s.LongTimes); err != nil {
		s.Sugar.Infof("save longTimes error: %s", err)
	}
}

func (s *SuperTrendTrader) sell(symbol exchange.Symbol) {
	balance, err := s.ex.SpotAvailableBalance()
	if err != nil {
		s.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	s.Sugar.Infof("balance: %v", balance)
	if s.position == -1 {
		s.Sugar.Infof("position already clear: %v", balance)
		return
	}
	// sell all balance
	amount := balance[symbol.BaseCurrency].Round(symbol.AmountPrecision)
	if amount.GreaterThan(balance[symbol.BaseCurrency]) {
		amount = amount.Sub(symbol.MinAmount)
	}
	//logger.Sugar.Debugf("try to sell %s, balance: %v, amount: %s, price: %s", symbol, balance, amount, price)
	if amount.LessThan(symbol.MinAmount) {
		s.Sugar.Infof("amount too small: %s", amount)
		s.SetPosition(-1)
		return
	}
	clientId := GetClientOrderId(sep, prefixSellMarketOrder, s.ShortTimes+1, s.LongTimes, s.GetUniqueId())
	orderId, err := s.ex.SellMarket(symbol, clientId, amount)
	if err != nil {
		s.Sugar.Errorf("sell error: %s", err)
		return
	}
	o := executor.Order{Id: orderId, ClientOrderId: clientId}
	s.addOrder(context.Background(), o)
	s.Sugar.Infof("市价清仓，订单号: %d / %s, amount: %s", orderId, clientId, amount)
	s.Broadcast("市价清仓，订单号: %d / %s, 卖出数量: %s", orderId, clientId, amount)

	s.SetPosition(-1)
	s.ShortTimes++
	if err := hs.SaveInt64(context.Background(), s.db.Collection(collNameState), "shortTimes", s.ShortTimes); err != nil {
		s.Sugar.Infof("save shortTimes error: %s", err)
	}
}

func (s *SuperTrendTrader) long(price, stopPrice decimal.Decimal) {
	s.Sugar.Infof("long %s, price %s, stopPrice %s", s.Symbol(), price, stopPrice)
	s.buy(s.Symbol(), price, stopPrice, s.AmountPrecision(), s.MinAmount(), s.MinTotal())
	if s.Reinforce() > 0 {
		// place reinforce order
		p := decimal.NewFromInt(1).Sub(s.fee.ActualMaker.Mul(decimal.NewFromFloat(2 + s.Reinforce())))
		s.Sugar.Debugf("p is %s", p)
		lower := price.Mul(p).Round(s.PricePrecision())
		s.reinforceBuy(lower)
	}
}

func (s *SuperTrendTrader) short(price, stopPrice decimal.Decimal) {
	s.Sugar.Infof("short %s, price %s, stopPrice %s", s.Symbol(), price, stopPrice)
	if s.StopLoss() {
		s.cancelSellStop()
	}
	if s.Reinforce() > 0 {
		// cancel reinforce order
		s.cancelReinforce()
	}
	s.sell(s.symbol)
}

func (s *SuperTrendTrader) sellStopLimit(symbol, currency string, price decimal.Decimal,
	amountPrecision int32, minAmount, minTotal decimal.Decimal) (*executor.SellStopOrder, error) {
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
	s.Sugar.Debugf("try to sell %s, balance: %v, amount: %s, price: %s, total: %s",
		symbol, balance, amount, price, price.Mul(amount))
	if amount.LessThan(minAmount) {
		s.Sugar.Infof("amount too small: %s (need %s)", amount, minAmount)
		s.SetPosition(-1)
		s.Sugar.Info("update position to `clear`")
		return nil, nil
	}
	total := price.Mul(amount)
	if total.LessThan(minTotal) {
		s.Sugar.Infof("total too small: %s (need %s)", total, minTotal)
		s.SetPosition(-1)
		s.Sugar.Info("update position to `clear`")
		return nil, nil
	}
	clientId := GetClientOrderId(sep, prefixSellStopOrder, s.ShortTimes, s.LongTimes, s.GetUniqueId())
	orderId, err := s.ex.SellStopLimit(symbol, clientId, price, amount, price)
	if err != nil {
		s.Sugar.Errorf("sell-stop error: %s", err)
		return nil, err
	}
	o := executor.Order{
		Id:            orderId,
		ClientOrderId: clientId,
		Type:          "sell-stop-limit",
	}
	s.addOrder(context.Background(), o)
	s.Sugar.Infof("止损单，订单号: %d / %s, amount: %s", orderId, clientId, amount)
	sso := emptySellStopOrder
	sso.Id = orderId
	sso.ClientOrderId = clientId
	sso.Price = price.String()
	sso.StopPrice = price.String()
	sso.Amount = amount.String()
	sso.Total = total.String()
	sso.Time = time.Now().String()
	sso.Status = "created"

	return &sso, nil
}

func (s *SuperTrendTrader) cancelSellStop() {
	s.sellStopLock.Lock()
	defer s.sellStopLock.Unlock()

	if s.sellStopOrder.Id != 0 {
		if err := s.ex.CancelOrder(s.Symbol(), uint64(s.sellStopOrder.Id)); err != nil {
			s.Sugar.Errorf("cancel sell-stop-limit order error: %s", err)
		} else {
			s.Sugar.Infof("cancelled sell-stop-limit order %s / %d", s.sellStopOrder.ClientOrderId, s.sellStopOrder.Id)
			s.SetSellStopOrder(emptySellStopOrder)
		}
	}
}

func (s *SuperTrendTrader) updateSellStop(price decimal.Decimal, force bool) {
	s.sellStopLock.Lock()
	defer s.sellStopLock.Unlock()
	if !force && s.sellStopOrder.Price != "" {
		oldStopPrice := decimal.RequireFromString(s.sellStopOrder.Price)
		if price.Equal(oldStopPrice) {
			s.Sugar.Debug("same stop price, no need to update")
			return
		}
	}
	s.Sugar.Debugf("try to place sell-stop-limit order, price %s", price)
	// clear old order
	if s.sellStopOrder.Id != 0 {
		if err := s.ex.CancelOrder(s.Symbol(), uint64(s.sellStopOrder.Id)); err != nil {
			s.Sugar.Errorf("cancel sell-stop-limit order error: %s", err)
		} else {
			s.Sugar.Infof("cancelled sell-stop-limit order %s / %d", s.sellStopOrder.ClientOrderId, s.sellStopOrder.Id)
			s.SetSellStopOrder(emptySellStopOrder)
		}
	}

	// place new order
	if sso, err := s.sellStopLimit(
		s.Symbol(), s.BaseCurrency(), price,
		s.AmountPrecision(), s.MinAmount(), s.MinTotal(),
	); err != nil {
		s.Sugar.Errorf("sell-stop error: %s", err)
	} else if sso != nil {
		s.SetSellStopOrder(*sso)
	}
}

func (s *SuperTrendTrader) cancelReinforce() {
	s.reinforceLock.Lock()
	defer s.reinforceLock.Unlock()

	s.realCancelReinforce(&s.reinforceBuyOrder)
	s.realCancelReinforce(&s.reinforceSellOrder)
}

func (s *SuperTrendTrader) reinforceBuy(price decimal.Decimal) {
	s.reinforceLock.Lock()
	defer s.reinforceLock.Unlock()

	if s.reinforceBuyOrder.Id != 0 {
		s.realCancelReinforce(&s.reinforceBuyOrder)
	}

	balance, err := s.ex.SpotAvailableBalance()
	if err != nil {
		s.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	s.Sugar.Infof("balance: %v", balance)
	maxTotal := balance[s.QuoteCurrency()]
	if maxTotal.GreaterThan(s.maxTotal) {
		maxTotal = s.maxTotal
	}
	amount := maxTotal.DivRound(price, s.AmountPrecision())
	total := amount.Mul(price)
	if amount.LessThan(s.MinAmount()) {
		s.Sugar.Infof("amount (%s / %s) is too small", amount, s.MinAmount())
		return
	}
	clientId := GetClientOrderId(sep, prefixBuyReinforceOrder, s.ShortTimes, s.LongTimes+1, s.GetUniqueId())
	orderId, err := s.ex.BuyLimit(s.Symbol(), clientId, price, amount)
	if err != nil {
		s.Sugar.Errorf("buy error: %s", err)
		return
	}
	o := executor.Order{
		Id:            orderId,
		ClientOrderId: clientId,
		Total:         total.String(),
		Updated:       time.Now(),
	}
	s.reinforceBuyOrder.Order = o
	coll := s.db.Collection(collNameState)
	if err := hs.SaveKey(context.Background(), coll, s.reinforceBuyOrder.Name, s.reinforceBuyOrder); err != nil {
		s.Sugar.Errorf("save reinforceOrder error: %s", err)
	}
	s.addOrder(context.Background(), o)
	s.Sugar.Infof("限价买入，订单号: %d / %s, price: %s, amount: %s, total: %s", orderId, clientId, price, amount, total)
	s.Broadcast("限价买入，订单号: %d / %s, 价格: %s, 数量: %s, 买入总金额: %s", orderId, clientId, price, amount, total)
}

func (s *SuperTrendTrader) reinforceSell(price, amount decimal.Decimal) {
	s.reinforceLock.Lock()
	defer s.reinforceLock.Unlock()

	if s.reinforceSellOrder.Id != 0 {
		if a, err := decimal.NewFromString(s.reinforceSellOrder.Amount); err != nil {
			s.Sugar.Errorf("parse reinforce sell order amount error: %s", err)
		} else {
			amount.Add(a)
		}
		s.realCancelReinforce(&s.reinforceSellOrder)
	}

	balance, err := s.ex.SpotAvailableBalance()
	if err != nil {
		s.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	s.Sugar.Infof("balance: %v", balance)
	if amount.GreaterThan(balance[s.BaseCurrency()]) {
		amount = balance[s.BaseCurrency()]
	}
	amount = amount.Round(s.AmountPrecision())
	if amount.LessThan(s.MinAmount()) {
		s.Sugar.Infof("amount too small: %s", amount)
		return
	}
	price = price.Round(s.PricePrecision())
	clientId := GetClientOrderId(sep, prefixSellReinforceOrder, s.ShortTimes+1, s.LongTimes, s.GetUniqueId())
	orderId, err := s.ex.SellLimit(s.Symbol(), clientId, price, amount)
	if err != nil {
		s.Sugar.Errorf("sell error: %s", err)
		return
	}
	o := executor.Order{Id: orderId, ClientOrderId: clientId}
	s.reinforceSellOrder.Order = o
	coll := s.db.Collection(collNameState)
	if err := hs.SaveKey(context.Background(), coll, s.reinforceSellOrder.Name, s.reinforceSellOrder); err != nil {
		s.Sugar.Errorf("save reinforceOrder error: %s", err)
	}
	s.addOrder(context.Background(), o)
	s.Sugar.Infof("限价卖出，订单号: %d / %s, amount: %s", orderId, clientId, amount)
	s.Broadcast("限价卖出，订单号: %d / %s, 卖出数量: %s", orderId, clientId, amount)
}

func (s *SuperTrendTrader) realCancelReinforce(o *executor.ReinforceOrder) {
	if o.Id == 0 {
		return
	}
	if err := s.ex.CancelOrder(s.Symbol(), uint64(o.Id)); err != nil {
		s.Sugar.Errorf("cancel reinforce order error: %s", err)
	} else {
		s.Sugar.Infof("cancelled reinforce order %s / %d",
			o.ClientOrderId, o.Id)
		o.Clear()
		coll := s.db.Collection(collNameState)
		if err := hs.SaveKey(context.Background(), coll, o.Name, o); err != nil {
			s.Sugar.Errorf("save reinforceOrder error: %s", err)
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

	msg := fmt.Sprintf("%s [%s] [%s] %s", timeStr, strings.Join(labels, "] ["), s.Symbol(), message)
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
		if o.ClientOrderId == "" {
			s.Sugar.Debugf("no clientOrderId, not my order %d", o.OrderId)
			return
		}
		//s.Sugar.Debugf("Order update, event: %s, symbol: %s, type: %s, id: %d, clientId: %s, status: %s",
		//	o.EventType, o.Symbol, o.Type, o.OrderId, o.ClientOrderId, o.OrderStatus)
		o2 := executor.Order{
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
			go s.createOrder(context.Background(), o2)
		case "cancellation":
			s.Sugar.Debugf("order cancelled, orderId: %d, clientOrderId: %s", o.OrderId, o.ClientOrderId)
			go s.cancelOrder(context.Background(), o2)
		case "trade":
			s.Sugar.Debugf("order filled, orderId: %d, clientOrderId: %s, fill type: %s",
				o.OrderId, o.ClientOrderId, o.OrderStatus)
			t := executor.Trade{
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
			go s.fillOrder(context.Background(), o2, t)
		case "deletion":
			s.Sugar.Debugf("order deleted, orderId: %d, clientOrderId: %s, fill type: %s",
				o.OrderId, o.ClientOrderId, o.OrderStatus)
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
		s.Sugar.Debugf(
			"[%d] %s %f %f %f %f, %f %v",
			i, types.TimestampToDate(s.candle.Timestamp[i]),
			s.candle.Open[i], s.candle.High[i], s.candle.Low[i], s.candle.Close[i],
			tsl[i], trend[i],
		)
	}
	s.Sugar.Debugf("SuperTrend = [..., (%f, %v), (%f, %v)", tsl[l-3], trend[l-3], tsl[l-2], trend[l-2])
	price := decimal.NewFromFloat(math.Min(s.candle.Close[l-1], s.candle.Close[l-2]))
	stop := decimal.NewFromFloat(tsl[l-2]).Round(s.PricePrecision())

	if trend[l-2] && (!trend[l-3] || s.position != 1) {
		// false -> true, buy/long
		s.Sugar.Info("[Signal] BUY")
		s.trend = 1
		if !dry {
			go s.long(price, stop)
		}
	} else if !trend[l-2] && trend[l-3] {
		// true -> false, sell/short
		s.Sugar.Info("[Signal] SELL")
		s.trend = -1
		if !dry {
			go s.short(price, stop)
		}
	} else if s.StopLoss() && s.position == 1 {
		// update sell-stop-limit order
		go s.updateSellStop(stop, false)
	}
}

func (s *SuperTrendTrader) loadState(ctx context.Context) {
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
	if s.StopLoss() {
		sellStopOrder := emptySellStopOrder
		if err := hs.LoadKey(ctx, coll, "sellStopOrder", &sellStopOrder); err != nil {
			s.Sugar.Fatalf("load sellStopOrder error: %s", err)
		} else if sellStopOrder.Id != 0 {
			s.sellStopOrder = sellStopOrder
			s.Sugar.Infof("loaded sellStopOrder: %v", sellStopOrder)
		}
	}
	if s.Reinforce() > 0 {
		buy := emptyReinforceBuyOrder
		if err := hs.LoadKey(ctx, coll, buy.Name, &buy); err != nil {
			s.Sugar.Errorf("load reinforce buy order error: %s", err)
		} else if buy.Id != 0 {
			s.reinforceBuyOrder = buy
			s.Sugar.Infof("loaded reinforceBuyOrder: %v", buy)
		}
		sell := emptyReinforceBuyOrder
		if err := hs.LoadKey(ctx, coll, sell.Name, &sell); err != nil {
			s.Sugar.Errorf("load reinforce sell order error: %s", err)
		} else if sell.Id != 0 {
			s.reinforceSellOrder = sell
			s.Sugar.Infof("loaded reinforceSellOrder: %v", sell)
		}
	}
}

func (s *SuperTrendTrader) GetUniqueId() int64 {
	s.uniqueId = (s.uniqueId + 1) % 10000
	coll := s.db.Collection(collNameState)
	if err := hs.SaveInt64(context.Background(), coll, "uniqueId", s.uniqueId); err != nil {
		s.Sugar.Errorf("save uniqueId error: %s", err)
	}
	return s.uniqueId
}
func (s *SuperTrendTrader) SetPosition(newPosition int64) {
	s.position = newPosition
	coll := s.db.Collection(collNameState)
	if err := hs.SaveInt64(context.Background(), coll, "position", s.position); err != nil {
		s.Sugar.Errorf("save position error: %s", err)
	}
}
func (s *SuperTrendTrader) SetSellStopOrder(newOrder executor.SellStopOrder) {
	coll := s.db.Collection(collNameState)
	s.sellStopOrder = newOrder
	if err := hs.SaveKey(context.Background(), coll, s.sellStopOrder.Name, s.sellStopOrder); err != nil {
		s.Sugar.Errorf("save sellStopOrder error: %s", err)
	}
}

func (s *SuperTrendTrader) clearState(ctx context.Context) {
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

func (s *SuperTrendTrader) checkState(ctx context.Context) {
	// check sell-stop order
	s.checkSellStopOrder(ctx)
}

func (s *SuperTrendTrader) addOrder(ctx context.Context, o executor.Order) {
	coll := s.db.Collection(collNameOrder)
	if _, err := coll.InsertOne(ctx, o); err != nil {
		s.Sugar.Errorf("add order error: %s", err)
	}
}

func (s *SuperTrendTrader) createOrder(ctx context.Context, o executor.Order) {
	coll := s.db.Collection(collNameOrder)
	option := options.FindOneAndUpdate().SetUpsert(true)
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
func (s *SuperTrendTrader) fillOrder(ctx context.Context, o executor.Order, t executor.Trade) {
	s.Broadcast("订单成交(%s), 订单号: %d / %s, 价格: %s, 数量: %s, 交易额: %s",
		o.Status, o.Id, o.ClientOrderId, t.Price, t.Amount, t.Total)
	coll := s.db.Collection(collNameOrder)
	option := options.FindOneAndUpdate().SetUpsert(true)
	r := coll.FindOneAndUpdate(
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
	)
	if r.Err() != nil {
		s.Sugar.Errorf("fill order error: %s", r.Err())
	}

	if strings.HasPrefix(o.ClientOrderId, prefixSellStopOrder) {
		if o.Status == "filled" {
			s.SetPosition(-1)
			s.Sugar.Infof("sell-stop order %d is filled, change position to -1 (clear)", o.Id)
			s.SetSellStopOrder(emptySellStopOrder)
		}
		// place buy-stop order?
	} else if strings.HasPrefix(o.ClientOrderId, prefixBuyLimitOrder) {
		s.Sugar.Infof("buy order %d / %s %s", o.Id, o.ClientOrderId, o.Status)
		if !s.StopLoss() {
			return
		}
		// find stopPrice
		old := executor.Order{}
		if err := r.Decode(&old); err != nil {
			s.Sugar.Errorf("decode order error: %s", err)
			return
		}
		if old.StopPrice == "" {
			s.Sugar.Errorf("order %s / %s has no stopPrice", o.Id, o.ClientOrderId)
			return
		}

		stopPrice, err1 := decimal.NewFromString(old.StopPrice)
		if err1 != nil {
			s.Sugar.Errorf("parse stopPrice error: %s", err1)
			return
		}
		s.updateSellStop(stopPrice, true)
	} else if strings.HasPrefix(o.ClientOrderId, prefixBuyReinforceOrder) {
		// place sell
		price, err := decimal.NewFromString(t.Price)
		if err != nil {
			s.Sugar.Errorf("bad trade price: %s", err)
			return
		}
		amount, err := decimal.NewFromString(t.Amount)
		if err != nil {
			s.Sugar.Errorf("bad trade amount: %s", err)
			return
		}
		p := decimal.NewFromInt(1).Sub(s.fee.ActualMaker.Mul(decimal.NewFromFloat(2 + s.Reinforce())))
		s.Sugar.Debugf("p is %s", p)
		price = price.DivRound(p.Mul(p), s.PricePrecision())
		s.reinforceSell(price, amount)
	} else if strings.HasPrefix(o.ClientOrderId, prefixSellReinforceOrder) {
		// place buy
		price, err := decimal.NewFromString(t.Price)
		if err != nil {
			s.Sugar.Errorf("bad trade price: %s", err)
			return
		}
		p := decimal.NewFromInt(1).Sub(s.fee.ActualMaker.Mul(decimal.NewFromFloat(2 + s.Reinforce())))
		s.Sugar.Debugf("p is %s", p)
		price = price.Mul(p).Mul(p).Round(s.PricePrecision())
		s.reinforceBuy(price)
	}
}
func (s *SuperTrendTrader) cancelOrder(ctx context.Context, o executor.Order) {
	s.updateOrderStatus(ctx, o)
}
func (s *SuperTrendTrader) deleteOrder(ctx context.Context, o executor.Order) {
	s.updateOrderStatus(ctx, o)
}
func (s *SuperTrendTrader) updateOrderStatus(ctx context.Context, o executor.Order) {
	coll := s.db.Collection(collNameOrder)
	option := options.FindOneAndUpdate().SetUpsert(true)
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
	o, err := s.ex.GetOrderById(uint64(s.sellStopOrder.Id), s.Symbol())
	if err != nil {
		s.Sugar.Infof("get sell-stop order error: %s", err)
		return
	}
	if o.Status == "filled" || o.Status == "cancelled" {
		s.SetSellStopOrder(emptySellStopOrder)
	} else {
		s.Sugar.Infof("sell-stop order id: %d / %s, price: %s, amount: %s, total: %s, status: %s, filled: %s",
			o.Id, o.ClientOrderId, o.Price, o.Amount, o.Price.Mul(o.Amount), o.Status, o.FilledAmount)
	}
}
