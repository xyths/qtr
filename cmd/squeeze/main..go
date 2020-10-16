package main

import (
	"context"
	"fmt"
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/trader/rest"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var app *cli.App

func init() {
	app = &cli.App{
		Name:    filepath.Base(os.Args[0]),
		Action:  squeeze,
		Usage:   "the Squeeze strategy trading robot",
		Version: "0.2.9",
	}

	app.Commands = []*cli.Command{
		{
			Action: squeezePrint,
			Name:   "print",
			Usage:  "Print the Squeeze state",
		},
		{
			Action: squeezeClear,
			Name:   "clear",
			Usage:  "clear all Squeeze state in database, cancel pending orders",
			Flags: []cli.Flag{
				utils.DryRunFlag,
			},
		},
	}
	app.Flags = []cli.Flag{
		utils.ConfigFlag,
		utils.ProtocolFlag,
		utils.DryRunFlag,
	}
}
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	if err := app.RunContext(ctx, os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func squeeze(ctx *cli.Context) error {
	configFile := ctx.String(utils.ConfigFlag.Name)
	dry := ctx.Bool(utils.DryRunFlag.Name)
	protocol := ctx.String(utils.ProtocolFlag.Name)
	switch protocol {
	case "r", "rest":
		squeezeRest(ctx, configFile, dry)
	case "w", "ws":
		//
	}
	return nil
}

func squeezeRest(ctx *cli.Context, cfg string, dry bool) {
	t, err := rest.NewSqueezeMomentumTrader(ctx.Context, cfg, dry)
	if err != nil {
		return
	}
	defer t.Close(ctx.Context)
	t.Run(ctx.Context)
}

func squeezePrint(ctx *cli.Context) error {
	//configFile := ctx.String(utils.ConfigFlag.Name)
	//t, err := ws.NewSuperTrendTrader(ctx.Context, configFile)
	//if err != nil {
	//	return err
	//}
	//defer t.Close(ctx.Context)
	//return t.Print(ctx.Context)
	return nil
}

func squeezeClear(ctx *cli.Context) error {
	//configFile := ctx.String(utils.ConfigFlag.Name)
	//t, err := ws.NewSuperTrendTrader(ctx.Context, configFile)
	//if err != nil {
	//	return err
	//}
	//defer t.Close(ctx.Context)
	//return t.Clear(ctx.Context)
	return nil
}
