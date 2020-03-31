package gateio

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

const DataSource = "https://data.gateio.life/api2/1"

type GateIO struct {
	Key    string
	Secret string
}

func NewGateIO(key, secret string) *GateIO {
	return &GateIO{Key: key, Secret: secret}
}

// all support pairs
func (g *GateIO) GetPairs() (string, error) {
	var method string = "GET"
	var url string = DataSource + "/pairs"
	var param string = ""
	if ret, err := g.httpDo(method, url, param); err != nil {
		return "", err
	} else {
		return string(ret), err
	}
}

// Market Info
//func (g *GateIO) marketinfo() string {
//	var method string = "GET"
//	var url string = DataSource + "/marketinfo"
//	var param string = ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// Market Details
//func (g *GateIO) marketlist() string {
//	var method string = "GET"
//	var url string = DataSource + "/marketlist"
//	var param string = ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// tickers
//func (g *GateIO) tickers() string {
//	var method string = "GET"
//	var url string = DataSource + "/tickers"
//	var param string = ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// ticker
//func (g *GateIO) ticker(ticker string) string {
//	var method string = "GET"
//	var url string = DataSource + "/ticker" + "/" + ticker
//	var param string = ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// Depth
//func (g *GateIO) orderBooks() string {
//	var method string = "GET"
//	var url string = DataSource + "/orderBooks"
//	var param string = ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// Depth of pair
//func (g *GateIO) orderBook(params string) string {
//	var method string = "GET"
//	var url string = DataSource + "/orderBook/" + params
//	var param string = ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
// Trade History
func (g *GateIO) TradeHistory(params string) (string, error) {
	var method string = "GET"
	var url string = DataSource + "/TradeHistory/" + params
	var param string = ""
	data, err := g.httpDo(method, url, param)
	if err != nil {
		return "", err
	} else {
		return string(data), err
	}
}

//// Get account fund balances
//func (g *GateIO) balances() string {
//	var method string = "POST"
//	var url string = DataSource + "/private/balances"
//	var param string = ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// get deposit address
//func (g *GateIO) depositAddress(currency string) string {
//	var method string = "POST"
//	var url string = DataSource + "/private/depositAddress"
//	var param string = "currency=" + currency
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// get deposit withdrawal history
//func (g *GateIO) depositsWithdrawals(start string, end string) string {
//	var method string = "POST"
//	var url string = DataSource + "/private/depositsWithdrawals"
//	var param string = "start=" + start + "&end=" + end
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// Place order buy
//func (g *GateIO) buy(currencyPair string, rate string, amount string) string {
//	var method string = "POST"
//	var url string = DataSource + "/private/buy"
//	var param string = "currencyPair=" + currencyPair + "&rate=" + rate + "&amount=" + amount
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// Place order sell
//func (g *GateIO) sell(currencyPair string, rate string, amount string) string {
//	var method string = "POST"
//	var url string = DataSource + "/private/sell"
//	var param string = "currencyPair=" + currencyPair + "&rate=" + rate + "&amount=" + amount
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// Cancel order
//func (g *GateIO) cancelOrder(orderNumber string, currencyPair string) string {
//	var method string = "POST"
//	var url string = DataSource + "/private/cancelOrder"
//	var param string = "orderNumber=" + orderNumber + "&currencyPair=" + currencyPair
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}

// Cancel all orders
//func (g *GateIO) cancelAllOrders(types string, currencyPair string) string {
//	var method string = "POST"
//	var url string = DataSource + "/private/cancelAllOrders"
//	var param string = "type=" + types + "&currencyPair=" + currencyPair
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}

// Get order status
//func (g *GateIO) getOrder(orderNumber string, currencyPair string) string {
//	var method string = "POST"
//	var url string = DataSource + "/private/getOrder"
//	var param string = "orderNumber=" + orderNumber + "&currencyPair=" + currencyPair
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}

// Get my open order list
//func (g *GateIO) openOrders() string {
//	var method string = "POST"
//	var url string = DataSource + "/private/openOrders"
//	var param string = ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}

// 获取我的24小时内成交记录
func (g *GateIO) MyTradeHistory(currencyPair string, orderNumber string) (*MyTradeHistoryResult, error) {
	method := "POST"
	url := DataSource + "/private/TradeHistory"
	param := "orderNumber=" + orderNumber + "&currencyPair=" + currencyPair
	data, err := g.httpDo(method, url, param)
	if err != nil {
		return nil, err
	}
	var result MyTradeHistoryResult
	err = json.Unmarshal(data, &result)
	return &result, err
}

// Get my last 24h trades
//func (g *GateIO) withdraw(currency string, amount string, address string) string {
//	var method string = "POST"
//	var url string = DataSource + "/private/withdraw"
//	var param string = "currency=" + currency + "&amount=" + amount + "&address=" + address
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}

func (g *GateIO) getSign(params string) string {
	key := []byte(g.Secret)
	mac := hmac.New(sha512.New, key)
	mac.Write([]byte(params))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

/**
*  http request
*/
func (g *GateIO) httpDo(method string, url string, param string) ([]byte, error) {
	client := &http.Client{}

	req, err := http.NewRequest(method, url, strings.NewReader(param))
	if err != nil {
		return nil, err
	}
	sign := g.getSign(param)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("key", g.Key)
	req.Header.Set("sign", sign)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error: %s", err)
		return nil, err
	}

	return body, nil
}
