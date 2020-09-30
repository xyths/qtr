package natr

import (
	"github.com/markcheno/go-talib"
	"github.com/thrasher-corp/gocryptotrader/currency"
	"github.com/thrasher-corp/gocryptotrader/exchanges"
	"github.com/thrasher-corp/gocryptotrader/exchanges/asset"
	"github.com/thrasher-corp/gocryptotrader/exchanges/gateio"
	"github.com/thrasher-corp/gocryptotrader/exchanges/huobi"
	"github.com/thrasher-corp/gocryptotrader/exchanges/kline"
	"github.com/xyths/hs/logger"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	"strings"
	"time"
)

type Line struct {
	Name string
	XYs  plotter.XYs
}

func All(exchangeName string, symbols []string, start, end time.Time, output string) error {
	var currencyPairs []currency.Pair
	for _, s := range symbols {
		currencyPair := currency.NewPairFromString(s)
		currencyPairs = append(currencyPairs, currencyPair)
	}

	var ex exchange.IBotExchange
	switch exchangeName {
	case "gate":
		g := new(gateio.Gateio)
		ex = g
		ex.SetDefaults()
		if currencyPairs == nil {
			allSymbol, err := g.GetSymbols()
			if err != nil {
				logger.Sugar.Errorf("Gate GetSymbols error: %s", err)
				return err
			}
			for _, s := range allSymbol {
				currencyPair := currency.NewPairFromString(s)
				currencyPairs = append(currencyPairs, currencyPair)
			}
		}
	case "huobi":
		h := new(huobi.HUOBI)
		ex = h
		ex.SetDefaults()
		if currencyPairs == nil {
			allSymbol, err := h.GetSymbols()
			if err != nil {
				logger.Sugar.Errorf("Gate GetSymbols error: %s", err)
				return err
			}
			for _, s := range allSymbol {
				currencyPair := currency.NewPairFromStrings(s.BaseCurrency, s.QuoteCurrency)
				logger.Sugar.Infof("symbol %s, base currency %s, quote currency %s", currencyPair, s.BaseCurrency, s.QuoteCurrency)
				switch strings.ToLower(s.QuoteCurrency) {
				case "usdt", "husd":
					currencyPairs = append(currencyPairs, currencyPair)
				}
			}
		}
	default:
		panic("unsupported exchange")
	}

	var lines []interface{}
	for _, currencyPair := range currencyPairs {
		logger.Sugar.Debugf("currency pair %s", currencyPair.String())
		item, err := ex.GetHistoricCandlesExtended(currencyPair, asset.Spot, start, end, kline.FifteenMin)
		if err != nil {
			logger.Sugar.Error(err)
			continue
		}
		var timestamps []float64
		var high []float64
		var low []float64
		var close_ []float64
		if len(item.Candles) == 0 {
			logger.Sugar.Debugf("pair %s no candle", currencyPair.String())
			continue
		}
		for _, c := range item.Candles {
			timestamps = append(timestamps, float64(c.Time.Unix()))
			high = append(high, c.High)
			low = append(low, c.Low)
			close_ = append(close_, c.Close)
		}
		atrSeries := talib.Natr(high, low, close_, 14)
		logger.Sugar.Info(timestamps)
		logger.Sugar.Info(atrSeries)
		line := makeLine(currencyPair.String(), timestamps, atrSeries)
		lines = append(lines, line.Name)
		lines = append(lines, line.XYs)
	}
	plotToPng(lines, output)
	return nil
}

func plotToPng(lines []interface{}, output string) {
	p, err := plot.New()
	if err != nil {
		panic(err)
	}

	p.Title.Text = "NATR"
	p.X.Label.Text = "Timestamp"
	p.Y.Label.Text = "NATR"

	err = plotutil.AddLines(p, lines...)
	if err != nil {
		logger.Sugar.Errorf("add lines error: %s", err)
	}

	// Save the plot to a PNG file.
	if err := p.Save(16*vg.Inch, 8*vg.Inch, output); err != nil {
		panic(err)
	}
}

func makeLine(symbol string, x, y []float64) Line {
	pts := make(plotter.XYs, len(y))
	for i, yi := range y {
		pts[i].X = x[i]
		pts[i].Y = yi
	}
	return Line{symbol, pts}
}
