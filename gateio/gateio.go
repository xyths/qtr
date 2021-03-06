package gateio

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/convert"
	"github.com/xyths/qtr/exchange"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

const (
	DefaultHost = "gateio.life"
)

type GateIO struct {
	Key    string
	Secret string

	publicBaseUrl  string
	privateBaseUrl string
}

func New(key, secret, host string) *GateIO {
	g := &GateIO{Key: key, Secret: secret}
	if host == "" {
		host = DefaultHost
	}
	g.publicBaseUrl = "https://data." + host + "/api2/1"
	g.privateBaseUrl = "https://api." + host + "/api2/1"
	return g
}

const (
	GET  = "GET"
	POST = "POST"
)

// all support pairs
func (g *GateIO) GetPairs() (string, error) {
	url := "/pairs"
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
//	url :=  "/marketinfo"
//	param := ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// Market Details
//func (g *GateIO) marketlist() string {
//	var method string = "GET"
//	url := "/marketlist"
//	param := ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// tickers
//func (g *GateIO) tickers() string {
//	var method string = "GET"
//	url := "/tickers"
//	param := ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
// ticker
func (g *GateIO) Ticker(currencyPair string) (*exchange.Ticker, error) {
	url := "/ticker" + "/" + currencyPair
	param := ""
	var t ResponseTicker
	if err := g.request(GET, url, param, &t); err != nil {
		return nil, err
	}
	ticker := &exchange.Ticker{
		Last:          decimal.RequireFromString(t.Last),
		LowestAsk:     decimal.RequireFromString(t.LowestAsk),
		HighestBid:    decimal.RequireFromString(t.HighestBid),
		PercentChange: decimal.RequireFromString(t.PercentChange),
		BaseVolume:    decimal.RequireFromString(t.BaseVolume),
		QuoteVolume:   decimal.RequireFromString(t.QuoteVolume),
		High24hr:      decimal.RequireFromString(t.High24hr),
		Low24hr:       decimal.RequireFromString(t.Low24hr),
	}
	return ticker, nil
}

func (g *GateIO) LastPrice(symbol string) (decimal.Decimal, error) {
	ticker, err := g.Ticker(symbol)
	if err != nil {
		return decimal.Zero, err
	}
	return ticker.Last, nil
}

//// Depth
//func (g *GateIO) orderBooks() string {
//	var method string = "GET"
//	url := "/orderBooks"
//	param := ""
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}

// Depth of pair
func (g *GateIO) OrderBook(symbol string) (ResponseOrderBook, error) {
	var method string = "GET"
	url := "/orderBook/" + symbol
	param := ""

	var result ResponseOrderBook
	err := g.request(method, url, param, &result)
	return result, err
}

// 获取Candle
func (g *GateIO) Candles(currencyPair string, groupSec, rangeHour int) (candles []exchange.Candle, err error) {
	url := fmt.Sprintf("/candlestick2/%s?group_sec=%d&range_hour=%d", currencyPair, groupSec, rangeHour)
	param := ""

	var result ResponseCandles
	err = g.request(GET, url, param, &result)
	if err != nil {
		return nil, err
	}
	for _, c := range result.Data {
		candles = append(candles, exchange.Candle{
			Timestamp: uint64(c[0]),
			Volume:    decimal.NewFromFloat(c[1]),
			Close:     decimal.NewFromFloat(c[2]),
			High:      decimal.NewFromFloat(c[3]),
			Low:       decimal.NewFromFloat(c[4]),
			Open:      decimal.NewFromFloat(c[5]),
		})
	}
	return
}

// 获取Candle
func (g *GateIO) GetCandle(symbol string, groupSec, rangeHour int) (candles hs.Candle, err error) {
	url := fmt.Sprintf("/candlestick2/%s?group_sec=%d&range_hour=%d", symbol, groupSec, rangeHour)
	param := ""

	var result ResponseCandles
	err = g.request(GET, url, param, &result)
	if err != nil {
		return candles, err
	}
	candles = hs.NewCandle(len(result.Data))
	for i := 0; i < len(result.Data); i++ {
		c := result.Data[i]
		candles.Append(hs.Ticker{
			Timestamp: int64(c[0]),
			Volume:    c[1],
			Close:     c[2],
			High:      c[3],
			Low:       c[4],
			Open:      c[5],
		})
	}
	return
}

// Trade History
func (g *GateIO) TradeHistory(params string) (string, error) {
	url := "/TradeHistory/" + params
	param := ""
	data, err := g.httpDo(GET, url, param)
	if err != nil {
		return "", err
	} else {
		return string(data), err
	}
}

// Get account fund balances
func (g *GateIO) AvailableBalance() (map[string]decimal.Decimal, error) {
	url := "/private/balances"
	param := ""
	data, err := g.httpDo(POST, url, param)
	if err != nil {
		return nil, err
	}
	var result ResponseBalances
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	balance := make(map[string]decimal.Decimal)
	for k, v := range result.Available {
		b := decimal.RequireFromString(v)
		if b.IsZero() {
			continue
		}
		if ob, ok := balance[k]; ok {
			balance[k] = ob.Add(b)
		} else {
			balance[k] = b
		}
	}
	return balance, nil
}

