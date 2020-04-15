package huobi

import (
	"context"
	"os"
	"testing"
)

func TestClient_GetTimestamp(t *testing.T) {
	client := NewClient(Config{})
	if time, err := client.GetTimestamp(); err != nil {
		t.Log(err)
	} else {
		t.Logf("timestamp is: %d", time)
	}
}

func TestClient_GetAccountInfo(t *testing.T) {
	client := NewClient(Config{
		Label:     "test",
		AccessKey: os.Getenv("ACCESS_KEY"),
		SecretKey: os.Getenv("SECRET_KEY"),
	})
	accounts, err := client.GetAccountInfo();
	if err != nil {
		t.Log(err)
	}

	for _, a := range accounts {
		t.Logf("account is: %#v", a)
	}
}

func TestClient_Balances(t *testing.T) {
	client := NewClient(Config{
		Label:        "test",
		AccessKey:    os.Getenv("ACCESS_KEY"),
		SecretKey:    os.Getenv("SECRET_KEY"),
		CurrencyList: []string{"usdt", "eth", "eos", "ht"},
	})
	balances, err := client.Balances();
	if err != nil {
		t.Log(err)
	}
	for currency, balance := range balances {
		t.Logf("%s: %f", currency, balance)
	}
}

func TestClient_Snapshot(t *testing.T) {
	client := NewClient(Config{
		Label:        "test",
		AccessKey:    os.Getenv("ACCESS_KEY"),
		SecretKey:    os.Getenv("SECRET_KEY"),
		CurrencyList: []string{"usdt", "eth", "eos", "ht"},
	})
	var huobiBalance HuobiBalance
	err := client.Snapshot(context.Background(), &huobiBalance);
	if err != nil {
		t.Log(err)
	}
	t.Logf("huobiBalance: %#v", huobiBalance)
}
