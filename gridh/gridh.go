package gridh

import (
	"context"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	. "github.com/xyths/hs/log"
	"github.com/xyths/hs/mongohelper"
	"github.com/xyths/qtr/gateio"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"sync"
	"time"
)

type StrategyConf struct {
	Number    int     // number of grid
	Total     float64 // total fund
	Spread    float64 // 0.01 = 1%
	Rebalance bool
	Interval  string // sleep interval
}

type Config struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Strategy StrategyConf
}

// A grid strategy with hedging using REST API
type Trader struct {
	config Config

	db *mongo.Database
	ex *gateio.GateIO

	longSymbol      string
	shortSymbol     string
	quoteCurrency   string
	pricePrecision  int32
	amountPrecision int32
	minAmount       decimal.Decimal
	minTotal        decimal.Decimal

	scale      decimal.Decimal
	longGrids  []hs.Grid
	longBase   int
	shortGrids []hs.Grid
	shortBase  int
	needSetup  bool
	asset      Asset
}

func New(configFilename string) *Trader {
	cfg := Config{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		Sugar.Fatal(err)
	}
	return &Trader{
		config: cfg,
	}
}

func (t *Trader) Init(ctx context.Context) {
	db, err := mongohelper.ConnectMongo(ctx, t.config.Mongo)
	if err != nil {
		Sugar.Fatal(err)
	}
	t.db = db
	t.scale = decimal.NewFromFloat(1 - t.config.Strategy.Spread)
	t.initEx(ctx)
	t.initGrids(ctx)
}

func (t *Trader) Close(ctx context.Context) {
	if t.db != nil {
		_ = t.db.Client().Disconnect(ctx)
	}
}

func (t *Trader) Trade(ctx context.Context) error {
	if t.needSetup {
		t.setupGrids(ctx)
		t.saveGrids(ctx)
		t.needSetup = false
	}
	_ = t.Print(ctx)
	interval, err := time.ParseDuration(t.config.Strategy.Interval)
	if err != nil {
		Sugar.Fatalf("error interval format: %s", t.config.Strategy.Interval)
	}
	t.checkOrders(ctx)
	for {
		select {
		case <-ctx.Done():
			Sugar.Info("context cancelled")
			return nil
		case <-time.After(interval):
			t.checkOrders(ctx)
		}
	}
}

func (t *Trader) Print(ctx context.Context) error {
	delta, _ := t.scale.Float64()
	delta = 1 - delta
	Sugar.Infof("Scale is %s (%1.2f%%)", t.scale.String(), 100*delta)
	Sugar.Infow("Long Grid", "base", t.longBase)
	Sugar.Infof("Id\tTotal\tPrice\tAmountBuy\tAmountSell\tOrder")
	for _, g := range t.longGrids {
		Sugar.Infof("%2d\t%s\t%s\t%s\t%s\t%d", g.Id, g.TotalBuy, g.Price, g.AmountBuy, g.AmountSell, g.Order)
	}
	Sugar.Infow("Short Grid", "base", t.shortBase)
	Sugar.Infof("Id\tTotal\tPrice\tAmountBuy\tAmountSell\tOrder")
	for _, g := range t.shortGrids {
		Sugar.Infof("%2d\t%s\t%s\t%s\t%s\t%d", g.Id, g.TotalBuy, g.Price, g.AmountBuy, g.AmountSell, g.Order)
	}

	return nil
}

func (t *Trader) initEx(ctx context.Context) {
	t.ex = gateio.New(t.config.Exchange.Key, t.config.Exchange.Secret, t.config.Exchange.Host)
	if t.config.Exchange.Symbols == "btc3l_usdt|btc3s_usdt" {
		t.longSymbol = gateio.BTC3L_USDT
		t.shortSymbol = gateio.BTC3S_USDT
		t.quoteCurrency = "USDT"
	}
	t.pricePrecision = int32(gateio.PricePrecision[t.longSymbol])
	t.amountPrecision = int32(gateio.AmountPrecision[t.longSymbol])
	t.minAmount = decimal.NewFromFloat(gateio.MinAmount[t.longSymbol])
	t.minTotal = decimal.NewFromInt(gateio.MinTotal[t.longSymbol])
}

