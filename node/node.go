package node

import (
	"context"
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/nntaoli-project/goex"
	"github.com/nntaoli-project/goex/builder"
	"github.com/xyths/hs"
	"github.com/xyths/qtr/exchange"
	"github.com/xyths/qtr/exchange/huobi"
	"github.com/xyths/qtr/gateio"
	"github.com/xyths/qtr/trader/rest/grid"
	"github.com/xyths/qtr/types"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

type Node struct {
	config Config
	mg     *mongo.Database
	gormDB *gorm.DB
	grids  []grid.RestGridTrader
}

func New(configFilename string) *Node {
	cfg := Config{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		log.Fatal(err)
	}
	return &Node{
		config: cfg,
	}
}

func (n *Node) Init(ctx context.Context) {
	n.initMongo(ctx)
	n.initMySQL(ctx)
}

func (n *Node) initMongo(ctx context.Context) {
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

func (n *Node) initMySQL(ctx context.Context) {
	db, err := gorm.Open("mysql", n.config.MySQL.URI)
	if err != nil {
		log.Fatal("Error when connect to MySQL:", err)
	}
	n.gormDB = db
}

func (n *Node) Close() {
	if n.gormDB != nil {
		if err := n.gormDB.Close(); err != nil {
			log.Printf("error when gorm close")
		}
	}
}

func (n *Node) Grid(ctx context.Context) error {
	d, err := time.ParseDuration(n.config.Grid.Interval)
	if err != nil {
		log.Fatalf("parse duration error: %s", err)
	}
	n.initGrid(ctx)
	if err := n.doGridOnce(ctx); err != nil {
		log.Printf("error when doGrid: %s", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		case <-time.After(d):
			if err := n.doGridOnce(ctx); err != nil {
				log.Printf("error when doGrid: %s", err)
			}
		}
	}
}

func (n *Node) doGridOnce(ctx context.Context) error {
	for i, _ := range n.grids {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		default:
			if err := n.grids[i].Start(ctx); err != nil {
				log.Printf("error when Start: %s", err)
			}
		}
	}
	return nil
}

func (n *Node) Profit(ctx context.Context, label, start, end string) error {
	startTime, endTime, err := parseTime(start, end)
	if err != nil {
		log.Println(err)
		return err
	}

	for _, u := range n.config.Users {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		default:
			if label == "" || u.Label == label {
				if err := n.getUserProfit(ctx, u, startTime, endTime); err != nil {
					log.Printf("error when getUserProfit: %s", err)
				}
			}
		}
	}

	return nil
}

func (n *Node) getUserProfit(ctx context.Context, u User, start, end time.Time) error {
	//trades, err := n.getUserTrades(ctx, u, start, end)
	//if err != nil {
	//	return err
	//}
	//sero := 0.0
	//usdt := 0.0
	//gtFee := 0.0
	//for _, t := range trades {
	//	//log.Printf("tradeId: %d, orderNumber: %d, date: %s, type: %s, rate: %f, amount: %f, total: %f, gtFee: %f",
	//	//	t.TradeId, t.OrderId, t.Date.String(), t.Type, t.Price, t.Amount, t.Total, t.GtFee)
	//	switch t.Type {
	//	//case "buy":
	//	//	sero += t.Amount
	//	//	usdt -= t.Total
	//	//	gtFee -= t.GtFee
	//	//case "sell":
	//	//	sero -= t.Amount
	//	//	usdt += t.Total
	//	//	gtFee -= t.GtFee
	//	default:
	//		log.Println("unknown trade type: %s", t.Type)
	//	}
	//}
	//log.Printf("%s(%s) %s summary: SERO %f, USDT %f, GT: %f", u.Exchange, u.Label, u.Pair, sero, usdt, gtFee)
	return nil
}

