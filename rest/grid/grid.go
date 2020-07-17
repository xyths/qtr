package grid

import (
	"context"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/hs/logger"
	"github.com/xyths/qtr/exchange"
	"github.com/xyths/qtr/gateio"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"math"
	"time"
)

type Config struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Strategy hs.RestGridStrategyConf
	Robots   []hs.BroadcastConf
}

type RestGridTrader struct {
	config Config

	db     *mongo.Database
	ex     *gateio.GateIO
	robots []broadcast.Broadcaster

	symbol          string
	baseCurrency    string
	quoteCurrency   string
	pricePrecision  int32
	amountPrecision int32
	minAmount       decimal.Decimal
	minTotal        decimal.Decimal

	scale  decimal.Decimal
	grids  []hs.Grid
	base   int
	cost   decimal.Decimal // average price
	amount decimal.Decimal // amount held
}

func New(configFilename string) *RestGridTrader {
	cfg := Config{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		log.Fatal(err)
	}
	return &RestGridTrader{
		config: cfg,
	}
}

func (r *RestGridTrader) Init(ctx context.Context) {
	db, err := hs.ConnectMongo(ctx, r.config.Mongo)
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	r.db = db
	r.initEx(ctx)
	r.initGrids(ctx)
	r.initRobots(ctx)
}

func (r *RestGridTrader) initEx(ctx context.Context) {
	r.ex = gateio.New(r.config.Exchange.Key, r.config.Exchange.Secret, r.config.Exchange.Host)
	switch r.config.Exchange.Symbols[0] {
	case "btc3l_usdt":
		r.symbol = gateio.BTC3L_USDT
		r.baseCurrency = gateio.BTC3L
		r.quoteCurrency = gateio.USDT
	case "btc_usdt":
		r.symbol = gateio.BTC_USDT
		r.baseCurrency = gateio.BTC
		r.quoteCurrency = gateio.USDT
	case "sero_usdt":
		r.symbol = gateio.SERO_USDT
		r.baseCurrency = gateio.SERO
		r.quoteCurrency = gateio.USDT
	default:
		r.symbol = "btc_usdt"
	}
	r.pricePrecision = int32(gateio.PricePrecision[r.symbol])
	r.amountPrecision = int32(gateio.AmountPrecision[r.symbol])
	r.minAmount = decimal.NewFromFloat(gateio.MinAmount[r.symbol])
	r.minTotal = decimal.NewFromInt(gateio.MinTotal[r.symbol])
	//Sugar.Debugf("init ex, pricePrecision = %d, amountPrecision = %d, minAmount = %s, minTotal = %s",
	//	r.pricePrecision, r.amountPrecision, r.minAmount.String(), r.minTotal.String())
}

func (r *RestGridTrader) initGrids(ctx context.Context) {
	maxPrice := r.config.Strategy.MaxPrice
	minPrice := r.config.Strategy.MinPrice
	number := r.config.Strategy.Number
	total := r.config.Strategy.Total
	//log.Debugf("init grids, MaxPrice: %f, MinPrice: %f, Grid Number: %d, total: %f",
	//	maxPrice, minPrice, number, total)
	r.scale = decimal.NewFromFloat(math.Pow(minPrice/maxPrice, 1.0/float64(number)))
	preTotal := decimal.NewFromFloat(total / float64(number))
	currentPrice := decimal.NewFromFloat(maxPrice)
	currentGrid := hs.Grid{
		Id:    0,
		Price: currentPrice.Round(r.pricePrecision),
	}
	r.grids = append(r.grids, currentGrid)
	for i := 1; i <= number; i++ {
		currentPrice = currentPrice.Mul(r.scale).Round(r.pricePrecision)
		amountBuy := preTotal.Div(currentPrice).Round(r.amountPrecision)
		if amountBuy.LessThan(r.minAmount) {
			log.Fatalf("amount %s less than minAmount(%s)", amountBuy, r.minAmount)
		}
		realTotal := currentPrice.Mul(amountBuy)
		if realTotal.LessThan(r.minTotal) {
			log.Fatalf("total %s less than minTotal(%s)", realTotal, r.minTotal)
		}
		currentGrid = hs.Grid{
			Id:        i,
			Price:     currentPrice,
			AmountBuy: amountBuy,
			TotalBuy:  realTotal,
		}
		r.grids = append(r.grids, currentGrid)
		r.grids[i-1].AmountSell = amountBuy
	}
}
func (r *RestGridTrader) initRobots(ctx context.Context) {
	for _, conf := range r.config.Robots {
		r.robots = append(r.robots, broadcast.New(conf))
	}
}

