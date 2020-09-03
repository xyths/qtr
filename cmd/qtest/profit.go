package main

import (
	"encoding/csv"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs/logger"
	"os"
	"strconv"
)

func readData(filename string) (timestamp []int64, open, high, low, close []float64) {
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
	for i := 0; i < l; i++ {
		timestamp[i], _ = strconv.ParseInt(records[i][0], 10, 64)
		open[i], _ = strconv.ParseFloat(records[i][1], 64)
		high[i], _ = strconv.ParseFloat(records[i][2], 64)
		low[i], _ = strconv.ParseFloat(records[i][3], 64)
		close[i], _ = strconv.ParseFloat(records[i][4], 64)
	}
	return
}

func Profit(timestamp []int64, open, high, low, close []float64, signal []bool, cash decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
