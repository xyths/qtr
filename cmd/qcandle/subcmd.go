package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"log"
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
		Action: export,
		Name:   "export",
		Usage:  "Export candle to csv",
		Flags: []cli.Flag{
			utils.StartTimeFlag,
			utils.EndTimeFlag,
			utils.OutputCsvFlag,
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

func export(ctx *cli.Context) error {
	n := utils.GetNode(ctx)
	defer n.Close()
	//return n.Export(ctx.Context)
	return nil
}
