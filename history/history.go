package history

import (
	"context"
	"encoding/csv"
	"fmt"
	"github.com/xyths/hs"
	. "github.com/xyths/hs/logger"
	"github.com/xyths/qtr/cmd/utils"
	"github.com/xyths/qtr/gateio"
	"github.com/xyths/qtr/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"os"
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
	history, err := h.ex.MyTradeHistory(h.config.Exchange.Symbols[0])
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

func (h *History) Export(ctx context.Context, start, end, csvfile string) error {
	startTime, endTime, err := utils.ParseStartEndTime(start, end)
	if err != nil {
		Sugar.Error(err)
		return err
	}
	f, err := os.Create(csvfile)
	if err != nil {
		Sugar.Error(err)
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	w := csv.NewWriter(f)
	header := []string{"account", "time", "type", "price", "amount", "total", "GT", "tradeId", "orderNumber"}
	if err = w.Write(header); err != nil {
		Sugar.Errorf("error when write csv header: %s", err)
	}
	w.Flush()

	trades, err := h.getUserTrades(ctx, startTime, endTime)
	if err != nil {
		Sugar.Errorf("error when getUserAsset: %s", err)
		return err
	}
	//write to csv
	for _, t := range trades {
		record := []string{
			h.config.Exchange.Label,
			t.Date,
			t.Type,
			t.Price,
			t.Amount,
			t.Total,
			t.Fee["gt"],
			fmt.Sprintf("%d", t.TradeId),
			fmt.Sprintf("%d", t.OrderId),
		}
		if err1 := w.Write(record); err1 != nil {
			log.Printf("error when write record: %s", err1)
		}
	}
	w.Flush()

	return nil
}

func (n *History) getUserTrades(ctx context.Context, start, end time.Time) (trades []types.Trade, err error) {
	coll := n.db.Collection(collNameHistory)
	cursor, err := coll.Find(ctx, bson.D{
		{"date", bson.D{
			{"$gte", start.Format(utils.TimeLayout)},
			{"$lte", end.Format(utils.TimeLayout)},
		}},
	})
	if err != nil {
		return
	}
	err = cursor.All(ctx, &trades)

	return
}
