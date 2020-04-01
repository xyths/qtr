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
		Action: history,
		Name:   "history",
		Usage:  "Get trading history",
	}
	profitCommand = &cli.Command{
		Action: profit,
		Name:   "profit",
		Usage:  "Get trading history",
		Flags: []cli.Flag{
			utils.LabelFlag,
			utils.StartTimeFlag,
			utils.EndTimeFlag,
		},
	}
)

func grid(ctx *cli.Context) error {
	n := getNode(ctx)
	return n.Grid(ctx.Context)
}

func history(ctx *cli.Context) error {
	n := getNode(ctx)
	return n.History(ctx.Context)
}

func profit(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	n := getNode(ctx)
	return n.Profit(ctx.Context, label, start, end)
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
