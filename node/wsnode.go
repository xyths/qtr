package node

import (
	"context"
	"github.com/huobirdcenter/huobi_golang/pkg/model/account"
	"github.com/jinzhu/gorm"
	"github.com/xyths/hs/convert"
	"github.com/xyths/qtr/exchange"
	"github.com/xyths/qtr/exchange/huobi"
	"github.com/xyths/qtr/grid"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

type WsNode struct {
	config Config
	mg     *mongo.Database
	gormDB *gorm.DB
	grids  []*grid.WsGrid
}

func (n *WsNode) Init(ctx context.Context, cfg Config) {
	n.config = cfg
	n.initMongo(ctx)
	n.initMySQL(ctx)
}

func (n *WsNode) initMongo(ctx context.Context) {
	clientOpts := options.Client().ApplyURI(n.config.Mongo.URI)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		log.Fatal("Error when connect to mongo:", err)
	}
	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("Error when ping to mongo:", err)
	}
	n.mg = client.Database(n.config.Mongo.Database)
}

func (n *WsNode) initMySQL(ctx context.Context) {
	db, err := gorm.Open("mysql", n.config.MySQL.URI)
	if err != nil {
		log.Fatal("Error when connect to MySQL:", err)
	}
	n.gormDB = db
}

func (n *WsNode) Close() {
	log.Println("close ws node")
	if n.gormDB != nil {
		if err := n.gormDB.Close(); err != nil {
			log.Printf("error when gorm close")
		}
	}
}

func (n *WsNode) Grid(ctx context.Context) error {
	if err := n.initWsGrid(ctx); err != nil {
		log.Fatal(err)
	}
	defer n.shutdownWsGrid(ctx)
	for _, g := range n.grids {
		if err := g.Run(ctx); err != nil {
			log.Fatalf("error when WsGrid.Run: %s", err)
		}
	}

	<-ctx.Done()
	log.Println("stop websocket grid server")

	return nil
}

func (n *WsNode) Snapshot(ctx context.Context, label string) error {
	for _, u := range n.config.Users {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		default:
			if label == "" || u.Label == label {
				switch u.Exchange {
				case "gate":
					//if err := n.getUserAsset(ctx, u); err != nil {
					//	log.Printf("error when getUserAsset: %s", err)
					//}
					log.Println("not support gate")
				case "huobi":
					cfg := huobi.Config{
						Label:        u.Label,
						AccessKey:    u.APIKeyPair.ApiKey,
						SecretKey:    u.APIKeyPair.SecretKey,
						CurrencyList: []string{"btc", "usdt", "ht"},
					}
					hb := huobi.NewClient(cfg)
					var balance huobi.HuobiBalance
					_ = n.getUserAssetByEx(ctx, hb, &balance)
				}
			}
		}
	}

	return nil
}

func (n *WsNode) getUserAssetByEx(ctx context.Context, ex exchange.Exchange, result interface{}) error {
	if !n.gormDB.HasTable(result) {
		n.gormDB.CreateTable(result)
	}
	if err := ex.Snapshot(ctx, result); err != nil {
		log.Printf("error when snapshot: %s", err)
		return err
	}
	n.gormDB.Create(result)
	return nil
}

func (n *WsNode) shutdownWsGrid(ctx context.Context) {
	log.Println("shutdown ws grid object")
	for _, g := range n.grids {
		_ = g.Shutdown(ctx)
	}
	return
}

func (n *WsNode) initWsGrid(ctx context.Context) error {
	log.Println("init ws grid object")
	for _, u := range n.config.Users {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		default:
			g := grid.WsGrid{
				ExchangeName: u.Exchange,
				Label:        u.Label,
				Pair:         u.Pair,
				APIKeyPair:   u.APIKeyPair,
				Percentage:   n.config.Grid.Percentage,
				Fund:         n.config.Grid.Fund,
				MaxGrid:      n.config.Grid.MaxGrid,
				MongClient:   n.mg,
				GormClient:   n.gormDB,
			}
			_ = g.Init(ctx)
			n.grids = append(n.grids, &g)
		}
	}

	return nil
}

func (n *WsNode) SubscribeBalance(ctx context.Context, label string) error {
	updateChan := make(chan exchange.Balance, 3)
	for _, u := range n.config.Users {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		default:
			if label == "" || u.Label == label {
				switch u.Exchange {
				case "huobi":
					cfg := huobi.Config{
						Label:        u.Label,
						AccessKey:    u.APIKeyPair.ApiKey,
						SecretKey:    u.APIKeyPair.SecretKey,
						CurrencyList: []string{"btc", "usdt", "ht"},
					}
					hb := huobi.NewClient(cfg)
					go n.SubBalanceForEx(ctx, hb, updateChan)
				default:
					log.Println("not support exchange")
				}
			}
		}
	}
	for {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		case balance := <-updateChan:
			log.Printf("receive balance update: %#v", balance)
			n.saveBalance(ctx, balance)
		}
	}
	//return nil
}

func (n *WsNode) SubBalanceForEx(ctx context.Context, ex exchange.Exchange, updateCh chan exchange.Balance) {
	err := ex.SubscribeBalanceUpdate("1111",
		func(resp interface{}) {
			subResponse, ok := resp.(account.SubscribeAccountV2Response)
			if ok {
				if &subResponse.Data != nil {
					b := subResponse.Data
					log.Printf("balance change: %#v", b)
					balance := exchange.Balance{
						Time:     time.Unix(b.ChangeTime/1000, 0),
						Exchange: ex.ExchangeName(),
						Label:    ex.Label(),
						Currency: b.Currency,
						Amount:   convert.StrToFloat64(b.Balance),
					}
					updateCh <- balance
				}
			} else {
				log.Printf("Received unknown response: %v\n", resp)
			}
		})

	if err != nil {
		log.Fatal(err)
	}
}

func (n *WsNode) saveBalance(ctx context.Context, balance exchange.Balance) {
	log.Printf("save balance: %#v", balance)
	coll := n.mg.Collection(exchange.CollBalance)
	if _, err := coll.InsertOne(ctx, &balance); err != nil {
		log.Println(err)
	}
}
