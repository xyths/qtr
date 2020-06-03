package gateio

const (
	BTC3L_USDT = "btc3l_usdt"
	BTC3S_USDT = "btc3s_usdt"
	SERO_USDT  = "sero_usdt"
)

var (
	PricePrecision = map[string]int{
		BTC3L_USDT: 4,
		BTC3S_USDT: 4,
		SERO_USDT:  5,
	}
	AmountPrecision = map[string]int{
		BTC3L_USDT: 3,
		BTC3S_USDT: 3,
		SERO_USDT:  3,
	}
	MinAmount = map[string]float64{
		BTC3L_USDT: 0.001,
		BTC3S_USDT: 0.001,
	}
	MinTotal = map[string]int64{
		BTC3L_USDT: 1,
		BTC3S_USDT: 1,
	}
)

// used by buy/sell
const (
	OrderTypeNormal = "gtc"
	OrderTypeGTC    = "gtc"
	OrderTypeIOC    = "ioc"
	OrderTypePOC    = "poc"
)

const (
	OrderStatusOpen      = "open"
	OrderStatusCancelled = "cancelled"
	OrderStatusClosed    = "closed"

	OrderTypeBuy  = "buy"
	OrderTypeSell = "sell"
)
