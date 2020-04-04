package grid

import (
	"context"
	"fmt"
	"github.com/xyths/qtr/exchange"
	"github.com/xyths/qtr/gateio"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"math"
	"strconv"
	"time"
)

const (
	CollName = "grid_status"
)

type Grid struct {
	Exchange   string
	Label      string
	Pair       string // 交易对
	APIKeyPair gateio.APIKeyPair `bson:"-"`

	Percentage      float64 `bson:"-"`
	Fund            float64 `bson:"-"`
	MaxGrid         int     `bson:"-"`
	PricePrecision  int     `bson:"-"`
	AmountPrecision int     `bson:"-"`

	DB *mongo.Database `bson:"-"`

	UpTimes   int     `bson:"upTimes"`
	DownTimes int     `bson:"downTimes"`
	Base      float64 `bson:"base"`

	topOrder    *exchange.Order `bson:"-"`
	bottomOrder *exchange.Order `bson:"-"`
}

func (g *Grid) Load(ctx context.Context) error {
	type gridStatus struct {
		Id primitive.ObjectID `bson:"_id"`

		Base      float64 `bson:"base"`
		UpTimes   int     `bson:"upTimes"`
		DownTimes int     `bson:"downTimes"`

		LastModified time.Time `bson:"lastModified"`
	}
	var s gridStatus
	coll := g.DB.Collection(CollName)
	err := coll.FindOne(ctx, bson.D{
		{"exchange", g.Exchange},
		{"label", g.Label},
		{"pair", g.Pair},
	}).Decode(&s)
	if err != nil {
		// ErrNoDocuments means that the filter did not match any documents in the collection
		if err == mongo.ErrNoDocuments {
			return nil
		}
		log.Print(err)
		return err
	}
	g.Base = s.Base
	g.UpTimes = s.UpTimes
	g.DownTimes = s.DownTimes
	log.Printf("[INFO] Load grid status: base %f, upTimes %d, downTimes %d", g.Base, g.UpTimes, g.DownTimes)
	return nil
}

