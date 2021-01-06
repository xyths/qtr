package super

import (
	"context"
	"fmt"
	"github.com/shopspring/decimal"
	indicator "github.com/xyths/go-indicators"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/qtr/types"
	"log"
	"math"
	"time"
)

// RESTful trader
type RestTrader struct {
	BaseTrader

	//candle     hs.Candle
	uniqueId   int64
	position   int64 // 1 long (full), -1 short (clear)
	trend      int64 // 1: long, -1 short
	LongTimes  int64
	ShortTimes int64

	buyOrderId           uint64
	sellOrderId          uint64
	sellStopOrderId      uint64
	reinforceBuyOrderId  uint64
	reinforceSellOrderId uint64
}

func NewRestTraderFromConfig(ctx context.Context, cfg Config) (*RestTrader, error) {
	b, err := NewBaseTraderFromConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	s := &RestTrader{
		BaseTrader: *b,
	}
	//err = s.Init(ctx)
	return s, err
}

func (t *RestTrader) Init(ctx context.Context) error {
	if err := t.BaseTrader.Init(ctx); err != nil {
		return err
	}
	//t.candle = hs.NewCandle(2000)
	t.Sugar.Info("Rest SuperTrend Trader initialized")
	return nil
}

func (t *RestTrader) Close(ctx context.Context) error {
	if t.db != nil {
		_ = t.db.Client().Disconnect(ctx)
	}
	if t.Sugar != nil {
		t.Sugar.Info("Rest SuperTrend Trader stopped")
		return t.Sugar.Sync()
	}
	return nil
}

func (t *RestTrader) Print(ctx context.Context) error {
	t.loadState(ctx)
	log.Printf(`State
	Position: %d
	Unique Id: %d
	Long times: %d
	Short times: %d`,
		t.position,
		t.uniqueId,
		t.LongTimes,
		t.ShortTimes,
	)
	//log.Printf(`Sell-stop order
	//Id: %d / %t
	//Price: %t
	//Amount: %t
	//Create Time: %t`,
	//	t.sellStopOrder.Id, t.sellStopOrder.ClientOrderId,
	//	t.sellStopOrder.Price,
	//	t.sellStopOrder.Amount,
	//	t.sellStopOrder.Time,
	//)

	return nil
}

func (t *RestTrader) Clear(ctx context.Context) error {
	t.clearState(ctx)
	return nil
}

func (t *RestTrader) Start(ctx context.Context) {
	t.loadState(ctx)
	t.checkState(ctx)

	t.doWork(ctx)
	wakeTime := time.Now()
	if t.interval == time.Hour*24 {
		wakeTime = time.Date(wakeTime.Year(), wakeTime.Month(), wakeTime.Day(), 0, 0, 0, 0, wakeTime.Location())
		// gate以8点钟为日线开始
		if t.config.Exchange.Name == "gate" {
			wakeTime = wakeTime.Add(time.Hour * 8)
		}
	} else {
		wakeTime = wakeTime.Truncate(t.interval)
	}
	wakeTime = wakeTime.Add(t.interval)
	sleepTime := time.Until(wakeTime)
	t.Sugar.Debugf("next check time: %s", wakeTime.String())
	for {
		select {
		case <-ctx.Done():
			t.Sugar.Info(ctx.Err())
			return
		case <-time.After(sleepTime):
			t.doWork(ctx)
			wakeTime = wakeTime.Add(t.interval)
			sleepTime = time.Until(wakeTime)
			t.Sugar.Debugf("next check time: %s", wakeTime.String())
		}
	}
}

// doWork do real work.
// 1. check order status
// 2. check candle state
// 3. buy or sell (market price)
func (t *RestTrader) doWork(ctx context.Context) {
	candle, err := t.ex.CandleBySize(t.Symbol(), t.interval, 2000)
	if err != nil {
		return
	}
	t.onTick(candle, false)
}

