package node

import "time"

// for mongo
type Trade struct {
	OrderNumber uint64    `bson:"_id"`
	Label       string    `bson:"label"`
	Pair        string    `bson:"pair"`
	Type        string    `bson:"type"`
	Rate        string    `bson:"rate"`
	Amount      float64   `bson:"amount"`
	Total       float64   `bson:"total"`
	Date        time.Time `bson:"date"`
	TimeUnix    int64    `bson:"timeUnix"`
	Role        string    `bson:"role"`
	Fee         float64   `bson:"fee"`
	FeeCoin     string    `bson:"feeCoin"`
	GtFee       float64   `bson:"gtFee"`
	PointFee    float64   `bson:"pointFee"`
}
