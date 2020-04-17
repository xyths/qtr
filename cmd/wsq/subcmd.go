package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
)

var (
	gridCommand = &cli.Command{
		Action: grid,
		Name:   "grid",
		Usage:  "Trading with grid strategy",
	}
	snapshotCommand = &cli.Command{
		Action: snapshot,
		Name:   "snapshot",
		Usage:  "Snapshot the asset",
		Flags: []cli.Flag{
			utils.LabelFlag,
		},
	}
	subscribeBalanceCommand = &cli.Command{
		Action: subscribeBalance,
		Name:   "subscribeBalance",
		Usage:  "subscribe balance",
		Flags: []cli.Flag{
			utils.LabelFlag,
		},
	}
)

func grid(ctx *cli.Context) error {
	n := utils.GetWsNode(ctx)
	defer n.Close()
	return n.Grid(ctx.Context)
}

func snapshot(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	n := utils.GetWsNode(ctx)
	defer n.Close()
	return n.Snapshot(ctx.Context, label)
}

func subscribeBalance(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	n := utils.GetWsNode(ctx)
	defer n.Close()
	return n.SubscribeBalance(ctx.Context, label)
}
