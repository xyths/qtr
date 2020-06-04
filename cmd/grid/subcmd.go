package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/gridh"
)

var (
	thCommand = &cli.Command{
		Action: th,
		Name:   "th",
		Usage:  "Trading using hedging grid strategy",
		Subcommands: []*cli.Command{
			{
				Action: printGrid,
				Name:   "print",
				Usage:  "Print the grid generated by strategy parameters",
			},
		},
	}
)

func th(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	g := gridh.New(configFile)
	g.Init(ctx.Context)
	defer g.Close(ctx.Context)
	return g.Trade(ctx.Context)
}

func printGrid(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	g := gridh.New(configFile)
	g.Init(ctx.Context)
	defer g.Close(ctx.Context)
	return g.Print(ctx.Context)
}