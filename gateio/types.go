package gateio

// from api result
type RawTrade struct {
	TradeId     uint64  `json:"tradeID"`
	OrderNumber uint64  `json:"orderNumber"`
	Pair        string  `json:"pair"`
	Type        string  `json:"type"`
	Rate        string  `json:"rate"`
	Amount      string  `json:"amount"`
	Total       float64 `json:"total"`
	Date        string  `json:"date"`
	TimeUnix    int64  `json:"time_unix"`
	Role        string  `json:"role"`
	Fee         string  `json:"fee"`
	FeeCoin     string  `json:"fee_coin"`
	GtFee       string  `json:"gt_fee"`
	PointFee    string  `json:"point_fee"`
}

type MyTradeHistoryResult struct {
	Result  string     `json:"result"`
	Trades  []RawTrade `json:"trades"`
	Message string     `json:"message"`
	Code    int        `json:"code"`
}
