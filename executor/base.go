package executor

import (
	"context"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/hs/exchange"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"strings"
	"time"
)

type State int

const (
	Empty   State = 0
	Open          = 1
	Buying        = 3
	Selling       = 4
)

type BaseExecutor struct {
	Name     string
	Label    string
	maxTotal decimal.Decimal
	Sugar    *zap.SugaredLogger
	db       *mongo.Database
	robots   []broadcast.Broadcaster
	ex       exchange.RestAPIExchange
	symbol   exchange.Symbol
	fee      exchange.Fee
	id       ClientIdManager

	quota Quota
	OrderProxy
}

func (e *BaseExecutor) Init(
	ex exchange.RestAPIExchange, sugar *zap.SugaredLogger, db *mongo.Database,
	exName, exLabel string, symbol exchange.Symbol, fee exchange.Fee, maxTotal decimal.Decimal,
	robots []broadcast.Broadcaster) {
	e.Name = exName
	e.Label = exLabel
	e.ex = ex
	e.Sugar = sugar
	e.db = db
	e.symbol = symbol
	e.fee = fee
	e.maxTotal = maxTotal
	e.robots = robots
	e.id.Init("-", db.Collection(collNameState))
	e.OrderProxy.Init(db.Collection(collNameOrder))
	e.quota.Init(db.Collection(collNameState), e.maxTotal)
}

func (e *BaseExecutor) Exchange() exchange.RestAPIExchange {
	return e.ex
}
func (e *BaseExecutor) Symbol() string {
	return e.symbol.Symbol
}
func (e *BaseExecutor) QuoteCurrency() string {
	return e.symbol.QuoteCurrency
}
func (e *BaseExecutor) BaseCurrency() string {
	return e.symbol.BaseCurrency
}
func (e *BaseExecutor) PricePrecision() int32 {
	return e.symbol.PricePrecision
}
func (e *BaseExecutor) AmountPrecision() int32 {
	return e.symbol.AmountPrecision
}
func (e *BaseExecutor) MinAmount() decimal.Decimal {
	return e.symbol.MinAmount
}
func (e *BaseExecutor) MinTotal() decimal.Decimal {
	return e.symbol.MinTotal
}
func (e *BaseExecutor) BaseMakerFee() decimal.Decimal {
	return e.fee.BaseMaker
}
func (e *BaseExecutor) BaseTakerFee() decimal.Decimal {
	return e.fee.BaseTaker
}
func (e *BaseExecutor) ActualMakerFee() decimal.Decimal {
	return e.fee.ActualMaker
}
func (e *BaseExecutor) ActualTakerFee() decimal.Decimal {
	return e.fee.ActualTaker
}

func (e *BaseExecutor) Load(ctx context.Context) error {
	if err := e.quota.Load(ctx); err != nil {
		return err
	}
	return nil
}

func (e *BaseExecutor) buyAllLimit(price decimal.Decimal) (orderId uint64, err error) {
	// 1. check if have available balance
	balance, err := e.ex.SpotAvailableBalance()
	if err != nil {
		return
	}
	total := balance[e.QuoteCurrency()]
	if total.GreaterThan(e.maxTotal) {
		total = e.maxTotal
	}
	quota := e.quota.Get()
	if quota.IsZero() {
		e.Sugar.Debug("no quota")
		return
	}
	if total.GreaterThan(quota) {
		total = quota
	}
	amount := total.DivRound(price, e.AmountPrecision())
	total = amount.Mul(price)
	if amount.LessThan(e.MinAmount()) {
		e.Sugar.Infof("amount (%s / %s) is too small", amount, e.MinAmount())
		return
	}
	clientId, err := e.id.GetClientOrderId(context.Background(), prefixBuyLimitOrder)
	if err != nil {
		//e.Sugar.Errorf("get client order id error: %s", err)
		return
	}
	orderId, err = e.ex.BuyLimit(e.Symbol(), clientId, price, amount)
	if err != nil {
		//e.Sugar.Errorf("buy error: %s", err)
		return
	}
	//e.SetBuyOrderId(orderId)
	e.Sugar.Infof("place buy-limit order, id %s / %s, price %s, amount %s", orderId, clientId, price, amount)
	if err1 := e.id.LongAdd(context.Background()); err1 != nil {
		e.Sugar.Errorf("update buy times error: %s", err1)
		// this error can ignore
		//return err
	}
	e.quota.Sub(total)
	return
}

