package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"os"
	"path/filepath"
)

var app *cli.App

func init() {
	app = &cli.App{
		Name:    filepath.Base(os.Args[0]),
		Usage:   "the quantitative trading robot",
		Version: "0.14.4",
	}

	app.Commands = []*cli.Command{
		gridCommand,
		turtleCommand,
		superTrendCommand,
		taCommand,
		historyCommand,
		profitCommand,
		snapshotCommand,
	}
	app.Flags = []cli.Flag{
		utils.ConfigFlag,
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
