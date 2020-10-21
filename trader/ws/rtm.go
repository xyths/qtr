package ws

import (
	"context"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/qtr/executor"
	"github.com/xyths/qtr/strategy"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"strings"
	"time"
)

type RtmTraderConfig struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Strategy strategy.RtmStrategyConf
	Log      hs.LogConf
	Robots   []hs.BroadcastConf
}

var (
	emptyBuyOrder  = executor.BuyOrder{Name: "buyOrder"}
	emptySellOrder = executor.SellOrder{Name: "sellOrder"}
)

type RtmTrader struct {
	config   RtmTraderConfig
	maxTotal decimal.Decimal
	dry      bool

	Sugar  *zap.SugaredLogger
	db     *mongo.Database
	ex     *executor.Executor
	robots []broadcast.Broadcaster

	strategy *strategy.RTMStrategy
}

func NewRtmTrader(ctx context.Context, configFilename string, dry bool) (*RtmTrader, error) {
	cfg := RtmTraderConfig{}
	err := hs.ParseJsonConfig(configFilename, &cfg)
	if err != nil {
		return nil, err
	}
	s := &RtmTrader{
		config:   cfg,
		maxTotal: decimal.NewFromFloat(cfg.Strategy.Total),
		strategy: strategy.NewRTMStrategy(cfg.Strategy, dry),
	}

	s.ex, err = executor.NewExecutor(cfg.Exchange)
	if err != nil {
		return nil, err
	}
	err = s.Init(ctx)
	return s, err
}

func (t *RtmTrader) Init(ctx context.Context) error {
	if err := t.initLogger(); err != nil {
		return err
	}
	db, err := hs.ConnectMongo(ctx, t.config.Mongo)
	if err != nil {
		return err
	}
	t.db = db
	t.initRobots()
	t.ex.Init(t.Sugar, t.db, t.maxTotal)
	t.strategy.Init(t.Sugar, t.ex)

	t.Sugar.Info("RTM Trader initialized")
	return nil
}

func (t *RtmTrader) Close(ctx context.Context) {
	if t.db != nil {
		_ = t.db.Client().Disconnect(ctx)
	}
	if t.Sugar != nil {
		t.Sugar.Info("RTM Trader stopped")
		t.Sugar.Sync()
	}
}

func (t *RtmTrader) Print(ctx context.Context) error {
	//t.loadState(ctx)
	//t.startedLock.RLock()
	//started := t.started
	//t.startedLock.RUnlock()
	//log.Printf(`State
	//Position: %d
	//Unique Id: %d
	//Long times: %d
	//Short times: %d
	//RTM status: %t`,
	//	t.position,
	//	t.uniqueId,
	//	t.LongTimes,
	//	t.ShortTimes,
	//	started,
	//)
	//log.Printf(`Sell-stop order
	//Id: %d / %s
	//Price: %s
	//Amount: %s
	//Create Time: %s`,
	//	t.sellStopOrder.Id, t.sellStopOrder.ClientOrderId,
	//	t.sellStopOrder.Price,
	//	t.sellStopOrder.Amount,
	//	t.sellStopOrder.Time,
	//)

	return nil
}

func (t *RtmTrader) Clear(ctx context.Context) error {
	t.clearState(ctx)
	return nil
}

func (t *RtmTrader) Start(ctx context.Context, dry bool) error {
	t.loadState(ctx)
	t.checkState(ctx)
	t.dry = dry
	if t.dry {
		t.Sugar.Info("This is dry-run")
	}

	// setup order subscriber

	return nil
}
func (t *RtmTrader) Stop() {
}

func (t *RtmTrader) initLogger() error {
	l, err := hs.NewZapLogger(t.config.Log)
	if err != nil {
		return err
	}
	t.Sugar = l.Sugar()
	t.Sugar.Info("Logger initialized")
	return nil
}

//func (t *RtmTrader) initEx() error {
//	ex, err := executor.NewExecutor(t.config.Exchange)
//	if err != nil {
//		return err
//	}
//	t.ex = ex
//	t.Sugar.Info("Executor initialized")
//	t.Sugar.Infof(
//		"Symbol: %s, PricePrecision: %d, AmountPrecision: %d, MinAmount: %s, MinTotal: %s",
//		t.ex.Symbol(),
//		t.ex.PricePrecision(), t.ex.AmountPrecision(),
//		t.ex.MinAmount(), t.ex.MinTotal(),
//	)
//	t.Sugar.Infof(
//		"BaseMakerFee: %s, BaseTakerFee: %s, ActualMakerFee: %s, ActualTakerFee: %s",
//		t.ex.BaseMakerFee(), t.ex.BaseTakerFee(),
//		t.ex.ActualMakerFee(), t.ex.ActualTakerFee(),
//	)
//	return nil
//}

func (t *RtmTrader) initRobots() {
	for _, conf := range t.config.Robots {
		t.robots = append(t.robots, broadcast.New(conf))
	}
	t.Sugar.Info("Broadcasters initialized")
}

func (t *RtmTrader) Broadcast(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	labels := []string{t.config.Exchange.Name, t.config.Exchange.Label}
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	timeStr := time.Now().In(beijing).Format(layout)

	msg := fmt.Sprintf("%s [%s] [%s] %s", timeStr, strings.Join(labels, "] ["), t.ex.Symbol(), message)
	for _, robot := range t.robots {
		if err := robot.SendText(msg); err != nil {
			t.Sugar.Infof("broadcast error: %s", err)
		}
	}
}

func (t *RtmTrader) loadState(ctx context.Context) error {
	if err := t.ex.Load(ctx); err != nil {
		return err
	}
	return nil
}

func (t *RtmTrader) clearState(ctx context.Context) {
	//t.ClientIdManager.LongReset(ctx)
	//t.ClientIdManager.ShortReset(ctx)

	coll := t.db.Collection(collNameState)
	if err := hs.DeleteInt64(ctx, coll, "sellStopOrder"); err != nil {
		t.Sugar.Errorf("delete sellStopOrder error: %s", err)
	} else {
		t.Sugar.Info("delete sellStopOrder from database")
	}
}

func (t *RtmTrader) checkState(ctx context.Context) {
	// check quota
}
