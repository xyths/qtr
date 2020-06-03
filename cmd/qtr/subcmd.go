package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/node"
)

var (
	gridCommand = &cli.Command{
		Action: grid,
		Name:   "grid",
		Usage:  "Trading with grid strategy",
	}
	historyCommand = &cli.Command{
		Name:  "history",
		Usage: "Manage trading history",
		Subcommands: []*cli.Command{
			{
				Action: pull,
				Name:   "pull",
				Usage:  "Pull trading history from exchange",
			},
			{
				Action: export,
				Name:   "export",
				Usage:  "Export trading history to csv",
				Flags: []cli.Flag{
					utils.LabelFlag,
					utils.StartTimeFlag,
					utils.EndTimeFlag,
					utils.CsvFlag,
				},
			},
		},
	}
	profitCommand = &cli.Command{
		Action: profit,
		Name:   "profit",
		Usage:  "Summary profit from trading history",
		Flags: []cli.Flag{
			utils.LabelFlag,
			utils.StartTimeFlag,
			utils.EndTimeFlag,
		},
	}
	snapshotCommand = &cli.Command{
		Action: snapshot,
		Name:   "snapshot",
		Usage:  "Snapshot the asset",
		Flags: []cli.Flag{
			utils.LabelFlag,
		},
	}
)

func grid(ctx *cli.Context) error {
	n := utils.GetNode(ctx)
	defer n.Close()
	return n.Grid(ctx.Context)
}

func pull(ctx *cli.Context) error {
	n := utils.GetNode(ctx)
	defer n.Close()
	return n.PullHistory(ctx.Context)
}

func profit(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	n := utils.GetNode(ctx)
	defer n.Close()
	return n.Profit(ctx.Context, label, start, end)
}

func snapshot(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	cfg := ctx.String(utils.ConfigFlag.Name)
	n := node.New(cfg)
	n.Init(ctx.Context)
	defer n.Close()
	return n.Snapshot(ctx.Context, label)
}

func export(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	csv := ctx.String(utils.CsvFlag.Name)
	n := utils.GetNode(ctx)
	defer n.Close()
	return n.Export(ctx.Context, label, start, end, csv)
}
