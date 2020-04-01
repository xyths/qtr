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
		Usage:   "only process the user with `label`",
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
)
