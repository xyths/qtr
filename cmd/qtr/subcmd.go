package main

import (
	"github.com/urfave/cli/v2"
	"github.com/xyths/hs"
	"github.com/xyths/hs/logger"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/history"
	"github.com/xyths/qtr/node"
	"github.com/xyths/qtr/ta"
	"github.com/xyths/qtr/ta/atr"
	"github.com/xyths/qtr/trader/rest/grid"
	"github.com/xyths/qtr/trader/rest/turtle"
	"github.com/xyths/qtr/trader/super"
	"github.com/xyths/qtr/trader/ws"
	"time"
)

var (
	gridCommand = &cli.Command{
		Action: gridAction,
		Name:   "grid",
		Usage:  "Trading with grid strategy (RESTful API)",
		Subcommands: []*cli.Command{
			{
				Action: printGrid,
				Name:   "print",
				Usage:  "Print the grid generated by strategy parameters",
			},
			{
				Action: rebalanceGrid,
				Name:   "rebalance",
				Usage:  "rebalance before run the grid generated",
				Flags: []cli.Flag{
					utils.DryRunFlag,
				},
			},
			{
				Action: clearGrid,
				Name:   "clear",
				Usage:  "clear all grids, cancel orders and reset base",
				Flags: []cli.Flag{
					utils.DryRunFlag,
				},
			},
		},
	}
	multiGridCommand = &cli.Command{
		Action:  mGridAction,
		Name:    "mgrid",
		Aliases: []string{"mg"},
		Usage:   "Trading with grid strategy (RESTful API)",
		Subcommands: []*cli.Command{
			{
				Action: mGridPrint,
				Name:   "print",
				Usage:  "Print the grid generated by strategy parameters",
			},
			{
				Action: mGridRebalance,
				Name:   "rebalance",
				Usage:  "rebalance before run the grid generated",
				Flags: []cli.Flag{
					utils.DryRunFlag,
				},
			},
			{
				Action: mGridClear,
				Name:   "clear",
				Usage:  "clear all grids, cancel orders and reset base",
				Flags: []cli.Flag{
					utils.DryRunFlag,
				},
			},
		},
		Flags: []cli.Flag{
			utils.DryRunFlag,
		},
	}
	turtleCommand = &cli.Command{
		Action: turtleAction,
		Name:   "turtle",
		Usage:  "Trading with turtle strategy",
		Subcommands: []*cli.Command{
			{
				Action: turtlePrintAction,
				Name:   "print",
				Usage:  "Print the turtle generated by strategy parameters",
			},
			{
				Action: turtleClearAction,
				Name:   "clear",
				Usage:  "clear all turtle state in database, cancel pending orders",
				//Flags: []cli.Flag{
				//	utils.DryRunFlag,
				//},
			},
		},
	}
	superTrendCommand = &cli.Command{
		Action: superTrend,
		Name:   "super",
		Usage:  "Trading with SuperTrend strategy",
		Subcommands: []*cli.Command{
			{
				Action: superTrendPrint,
				Name:   "print",
				Usage:  "Print the SuperTrend state",
			},
			{
				Action: superTrendClear,
				Name:   "clear",
				Usage:  "clear all SuperTrend state in database, cancel pending orders",
				Flags: []cli.Flag{
					utils.DryRunFlag,
				},
			},
		},
		Flags: []cli.Flag{
			utils.DryRunFlag,
		},
	}
	sniperCommand = &cli.Command{
		Action: sniper,
		Name:   "sniper",
		Usage:  "Trading with Sniper strategy",
		Subcommands: []*cli.Command{
			{
				Action: sniperPrint,
				Name:   "print",
				Usage:  "Print the Sniper strategy state",
			},
			{
				Action: sniperClear,
				Name:   "clear",
				Usage:  "clear all Sniper state in database, cancel pending orders",
				Flags: []cli.Flag{
					utils.DryRunFlag,
				},
			},
		},
		Flags: []cli.Flag{
			utils.ProtocolFlag,
			utils.DryRunFlag,
		},
	}
	rtmCommand = &cli.Command{
		Action: rtm,
		Name:   "rtm",
		Usage:  "Trading with RTM(reversion to the mean) strategy",
		Subcommands: []*cli.Command{
			{
				Action: rtmPrint,
				Name:   "print",
				Usage:  "Print the RTM state",
			},
			{
				Action: rtmClear,
				Name:   "clear",
				Usage:  "clear all RTM state in database, cancel pending orders",
				Flags: []cli.Flag{
					utils.DryRunFlag,
				},
			},
		},
		Flags: []cli.Flag{
			utils.ProtocolFlag,
			utils.DryRunFlag,
		},
	}
	taCommand = &cli.Command{
		Name:  "ta",
		Usage: "Technical analysis on cryptocurrency",
		Flags: []cli.Flag{
			utils.StartTimeFlag,
			utils.EndTimeFlag,
			utils.PeriodFlag,
			utils.SizeFlag,
			utils.OutputCsvFlag,
		},
		Subcommands: []*cli.Command{
			{
				Action: atrFunc,
				Name:   "atr",
				Usage:  "ATR(Average True Range)",
			},
			{
				Action: natrFunc,
				Name:   "natr",
				Usage:  "NATR(Normalized Average True Range)",
			},
			{
				Action: superFunc,
				Name:   "super",
				Usage:  "SuperTrend indicator values in detail for one symbol",
			},
			{
				Action: boll,
				Name:   "boll",
				Usage:  "Bollinger Bands",
			},
		},
	}
	scanCommand = &cli.Command{
		Name:  "scan",
		Usage: "Scan cryptocurrencies on Exchange by Technical analysis indicators",
		Flags: []cli.Flag{
			utils.OutputCsvFlag,
			utils.SizeFlag,
			utils.ScanMonthlyFlag,
			utils.ScanWeeklyFlag,
			utils.ScanDailyFlag,
			utils.Scan4HFlag,
			utils.ScanHourlyFlag,
		},
		Subcommands: []*cli.Command{
			{
				Action: superScan,
				Name:   "super",
				Usage:  "Scan by SuperTrend indicator",
			},
			{
				Action: squeezeScan,
				Name:   "squeeze",
				Usage:  "Scan by Squeeze indicator",
			},
			{
				Action: gridScan,
				Name:   "grid",
				Usage:  "Scan for grid strategy, filter by SuperTrend and Squeeze indicator",
			},
		},
	}
	historyCommand = &cli.Command{
		Name:  "history",
		Usage: "Manage trading history",
		Subcommands: []*cli.Command{
			{
				Action: pull,
				Name:   "pull",
				Usage:  "Pull trading history from exchange",
			},
			{
				Action:      export,
				Name:        "export",
				Usage:       "Export trading history to csv",
				Description: description,
				Flags: []cli.Flag{
					utils.StartTimeFlag,
					utils.EndTimeFlag,
					utils.OutputCsvFlag,
				},
			},
		},
	}
	profitCommand = &cli.Command{
		Action: profit,
		Name:   "profit",
		Usage:  "Summary profit from trading history",
		Flags: []cli.Flag{
			utils.LabelFlag,
			utils.StartTimeFlag,
			utils.EndTimeFlag,
		},
	}
	snapshotCommand = &cli.Command{
		Action: snapshot,
		Name:   "snapshot",
		Usage:  "Snapshot the asset",
		Flags: []cli.Flag{
			utils.LabelFlag,
		},
	}
	//candleCommand = &cli.Command{
	//	Action: candle,
	//	Name:   "candle",
	//	Usage:  "download the candlestick",
	//	Flags: []cli.Flag{
	//		utils.LabelFlag,
	//		utils.StartTimeFlag,
	//		utils.EndTimeFlag,
	//		utils.OutputCsvFlag,
	//	},
	//}
)

