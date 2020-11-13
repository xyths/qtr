package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"github.com/xyths/hs"
	"github.com/xyths/hs/logger"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/ta"
	"time"
)

func superFunc(ctx *cli.Context) error {
	cfgFile := ctx.String(utils.ConfigFlag.Name)
	cfg := ta.Config{}
	if err := hs.ParseJsonConfig(cfgFile, &cfg); err != nil {
		return err
	}
	//start := ctx.String(utils.StartTimeFlag.Name)
	//end := ctx.String(utils.EndTimeFlag.Name)
	period := ctx.String(utils.PeriodFlag.Name)
	size:=ctx.Int64(utils.SizeFlag.Name)
	output := ctx.String(utils.OutputCsvFlag.Name)
	//startTime, endTime, err := utils.ParseStartEndTime(start, end)
	//if err != nil {
	//	logger.Sugar.Fatal(err)
	//}
	d, err := time.ParseDuration(period)
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	agent := ta.NewAgent(cfg)
	if err := agent.Init(); err != nil {
		return err
	}
	for i, symbol := range ctx.Args().Slice() {
		if err := agent.SuperTrendSingle(ctx.Context, symbol, size, d, fmt.Sprintf("%s.%d.%s", output, i, symbol)); err != nil {
			return err
		}
	}
	return nil
}
