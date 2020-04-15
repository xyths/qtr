package huobi

import "time"

type HuobiBalance struct {
	Label    string
	BTC      float64 `gorm:"Column:btc"`
	USDT     float64 `gorm:"Column:usdt"`
	HT       float64 `gorm:"Column:ht"`
	BTCPrice float64 `gorm:"Column:btc_price"`
	HTPrice  float64 `gorm:"Column:ht_price"`
	Time     time.Time
}