func (r *RestGridTrader) Print(ctx context.Context) error {
	delta, _ := r.scale.Float64()
	delta = 1 - delta
	logger.Sugar.Infof("Scale is %s (%1.2f%%)", r.scale.String(), 100*delta)
	logger.Sugar.Infof("Id\tTotal\tPrice\tAmountBuy\tAmountSell")
	for _, g := range r.grids {
		logger.Sugar.Infof("%2d\t%s\t%s\t%s\t%s", g.Id,
			g.TotalBuy.StringFixed(r.amountPrecision+r.pricePrecision), g.Price.StringFixed(r.pricePrecision),
			g.AmountBuy.StringFixed(r.amountPrecision), g.AmountSell.StringFixed(r.amountPrecision))
	}

	return nil
}

func (r *RestGridTrader) Close(ctx context.Context) {
	if r.db != nil {
		_ = r.db.Client().Disconnect(ctx)
	}
}

const (
	collNameGrid = "grid"
	collNameBase = "base"
)

func (r *RestGridTrader) saveGrids(ctx context.Context) {
	collGrid := r.db.Collection(collNameGrid)
	for _, g := range r.grids {
		if _, err := collGrid.InsertOne(ctx, bson.D{
			{"id", g.Id},
			{"price", g.Price.String()},
			{"amountBuy", g.AmountBuy.String()},
			{"amountSell", g.AmountSell.String()},
			{"totalBuy", g.TotalBuy.String()},
			{"order", g.Order},
		}); err != nil {
			logger.Sugar.Fatalf("error when save Grids: %s", err)
		}
	}
	collBase := r.db.Collection(collNameBase)
	if _, err := collBase.InsertOne(ctx, bson.D{
		{"symbol", r.symbol},
		{"base", r.base},
	}); err != nil {
		logger.Sugar.Fatalf("error when save short base: %s", err)
	}
}

func (r *RestGridTrader) loadGrids(ctx context.Context) bool {
	collBase := r.db.Collection(collNameBase)
	var base struct {
		Symbol string
		Base   int
	}
	if err := collBase.FindOne(ctx, bson.D{{"symbol", r.symbol}}).Decode(&base); err == mongo.ErrNoDocuments {
		return false
	}
	r.base = base.Base

	collGrid := r.db.Collection(collNameGrid)
	cursor, err := collGrid.Find(ctx, bson.D{})
	var items []struct {
		Id                                     int
		Price, AmountBuy, AmountSell, TotalBuy string
		Order                                  uint64
	}

	if err = cursor.All(ctx, &items); err != nil {
		return false
	}

	for _, item := range items {
		if item.Id < 0 || item.Id >= len(r.grids) {
			logger.Sugar.Fatalw("loaded grid index out of range",
				"id", item.Id,
				"grids", len(r.grids),
				"price", item.Price,
				"order", item.Order,
			)
			continue
		}
		if item.Order != 0 {
			r.grids[item.Id].Order = item.Order
			logger.Sugar.Infow("grid loaded",
				"id", item.Id,
				"price", item.Price,
				"order", item.Order,
			)
		} else {
			logger.Sugar.Infow("base grid ignored",
				"id", item.Id,
				"price", item.Price,
				"order", item.Order,
			)
		}
	}

	return true
}

func (r *RestGridTrader) Start(ctx context.Context) error {
	_ = r.Print(ctx)

	if !r.loadGrids(ctx) {
		logger.Sugar.Info("no order loaded")
		// rebalance
		if r.config.Strategy.Rebalance {
			if err := r.ReBalance(ctx, false); err != nil {
				log.Fatalf("error when rebalance: %s", err)
			}
		}
		// setup all grid orders
		r.setupGridOrders(ctx)
		r.saveGrids(ctx)
	}

	interval, err := time.ParseDuration(r.config.Strategy.Interval)
	if err != nil {
		logger.Sugar.Fatalf("error interval format: %s", r.config.Strategy.Interval)
	}
	r.checkOrders(ctx)
	for {
		select {
		case <-ctx.Done():
			logger.Sugar.Info("context cancelled")
			return nil
		case <-time.After(interval):
			r.checkOrders(ctx)
		}
	}
}

