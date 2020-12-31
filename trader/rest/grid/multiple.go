package grid

import (
	"context"
	"github.com/markcheno/go-talib"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/hs/exchange/gateio"
	"github.com/xyths/hs/exchange/huobi"
	"github.com/xyths/qtr/trader/rest/trigger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"sort"
	"strings"
	"time"
)

type MultipleGridConfig struct {
	Exchange        hs.ExchangeConf
	Mongo           hs.MongoConf
	Total           float64
	Number          int
	Interval        string
	VolumeThreshold float64 `json:"volumeThreshold"`
	Trigger         trigger.Config
	Log             hs.LogConf
	Robots          []hs.BroadcastConf
}

type MultipleGridTrader struct {
	config   MultipleGridConfig
	maxTotal decimal.Decimal
	dry      bool
	interval time.Duration

	Sugar  *zap.SugaredLogger
	db     *mongo.Database
	ex     exchange.RestAPIExchange
	robots []broadcast.Broadcaster

	Log    hs.LogConf
	Robots []hs.BroadcastConf

	stopped []exchange.Symbol
	started []exchange.Symbol

	trigger *trigger.Trigger
	grids   map[string]*RestGridTrader
}

func NewMultipleGridTrader(ctx context.Context, configFilename string, dry bool) (*MultipleGridTrader, error) {
	cfg := MultipleGridConfig{}
	err := hs.ParseJsonConfig(configFilename, &cfg)
	if err != nil {
		return nil, err
	}
	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil {
		return nil, err
	}
	t := &MultipleGridTrader{
		config:   cfg,
		maxTotal: decimal.NewFromFloat(cfg.Total),
		dry:      dry,
		interval: interval,
		trigger:  trigger.NewTrigger(cfg.Trigger),
		grids:    make(map[string]*RestGridTrader),
	}

	err = t.init(ctx)
	return t, nil
}

func (t *MultipleGridTrader) Close(ctx context.Context) {

}

func (t *MultipleGridTrader) Start(ctx context.Context) error {
	t.doWork(ctx)
	wakeTime := time.Now().Truncate(t.interval)
	wakeTime = wakeTime.Add(t.interval)
	sleepTime := time.Until(wakeTime)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepTime):
			t.doWork(ctx)
			wakeTime = wakeTime.Add(t.interval)
			sleepTime = time.Until(wakeTime)
		}
	}
}

func (t *MultipleGridTrader) doWork(ctx context.Context) {
	t.Sugar.Info("multiple grid check started")
	if err := t.updateSymbols(ctx); err != nil {
		t.Sugar.Errorf("update symbol error: %s", err)
		return
	}
	var newStarted []exchange.Symbol
	var newStopped []exchange.Symbol
	for _, s := range t.started {
		t.Sugar.Infof("check running grid %s", s.Symbol)
		started, err := t.checkSymbol(ctx, s)
		if err != nil || started {
			newStarted = append(newStarted, s)
			t.Sugar.Infof("running grid %s keep running", s.Symbol)
		} else {
			newStopped = append(newStopped, s)
			t.Sugar.Infof("running grid %s need stop", s.Symbol)
		}
	}
	for _, s := range t.stopped {
		if len(newStarted) >= t.config.Number {
			newStopped = append(newStopped, s)
			continue
		}
		t.Sugar.Infof("check new symbol %s", s.Symbol)
		started, err := t.checkSymbol(ctx, s)
		if err == nil && started {
			newStarted = append(newStarted, s)
			t.Sugar.Infof("new grid %s started", s.Symbol)
		} else {
			newStopped = append(newStopped, s)
		}
	}

	t.started = newStarted
	t.stopped = newStopped
	t.Sugar.Info("multiple grid check finished")
}

func (t *MultipleGridTrader) Stop(ctx context.Context) error {
	t.Sugar.Info("MultipleGridTrader stopped")
	return nil
}

func (t *MultipleGridTrader) init(ctx context.Context) error {
	if l, err := hs.NewZapLogger(t.config.Log); err != nil {
		return err
	} else {
		t.Sugar = l.Sugar()
	}
	t.Sugar.Info("Logger initialized")
	db, err := hs.ConnectMongo(ctx, t.config.Mongo)
	if err != nil {
		return err
	}
	t.db = db
	t.initRobots()
	t.Sugar.Info("robots initialized")
	if err := t.initExecutor(); err != nil {
		return err
	}
	t.Sugar.Info("executor initialized")
	t.trigger.Init(t.Sugar, t.ex)
	t.Sugar.Info("trigger initialized")
	t.Sugar.Info("Multiple Grid Trader initialized")
	return nil
}

func (t *MultipleGridTrader) initRobots() {
	for _, conf := range t.config.Robots {
		t.robots = append(t.robots, broadcast.New(conf))
	}
	t.Sugar.Info("Broadcasters initialized")
}

