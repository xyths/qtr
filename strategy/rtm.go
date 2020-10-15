package strategy

import (
	"github.com/markcheno/go-talib"
	indicator "github.com/xyths/go-indicators"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/qtr/exchange/huobi"
	"go.uber.org/zap"
	"math"
	"sync"
	"time"
)

type RtmExecutor interface {
	Exchange() exchange.Exchange
	Symbol() string
	//SubscribeCandle(clientId string, period time.Duration, responseHandler func(interface{}))

	Start()
	Stop()

	BuyAllLimit(price float64) error
	SellAllLimit(price float64) error
	BuyAllMarket() error
	SellAllMarket() error
	CancelAll() error
	CancelAllBuy() error
	CancelAllSell() error
}

type RtmStrategyConf struct {
	Total    float64
	Interval string
	Period   int
	Factor   float64
	Squeeze  SqueezeStrategyConf
}

type RTMStrategy struct {
	config   RtmStrategyConf
	interval time.Duration
	symbol   string
	dry      bool

	Sugar    *zap.SugaredLogger
	executor RtmExecutor
	squeeze  *SqueezeWs

	enabledLock sync.RWMutex
	enabled     bool

	candle hs.Candle

	mean      float64
	upper     float64
	lower     float64
	atr       float64
	trend     bool
	sellPrice float64
}

func NewRTMStrategy(config RtmStrategyConf, dry bool) *RTMStrategy {
	return &RTMStrategy{
		config:  config,
		squeeze: NewSqueezeWs(config.Squeeze, dry),
		candle:  hs.NewCandle(2000),
	}
}

func (s *RTMStrategy) Init(logger *zap.SugaredLogger, ex RtmExecutor) {
	interval, err := time.ParseDuration(s.config.Interval)
	if err != nil {
		panic(err)
	}
	s.interval = interval

	s.squeeze.Init(logger, ex.Exchange(), ex.Symbol(), s.SqueezeOn, s.TrendOn, s.TrendOff)
	s.Sugar = logger
	s.executor = ex
	s.symbol = ex.Symbol()
}

func (s *RTMStrategy) Start() {
	s.executor.Start()
	// start squeeze first
	s.squeeze.Start()

	{
		to := time.Now()
		from := to.Add(-2000 * s.interval)
		candle, err := s.executor.Exchange().CandleFrom(s.symbol, "rtm-candle", s.interval, from, to)
		if err != nil {
			s.Sugar.Fatalf("get rtm candle error: %s", err)
		}
		s.candle.Add(candle)
		// force to check trend on startup
		s.onTick(s.dry, candle, true)
	}
	s.executor.Exchange().SubscribeCandlestick(s.symbol, "rtm-tick", s.interval, s.tickerHandler)
}

func (s *RTMStrategy) Stop() {
	s.executor.Exchange().UnsubscribeCandlestick(s.symbol, "rtm-tick", s.interval)
	s.squeeze.Stop()
	s.executor.Stop()
}

// when squeeze is fire off, stop new orders, waiting for sell out
func (s *RTMStrategy) SqueezeOn(last int, dry bool) {
	if s.IsEnabled() {
		s.Disable()
	}
}

// trend is on, cancel opening orders and sell all coins
func (s *RTMStrategy) TrendOn(up bool, last int, dry bool) {
	if s.IsEnabled() {
		s.Disable()
	}

	// cancel all orders
	s.executor.CancelAll()
	// sell all coins
	s.executor.SellAllMarket()
}

// start to RTM trading
func (s *RTMStrategy) TrendOff(up bool, last int, dry bool) {
	if !s.IsEnabled() {
		s.Enable()
	}
}

func (s *RTMStrategy) Enable() {
	s.enabledLock.Lock()
	defer s.enabledLock.Unlock()
	s.enabled = true
}
func (s *RTMStrategy) Disable() {
	s.enabledLock.Lock()
	defer s.enabledLock.Unlock()
	s.enabled = false
}
func (s *RTMStrategy) IsEnabled() bool {
	s.enabledLock.RLock()
	defer s.enabledLock.RUnlock()
	return s.enabled
}

func (s *RTMStrategy) tickerHandler(resp interface{}) {
	//s.Sugar.Debug("rtm got candle update")
	ticker, _, err := huobi.CandlestickHandler(resp)
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
				finished = true
			}
		} else {
			s.Sugar.Info("candle is not ready for append")
		}
		c2 := s.candle
		s.onTick(s.dry, c2, finished)
	}
}

func (s *RTMStrategy) onTick(dry bool, candle hs.Candle, finished bool) {
	//s.Sugar.Debug("onTick")
	if !s.IsEnabled() {
		s.Sugar.Debug("RTM strategy disabled")
		return
	}
	if dry {
		s.Sugar.Debug("strategy is dry run")
	}

	s.Sugar.Debug("RTM strategy processing")
	l := candle.Length()
	current := l - 2
	previous := current - 1

	if finished {
		mean, upper, lower, atr := indicator.Rtm(
			s.config.Period, s.config.Period, s.config.Factor,
			candle.High, candle.Low, candle.Close,
		)
		mean2 := talib.LinearReg(mean, s.config.Period)
		s.mean = mean[current]
		s.upper = upper[current]
		s.lower = lower[current]
		s.atr = atr[current]
		oldSellPrice := s.sellPrice
		s.sellPrice = s.mean + s.atr
		if mean2[current] > mean2[previous] {
			s.trend = true
		} else {
			// must reset, in case trend turns down from up
			s.trend = false
		}

		// always update sell order
		if s.sellPrice != oldSellPrice {
			s.executor.CancelAllSell()
			s.executor.SellAllLimit(s.sellPrice)
		}
	}

	// 1. upper trend, 2. lower than average
	if s.trend {
		// up trend
		if candle.Close[l-1] < s.mean {
			// latest close is less than mean
			target := math.Min(candle.Close[l-1], s.mean-0.5*s.atr)
			if !dry {
				s.executor.CancelAllBuy()
				s.executor.BuyAllLimit(target)
			}
			s.Sugar.Infof("long at price %f", target)
		}
	} else {
		// down trend
		s.Sugar.Debug("it's down trend, no trade")
	}
}