func (r *RestGridTrader) ReBalance(ctx context.Context, dryRun bool) error {
	price, err := r.last()
	if err != nil {
		logger.Sugar.Fatalf("get ticker error: %s", err)
	}
	r.base = 0
	moneyNeed := decimal.NewFromInt(0)
	coinNeed := decimal.NewFromInt(0)
	for i, g := range r.grids {
		if g.Price.GreaterThan(price) {
			r.base = i
			coinNeed = coinNeed.Add(g.AmountBuy)
		} else {
			moneyNeed = moneyNeed.Add(g.TotalBuy)
		}
	}
	logger.Sugar.Infof("now base = %d, moneyNeed = %s, coinNeed = %s", r.base, moneyNeed, coinNeed)
	balance, err := r.ex.AvailableBalance()
	if err != nil {
		logger.Sugar.Fatalf("error when get balance in rebalance: %s", err)
	}
	moneyHeld := balance[r.quoteCurrency]
	coinHeld := balance[r.baseCurrency]
	logger.Sugar.Infof("account has money %s, coin %s", moneyHeld, coinHeld)
	if dryRun {
		return nil
	}
	r.cost = price
	r.amount = coinNeed
	direct, amount := r.assetRebalancing(moneyNeed, coinNeed, moneyHeld, coinHeld, price)
	if direct == -2 || direct == 2 {
		log.Fatalf("no enough money for rebalance, direct: %d", direct)
	} else if direct == 0 {
		logger.Sugar.Info("no need to rebalance")
	} else if direct == -1 {
		// place sell order
		r.base++
		clientOrderId := fmt.Sprintf("pre-sell")
		orderId, err := r.sell(price, amount, clientOrderId)
		if err != nil {
			log.Fatalf("error when rebalance: %s", err)
		}
		logger.Sugar.Debugf("rebalance: sell %s coin at price %s, orderId is %d, clientOrderId is %s",
			amount, price, orderId, clientOrderId)
	} else if direct == 1 {
		// place buy order
		clientOrderId := fmt.Sprintf("pre-buy")
		orderId, err := r.buy(price, amount, clientOrderId)
		if err != nil {
			log.Fatalf("error when rebalance: %s", err)
		}
		logger.Sugar.Debugf("rebalance: buy %s coin at price %s, orderId is %d, clientOrderId is %s",
			amount, price, orderId, clientOrderId)
	}

	return nil
}

func (r *RestGridTrader) Clear(ctx context.Context, dryRun bool) error {
	if !r.loadGrids(ctx) {
		logger.Sugar.Info("no order loaded, no need to clear")
		return nil
	}
	collGrid := r.db.Collection(collNameGrid)
	for _, g := range r.grids {
		if g.Order == 0 {
			logger.Sugar.Debugw("order id is 0", "grid", g.Id, "price", g.Price)
		} else {
			logger.Sugar.Debugw("cancel order", "symbol", r.symbol, "orderNumber", g.Order)
			if ok, err := r.ex.CancelOrder(r.symbol, g.Order); err != nil {
				logger.Sugar.Errorf("cancel order %d error: %s", g.Order, err)
				continue
			} else if !ok {
				logger.Sugar.Errorf("cancel order %d response not ok", g.Order)
				continue
			}
			g.Order = 0
		}
		if _, err := collGrid.DeleteOne(ctx, bson.D{{"id", g.Id}}); err != nil {
			logger.Sugar.Fatalf("error when delete grid %d: %s", g.Order, err)
		}
	}
	if r.base != 0 {
		collBase := r.db.Collection(collNameBase)
		if _, err := collBase.DeleteOne(ctx, bson.D{
			{"symbol", r.symbol},
			{"base", r.base},
		}); err != nil {
			logger.Sugar.Fatalf("error when delete base: %s", err)
		}
	}
	return nil
}

