package exchange

import (
	"context"
	"github.com/huobirdcenter/huobi_golang/pkg/client/websocketclientbase"
	"github.com/xyths/hs/exchange"
	"sort"
)

type Exchange interface {
	ExchangeName() string
	Label() string
	Snapshot(ctx context.Context, result interface{}) error
	LastPrice(symbol string) (float64, error)
	SubscribeOrders(clientId string, responseHandler websocketclientbase.ResponseHandler) error
	SubscribeBalanceUpdate(clientId string, responseHandler websocketclientbase.ResponseHandler) error
	Sell(symbol string, price, amount float64) (orderId uint64, err error)
	Buy(symbol string, price, amount float64) (orderId uint64, err error)
	CancelOrder(orderId uint64) error
}

type volume struct {
	Symbol string
	Vol    float64
}
type symbolVolume []volume

func (sv symbolVolume) Len() int {
	return len(sv)
}
func (sv symbolVolume) Swap(i, j int) {
	sv[i], sv[j] = sv[j], sv[i]
}
func (sv symbolVolume) Less(i, j int) bool {
	return sv[i].Vol < sv[j].Vol
}

// sort symbols by 24h trade volume
func SortByVol24h(ex exchange.RestAPIExchange, symbols []string) ([]string, error) {
	var vols symbolVolume
	for _, s := range symbols {
		vol1, err := ex.Last24hVolume(s)
		if err != nil {
			continue
		}
		if !vol1.IsPositive() {
			continue
		}
		vol2, _ := vol1.Float64()
		vols = append(vols, volume{
			Symbol: s,
			Vol:    vol2,
		})
	}
	sort.Sort(sort.Reverse(vols))
	ret := make([]string, len(vols))
	for i := 0; i < len(vols); i++ {
		ret[i] = vols[i].Symbol
	}
	return ret, nil
}
