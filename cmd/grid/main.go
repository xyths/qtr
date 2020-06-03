package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"log"
	"os"
	"path/filepath"
)

var app *cli.App

func init() {
	log.SetFlags(log.Ldate | log.Ltime)

	app = &cli.App{
		Name:    filepath.Base(os.Args[0]),
		Usage:   "the grid trading robot using RESTful API",
		Version: "0.1.0",
		Action:  th,
	}

	app.Commands = []*cli.Command{
		thCommand,
		//historyCommand,
		//profitCommand,
		//snapshotCommand,
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
