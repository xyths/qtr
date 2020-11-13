package main

import (
	"fmt"
	"github.com/thrasher-corp/gocryptotrader/currency"
	"github.com/thrasher-corp/gocryptotrader/exchanges/asset"
	"github.com/thrasher-corp/gocryptotrader/exchanges/gateio"
	"github.com/thrasher-corp/gocryptotrader/exchanges/huobi"
	"github.com/thrasher-corp/gocryptotrader/exchanges/kline"
	"github.com/xyths/hs"
	"os"
	"time"
)

type IExchange interface {
	GetHistoricCandles(pair currency.Pair, a asset.Item, start, end time.Time, interval kline.Interval) (kline.Item, error)
}

func getExchange(cfg hs.ExchangeConf) IExchange {
	switch cfg.Name {
	case "gate":
		var g gateio.Gateio
		g.SetDefaults()
		return &g
	case "huobi":
		var h huobi.HUOBI
		h.SetDefaults()
		return &h
	}
	return nil
}

func getInterval(period time.Duration) kline.Interval {
	return kline.Interval(period)
}

func writeToCsv(item kline.Item, output string) error {
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()

	_, _ = fmt.Fprintln(f, "Time,Open,High,Low,Close,Volume")
	for _, c := range item.Candles {
		_, _ = fmt.Fprintf(f, "%d,%f,%f,%f,%f,%f\n", c.Time.Unix(), c.Open, c.High, c.Low, c.Close, c.Volume)
	}
	return nil
}
