package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/hs"
	"github.com/xyths/hs/logger"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/ta"
	"log"
	"time"
)

var (
	gateCommand = &cli.Command{
		Name:  "gate",
		Usage: "Download candle from exchange",
		Subcommands: []*cli.Command{
			{
				Action: gateCandlestick,
				Name:   "candlestick",
				Usage:  "candlestick of gate",
				Flags: []cli.Flag{
					SymbolFlag,
					TypeFlag,
					utils.StartTimeFlag,
					utils.EndTimeFlag,
					utils.OutputCsvFlag,
					HostFlag,
				},
			},
		},
	}
	huobiCommand = &cli.Command{
		Name:  "huobi",
		Usage: "Download candle from Huobi exchange",
		Subcommands: []*cli.Command{
			{
				Action: huobiDownload,
				Name:   "download",
				Usage:  "download Huobi candlestick to csv",
				Flags: []cli.Flag{
					SymbolFlag,
					utils.PeriodFlag,
					utils.StartTimeFlag,
					utils.EndTimeFlag,
					utils.OutputCsvFlag,
				},
			},
		},
	}
)

func gateCandlestick(ctx *cli.Context) error {
	symbol := ctx.String(SymbolFlag.Name)
	cType := ctx.String(TypeFlag.Name)
	startTime := ctx.String(utils.StartTimeFlag.Name)
	endTime := ctx.String(utils.EndTimeFlag.Name)
	host := ctx.String(HostFlag.Name)
	log.Printf("symbol: %s, type: %s,  start: %s, end: %s, host: %s", symbol, cType, startTime, endTime, host)

	// use hs.gateio.CandleFrom
	//g := gateio.New("", "", host)
	//candles, err := g.Candles(symbol, groupSec, rangeHour)
	//if err != nil {
	//	log.Printf("get candle error: %s", err)
	//}
	//for _, c := range candles {
	//	log.Printf("%d,%s,%s,%s,%s,%s", c.Timestamp, c.Open, c.High, c.Low, c.Close, c.Volume)
	//}
	return nil
}

func huobiDownload(ctx *cli.Context) error {
	cfgFile := ctx.String(utils.ConfigFlag.Name)
	cfg := ta.Config{}
	if err := hs.ParseJsonConfig(cfgFile, &cfg); err != nil {
		return err
	}
	symbol := ctx.String(SymbolFlag.Name)
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	period := ctx.String(utils.PeriodFlag.Name)
	output := ctx.String(utils.OutputCsvFlag.Name)
	startTime, endTime, err := utils.ParseStartEndTime(start, end)
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	d, err := time.ParseDuration(period)
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	agent := ta.NewAgent(cfg)
	if err := agent.Init(); err != nil {
		return err
	}
	if err := agent.DownloadFrom(ctx.Context, symbol, startTime, endTime, d, output); err != nil {
		return err
	}

	return nil
}
