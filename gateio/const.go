package gateio

const (
	BTC_USDT   = "btc_usdt"
	BTC3L_USDT = "btc3l_usdt"
	BTC3S_USDT = "btc3s_usdt"
	SERO_USDT  = "sero_usdt"

	BTC   = "BTC"
	BTC3L = "BTC3L"
	BTC3S = "BTC3S"
	USDT  = "USDT"
	SERO  = "SERO"
)

var (
	PricePrecision = map[string]int{
		BTC_USDT:   2,
		BTC3L_USDT: 4,
		BTC3S_USDT: 4,
		SERO_USDT:  5,
	}
	AmountPrecision = map[string]int{
		BTC_USDT:   4,
		BTC3L_USDT: 3,
		BTC3S_USDT: 3,
		SERO_USDT:  3,
	}
	MinAmount = map[string]float64{
		BTC_USDT:   0.0001,
		BTC3L_USDT: 0.001,
		BTC3S_USDT: 0.001,
		SERO_USDT:  0.001,
	}
	MinTotal = map[string]int64{
		BTC_USDT:   1,
		BTC3L_USDT: 1,
		BTC3S_USDT: 1,
		SERO_USDT:  1,
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
