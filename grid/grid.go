package grid

import (
	"context"
	"fmt"
	"github.com/xyths/qtr/exchange"
	"github.com/xyths/qtr/gateio"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"math"
	"strconv"
)

type Grid struct {
	Exchange   string
	Label      string
	Pair       string // 交易对
	APIKeyPair exchange.APIKeyPair `bson:"-"`

	Percentage      float64 `bson:"-"`
	Fund            float64 `bson:"-"`
	MaxGrid         int     `bson:"-"`
	PricePrecision  int     `bson:"-"`
	AmountPrecision int     `bson:"-"`

	DB *mongo.Database `bson:"-"`

	Base          float64 `bson:"base"`
	Position      int     `bson:"position"`
	UpTimes       int     `bson:"upTimes"`
	DownTimes     int     `bson:"downTimes"`
	TopOrderId    uint64  `bson:"topOrderId"`
	BottomOrderId uint64  `bson:"bottomOrderId"`
}

func (g *Grid) Load(ctx context.Context) error {
	coll := g.DB.Collection(CollName)
	err := coll.FindOne(ctx, bson.D{
		{"exchange", g.Exchange},
		{"label", g.Label},
		{"pair", g.Pair},
	}).Decode(&g)
	if err != nil {
		// ErrNoDocuments means that the filter did not match any documents in the collection
		if err == mongo.ErrNoDocuments {
			return nil
		}
		log.Print(err)
		return err
	}
	log.Printf("[INFO] Load grid status: base %f, position %d, upTimes %d, downTimes %d, topOrder %d, bottomOrder %d",
		g.Base, g.Position, g.UpTimes, g.DownTimes, g.TopOrderId, g.BottomOrderId)
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

func (g *Grid) DoWork(ctx context.Context) error {
	last, err := g.last()
	if err != nil {
		log.Printf("get last price error: %s", err)
		return err
	}
	if g.Base == 0 {
		g.Base = last
		_ = g.Save(ctx)
		log.Printf("[INFO] init base to %f", g.Base)
	}
	if g.TopOrderId == 0 {
		g.orderTop(last)
		_ = g.Save(ctx)
	}
	if g.BottomOrderId == 0 {
		g.orderBottom(last)
		_ = g.Save(ctx)
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
		log.Printf("[INFO] 已连续向上突破 %d 次，达到设置的最大次数(%d)。", g.UpTimes, g.MaxGrid)
		return false
	}
	if g.Position >= 2*g.MaxGrid {
		log.Printf("[INFO] 共计向上突破 %d 次，已达到设置的最大次数(%d)。", g.UpTimes, 2*g.MaxGrid)
		return false
	}
	if g.DownTimes > 0 {
		g.DownTimes = 0
	}
	g.UpTimes++
	g.Position++

	newBase := g.Base / (1 - g.Percentage)
	log.Printf("[INFO] base UP from %f to %f, position %d, upTimes %d, downTimes %d",
		g.Base, newBase, g.Position, g.UpTimes, g.DownTimes)
	g.Base = g.Truncate(newBase, g.PricePrecision)

	g.cancelBottom()
	g.orderTop(last)
	g.orderBottom(last)

	return true
}

func (g *Grid) down(last float64) bool {
	if g.DownTimes >= g.MaxGrid {
		log.Printf("[INFO] 已连续向下突破 %d 次，达到设置的最大次数(%d)。", g.DownTimes, g.MaxGrid)
		return false
	}
	if g.Position <= -2*g.MaxGrid {
		log.Printf("[INFO] 共计向下突破 %d 次，已达到设置的最大次数(%d)。", g.DownTimes, 2*g.MaxGrid)
		return false
	}
	if g.UpTimes > 0 {
		g.UpTimes = 0
	}
	g.DownTimes++
	g.Position--

	newBase := g.Base * (1 - g.Percentage)
	log.Printf("[INFO] base DOWN from %f to %f, position %d, upTimes %d, downTimes %d",
		g.Base, newBase, g.Position, g.UpTimes, g.DownTimes)
	g.Base = g.Truncate(newBase, g.PricePrecision)

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
	g.TopOrderId = res.OrderNumber
	log.Printf("[INFO] orderTop: price %f, amount %f, orderNumber %d", price, amount, g.TopOrderId)
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
	g.BottomOrderId = res.OrderNumber
	log.Printf("[INFO] orderBottom: price %f, amount %f, orderNumber %d", price, amount, g.TopOrderId)
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
	g.cancelOrder(g.TopOrderId)
	g.TopOrderId = 0
}

func (g *Grid) cancelBottom() {
	g.cancelOrder(g.BottomOrderId)
	g.BottomOrderId = 0
}

func (g *Grid) cancelBoth() {
	g.cancelTop()
	g.cancelBottom()
}

func (g *Grid) checkTopOrder() (string, error) {
	return g.checkOrder(g.TopOrderId)
}

func (g *Grid) checkBottomOrder() (string, error) {
	return g.checkOrder(g.BottomOrderId)
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

func (g *Grid) Truncate(value float64, precision int) float64 {
	str := fmt.Sprintf("%."+strconv.Itoa(precision)+"f", value)
	newValue, _ := strconv.ParseFloat(str, 64)
	return newValue
}
