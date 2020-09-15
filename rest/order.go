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

// sell-stop order, save in mongoDB
type SellStopOrder struct {
	Id        int64
	//ClientId  string `bson:"clientId"`
	//Type      string
	Price     string
	//StopPrice string `bson:"stopPrice"`
	//Amount    string
	//Total     string
	//Time      string
	//Status    string
}
