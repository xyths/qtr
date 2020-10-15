package utils

import "github.com/urfave/cli/v2"

var (
	ConfigFlag = &cli.StringFlag{
		Name:    "config",
		Aliases: []string{"c"},
		Value:   "config.json",
		Usage:   "load configuration from `file`",
	}

	LabelFlag = &cli.StringFlag{
		Name:    "label",
		Aliases: []string{"l"},
		Value:   "",
		Usage:   "only process the user with `label` (default: \"\", for all users)",
	}
	StartTimeFlag = &cli.StringFlag{
		Name:    "start",
		Aliases: []string{"s"},
		Value:   "",
		Usage:   "start `time` (e.g. \"2020-04-01 00:00:00\", default: yesterday)",
	}
	EndTimeFlag = &cli.StringFlag{
		Name:    "end",
		Aliases: []string{"e"},
		Value:   "",
		Usage:   "end `time` (e.g. \"2020-04-01 23:59:59\", default: now)",
	}
	ProtocolFlag = &cli.StringFlag{
		Name:    "protocol",
		Aliases: []string{"p"},
		Value:   "r",
		Usage:   "protocol, rest/r, or ws/w",
	}
	InputCsvFlag = &cli.StringFlag{
		Name:    "input",
		Aliases: []string{"i"},
		Value:   "input.csv",
		Usage:   "read data from `csv`",
	}
	OutputCsvFlag = &cli.StringFlag{
		Name:    "output",
		Aliases: []string{"o"},
		Value:   "output.csv",
		Usage:   "write to `csv`",
	}
	OutputPngFlag = &cli.StringFlag{
		Name:    "output",
		Aliases: []string{"o"},
		Value:   "output.png",
		Usage:   "write to `png`",
	}
	ExchangeFlag = &cli.StringFlag{
		Name:    "exchange",
		Aliases: []string{"ex"},
		Value:   "huobi",
		Usage:   "compute only the `exchange` symbols",
	}
	TestConfigFlag = &cli.BoolFlag{
		Name:    "test",
		Aliases: []string{"t"},
		Value:   false,
		Usage:   "do not run, just test config file",
	}
	DryRunFlag = &cli.BoolFlag{
		Name:  "dry-run",
		Value: false,
		Usage: "do not run, just print the result",
	}
)