func (t *RestTrader) onTick(c hs.Candle, dry bool) {
	tsl, trend := indicator.SuperTrend(t.Factor, t.Period, c.High, c.Low, c.Close)
	l := c.Length()
	if l < 3 {
		return
	}
	for i := l - 3; i < l-1; i++ {
		t.Sugar.Debugf(
			"[%d] %s %f %f %f %f, %f %v",
			i, types.TimestampToDate(c.Timestamp[i]),
			c.Open[i], c.High[i], c.Low[i], c.Close[i],
			tsl[i], trend[i],
		)
	}
	t.Sugar.Debugf("SuperTrend = [..., (%f, %v), (%f, %v)", tsl[l-3], trend[l-3], tsl[l-2], trend[l-2])
	price := decimal.NewFromFloat(math.Min(c.Close[l-1], c.Close[l-2]))
	stop := decimal.NewFromFloat(tsl[l-2]).Round(t.PricePrecision())

	_ = price
	if trend[l-2] {
		t.Sugar.Infof("in buy channel, stop price is %s, position is %d", stop, t.position)
	} else {
		t.Sugar.Infof("in sell channel, upper band price is %s, position is %d", stop, t.position)
	}

	if trend[l-2] && (!trend[l-3] || t.position != 1) {
		// false -> true, buy/long
		t.Sugar.Info("[Signal] BUY")
		t.trend = 1
		if !dry {
			t.Long(price, stop)
		}
	} else if !trend[l-2] && trend[l-3] {
		// true -> false, sell/short
		t.Sugar.Info("[Signal] SELL")
		t.trend = -1
		t.Sugar.Info("sell all at market price")
		if !dry {
			t.Short(price, stop)
		}
	} else if t.StopLoss && t.position == 1 {
		// update sell-stop-limit order
		//go t.updateSellStop(stop, false)
	}
}

// Long buy maxTotal amount coin at market price
func (t *RestTrader) Long(price, stop decimal.Decimal) {
	t.MarketBuyAll(price)
	if t.Reinforce > 0 {
		// place reinforce order
		p := decimal.NewFromInt(1).Sub(t.fee.ActualMaker.Mul(decimal.NewFromFloat(2 + t.Reinforce)))
		t.Sugar.Debugf("p is %s", p)
		//lower := price.Mul(p).Round(t.PricePrecision())
		//t.reinforceBuy(lower)
	}
}

