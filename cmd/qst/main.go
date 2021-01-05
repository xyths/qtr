package main

import (
	"context"
	"fmt"
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var app *cli.App

func init() {
	app = &cli.App{
		Name:    filepath.Base(os.Args[0]),
		Action:  superTrend,
		Usage:   "the SuperTrend trading robot, use restful api",
		Version: "0.3.0",
	}

	app.Commands = []*cli.Command{
		{
			Action: superTrendPrint,
			Name:   "print",
			Usage:  "Print the SuperTrend state",
		},
		{
			Action: superTrendClear,
			Name:   "clear",
			Usage:  "clear all SuperTrend state in database, cancel pending orders",
			Flags: []cli.Flag{
				utils.DryRunFlag,
			},
		},
	}
	app.Flags = []cli.Flag{
		utils.ConfigFlag,
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	if err := app.RunContext(ctx, os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