func (t *Trader) initGrids(ctx context.Context) {
	if t.loadGrids(ctx) {
		return
	}
	if ticker, err := t.ex.Ticker(t.longSymbol); err == nil {
		Sugar.Infof("%s last price: %f", t.longSymbol, ticker.Last)
		t.longGrids, t.longBase = t.initOneGrids(decimal.NewFromFloat(ticker.Last))
	} else {
		Sugar.Fatalf("error when get ticker: %s", err)
	}
	if ticker, err := t.ex.Ticker(t.shortSymbol); err == nil {
		Sugar.Infof("%s last price: %f", t.shortSymbol, ticker.Last)
		t.shortGrids, t.shortBase = t.initOneGrids(decimal.NewFromFloat(ticker.Last))
	} else {
		Sugar.Fatalf("error when get ticker: %s", err)
	}
	t.needSetup = true
	if t.config.Strategy.Rebalance {
		t.rebalance(ctx)
	}
}

func (t *Trader) initOneGrids(price decimal.Decimal) (grids []hs.Grid, base int) {
	total := decimal.NewFromFloat(t.config.Strategy.Total / 2).Div(decimal.NewFromInt(int64(t.config.Strategy.Number)))
	for i := 0; i <= t.config.Strategy.Number; i++ {
		p1 := t.scale.Pow(decimal.NewFromInt(int64(i - (t.config.Strategy.Number+1)/2))).Mul(price).Round(t.pricePrecision)
		if p1.GreaterThan(price) {
			base++
		}
		amountBuy := total.Div(p1).Round(t.amountPrecision)
		if amountBuy.LessThan(t.minAmount) {
			Sugar.Fatalf("amount %s is less than minAmount %s", amountBuy, t.minAmount)
		}
		realTotal := p1.Mul(amountBuy)
		if realTotal.LessThan(t.minTotal) {
			Sugar.Fatalf("total %s is less than minTotal %s", realTotal, t.minTotal)
		}
		grid := hs.Grid{
			Id:    i,
			Price: p1,
		}
		if i > 0 {
			grids[i-1].AmountSell = amountBuy
			grid.AmountBuy = amountBuy
			grid.TotalBuy = realTotal
		}
		grids = append(grids, grid)
	}
	return
}

const (
	collNameLongGrid  = "longGrid"
	collNameShortGrid = "shortGrid"
	collNameBase      = "base"
)

func (t *Trader) saveGrids(ctx context.Context) {
	t.saveGridsOneSide(ctx, t.db.Collection(collNameLongGrid), t.longGrids)
	t.saveGridsOneSide(ctx, t.db.Collection(collNameShortGrid), t.shortGrids)
	collBase := t.db.Collection(collNameBase)
	if _, err := collBase.InsertOne(ctx, bson.D{
		{"symbol", t.longSymbol},
		{"base", t.longBase},
	}); err != nil {
		Sugar.Fatalf("error when save long base: %s", err)
	}
	if _, err := collBase.InsertOne(ctx, bson.D{
		{"symbol", t.shortSymbol},
		{"base", t.shortBase},
	}); err != nil {
		Sugar.Fatalf("error when save short base: %s", err)
	}
}

func (t *Trader) saveGridsOneSide(ctx context.Context, collection *mongo.Collection, grids []hs.Grid) {
	for _, g := range grids {
		if _, err := collection.InsertOne(ctx, bson.D{
			{"id", g.Id},
			{"price", g.Price.String()},
			{"amountBuy", g.AmountBuy.String()},
			{"amountSell", g.AmountSell.String()},
			{"totalBuy", g.TotalBuy.String()},
			{"order", g.Order},
		}); err != nil {
			Sugar.Fatalf("error when save Grids: %s", err)
		}
	}
}

