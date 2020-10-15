package executor

import (
	"context"
	"github.com/huobirdcenter/huobi_golang/pkg/model/order"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/hs/exchange/huobi"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"
)

type Executor struct {
	config   hs.ExchangeConf
	maxTotal decimal.Decimal
	Sugar    *zap.SugaredLogger
	db       *mongo.Database

	ex     exchange.Exchange
	symbol exchange.Symbol
	fee    exchange.Fee
	id     ClientIdManager

	quota Quota

	buyOrderLock  sync.RWMutex
	buyOrderId    uint64
	sellOrderLock sync.RWMutex
	sellOrderId   uint64

	OrderProxy
}

func NewExecutor(config hs.ExchangeConf) (*Executor, error) {
	e := Executor{
		config: config,
	}
	ex, err := huobi.New(e.config.Label, e.config.Key, e.config.Secret, e.config.Host)
	if err != nil {
		return nil, err
	}
	e.ex = ex
	e.symbol, err = e.ex.GetSymbol(e.config.Symbols[0])
	if err != nil {
		return nil, err
	}
	e.fee, err = e.ex.GetFee(e.Symbol())
	if err != nil {
		return nil, err
	}

	return &e, nil
}

func (e *Executor) Init(sugar *zap.SugaredLogger, db *mongo.Database, maxTotal decimal.Decimal) {
	e.Sugar = sugar
	e.db = db
	e.maxTotal = maxTotal
	e.id.Init("-", db.Collection(collNameState))
	e.OrderProxy.Init(db.Collection(collNameOrder))
	e.quota.Init(db.Collection(collNameState), e.maxTotal)
}

func (e *Executor) Load(ctx context.Context) error {
	if err := e.quota.Load(ctx); err != nil {
		return err
	}
	if err := e.LoadBuyOrderId(ctx); err != nil {
		return err
	}
	if err := e.LoadSellOrderId(ctx); err != nil {
		return err
	}
	return nil
}

func (e *Executor) Exchange() exchange.Exchange {
	return e.ex
}
func (e *Executor) Symbol() string {
	return e.symbol.Symbol
}
func (e *Executor) QuoteCurrency() string {
	return e.symbol.QuoteCurrency
}
func (e *Executor) BaseCurrency() string {
	return e.symbol.BaseCurrency
}
func (e *Executor) PricePrecision() int32 {
	return e.symbol.PricePrecision
}
func (e *Executor) AmountPrecision() int32 {
	return e.symbol.AmountPrecision
}
func (e *Executor) MinAmount() decimal.Decimal {
	return e.symbol.MinAmount
}
func (e *Executor) MinTotal() decimal.Decimal {
	return e.symbol.MinTotal
}
func (e *Executor) BaseMakerFee() decimal.Decimal {
	return e.fee.BaseMaker
}
func (e *Executor) BaseTakerFee() decimal.Decimal {
	return e.fee.BaseTaker
}
func (e *Executor) ActualMakerFee() decimal.Decimal {
	return e.fee.ActualMaker
}
func (e *Executor) ActualTakerFee() decimal.Decimal {
	return e.fee.ActualTaker
}

func (e *Executor) SubscribeCandle(clientId string, period time.Duration, responseHandler func(interface{})) {

}

func (e *Executor) Start() {
	e.ex.SubscribeOrder(e.Symbol(), "rtm-order", e.OrderUpdateHandler)
}

func (e *Executor) Stop() {
	e.ex.UnsubscribeOrder(e.Symbol(), "rtm-order")
}

