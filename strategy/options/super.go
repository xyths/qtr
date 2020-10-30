package options

import "github.com/xyths/qtr/strategy/params"

type CheckPeriod struct {
	Hour1 bool `json:"hour1"`
	Hour4 bool `json:"hour4"`
	Day   bool `json:"day"`
	Week  bool `json:"week"`
	Month bool `json:"month"`
}
type SuperTrendStrategyOption struct {
	params.SuperTrendParam
	CheckPeriod
}

type SqueezeStrategyOption struct {
	params.SqueezeParam
	CheckPeriod
}
