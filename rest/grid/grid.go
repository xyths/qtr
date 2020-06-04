package grid

import (
	"context"
	"fmt"
	"github.com/jinzhu/gorm"
	//_ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/mattn/go-sqlite3"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	. "github.com/xyths/hs/log"
	"github.com/xyths/qtr/gateio"
	"log"
	"math"
	"time"
)

type Config struct {
	Exchange hs.ExchangeConf
	SQLite   hs.SQLiteConf
	Strategy hs.RestGridStrategyConf
}

type RestGridTrader struct {
	config Config

	db *gorm.DB
	ex *gateio.GateIO

	symbol          string
	baseCurrency    string
	quoteCurrency   string
	pricePrecision  int32
	amountPrecision int32
	minAmount       decimal.Decimal
	minTotal        decimal.Decimal

	scale  decimal.Decimal
	grids  []Grid
	base   Base
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
	r.initSQLite(ctx)
	r.initEx(ctx)
	r.initGrids(ctx)
}

func (r *RestGridTrader) initSQLite(ctx context.Context) {
	db, err := gorm.Open("sqlite3", r.config.SQLite.Location)
	if err != nil {
		Sugar.Fatal(err)
	}
	r.db = db
	o := Order{}
	if !r.db.HasTable(&o) {
		r.db.CreateTable(&o)
	}
	b := Base{}
	if !r.db.HasTable(&b) {
		r.db.CreateTable(&b)
	}
}

