package exchange

import "context"

type Exchange interface {
	//Candles(currencyPair string, groupSec, rangeHour int) (candles []Candle, err error)
	Snapshot(ctx context.Context, result interface{}) error
}
