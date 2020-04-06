package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
)

var (
	downloadCommand = &cli.Command{
		Action: download,
		Name:   "download",
		Usage:  "Download candle from exchange",
	}
	exportCommand = &cli.Command{
		Action: export,
		Name:   "export",
		Usage:  "Export candle to csv",
		Flags: []cli.Flag{
			utils.StartTimeFlag,
			utils.EndTimeFlag,
			utils.CsvFlag,
		},
	}
)

func download(ctx *cli.Context) error {
	//n := getNode(ctx)
	//return n.Grid(ctx.Context)
	return nil
}

func export(ctx *cli.Context) error {
	//n := getNode(ctx)
	//return n.Grid(ctx.Context)
	return nil
}
