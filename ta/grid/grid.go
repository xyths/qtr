package grid

import (
	"context"
	"encoding/csv"
	"fmt"
	"github.com/markcheno/go-talib"
	indicator "github.com/xyths/go-indicators"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/qtr/ta/squeeze"
	"github.com/xyths/qtr/ta/supertrend"
	"go.uber.org/zap"
	"math"
	"os"
)

type ScanResult struct {
	Headers []string
	Rows    [][]interface{}
}

type Scanner struct {
	Sugar *zap.SugaredLogger
	Ex    exchange.RestAPIExchange
}

func (s *Scanner) Scan(ctx context.Context, symbols []string, size int64) (r ScanResult, err error) {
	// prepare header
	r.Headers = []string{"Symbol", "Squeeze-Week", "Squeeze-Day", "SuperTrend-Day", "NATR", "Upper", "Lower", "Grids", "Percent", "Min"}
	s1 := &squeeze.Squeeze{
		Sugar: s.Sugar, Ex: s.Ex,
	}
	s2 := &supertrend.SuperTrend{
		Sugar: s.Sugar, Ex: s.Ex,
	}

	for i, symbol := range symbols {
		select {
		case <-ctx.Done():
			return

		default:
			s.Sugar.Infof("[%d] %s scanning start ...", i, symbol)
			row := s.ScanSymbol(ctx, s1, s2, symbol, int(size))
			if row != nil {
				r.Rows = append(r.Rows, row)
			}

			s.Sugar.Infof("[%d] %s scanning finished", i, symbol)
		}
	}
	s.Sugar.Info("all symbols is scanned")
	return
}

func (s *Scanner) ScanSymbol(ctx context.Context, s1 *squeeze.Squeeze, s2 *supertrend.SuperTrend, symbol string, size int) []interface{} {
	var r []interface{}
	r = append(r, symbol)
	week, err := s.Ex.CandleBySize(symbol, exchange.WEEK1, size)
	if err != nil {
		s.Sugar.Error(err)
		return nil
	}
	weekSqueeze := s1.Calculate(20, 20, 2.0, 1.5, week)
	r = append(r, weekSqueeze == 2)

	day, err := s.Ex.CandleBySize(symbol, exchange.DAY1, size)
	if err != nil {
		s.Sugar.Error(err)
		return nil
	}
	if day.Length() < 22 {
		return nil
	}
	daySqueeze := s1.Calculate(20, 20, 2.0, 1.5, week)
	r = append(r, daySqueeze == 2)
	superTrend := s2.Calculate(3, 7, day)
	r = append(r, superTrend)

	natr := talib.Natr(day.High, day.Low, day.Close, 14)
	r = append(r, natr[len(natr)-2])

	trendUp, trendDown, _, _ := indicator.SuperTrendDetail(3, 7, day.High, day.Low, day.Close)
	upper := Max(trendDown[day.Length()-8 : day.Length()-1])
	r = append(r, upper)
	lower := Min(trendUp[day.Length()-8 : day.Length()-1])
	r = append(r, lower)
	if upper < 0 || lower < 0 {
		return r
	}
	grids, percent, min := ComputeGrid(upper, lower)
	r = append(r, grids)
	r = append(r, percent)
	r = append(r, min)
	return r
}

func (s *Scanner) WriteToCsv(ctx context.Context, r ScanResult, output string) error {
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write(r.Headers)
	for _, row := range r.Rows {
		select {
		case <-ctx.Done():
			return ctx.Err()

		default:
			var line []string
			for _, token := range row {
				line = append(line, fmt.Sprintf("%v", token))
			}
			_ = w.Write(line)
		}
	}
	return nil
}

func Max(array []float64) float64 {
	var max float64
	for _, f := range array {
		if f > max {
			max = f
		}
	}
	return max
}
func Min(array []float64) float64 {
	min := math.MaxFloat64
	for _, f := range array {
		if f < min {
			min = f
		}
	}
	return min
}

func ComputeGrid(upper, lower float64) (grids int, percent float64, min float64) {
	percent = (upper - lower) / upper
	if percent <= 0.004 {
		return
	}
	grids = 1
	for ; math.Pow(0.995, float64(grids)) >= 1-percent; grids++ {
	}
	return
}
