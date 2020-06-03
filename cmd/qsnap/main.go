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
		Usage:   "snapshot the asset of exchange",
		Version: "0.1.0",
	}

	app.Commands = []*cli.Command{
		huobiCommand,
		gateCommand,
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
