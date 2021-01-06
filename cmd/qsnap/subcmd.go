package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/snapshot"
)

var (
	logCommand = &cli.Command{
		Action: snapLog,
		Name:   "log",
		Usage:  "Log balance to file",
	}
)

func snapLog(ctx *cli.Context) error {
	config := ctx.String(utils.ConfigFlag.Name)
	snap := snapshot.New(config)
	snap.Log()
	return nil
}
