package okex

import (
	"github.com/nntaoli-project/goex"
	"github.com/nntaoli-project/goex/builder"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestGoexByOkex(t *testing.T) {
	passphrase := os.Getenv("OKEX_PASSPHRASE")
	apiKey := os.Getenv("OKEX_APIKEY")
	secretKey := os.Getenv("OKEX_SECRETKEY")
	//apiBuilder := builder.NewAPIBuilder().HttpTimeout(5 * time.Second)
	apiBuilder := builder.NewAPIBuilder().HttpTimeout(5 * time.Second).HttpProxy("socks5://127.0.0.1:7070")

	//build spot api
	//api := apiBuilder.APIKey("").APISecretkey("").ClientID("123").Build(goex.BITSTAMP)
	api := apiBuilder.ApiPassphrase(passphrase).APIKey(apiKey).APISecretkey(secretKey).Build(goex.OKEX)
	t.Logf("GetExchangeName: %s", api.GetExchangeName())
	ticker, err := api.GetTicker(goex.BTC_USD)
	require.NoError(t, err)
	t.Logf("GetTicker(goex.BTC_USD): %#v", ticker)
	depth, err := api.GetDepth(2, goex.BTC_USD)
	require.NoError(t, err)
	t.Logf("GetDepth(2, goex.BTC_USD): %#v", depth)
	account, err := api.GetAccount()
	require.NoError(t, err)
	t.Logf("GetAccount(): %#v", account)
	orders, err := api.GetUnfinishOrders(goex.BTC_USD)
	require.NoError(t, err)
	t.Logf("GetUnfinishOrders(goex.BTC_USD): %#v", orders)

	//build future api
	//futureApi := apiBuilder.APIKey("").APISecretkey("").BuildFuture(goex.HBDM)
	//log.Println(futureApi.GetExchangeName())
	//log.Println(futureApi.GetFutureTicker(goex.BTC_USD, goex.QUARTER_CONTRACT))
	//log.Println(futureApi.GetFutureDepth(goex.BTC_USD, goex.QUARTER_CONTRACT, 5))
	//log.Println(futureApi.GetFutureUserinfo()) // account
	//log.Println(futureApi.GetFuturePosition(goex.BTC_USD , goex.QUARTER_CONTRACT))//position info
}
