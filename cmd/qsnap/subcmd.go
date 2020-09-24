package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/snapshot"
)

func snap(ctx *cli.Context) error {
	config := ctx.String(utils.ConfigFlag.Name)
	snap := snapshot.New(config)
	snap.Snapshot(ctx.Context)
	return nil
}
