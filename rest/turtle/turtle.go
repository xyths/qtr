package turtle

import (
	"context"
	"fmt"
	"github.com/markcheno/go-talib"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/hs/logger"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/gateio"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

type Trader struct {
	config   Config
	interval time.Duration

	db     *mongo.Database
	ex     *gateio.GateIO
	robots []broadcast.Broadcaster

	symbol          string
	baseCurrency    string // coin, eg. BTC
	quoteCurrency   string // cash, eg. USDT
	pricePrecision  int32
	amountPrecision int32
	minAmount       decimal.Decimal
	minTotal        decimal.Decimal

	state   state
	balance map[string]decimal.Decimal
}

func New(configFilename string) *Trader {
	cfg := Config{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		log.Fatal(err)
	}
	interval, err := time.ParseDuration(cfg.Strategy.Interval)
	if err != nil {
		logger.Sugar.Fatalf("error interval format: %s", cfg.Strategy.Interval)
	}
	return &Trader{
		config:   cfg,
		interval: interval,
	}
}

func (t *Trader) Init(ctx context.Context) {
	db, err := hs.ConnectMongo(ctx, t.config.Mongo)
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	t.db = db
	t.initEx(ctx)
	t.initRobots(ctx)
}

func (t *Trader) initEx(ctx context.Context) {
	t.ex = gateio.New(t.config.Exchange.Key, t.config.Exchange.Secret, t.config.Exchange.Host)
	switch t.config.Exchange.Symbols[0] {
	case "btc3l_usdt":
		t.symbol = gateio.BTC3L_USDT
		t.baseCurrency = gateio.BTC3L
		t.quoteCurrency = gateio.USDT
	case "btc_usdt":
		t.symbol = gateio.BTC_USDT
		t.baseCurrency = gateio.BTC
		t.quoteCurrency = gateio.USDT
	case "sero_usdt":
		t.symbol = gateio.SERO_USDT
		t.baseCurrency = gateio.SERO
		t.quoteCurrency = gateio.USDT
	default:
		t.symbol = "btc_usdt"
	}
	t.pricePrecision = int32(gateio.PricePrecision[t.symbol])
	t.amountPrecision = int32(gateio.AmountPrecision[t.symbol])
	t.minAmount = decimal.NewFromFloat(gateio.MinAmount[t.symbol])
	t.minTotal = decimal.NewFromInt(gateio.MinTotal[t.symbol])
	//Sugar.Debugf("init ex, pricePrecision = %d, amountPrecision = %d, minAmount = %s, minTotal = %s",
	//	t.pricePrecision, t.amountPrecision, t.minAmount.String(), t.minTotal.String())
}

func (t *Trader) initRobots(ctx context.Context) {
	for _, conf := range t.config.Robots {
		t.robots = append(t.robots, broadcast.New(conf))
	}
}

func (t *Trader) Close(ctx context.Context) {
	if t.db != nil {
		_ = t.db.Client().Disconnect(ctx)
	}
}

func (t *Trader) Print(ctx context.Context) error {
	t.getPosition(ctx)
	return nil
}

func (t *Trader) Start(ctx context.Context) error {
	if !t.loadPosition(ctx) {
		t.getPosition(ctx)
		t.savePosition(ctx)
	}

	t.doWork(ctx)
	for {
		select {
		case <-ctx.Done():
			logger.Sugar.Info("Turtle trader stopped")
			return nil
		case <-time.After(t.interval):
			t.doWork(ctx)
		}
	}
	//return nil
}

func (t *Trader) doWork(ctx context.Context) {
	// max 300 data
	candle, err := t.ex.GetCandle(t.symbol, int(t.interval)/1000000000, int(300*t.interval/time.Hour)-1)
	if err != nil {
		logger.Sugar.Errorf("get candle error: %s", err)
	}
	//for i := 0; i < candle.Length(); i++ {
	//	logger.Sugar.Debugf("[%d] %d, %f, %f, %f, %f, %f",
	//		i, candle.Timestamp[i], candle.Open[i], candle.High[i],
	//		candle.Low[i], candle.Close[i], candle.Volume[i])
	//}
	//natrs := talib.Natr(candle.High, candle.Low, candle.Close, t.config.Strategy.PeriodATR)
	//logger.Sugar.Debugf("natrs: %v", natrs)
	atrs := talib.Atr(candle.High, candle.Low, candle.Close, t.config.Strategy.PeriodATR)
	//logger.Sugar.Debugf("atrs: %v", atrs)
	atr := atrs[len(atrs)-1]
	uppers := talib.Max(candle.High, t.config.Strategy.PeriodUpper)
	upper := uppers[len(uppers)-2]
	lowers := talib.Min(candle.Low, t.config.Strategy.PeriodLower)
	lower := lowers[len(lowers)-2]
	logger.Sugar.Debugf("atr: %f, upper: %f, lower: %f", atr, upper, lower)

	l := candle.Length()
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	datetime := time.Unix(candle.Timestamp[l-1]/1000, 0).In(beijing).Format(utils.TimeLayout)

	var signal bool
	if !signal && candle.Low[l-1] <= lower {
		logger.Sugar.Infof("突破下轨, Timestamp: %s, Lower: %f, Low: %f", datetime, lower, candle.Low[l-1])
		if t.state.Position > 0 {
			logger.Sugar.Info("目前是持仓，开始清仓")
			signal = true
			t.clearPosition(ctx)
		} else {
			logger.Sugar.Info("目前是空仓，无需操作")
		}
	}
	if !signal && t.state.LastBuyPrice > 0 && candle.Low[l-1]+2*atr <= t.state.LastBuyPrice {
		logger.Sugar.Infof("下跌超过2N, Timestamp: %s, N: %f, LastBuy: %f, Low: %f", datetime, atr, t.state.LastBuyPrice, candle.Low[l-1])
		if t.state.Position > 0 {
			logger.Sugar.Info("目前是持仓，开始清仓")
			signal = true
			t.clearPosition(ctx)
		} else {
			logger.Sugar.Info("目前是空仓，无需操作")
		}
	}
	if !signal && candle.High[l-1] >= upper {
		logger.Sugar.Infof("突破上轨, Timestamp: %s, Upper: %f, High: %f", datetime, upper, candle.High[l-1])
		if t.state.Position == 0 {
			logger.Sugar.Info("目前是空仓，开始入场")
			signal = true
			t.openPosition(ctx, atr)
		} else {
			logger.Sugar.Info("目前不是空仓，无需操作")
		}
	}
	if !signal && t.state.LastBuyPrice > 0 && candle.High[l-1] >= t.state.LastBuyPrice+0.5*atr {
		logger.Sugar.Infof("上涨超过0.5N, Timestamp: %s, N: %f, LastBuy: %f, High: %f", datetime, atr, t.state.LastBuyPrice, candle.High[l-1])
		if t.state.Position > 0 && t.state.BuyTimes < t.config.Strategy.MaxTimes {
			logger.Sugar.Info("目前是持仓，开始加仓")
			signal = true
			t.addPosition(ctx, atr)
		} else if t.state.Position == 0 {
			logger.Sugar.Info("目前是空仓，无需操作")
		} else {
			logger.Sugar.Info("目前是满仓，无需操作")
		}
	}
}

const collName = "position"

func (t *Trader) loadPosition(ctx context.Context) bool {
	coll := t.db.Collection(collName)
	if err := coll.FindOne(ctx, bson.D{}).Decode(&t.state); err == mongo.ErrNoDocuments {
		//logger.Sugar.Errorf("load Position error: %s",err)
		return false
	} else if err != nil {
		logger.Sugar.Fatalf("load state error: %s", err)
	}
	return true
}

func (t *Trader) savePosition(ctx context.Context) {
	coll := t.db.Collection(collName)
	//if _, err := coll.InsertOne(ctx, t.state); err != nil {
	//	logger.Sugar.Errorf("save state error: %s", err)
	//}
	option := &options.UpdateOptions{}
	option.SetUpsert(true)
	if _, err := coll.UpdateOne(ctx,
		bson.D{
			{"_id", 1},
		},
		bson.D{
			{"$set", bson.D{
				{"position", t.state.Position},
				{"buyTimes", t.state.BuyTimes},
				{"sellTimes", t.state.SellTimes},
				{"lastBuyPrice", t.state.LastBuyPrice},
				{"lastModified", t.state.LastModified},
			}},
		},
		option,
	); err != nil {
		logger.Sugar.Errorf("save state error: %s", err)
	}
}

func (t *Trader) getPosition(ctx context.Context) {
	balance, err := t.ex.AvailableBalance()
	if err != nil {
		logger.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	t.balance = balance
	logger.Sugar.Debugf("balance: %v", t.balance)

	if t.balance[t.quoteCurrency].LessThan(decimal.NewFromInt(1)) {
		// full
		t.state.Position = 3
	} else if balance[t.baseCurrency].GreaterThan(t.minAmount) {
		t.state.Position = 1
	}

	t.state.LastModified = time.Now()
}

func (t *Trader) openPosition(ctx context.Context, N float64) {
	balance, err := t.ex.AvailableBalance()
	if err != nil {
		logger.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	ob, err := t.ex.OrderBook(t.symbol)
	if err != nil {
		logger.Sugar.Errorf("get order book error: %s", err)
		return
	}
	s1, _ := gateio.Sell1(ob.Asks)
	price := decimal.NewFromFloat(s1)
	unit := decimal.NewFromFloat(t.config.Strategy.Total * 0.01 / N)
	amount := unit.Round(t.amountPrecision)
	total := price.Mul(amount)
	cash := balance[t.quoteCurrency]
	if cash.LessThan(decimal.NewFromInt(1)) {
		logger.Sugar.Infof("have no money, cash: %s", cash)
		return
	}
	if total.GreaterThanOrEqual(cash) {
		amount = cash.Div(price).Round(t.amountPrecision)
		total = price.Mul(amount)
	}
	if total.LessThan(t.minTotal) {
		total = t.minTotal
		amount = total.Div(price).Round(t.amountPrecision)
	}
	if amount.LessThan(t.minAmount) {
		amount = t.minAmount
		total = price.Mul(amount)
	}
	clientId := fmt.Sprintf("o-%d-%d", t.state.SellTimes, t.state.BuyTimes)

	logger.Sugar.Debugf("buy price %s amount %s total %s", price, amount, total)
	orderId, err := t.ex.Buy(t.symbol, price, amount, gateio.OrderTypeNormal, clientId)
	if err != nil {
		logger.Sugar.Errorf("buy error: %s", err)
		return
	}
	logger.Sugar.Infof("开仓，订单号: %d / %s", orderId, clientId)
	t.state.Position = 1
	t.state.BuyTimes++
	t.state.LastBuyPrice, _ = price.Float64()
	t.state.LastModified = time.Now()
	t.savePosition(ctx)
}

func (t *Trader) addPosition(ctx context.Context, N float64) {
	balance, err := t.ex.AvailableBalance()
	if err != nil {
		logger.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	ob, err := t.ex.OrderBook(t.symbol)
	if err != nil {
		logger.Sugar.Errorf("get order book error: %s", err)
		return
	}
	s1, _ := gateio.Sell1(ob.Asks)
	price := decimal.NewFromFloat(s1)
	unit := decimal.NewFromFloat(t.config.Strategy.Total * 0.01 / N)
	amount := unit.Round(t.amountPrecision)
	total := price.Mul(amount)
	cash := balance[t.quoteCurrency]
	if cash.LessThan(decimal.NewFromInt(1)) {
		logger.Sugar.Infof("have no money, cash: %s", cash)
		return
	}
	if total.GreaterThanOrEqual(cash) {
		amount = cash.Div(price).Round(t.amountPrecision)
		total = price.Mul(amount)
	}
	if total.LessThan(t.minTotal) {
		total = t.minTotal
		amount = total.Div(price).Round(t.amountPrecision)
	}
	if amount.LessThan(t.minAmount) {
		amount = t.minAmount
		total = price.Mul(amount)
	}
	clientId := fmt.Sprintf("a-%d-%d", t.state.SellTimes, t.state.BuyTimes)
	logger.Sugar.Debugf("buy price %s amount %s total %s", price, amount, total)

	orderId, err := t.ex.Buy(t.symbol, price, amount, gateio.OrderTypeNormal, clientId)
	if err != nil {
		logger.Sugar.Errorf("buy error: %s", err)
		return
	}
	logger.Sugar.Infof("加仓，订单号: %d / %s", orderId, clientId)

	t.state.BuyTimes++
	t.savePosition(ctx)
}

func (t *Trader) clearPosition(ctx context.Context) {
	balance, err := t.ex.AvailableBalance()
	if err != nil {
		logger.Sugar.Errorf("get available balance error: %s", err)
		return
	}
	ob, err := t.ex.OrderBook(t.symbol)
	if err != nil {
		logger.Sugar.Errorf("get order book error: %s", err)
		return
	}
	b1, _ := gateio.Buy1(ob.Bids)
	price := decimal.NewFromFloat(b1)
	// sell all balance
	amount := balance[t.baseCurrency].Round(t.amountPrecision)
	if amount.GreaterThan(balance[t.baseCurrency]) {
		amount = amount.Sub(t.minAmount)
	}
	logger.Sugar.Debugf("balance: %v, amount: %s", balance, amount)
	if amount.LessThan(t.minAmount) {
		logger.Sugar.Infof("amount too small: %s", amount)
		return
	}
	clientId := fmt.Sprintf("c-%d-0", t.state.SellTimes+1)
	orderId, err := t.ex.Sell(t.symbol, price, amount, gateio.OrderTypeNormal, clientId)
	if err != nil {
		logger.Sugar.Errorf("sell error: %s", err)
		return
	}
	logger.Sugar.Infof("清仓，订单号: %d / %s", orderId, clientId)

	t.state.BuyTimes = 0
	t.state.SellTimes++
	t.state.Position = 0
	t.state.LastBuyPrice = 0
	t.savePosition(ctx)
}
