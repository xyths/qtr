package huobi

import (
	"context"
	"errors"
	"fmt"
	"github.com/huobirdcenter/huobi_golang/pkg/client"
	"github.com/huobirdcenter/huobi_golang/pkg/getrequest"
	"github.com/huobirdcenter/huobi_golang/pkg/response/account"
	"github.com/xyths/hs/convert"
	"log"
	"time"
)

type Config struct {
	Label        string
	AccessKey    string
	SecretKey    string
	CurrencyList []string
}

const Host = "api.huobi.io"

type Client struct {
	Config      Config
	Accounts    []account.AccountInfo
	CurrencyMap map[string]bool
}

func NewClient(config Config) *Client {
	c := &Client{
		Config: config,
	}
	c.CurrencyMap = make(map[string]bool)
	for _, currency := range c.Config.CurrencyList {
		c.CurrencyMap[currency] = true
	}
	return c
}

func (c *Client) GetTimestamp() (int, error) {
	hb := new(client.CommonClient).Init(Host)
	return hb.GetTimestamp()
}

func (c *Client) GetAccountInfo() ([]account.AccountInfo, error) {
	hb := new(client.AccountClient).Init(c.Config.AccessKey, c.Config.SecretKey, Host)
	return hb.GetAccountInfo()
}

func (c *Client) Balances() (map[string]float64, error) {
	if c.Accounts == nil {
		accounts, err := c.GetAccountInfo()
		if err != nil {
			log.Printf("error when get account info: %s", err)
			return nil, err
		}
		c.Accounts = accounts

	}
	balances := make(map[string]float64)
	hb := new(client.AccountClient).Init(c.Config.AccessKey, c.Config.SecretKey, Host)
	for _, acc := range c.Accounts {
		ab, err := hb.GetAccountBalance(fmt.Sprintf("%d", acc.Id))
		if err != nil {
			log.Printf("[ERROR] error when get account %d balance: %s", acc.Id, err)
			return balances, err
		}
		for _, b := range ab.List {
			if !c.CurrencyMap[b.Currency] {
				continue
			}
			realBalance := convert.StrToFloat64(b.Balance)
			balances[b.Currency] += realBalance
		}
	}
	return balances, nil
}

func (c *Client) Price(symbol string) (float64, error) {
	hb := new(client.MarketClient).Init(Host)
	optionalRequest := getrequest.GetCandlestickOptionalRequest{Period: getrequest.MIN1, Size: 1}
	candlesticks, err := hb.GetCandlestick(fmt.Sprintf("%susdt", symbol), optionalRequest)
	if err != nil {
		return 0, err
	}
	for _, cs := range candlesticks {
		price, _ := cs.Close.Float64()
		return price, nil
	}
	return 0, nil
}

func (c *Client) Snapshot(ctx context.Context, result interface{}) error {
	huobiBalance, ok := result.(*HuobiBalance)
	if !ok {
		return errors.New("bad result type, should be *HuobiBalance")
	}
	balanceMap, err := c.Balances()
	if err != nil {
		return err
	}
	huobiBalance.Label = c.Config.Label
	huobiBalance.BTC = balanceMap["BTC"]
	huobiBalance.USDT = balanceMap["USDT"]
	huobiBalance.HT = balanceMap["HT"]
	btcPrice, err := c.Price("btc")
	if err != nil {
		return err
	}
	huobiBalance.BTCPrice = btcPrice
	htPrice, err := c.Price("ht")
	if err != nil {
		return err
	}
	huobiBalance.HTPrice = htPrice
	huobiBalance.Time = time.Now()
	return nil
}