func (e *Executor) OrderUpdateHandler(response interface{}) {
	subOrderResponse, ok := response.(order.SubscribeOrderV2Response)
	if !ok {
		e.Sugar.Warnf("Received unknown response: %v", response)
	}
	//log.Printf("subOrderResponse = %#v", subOrderResponse)
	if subOrderResponse.Action == "sub" {
		if subOrderResponse.IsSuccess() {
			e.Sugar.Infof("Subscribe topic %s successfully", subOrderResponse.Ch)
		} else {
			e.Sugar.Fatalf("Subscribe topic %s error, code: %d, message: %s",
				subOrderResponse.Ch, subOrderResponse.Code, subOrderResponse.Message)
		}
	} else if subOrderResponse.Action == "push" {
		if subOrderResponse.Data == nil {
			e.Sugar.Infof("SubscribeOrderV2Response has no data: %#v", subOrderResponse)
			return
		}
		o := subOrderResponse.Data
		if o.ClientOrderId == "" {
			e.Sugar.Debugf("no clientOrderId, not my order %d", o.OrderId)
			return
		}
		o2 := Order{
			Id:            uint64(o.OrderId),
			ClientOrderId: o.ClientOrderId,
			Type:          o.Type,
		}
		switch o.EventType {
		case "creation":
			o2.Price = o.OrderPrice
			o2.Amount = o.OrderSize
			e.Sugar.Debugf("order created, orderId: %d, clientOrderId: %s", o.OrderId, o.ClientOrderId)
			_ = e.CreateOrder(context.Background(), o2)
		case "cancellation":
			e.Sugar.Debugf("order cancelled, orderId: %d, clientOrderId: %s", o.OrderId, o.ClientOrderId)
			if !strings.HasPrefix(o.ClientOrderId, prefixBuyLimitOrder) {
				return
			}
			o2.Remain = o.RemainAmt
			remain, err := decimal.NewFromString(o2.Remain)
			if err != nil || !remain.IsPositive() {
				return
			}
			// need refund the quota
			// need a interface like order.GetPrice(), load from database
			if err := e.GetOrder(context.Background(), &o2); err == nil {
				e.Sugar.Debugf("cancel order %d, price %s, remain amount %s", o2.Price, o2.Remain)
				price, err2 := decimal.NewFromString(o2.Price)
				if err2 != nil || !price.IsPositive() {
					return
				}
				e.quota.Add(price.Mul(remain))
			}
		case "trade":
			e.Sugar.Debugf("order filled, orderId: %d, clientOrderId: %s, fill type: %s",
				o.OrderId, o.ClientOrderId, o.OrderStatus)
			if !strings.HasPrefix(o.ClientOrderId, prefixSellLimitOrder) && !strings.HasPrefix(o.ClientOrderId, prefixSellMarketOrder) {
				return
			}
			td := Trade{
				Id:     uint64(o.TradeId),
				Price:  o.TradePrice,
				Amount: o.TradeVolume,
				Remain: o.RemainAmt,
				Time:   time.Unix(o.TradeTime/1000, o.TradeTime%1000),
			}
			total := decimal.Zero
			if p, err1 := decimal.NewFromString(o.TradePrice); err1 == nil {
				if a, err2 := decimal.NewFromString(o.TradeVolume); err2 == nil {
					total = p.Mul(a)
					td.Total = p.Mul(a).String()
				}
			}
			e.quota.Add(total)
			//if err := t.fillOrder(context.Background(), o2, td); err != nil {
			//	t.Sugar.Error(err)
			//}
			//t.Broadcast("订单成交(%s), 订单号: %d / %s, 价格: %s, 数量: %s, 交易额: %s",
			//	o2.Status, o2.Id, o2.ClientOrderId, td.Price, td.Amount, td.Total)
			//t.orderFilled(o2, td)
		case "deletion":
			e.Sugar.Debugf("order deleted, orderId: %d, clientOrderId: %s, fill type: %s",
				o.OrderId, o.ClientOrderId, o.OrderStatus)
			//go t.deleteOrder(context.Background(), o2)
		default:
			e.Sugar.Warnf("unknown eventType, should never happen, orderId: %d, clientOrderId: %s, eventType: %s",
				o.OrderId, o.ClientOrderId, o.EventType)
		}
	}
}

