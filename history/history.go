package history

import (
	"context"
	"fmt"
	"github.com/xyths/hs"
	. "github.com/xyths/hs/log"
	"github.com/xyths/qtr/gateio"
	"github.com/xyths/qtr/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

type Config struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
	History  hs.HistoryConf
}
type History struct {
	config Config

	db *mongo.Database
	ex *gateio.GateIO

	interval time.Duration
}

func New(configFilename string) *History {
	cfg := Config{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		Sugar.Fatal(err)
	}
	d, err := time.ParseDuration(cfg.History.Interval)
	if err != nil {
		log.Fatalf("parse duration error: %s", err)
	}
	return &History{
		config:   cfg,
		interval: d,
	}
}

func (h *History) Init(ctx context.Context) {
	db, err := hs.ConnectMongo(ctx, h.config.Mongo)
	if err != nil {
		Sugar.Fatal(err)
	}
	h.db = db
	h.ex = gateio.New(h.config.Exchange.Key, h.config.Exchange.Secret, h.config.Exchange.Host)
}

func (h *History) Close(ctx context.Context) {
	if h.db != nil {
		_ = h.db.Client().Disconnect(ctx)
	}
}

func (h *History) Pull(ctx context.Context) error {
	if err := h.getHistoryOnce(ctx); err != nil {
		Sugar.Errorf("error when getHistory: %s", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		case <-time.After(h.interval):
			if err := h.getHistoryOnce(ctx); err != nil {
				Sugar.Errorf("error when getHistory: %s", err)
			}
		}
	}
}

const collNameHistory = "history"

func (h *History) getHistoryOnce(ctx context.Context) error {
	history, err := h.ex.MyTradeHistory(h.config.Exchange.Symbols)
	if err != nil {
		return err
	}

	all := len(history.Trades)
	success := 0
	duplicate := 0
	fail := 0

	coll := h.db.Collection(collNameHistory)
	for _, t := range history.Trades {
		Sugar.Infow("got trade", "trade", t)
		trade := types.Trade{
			TradeId: t.TradeId,
			OrderId: t.OrderNumber,
			Symbol:  t.Pair,
			Type:    t.Type,
			Price:   t.Rate,
			Date:    t.Date,
			Role:    t.Role,
			Fee:     gateio.Fee(t.Fee, t.FeeCoin, t.GtFee, t.PointFee),
		}
		switch trade.Type {
		case "buy":
			trade.Amount = t.Amount
			trade.Total = fmt.Sprintf("-%f", t.Total)
		case "sell":
			trade.Amount = "-" + t.Amount
			trade.Total = fmt.Sprintf("%f", t.Total)
		}

		if c, err := coll.CountDocuments(ctx, bson.D{{"_id", trade.TradeId}}); err != nil {
		} else if c == 0 {
			if _, err1 := coll.InsertOne(ctx, &trade); err1 != nil {
				Sugar.Errorf("insert error", "tradeId", trade.TradeId)
				fail++
			} else {
				success++
			}
		} else {
			duplicate++
		}
	}
	log.Printf("get history for %s-%s finish now, all: %d, success: %d, duplicate: %d, fail: %d",
		h.config.Exchange.Name, h.config.Exchange.Label, all, success, duplicate, fail)
	return nil
}