func (n *Node) Snapshot(ctx context.Context, label string) error {
	for _, u := range n.config.Users {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			return nil
		default:
			if label == "" || u.Label == label {
				switch u.Exchange {
				case "gate":
					if err := n.getUserAsset(ctx, u); err != nil {
						log.Printf("error when getUserAsset: %s", err)
					}
				case "huobi":
					cfg := huobi.Config{
						Label:        u.Label,
						AccessKey:    u.APIKeyPair.ApiKey,
						SecretKey:    u.APIKeyPair.SecretKey,
						CurrencyList: []string{"btc", "usdt", "ht"},
						Host:         u.APIKeyPair.Domain,
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

func (n *Node) getUserAsset(ctx context.Context, u User) error {
	// delay to create table
	if !n.gormDB.HasTable(&types.GateBalance{}) {
		n.gormDB.CreateTable(&types.GateBalance{})
	}
	//client := gateio.New(u.APIKeyPair.ApiKey, u.APIKeyPair.SecretKey, "gatecn.io")
	//balances, err := client.AvailableBalance()
	//if err != nil {
	//	return err
	//}
	//seroTicker, err := client.Ticker("SERO_USDT")
	//if err != nil {
	//	return err
	//}
	//gtTicker, err := client.Ticker("GT_USDT")
	//if err != nil {
	//	return err
	//}
	//b := utils.GateBalance{
	//	Label:     u.Label,
	//	SERO:      convert.StrToFloat64(balances.Available["SERO"]) + convert.StrToFloat64(balances.Locked["SERO"]),
	//	USDT:      convert.StrToFloat64(balances.Available["USDT"]) + convert.StrToFloat64(balances.Locked["USDT"]),
	//	GT:        convert.StrToFloat64(balances.Available["GT"]) + convert.StrToFloat64(balances.Locked["GT"]),
	//	Time:      time.Now(),
	//	//SeroPrice: seroTicker.Last,
	//	//GtPrice:   gtTicker.Last,
	//}

	//n.gormDB.Create(&b)
	return nil
}

func (n *Node) getUserAssetByEx(ctx context.Context, ex exchange.Exchange, result interface{}) error {
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

func (n *Node) initGrid(ctx context.Context) {
	// init exchange before grid
	for _, u := range n.config.Users {
		//exApi := n.initExAPI(u)
		_ = u
		g := grid.RestGridTrader{
			//Exchange: u.Exchange,
			//Label:    u.Label,
			//Symbol:     u.Symbol,
			//ExAPI: exApi,
			//
			//Percentage: n.config.Grid.Percentage,
			//Fund:       n.config.Grid.Fund,
			//MaxGrid:    n.config.Grid.MaxGrid,

			//PricePrecision:  u.PricePrecision,
			//AmountPrecision: u.AmountPrecision,
			//MinAmount:       u.MinAmount,

			//db: n.mg,
		}
		//if err := g.Load(ctx); err != nil {
		//	log.Fatalf("error when load grid: %s", err)
		//}
		n.grids = append(n.grids, g)
	}
}

func (n *Node) resetGridBase() {
	// use ticker.Last to set base
	//client := gateio.New(u.APIKeyPair.ApiKey, u.APIKeyPair.SecretKey)
}

func (n *Node) balanceCollName(u User) string {
	return "balance_" + u.Exchange
}

func (n *Node) PullCandle(ctx context.Context) error {
	if len(n.config.Users) == 0 {
		log.Fatalf("no user")
	}
	if !n.gormDB.HasTable(exchange.Candle{}) {
		n.gormDB.CreateTable(exchange.Candle{})
	}
	u := n.config.Users[0]
	client := gateio.New(u.APIKeyPair.ApiKey, u.APIKeyPair.SecretKey, "gatecn.io")
	candles, err := client.Candles(u.Pair, 60, 1)
	if err != nil {
		log.Printf("error when get candle data: %s", err)
		return err
	}

	success := 0
	duplicate := 0
	for _, c := range candles {
		if n.gormDB.First(&c).RecordNotFound() {
			n.gormDB.Create(&c)
			success++
		} else {
			duplicate++
		}
	}
	log.Printf("[INFO] download record success %d, duplicate %d", success, duplicate)
	return nil
}

func (n *Node) initExAPI(u User) goex.API {
	apiBuilder := builder.NewAPIBuilder().HttpTimeout(5 * time.Second)
	api := apiBuilder.APIKey(u.APIKeyPair.ApiKey).APISecretkey(u.APIKeyPair.SecretKey)
	switch u.Exchange {
	case "gate":
		return api.Build(goex.GATEIO)
	case "okex":
		return api.ApiPassphrase(u.APIKeyPair.PassPhrase).Build(goex.OKEX_V3)
	case "huobi":
		return api.Build(goex.HUOBI_PRO)
	default:
		return nil
	}
}

func parseTime(start, end string) (startTime, endTime time.Time, err error) {
	secondsEastOfUTC := int((8 * time.Hour).Seconds())
	beijing := time.FixedZone("Beijing Time", secondsEastOfUTC)
	layout := "2006-01-02 15:04:05"
	startTime, err = time.ParseInLocation(layout, start, beijing)
	if err != nil {
		log.Printf("error start format: %s", start)
		return
	}
	endTime, err = time.ParseInLocation(layout, end, beijing)
	if err != nil {
		log.Printf("error end format: %s", end)
		return
	}
	if !startTime.Before(endTime) {
		err = errors.New(fmt.Sprintf("start time(%s) must before end time(%s)", startTime.String(), endTime.String()))
		log.Println(err)
		return
	}
	return
}
