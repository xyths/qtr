package snapshot

import (
	"encoding/json"
	"fmt"
	"github.com/google/martian/log"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/hs/exchange/gateio"
	"github.com/xyths/hs/exchange/huobi"
	. "github.com/xyths/hs/logger"
	"go.uber.org/zap"
	"os"
	"strings"
	"time"
)

type Config struct {
	Exchanges []hs.ExchangeConf
	Mongo     hs.MongoConf
	Log       hs.LogConf
	Output    string
}

type Snapshot struct {
	config Config
	Sugar  *zap.SugaredLogger
}

func New(configFilename string) *Snapshot {
	cfg := Config{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		Sugar.Fatal(err)
	}
	l, err := hs.NewZapLogger(cfg.Log)
	if err != nil {
		return nil
	}
	l.Sugar().Info("Logger initialized")
	s := &Snapshot{
		config: cfg,
		Sugar:  l.Sugar(),
	}
	return s
}

// balance of currency
type Currency struct {
	Exchange string  `json:"exchange"`
	Account  string  `json:"account"`
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
	Price    float64 `json:"price"`
	Value    float64 `json:"value"`
	Time     string  `json:"time"`
}

func (s *Snapshot) balance(e hs.ExchangeConf) (currencies []Currency, err error) {
	var ex exchange.RestAPIExchange
	switch e.Name {
	case "huobi":
		ex, err = huobi.New(e.Label, e.Key, e.Secret, e.Host)
		if err != nil {
			return
		}
	case "gate":
		ex = gateio.New(e.Key, e.Secret, e.Host, s.Sugar)
	}
	amounts, err := ex.SpotBalance()
	if err != nil {
		return
	}
	now := time.Now().String()
	for coin, amount := range amounts {
		// ignore
		if strings.ToLower(coin) == "point" {
			continue
		}
		currency := Currency{
			Exchange: e.Name,
			Account:  e.Label,
			Time:     now,
			Currency: strings.ToUpper(coin),
		}
		currency.Amount, _ = amount.Float64()
		if strings.ToLower(coin) == "usdt" {
			total, _ := amount.Float64()
			currency.Value = total
		} else {
			symbol := ex.FormatSymbol(coin, "usdt")
			price, err1 := ex.LastPrice(symbol)
			if err1 != nil {
				continue
			}
			//Sugar.Debugf("%s price: %s", symbol, price)
			currency.Price, _ = price.Float64()
			currency.Value, _ = price.Mul(amount).Float64()
		}
		currencies = append(currencies, currency)
	}
	return
}

func (s *Snapshot) Log() error {
	f, err := os.OpenFile(s.config.Output, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		Sugar.Error(err)
		return err
	}
	defer f.Close()

	for _, e := range s.config.Exchanges {
		currencies, err := s.balance(e)
		if err != nil {
			log.Errorf("balance error: %s", err)
			continue
		}
		for _, c := range currencies {
			b2, err2 := json.Marshal(c)
			if err2 != nil {
				Sugar.Error(err2)
				continue
			}
			_, err1 := fmt.Fprintf(f, "%s\n", string(b2))
			if err1 != nil {
				Sugar.Error(err1)
				continue
			}
		}
	}
	return nil
}
