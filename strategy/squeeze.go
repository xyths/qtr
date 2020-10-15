package strategy

import (
	"context"
	indicator "github.com/xyths/go-indicators"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/qtr/exchange/huobi"
	"github.com/xyths/qtr/types"
	"go.uber.org/zap"
	"time"
)

type SqueezeStrategyConf struct {
	Total    float64
	Interval string

	BBL int     `json:"bbl"` // BB Length
	BBF float64 `json:"bbf"` // BB MultiFactor
	KCL int     `json:"kcl"` // KC Length
	KCF float64 `json:"kcf"` // KC MultiFactor
}

type SqueezeBase struct {
	config   SqueezeStrategyConf
	interval time.Duration
	symbol   string
	dry      bool

	Sugar *zap.SugaredLogger
	ex    exchange.RestAPIExchange

	candle hs.Candle

	handlerSqueezeOn func(last int, dry bool)
	handlerTrendOn   func(up bool, last int, dry bool)
	handlerTrendOff  func(up bool, last int, dry bool)
}

func NewSqueezeBase(config SqueezeStrategyConf, dry bool) *SqueezeBase {
	s := &SqueezeBase{
		config: config,
		dry:    dry,
		candle: hs.NewCandle(2000),
	}
	return s
}

func (s *SqueezeBase) Init(logger *zap.SugaredLogger, ex exchange.RestAPIExchange, symbol string,
	handlerSqueezeOn func(last int, dry bool), handlerTrendOn, handlerTrendOff func(up bool, last int, dry bool)) {
	interval, err := time.ParseDuration(s.config.Interval)
	if err != nil {
		panic(err)
	}
	s.interval = interval
	s.Sugar = logger
	s.symbol = symbol
	s.ex = ex
	s.handlerSqueezeOn = handlerSqueezeOn
	s.handlerTrendOn = handlerTrendOn
	s.handlerTrendOff = handlerTrendOff
}

// use candle not s.candle, for concurrency
func (s *SqueezeBase) onTick(candle hs.Candle, finished bool) {
	//s.Sugar.Debugf("onSqueezeTick")
	if !finished {
		return
	}
	r, d := indicator.Squeeze(
		s.config.BBL, s.config.KCL, s.config.BBF, s.config.KCF,
		candle.High, candle.Low, candle.Close,
	)
	l := candle.Length()
	current := l - 2
	if l < 3 {
		return
	}
	for i := current - 10; i >= 0 && i <= current; i++ {
		s.Sugar.Debugf(
			"[%d] %s %f %f %f %f, %.2f %t %t %t, kc(%.2f %.2f %.2f), bb(%.2f %.2f %.2f)",
			i, types.TimestampToDate(candle.Timestamp[i]),
			candle.Open[i], candle.High[i], candle.Low[i], candle.Close[i],
			d.Value[i], d.SqueezeOn[i], d.SqueezeOff[i], d.NoSqueeze[i],
			d.KCUpper[i], d.KCMiddle[i], d.KCLower[i], d.BBUpper[i], d.BBMiddle[i], d.BBLower[i],
		)
	}
	s.Sugar.Infof("trend %d, last %d", r.Trend, r.Last)
	switch r.Trend {
	case 0:
		s.handlerTrendOff(true, r.Last, s.dry)
	case 1:
		s.handlerSqueezeOn(r.Last, s.dry)
	case 2:
		s.handlerTrendOn(true, r.Last, s.dry)
	case -2:
		s.handlerTrendOn(false, r.Last, s.dry)
	}
}

type SqueezeWs struct {
	SqueezeBase
	wsEx exchange.Exchange
}

func NewSqueezeWs(config SqueezeStrategyConf, dry bool) *SqueezeWs {
	s := &SqueezeWs{
		SqueezeBase: *NewSqueezeBase(config, dry),
	}
	return s
}

func (s *SqueezeWs) Init(logger *zap.SugaredLogger, ex exchange.Exchange, symbol string,
	handlerSqueezeOn func(last int, dry bool), handlerTrendOn, handlerTrendOff func(up bool, last int, dry bool)) {
	s.SqueezeBase.Init(logger, ex, symbol, handlerSqueezeOn, handlerTrendOn, handlerTrendOff)
	s.wsEx = ex
}

func (s *SqueezeWs) Start() error {
	{
		to := time.Now()
		from := to.Add(-2000 * s.interval)
		candle, err := s.ex.CandleFrom(s.symbol, "squeeze-candle", s.interval, from, to)
		if err != nil {
			return err
		}
		s.candle.Add(candle)
		s.onTick(s.candle, true)
	}
	s.wsEx.SubscribeCandlestick(s.symbol, "squeeze-tick", s.interval, s.tickerHandler)
	return nil
}

func (s *SqueezeWs) Stop() {
	s.wsEx.UnsubscribeCandlestick(s.symbol, "squeeze-tick", s.interval)
}

func (s *SqueezeWs) tickerHandler(resp interface{}) {
	ticker, candle, err := huobi.CandlestickHandler(resp)
	if err != nil {
		s.Sugar.Info(err)
		return
	}
	if ticker != nil {
		finished := false
		if s.candle.Length() > 0 {
			oldTime := s.candle.Timestamp[s.candle.Length()-1]
			newTime := ticker.Timestamp
			s.candle.Append(*ticker)
			if oldTime != newTime {
				s.Sugar.Info("candle is finished, ready for strategy")
				finished = true
			}
			s.onTick(s.candle, finished)
		} else {
			s.Sugar.Info("candle is not ready for append")
		}
	}
	if candle != nil {
		s.Sugar.Info("tickerHandler has candle data, this should not append")
	}
}

type SqueezeRest struct {
	SqueezeBase
}

func NewSqueezeRest(config SqueezeStrategyConf, dry bool) *SqueezeRest {
	s := &SqueezeRest{
		SqueezeBase: *NewSqueezeBase(config, dry),
	}
	return s
}

func (s *SqueezeRest) Init(logger *zap.SugaredLogger, ex exchange.RestAPIExchange, symbol string,
	handlerSqueezeOn func(last int, dry bool), handlerTrendOn, handlerTrendOff func(up bool, last int, dry bool)) {
	s.SqueezeBase.Init(logger, ex, symbol, handlerSqueezeOn, handlerTrendOn, handlerTrendOff)
}

func (s *SqueezeRest) Run(ctx context.Context) {
	s.doWork(ctx)
}

func (s *SqueezeRest) doWork(ctx context.Context) {
	s.Sugar.Debug("doWork")
	candle, err := s.ex.CandleBySize(s.symbol, s.interval, 2000)
	if err != nil {
		s.Sugar.Error(err)
		return
	}

	s.onTick(candle, true)
}
