package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/go-indicators"
	"github.com/xyths/hs/logger"
	"github.com/xyths/qtr/cmd/utils"
	"time"
)

var (
	superTrendCommand = &cli.Command{
		Action: superTrend,
		Name:   "super",
		Usage:  "Trading with grid strategy (RESTful API)",
		Flags: []cli.Flag{
			utils.InputCsvFlag,
			factorFlag,
			periodFlag,
			totalFlag,
			utils.StartTimeFlag,
			utils.EndTimeFlag,
		},
	}
)

var (
	factorFlag = &cli.Float64Flag{
		Name:  "factor",
		Usage: "`factor` in SuperTrend",
	}
	periodFlag = &cli.IntFlag{
		Name:  "period",
		Usage: "`period` in SuperTrend",
	}
	totalFlag = &cli.Float64Flag{
		Name:  "total",
		Usage: "total money",
	}
)

func superTrend(ctx *cli.Context) error {
	filename := ctx.String(utils.InputCsvFlag.Name)
	factor := ctx.Float64(factorFlag.Name)
	period := ctx.Int(periodFlag.Name)
	initial := ctx.Float64(totalFlag.Name)
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	start, _ := time.ParseInLocation(layout, ctx.String(utils.StartTimeFlag.Name), beijing)
	end, _ := time.ParseInLocation(layout, ctx.String(utils.EndTimeFlag.Name), beijing)

	timestamp, open, high, low, close := readData(filename)
	tsl, trend := indicator.SuperTrend(factor, period, high, low, close)

	signal := make([]int, len(trend))
	cash := initial
	coin := 0.0
	for i := 0; i < len(trend); i++ {
		realtime := time.Unix(timestamp[i], 0)
		timeStr := realtime.In(beijing).Format(layout)
		logger.Sugar.Infof("%s %f %f %f %f, %f %v", timeStr, open[i], high[i], low[i], close[i], tsl[i], trend[i])
		if !realtime.Before(start) && !realtime.After(end) {
			if trend[i] && !trend[i-1] {
				signal[i] = 1
				amount := cash / close[i]
				coin += amount
				cash = 0
				logger.Sugar.Infow("Buy", "time", timeStr, "price", close[i], "amount", amount, "cash", cash, "coin", coin)
			} else if !trend[i] && trend[i-1] {
				signal[i] = -1
				amount := coin
				cash += coin * close[i]
				coin = 0
				logger.Sugar.Infow("Sell", "time", timeStr, "price", close[i], "amount", amount, "cash", cash, "coin", coin)
			}
		}
	}
	final := cash + coin*close[len(close)-1]
	rate := (final - initial) / initial
	rate2 := rate * (24 * 365 / end.Sub(start).Hours())
	logger.Sugar.Infof("Factor: %f, Period: %d, Initial: %f, Final: %f, Rate: %.4f / %.4f",
		factor, period, initial, final, rate, rate2)
	return nil
}
