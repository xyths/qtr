package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/hs"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/ta"
)

func superScan(ctx *cli.Context) error {
	cfgFile := ctx.String(utils.ConfigFlag.Name)
	cfg := ta.Config{}
	if err := hs.ParseJsonConfig(cfgFile, &cfg); err != nil {
		return err
	}
	output := ctx.String(utils.OutputCsvFlag.Name)
	size := ctx.Int64(utils.SizeFlag.Name)
	monthly := ctx.Bool(utils.ScanMonthlyFlag.Name)
	weekly := ctx.Bool(utils.ScanWeeklyFlag.Name)
	daily := ctx.Bool(utils.ScanDailyFlag.Name)
	h4 := ctx.Bool(utils.Scan4HFlag.Name)
	h1 := ctx.Bool(utils.ScanHourlyFlag.Name)
	agent := ta.NewAgent(cfg)
	if err := agent.Init(); err != nil {
		return err
	}
	return agent.SuperTrend(ctx.Context, ctx.Args().Slice(), size, monthly, weekly, daily, h4, h1, output)
}

func squeezeScan(ctx *cli.Context) error {
	cfgFile := ctx.String(utils.ConfigFlag.Name)
	cfg := ta.Config{}
	if err := hs.ParseJsonConfig(cfgFile, &cfg); err != nil {
		return err
	}
	output := ctx.String(utils.OutputCsvFlag.Name)
	size := ctx.Int64(utils.SizeFlag.Name)
	monthly := ctx.Bool(utils.ScanMonthlyFlag.Name)
	weekly := ctx.Bool(utils.ScanWeeklyFlag.Name)
	daily := ctx.Bool(utils.ScanDailyFlag.Name)
	h4 := ctx.Bool(utils.Scan4HFlag.Name)
	h1 := ctx.Bool(utils.ScanHourlyFlag.Name)
	agent := ta.NewAgent(cfg)
	if err := agent.Init(); err != nil {
		return err
	}
	return agent.Squeeze(ctx.Context, ctx.Args().Slice(), size, monthly, weekly, daily, h4, h1, output)
}

func gridScan(ctx *cli.Context) error {
	cfgFile := ctx.String(utils.ConfigFlag.Name)
	cfg := ta.Config{}
	if err := hs.ParseJsonConfig(cfgFile, &cfg); err != nil {
		return err
	}
	output := ctx.String(utils.OutputCsvFlag.Name)
	size := ctx.Int64(utils.SizeFlag.Name)
	agent := ta.NewAgent(cfg)
	if err := agent.Init(); err != nil {
		return err
	}
	return agent.Grid(ctx.Context, ctx.Args().Slice(), size, output)
}
