package rest

import "time"

// for mongodb
type Order struct {
	Id            int64
	ClientOrderId string `bson:"clientOrderId"`
	Type          string
	Price         string
	Amount        string
	Total         string

	Status string

	Trades []Trade

	Updated time.Time
}

type Trade struct {
	Id     int64
	Price  string
	Amount string
	Total  string
	Remain string

	Time string
}
