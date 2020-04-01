package node

import (
	"context"
	"github.com/xyths/qtr/gateio"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"math/big"
	"strings"
	"time"
)

type Node struct {
	config Config
	db     *mongo.Database
}

func (n *Node) Init(ctx context.Context, cfg Config) {
	n.config = cfg
	n.initMongo(ctx, cfg)
}

func (n *Node) initMongo(ctx context.Context, config Config) {
	clientOpts := options.Client().ApplyURI(config.Mongo.URI)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		log.Fatal("Error when connect to mongo:", err)
	}
	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("Error when ping to mongo:", err)
	}
	n.db = client.Database(config.Mongo.Database)
}

func (n *Node) Grid(ctx context.Context) error {

	return nil
}

func (n *Node) History(ctx context.Context) error {
	d, err := time.ParseDuration(n.config.History.Interval)
	if err != nil {
		log.Fatalf("parse duration error: %s", err)
	}

	if err := n.getHistoryOnce(ctx); err != nil {
		log.Printf("error when getHistory: %s", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		case <-time.After(d):
			if err := n.getHistoryOnce(ctx); err != nil {
				log.Printf("error when getHistory: %s", err)
			}
		}
	}
}

func (n *Node) getHistoryOnce(ctx context.Context) error {
	for _, u := range n.config.Users {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		default:
			if err := n.getUserHistory(ctx, u); err != nil {
				log.Printf("error when getHistory: %s", err)
			}
		}
	}

	return nil
}

func (n *Node) getUserHistory(ctx context.Context, u User) error {
	log.Printf("get history for %s-%s start now", u.Exchange, u.Label)

	client := gateio.NewGateIO(u.APIKeyPair.ApiKey, u.APIKeyPair.SecretKey)
	history, err := client.MyTradeHistory(strings.ToUpper(u.Pair), "")
	if err != nil {
		return err
	}

	all := len(history.Trades)
	success := 0
	duplicate := 0
	fail := 0
	label := u.Label

	coll := n.db.Collection(n.historyCollName(u))
	for _, t := range history.Trades {
		trade := Trade{
			TradeId:     t.TradeId,
			OrderNumber: t.OrderNumber,
			Label:       label,
			Pair:        strings.ToUpper(t.Pair),
			Type:        t.Type,
			Rate:        strToFloat64(t.Rate),
			Amount:      strToFloat64(t.Amount),
			Total:       t.Total,
			DateString:  t.Date,
			Date:        time.Unix(t.TimeUnix, 0),
			TimeUnix:    t.TimeUnix,
			Role:        t.Role,
			Fee:         strToFloat64(t.Fee),
			FeeCoin:     t.FeeCoin,
			GtFee:       strToFloat64(t.GtFee),
			PointFee:    strToFloat64(t.PointFee),
		}

		if _, err := coll.InsertOne(ctx, trade); err != nil {
			if !isDuplicateError(err) {
				log.Printf("Error when insert to mongo: %s", err)
				fail++
			} else {
				duplicate++
			}
		} else {
			success++
		}
	}
	log.Printf("get history for %s-%s finish now, all: %d, success: %d, duplicate: %d, fail: %d",
		u.Exchange, u.Label, all, success, duplicate, fail)
	return nil
}

func (n *Node) Profit(ctx context.Context, label, start, end string) error {
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	startTime, err := time.ParseInLocation(layout, start, beijing)
	if err != nil {
		log.Printf("error start format: %s", start)
		return err
	}
	endTime, err := time.ParseInLocation(layout, end, beijing)
	if err != nil {
		log.Printf("error end format: %s", end)
		return err
	}
	if !startTime.Before(endTime) {
		log.Printf("start time(%s) must before end time(%s)", startTime.String(), endTime.String())
	}
	for _, u := range n.config.Users {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		default:
			if label == "" || u.Label == label {
				if err := n.getUserProfit(ctx, u, startTime, endTime); err != nil {
					log.Printf("error when getHistory: %s", err)
				}
			}
		}
	}

	return nil
}

func (n *Node) getUserProfit(ctx context.Context, u User, start, end time.Time) error {
	coll := n.db.Collection(n.historyCollName(u))
	cursor, err := coll.Find(ctx, bson.D{
		{"pair", strings.ToUpper(u.Pair)},
		{"label", u.Label},
		{"date", bson.D{
			{"$gte", start},
			{"$lte", end},
		}},
	})
	if err != nil {
		return err
	}
	var trades []Trade
	if err1 := cursor.All(ctx, &trades); err1 != nil {
		return err1
	}
	sero := 0.0
	usdt := 0.0
	gtFee := 0.0
	for _, t := range trades {
		log.Printf("tradeId: %d, orderNumber: %d, date: %s, type: %s, rate: %f, amount: %f, total: %f, gtFee: %f",
			t.TradeId, t.OrderNumber, t.DateString, t.Type, t.Rate, t.Amount, t.Total, t.GtFee)
		switch t.Type {
		case "buy":
			sero += t.Amount
			usdt -= t.Total
			gtFee -= t.GtFee
		case "sell":
			sero -= t.Amount
			usdt += t.Total
			gtFee -= t.GtFee
		default:
			log.Println("unknown trade type: %s", t.Type)
		}
	}
	log.Printf("%s(%s) %s summary: SERO %f, USDT %f, GT: %f", u.Exchange, u.Label, u.Pair, sero, usdt, gtFee)
	return nil
}

func strToInt64(s string, i64 *int64) {
	if i, ok := big.NewInt(0).SetString(s, 0); ok {
		*i64 = i.Int64()
	}
}
func strToFloat64(s string) float64 {
	if f, ok := big.NewFloat(0).SetString(s); ok {
		f64, _ := f.Float64()
		return f64
	}
	return 0.0
}

func isDuplicateError(err error) bool {
	e, ok := err.(mongo.WriteException)
	if !ok {
		return false
	}
	if e.WriteConcernError == nil && len(e.WriteErrors) == 1 && e.WriteErrors[0].Code == 11000 {
		return true
	}
	return false
}

func (n *Node) historyCollName(u User) string {
	return n.config.History.Prefix + "_" + u.Exchange
}