func (t *Trader) loadGrids(ctx context.Context) bool {
	collBase := t.db.Collection(collNameBase)
	type Base struct {
		Symbol string
		Base   int
	}
	var longBase Base
	if err := collBase.FindOne(ctx, bson.D{{"symbol", t.longSymbol}}).Decode(&longBase); err == mongo.ErrNoDocuments {
		return false
	} else {
		t.longBase = longBase.Base
	}
	var shortBase Base
	_ = collBase.FindOne(ctx, bson.D{{"symbol", t.shortSymbol}}).Decode(&shortBase)
	t.shortBase = shortBase.Base

	t.longGrids, _ = t.loadGridsOneSide(ctx, t.db.Collection(collNameLongGrid))
	t.shortGrids, _ = t.loadGridsOneSide(ctx, t.db.Collection(collNameShortGrid))
	return true
}

func (t *Trader) loadGridsOneSide(ctx context.Context, collection *mongo.Collection) (grids []hs.Grid, err error) {
	cursor, err := collection.Find(ctx, bson.D{})
	type Item struct {
		Id                                     int
		Price, AmountBuy, AmountSell, TotalBuy string
		Order                                  uint64
	}
	var items []Item
	err = cursor.All(ctx, &items)
	if err != nil {
		return
	}
	for _, item := range items {
		grids = append(grids, hs.Grid{
			Id:         item.Id,
			Price:      decimal.RequireFromString(item.Price),
			AmountBuy:  decimal.RequireFromString(item.AmountBuy),
			AmountSell: decimal.RequireFromString(item.AmountSell),
			TotalBuy:   decimal.RequireFromString(item.TotalBuy),
			Order:      item.Order,
		})
	}
	return
}

func (t *Trader) rebalance(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)
	go t.rebalanceOneSide(ctx, t.longSymbol, t.longBase, t.longGrids, &wg)
	go t.rebalanceOneSide(ctx, t.shortSymbol, t.shortBase, t.shortGrids, &wg)
	wg.Wait()
}

func (t *Trader) rebalanceOneSide(ctx context.Context, symbol string, base int, grids []hs.Grid, wg *sync.WaitGroup) {
	defer wg.Done()
	amount := decimal.Zero
	for i := 1; i <= base; i++ {
		amount = amount.Add(grids[i].AmountBuy)
	}
	price := grids[base].Price
	Sugar.Infow("rebalance, try to buy currency", "symbol", symbol, "price", price, "amount", amount, "total", price.Mul(amount))
	filledAmount := decimal.Zero
	times := 0
	for filledAmount.LessThan(amount) {
		select {
		case <-ctx.Done():
			Sugar.Errorf("context cancelled when rebalance: %s", ctx.Err())
			return
		default:
			times++
			ticker, err := t.ex.Ticker(symbol)
			if err != nil {
				Sugar.Errorf("get ticker error: %s", err)
			}
			last := decimal.NewFromFloat(ticker.Last)
			placeAmount := amount.Sub(filledAmount)
			clientOrderId := fmt.Sprintf("p-%d", times)
			Sugar.Debugw("try place order",
				"symbol", symbol,
				"price", last,
				"amount", placeAmount,
				"clientOrderId", clientOrderId)
			resp, err := t.ex.BuyOrder(symbol, last, placeAmount, gateio.OrderTypeNormal, clientOrderId)
			Sugar.Debugw("place order", "response", resp)
			if resp.Result == "false" || resp.OrderNumber == 0 {
				Sugar.Errorf("place order error", "result", resp.Result, "order", resp.OrderNumber)
				continue
			}
			//filledAmount = filledAmount.Add(decimal.RequireFromString(resp.FilledAmount))
			leftAmount := decimal.RequireFromString(resp.LeftAmount)
			if leftAmount.IsZero() {
				return
			}
			orderId := resp.OrderNumber
			finished := false
			for !finished {
				select {
				case <-ctx.Done():
					Sugar.Errorf("context cancelled when check rebalance order: %s", ctx.Err())
					return
				case <-time.After(2 * time.Minute):
					// check order status
					order, err := t.ex.GetOrder(orderId, symbol)
					if err != nil {
						Sugar.Error(err)
						continue
					}
					Sugar.Debugw("check order status", "order", order)
					switch order.Status {
					case gateio.OrderStatusOpen:
						_, _ = t.ex.CancelOrder(symbol, orderId)
					case gateio.OrderStatusCancelled, gateio.OrderStatusClosed:
						filledAmount = filledAmount.Add(order.FilledAmount)
						Sugar.Debugw("order finished",
							"symbol", symbol,
							"orderId", orderId,
							"filledAmount", filledAmount)
						finished = true
					}
				}
			}
		}
	}
	Sugar.Infow("rebalance, bought currency",
		"symbol", symbol,
		"filledAmount", filledAmount)
}

