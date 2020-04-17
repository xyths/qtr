package huobi

import "time"

var (
	PricePrecision = map[string]int{
		"btcusdt": 2,
	}
	AmountPrecision = map[string]int{
		"btcusdt": 5,
	}
)

type HuobiBalance struct {
	Label    string
	BTC      float64 `gorm:"Column:btc"`
	USDT     float64 `gorm:"Column:usdt"`
	HT       float64 `gorm:"Column:ht"`
	BTCPrice float64 `gorm:"Column:btc_price"`
	HTPrice  float64 `gorm:"Column:ht_price"`
	Time     time.Time
}
