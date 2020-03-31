package node

import (
	"context"
	"github.com/xyths/qtr/gateio"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"math/big"
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
	log.Printf("get history for %s-%s start now", n.config.User.Exchange, n.config.User.Label)

	client := gateio.NewGateIO(n.config.User.APIKeyPair.ApiKey, n.config.User.APIKeyPair.SecretKey)
	history, err := client.MyTradeHistory("SERO_USDT", "")
	if err != nil {
		return err
	}

	all := len(history.Trades)
	success := 0
	duplicate := 0
	fail := 0
	label := n.config.User.Label
	for _, t := range history.Trades {
		trade := Trade{
			OrderNumber: t.OrderNumber,
			Label:       label,
			Pair:        t.Pair,
			Type:        t.Type,
			Rate:        t.Rate,
			Amount:      strToFloat64(t.Amount),
			Total:       t.Total,
			Date:        time.Unix(t.TimeUnix, 0),
			TimeUnix:    t.TimeUnix,
			Role:        t.Role,
			Fee:         strToFloat64(t.Fee),
			FeeCoin:     t.FeeCoin,
			GtFee:       strToFloat64(t.GtFee),
			PointFee:    strToFloat64(t.PointFee),
		}

		coll := n.db.Collection(n.config.History.Prefix + n.config.User.Label)

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
		n.config.User.Exchange, n.config.User.Label, all, success, duplicate, fail)

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
