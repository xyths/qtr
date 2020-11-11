package super

import (
	"context"
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/broadcast"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/hs/exchange/gateio"
	"github.com/xyths/hs/exchange/huobi"
	"github.com/xyths/hs/logger"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"strings"
	"time"
)

type BaseTrader struct {
	config    Config
	interval  time.Duration
	Factor    float64
	Period    int
	StopLoss  bool
	Reinforce float64
	dry       bool

	Sugar  *zap.SugaredLogger
	db     *mongo.Database
	ex     exchange.RestAPIExchange
	symbol exchange.Symbol
	fee    exchange.Fee
	robots []broadcast.Broadcaster

	maxTotal decimal.Decimal // max total for buy order, half total in config
}

func NewBaseTraderFromConfig(ctx context.Context, cfg Config) (*BaseTrader, error) {
	interval, err := time.ParseDuration(cfg.Strategy.Interval)
	if err != nil {
		logger.Sugar.Fatalf("error interval format: %s", cfg.Strategy.Interval)
	}
	s := &BaseTrader{
		config:   cfg,
		interval: interval,
		maxTotal: decimal.NewFromFloat(cfg.Strategy.Total / 2),
	}
	return s, err
}

func (t *BaseTrader) Init(ctx context.Context) error {
	t.Factor = t.config.Strategy.Factor
	t.Period = t.config.Strategy.Period
	t.StopLoss = t.config.Strategy.StopLoss
	t.Reinforce = t.config.Strategy.Reinforce
	if err := t.initLogger(); err != nil {
		return err
	}
	db, err := hs.ConnectMongo(ctx, t.config.Mongo)
	if err != nil {
		return err
	}
	t.db = db
	if err := t.initEx(); err != nil {
		return err
	}
	t.initRobots(ctx)
	return nil
}

func (t *BaseTrader) initLogger() error {
	l, err := hs.NewZapLogger(t.config.Log)
	if err != nil {
		return err
	}
	t.Sugar = l.Sugar()
	t.Sugar.Info("Logger initialized")
	return nil
}

func (t *BaseTrader) initEx() error {
	switch t.config.Exchange.Name {
	case "gate":
		if err := t.initGate(); err != nil {
			return err
		}
	case "huobi":
		if err := t.initHuobi(); err != nil {
			return err
		}
	default:
		return errors.New("unsupported exchange")
	}
	t.Sugar.Info("Exchange initialized")
	t.Sugar.Infof(
		"Symbol: %s, PricePrecision: %d, AmountPrecision: %d, MinAmount: %s, MinTotal: %s",
		t.Symbol(),
		t.PricePrecision(), t.AmountPrecision(),
		t.MinAmount(), t.MinTotal(),
	)
	t.Sugar.Infof(
		"BaseMakerFee: %s, BaseTakerFee: %s, ActualMakerFee: %s, ActualTakerFee: %s",
		t.BaseMakerFee(), t.BaseTakerFee(),
		t.ActualMakerFee(), t.ActualTakerFee(),
	)
	return nil
}

func (t *BaseTrader) initGate() error {
	t.ex = gateio.New(t.config.Exchange.Key, t.config.Exchange.Secret, t.config.Exchange.Host)
	symbol, err := t.ex.GetSymbol(context.Background(), t.config.Exchange.Symbols[0])
	if err != nil {
		return err
	}
	t.symbol = symbol
	t.fee, err = t.ex.GetFee(t.Symbol())
	return err
}

func (t *BaseTrader) initHuobi() (err error) {
	t.ex, err = huobi.New(t.config.Exchange.Label, t.config.Exchange.Key, t.config.Exchange.Secret, t.config.Exchange.Host)
	if err != nil {
		return err
	}
	t.symbol, err = t.ex.GetSymbol(context.Background(), t.config.Exchange.Symbols[0])
	if err != nil {
		return err
	}
	t.fee, err = t.ex.GetFee(t.Symbol())
	return err
}

func (t *BaseTrader) initRobots(ctx context.Context) {
	for _, conf := range t.config.Robots {
		t.robots = append(t.robots, broadcast.New(conf))
	}
	t.Sugar.Info("Broadcasters initialized")
}

func (t *BaseTrader) Broadcast(format string, a ...interface{}) {
	message := fmt.Sprintf(format, a...)
	labels := []string{t.config.Exchange.Name, t.config.Exchange.Label}
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	timeStr := time.Now().In(beijing).Format(layout)

	msg := fmt.Sprintf("%t [%t] [%t] %t", timeStr, strings.Join(labels, "] ["), t.Symbol(), message)
	for _, robot := range t.robots {
		if err := robot.SendText(msg); err != nil {
			t.Sugar.Infof("broadcast error: %t", err)
		}
	}
}
func (t *BaseTrader) Symbol() string {
	return t.symbol.Symbol
}
func (t *BaseTrader) QuoteCurrency() string {
	return t.symbol.QuoteCurrency
}
func (t *BaseTrader) BaseCurrency() string {
	return t.symbol.BaseCurrency
}
func (t *BaseTrader) PricePrecision() int32 {
	return t.symbol.PricePrecision
}
func (t *BaseTrader) AmountPrecision() int32 {
	return t.symbol.AmountPrecision
}
func (t *BaseTrader) MinAmount() decimal.Decimal {
	return t.symbol.MinAmount
}
func (t *BaseTrader) MinTotal() decimal.Decimal {
	return t.symbol.MinTotal
}
func (t *BaseTrader) BaseMakerFee() decimal.Decimal {
	return t.fee.BaseMaker
}
func (t *BaseTrader) BaseTakerFee() decimal.Decimal {
	return t.fee.BaseTaker
}
func (t *BaseTrader) ActualMakerFee() decimal.Decimal {
	return t.fee.ActualMaker
}
func (t *BaseTrader) ActualTakerFee() decimal.Decimal {
	return t.fee.ActualTaker
}
