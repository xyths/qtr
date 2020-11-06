package research

import (
	"encoding/csv"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs/logger"
	"os"
	"strconv"
)

func readData(filename string,header bool) (timestamp []int64, open, high, low, close []float64) {
	f, err := os.Open(filename)
	if err != nil {
		logger.Sugar.Fatalf("open file error: %s", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		logger.Sugar.Fatalf("read csv data error: %s", err)
	}
	l := len(records)
	timestamp = make([]int64, l)
	open = make([]float64, l)
	high = make([]float64, l)
	low = make([]float64, l)
	close = make([]float64, l)
	i := 0
	if header {
		i++
	}
	for ; i < l; i++ {
		timestamp[i], _ = strconv.ParseInt(records[i][0], 10, 64)
		open[i], _ = strconv.ParseFloat(records[i][1], 64)
		high[i], _ = strconv.ParseFloat(records[i][2], 64)
		low[i], _ = strconv.ParseFloat(records[i][3], 64)
		close[i], _ = strconv.ParseFloat(records[i][4], 64)
	}
	return
}

type SuperTrendReturn struct {
	Factor float64
	Period int
	Final  float64
	Rate   float64
	Annual float64
}

func writeResult(results []SuperTrendReturn, output string) error {
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()


	fmt.Fprintln(f, "Factor,Period,Final,Rate,AnnualRate")
	for _, r := range results {
		fmt.Fprintf(f,"%f,%d,%f,%f,%f\n", r.Factor, r.Period, r.Final, r.Rate, r.Annual)
	}
	return nil
}

func Profit(timestamp []int64, open, high, low, close []float64, signal []bool, cash decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