const description = "Export the account's (in config file) trading history, start from `start` to`end` (closed interval, [`start`, `end`]), save items to `csv` file."

func gridAction(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	g := grid.New(configFile)
	g.Init(ctx.Context)
	defer g.Close(ctx.Context)
	return g.Start(ctx.Context)
}

func printGrid(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	g := grid.New(configFile)
	g.Init(ctx.Context)
	defer g.Close(ctx.Context)
	return g.Print(ctx.Context)
}

func rebalanceGrid(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	dryRun := ctx.Bool(utils.DryRunFlag.Name)
	g := grid.New(configFile)
	g.Init(ctx.Context)
	defer g.Close(ctx.Context)
	_ = g.Print(ctx.Context)
	return g.ReBalance(ctx.Context, dryRun)
}

func clearGrid(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	dryRun := ctx.Bool(utils.DryRunFlag.Name)
	g := grid.New(configFile)
	g.Init(ctx.Context)
	defer g.Close(ctx.Context)
	return g.Clear(ctx.Context, dryRun)
}

func mGridAction(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	dry := ctx.Bool(utils.DryRunFlag.Name)
	t, err := grid.NewMultipleGridTrader(ctx.Context, configFile, dry)
	if err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	if err := t.Start(ctx.Context); err != nil {
		return err
	}
	<-ctx.Done()
	if err := t.Stop(ctx.Context); err != nil {
		return err
	}
	return nil
}

