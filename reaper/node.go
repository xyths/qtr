package reaper

import (
	"context"
	"errors"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange/huobi"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type Config struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Log      hs.LogConf
	Robots   []hs.BroadcastConf
}

type Reaper struct {
	cfg Config

	Sugar  *zap.SugaredLogger
	db     *mongo.Database
	ex     *huobi.Client
	symbol string

	ch     chan Signal
	beacon Beacon
}

func New(cfg Config) *Reaper {
	return &Reaper{
		cfg: cfg,
		beacon: Beacon{
			MaxLength: 2000,
			MinLength: 200,
		},
	}
}

func (r *Reaper) Init(ctx context.Context) error {
	r.symbol = r.cfg.Exchange.Symbols[0]
	l, err := hs.NewZapLogger(r.cfg.Log)
	if err != nil {
		return err
	}
	r.Sugar = l.Sugar()
	r.Sugar.Info("Logger initialized")
	db, err := hs.ConnectMongo(ctx, r.cfg.Mongo)
	if err != nil {
		return err
	}
	r.db = db
	switch r.cfg.Exchange.Name {
	case "gate":
		// 使用V4版本API
		//if err := r.initGate(); err != nil {
		//	return err
		//}
		r.Sugar.Info("should try gate v4 API here")
	case "huobi":
		r.ex, err = huobi.New(r.cfg.Exchange.Label, r.cfg.Exchange.Key, r.cfg.Exchange.Secret, r.cfg.Exchange.Host)
		if err != nil {
			r.Sugar.Errorf("new huobi ex error: %s", err)
			return err
		}
		r.Sugar.Info("exchange huobi initialized")
	default:
		return errors.New("unsupported exchange")
	}
	r.Sugar.Info("Exchange initialized")
	//r.Sugar.Infof(
	//	"Symbol: %s, PricePrecision: %d, AmountPrecision: %d, MinAmount: %s, MinTotal: %s",
	//	r.Symbol(),
	//	r.PricePrecision(), r.AmountPrecision(),
	//	r.MinAmount(), r.MinTotal(),
	//)
	//r.Sugar.Infof(
	//	"BaseMakerFee: %s, BaseTakerFee: %s, ActualMakerFee: %s, ActualTakerFee: %s",
	//	r.BaseMakerFee(), r.BaseTakerFee(),
	//	r.ActualMakerFee(), r.ActualTakerFee(),
	//)
	r.Sugar.Info("reaper initialized")
	return nil
}

func (r *Reaper) Close(ctx context.Context) {
	r.Sugar.Info("reaper closed")
}

func (r *Reaper) Start(ctx context.Context) error {
	r.ch = make(chan Signal, 10)
	go r.startExecutor(ctx)
	r.subscribeTrade()

	r.Sugar.Info("reaper started")
	return nil
}

func (r *Reaper) Stop(ctx context.Context) {
	r.unsubscribeTrade()
	close(r.ch)
	r.Sugar.Info("reaper stopped")
}

func (r *Reaper) Print(ctx context.Context) error {
	r.Sugar.Info("reaper print")
	return nil
}

func (r *Reaper) Clear(ctx context.Context) error {
	r.Sugar.Info("reaper clear")
	return nil
}