func (t *RestTrader) MarketBuyAll(price decimal.Decimal) {
	balance, err := t.ex.SpotAvailableBalance()
	if err != nil {
		t.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	t.Sugar.Infof("balance: %v", balance)
	if t.position == 1 {
		t.Sugar.Infof("position full: %v", balance)
		return
	}
	maxTotal := balance[t.QuoteCurrency()]
	if maxTotal.GreaterThan(t.maxTotal) {
		maxTotal = t.maxTotal
	}
	clientId := GetClientOrderId(sep, prefixBuyLimitOrder, t.ShortTimes, t.LongTimes+1, t.GetUniqueId())
	switch t.config.Exchange.Name {
	case "gate":
		t.smoothBuy(t.symbol, clientId, maxTotal)
	default:
		total := maxTotal
		orderId, err := t.ex.BuyMarket(t.symbol, clientId, total)
		if err != nil {
			t.Sugar.Errorf("sell error: %s", err)
			return
		}
		t.Sugar.Infof("市价买入，订单号: %d / %s, total: %s", orderId, clientId, total)
		t.Broadcast("市价买入，订单号: %d / %s, 买入总金额: %s", orderId, clientId, total)
	}

	t.SetPosition(1)
	t.LongTimes++
	if err := hs.SaveInt64(context.Background(), t.db.Collection(collNameState), "longTimes", t.LongTimes); err != nil {
		t.Sugar.Infof("save longTimes error: %s", err)
	}
}

// 专为Gate优化的市价买入策略，因为Gate不支持市价单，所以从Ticker获得的最新价，不一定能成交，尤其是极端行情
// 解决办法是：下单后等待20s，再次下单（20s才能更新一次Ticker）
func (t *RestTrader) smoothBuy(symbol exchange.Symbol, clientId string, total decimal.Decimal) {
	left := total
	i := 0
	for left.IsPositive() {
		lastPrice, err := t.ex.LastPrice(symbol.Symbol)
		if err != nil {
			t.Sugar.Errorf("get last price error: %s", err)
			continue
		}
		price := lastPrice.Round(t.PricePrecision())
		amount := left.DivRound(price, t.AmountPrecision())
		orderId, err := t.ex.BuyLimit(t.Symbol(), fmt.Sprintf("%s-%d", clientId, i), price, amount)
		i++
		if err != nil {
			t.Sugar.Errorf("buy error: %s", err)
			return
		}

		t.Sugar.Infof("市价买入，订单号: %d / %s, total: %s", orderId, clientId, left)
		// check order
		time.Sleep(time.Second * 20)
		o2, err := t.ex.GetOrderById(orderId, t.Symbol())
		if o2.FilledAmount.IsPositive() {
			// 成交或部分成交
			t.Broadcast("市价买入，订单号: %d / %s\n\t下单价格: %s, 下单数量: %s\n\t成交价格: %s, 成交数量: %s\n\t下单总金额: %s, 成交总金额: %s",
				orderId, o2.ClientOrderId,
				o2.Price, o2.Amount,
				o2.FilledPrice, o2.FilledAmount,
				o2.Price.Mul(o2.Amount), o2.FilledPrice.Mul(o2.FilledAmount),
			)
		}
		if o2.FilledAmount.Equal(o2.Amount) {
			t.Sugar.Info("buy order full-filled")
			break
		}
		if err := t.ex.CancelOrder(t.Symbol(), orderId); err != nil {
			t.Sugar.Errorf("cancel order error: %s", err)
			continue
		}
		left = left.Sub(o2.FilledPrice.Mul(o2.FilledAmount))
	}
}

// 专为Gate优化的市价买出策略，因为Gate不支持市价单，所以从Ticker获得的最新价，不一定能成交，尤其是极端行情
// 解决办法是：下单后等待20s，再次下单（20s才能更新一次Ticker）
func (t *RestTrader) smoothSell(symbol exchange.Symbol, clientId string, amount decimal.Decimal) {
	left := amount
	i := 0
	for left.IsPositive() {
		lastPrice, err := t.ex.LastPrice(symbol.Symbol)
		if err != nil {
			t.Sugar.Errorf("get last price error: %s", err)
			continue
		}
		price := lastPrice.Round(t.PricePrecision())
		text := fmt.Sprintf("%s-%d", clientId, i)
		orderId, err := t.ex.SellLimit(t.Symbol(), text, price, left)
		i++
		if err != nil {
			t.Sugar.Errorf("sell error: %s", err)
			return
		}

		t.Sugar.Infof("尝试市价清仓，订单号: %d / %s, amount: %s", orderId, text, left)
		// check order
		time.Sleep(time.Second * 20)
		o2, err := t.ex.GetOrderById(orderId, t.Symbol())
		if o2.FilledAmount.IsPositive() {
			// 成交或部分成交
			t.Broadcast("市价清仓成交，订单号: %d / %s\n 下单价格: %s, 下单数量: %s\n 成交价格: %s, 成交数量: %s\n 下单总金额: %s, 成交总金额: %s",
				orderId, o2.ClientOrderId,
				o2.Price, o2.FilledPrice,
				o2.Amount, o2.FilledAmount,
				o2.Price.Mul(o2.Amount), o2.FilledPrice.Mul(o2.FilledAmount),
			)
		}
		if o2.FilledAmount.Equal(o2.Amount) {
			t.Sugar.Info("sell order full-filled")
			break
		}
		if err := t.ex.CancelOrder(t.Symbol(), orderId); err != nil {
			t.Sugar.Errorf("cancel order error: %s", err)
			continue
		}
		left = left.Sub(o2.FilledAmount)
	}
}

func (t *RestTrader) reinforceBuy(price decimal.Decimal) {
	if t.reinforceBuyOrderId != 0 {
		if err := t.realCancelReinforce("reinforceBuyOrderId", &t.reinforceBuyOrderId); err != nil {
			t.Sugar.Errorf("cancel reinforce order error: %s", err)
			return
		}
	}

	balance, err := t.ex.SpotAvailableBalance()
	if err != nil {
		t.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	t.Sugar.Infof("balance: %v", balance)
	maxTotal := balance[t.QuoteCurrency()]
	if maxTotal.GreaterThan(t.maxTotal) {
		maxTotal = t.maxTotal
	}
	amount := maxTotal.DivRound(price, t.AmountPrecision())
	total := amount.Mul(price)
	if amount.LessThan(t.MinAmount()) {
		t.Sugar.Infof("amount (%s / %s) is too small", amount, t.MinAmount())
		return
	}
	clientId := GetClientOrderId(sep, prefixBuyReinforceOrder, t.ShortTimes, t.LongTimes+1, t.GetUniqueId())
	orderId, err := t.ex.BuyLimit(t.Symbol(), clientId, price, amount)
	if err != nil {
		t.Sugar.Errorf("buy error: %s", err)
		return
	}
	t.reinforceBuyOrderId = orderId
	coll := t.db.Collection(collNameState)
	if err := hs.SaveKey(context.Background(), coll, "reinforceBuyOrderId", t.reinforceBuyOrderId); err != nil {
		t.Sugar.Errorf("save reinforceOrder error: %s", err)
	}
	t.Sugar.Infof("限价买入，订单号: %d / %s, price: %s, amount: %s, total: %s", orderId, clientId, price, amount, total)
	t.Broadcast("限价买入，订单号: %d / %s, 价格: %s, 数量: %s, 买入总金额: %s", orderId, clientId, price, amount, total)
}

func (t *RestTrader) realCancelReinforce(key string, orderId *uint64) error {
	if *orderId == 0 {
		return nil
	}
	if err := t.ex.CancelOrder(t.Symbol(), *orderId); err != nil {
		return err
	}

	t.Sugar.Infof("cancelled %s %d", key, *orderId)
	*orderId = 0
	coll := t.db.Collection(collNameState)
	if err := hs.SaveKey(context.Background(), coll, key, *orderId); err != nil {
		return err
	}
	return nil
}

// Short sell all coins at market price
func (t *RestTrader) Short(price, stop decimal.Decimal) {
	if t.StopLoss {
		t.cancelSellStop()
	}
	if t.Reinforce > 0 {
		// cancel reinforce order
		t.cancelReinforce()
	}
	t.MarketSellAll()
}

func (t *RestTrader) cancelSellStop() {
	if t.sellStopOrderId != 0 {
		if err := t.ex.CancelOrder(t.Symbol(), t.sellStopOrderId); err != nil {
			t.Sugar.Errorf("cancel sell-stop-limit order error: %s", err)
		}
		t.Sugar.Infof("cancelled sell-stop-limit order %d", t.sellStopOrderId)
		t.SetSellStopOrder(t.sellStopOrderId)
	}
}

func (t *RestTrader) cancelReinforce() {
	if err := t.realCancelReinforce("reinforceBuyOrderId", &t.reinforceBuyOrderId); err != nil {
		t.Sugar.Errorf("cancel reinforce order error: %s", err)
	}
	if err := t.realCancelReinforce("reinforceSellOrderId", &t.reinforceSellOrderId); err != nil {
		t.Sugar.Errorf("cancel reinforce order error: %s", err)
	}
}

func (t *RestTrader) MarketSellAll() {
	balance, err := t.ex.SpotAvailableBalance()
	if err != nil {
		t.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	t.Sugar.Infof("balance: %v", balance)
	if t.position == -1 {
		t.Sugar.Infof("position already clear: %v", balance)
		return
	}
	// sell all balance
	amount := balance[t.BaseCurrency()].Round(t.AmountPrecision())
	if amount.GreaterThan(balance[t.BaseCurrency()]) {
		amount = amount.Sub(t.MinAmount())
	}
	//logger.Sugar.Debugf("try to sell %s, balance: %v, amount: %s, price: %s", symbol, balance, amount, price)
	if amount.LessThan(t.MinAmount()) {
		t.Sugar.Infof("amount too small: %s", amount)
		t.SetPosition(-1)
		return
	}
	clientId := GetClientOrderId(sep, prefixSellMarketOrder, t.ShortTimes+1, t.LongTimes, t.GetUniqueId())
	switch t.config.Exchange.Name {
	case "gate":
		t.smoothSell(t.symbol, clientId, amount)
	default:
		orderId, err := t.ex.SellMarket(t.symbol, clientId, amount)
		if err != nil {
			t.Sugar.Errorf("sell error: %s", err)
			return
		}
		t.Sugar.Infof("市价清仓，订单号: %d / %s, amount: %s", orderId, clientId, amount)
		t.Broadcast("市价清仓，订单号: %d / %s, 卖出数量: %s", orderId, clientId, amount)
	}

	t.SetPosition(-1)
	t.ShortTimes++
	if err := hs.SaveInt64(context.Background(), t.db.Collection(collNameState), "shortTimes", t.ShortTimes); err != nil {
		t.Sugar.Infof("save shortTimes error: %s", err)
	}
}

func (t *RestTrader) Stop() {
	t.Sugar.Info(" Rest SuperTrend Trader stopped.")
}

func (t *RestTrader) loadState(ctx context.Context) {
	coll := t.db.Collection(collNameState)
	if uniqueId, err := hs.LoadInt64(ctx, coll, "uniqueId"); err != nil {
		t.Sugar.Fatalf("load UniqueId error: %t", err)
	} else if uniqueId != 0 {
		t.uniqueId = uniqueId
		t.Sugar.Infof("loaded UniqueId: %d", uniqueId)
	}
	if position, err := hs.LoadInt64(ctx, coll, "position"); err != nil {
		t.Sugar.Fatalf("load position error: %t", err)
	} else if position != 0 {
		t.position = position
		t.Sugar.Infof("loaded position: %d", position)
	}
	if longTimes, err := hs.LoadInt64(ctx, coll, "longTimes"); err != nil {
		t.Sugar.Errorf("load longTimes error: %t", err)
	} else if longTimes != 0 {
		t.LongTimes = longTimes
		t.Sugar.Infof("loaded longTimes: %d", longTimes)
	}
	if shortTimes, err := hs.LoadInt64(ctx, coll, "shortTimes"); err != nil {
		t.Sugar.Errorf("load shortTimes error: %t", err)
	} else if shortTimes != 0 {
		t.ShortTimes = shortTimes
		t.Sugar.Infof("loaded shortTimes: %d", shortTimes)
	}
	//if t.StopLoss() {
	//	sellStopOrder := emptySellStopOrder
	//	if err := hs.LoadKey(ctx, coll, "sellStopOrder", &sellStopOrder); err != nil {
	//		t.Sugar.Fatalf("load sellStopOrder error: %t", err)
	//	} else if sellStopOrder.Id != 0 {
	//		t.sellStopOrder = sellStopOrder
	//		t.Sugar.Infof("loaded sellStopOrder: %v", sellStopOrder)
	//	}
	//}
	//if t.Reinforce() > 0 {
	//	buy := emptyReinforceBuyOrder
	//	if err := hs.LoadKey(ctx, coll, buy.Name, &buy); err != nil {
	//		t.Sugar.Errorf("load reinforce buy order error: %t", err)
	//	} else if buy.Id != 0 {
	//		t.reinforceBuyOrder = buy
	//		t.Sugar.Infof("loaded reinforceBuyOrder: %v", buy)
	//	}
	//	sell := emptyReinforceBuyOrder
	//	if err := hs.LoadKey(ctx, coll, sell.Name, &sell); err != nil {
	//		t.Sugar.Errorf("load reinforce sell order error: %t", err)
	//	} else if sell.Id != 0 {
	//		t.reinforceSellOrder = sell
	//		t.Sugar.Infof("loaded reinforceSellOrder: %v", sell)
	//	}
	//}
}

func (t *RestTrader) GetUniqueId() int64 {
	t.uniqueId = (t.uniqueId + 1) % 10000
	coll := t.db.Collection(collNameState)
	if err := hs.SaveInt64(context.Background(), coll, "uniqueId", t.uniqueId); err != nil {
		t.Sugar.Errorf("save uniqueId error: %t", err)
	}
	return t.uniqueId
}
func (t *RestTrader) SetPosition(newPosition int64) {
	t.position = newPosition
	coll := t.db.Collection(collNameState)
	if err := hs.SaveInt64(context.Background(), coll, "position", t.position); err != nil {
		t.Sugar.Errorf("save position error: %t", err)
	}
}
func (t *RestTrader) SetSellStopOrder(newOrderId uint64) {
	coll := t.db.Collection(collNameState)
	t.sellStopOrderId = newOrderId
	if err := hs.SaveKey(context.Background(), coll, "sellStopOrderId", t.sellStopOrderId); err != nil {
		t.Sugar.Errorf("save sellStopOrder error: %t", err)
	}
}

func (t *RestTrader) clearState(ctx context.Context) {
	coll := t.db.Collection(collNameState)
	if err := hs.DeleteInt64(ctx, coll, "longTimes"); err != nil {
		t.Sugar.Errorf("delete longTimes error: %t", err)
	} else {
		t.Sugar.Info("delete longTimes from database")
	}
	if err := hs.DeleteInt64(ctx, coll, "shortTimes"); err != nil {
		t.Sugar.Errorf("delete shortTimes error: %t", err)
	} else {
		t.Sugar.Info("delete shortTimes from database")
	}
	if err := hs.DeleteInt64(ctx, coll, "sellStopOrder"); err != nil {
		t.Sugar.Errorf("delete sellStopOrder error: %t", err)
	} else {
		t.Sugar.Info("delete sellStopOrder from database")
	}
}

func (t *RestTrader) checkState(ctx context.Context) {
	// check sell-stop order
	t.checkSellStopOrder(ctx)
}

func (t *RestTrader) checkSellStopOrder(ctx context.Context) {
	//if t.sellStopOrder.Id == 0 {
	//	t.Sugar.Info("no sell-stop order")
	//	return
	//}
	//o, err := t.ex.GetOrderById(uint64(t.sellStopOrder.Id), t.Symbol())
	//if err != nil {
	//	t.Sugar.Infof("get sell-stop order error: %t", err)
	//	return
	//}
	//if o.Status == "filled" || o.Status == "cancelled" {
	//	t.SetSellStopOrder(emptySellStopOrder)
	//} else {
	//	t.Sugar.Infof("sell-stop order id: %d / %t, price: %t, amount: %t, total: %t, status: %t, filled: %t",
	//		o.Id, o.ClientOrderId, o.Price, o.Amount, o.Price.Mul(o.Amount), o.Status, o.FilledAmount)
	//}
}

func (t *RestTrader) Symbol() string {
	return t.symbol.Symbol
}
func (t *RestTrader) QuoteCurrency() string {
	return t.symbol.QuoteCurrency
}
func (t *RestTrader) BaseCurrency() string {
	return t.symbol.BaseCurrency
}
func (t *RestTrader) PricePrecision() int32 {
	return t.symbol.PricePrecision
}
func (t *RestTrader) AmountPrecision() int32 {
	return t.symbol.AmountPrecision
}
func (t *RestTrader) MinAmount() decimal.Decimal {
	return t.symbol.MinAmount
}
func (t *RestTrader) MinTotal() decimal.Decimal {
	return t.symbol.MinTotal
}
func (t *RestTrader) BaseMakerFee() decimal.Decimal {
	return t.fee.BaseMaker
}
func (t *RestTrader) BaseTakerFee() decimal.Decimal {
	return t.fee.BaseTaker
}
func (t *RestTrader) ActualMakerFee() decimal.Decimal {
	return t.fee.ActualMaker
}
func (t *RestTrader) ActualTakerFee() decimal.Decimal {
	return t.fee.ActualTaker
}