func mGridPrint(ctx *cli.Context) error {
	return nil
}
func mGridRebalance(ctx *cli.Context) error {
	return nil
}
func mGridClear(ctx *cli.Context) error {
	return nil
}

func pull(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	h := history.New(configFile)
	h.Init(ctx.Context)
	defer h.Close(ctx.Context)
	return h.Pull(ctx.Context)
}

func export(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	h := history.New(configFile)
	h.Init(ctx.Context)
	defer h.Close(ctx.Context)

	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	csv := ctx.String(utils.OutputCsvFlag.Name)

	return h.Export(ctx.Context, start, end, csv)
}

func profit(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	n := utils.GetNode(ctx)
	defer n.Close()
	return n.Profit(ctx.Context, label, start, end)
}

func snapshot(ctx *cli.Context) error {
	label := ctx.String(utils.LabelFlag.Name)
	cfg := ctx.String(utils.ConfigFlag.Name)
	n := node.New(cfg)
	n.Init(ctx.Context)
	defer n.Close()
	return n.Snapshot(ctx.Context, label)
}

func atrFunc(ctx *cli.Context) error {
	start := ctx.String(utils.StartTimeFlag.Name)
	end := ctx.String(utils.EndTimeFlag.Name)
	startTime, endTime, err := utils.ParseStartEndTime(start, end)
	if err != nil {
		logger.Sugar.Fatal(err)
	}
	return atr.All("gate", []string{"BTC_USDT", "ETH_USDT", "EOS_USDT"}, startTime, endTime)
}

func natrFunc(ctx *cli.Context) error {
	cfgFile := ctx.String(utils.ConfigFlag.Name)
	cfg := ta.Config{}
	if err := hs.ParseJsonConfig(cfgFile, &cfg); err != nil {
		return err
	}
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
	return agent.NATR(ctx.Context, ctx.Args().Slice(), startTime, endTime, d, output)
}

func boll(ctx *cli.Context) error {
	return nil
}

func turtleAction(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	t := turtle.New(configFile)
	t.Init(ctx.Context)
	defer t.Close(ctx.Context)
	return t.Start(ctx.Context)
}

func turtlePrintAction(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	t := turtle.New(configFile)
	t.Init(ctx.Context)
	defer t.Close(ctx.Context)
	return t.Print(ctx.Context)
}

func turtleClearAction(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	t := turtle.New(configFile)
	t.Init(ctx.Context)
	defer t.Close(ctx.Context)
	return t.Clear(ctx.Context)
}

func superTrend(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	dry := ctx.Bool(utils.DryRunFlag.Name)
	t, err := super.NewWsTrader(ctx.Context, configFile)
	if err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	t.Start(ctx.Context, dry)
	<-ctx.Done()
	t.Stop()
	return nil
}

func superTrendPrint(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	t, err := super.NewWsTrader(ctx.Context, configFile)
	if err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	return t.Print(ctx.Context)
}

func superTrendClear(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	t, err := super.NewWsTrader(ctx.Context, configFile)
	if err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	return t.Clear(ctx.Context)
}

func sniper(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	dry := ctx.Bool(utils.DryRunFlag.Name)
	t, err := super.NewWsTrader(ctx.Context, configFile)
	if err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	t.Start(ctx.Context, dry)
	<-ctx.Done()
	t.Stop()
	return nil
}

func sniperPrint(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	t, err := super.NewWsTrader(ctx.Context, configFile)
	if err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	return t.Print(ctx.Context)
}

func sniperClear(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	t, err := super.NewWsTrader(ctx.Context, configFile)
	if err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	return t.Clear(ctx.Context)
}

func rtm(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	dry := ctx.Bool(utils.DryRunFlag.Name)
	t, err := ws.NewRtmTrader(ctx.Context, configFile, dry)
	if err != nil {
		return err
	}
	defer t.Close(ctx.Context)
	t.Start(ctx.Context, dry)
	<-ctx.Done()
	t.Stop()
	return nil
}

func rtmPrint(ctx *cli.Context) error {
	//configFile := ctx.String(utils.ConfigFlag.Name)
	//t, err := super.NewWsTrader(ctx.Context, configFile)
	//if err != nil {
	//	return err
	//}
	//defer t.Close(ctx.Context)
	//return t.Print(ctx.Context)
	return nil
}

func rtmClear(ctx *cli.Context) error {
	//configFile := ctx.String(utils.ConfigFlag.Name)
	//t, err := super.NewWsTrader(ctx.Context, configFile)
	//if err != nil {
	//	return err
	//}
	//defer t.Close(ctx.Context)
	//return t.Clear(ctx.Context)
	return nil
}
