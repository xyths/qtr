package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/hs"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/trader/super"
)

func superTrend(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	cfg := super.Config{}
	if err := hs.ParseJsonConfig(configFile, &cfg); err != nil {
		return err
	}
	t, err := super.NewRestTraderFromConfig(ctx.Context, cfg)
	if err != nil {
		return err
	}
	if err := t.Init(ctx.Context); err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	t.Start(ctx.Context)
	return nil
}

func superTrendPrint(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	cfg := super.Config{}
	if err := hs.ParseJsonConfig(configFile, &cfg); err != nil {
		return err
	}
	t, err := super.NewRestTraderFromConfig(ctx.Context, cfg)
	if err != nil {
		return err
	}
	if err := t.Init(ctx.Context); err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	return t.Print(ctx.Context)
}

func superTrendClear(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	cfg := super.Config{}
	if err := hs.ParseJsonConfig(configFile, &cfg); err != nil {
		return err
	}
	dry := ctx.Bool(utils.DryRunFlag.Name)
	_ = dry
	t, err := super.NewRestTraderFromConfig(ctx.Context, cfg)
	if err != nil {
		return err
	}
	if err := t.Init(ctx.Context); err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	return t.Clear(ctx.Context)
}
