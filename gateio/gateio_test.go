package gateio

import (
	"bytes"
	"encoding/json"
	"github.com/shopspring/decimal"
	"os"
	"strconv"
	"testing"
)

// apiKey=xxx secretKey=yyy go test -v ./gateio

// apiKey=xxx secretKey=yyy go test -v -run TestGetPairs ./gateio
func TestGetPairs(t *testing.T) {
	apiKey := os.Getenv("apiKey")
	secretKey := os.Getenv("secretKey")
	host := os.Getenv("host")
	t.Logf("apiKey: %s, secretKey: %s", apiKey, secretKey)
	gateio := New(apiKey, secretKey, host)

	if pairs, err := gateio.GetPairs(); err != nil {
		t.Logf("error when GetPairs: %s", err)
	} else {
		t.Logf("GetPairs: %s", pairs)
	}
}

// apiKey=xxx secretKey=yyy go test -v -run TestTradeHistory ./gateio
func TestTradeHistory(t *testing.T) {
	apiKey := os.Getenv("apiKey")
	secretKey := os.Getenv("secretKey")
	host := os.Getenv("host")
	t.Logf("apiKey: %s, secretKey: %s", apiKey, secretKey)
	gateio := New(apiKey, secretKey, host)
	if history, err := gateio.TradeHistory("SERO_USDT"); err != nil {
		t.Logf("error when TradeHistory: %s", err)
	} else {
		t.Logf("TradeHistory(SERO_USDT): %s", history)
	}
}

// apiKey=xxx secretKey=yyy go test -v -run TestGateIO_MyTradeHistory ./gateio
func TestGateIO_MyTradeHistory(t *testing.T) {
	apiKey := os.Getenv("apiKey")
	secretKey := os.Getenv("secretKey")
	host := os.Getenv("host")
	t.Logf("apiKey: %s, secretKey: %s", apiKey, secretKey)
	gateio := New(apiKey, secretKey, host)
	if history, err := gateio.MyTradeHistory("SERO_USDT"); err != nil {
		t.Logf("error when TradeHistory: %s", err)
	} else {
		var buf bytes.Buffer
		e := json.NewEncoder(&buf)
		e.SetIndent("", "\t")
		if err1 := e.Encode(history); err1 != nil {
			t.Logf("error when encode history: %s", err1)
		}
		t.Logf("MyTradeHistory(\"SERO_USDT\", \"\"): %s", buf.String())
	}
}

// apiKey=xxx secretKey=yyy go test -v -run TestGateIO_Candles ./gateio
func TestGateIO_Candles(t *testing.T) {
	apiKey := os.Getenv("apiKey")
	secretKey := os.Getenv("secretKey")
	host := os.Getenv("host")
	t.Logf("apiKey: %s, secretKey: %s", apiKey, secretKey)
	gateio := New(apiKey, secretKey, host)
	if candles, err := gateio.Candles("SERO_USDT", 60, 10); err != nil {
		t.Logf("error when Candles: %s", err)
	} else {
		t.Log(candles)
	}
}

// apiKey=xxx secretKey=yyy go test -v -run TestGateIO_Balances ./gateio
func TestGateIO_Balances(t *testing.T) {
	apiKey := os.Getenv("apiKey")
	secretKey := os.Getenv("secretKey")
	host := os.Getenv("host")
	gateio := New(apiKey, secretKey, host)

	balances, err := gateio.AvailableBalance()
	if err != nil {
		t.Logf("error when AvailableBalance: %s", err)
	}
	t.Logf("balances is %#v", balances)
}

// apiKey=xxx secretKey=yyy go test -v -run TestGateIO_Buy ./gateio
func TestGateIO_Buy(t *testing.T) {
	apiKey := os.Getenv("apiKey")
	secretKey := os.Getenv("secretKey")
	host := os.Getenv("host")
	gateio := New(apiKey, secretKey, host)

	res, err := gateio.Buy("SERO_USDT",
		decimal.NewFromFloat(0.023456), decimal.NewFromFloat(100.123456),
		"normal", "test")
	if err != nil {
		t.Logf("error: %s", err)
		return
	}
	t.Logf("res is %#v", res)
}

// apiKey=xxx secretKey=yyy orderNumber=zzz go test -v -run TestGateIO_GetOrder ./gateio
func TestGateIO_GetOrder(t *testing.T) {
	apiKey := os.Getenv("apiKey")
	secretKey := os.Getenv("secretKey")
	host := os.Getenv("host")
	orderNumber, _ := strconv.Atoi(os.Getenv("orderNumber"))
	gateio := New(apiKey, secretKey, host)

	res, err := gateio.GetOrder(uint64(orderNumber), "SERO_USDT")
	if err != nil {
		t.Logf("error: %s", err)
		return
	}
	t.Logf("res is %#v", res)
}

// apiKey=xxx secretKey=yyy go test -v -run TestGateIO_OpenOrders ./gateio
func TestGateIO_OpenOrders(t *testing.T) {
	apiKey := os.Getenv("apiKey")
	secretKey := os.Getenv("secretKey")
	host := os.Getenv("host")
	gateio := New(apiKey, secretKey, host)

	//_ = gateio
	res, err := gateio.OpenOrders()
	if err != nil {
		t.Logf("error: %s", err)
	}
	t.Logf("res is %#v", res)
}

// apiKey=xxx secretKey=yyy go test -v -run TestGateIO_Ticker ./gateio
func TestGateIO_Ticker(t *testing.T) {
	apiKey := os.Getenv("apiKey")
	secretKey := os.Getenv("secretKey")
	host := os.Getenv("host")
	gateio := New(apiKey, secretKey, host)

	//_ = gateio
	res, err := gateio.Ticker("SERO_USDT")
	if err != nil {
		t.Logf("error: %s", err)
	}
	t.Logf("SERO_USDT ticker is %#v", res)
}

func TestFloatPrint(t *testing.T) {
	testPairs := []float64{
		3.3,
		0.123456,
		111111.12345,
		1.235,
	}
	for _, f := range testPairs {
		t.Logf("%f => %.2f", f, f)
	}
}
