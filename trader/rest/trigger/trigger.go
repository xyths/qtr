package trigger

import (
	"context"
	indicator "github.com/xyths/go-indicators"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/qtr/strategy/options"
	"go.uber.org/zap"
	"time"
)

type Config struct {
	Squeeze    *options.SqueezeStrategyOption    `json:"squeeze"`
	SuperTrend *options.SuperTrendStrategyOption `json:"superTrend"`
	Size       int64
}

type Rule struct {
	Period     time.Duration
	Squeeze    bool
	SuperTrend bool
}

type Trigger struct {
	config Config
	Sugar  *zap.SugaredLogger
	ex     exchange.RestAPIExchange
	rules  []Rule
}

func NewTrigger(cfg Config, ) *Trigger {
	return &Trigger{
		config: cfg,
	}
}

func (t *Trigger) Init(sugar *zap.SugaredLogger, ex exchange.RestAPIExchange) {
	t.Sugar = sugar
	t.ex = ex
	// check rules
	t.initCheckRules()
}

func (t *Trigger) Check(ctx context.Context, symbol string) (bool, error) {
	allOn := false
	for _, r := range t.rules {
		if r.Squeeze || r.SuperTrend {
			on, err := t.CheckPeriod(ctx, symbol, t.config.Size, exchange.MON1, r.Squeeze, r.SuperTrend)
			if err != nil || !on {
				return on, err
			}
			allOn = true
		}
	}
	return allOn, nil
}

func (t *Trigger) CheckPeriod(ctx context.Context, symbol string, size int64, period time.Duration, squeeze, super bool) (on bool, err error) {
	candle, err := t.ex.CandleBySize(symbol, period, int(size))
	if err != nil {
		return
	}
	if squeeze {
		if candle.Length() <= t.config.Squeeze.BBL+2 || candle.Length() <= t.config.Squeeze.KCL+2 {
			return
		}
		r, _ := indicator.Squeeze(
			t.config.Squeeze.BBL, t.config.Squeeze.KCL,
			t.config.Squeeze.BBF, t.config.Squeeze.KCF,
			candle.High, candle.Low, candle.Close,
		)
		if r.Trend != 2 {
			return false, nil
		}
	}
	if super {
		if candle.Length() <= t.config.SuperTrend.Period+2 {
			return
		}
		_, trend := indicator.SuperTrend(
			t.config.SuperTrend.Factor, t.config.SuperTrend.Period,
			candle.High, candle.Low, candle.Close,
		)
		return trend[len(trend)-2], nil
	}
	return
}

func (t *Trigger) initCheckRules() {
	if (t.config.Squeeze != nil && t.config.Squeeze.Month) || (t.config.SuperTrend != nil && t.config.SuperTrend.Month) {
		t.rules = append(t.rules, Rule{
			Period:     exchange.MON1,
			Squeeze:    t.config.Squeeze != nil && t.config.Squeeze.Month,
			SuperTrend: t.config.SuperTrend != nil && t.config.SuperTrend.Month,
		})
	}
	if (t.config.Squeeze != nil && t.config.Squeeze.Week) || (t.config.SuperTrend != nil && t.config.SuperTrend.Week) {
		t.rules = append(t.rules, Rule{
			Period:     exchange.WEEK1,
			Squeeze:    t.config.Squeeze != nil && t.config.Squeeze.Week,
			SuperTrend: t.config.SuperTrend != nil && t.config.SuperTrend.Week,
		})
	}
	if (t.config.Squeeze != nil && t.config.Squeeze.Day) || (t.config.SuperTrend != nil && t.config.SuperTrend.Day) {
		t.rules = append(t.rules, Rule{
			Period:     exchange.DAY1,
			Squeeze:    t.config.Squeeze != nil && t.config.Squeeze.Day,
			SuperTrend: t.config.SuperTrend != nil && t.config.SuperTrend.Day,
		})
	}
	if (t.config.Squeeze != nil && t.config.Squeeze.Hour4) || (t.config.SuperTrend != nil && t.config.SuperTrend.Hour4) {
		t.rules = append(t.rules, Rule{
			Period:     exchange.HOUR4,
			Squeeze:    t.config.Squeeze != nil && t.config.Squeeze.Hour4,
			SuperTrend: t.config.SuperTrend != nil && t.config.SuperTrend.Hour4,
		})
	}
	if (t.config.Squeeze != nil && t.config.Squeeze.Hour1) || (t.config.SuperTrend != nil && t.config.SuperTrend.Hour1) {
		t.rules = append(t.rules, Rule{
			Period:     exchange.HOUR1,
			Squeeze:    t.config.Squeeze != nil && t.config.Squeeze.Hour1,
			SuperTrend: t.config.SuperTrend != nil && t.config.SuperTrend.Hour1,
		})
	}
}
