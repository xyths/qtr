package huobi

import (
	"context"
	"errors"
	"fmt"
	"github.com/huobirdcenter/huobi_golang/pkg/client"
	"github.com/huobirdcenter/huobi_golang/pkg/client/orderwebsocketclient"
	"github.com/huobirdcenter/huobi_golang/pkg/client/websocketclientbase"
	"github.com/huobirdcenter/huobi_golang/pkg/model/account"
	"github.com/huobirdcenter/huobi_golang/pkg/model/market"
	"github.com/huobirdcenter/huobi_golang/pkg/model/order"
	"github.com/xyths/hs/convert"
	"github.com/xyths/hs/exchange/huobi"
	"log"
	"strconv"
	"time"
)

type Config struct {
	Label        string
	AccessKey    string
	SecretKey    string
	CurrencyList []string
	Host         string
}

const DefaultHost = "api.huobi.io"

type Client struct {
	Config      Config
	Accounts    map[string]account.AccountInfo
	CurrencyMap map[string]bool

	orderSubscriber *orderwebsocketclient.SubscribeOrderWebSocketV2Client
}

func NewClient(config Config) *Client {
	c := &Client{
		Config: config,
	}
	if config.Host == "" {
		c.Config.Host = huobi.DefaultHost
	}
	c.CurrencyMap = make(map[string]bool)
	for _, currency := range c.Config.CurrencyList {
		c.CurrencyMap[currency] = true
	}
	c.Accounts = make(map[string]account.AccountInfo)
	if err := c.GetAccountInfo(); err != nil {
		log.Fatal(err)
	}
	return c
}

func (c *Client) ExchangeName() string {
	return "huobi"
}

func (c *Client) Label() string {
	return c.Config.Label
}

func (c *Client) GetTimestamp() (int, error) {
	hb := new(client.CommonClient).Init(c.Config.Host)
	return hb.GetTimestamp()
}

func (c *Client) GetAccountInfo() error {
	hb := new(client.AccountClient).Init(c.Config.AccessKey, c.Config.SecretKey, c.Config.Host)
	accounts, err := hb.GetAccountInfo()

	if err != nil {
		log.Printf("error when get account info: %s", err)
		return err
	}
	for _, acc := range accounts {
		c.Accounts[acc.Type] = acc
	}

	return nil
}

func (c *Client) Balances() (map[string]float64, error) {
	balances := make(map[string]float64)
	hb := new(client.AccountClient).Init(c.Config.AccessKey, c.Config.SecretKey, c.Config.Host)
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

func (c *Client) LastPrice(symbol string) (float64, error) {
	hb := new(client.MarketClient).Init(c.Config.Host)
	optionalRequest := market.GetCandlestickOptionalRequest{Period: market.MIN1, Size: 1}
	candlesticks, err := hb.GetCandlestick(symbol, optionalRequest)
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
	huobiBalance.BTC = balanceMap["btc"]
	huobiBalance.USDT = balanceMap["usdt"]
	huobiBalance.HT = balanceMap["ht"]
	btcPrice, err := c.LastPrice("btcusdt")
	if err != nil {
		return err
	}
	huobiBalance.BTCPrice = btcPrice
	htPrice, err := c.LastPrice("htusdt")
	if err != nil {
		return err
	}
	huobiBalance.HTPrice = htPrice
	huobiBalance.Time = time.Now()
	return nil
}

func (c *Client) SubscribeOrders(clientId string, responseHandler websocketclientbase.ResponseHandler) error {
	//hb := new(orderwebsocketclient.SubscribeOrderWebSocketV2Client).Init(c.Config.AccessKey, c.Config.SecretKey, Host)
	//hb.SetHandler(
	//	// Authentication response handler
	//	func(resp *auth.WebSocketV2AuthenticationResponse) {
	//		if resp.IsAuth() {
	//			err := hb.Subscribe("1", clientId)
	//			if err != nil {
	//				log.Printf("Subscribe error: %s\n", err)
	//			} else {
	//				log.Println("Sent subscription")
	//			}
	//		} else {
	//			log.Printf("Authentication error: %d\n", resp.Code)
	//		}
	//	},
	//	responseHandler)
	//return hb.Connect(true)
	return nil
}

func (c *Client) PlaceOrder(orderType, symbol string, price, amount float64) (uint64, error) {
	hb := new(client.OrderClient).Init(c.Config.AccessKey, c.Config.SecretKey, c.Config.Host)

	strPrice := fmt.Sprintf("%."+strconv.Itoa(PricePrecision[symbol])+"f", price)
	strAmount := fmt.Sprintf("%."+strconv.Itoa(AmountPrecision[symbol])+"f", amount)
	request := order.PlaceOrderRequest{
		AccountId: fmt.Sprintf("%d", c.Accounts["spot"].Id),
		Type:      orderType,
		Source:    "spot-api",
		Symbol:    symbol,
		Price:     strPrice,
		Amount:    strAmount,
	}
	resp, err := hb.PlaceOrder(&request)
	if err != nil {
		log.Println(err)
		return 0, err
	}
	switch resp.Status {
	case "ok":
		log.Printf("Place order successfully, order id: %s\n", resp.Data)
		return convert.StrToUint64(resp.Data), nil
	case "error":
		log.Printf("Place order error: %s\n", resp.ErrorMessage)
		if resp.ErrorCode == "account-frozen-balance-insufficient-error" {
			return 0, nil
		}
		return 0, errors.New(resp.ErrorMessage)
	}

	return 0, errors.New("unknown status")
}

func (c *Client) Sell(symbol string, price, amount float64) (orderId uint64, err error) {
	return c.PlaceOrder("sell-limit", symbol, price, amount)
}

func (c *Client) Buy(symbol string, price, amount float64) (orderId uint64, err error) {
	return c.PlaceOrder("buy-limit", symbol, price, amount)
}

func (c *Client) SubscribeBalanceUpdate(clientId string, responseHandler websocketclientbase.ResponseHandler) error {
	//hb := new(accountwebsocketclient.SubscribeAccountWebSocketV2Client).Init(c.Config.AccessKey, c.Config.SecretKey, Host)
	//hb.SetHandler(
	//	// Authentication response handler
	//	func(resp *auth.WebSocketV2AuthenticationResponse) {
	//		if resp.IsAuth() {
	//			err := hb.Subscribe("1", clientId)
	//			if err != nil {
	//				log.Printf("Subscribe error: %s\n", err)
	//			} else {
	//				log.Println("Sent subscription")
	//			}
	//		} else {
	//			log.Printf("Authentication error: %d\n", resp.Code)
	//		}
	//	},
	//	responseHandler)
	//return hb.Connect(true)
	return nil
}

func (c *Client) CancelOrder(orderId uint64) error {
	hb := new(client.OrderClient).Init(c.Config.AccessKey, c.Config.SecretKey, c.Config.Host)
	resp, err := hb.CancelOrderById("1")
	if err != nil {
		log.Println(err)
		return err
	}
	switch resp.Status {
	case "ok":
		log.Printf("Cancel order successfully, order id: %s\n", resp.Data)
		return nil
	case "error":
		log.Printf("Cancel order error: %s\n", resp.ErrorMessage)
		return errors.New(resp.ErrorMessage)
	}

	return nil
}
