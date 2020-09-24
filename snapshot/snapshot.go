package snapshot

import (
	"context"
	"fmt"
	"github.com/google/martian/log"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/hs/exchange/gateio"
	"github.com/xyths/hs/exchange/huobi"
	. "github.com/xyths/hs/logger"
	"strings"
	"time"
)

type Config struct {
	Exchanges []hs.ExchangeConf
	Mongo     hs.MongoConf
}

type Snapshot struct {
	config Config

	//ex exchange
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
		b, err := s.balance(ctx, e)
		if err != nil {
			log.Errorf("balance error: %s", err)
			return err
		}
		key := fmt.Sprintf("%s-%s", e.Name, e.Label)
		balance[key] = b
	}
	balance["time"] = fmt.Sprintf("%d", time.Now().Unix())
	Sugar.Info(balance)
	coll := db.Collection("balance")
	_, err = coll.InsertOne(ctx, balance)
	return err
}
func (s *Snapshot) balance(ctx context.Context, e hs.ExchangeConf) (balance string, err error) {
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
	balance = cash.String()
	//Sugar.Debugf("cash: %s", cash)
	return balance, nil
}
