package mxc

type ResponseTimestamp struct {
	Code int
	Data uint64
}

/*
{
    "code": 200,
    "data": [
        {
            "id": "e5bb6963250146edb2f8677fcfcc97aa",
            "symbol": "MX_ETH",
            "price": "0.000907",
            "quantity": "300000",
            "state": "NEW",
            "type": "BID",
            "remain_quantity": "300000",
            "remain_amount": "272.1",
            "create_time": 1574338341797
        }
    ]
}
 */
type ResponseOpenOrders struct {
	Code int
	Data []RawOrder
}
type RawOrder struct {
	Symbol         string
	Id             string
	Price          string
	Quantity       string
	RemainQuantity string `json:"remain_quantity"`
	RemainAmount   string `json:"remain_amount"`
	CreateTime     string `json:"create_time"`
	State          string
	Type           string
	ClientOrderId  string `json:"client_order_id"`
}

/*
{
    "code": 200,
    "data": [
        {
            "symbol": "ETH_USDT",
            "order_id": "a39ea6b7afcf4f5cbba1e515210ff827",
            "quantity": "54.1",
            "price": "182.6317377",
            "amount": "9880.37700957",
            "fee": "9.88037700957",
            "trade_type": "BID",
            "fee_currency": "USDT",
            "is_taker": true,
            "create_time": 1572693911000
        }
    ]
}
 */
type ResponseDeals struct {
	Code int
	Data []Deal
}

type Deal struct {
	Symbol      string
	OrderId     string
	Quantity    string
	Price       string
	Amount      string
	Fee         string
	TradeType   string `json:"trade_type"`
	FeeCurrency string `json:"fee_currency"`
	IsTaker     bool   `json:"is_taker"`
	CreateTime  uint64 `json:"create_time"`
}

/*
{
    "code": 200,
    "data": [
        {
            "symbol": "ETH_USDT",
            "order_id": "a39ea6b7afcf4f5cbba1e515210ff827",
            "quantity": "54.1",
            "price": "182.6317377",
            "amount": "9880.37700957",
            "fee": "9.88037700957",
            "trade_type": "BID",
            "fee_currency": "USDT",
            "is_taker": true,
            "create_time": 1572693911000
        }
    ]
}
 */