//// get deposit address
//func (g *GateIO) depositAddress(currency string) string {
//	var method string = "POST"
//	url := "/private/depositAddress"
//	param := "currency=" + currency
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//
//// get deposit withdrawal history
//func (g *GateIO) depositsWithdrawals(start string, end string) string {
//	var method string = "POST"
//	url := "/private/depositsWithdrawals"
//	param := "start=" + start + "&end=" + end
//	var ret string = g.httpDo(method, url, param)
//	return ret
//}
//

// 订单类型("gtc"：普通订单（默认）；“ioc”：立即执行否则取消订单（Immediate-Or-Cancel，IOC）；"poc":被动委托（只挂单，不吃单）（Pending-Or-Cancelled，POC）)
// Place order buy
func (g *GateIO) Buy(symbol string, price, amount decimal.Decimal, orderType, text string) (orderId uint64, err error) {
	resp, err := g.BuyOrder(symbol, price, amount, orderType, text)
	if err != nil {
		return 0, err
	}
	if resp.Result == "false" || resp.OrderNumber == 0 {
		return 0, errors.New(resp.Message)
	}
	return resp.OrderNumber, nil
}

func (g *GateIO) BuyOrder(symbol string, price, amount decimal.Decimal, orderType, text string) (resp ResponseOrder, err error) {
	url := "/private/buy"
	param := fmt.Sprintf("currencyPair=%s&rate=%s&amount=%s&orderType=%s&text=t-%s", symbol, price, amount, orderType, text)
	err = g.request(POST, url, param, &resp)
	return
}

// Place order sell
func (g *GateIO) Sell(symbol string, price, amount decimal.Decimal, orderType, text string) (orderId uint64, err error) {
	url := "/private/sell"
	// 价格精度：5，数量精度：3
	param := fmt.Sprintf("currencyPair=%s&rate=%s&amount=%s&orderType=%s&text=t-%s", symbol, price, amount, orderType, text)
	var res ResponseOrder
	err = g.request(POST, url, param, &res)
	if err != nil {
		return 0, err
	}
	if res.Result == "false" || res.OrderNumber == 0 {
		return 0, errors.New(res.Message)
	}
	return res.OrderNumber, nil
}

// Cancel order
func (g *GateIO) CancelOrder(currencyPair string, orderNumber uint64) (ok bool, err error) {
	url := "/private/cancelOrder"
	param := fmt.Sprintf("currencyPair=%s&orderNumber=%d", currencyPair, orderNumber)
	var res ResponseCancel
	err = g.request(POST, url, param, &res)
	ok = res.Result
	return
}

// Cancel all orders
func (g *GateIO) CancelAllOrders(types string, currencyPair string) (res ResponseCancel, err error) {
	url := "/private/cancelAllOrders"
	param := "type=" + types + "&currencyPair=" + currencyPair
	err = g.request(POST, url, param, &res)
	return
}

// Get order status
func (g *GateIO) GetOrder(orderNumber uint64, currencyPair string) (order exchange.Order, err error) {
	url := "/private/getOrder"
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
	order.CurrencyPair = o.CurrencyPair
	order.Type = o.Type
	order.InitialRate = decimal.RequireFromString(o.InitialRate)
	order.InitialAmount = decimal.RequireFromString(o.InitialAmount)
	order.Status = o.Status
	order.Rate = decimal.RequireFromString(o.Rate)
	// amount maybe 0
	order.Amount = decimal.RequireFromString(o.Amount)
	//order.FilledRate=decimal.RequireFromString(o.FilledRate)
	order.FilledAmount = decimal.RequireFromString(o.FilledAmount)
	order.FeePercentage = o.FeePercentage
	order.FeeValue = decimal.RequireFromString(o.FeeValue)
	order.Timestamp = o.Timestamp

	return
}

func (g *GateIO) IsOrderClose(symbol string, orderId uint64) (order exchange.Order, closed bool) {
	o, err := g.GetOrder(orderId, symbol)
	if err != nil {
		return o, false
	}
	if o.Status == OrderStatusClosed {

		return o, true
	}
	return o, false
}

func (g *GateIO) Broadcast(order exchange.Order) {

}

// Get my open order list
func (g *GateIO) OpenOrders() (res ResponseOpenOrders, err error) {
	url := "/private/openOrders"
	param := ""
	err = g.request(POST, url, param, &res)
	return
}

// 获取我的24小时内成交记录
func (g *GateIO) MyTradeHistory(currencyPair string) (*MyTradeHistoryResult, error) {
	method := "POST"
	url := "/private/TradeHistory"
	param := "orderNumber=&currencyPair=" + currencyPair
	var result MyTradeHistoryResult
	if err := g.request(method, url, param, &result); err != nil {
		return nil, err
	} else {
		return &result, nil
	}
}

// Get my last 24h trades
//func (g *GateIO) withdraw(currency string, amount string, address string) string {
//	var method string = "POST"
//	url := "/private/withdraw"
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
	if method == GET {
		url = g.publicBaseUrl + url
	} else if method == POST {
		url = g.privateBaseUrl + url
	} else {
		return nil, errors.New("unknown method")
	}

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
	if method == GET {
		url = g.publicBaseUrl + url
	} else if method == POST {
		url = g.privateBaseUrl + url
	} else {
		return errors.New("unsupported method")
	}

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
