package exchange

type Exchange interface {
	Candles(currencyPair string, groupSec, rangeHour int) (candles []Candle, err error)
}