func (e *Executor) BuyAllLimit(price float64) error {
	realPrice := decimal.NewFromFloat(price).Round(e.PricePrecision())
	// 1. check if have available balance
	balance, err := e.ex.SpotAvailableBalance()
	if err != nil {
		return err
	}
	total := balance[e.QuoteCurrency()]
	if total.GreaterThan(e.maxTotal) {
		total = e.maxTotal
	}
	quota := e.quota.Get()
	if quota.IsZero() {
		e.Sugar.Debug("no quota")
		return nil
	}
	if total.GreaterThan(quota) {
		total = quota
	}
	amount := total.DivRound(realPrice, e.AmountPrecision())
	total = amount.Mul(realPrice)
	if amount.LessThan(e.MinAmount()) {
		e.Sugar.Infof("amount (%s / %s) is too small", amount, e.MinAmount())
		return nil
	}
	clientId, err := e.id.GetClientOrderId(context.Background(), prefixBuyLimitOrder)
	if err != nil {
		//e.Sugar.Errorf("get client order id error: %s", err)
		return err
	}
	orderId, err := e.ex.BuyLimit(e.Symbol(), clientId, realPrice, amount)
	if err != nil {
		//e.Sugar.Errorf("buy error: %s", err)
		return err
	}
	e.SetBuyOrderId(orderId)
	e.Sugar.Infof("place buy-limit order, id %s / %s, price %s, amount %s", orderId, clientId, price, amount)
	if err := e.id.LongAdd(context.Background()); err != nil {
		e.Sugar.Errorf("update buy times error: %s", err)
		// this error can ignore
		//return err
	}
	e.quota.Sub(total)
	return nil
}

func (e *Executor) SellAllLimit(price float64) error {
	realPrice := decimal.NewFromFloat(price).Round(e.PricePrecision())
	balance, err := e.ex.SpotAvailableBalance()
	if err != nil {
		return err
	}
	amount := balance[e.BaseCurrency()]
	if amount.LessThan(e.MinAmount()) {
		e.Sugar.Infof("amount ( %s / %s ) is too small", amount, e.MinAmount())
		return nil
	}
	clientId, err := e.id.GetClientOrderId(context.Background(), prefixSellLimitOrder)
	if err != nil {
		//e.Sugar.Errorf("get client order id error: %s", err)
		return err
	}
	orderId, err := e.ex.SellLimit(e.Symbol(), clientId, realPrice, amount)
	if err != nil {
		//e.Sugar.Errorf("sell error: %s", err)
		return err
	}
	e.SetSellOrderId(orderId)
	if err := e.id.ShortAdd(context.Background()); err != nil {
		e.Sugar.Errorf("update sell times error: %s", err)
		// this error can ignore
		//return err
	}
	return nil
}

func (e *Executor) BuyAllMarket() error {
	// 1. check if have available balance
	balance, err := e.ex.SpotAvailableBalance()
	if err != nil {
		return err
	}
	total := balance[e.QuoteCurrency()]
	if total.GreaterThan(e.maxTotal) {
		total = e.maxTotal
	}
	quota := e.quota.Get()
	if quota.IsZero() {
		e.Sugar.Debug("no quota")
		return nil
	}
	if total.GreaterThan(quota) {
		total = quota
	}
	if total.LessThan(e.MinTotal()) {
		e.Sugar.Infof("buy-market total (%s / %s) is too small", total, e.MinTotal())
		return nil
	}
	clientId, err := e.id.GetClientOrderId(context.Background(), prefixBuyMarketOrder)
	if err != nil {
		//e.Sugar.Errorf("get client order id error: %s", err)
		return err
	}
	orderId, err := e.ex.BuyMarket(e.Symbol(), clientId, total)
	if err != nil {
		//e.Sugar.Errorf("buy error: %s", err)
		return err
	}
	e.SetBuyOrderId(orderId)
	e.Sugar.Infof("place buy-market order, id %s / %s, total %s", orderId, clientId, total)
	if err := e.id.LongAdd(context.Background()); err != nil {
		e.Sugar.Errorf("update buy times error: %s", err)
		// this error can ignore
		//return err
	}
	e.quota.Sub(total)
	return nil
}

