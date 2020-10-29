package natr

import (
	"encoding/csv"
	"fmt"
	"github.com/markcheno/go-talib"
	"github.com/xyths/hs/exchange"
	"os"
	"time"
)

type NatrResult struct {
	Timestamp []int64
	Symbols   []string
	Natr      [][]float64
}

func NATR(ex exchange.RestAPIExchange, symbols []string, start, end time.Time, period time.Duration) (r NatrResult, err error) {
	for _, symbol := range symbols {
		candle, err := ex.CandleFrom(symbol, "natr", period, start, end)
		if err != nil {
			return r, err
		}
		if len(r.Timestamp) == 0 {
			r.Timestamp = candle.Timestamp
		}
		natrSeries := talib.Natr(candle.High, candle.Low, candle.Close, 14)
		r.Symbols = append(r.Symbols, symbol)
		r.Natr = append(r.Natr, natrSeries)
	}
	return
}

func WriteToCsv(r NatrResult, output string) error {
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"timestamp"}
	for _, symbol := range r.Symbols {
		header = append(header, symbol)
	}
	_ = w.Write(header)
	for i := 0; i < len(r.Timestamp); i++ {
		line := []string{fmt.Sprintf("%d", r.Timestamp[i])}
		for j := 0; j < len(r.Natr); j++ {
			line = append(line, fmt.Sprintf("%f", r.Natr[j][i]))
		}
		_ = w.Write(line)
	}
	return nil
}
