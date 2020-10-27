package executor

import (
	"context"
	"github.com/shopspring/decimal"
	"github.com/xyths/hs"
)

type RestExecutor struct {
	BaseExecutor
	buyOrderId  uint64
	sellOrderId uint64
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

func (e *RestExecutor) CheckAll() error {
	if err := e.CheckAllBuy(); err != nil {
		return err
	}
	if err := e.CheckAllSell(); err != nil {
		return err
	}
	return nil
}

func (e *RestExecutor) CheckAllBuy() error {
	orderId := e.GetBuyOrderId()
	if orderId != 0 {
		o, fullFilled, err := e.ex.IsFullFilled(e.Symbol(), orderId)
		if err != nil {
			e.Sugar.Errorf("check buy order %d error: %s", orderId, err)
			return err
		}
		if fullFilled {
			e.Sugar.Debugf("buy order %d is full-filled", orderId)
			e.SetBuyOrderId(0)
			e.Broadcast("买入成交，订单号: %d / %s, 价格: %s, 数量: %s, 买入总金额: %s",
				orderId, o.ClientOrderId, o.Price, o.Amount, o.Amount.Mul(o.Price))
			return nil
		}
	}
	return nil

}

func (e *RestExecutor) CheckAllSell() error {
	orderId := e.GetSellOrderId()
	if orderId != 0 {
		o, fullFilled, err := e.ex.IsFullFilled(e.Symbol(), orderId)
		if err != nil {
			e.Sugar.Errorf("check sell order %d error: %s", orderId, err)
			return err
		}
		if fullFilled {
			e.Sugar.Debugf("sell order %d is full-filled", orderId)
			e.SetSellOrderId(0)
			e.Broadcast("卖出成交，订单号: %d / %s, 价格: %s, 数量: %s, 买入总金额: %s",
				orderId, o.ClientOrderId, o.Price, o.Amount, o.Amount.Mul(o.Price))
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