func (r *RestGridTrader) setupGridOrders(ctx context.Context) {
	for i := r.base - 1; i >= 0; i-- {
		// sell
		clientOrderId := fmt.Sprintf("s-%d", i)
		orderId, err := r.sell(r.grids[i].Price, r.grids[i].AmountSell, clientOrderId)
		if err != nil {
			logger.Sugar.Errorf("error when setupGridOrders, grid number: %d, err: %s", i, err)
			continue
		}
		r.grids[i].Order = orderId
	}
	for i := r.base + 1; i < len(r.grids); i++ {
		// buy
		clientOrderId := fmt.Sprintf("b-%d", i)
		orderId, err := r.buy(r.grids[i].Price, r.grids[i].AmountBuy, clientOrderId)
		if err != nil {
			logger.Sugar.Errorf("error when setupGridOrders, grid number: %d, err: %s", i, err)
			continue
		}
		r.grids[i].Order = orderId
	}
}

// 0: no need
// 1: buy
// -1: sell
// 2: no enough money
// -2: no enough coin
func (r *RestGridTrader) assetRebalancing(moneyNeed, coinNeed, moneyHeld, coinHeld, price decimal.Decimal) (direct int, amount decimal.Decimal) {
	if moneyNeed.GreaterThan(moneyHeld) {
		// sell coin
		moneyDelta := moneyNeed.Sub(moneyHeld)
		sellAmount := moneyDelta.Div(price).Round(r.amountPrecision)
		if coinHeld.LessThan(coinNeed.Add(sellAmount)) {
			logger.Sugar.Errorf("no enough coin for rebalance: need hold %s and sell %s (%s in total), only have %s",
				coinNeed, sellAmount, coinNeed.Add(sellAmount), coinHeld)
			direct = -2
			return
		}

		if sellAmount.LessThan(r.minAmount) {
			logger.Sugar.Errorf("sell amount %s less than minAmount(%s), won't sell", sellAmount, r.minAmount)
			direct = 0
			return
		}
		if r.minTotal.GreaterThan(price.Mul(sellAmount)) {
			logger.Sugar.Infof("sell total %s less than minTotal(%s), won't sell", price.Mul(sellAmount), r.minTotal)
			direct = 0
			return
		}
		direct = -1
		amount = sellAmount
	} else {
		// buy coin
		if coinNeed.LessThanOrEqual(coinHeld) {
			logger.Sugar.Infof("no need to rebalance: need coin %s, has %s, need money %s, has %s",
				coinNeed, coinHeld, moneyNeed, moneyHeld)
			direct = 0
			return
		}
		coinDelta := coinNeed.Sub(coinHeld).Round(r.amountPrecision)
		buyTotal := coinDelta.Mul(price)
		if moneyHeld.LessThan(moneyNeed.Add(buyTotal)) {
			log.Fatalf("no enough money for rebalance: need hold %s and spend %s (%s in total)，only have %s",
				moneyNeed, buyTotal, moneyNeed.Add(buyTotal), moneyHeld)
			direct = 2
		}
		if coinDelta.LessThan(r.minAmount) {
			logger.Sugar.Errorf("buy amount %s less than minAmount(%s), won't sell", coinDelta, r.minAmount)
			direct = 0
			return
		}
		if buyTotal.LessThan(r.minTotal) {
			logger.Sugar.Errorf("buy total %s less than minTotal(%s), won't sell", buyTotal, r.minTotal)
			direct = 0
			return
		}
		direct = 1
		amount = coinDelta
	}
	return
}

func (r *RestGridTrader) up(ctx context.Context) {
	// make sure base >= 0
	if r.base == 0 {
		logger.Sugar.Infof("grid base = 0, up OUT")
		return
	}
	if r.base > len(r.grids)-1 {
		logger.Sugar.Errorw("wrong base when up", "base", r.base)
		return
	}
	// place buy order
	clientOrderId := fmt.Sprintf("b-%d", r.base)
	if orderId, err := r.buy(r.grids[r.base].Price, r.grids[r.base].AmountBuy, clientOrderId); err == nil {
		r.grids[r.base].Order = orderId
		if err := r.updateOrder(ctx, r.base, r.grids[r.base].Order); err != nil {
			logger.Sugar.Errorf("update order error: %s", err)
		}
	} else {
		logger.Sugar.Errorf("place order error: %s", err)
		return
	}
	r.base--
	if err := r.updateBase(ctx, r.base); err != nil {
		logger.Sugar.Errorf("update order error: %s", err)
	}

	r.grids[r.base].Order = 0
	if err := r.updateOrder(ctx, r.base, r.grids[r.base].Order); err != nil {
		logger.Sugar.Errorf("update order error: %s", err)
	}
}

