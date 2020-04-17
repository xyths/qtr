package exchange

import (
	"context"
	"github.com/huobirdcenter/huobi_golang/pkg/client/websocketclientbase"
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
