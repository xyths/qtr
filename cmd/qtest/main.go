package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"os"
	"path/filepath"
)

var app *cli.App

func init() {
	app = &cli.App{
		Name:    filepath.Base(os.Args[0]),
		Usage:   "test kline data for specific strategy",
		Version: "0.1.0",
	}

	app.Commands = []*cli.Command{
		//gridCommand,
		//turtleCommand,
		superTrendCommand,
		//taCommand,
		//historyCommand,
		//profitCommand,
		//snapshotCommand,
	}
	//app.Flags = []cli.Flag{
	//	utils.ConfigFlag,
	//}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