func (r *RestGridTrader) down(ctx context.Context) {
	// make sure base <= len(grids)
	if r.base == len(r.grids) {
		logger.Sugar.Infof("grid base = %d, down OUT", r.base)
		return
	}
	if r.base < 0 {
		logger.Sugar.Errorw("wrong base when up", "base", r.base)
		return
	}
	// place sell order
	clientOrderId := fmt.Sprintf("s-%d", r.base)
	if orderId, err := r.sell(r.grids[r.base].Price, r.grids[r.base].AmountSell, clientOrderId); err == nil {
		r.grids[r.base].Order = orderId
		if err := r.updateOrder(ctx, r.base, r.grids[r.base].Order); err != nil {
			logger.Sugar.Errorf("update order error: %s", err)
		}
	}
	r.base++
	if err := r.updateBase(ctx, r.base); err != nil {
		logger.Sugar.Errorf("update order error: %s", err)
	}
	r.grids[r.base].Order = 0
	if err := r.updateOrder(ctx, r.base, r.grids[r.base].Order); err != nil {
		logger.Sugar.Errorf("update order error: %s", err)
	}
}

func (r *RestGridTrader) buy(price, amount decimal.Decimal, clientOrderId string) (uint64, error) {
	logger.Sugar.Infof("[Order][buy] price: %s, amount: %s, clientOrderId: %s", price, amount, clientOrderId)
	return r.ex.Buy(r.symbol, price, amount, gateio.OrderTypeNormal, clientOrderId)
}

func (r *RestGridTrader) sell(price, amount decimal.Decimal, clientOrderId string) (uint64, error) {
	logger.Sugar.Infof("[Order][sell] price: %s, amount: %s, clientOrderId: %s", price, amount, clientOrderId)
	return r.ex.Sell(r.symbol, price, amount, gateio.OrderTypeNormal, clientOrderId)
}

// 最后成交价格
func (r *RestGridTrader) last() (decimal.Decimal, error) {
	if ticker, err := r.ex.Ticker(r.symbol); err != nil {
		return decimal.Zero, err
	} else {
		return ticker.Last, err
	}
}

func (r *RestGridTrader) checkOrders(ctx context.Context) {
	top := r.base - 1
	if top >= 0 {
		logger.Sugar.Debugw("check order",
			"index", top,
			"symbol", r.symbol,
			"type", "sell",
			"order", r.grids[top].Order)
		if r.grids[top].Order != 0 {
			if order, closed := r.ex.IsOrderClose(r.symbol, r.grids[top].Order); closed {
				go r.up(ctx)
				go r.Broadcast(ctx, order)
				return
			}
		}
	}

	bottom := r.base + 1
	if bottom < len(r.grids) {
		logger.Sugar.Debugw("check order",
			"index", bottom,
			"symbol", r.symbol,
			"type", "buy",
			"order", r.grids[bottom].Order)
		if r.grids[bottom].Order != 0 {
			if order, closed := r.ex.IsOrderClose(r.symbol, r.grids[bottom].Order); closed {
				go r.down(ctx)
				go r.Broadcast(ctx, order)
			}
		}
	}
}

func (r *RestGridTrader) updateBase(ctx context.Context, newBase int) error {
	coll := r.db.Collection(collNameBase)
	_, err := coll.UpdateOne(
		ctx,
		bson.D{
			{"symbol", r.symbol},
		},
		bson.D{
			{"$set", bson.D{
				{"base", newBase},
			}},
			{"$currentDate", bson.D{
				{"lastModified", true},
			}},
		},
	)
	return err
}

func (r *RestGridTrader) updateOrder(ctx context.Context, id int, order uint64) error {
	collGrid := r.db.Collection(collNameGrid)
	_, err := collGrid.UpdateOne(
		ctx,
		bson.D{
			{"id", id},
		},
		bson.D{
			{"$set", bson.D{
				{"order", order},
			}},
			{"$currentDate", bson.D{
				{"lastModified", true},
			}},
		},
	)
	return err
}

func (r *RestGridTrader) Broadcast(ctx context.Context, order exchange.Order) {
	for _, robot := range r.robots {
		robot.Broadcast([]string{r.config.Exchange.Label}, order.CurrencyPair, order.Type, order.Rate.String(), order.Amount.String(), order.Rate.Mul(order.Amount).String(), "-")
	}
}
