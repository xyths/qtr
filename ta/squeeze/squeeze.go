package squeeze

import (
	"context"
	"encoding/csv"
	"fmt"
	indicator "github.com/xyths/go-indicators"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"go.uber.org/zap"
	"os"
	"time"
)

type Trend struct {
	Symbol string
	Trends []int
}
type ScanResult struct {
	Periods []string
	Trends  []Trend
}

type Squeeze struct {
	Sugar *zap.SugaredLogger
	Ex    exchange.RestAPIExchange
}

func (s *Squeeze) Scan(ctx context.Context, symbols []string, size int64, monthly, weekly, daily, hour4, hour1 bool) (r ScanResult, err error) {
	// prepare header
	{
		if monthly {
			r.Periods = append(r.Periods, "M")
		}
		if weekly {
			r.Periods = append(r.Periods, "W")
		}
		if daily {
			r.Periods = append(r.Periods, "D")
		}
		if hour4 {
			r.Periods = append(r.Periods, "4h")
		}
		if hour1 {
			r.Periods = append(r.Periods, "1h")
		}
	}
	for _, symbol := range symbols {
		select {
		case <-ctx.Done():
			return r, ctx.Err()
		default:
			t := Trend{Symbol: symbol}
			if monthly {
				trend, _ := s.period(symbol, exchange.MON1, size)
				t.Trends = append(t.Trends, trend)
			}
			if weekly {
				trend, _ := s.period(symbol, exchange.WEEK1, size)
				t.Trends = append(t.Trends, trend)
			}
			if daily {
				trend, _ := s.period(symbol, exchange.DAY1, size)
				t.Trends = append(t.Trends, trend)
			}
			if hour4 {
				trend, _ := s.period(symbol, exchange.HOUR4, size)
				t.Trends = append(t.Trends, trend)
			}
			if hour1 {
				trend, _ := s.period(symbol, exchange.HOUR1, size)
				t.Trends = append(t.Trends, trend)
			}
			r.Trends = append(r.Trends, t)
		}
	}
	return
}

func (s *Squeeze) period(symbol string, period time.Duration, size int64) (trend int, err error) {
	candle, err := s.Ex.CandleBySize(symbol, period, int(size))
	if err != nil {
		return
	}
	return s.Calculate(20, 20, 2.0, 1.5, candle), nil
}

func (s *Squeeze) Calculate(bbl, kcl int, bbf, kcf float64, candle hs.Candle) int {
	if candle.Length() < bbl+2 && candle.Length() < kcl+2 {
		return 0
	}
	r, _ := indicator.Squeeze(bbl, kcl, bbf, kcf, candle.High, candle.Low, candle.Close)
	return r.Trend
}

func (s *Squeeze) WriteToCsv(ctx context.Context, r ScanResult, output string) error {
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"symbol"}
	for _, period := range r.Periods {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			header = append(header, period)
		}
	}
	_ = w.Write(header)
	for i := 0; i < len(r.Trends); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line := []string{r.Trends[i].Symbol}
			for j := 0; j < len(r.Trends[i].Trends); j++ {
				line = append(line, fmt.Sprintf("%v", r.Trends[i].Trends[j]))
			}
			_ = w.Write(line)
		}
	}
	return nil
}
