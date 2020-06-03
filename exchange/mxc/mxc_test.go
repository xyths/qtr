package mxc

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

const BaseUrl = "https://www.mxc.ai"

func TestMXC_Timestamp(t *testing.T) {
	mxc := MXC{
		Domain: BaseUrl,
		Key:    "xxx",
		Secret: "yyy",
	}
	if timestamp, err := mxc.Timestamp(); err != nil {
		t.Log(err)
	} else {
		t.Logf("timestamp: %d", timestamp)
	}
}

func getApiKey() (apiKey, secretKey string) {
	apiKey = os.Getenv("apiKey")
	secretKey = os.Getenv("secretKey")
	return apiKey, secretKey
}

func TestMXC_Deals(t *testing.T) {
	apiKey, secretKey := getApiKey()
	mxc := MXC{
		Domain: BaseUrl,
		Key:    apiKey,
		Secret: secretKey,
	}
	deals, err := mxc.Deals("BTC_USDT", "1000", "1585904212")
	require.NoError(t, err)
	t.Logf("deals: %#v", deals)
}

func TestMXC_OpenOrders(t *testing.T) {
	apiKey, secretKey := getApiKey()
	mxc := MXC{
		Domain: BaseUrl,
		Key:    apiKey,
		Secret: secretKey,
	}
	orders, err := mxc.OpenOrders("BTC_USDT", "1000", "1585904212")
	require.NoError(t, err)
	t.Logf("orders: %#v", orders)
}
