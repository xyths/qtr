package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/martian/log"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/hs/exchange/gateio"
	"github.com/xyths/hs/exchange/huobi"
	. "github.com/xyths/hs/logger"
	"os"
	"strings"
	"time"
)

type Config struct {
	Exchanges []hs.ExchangeConf
	Mongo     hs.MongoConf
	Output    string
}

type Snapshot struct {
	config Config
}

func New(configFilename string) *Snapshot {
	cfg := Config{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		Sugar.Fatal(err)
	}
	s := &Snapshot{
		config: cfg,
	}
	return s
}

func (s *Snapshot) Snapshot(ctx context.Context) error {
	db, err := hs.ConnectMongo(ctx, s.config.Mongo)
	if err != nil {
		return err
	}
	defer db.Client().Disconnect(ctx)
	balance := make(map[string]string)
	for _, e := range s.config.Exchanges {
		b, err := s.balance(e)
		if err != nil {
			log.Errorf("balance error: %s", err)
			return err
		}
		key := fmt.Sprintf("%s-%s", e.Name, e.Label)
		balance[key] = b.String()
	}
	balance["time"] = fmt.Sprintf("%d", time.Now().Unix())
	Sugar.Info(balance)
	coll := db.Collection("balance")
	_, err = coll.InsertOne(ctx, balance)
	return err
}

func (s *Snapshot) balance(e hs.ExchangeConf) (balance decimal.Decimal, err error) {
	var ex exchange.RestAPIExchange
	switch e.Name {
	case "huobi":
		ex, err = huobi.New(e.Label, e.Key, e.Secret, e.Host)
	case "gate":
		ex = gateio.New(e.Key, e.Secret, e.Host)
	}
	amounts, err := ex.SpotBalance()
	if err != nil {
		return
	}
	cash := decimal.Zero
	for coin, amount := range amounts {
		//Sugar.Debugf("%s: %s", coin, amount)
		if strings.ToLower(coin) == "usdt" {
			cash = cash.Add(amount)
			continue
		}
		if strings.ToLower(coin) == "point" { // ignore
			continue
		}
		symbol := ex.FormatSymbol(coin, "usdt")
		price, err1 := ex.LastPrice(symbol)
		if err1 != nil {
			return balance, err1
		}
		//Sugar.Debugf("%s price: %s", symbol, price)
		cash = cash.Add(price.Mul(amount))
	}
	return cash, nil
}

func (s *Snapshot) Log() error {
	f, err := os.OpenFile(s.config.Output, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		Sugar.Error(err)
		return err
	}
	defer f.Close()

	type Balance struct {
		Exchange string
		Account  string
		Balance  float64
		Time     string
	}
	timeStr := time.Now().String()
	var balance Balance
	for _, e := range s.config.Exchanges {
		b, err := s.balance(e)
		if err != nil {
			log.Errorf("balance error: %s", err)
			continue
		}
		balance.Exchange = e.Name
		balance.Account = e.Label
		balance.Balance, _ = b.Float64()
		balance.Time = timeStr
		b2, err2 := json.Marshal(balance)
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
	return nil
}
