package gateio

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// apiKey=xxx secretKey=yyy go test -v ./gateio

// apiKey=xxx secretKey=yyy go test -v -run TestGetPairs ./gateio
func TestGetPairs(t *testing.T) {
	apiKey := os.Getenv("apiKey")
	secretKey := os.Getenv("secretKey")
	t.Logf("apiKey: %s, secretKey: %s", apiKey, secretKey)
	gateio := NewGateIO(apiKey, secretKey)

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
	t.Logf("apiKey: %s, secretKey: %s", apiKey, secretKey)
	gateio := NewGateIO(apiKey, secretKey)
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
	t.Logf("apiKey: %s, secretKey: %s", apiKey, secretKey)
	gateio := NewGateIO(apiKey, secretKey)
	if history, err := gateio.MyTradeHistory("SERO_USDT", ""); err != nil {
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
