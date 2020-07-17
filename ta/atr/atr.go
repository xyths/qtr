package atr

import (
	"github.com/thrasher-corp/gct-ta/indicators"
	"github.com/thrasher-corp/gocryptotrader/currency"
	"github.com/thrasher-corp/gocryptotrader/exchanges/asset"
	"github.com/thrasher-corp/gocryptotrader/exchanges/gateio"
	"github.com/thrasher-corp/gocryptotrader/exchanges/kline"
	"github.com/xyths/hs/logger"
	"time"
)

func All(exchange string, symbols []string, start, end time.Time) error {
	var g gateio.Gateio
	g.SetDefaults()
	for _, s := range symbols {
		currencyPair := currency.NewPairFromString(s)
		item, err := g.GetHistoricCandlesExtended(currencyPair, asset.Spot, start, end, kline.OneMin)
		if err != nil {
			logger.Sugar.Error(err)
			continue
		}
		var high []float64
		var low []float64
		var close []float64
		for _, c := range item.Candles {
			high = append(high, c.High)
			low = append(low, c.Low)
			close = append(close, c.Close)
		}
		atrSeries := indicators.ATR(high, low, close, 14)
		logger.Sugar.Info(atrSeries)
	}
	return nil
}
