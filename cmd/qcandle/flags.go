package main

import "github.com/urfave/cli/v2"

var (
	SymbolFlag = &cli.StringFlag{
		Name:    "symbol",
		Value:   "btc_usdt",
		Usage:   "get the candlestick of `symbol`",
	}
	TypeFlag = &cli.StringFlag{
		Name:    "type",
		Aliases: []string{"t"},
		Value:   "1d",
		Usage:   "candlestick `type`",
	}
	GroupSecFlag = &cli.IntFlag{
		Name:    "group_sec",
		Aliases: []string{"g"},
		Value:   60,
		Usage:   "`group_sec` of candlestick",
	}
	RangeHourFlag = &cli.IntFlag{
		Name:    "range_hour",
		Aliases: []string{"r"},
		Value:   1,
		Usage:   "`range_hour` of candlestick",
	}
	HostFlag = &cli.StringFlag{
		Name:  "host",
		Usage: "gate `host`",
	}
)
