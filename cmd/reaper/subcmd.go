package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/hs"
	"github.com/xyths/qtr/cmd/utils"
	reaper2 "github.com/xyths/qtr/reaper"
)

func reaper(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	cfg := reaper2.Config{}
	if err := hs.ParseJsonConfig(configFile, &cfg); err != nil {
		return err
	}
	r := reaper2.New(cfg)
	if err := r.Init(ctx.Context); err != nil {
		return err
	}
	defer r.Close(ctx.Context)
	if err := r.Start(ctx.Context); err != nil {
		return err
	}
	defer r.Stop(ctx.Context)

	<-ctx.Done()

	return nil
}

func print(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	cfg := reaper2.Config{}
	if err := hs.ParseJsonConfig(configFile, &cfg); err != nil {
		return err
	}
	r := reaper2.New(cfg)
	if err := r.Init(ctx.Context); err != nil {
		return err
	}
	defer r.Close(ctx.Context)
	return r.Print(ctx.Context)
}

func clear(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	cfg := reaper2.Config{}
	if err := hs.ParseJsonConfig(configFile, &cfg); err != nil {
		return err
	}
	r := reaper2.New(cfg)
	if err := r.Init(ctx.Context); err != nil {
		return err
	}
	defer r.Close(ctx.Context)
	return r.Clear(ctx.Context)
}
