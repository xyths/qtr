package super

import (
	"github.com/xyths/hs"
)

// SuperTrendConfig
type Config struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	Strategy StrategyConf
	Log      hs.LogConf
	Robots   []hs.BroadcastConf
}

type StrategyConf struct {
	Total     float64
	Interval  string
	Factor    float64
	Period    int
	StopLoss  bool `json:"stopLoss"`
	Reinforce float64
}
