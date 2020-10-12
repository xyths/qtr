package rest

import "time"

// for mongodb
type Order struct {
	Id            uint64 `bson:"_id"`
	ClientOrderId string `bson:"clientOrderId"`

	Type      string `bson:",omitempty"`
	Price     string `bson:",omitempty"`
	StopPrice string `bson:"stopPrice,omitempty"`
	Amount    string `bson:",omitempty"`
	Remain    string `bson:",omitempty"` // remain amount
	Total     string `bson:",omitempty"`
	Time      string `bson:",omitempty"`

	Context map[string]string `bson:",omitempty"`

	// created (not submitted),
	// submitted,
	// partial-filled, filled,
	// partial-canceled, canceled
	Status string `bson:",omitempty"`

	Trades []Trade `bson:",omitempty"`

	Updated time.Time `bson:",omitempty"`
}

type Trade struct {
	Id     uint64
	Price  string `bson:",omitempty"`
	Amount string `bson:",omitempty"`
	Total  string `bson:",omitempty"`
	Remain string `bson:",omitempty"`

	Time time.Time `bson:",omitempty"`
}

type NamedOrder struct {
	Name string
	Order
}

type BuyOrder = NamedOrder
type SellOrder = NamedOrder
type SellStopOrder = NamedOrder
type ReinforceOrder = NamedOrder

func (o *NamedOrder) Clear() {
	o.Id = 0
	o.ClientOrderId = ""
	o.Type = ""
	o.Price = ""
	o.StopPrice = ""
	o.Amount = ""
	o.Total = ""
	o.Time = ""
	o.Status = ""
	o.Trades = nil
	o.Updated = time.Now()
}