func (t *Trader) setupGrids(ctx context.Context) {
	Sugar.Debug("sleep 10 seconds, wait for fund ready")
	time.Sleep(10 * time.Second)
	for i := 0; i < t.longBase; i++ {
		// sell long
		clientOrderId := fmt.Sprintf("l-s-%d", i)
		order, err := t.ex.Sell(t.longSymbol, t.longGrids[i].Price, t.longGrids[i].AmountSell, gateio.OrderTypeNormal, clientOrderId)
		if err == nil {
			t.longGrids[i].Order = order
		} else {
			Sugar.Fatalf("place long init sell order error: %s", err)
		}
	}
	for i := t.longBase + 1; i < len(t.longGrids); i++ {
		// buy long
		clientOrderId := fmt.Sprintf("l-b-%d", i)
		order, err := t.ex.Buy(t.longSymbol, t.longGrids[i].Price, t.longGrids[i].AmountBuy, gateio.OrderTypeNormal, clientOrderId)
		if err == nil {
			t.longGrids[i].Order = order
		} else {
			Sugar.Fatalf("place long init buy order error: %s", err)
		}
	}
	for i := 0; i < t.shortBase; i++ {
		// sell short
		clientOrderId := fmt.Sprintf("s-s-%d", i)
		order, err := t.ex.Sell(t.shortSymbol, t.shortGrids[i].Price, t.shortGrids[i].AmountSell, gateio.OrderTypeNormal, clientOrderId)
		if err == nil {
			t.shortGrids[i].Order = order
		} else {
			Sugar.Fatalf("place short init sell order error: %s", err)
		}
	}
	for i := t.shortBase + 1; i < len(t.shortGrids); i++ {
		// buy short
		clientOrderId := fmt.Sprintf("s-b-%d", i)
		order, err := t.ex.Buy(t.shortSymbol, t.shortGrids[i].Price, t.shortGrids[i].AmountBuy, gateio.OrderTypeNormal, clientOrderId)
		if err == nil {
			t.shortGrids[i].Order = order
		} else {
			Sugar.Fatalf("place short init buy order error: %s", err)
		}
	}
}

func (t *Trader) checkOrders(ctx context.Context) {
	Sugar.Debug("checkOrders now")
	t.checkOrdersOneSide(ctx, t.longSymbol, t.longBase, t.longGrids)
	t.checkOrdersOneSide(ctx, t.shortSymbol, t.shortBase, t.shortGrids)
	Sugar.Debug("checkOrders finish")
}

