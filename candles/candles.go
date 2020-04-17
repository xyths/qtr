package candles

import (
	"encoding/csv"
	"github.com/xyths/qtr/exchange"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

func Pull(ex exchange.Exchange, currencyPair string, groupSec, rangeHour int) ([]exchange.Candle, error) {
	return nil, nil
}

func Write(coll *mongo.Collection, candles []exchange.Candle) (success, fail, duplidate int, err error) {

	return 0, 0, 0, nil
}
func Export(coll *mongo.Collection, start, end time.Time, w *csv.Writer) error {

	return nil
}
