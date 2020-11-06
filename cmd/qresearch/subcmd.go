package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/hs"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/research"
)

var (
	superTrendCommand = &cli.Command{
		Action: superTrend,
		Name:   "super",
		Usage:  "Trading with grid strategy (RESTful API)",
		Flags: []cli.Flag{
			utils.InputCsvFlag,
			factorFlag,
			periodFlag,
			totalFlag,
			utils.StartTimeFlag,
			utils.EndTimeFlag,
			utils.OutputCsvFlag,
		},
	}
)

var (
	factorFlag = &cli.Float64Flag{
		Name:  "factor",
		Usage: "`factor` in SuperTrend",
	}
	periodFlag = &cli.IntFlag{
		Name:  "period",
		Usage: "`period` in SuperTrend",
	}
	totalFlag = &cli.Float64Flag{
		Name:  "total",
		Usage: "total money",
	}
)

func superTrend(ctx *cli.Context) error {
	cfgFile := ctx.String(utils.ConfigFlag.Name)
	cfg := research.Config{}
	if err := hs.ParseJsonConfig(cfgFile, &cfg); err != nil {
		return err
	}
	r := research.NewResearch(cfg)
	if err := r.Init(); err != nil {
		return err
	}

	input := ctx.String(utils.InputCsvFlag.Name)
	factor := ctx.Float64(factorFlag.Name)
	period := ctx.Int(periodFlag.Name)
	initial := ctx.Float64(totalFlag.Name)
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	startTime, endTime, err := utils.ParseStartEndTime(start, end)
	if err != nil {
		return err
	}
	output := ctx.String(utils.OutputCsvFlag.Name)
	return r.SuperTrend(input, factor, period, startTime, endTime, initial, output)
}
