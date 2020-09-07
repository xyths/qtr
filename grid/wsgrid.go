package grid

import (
	"context"
	"fmt"
	"github.com/huobirdcenter/huobi_golang/pkg/model/order"
	"github.com/jinzhu/gorm"
	"github.com/xyths/qtr/exchange"
	"github.com/xyths/qtr/exchange/huobi"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"math"
	"strconv"
)

const (
	ClientId = "ws-grid-v0.1.0"
)

type WsGrid struct {
	ExchangeName string              `bson:"exchange"`
	Label        string              `bson:"label"`
	Pair         string              `bson:"pair"`
	APIKeyPair   exchange.APIKeyPair `bson:"-"`

	ExClient   exchange.Exchange `bson:"-"`
	MongClient *mongo.Database   `bson:"-"`
	GormClient *gorm.DB          `bson:"-"`

	Percentage      float64 `bson:"-"`
	Fund            float64 `bson:"-"`
	MaxGrid         int     `bson:"-"`
	PricePrecision  int     `bson:"-"`
	AmountPrecision int     `bson:"-"`

	Base          float64 `bson:"base"`
	Position      int     `bson:"position"`
	UpTimes       int     `bson:"upTimes"`
	DownTimes     int     `bson:"downTimes"`
	TopOrderId    uint64  `bson:"topOrderId"`
	BottomOrderId uint64  `bson:"bottomOrderId"`
}

