package gateio

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/xyths/hs/convert"
	"github.com/xyths/qtr/exchange"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

const DefaultDataSource = "https://gatecn.io/api2/1"

type GateIO struct {
	DataSource string

	Key    string
	Secret string
}

func NewGateIO(dataSource, key, secret string) *GateIO {
	return &GateIO{DataSource: dataSource, Key: key, Secret: secret}
}

const (
	GET  = "GET"
	POST = "POST"
)

// all support pairs
func (g *GateIO) GetPairs() (string, error) {
	url := g.DataSource + "/pairs"
	param := ""
	if ret, err := g.httpDo(GET, url, param); err != nil {
		return "", err
	} else {
		return string(ret), err
	}
}

// Market Info
//func (g *GateIO) marketinfo() string {
//	var method string = "GET"
//	url := g.DataSource + "/marketinfo"
//	param := ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// Market Details
//func (g *GateIO) marketlist() string {
//	var method string = "GET"
//	url := g.DataSource + "/marketlist"
//	param := ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// tickers
//func (g *GateIO) tickers() string {
//	var method string = "GET"
//	url := g.DataSource + "/tickers"
//	param := ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
// ticker
func (g *GateIO) Ticker(currencyPair string) (ticker *exchange.Ticker, err error) {
	url := g.DataSource + "/ticker" + "/" + currencyPair
	param := ""
	var t ResponseTicker
	if err = g.request(GET, url, param, &t); err != nil {
		return
	}
	ticker = &exchange.Ticker{
		Last:          convert.StrToFloat64(t.Last),
		LowestAsk:     convert.StrToFloat64(t.LowestAsk),
		HighestBid:    convert.StrToFloat64(t.HighestBid),
		PercentChange: convert.StrToFloat64(t.PercentChange),
		BaseVolume:    convert.StrToFloat64(t.BaseVolume),
		QuoteVolume:   convert.StrToFloat64(t.QuoteVolume),
		High24hr:      convert.StrToFloat64(t.High24hr),
		Low24hr:       convert.StrToFloat64(t.Low24hr),
	}
	return
}

//// Depth
//func (g *GateIO) orderBooks() string {
//	var method string = "GET"
//	url := g.DataSource + "/orderBooks"
//	param := ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// Depth of pair
//func (g *GateIO) orderBook(params string) string {
//	var method string = "GET"
//	url := g.DataSource + "/orderBook/" + params
//	param := ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//

// 获取Candle
func (g *GateIO) Candles(currencyPair string, groupSec, rangeHour int) (candles []exchange.Candle, err error) {
	url := fmt.Sprintf("%s/candlestick2/%s?group_sec=%d&range_hour=%d", g.DataSource, currencyPair, groupSec, rangeHour)
	param := ""

	var result ResponseCandles
	err = g.request(GET, url, param, &result)
	if err != nil {
		return nil, err
	}
	for _, c := range result.Data {
		candles = append(candles, exchange.Candle{
			Timestamp: convert.StrToUint64(c[0]),
			Volume:    convert.StrToFloat64(c[1]),
			Close:     convert.StrToFloat64(c[2]),
			High:      convert.StrToFloat64(c[3]),
			Low:       convert.StrToFloat64(c[4]),
			Open:      convert.StrToFloat64(c[5]),
		})
	}
	return
}

// Trade History
func (g *GateIO) TradeHistory(params string) (string, error) {
	url := g.DataSource + "/TradeHistory/" + params
	param := ""
	data, err := g.httpDo(GET, url, param)
	if err != nil {
		return "", err
	} else {
		return string(data), err
	}
}

// Get account fund balances
func (g *GateIO) Balances() (*ResponseBalances, error) {
	url := g.DataSource + "/private/balances"
	param := ""
	data, err := g.httpDo(POST, url, param)
	if err != nil {
		return nil, err
	}
	var result ResponseBalances
	err = json.Unmarshal(data, &result)
	return &result, err
}

//// get deposit address
//func (g *GateIO) depositAddress(currency string) string {
//	var method string = "POST"
//	url := g.DataSource + "/private/depositAddress"
//	param := "currency=" + currency
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// get deposit withdrawal history
//func (g *GateIO) depositsWithdrawals(start string, end string) string {
//	var method string = "POST"
//	url := g.DataSource + "/private/depositsWithdrawals"
//	param := "start=" + start + "&end=" + end
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
// Place order buy
func (g *GateIO) Buy(currencyPair string, rate, amount float64) (res ResponseBuy, err error) {
	url := g.DataSource + "/private/buy"
	// 价格精度：5，数量精度：3
	param := fmt.Sprintf("currencyPair=%s&rate=%.5f&amount=%.3f", currencyPair, rate, amount)
	err = g.request(POST, url, param, &res)
	return
}

// Place order sell
func (g *GateIO) Sell(currencyPair string, rate, amount float64) (res ResponseSell, err error) {
	url := g.DataSource + "/private/sell"
	// 价格精度：5，数量精度：3
	param := fmt.Sprintf("currencyPair=%s&rate=%.5f&amount=%.3f", currencyPair, rate, amount)
	err = g.request(POST, url, param, &res)
	return
}

// Cancel order
func (g *GateIO) CancelOrder(currencyPair string, orderNumber uint64) (ok bool, err error) {
	url := g.DataSource + "/private/cancelOrder"
	param := fmt.Sprintf("currencyPair=%s&orderNumber=%d", currencyPair, orderNumber)
	var res ResponseCancel
	err = g.request(POST, url, param, &res)
	ok = res.Result
	return
}

// Cancel all orders
func (g *GateIO) CancelAllOrders(types string, currencyPair string) (res ResponseCancel, err error) {
	url := g.DataSource + "/private/cancelAllOrders"
	param := "type=" + types + "&currencyPair=" + currencyPair
	err = g.request(POST, url, param, &res)
	return
}

// Get order status
func (g *GateIO) GetOrder(orderNumber uint64, currencyPair string) (order exchange.Order, err error) {
	url := g.DataSource + "/private/getOrder"
	param := fmt.Sprintf("orderNumber=%d&currencyPair=%s", orderNumber, currencyPair)
	var res ResponseGetOrder
	err = g.request(POST, url, param, &res)
	if err != nil {
		return
	}
	if res.Result != "true" || res.Message != "Success" {
		log.Printf("request not success: %#v", res)
		return order, errors.New(res.Message)
	}
	o := &res.Order
	order.OrderNumber = convert.StrToUint64(o.OrderNumber)
	order.Status = o.Status

	return
}

// Get my open order list
func (g *GateIO) OpenOrders() (res ResponseOpenOrders, err error) {
	url := g.DataSource + "/private/openOrders"
	param := ""
	err = g.request(POST, url, param, &res)
	return
}

// 获取我的24小时内成交记录
func (g *GateIO) MyTradeHistory(currencyPair string) (*MyTradeHistoryResult, error) {
	method := "POST"
	url := g.DataSource + "/private/TradeHistory"
	param := "orderNumber=&currencyPair=" + currencyPair
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
//	url := g.DataSource + "/private/withdraw"
//	param := "currency=" + currency + "&amount=" + amount + "&address=" + address
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

func (g *GateIO) request(method string, url string, param string, result interface{}) error {
	client := &http.Client{}

	req, err := http.NewRequest(method, url, strings.NewReader(param))
	if err != nil {
		return err
	}
	sign := g.getSign(param)

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("key", g.Key)
	req.Header.Set("sign", sign)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	}()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		//log.Printf("error: %s", err)
		return err
	}
	if err = json.Unmarshal(data, result); err != nil {
		log.Printf("raw response: %s", string(data))
	}
	return err
}