func (e *BaseExecutor) sellAllLimit(price decimal.Decimal) (orderId uint64, err error) {
	balance, err := e.ex.SpotAvailableBalance()
	if err != nil {
		return
	}
	amount := balance[e.BaseCurrency()]
	if amount.LessThan(e.MinAmount()) {
		e.Sugar.Infof("amount ( %s / %s ) is too small", amount, e.MinAmount())
		return
	}
	clientId, err := e.id.GetClientOrderId(context.Background(), prefixSellLimitOrder)
	if err != nil {
		//e.Sugar.Errorf("get client order id error: %s", err)
		return
	}
	orderId, err = e.ex.SellLimit(e.Symbol(), clientId, price, amount)
	if err != nil {
		//e.Sugar.Errorf("sell error: %s", err)
		return
	}
	//e.SetSellOrderId(orderId)
	e.Sugar.Infof("place sell-limit order, id %s / %s, price %s, amount %s", orderId, clientId, price, amount)
	if err1 := e.id.ShortAdd(context.Background()); err1 != nil {
		e.Sugar.Errorf("update sell times error: %s", err1)
		// this error can ignore
		//return err
	}
	return
}

func (e *BaseExecutor) buyAllMarket() (orderId uint64, err error) {
	// 1. check if have available balance
	balance, err := e.ex.SpotAvailableBalance()
	if err != nil {
		return
	}
	total := balance[e.QuoteCurrency()]
	if total.GreaterThan(e.maxTotal) {
		total = e.maxTotal
	}
	quota := e.quota.Get()
	if quota.IsZero() {
		e.Sugar.Debug("no quota")
		return
	}
	if total.GreaterThan(quota) {
		total = quota
	}
	if total.LessThan(e.MinTotal()) {
		e.Sugar.Infof("buy-market total (%s / %s) is too small", total, e.MinTotal())
		return
	}
	clientId, err := e.id.GetClientOrderId(context.Background(), prefixBuyMarketOrder)
	if err != nil {
		//e.Sugar.Errorf("get client order id error: %s", err)
		return
	}
	e.Sugar.Infof("buy all (market), clientOrderId is %s", clientId)
	orderId, err = e.ex.BuyMarket(e.symbol, clientId, total)
	if err != nil {
		//e.Sugar.Errorf("buy error: %s", err)
		return
	}
	e.Sugar.Infof("place buy-market order, id %d / %s, total %s", orderId, clientId, total)
	if err1 := e.id.LongAdd(context.Background()); err1 != nil {
		e.Sugar.Errorf("update buy times error: %s", err1)
		// this error can ignore
		//return err
	}
	e.quota.Sub(total)
	return
}

func (e *RestExecutor) sellAllMarket() (orderId uint64, err error) {
	balance, err := e.ex.SpotAvailableBalance()
	if err != nil {
		return
	}
	amount := balance[e.BaseCurrency()]
	amount = amount.Round(e.AmountPrecision())
	if amount.LessThan(e.MinAmount()) {
		e.Sugar.Infof("amount ( %s / %s ) is too small", amount, e.MinAmount())
		return
	}
	clientId, err := e.id.GetClientOrderId(context.Background(), prefixSellMarketOrder)
	if err != nil {
		return
	}
	e.Sugar.Infof("sell all (market), clientOrderId is %s", clientId)
	orderId, err = e.ex.SellMarket(e.symbol, clientId, amount)
	if err != nil {
		return
	}
	//e.SetSellOrderId(orderId)
	if err := e.id.ShortAdd(context.Background()); err != nil {
		e.Sugar.Errorf("update sell times error: %s", err)
		// this error can ignore
		//return err
	}
	return
}

func (e *RestExecutor) Broadcast(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	labels := []string{e.Name, e.Label}
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	timeStr := time.Now().In(beijing).Format(layout)

	msg := fmt.Sprintf("%s [%s] [%s] %s", timeStr, strings.Join(labels, "] ["), e.Symbol(), message)
	for _, robot := range e.robots {
		if err := robot.SendText(msg); err != nil {
			e.Sugar.Infof("broadcast error: %s", err)
		}
	}
}
