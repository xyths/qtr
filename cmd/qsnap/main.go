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
		Usage:   "snapshot the asset of exchange account",
		Version: "0.1.4",
		Action:  snap,
	}
	app.Flags = []cli.Flag{
		utils.ConfigFlag,
	}
	app.Commands = []*cli.Command{
		logCommand,
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
