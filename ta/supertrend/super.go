package supertrend

import (
	"context"
	"encoding/csv"
	"fmt"
	indicator "github.com/xyths/go-indicators"
	"github.com/xyths/hs/exchange"
	"go.uber.org/zap"
	"os"
	"time"
)

type Trend struct {
	Symbol string
	Trends []bool
}
type ScanResult struct {
	Periods []string
	Trends  []Trend
}

type SuperTrend struct {
	Sugar *zap.SugaredLogger
	Ex    exchange.RestAPIExchange
}

func (st *SuperTrend) Scan(ctx context.Context, symbols []string, size int64, monthly, weekly, daily, hour4, hour1 bool) (r ScanResult, err error) {
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
			return
		default:
			t := Trend{Symbol: symbol}
			if monthly {
				up, err := st.period(symbol, exchange.MON1, size)
				if err == nil && up {
					t.Trends = append(t.Trends, true)
				} else {
					t.Trends = append(t.Trends, false)
				}
			}
			if weekly {
				up, err := st.period(symbol, exchange.WEEK1, size)
				if err == nil && up {
					t.Trends = append(t.Trends, true)
				} else {
					t.Trends = append(t.Trends, false)
				}
			}
			if daily {
				up, err := st.period(symbol, exchange.DAY1, size)
				if err == nil && up {
					t.Trends = append(t.Trends, true)
				} else {
					t.Trends = append(t.Trends, false)
				}
			}
			if hour4 {
				up, err := st.period(symbol, exchange.HOUR4, size)
				if err == nil && up {
					t.Trends = append(t.Trends, true)
				} else {
					t.Trends = append(t.Trends, false)
				}
			}
			if hour1 {
				up, err := st.period(symbol, exchange.HOUR1, size)
				if err == nil && up {
					t.Trends = append(t.Trends, true)
				} else {
					t.Trends = append(t.Trends, false)
				}
			}
			r.Trends = append(r.Trends, t)
		}
	}
	return
}

func (st *SuperTrend) period(symbol string, period time.Duration, size int64) (up bool, err error) {
	candle, err := st.Ex.CandleBySize(symbol, period, int(size))
	if err != nil {
		return
	}
	if candle.Length() <= 9 {
		return
	}
	_, trend := indicator.SuperTrend(3, 7, candle.High, candle.Low, candle.Close)
	if len(trend) > 2 {
		up = trend[len(trend)-2]
	}
	return
}

func (st *SuperTrend) WriteToCsv(ctx context.Context, r ScanResult, output string) error {
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