func (t *Trader) checkOrdersOneSide(ctx context.Context, symbol string, base int, grids []hs.Grid) {
	top := base - 1
	if top >= 0 {
		Sugar.Debugw("check order",
			"index", top,
			"symbol", symbol,
			"direct", "sell",
			"order", grids[top].Order)
		if grids[top].Order != 0 && t.ex.IsOrderClose(symbol, grids[top].Order) {
			go t.up(ctx, symbol)
			return
		}
	}

	bottom := base + 1
	if bottom < len(grids) {
		Sugar.Debugw("check order",
			"index", bottom,
			"symbol", symbol,
			"direct", "buy",
			"order", grids[bottom].Order)
		if grids[bottom].Order != 0 && t.ex.IsOrderClose(symbol, grids[bottom].Order) {
			go t.down(ctx, symbol)
		}
	}
}

func (t *Trader) up(ctx context.Context, symbol string) {
	switch symbol {
	case t.longSymbol:
		t.longUp(ctx)
	case t.shortSymbol:
		t.shortUp(ctx)
	default:
		Sugar.Error("unknown symbol, should never happen")
	}
}

func (t *Trader) longUp(ctx context.Context) {
	if t.longBase == 0 {
		Sugar.Errorf("%s break up", t.longSymbol)
		return
	}
	clientOrderId := fmt.Sprintf("l-b-%d", t.longBase)
	order, err := t.ex.Buy(t.longSymbol, t.longGrids[t.longBase].Price, t.longGrids[t.longBase].AmountBuy, gateio.OrderTypeNormal, clientOrderId)
	if err != nil {
		Sugar.Errorf("error when buy %s: %s", t.longSymbol, err)
	}
	t.longGrids[t.longBase].Order = order
	if err1 := t.updateOrder(ctx, t.longSymbol, t.longBase, t.longGrids[t.longBase].Order); err1 != nil {
		Sugar.Errorf("error when update order in db: %s", err)
	}
	t.longBase--
	if err1 := t.updateBase(ctx, t.longSymbol, t.longBase); err1 != nil {
		Sugar.Errorf("error when update base in db: %s", err)
	}
	t.longGrids[t.longBase].Order = 0
	if err1 := t.updateOrder(ctx, t.longSymbol, t.longBase, t.longGrids[t.longBase].Order); err1 != nil {
		Sugar.Errorf("error when update order in db: %s", err)
	}
	Sugar.Infow("long up",
		"symbol", t.longSymbol,
		"index", t.longBase,
		"price", t.longGrids[t.longBase].Price)
}

func (t *Trader) shortUp(ctx context.Context) {
	if t.shortBase == 0 {
		Sugar.Errorf("%s break up", t.shortSymbol)
		return
	}
	clientOrderId := fmt.Sprintf("s-b-%d", t.shortBase)
	if order, err := t.ex.Buy(t.shortSymbol, t.shortGrids[t.shortBase].Price, t.shortGrids[t.shortBase].AmountBuy, gateio.OrderTypeNormal, clientOrderId); err == nil {
		t.shortGrids[t.shortBase].Order = order
		if err1 := t.updateOrder(ctx, t.shortSymbol, t.shortBase, t.shortGrids[t.shortBase].Order); err1 != nil {
			Sugar.Errorf("error when update order in db: %s", err)
		}
		t.shortBase--
		if err1 := t.updateBase(ctx, t.shortSymbol, t.shortBase); err1 != nil {
			Sugar.Errorf("error when update base in db: %s", err)
		}
		t.shortGrids[t.shortBase].Order = 0
		if err1 := t.updateOrder(ctx, t.shortSymbol, t.shortBase, t.shortGrids[t.shortBase].Order); err1 != nil {
			Sugar.Errorf("error when update order in db: %s", err)
		}
		Sugar.Infow("short up",
			"symbol", t.longSymbol,
			"index", t.longBase,
			"price", t.longGrids[t.longBase].Price)
	} else {
		Sugar.Errorf("error when buy %s: %s", t.shortSymbol, err)
	}
}

func (t *Trader) down(ctx context.Context, symbol string) {
	switch symbol {
	case t.longSymbol:
		t.longDown(ctx)
	case t.shortSymbol:
		t.shortDown(ctx)
	default:
		Sugar.Error("unknown symbol, should never happen")
	}
}