func (g *WsGrid) Init(ctx context.Context) error {
	switch g.ExchangeName {
	case "huobi":
		g.ExClient = huobi.NewClient(huobi.Config{
			Label:        g.Label,
			AccessKey:    g.APIKeyPair.ApiKey,
			SecretKey:    g.APIKeyPair.SecretKey,
			CurrencyList: []string{"btc", "usdt", "ht"},
		})
		g.PricePrecision = huobi.PricePrecision[g.Pair]
		g.AmountPrecision = huobi.AmountPrecision[g.Pair]
	default:
		log.Fatalf("ws grid not support exchange %s", g.ExchangeName)
	}

	ok, err := g.load(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if !ok {
		ok, err := g.getBase(ctx)
		if err != nil {
			log.Fatal(err)
		}
		if !ok {
			log.Fatal("unknown error occurs when get base")
		}
	}

	return nil
}

func (g *WsGrid) Run(ctx context.Context) error {
	// 1. Order
	last, err := g.ExClient.LastPrice(g.Pair)
	if err != nil {
		log.Println(err)
		return err
	}
	g.PlaceOrders(ctx, last)

	// 2. Subscribe
	if err := g.ExClient.SubscribeOrders(ClientId, g.OrderUpdateHandler); err != nil {
		log.Println(err)
	}

	return nil
}

const CollName = "grid_status"

// load base from mongo db
func (g *WsGrid) load(ctx context.Context) (bool, error) {
	coll := g.MongClient.Collection(CollName)
	err := coll.FindOne(ctx, bson.D{
		{"exchange", g.ExchangeName},
		{"label", g.Label},
		{"pair", g.Pair},
	}).Decode(&g)
	if err != nil {
		// ErrNoDocuments means that the filter did not match any documents in the collection
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		log.Print(err)
		return false, err
	}
	log.Printf("[INFO] Load grid status: base %f, position %d, upTimes %d, downTimes %d, topOrder %d, bottomOrder %d",
		g.Base, g.Position, g.UpTimes, g.DownTimes, g.TopOrderId, g.BottomOrderId)
	return true, nil
}

func (g *WsGrid) save(ctx context.Context) error {
	coll := g.MongClient.Collection(CollName)
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(ctx,
		bson.D{
			{"exchange", g.ExchangeName},
			{"label", g.Label},
			{"pair", g.Pair},
		},
		bson.D{
			{"$set", bson.D{
				{"base", g.Base},
				{"position", g.Position},
				{"upTimes", g.UpTimes},
				{"downTimes", g.DownTimes},
				{"topOrderId", g.TopOrderId},
				{"bottomOrderId", g.BottomOrderId},
			}},
			{"$currentDate", bson.D{
				{"lastModified", true},
			}},
		}, opts)
	if err != nil {
		log.Println(err)
	}
	return err
}

func (g *WsGrid) getBase(ctx context.Context) (bool, error) {
	price, err := g.ExClient.LastPrice(g.Pair)
	ok := false
	if err == nil {
		g.Base = price
		ok = true
	}
	return ok, err
}

func (g *WsGrid) Shutdown(ctx context.Context) error {
	// nothing to do till now
	return nil
}

// Response handler
func (g *WsGrid) OrderUpdateHandler(resp interface{}) {
	subResponse, ok := resp.(order.SubscribeOrderV2Response)
	if ok {
		if &subResponse.Data != nil {
			o := subResponse.Data
			fmt.Printf("Order update, symbol: %s, order id: %d, price: %s, volume: %s",
				o.Symbol, o.OrderId, o.TradePrice, o.TradeVolume)
		}
	} else {
		fmt.Printf("Received unknown response: %v\n", resp)
	}
}

func (g *WsGrid) PlaceOrders(ctx context.Context, last float64) {
	if g.TopOrderId == 0 {
		g.placeTopOrder(last)
		_ = g.save(ctx)
	}
	if g.BottomOrderId == 0 {
		g.placeBottomOrder(last)
		_ = g.save(ctx)
	}
}

func (g *WsGrid) placeTopOrder(last float64) {
	price, amount := g.top()
	price = math.Max(price, last)
	orderId, err := g.ExClient.Sell(g.Pair, price, amount)
	if err != nil {
		log.Printf("error when order top, price: %f, amount: %f, error: %s", price, amount, err)
		return
	}
	g.TopOrderId = orderId
	log.Printf("[INFO] placeTopOrder: price %f, amount %f, orderNumber %d", price, amount, g.TopOrderId)
}

func (g *WsGrid) placeBottomOrder(last float64) {
	price, amount := g.bottom()
	price = math.Min(price, last)
	orderId, err := g.ExClient.Buy(g.Pair, price, amount)
	if err != nil {
		log.Printf("error when order bottom, price: %f, amount: %f, error: %s", price, amount, err)
		return
	}
	g.BottomOrderId = orderId
	log.Printf("[INFO] placeBottomOrder: price %f, amount %f, orderNumber %d", price, amount, g.TopOrderId)
}

func (g *WsGrid) cancelOrder(orderNumber uint64) error {
	return g.ExClient.CancelOrder(orderNumber)
}

func (g *WsGrid) cancelTop() error {
	err := g.cancelOrder(g.TopOrderId)
	if err != nil {
		log.Printf("cancel order %d error: %s", g.TopOrderId, err)
		return err
	}
	g.TopOrderId = 0
	return nil
}

func (g *WsGrid) cancelBottom() error {
	err := g.cancelOrder(g.BottomOrderId)
	if err != nil {
		log.Printf("cancel order %d error: %s", g.TopOrderId, err)
		return err
	}
	g.BottomOrderId = 0
	return nil
}

func (g *WsGrid) CancelOrders() {
	_ = g.cancelTop()
	_ = g.cancelBottom()
}

func (g *WsGrid) top() (price, amount float64) {
	price = g.Base / (1 - g.Percentage)
	price = g.Truncate(price, g.PricePrecision)
	amount = g.Fund / g.Base
	amount = g.Truncate(amount, g.AmountPrecision)
	return
}

func (g *WsGrid) bottom() (price, amount float64) {
	price = g.Base * (1 - g.Percentage)
	price = g.Truncate(price, g.PricePrecision)
	amount = g.Fund / price
	amount = g.Truncate(amount, g.AmountPrecision)
	return
}

func (g *WsGrid) Truncate(value float64, precision int) float64 {
	str := fmt.Sprintf("%."+strconv.Itoa(precision)+"f", value)
	newValue, _ := strconv.ParseFloat(str, 64)
	return newValue
}
