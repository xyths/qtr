package main

import (
	"errors"
	"github.com/urfave/cli/v2"
	"github.com/xyths/hs"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/research"
	"time"
)

var (
	superTrendCommand = &cli.Command{
		Name:  "super",
		Usage: "Trading with grid strategy (RESTful API)",
		Subcommands: []*cli.Command{
			{
				Action: optimize,
				Name:   "optimize",
				Usage:  "Optimize the strategy parameters",
				Flags: []cli.Flag{
					utils.InputCsvFlag,
					factorsFlag,
					periodsFlag,
					totalFlag,
					utils.StartTimeFlag,
					utils.EndTimeFlag,
					utils.OutputCsvFlag,
				},
			},
			{
				Action: window,
				Name:   "window",
				Usage:  "Check the strategy parameters, by window the data",
				Flags: []cli.Flag{
					utils.InputCsvFlag,
					factorFlag,
					periodFlag,
					totalFlag,
					utils.StartTimeFlag,
					windowLengthFlag,
					windowStepFlag,
					utils.OutputCsvFlag,
				},
			},
		},
	}
)

var (
	factorFlag = &cli.Float64Flag{
		Name:  "factor",
		Usage: "`factor` in SuperTrend",
	}
	factorsFlag = &cli.Float64SliceFlag{
		Name:  "factors",
		Usage: "factors in SuperTrend",
		Value: cli.NewFloat64Slice(0.1, 0.1, 10),
	}
	periodFlag = &cli.IntFlag{
		Name:  "period",
		Usage: "`period` in SuperTrend",
	}
	periodsFlag = &cli.IntSliceFlag{
		Name:  "periods",
		Usage: "periods in SuperTrend",
		Value: cli.NewIntSlice(1, 1, 30),
	}
	totalFlag = &cli.Float64Flag{
		Name:  "total",
		Usage: "total money",
	}
	windowLengthFlag = &cli.StringFlag{
		Name:    "length",
		Aliases: []string{"l"},
		Usage:   "window length",
		Value:   "720h",
	}
	windowStepFlag = &cli.StringFlag{
		Name:  "step",
		Usage: "window step",
		Value: "24h",
	}
)

func optimize(ctx *cli.Context) error {
	cfgFile := ctx.String(utils.ConfigFlag.Name)
	cfg := research.Config{}
	if err := hs.ParseJsonConfig(cfgFile, &cfg); err != nil {
		return err
	}
	input := ctx.String(utils.InputCsvFlag.Name)
	factors := ctx.Float64Slice(factorsFlag.Name)
	if len(factors) != 3 {
		return errors.New("factors must has 3 value")
	}
	periods := ctx.IntSlice(periodsFlag.Name)
	if len(periods) != 3 {
		return errors.New("periods must has 3 value")
	}
	initial := ctx.Float64(totalFlag.Name)
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	startTime, endTime, err := utils.ParseStartEndTime(start, end)
	if err != nil {
		return err
	}
	output := ctx.String(utils.OutputCsvFlag.Name)
	r := research.NewResearch(cfg)
	if err := r.Init(); err != nil {
		return err
	}
	return r.SuperTrend(input, factors, periods, startTime, endTime, initial, output)
}

func window(ctx *cli.Context) error {
	cfgFile := ctx.String(utils.ConfigFlag.Name)
	cfg := research.Config{}
	if err := hs.ParseJsonConfig(cfgFile, &cfg); err != nil {
		return err
	}
	input := ctx.String(utils.InputCsvFlag.Name)
	factor := ctx.Float64(factorFlag.Name)
	period := ctx.Int(periodFlag.Name)
	initial := ctx.Float64(totalFlag.Name)
	start, err := utils.ParseBeijingTime(ctx.String(utils.StartTimeFlag.Name))
	if err != nil {
		return nil
	}
	length, err := time.ParseDuration(ctx.String(windowLengthFlag.Name))
	if err != nil {
		return nil
	}
	step, err := time.ParseDuration(ctx.String(windowStepFlag.Name))
	if err != nil {
		return nil
	}
	output := ctx.String(utils.OutputCsvFlag.Name)
	r := research.NewResearch(cfg)
	if err := r.Init(); err != nil {
		return err
	}

	return r.SuperTrendWindow(input, factor, period, start, length, step, initial, output)
}