func (g *Grid) Save(ctx context.Context) error {
	coll := g.DB.Collection(CollName)
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(ctx,
		bson.D{
			{"exchange", g.Exchange},
			{"label", g.Label},
			{"pair", g.Pair},
		},
		bson.D{
			{"$set", bson.D{
				{"base", g.Base},
				{"upTimes", g.UpTimes},
				{"downTimes", g.DownTimes},
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

func (g *Grid) DoWork(ctx context.Context) error {
	last, err := g.last()
	if err != nil {
		log.Printf("get last price error: %s", err)
		return err
	}
	if g.Base == 0 {
		g.Base = last
		_ = g.Save(ctx)
	}
	if g.topOrder == nil {
		g.orderTop(last)
	}
	if g.bottomOrder == nil {
		g.orderBottom(last)
	}
	// 如果向上突破，撤bottom，下双边新单
	if status, err := g.checkTopOrder(); err != nil {
		return err
	} else {
		if status == gateio.OrderStatusClosed {
			g.up(last)
			_ = g.Save(ctx)
		}
	}
	// 如果向下突破，撤Top，下双边新单
	if status, err := g.checkBottomOrder(); err != nil {
		return err
	} else {
		if status == gateio.OrderStatusClosed {
			g.down(last)
			_ = g.Save(ctx)
		}
	}
	return nil
}

func (g *Grid) up(last float64) bool {
	if g.UpTimes >= g.MaxGrid {
		log.Printf("已连续向上突破 %d 次，达到设置的最大次数(%d)。", g.UpTimes, g.MaxGrid)
		return false
	}
	if g.DownTimes > 0 {
		g.DownTimes = 0
	}
	g.UpTimes++

	newBase := g.Base / (1 - g.Percentage)

	g.Base = g.Truncate(newBase, g.PricePrecision)

	log.Printf("[INFO] base UP from %f to %f", g.Base, newBase)
	g.cancelBottom()
	g.orderTop(last)
	g.orderBottom(last)

	return true
}

func (g *Grid) down(last float64) bool {
	if g.DownTimes >= g.MaxGrid {
		log.Printf("已连续向下突破 %d 次，达到设置的最大次数(%d)。", g.DownTimes, g.MaxGrid)
		return false
	}
	if g.UpTimes > 0 {
		g.UpTimes = 0
	}
	g.DownTimes++

	newBase := g.Base * (1 - g.Percentage)

	g.Base = g.Truncate(newBase, g.PricePrecision)

	log.Printf("[INFO] base DOWN from %f to %f", g.Base, newBase)
	g.cancelTop()
	g.orderTop(last)
	g.orderBottom(last)
	return true
}

// 最后成交价格
func (g *Grid) last() (price float64, err error) {
	client := gateio.NewGateIO(g.APIKeyPair.ApiKey, g.APIKeyPair.SecretKey)
	if ticker, err := client.Ticker(g.Pair); err != nil {
		return price, err
	} else {
		return ticker.Last, err
	}
}

func (g *Grid) orderTop(last float64) {
	price, amount := g.top()
	price = math.Max(price, last)
	client := gateio.NewGateIO(g.APIKeyPair.ApiKey, g.APIKeyPair.SecretKey)
	res, err := client.Sell(g.Pair, price, amount)
	if err != nil {
		log.Printf("error when order top, price: %f, amount: %f, error: %s", price, amount, err)
		return
	}
	g.topOrder = &exchange.Order{
		OrderNumber: res.OrderNumber,
	}
	log.Printf("[INFO] orderTop: price %f, amount %f, orderNumber %d", price, amount, g.topOrder.OrderNumber)
}

func (g *Grid) orderBottom(last float64) {
	price, amount := g.bottom()
	price = math.Min(price, last)
	client := gateio.NewGateIO(g.APIKeyPair.ApiKey, g.APIKeyPair.SecretKey)
	res, err := client.Buy(g.Pair, price, amount)
	if err != nil {
		log.Printf("error when order bottom, price: %f, amount: %f, error: %s", price, amount, err)
		return
	}
	g.bottomOrder = &exchange.Order{
		OrderNumber: res.OrderNumber,
	}
	log.Printf("[INFO] orderBottom: price %f, amount %f, orderNumber %d", price, amount, g.topOrder.OrderNumber)
}

func (g *Grid) OrderBoth(last float64) {
	g.orderTop(last)
	g.orderBottom(last)
}

func (g *Grid) cancelOrder(orderNumber uint64) {
	client := gateio.NewGateIO(g.APIKeyPair.ApiKey, g.APIKeyPair.SecretKey)
	success, err := client.CancelOrder(g.Pair, orderNumber)
	if err != nil {
		log.Printf("cancel order %d error: %s", orderNumber, err)
		return
	}
	if !success {
		log.Printf("cancel order %d failed", orderNumber)
		return
	}
}

func (g *Grid) cancelTop() {
	g.cancelOrder(g.topOrder.OrderNumber)
	g.topOrder = nil
}

func (g *Grid) cancelBottom() {
	g.cancelOrder(g.bottomOrder.OrderNumber)
	g.bottomOrder = nil
}

func (g *Grid) cancelBoth() {
	g.cancelTop()
	g.cancelBottom()
}

func (g *Grid) checkTopOrder() (string, error) {
	return g.checkOrder(g.topOrder.OrderNumber)
}

func (g *Grid) checkBottomOrder() (string, error) {
	return g.checkOrder(g.bottomOrder.OrderNumber)
}

func (g *Grid) checkOrder(orderNumber uint64) (string, error) {
	client := gateio.NewGateIO(g.APIKeyPair.ApiKey, g.APIKeyPair.SecretKey)
	if o, err := client.GetOrder(orderNumber, g.Pair); err != nil {
		return "", err
	} else {
		return o.Status, nil
	}
}

func (g *Grid) top() (price, amount float64) {
	price = g.Base / (1 - g.Percentage)
	price = g.Truncate(price, g.PricePrecision)
	amount = g.Fund / g.Base
	amount = g.Truncate(amount, g.AmountPrecision)
	return
}

func (g *Grid) bottom() (price, amount float64) {
	price = g.Base * (1 - g.Percentage)
	price = g.Truncate(price, g.PricePrecision)
	amount = g.Fund / price
	amount = g.Truncate(amount, g.AmountPrecision)
	return
}

func (g *Grid) Truncate(value float64, percision int) float64 {
	str := fmt.Sprintf("%."+strconv.Itoa(percision)+"f", value)
	newValue, _ := strconv.ParseFloat(str, 64)
	return newValue
}
