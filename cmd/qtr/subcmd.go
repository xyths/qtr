package main

import (
	"encoding/json"
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/node"
	"log"
	"os"
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
	n := getNode(ctx)
	return n.Grid(ctx.Context)
}

func pull(ctx *cli.Context) error {
	n := getNode(ctx)
	return n.PullHistory(ctx.Context)
}

func profit(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	n := getNode(ctx)
	return n.Profit(ctx.Context, label, start, end)
}

func snapshot(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	n := getNode(ctx)
	return n.Snapshot(ctx.Context, label)
}

func export(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	csv := ctx.String(utils.CsvFlag.Name)
	n := getNode(ctx)
	return n.Export(ctx.Context, label, start, end, csv)
}

func parseConfig(filename string) (c node.Config) {
	configFile, err := os.Open(filename)
	defer func() {
		_ = configFile.Close()
	}()
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(configFile).Decode(&c)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func getNode(ctx *cli.Context) node.Node {
	c := parseConfig(ctx.String(utils.ConfigFlag.Name))
	n := node.Node{}
	n.Init(ctx.Context, c)
	return n
}