func (t *Trader) longDown(ctx context.Context) {
	if t.longBase == len(t.longGrids)-1 {
		Sugar.Errorf("%s break down", t.longSymbol)
		return
	}
	clientOrderId := fmt.Sprintf("l-s-%d", t.longBase)
	if order, err := t.ex.Sell(t.longSymbol, t.longGrids[t.longBase].Price, t.longGrids[t.longBase].AmountSell, gateio.OrderTypeNormal, clientOrderId); err == nil {
		t.longGrids[t.longBase].Order = order
		if err1 := t.updateOrder(ctx, t.longSymbol, t.longBase, t.longGrids[t.longBase].Order); err1 != nil {
			Sugar.Errorf("error when update order in db: %s", err)
		}
		t.longBase++
		if err1 := t.updateBase(ctx, t.longSymbol, t.longBase); err1 != nil {
			Sugar.Errorf("error when update base in db: %s", err)
		}
		t.longGrids[t.longBase].Order = 0
		if err1 := t.updateOrder(ctx, t.longSymbol, t.longBase, t.longGrids[t.longBase].Order); err1 != nil {
			Sugar.Errorf("error when update order in db: %s", err)
		}
		Sugar.Infow("long down",
			"symbol", t.longSymbol,
			"index", t.longBase,
			"price", t.longGrids[t.longBase].Price)
	} else {
		Sugar.Errorf("error when sell %s: %s", t.longSymbol, err)
	}
}

func (t *Trader) shortDown(ctx context.Context) {
	if t.shortBase == len(t.shortGrids)-1 {
		Sugar.Errorf("%s break down", t.shortSymbol)
		return
	}
	clientOrderId := fmt.Sprintf("s-s-%d", t.shortBase)
	if order, err := t.ex.Sell(t.shortSymbol, t.shortGrids[t.shortBase].Price, t.shortGrids[t.shortBase].AmountSell, gateio.OrderTypeNormal, clientOrderId); err == nil {
		t.shortGrids[t.shortBase].Order = order
		if err1 := t.updateOrder(ctx, t.shortSymbol, t.shortBase, t.shortGrids[t.shortBase].Order); err1 != nil {
			Sugar.Errorf("error when update order in db: %s", err)
		}
		t.shortBase++
		if err1 := t.updateBase(ctx, t.shortSymbol, t.shortBase); err1 != nil {
			Sugar.Errorf("error when update base in db: %s", err)
		}
		t.shortGrids[t.shortBase].Order = 0
		if err1 := t.updateOrder(ctx, t.shortSymbol, t.shortBase, t.shortGrids[t.shortBase].Order); err1 != nil {
			Sugar.Errorf("error when update order in db: %s", err)
		}
		Sugar.Infow("short down",
			"symbol", t.shortSymbol,
			"index", t.shortBase,
			"price", t.shortGrids[t.shortBase].Price)
	} else {
		Sugar.Errorf("error when sell %s: %s", t.shortSymbol, err)
	}
}

func (t *Trader) updateBase(ctx context.Context, symbol string, newBase int) error {
	coll := t.db.Collection(collNameBase)
	_, err := coll.UpdateOne(
		ctx,
		bson.D{
			{"symbol", symbol},
		},
		bson.D{
			{"$set", bson.D{
				{"base", newBase},
			}},
		},
	)
	return err
}

func (t *Trader) updateOrder(ctx context.Context, symbol string, id int, order uint64) error {
	var coll *mongo.Collection
	switch symbol {
	case t.longSymbol:
		coll = t.db.Collection(collNameLongGrid)
	case t.shortSymbol:
		coll = t.db.Collection(collNameShortGrid)
	}
	_, err := coll.UpdateOne(
		ctx,
		bson.D{
			{"id", id},
		},
		bson.D{
			{"$set", bson.D{
				{"order", order},
			}},
		},
	)
	return err
}