func (e *Executor) SellAllMarket() error {
	balance, err := e.ex.SpotAvailableBalance()
	if err != nil {
		return err
	}
	amount := balance[e.BaseCurrency()]
	if amount.LessThan(e.MinAmount()) {
		e.Sugar.Infof("amount ( %s / %s ) is too small", amount, e.MinAmount())
		return nil
	}
	clientId, err := e.id.GetClientOrderId(context.Background(), prefixSellMarketOrder)
	if err != nil {
		//e.Sugar.Errorf("get client order id error: %s", err)
		return err
	}
	orderId, err := e.ex.SellMarket(e.Symbol(), clientId, amount)
	if err != nil {
		//e.Sugar.Errorf("sell error: %s", err)
		return err
	}
	e.SetSellOrderId(orderId)
	if err := e.id.ShortAdd(context.Background()); err != nil {
		e.Sugar.Errorf("update sell times error: %s", err)
		// this error can ignore
		//return err
	}
	return nil
}

func (e *Executor) CancelAll() error {
	if err := e.CancelAllBuy(); err != nil {
		return err
	}
	if err := e.CancelAllSell(); err != nil {
		return err
	}
	return nil
}

func (e *Executor) CancelAllBuy() error {
	orderId := e.GetBuyOrderId()
	if orderId != 0 {
		if err := e.ex.CancelOrder(e.Symbol(), orderId); err != nil {
			e.Sugar.Errorf("cancel order %d error: %s", orderId, err)
			return err
		} else {
			e.Sugar.Debugf("cancelled order %d", orderId)
			e.SetBuyOrderId(0)
			return nil
		}
	}
	return nil
}

func (e *Executor) CancelAllSell() error {
	orderId := e.GetSellOrderId()
	if orderId != 0 {
		if err := e.ex.CancelOrder(e.Symbol(), orderId); err != nil {
			e.Sugar.Errorf("cancel order %d error: %s", orderId, err)
			return err
		} else {
			e.Sugar.Debugf("cancelled order %d", orderId)
			e.SetSellOrderId(0)
			return nil
		}
	}
	return nil
}

func (e *Executor) GetBuyOrderId() uint64 {
	e.buyOrderLock.RLock()
	defer e.buyOrderLock.RUnlock()
	return e.buyOrderId
}

func (e *Executor) SetBuyOrderId(newId uint64) {
	e.buyOrderLock.Lock()
	defer e.buyOrderLock.Unlock()

	e.buyOrderId = newId
	_ = hs.SaveKey(context.Background(), e.db.Collection(collNameState), "buyOrderId", e.buyOrderId)
}

func (e *Executor) LoadBuyOrderId(ctx context.Context) error {
	e.buyOrderLock.Lock()
	defer e.buyOrderLock.Unlock()

	return hs.LoadKey(ctx, e.db.Collection(collNameState), "buyOrderId", &e.buyOrderId)
}

func (e *Executor) GetSellOrderId() uint64 {
	e.sellOrderLock.RLock()
	defer e.sellOrderLock.RUnlock()
	return e.sellOrderId
}

func (e *Executor) SetSellOrderId(newId uint64) {
	e.sellOrderLock.Lock()
	defer e.sellOrderLock.Unlock()

	e.sellOrderId = newId
	_ = hs.SaveKey(context.Background(), e.db.Collection(collNameState), "sellOrderId", e.sellOrderId)
}

func (e *Executor) LoadSellOrderId(ctx context.Context) error {
	e.sellOrderLock.Lock()
	defer e.sellOrderLock.Unlock()

	return hs.LoadKey(ctx, e.db.Collection(collNameState), "sellOrderId", &e.sellOrderId)
}
