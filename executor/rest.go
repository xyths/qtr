package executor

import (
	"context"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type RestExecutor struct {
	BaseExecutor
	buyOrderId  uint64
	sellOrderId uint64
}

//func NewRestExecutor(config hs.ExchangeConf) (*RestExecutor, error) {
//	e := RestExecutor{
//		config: config,
//	}
//	ex, err := huobi.New(e.config.Label, e.config.Key, e.config.Secret, e.config.Host)
//	if err != nil {
//		return nil, err
//	}
//	e.ex = ex
//	e.symbol, err = e.ex.GetSymbol(e.config.Symbols[0])
//	if err != nil {
//		return nil, err
//	}
//	e.fee, err = e.ex.GetFee(e.Symbol())
//	if err != nil {
//		return nil, err
//	}
//
//	return &e, nil
//}

func (e *RestExecutor) Init(
	ex exchange.Exchange, sugar *zap.SugaredLogger, db *mongo.Database,
	symbol exchange.Symbol, fee exchange.Fee, maxTotal decimal.Decimal) {
	e.BaseExecutor.Init(ex, sugar, db, symbol, fee, maxTotal)
}

func (e *RestExecutor) Load(ctx context.Context) error {
	if err := e.BaseExecutor.Load(ctx); err != nil {
		return err
	}
	if err := e.LoadBuyOrderId(ctx); err != nil {
		return err
	}
	if err := e.LoadSellOrderId(ctx); err != nil {
		return err
	}
	return nil
}

func (e *RestExecutor) BuyAllLimit(price float64) error {
	realPrice := decimal.NewFromFloat(price).Round(e.PricePrecision())
	orderId, err := e.buyAllLimit(realPrice)
	if err != nil {
		return err
	}
	e.SetBuyOrderId(orderId)
	return nil
}

func (e *RestExecutor) SellAllLimit(price float64) error {
	realPrice := decimal.NewFromFloat(price).Round(e.PricePrecision())
	orderId, err := e.sellAllLimit(realPrice)
	if err != nil {
		//e.Sugar.Errorf("sell error: %s", err)
		return err
	}
	e.SetSellOrderId(orderId)
	return nil
}

func (e *RestExecutor) BuyAllMarket() error {
	orderId, err := e.buyAllMarket()
	if err != nil {
		//e.Sugar.Errorf("buy error: %s", err)
		return err
	}
	e.SetBuyOrderId(orderId)
	return nil
}

func (e *RestExecutor) SellAllMarket() error {
	orderId, err := e.sellAllMarket()
	if err != nil {
		return err
	}
	e.SetSellOrderId(orderId)
	return nil
}

func (e *RestExecutor) CancelAll() error {
	if err := e.CancelAllBuy(); err != nil {
		return err
	}
	if err := e.CancelAllSell(); err != nil {
		return err
	}
	return nil
}

func (e *RestExecutor) CancelAllBuy() error {
	orderId := e.GetBuyOrderId()
	if orderId != 0 {
		if err := e.ex.CancelOrder(e.Symbol(), orderId); err != nil {
			e.Sugar.Errorf("cancel order %d error: %s", orderId, err)
			return err
		} else {
			e.Sugar.Debugf("cancelled order %d", orderId)
			e.SetBuyOrderId(0)
			return nil
		}
	}
	return nil
}

func (e *RestExecutor) CancelAllSell() error {
	orderId := e.GetSellOrderId()
	if orderId != 0 {
		if err := e.ex.CancelOrder(e.Symbol(), orderId); err != nil {
			e.Sugar.Errorf("cancel order %d error: %s", orderId, err)
			return err
		} else {
			e.Sugar.Debugf("cancelled order %d", orderId)
			e.SetSellOrderId(0)
			return nil
		}
	}
	return nil
}

func (e *RestExecutor) GetBuyOrderId() uint64 {
	return e.buyOrderId
}

func (e *RestExecutor) SetBuyOrderId(newId uint64) {
	e.buyOrderId = newId
	_ = hs.SaveKey(context.Background(), e.db.Collection(collNameState), "buyOrderId", e.buyOrderId)
}

func (e *RestExecutor) LoadBuyOrderId(ctx context.Context) error {
	return hs.LoadKey(ctx, e.db.Collection(collNameState), "buyOrderId", &e.buyOrderId)
}

func (e *RestExecutor) GetSellOrderId() uint64 {
	return e.sellOrderId
}

func (e *RestExecutor) SetSellOrderId(newId uint64) {
	e.sellOrderId = newId
	_ = hs.SaveKey(context.Background(), e.db.Collection(collNameState), "sellOrderId", e.sellOrderId)
}

func (e *RestExecutor) LoadSellOrderId(ctx context.Context) error {
	return hs.LoadKey(ctx, e.db.Collection(collNameState), "sellOrderId", &e.sellOrderId)
}
