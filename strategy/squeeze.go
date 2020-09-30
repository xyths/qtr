package strategy

import (
	indicator "github.com/xyths/go-indicators"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/qtr/exchange/huobi"
	"github.com/xyths/qtr/types"
	"go.uber.org/zap"
	"time"
)

type SqueezeStrategyConf struct {
	Interval string

	BBL int     `json:"bbl"` // BB Length
	BBF float64 `json:"bbf"` // BB MultiFactor
	KCL int     `json:"kcl"` // KC Length
	KCF float64 `json:"kcf"` // KC MultiFactor
}

type SqueezeStrategy struct {
	config   SqueezeStrategyConf
	interval time.Duration
	symbol   string

	Sugar *zap.SugaredLogger
	ex    exchange.Exchange

	candle hs.Candle

	handlerSqueezeOn func(last int)
	handlerTrendOn   func(up bool, last int)
	handlerTrendOff  func(up bool, last int)
}

type SqueezeExecutor interface {
	SubscribeCandle(clientId string, period time.Duration, responseHandler func(interface{}))
}

func NewSqueezeStrategy(config SqueezeStrategyConf) *SqueezeStrategy {
	s := &SqueezeStrategy{
		config: config,
		candle: hs.NewCandle(2000),
	}
	return s
}

func (s *SqueezeStrategy) Init(logger *zap.SugaredLogger, ex exchange.Exchange, symbol string,
	handlerSqueezeOn func(last int), handlerTrendOn, handlerTrendOff func(up bool, last int)) {
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

func (s *SqueezeStrategy) Start() error {
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
	s.ex.SubscribeCandlestick(s.symbol, "squeeze-tick", s.interval, s.tickerHandler)
	return nil
}

func (s *SqueezeStrategy) Stop() {
	s.ex.UnsubscribeCandlestick(s.symbol, "squeeze-tick", s.interval)
}

func (s *SqueezeStrategy) tickerHandler(resp interface{}) {
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

// use candle not s.candle, for concurrency
func (s *SqueezeStrategy) onTick(candle hs.Candle, finished bool) {
	//s.Sugar.Debugf("onSqueezeTick")
	if !finished {
		return
	}
	val, squeezeOn, squeezeOff, noSqueeze := indicator.SqueezeMomentum(
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
			"[%d] %s %f %f %f %f, %.2f %t %t %t",
			i, types.TimestampToDate(candle.Timestamp[i]),
			candle.Open[i], candle.High[i], candle.Low[i], candle.Close[i],
			val[i], squeezeOn[i], squeezeOff[i], noSqueeze[i],
		)
	}
	if noSqueeze[current] {
		s.Sugar.Info("no squeeze")
		return
	}

	if squeezeOn[current] {
		// count the dark cross
		last := 0
		for i := current; squeezeOn[i] && i >= 0; i-- {
			last++
		}
		s.handlerSqueezeOn(last)
	} else {
		// find first gray cross
		firstGrayIndex := 0
		for i := current; !squeezeOn[i] && i >= 0; i-- {
			firstGrayIndex = i
		}
		isUptrend := val[firstGrayIndex] > 0
		trendStopped := false
		stopIndex := 0
		for i := firstGrayIndex; i <= current; i++ {
			if isUptrend && val[i] <= val[i-1] || !isUptrend && val[i] >= val[i-1] {
				trendStopped = true
				stopIndex = i
				break
			}
		}
		if !trendStopped {
			s.handlerTrendOn(isUptrend, current-firstGrayIndex+1)
		} else {
			s.handlerTrendOff(isUptrend, current-stopIndex+1)
		}
	}
}