func (t *MultipleGridTrader) initExecutor() (err error) {
	cfg := t.config.Exchange
	switch cfg.Name {
	case "huobi":
		t.ex, err = huobi.New(cfg.Label, cfg.Key, cfg.Secret, cfg.Host)
		if err != nil {
			return
		}
	case "gate":
		t.ex = gateio.New(cfg.Key, cfg.Secret, cfg.Host, t.Sugar)
	}
	return nil
}

func (t *MultipleGridTrader) updateSymbols(ctx context.Context) error {
	allSymbols, err := t.allSymbols(ctx)
	if err != nil {
		return err
	}
	// check if there is new symbols
	t.stopped = nil
	for _, s := range allSymbols {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if g, ok := t.grids[s.Symbol]; !ok || !g.Running {
				t.stopped = append(t.stopped, s)
				t.Sugar.Infof("update new or stopped symbol: %s", s.Symbol)
			} else {
				t.Sugar.Infof("ignore running symbol: %s", s.Symbol)
			}
		}
	}
	// remove disabled symbols
	return nil
}

func (t *MultipleGridTrader) allSymbols(ctx context.Context) ([]exchange.Symbol, error) {
	symbols, err := t.ex.AllSymbols(ctx)
	if err != nil {
		return nil, err
	}
	var ret []exchange.Symbol
	threshold := decimal.NewFromFloat(t.config.VolumeThreshold)
	for _, s := range symbols {
		select {
		case <-ctx.Done():
			return ret, ctx.Err()
		default:
			// filter by quote currency "usdt"
			if !s.Disabled && strings.HasSuffix(s.Symbol, "usdt") {
				v, err1 := t.ex.Last24hVolume(s.Symbol)
				if err1 != nil {
					t.Sugar.Errorf("get last 24h volume error: %s", err1)
					continue
				}
				t.Sugar.Debugf("%s vol: %s", s.Symbol, v)
				if v.GreaterThanOrEqual(threshold) {
					ret = append(ret, s)
				}
			}
		}
	}
	var natrs hs.KVSlice
	for _, s := range ret {
		select {
		case <-ctx.Done():
			return ret, ctx.Err()
		default:
			c, err1 := t.ex.CandleBySize(s.Symbol, exchange.DAY1, 1000)
			if err1 != nil || c.Length() < 16 {
				continue
			}
			natr := talib.Natr(c.High, c.Low, c.Close, 14)
			natrs = append(natrs, hs.FloatTuple{
				Key:   s,
				Value: natr[len(natr)-2],
			})
		}
	}
	sort.Sort(sort.Reverse(natrs))
	ret = make([]exchange.Symbol, len(natrs))
	for i := 0; i < len(natrs); i++ {
		ret[i] = natrs[i].Key.(exchange.Symbol)
		t.Sugar.Debugf("%s natr: %f", ret[i].Symbol, natrs[i].Value)
	}

	return ret, nil
}

func (t *MultipleGridTrader) checkSymbol(ctx context.Context, symbol exchange.Symbol) (started bool, err error) {
	turnOn, err := t.trigger.Check(ctx, symbol.Symbol)
	if err != nil {
		t.Sugar.Errorf("check symbol %s error", symbol.Symbol, err)
		return
	}
	g, err := t.getGridService(ctx, symbol)
	if err != nil {
		t.Sugar.Errorf("get grid %s error", symbol.Symbol, err)
		return
	}
	if turnOn {
		return t.startGridService(ctx, g)
	}
	_, err = t.stopGridService(ctx, g)
	return
}

func (t *MultipleGridTrader) getGridService(ctx context.Context, symbol exchange.Symbol) (g *RestGridTrader, err error) {
	g, ok := t.grids[symbol.Symbol]
	if ok {
		t.Sugar.Debugf("recall grid %s from cache", symbol.Symbol)
		return g, nil
	}
	t.Sugar.Infof("new grid %s service", symbol.Symbol)
	// dummy grid
	g = &RestGridTrader{
		Symbol: symbol,
	}
	t.grids[symbol.Symbol] = g
	return
}

func (t *MultipleGridTrader) startGridService(ctx context.Context, g *RestGridTrader) (success bool, err error) {
	//if g.Running {
	//	return true, nil
	//}
	//go func() {
	//	if err := g.Start(ctx); err != nil {
	//		t.Sugar.Errorf("start grid (symbol %s) error", g.Symbol.Symbol, err)
	//	}
	//}()
	t.Sugar.Infof("grid %s is started", g.Symbol.Symbol)
	return true, nil
}

func (t *MultipleGridTrader) stopGridService(ctx context.Context, g *RestGridTrader) (success bool, err error) {
	//if !g.Running {
	//	return true, nil
	//}
	//err = g.Stop(ctx)
	//if err != nil {
	//	t.Sugar.Errorf("stop grid (symbol %s) error", g.Symbol.Symbol, err)
	//	return
	//} else {
	//	success = true
	//}
	//return
	t.Sugar.Infof("grid %s is stopped", g.Symbol.Symbol)
	return true, nil
}
