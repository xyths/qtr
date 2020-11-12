package main

import (
	"github.com/thrasher-corp/gocryptotrader/currency"
	"github.com/thrasher-corp/gocryptotrader/exchanges/asset"
	"github.com/urfave/cli/v2"
	"github.com/xyths/hs"
	"github.com/xyths/hs/logger"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/ta"
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
					utils.PeriodFlag,
					utils.StartTimeFlag,
					utils.EndTimeFlag,
					utils.OutputCsvFlag,
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
	cfgFile := ctx.String(utils.ConfigFlag.Name)
	cfg := ta.Config{}
	if err := hs.ParseJsonConfig(cfgFile, &cfg); err != nil {
		return err
	}
	period := ctx.String(utils.PeriodFlag.Name)
	d, err := time.ParseDuration(period)
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	symbol := ctx.String(SymbolFlag.Name)
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	output := ctx.String(utils.OutputCsvFlag.Name)
	startTime, endTime, err := utils.ParseStartEndTime(start, end)

	ex := getExchange(cfg.Exchange)
	if ex == nil {
		return nil
	}
	currencyPair := currency.NewPairFromString(symbol)
	interval := getInterval(d)
	item, err := ex.GetHistoricCandles(currencyPair, asset.Spot, startTime, endTime, interval)
	if err != nil {
		return err
	}
	return writeToCsv(item, output)
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
