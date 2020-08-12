package natr

import (
	"github.com/markcheno/go-talib"
	"github.com/thrasher-corp/gocryptotrader/currency"
	"github.com/thrasher-corp/gocryptotrader/exchanges/asset"
	"github.com/thrasher-corp/gocryptotrader/exchanges/gateio"
	"github.com/thrasher-corp/gocryptotrader/exchanges/kline"
	"github.com/xyths/hs/logger"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	"time"
)

type Line struct {
	Name string
	XYs  plotter.XYs
}

func All(exchange string, symbols []string, start, end time.Time) error {
	var g gateio.Gateio
	g.SetDefaults()
	var lines []Line
	for _, s := range symbols {
		currencyPair := currency.NewPairFromString(s)
		item, err := g.GetHistoricCandlesExtended(currencyPair, asset.Spot, start, end, kline.FifteenMin)
		if err != nil {
			logger.Sugar.Error(err)
			continue
		}
		var timestamps []float64
		var high []float64
		var low []float64
		var closes []float64
		for _, c := range item.Candles {
			timestamps = append(timestamps, float64(c.Time.Unix()))
			high = append(high, c.High)
			low = append(low, c.Low)
			closes = append(closes, c.Close)
		}
		atrSeries := talib.Natr(high, low, closes, 14)
		logger.Sugar.Info(timestamps)
		logger.Sugar.Info(atrSeries)
		lines = append(lines, makeLine(s, timestamps, atrSeries))
	}
	plotToPng(lines, "data/natr_15min.png")
	return nil
}

func plotToPng(lines []Line, output string) {
	p, err := plot.New()
	if err != nil {
		panic(err)
	}

	p.Title.Text = "Gate Symbols NATR"
	p.X.Label.Text = "Timestamp"
	p.Y.Label.Text = "NATR"

	err = plotutil.AddLinePoints(p,
		lines[0].Name, lines[0].XYs,
		lines[1].Name, lines[1].XYs,
		lines[2].Name, lines[2].XYs,
	)
	if err != nil {
		panic(err)
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
