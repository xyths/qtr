package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/snapshot"
)

var (
	huobiCommand = &cli.Command{
		Action: huobi,
		Name:   "huobi",
		Usage:  "Asset snapshot for huobi account",
		Flags: []cli.Flag{
			utils.ConfigFlag,
		},
	}
	gateCommand = &cli.Command{
		Action: gate,
		Name:   "gate",
		Usage:  "Asset snapshot for gate account",
		Flags: []cli.Flag{
			utils.ConfigFlag,
		},
	}
)

func huobi(ctx *cli.Context) error {
	config := ctx.String(utils.ConfigFlag.Name)
	snap := snapshot.New(config)
	snap.Do(ctx.Context)
	return nil
}

func gate(ctx *cli.Context) error {
	return nil
}
