package rest

import (
	"context"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/qtr/executor"
	"github.com/xyths/qtr/strategy"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type SqueezeMomentumConfig struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Strategy strategy.SqueezeStrategyConf
	Log      hs.LogConf
	Robots   []hs.BroadcastConf
}

type SqueezeMomentumTrader struct {
	config   SqueezeMomentumConfig
	maxTotal decimal.Decimal

	Sugar  *zap.SugaredLogger
	db     *mongo.Database
	ex     *executor.Executor
	robots []broadcast.Broadcaster

	strategy *strategy.SqueezeRest

	trend int // 0: default (no squeeze/trend stop), 1 squeeze, 2 up trend on, -2 down trend on
}

func NewSqueezeMomentumTrader(ctx context.Context, configFilename string, dry bool) (*SqueezeMomentumTrader, error) {
	cfg := SqueezeMomentumConfig{}
	err := hs.ParseJsonConfig(configFilename, &cfg)
	if err != nil {
		return nil, err
	}
	t := &SqueezeMomentumTrader{
		config:   cfg,
		maxTotal: decimal.NewFromFloat(cfg.Strategy.Total),
		strategy: strategy.NewSqueezeRest(cfg.Strategy, dry),
	}
	t.ex, err = executor.NewExecutor(cfg.Exchange)
	if err != nil {
		return nil, err
	}
	err = t.init(ctx)
	return t, nil
}

func (t *SqueezeMomentumTrader) Run(ctx context.Context) {
	t.Sugar.Info("Squeeze started")
	// load previous state
	t.Load(ctx)
	t.Sugar.Debugf("old trend: %d", t.trend)
	t.strategy.Run(ctx)
	t.Sugar.Debugf("new trend: %d", t.trend)
	t.Sugar.Info("Squeeze finished")
}

func (t *SqueezeMomentumTrader) Print(_ context.Context) {
	t.Sugar.Info("print no implemented")
}

func (t *SqueezeMomentumTrader) Clear(ctx context.Context) {
	t.Sugar.Info("clear no implemented")
}

func (t *SqueezeMomentumTrader) Close(ctx context.Context) {
	if t.db != nil {
		_ = t.db.Client().Disconnect(ctx)
	}
	if t.Sugar != nil {
		t.Sugar.Info("Squeeze Trader closed with log synced")
		t.Sugar.Sync()
	}
}

func (t *SqueezeMomentumTrader) init(ctx context.Context) error {
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
	t.strategy.Init(t.Sugar, t.ex.Exchange(), t.ex.Symbol(), t.squeezeOn, t.trendOn, t.trendOff)

	t.Sugar.Info("Squeeze restful Trader initialized")
	return nil
}
func (t *SqueezeMomentumTrader) initLogger() error {
	l, err := hs.NewZapLogger(t.config.Log)
	if err != nil {
		return err
	}
	t.Sugar = l.Sugar()
	t.Sugar.Info("Logger initialized")
	return nil
}

func (t *SqueezeMomentumTrader) initRobots() {
	for _, conf := range t.config.Robots {
		t.robots = append(t.robots, broadcast.New(conf))
	}
	t.Sugar.Info("Broadcasters initialized")
}

func (t *SqueezeMomentumTrader) Load(ctx context.Context) {
	t.loadTrend(ctx)
}

const collNameState = "state"

func (t *SqueezeMomentumTrader) loadTrend(ctx context.Context) {
	if err := hs.LoadKey(ctx, t.db.Collection(collNameState), "trend", &t.trend); err != nil {
		t.Sugar.Errorf("load trend error: %s", err)
	} else {
		t.Sugar.Infof("load trend: %d", t.trend)
	}
}
func (t *SqueezeMomentumTrader) saveTrend(ctx context.Context) {
	if err := hs.SaveKey(ctx, t.db.Collection(collNameState), "trend", t.trend); err != nil {
		t.Sugar.Errorf("save trend error: %s", err)
	} else {
		t.Sugar.Infof("save trend: %d", t.trend)
	}
}

func (t *SqueezeMomentumTrader) squeezeOn(last int, dry bool) {
	t.Sugar.Infof("squeeze on, last %d, wait for trend", last)
	if t.trend != 1 {
		t.trend = 1
		t.saveTrend(context.Background())
	}
}
func (t *SqueezeMomentumTrader) trendOn(up bool, last int, dry bool) {
	t.Sugar.Infof("trend fire off, up %v, last %d", up, last)
	if up {
		if t.trend != 2 {
			t.Sugar.Infof("first time go to up trend")
			t.trend = 2
			t.saveTrend(context.Background())
			// buy market
			if !dry {
				if err := t.ex.BuyAllMarket(); err != nil {
					t.Sugar.Error(err)
				}
			}
		}
	} else {
		if t.trend != -2 {
			t.Sugar.Infof("first time go to down trend")
			t.trend = -2
			t.saveTrend(context.Background())
		}
	}
}
func (t *SqueezeMomentumTrader) trendOff(up bool, last int, dry bool) {
	t.Sugar.Infof("trend stopped, up %v, last %d", up, last)
	if t.trend != 0 {
		t.Sugar.Infof("first time trend stopped")
		t.trend = 0
		t.saveTrend(context.Background())
		// sell market
		if !dry {
			if err := t.ex.SellAllMarket(); err != nil {
				t.Sugar.Error(err)
			}
		}
	}
}
