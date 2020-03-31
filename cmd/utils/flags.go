package utils

import "github.com/urfave/cli/v2"

var (
	ConfigFlag = &cli.StringFlag{
		Name:    "config",
		Aliases: []string{"c"},
		Value:   "config.json",
		Usage:   "load configuration from `file`",
	}

	StartTimeFlag = &cli.StringFlag{
		Name:    "start",
		Aliases: []string{"s"},
		Value:   "",
		Usage:   "start `time`",
	}
	EndTimeFlag = &cli.StringFlag{
		Name:    "end",
		Aliases: []string{"e"},
		Value:   "",
		Usage:   "end `time`",
	}
)