func (r *RestGridTrader) initEx(ctx context.Context) {
	r.ex = gateio.New(r.config.Exchange.Key, r.config.Exchange.Secret, r.config.Exchange.Host)
	switch r.config.Exchange.Symbols {
	case "btc3l_usdt":
		r.symbol = gateio.BTC3L_USDT
		r.baseCurrency = gateio.BTC3L
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
	currentGrid := Grid{
		Param: Param{
			Id:    0,
			Price: currentPrice.Round(r.pricePrecision),
		},
		Order: Order{
			Grid: 0,
		},
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
		currentGrid = Grid{
			Param: Param{
				Id:        i,
				Price:     currentPrice,
				AmountBuy: amountBuy,
				TotalBuy:  realTotal,
			},
			Order: Order{
				Grid: i,
			},
		}
		r.grids = append(r.grids, currentGrid)
		r.grids[i-1].AmountSell = amountBuy
	}
}

func (r *RestGridTrader) Print(ctx context.Context) error {
	delta, _ := r.scale.Float64()
	delta = 1 - delta
	Sugar.Infof("Scale is %s (%1.2f%%)", r.scale.String(), 100*delta)
	Sugar.Infof("Id\tTotal\tPrice\tAmountBuy\tAmountSell")
	for _, g := range r.grids {
		Sugar.Infof("%2d\t%s\t%s\t%s\t%s", g.Id,
			g.TotalBuy.StringFixed(r.amountPrecision+r.pricePrecision), g.Price.StringFixed(r.pricePrecision),
			g.AmountBuy.StringFixed(r.amountPrecision), g.AmountSell.StringFixed(r.amountPrecision))
	}

	return nil
}

func (r *RestGridTrader) Close(ctx context.Context) {
	if r.db != nil {
		err := r.db.Close()
		if err != nil {
			Sugar.Errorf("close db error: %s", err)
		}
	}
}

func (r *RestGridTrader) saveGrids(ctx context.Context) {
	//if r.db.HasTable()
	for _, g := range r.grids {
		if err := r.db.Create(&g.Order).Error; err != nil {
			Sugar.Fatalf("error when save Grids: %s", err)
		}
	}
	if err := r.db.Create(&r.base); err != nil {
		Sugar.Fatalf("error when save base: %s", err)
	}
}

func (r *RestGridTrader) loadGrids(ctx context.Context) bool {
	if r.db.First(&r.base).RecordNotFound() {
		return false
	}

	var orders []Order
	if err := r.db.Find(&orders).Error; err != nil {
		Sugar.Fatalf("find orders error: %s", err)
	}
	for _, o := range orders {
		if o.Grid >= 0 && o.Grid < len(r.grids) {
			Sugar.Infow("load grid", "id", o.Grid, "order", o.OrderId)
			r.grids[o.Grid].OrderId = o.OrderId
		}
	}

	return true
}

func (r *RestGridTrader) Start(ctx context.Context) error {
	_ = r.Print(ctx)

	if !r.loadGrids(ctx) {
		Sugar.Info("no order loaded")
		// rebalance
		if r.config.Strategy.Rebalance {
			if err := r.ReBalance(ctx); err != nil {
				log.Fatalf("error when rebalance: %s", err)
			}
		}
		// setup all grid orders
		r.setupGridOrders(ctx)
		r.saveGrids(ctx)
	}

	interval, err := time.ParseDuration(r.config.Strategy.Interval)
	if err != nil {
		Sugar.Fatalf("error interval format: %s", r.config.Strategy.Interval)
	}
	r.checkOrders(ctx)
	for {
		select {
		case <-ctx.Done():
			Sugar.Info("context cancelled")
			return nil
		case <-time.After(interval):
			r.checkOrders(ctx)
		}
	}
}

func (r *RestGridTrader) ReBalance(ctx context.Context) error {
	price, err := r.last()
	if err != nil {
		Sugar.Fatalf("get ticker error: %s", err)
	}
	r.base.Grid = 0
	moneyNeed := decimal.NewFromInt(0)
	coinNeed := decimal.NewFromInt(0)
	for i, g := range r.grids {
		if g.Price.GreaterThan(price) {
			r.base.Grid = i
			coinNeed = coinNeed.Add(g.AmountBuy)
		} else {
			moneyNeed = moneyNeed.Add(g.TotalBuy)
		}
	}
	Sugar.Infof("now base = %d, moneyNeed = %s, coinNeed = %s", r.base, moneyNeed, coinNeed)
	balance, err := r.ex.Balances()
	if err != nil {
		Sugar.Fatalf("error when get balance in rebalance: %s", err)
	}
	moneyHeld := balance[r.quoteCurrency]
	coinHeld := balance[r.baseCurrency]
	Sugar.Infof("account has money %s, coin %s", moneyHeld, coinHeld)
	r.cost = price
	r.amount = coinNeed
	direct, amount := r.assetRebalancing(moneyNeed, coinNeed, moneyHeld, coinHeld, price)
	if direct == -2 || direct == 2 {
		log.Fatalf("no enough money for rebalance, direct: %d", direct)
	} else if direct == 0 {
		Sugar.Info("no need to rebalance")
	} else if direct == -1 {
		// place sell order
		r.base.Grid++
		clientOrderId := fmt.Sprintf("pre-sell")
		orderId, err := r.sell(price, amount, clientOrderId)
		if err != nil {
			log.Fatalf("error when rebalance: %s", err)
		}
		Sugar.Debugf("rebalance: sell %s coin at price %s, orderId is %d, clientOrderId is %s",
			amount, price, orderId, clientOrderId)
	} else if direct == 1 {
		// place buy order
		clientOrderId := fmt.Sprintf("pre-buy")
		orderId, err := r.buy(price, amount, clientOrderId)
		if err != nil {
			log.Fatalf("error when rebalance: %s", err)
		}
		Sugar.Debugf("rebalance: buy %s coin at price %s, orderId is %d, clientOrderId is %s",
			amount, price, orderId, clientOrderId)
	}

	return nil
}

func (r *RestGridTrader) setupGridOrders(ctx context.Context) {
	for i := r.base.Grid - 1; i >= 0; i-- {
		// sell
		clientOrderId := fmt.Sprintf("s-%d", i)
		orderId, err := r.sell(r.grids[i].Price, r.grids[i].AmountSell, clientOrderId)
		if err != nil {
			Sugar.Errorf("error when setupGridOrders, grid number: %d, err: %s", i, err)
			continue
		}
		r.grids[i].OrderId = orderId
	}
	for i := r.base.Grid + 1; i < len(r.grids); i++ {
		// buy
		clientOrderId := fmt.Sprintf("b-%d", i)
		orderId, err := r.buy(r.grids[i].Price, r.grids[i].AmountBuy, clientOrderId)
		if err != nil {
			Sugar.Errorf("error when setupGridOrders, grid number: %d, err: %s", i, err)
			continue
		}
		r.grids[i].OrderId = orderId
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
			Sugar.Errorf("no enough coin for rebalance: need hold %s and sell %s (%s in total), only have %s",
				coinNeed, sellAmount, coinNeed.Add(sellAmount), coinHeld)
			direct = -2
			return
		}

		if sellAmount.LessThan(r.minAmount) {
			Sugar.Errorf("sell amount %s less than minAmount(%s), won't sell", sellAmount, r.minAmount)
			direct = 0
			return
		}
		if r.minTotal.GreaterThan(price.Mul(sellAmount)) {
			Sugar.Infof("sell total %s less than minTotal(%s), won't sell", price.Mul(sellAmount), r.minTotal)
			direct = 0
			return
		}
		direct = -1
		amount = sellAmount
	} else {
		// buy coin
		if coinNeed.LessThanOrEqual(coinHeld) {
			Sugar.Infof("no need to rebalance: need coin %s, has %s, need money %s, has %s",
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
			Sugar.Errorf("buy amount %s less than minAmount(%s), won't sell", coinDelta, r.minAmount)
			direct = 0
			return
		}
		if buyTotal.LessThan(r.minTotal) {
			Sugar.Errorf("buy total %s less than minTotal(%s), won't sell", buyTotal, r.minTotal)
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
	if r.base.Grid == 0 {
		Sugar.Infof("grid base = 0, up OUT")
		return
	}
	if r.base.Grid > len(r.grids)-1 {
		Sugar.Errorw("wrong base when up", "base", r.base)
		return
	}
	// place buy order
	clientOrderId := fmt.Sprintf("b-%d", r.base)
	if orderId, err := r.buy(r.grids[r.base.Grid].Price, r.grids[r.base.Grid].AmountBuy, clientOrderId); err == nil {
		r.grids[r.base.Grid].OrderId = orderId
		if err := r.updateOrder(ctx, r.base.Grid, r.grids[r.base.Grid].OrderId); err != nil {
			Sugar.Errorf("update order error: %s", err)
		}
	} else {
		Sugar.Errorf("place order error: %s", err)
		return
	}
	r.base.Grid--
	if err := r.updateBase(ctx, r.base.Grid); err != nil {
		Sugar.Errorf("update order error: %s", err)
	}

	r.grids[r.base.Grid].OrderId = 0
	if err := r.updateOrder(ctx, r.base.Grid, r.grids[r.base.Grid].OrderId); err != nil {
		Sugar.Errorf("update order error: %s", err)
	}
}

func (r *RestGridTrader) down(ctx context.Context) {
	// make sure base <= len(grids)
	if r.base.Grid == len(r.grids) {
		Sugar.Infof("grid base = %d, down OUT", r.base)
		return
	}
	if r.base.Grid < 0 {
		Sugar.Errorw("wrong base when up", "base", r.base)
		return
	}
	// place sell order
	clientOrderId := fmt.Sprintf("s-%d", r.base)
	if orderId, err := r.sell(r.grids[r.base.Grid].Price, r.grids[r.base.Grid].AmountSell, clientOrderId); err == nil {
		r.grids[r.base.Grid].OrderId = orderId
		if err := r.updateOrder(ctx, r.base.Grid, r.grids[r.base.Grid].OrderId); err != nil {
			Sugar.Errorf("update order error: %s", err)
		}
	}
	r.base.Grid++
	if err := r.updateBase(ctx, r.base.Grid); err != nil {
		Sugar.Errorf("update order error: %s", err)
	}
	r.grids[r.base.Grid].OrderId = 0
	if err := r.updateOrder(ctx, r.base.Grid, r.grids[r.base.Grid].OrderId); err != nil {
		Sugar.Errorf("update order error: %s", err)
	}
}

func (r *RestGridTrader) buy(price, amount decimal.Decimal, clientOrderId string) (uint64, error) {
	Sugar.Infof("[Order][buy] price: %s, amount: %s, clientOrderId: %s", price, amount, clientOrderId)
	return r.ex.Buy(r.symbol, price, amount, gateio.OrderTypeNormal, clientOrderId)
}

func (r *RestGridTrader) sell(price, amount decimal.Decimal, clientOrderId string) (uint64, error) {
	Sugar.Infof("[Order][sell] price: %s, amount: %s, clientOrderId: %s", price, amount, clientOrderId)
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
	top := r.base.Grid - 1
	if top >= 0 {
		Sugar.Debugw("check order",
			"index", top,
			"symbol", r.symbol,
			"direct", "sell",
			"order", r.grids[top].Order)
		if r.grids[top].OrderId != 0 && r.ex.IsOrderClose(r.symbol, r.grids[top].OrderId) {
			go r.up(ctx)
			return
		}
	}

	bottom := r.base.Grid + 1
	if bottom < len(r.grids) {
		Sugar.Debugw("check order",
			"index", bottom,
			"symbol", r.symbol,
			"direct", "buy",
			"order", r.grids[bottom].Order)
		if r.grids[bottom].OrderId != 0 && r.ex.IsOrderClose(r.symbol, r.grids[bottom].OrderId) {
			go r.down(ctx)
		}
	}
}

func (r *RestGridTrader) updateBase(ctx context.Context, newBase int) error {
	return r.db.Model(&r.base).Update("grid", newBase).Error
}

func (r *RestGridTrader) updateOrder(ctx context.Context, id int, order uint64) error {
	o := Order{}
	return r.db.Model(&o).Where(map[string]interface{}{"grid": id}).Update("order", order).Error
}