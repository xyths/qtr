package reaper

import (
	"github.com/xyths/hs/exchange"
	"log"
	"sync"
)

func (r *Reaper) subscribeTrade() {
	r.ex.SubscribeTrade(r.symbol, "reaper", r.handleTradeUpdate)
	r.Sugar.Infof("subscribe to %s's trade history", r.symbol)
}

func (r *Reaper) unsubscribeTrade() {
	r.ex.UnsubscribeTrade(r.symbol, "reaper")
	r.Sugar.Infof("unsubscribe to %s's trade history", r.symbol)
}

func (r *Reaper) handleTradeUpdate(trades []exchange.TradeDetail) {
	for _, t := range trades {
		r.Sugar.Debugf("frame:%d,%s,%s", t.Timestamp, t.Price.String(), t.Amount)
	}
	// add trades
	r.beacon.Add(trades)
	// test signal
	if signal := r.beacon.Signal(); signal != nil {
		r.Sugar.Debugf("got signal: %s, %f, %f%%", signal.Direction, signal.Price, signal.Score*100)
	}
}

const (
	defaultMinLength = 6
	window           = 5
)

type Beacon struct {
	MaxLength int
	MinLength int

	lock       sync.RWMutex
	Timestamps []int64
	Prices     []float64
	Amount     []float64
}

type Signal struct {
	Direction exchange.Direction
	Price     float64
	Score     float64
}

func (b *Beacon) Signal() *Signal {
	b.lock.RLock()
	defer b.lock.RUnlock()

	l := len(b.Timestamps)
	if l < b.MinLength || l < defaultMinLength {
		return nil
	}

	var avg1, avg2 float64
	for i := l - defaultMinLength; i < l-2; i++ {
		avg2 += b.Prices[i]
	}
	avg1 = avg2 + b.Prices[l-2]
	avg1 /= defaultMinLength - 1
	avg2 /= defaultMinLength - 2
	latest := b.Prices[l-1]
	second := b.Prices[l-2]
	if latest > avg1 || (latest > second && latest > avg2) {
		return &Signal{
			Direction: exchange.TradeDirectionBuy,
			Price:     latest,
			Score:     (latest - avg1) / avg1,
		}
	} else if latest < avg1 || (latest < second && latest < avg2) {
		return &Signal{
			Direction: exchange.TradeDirectionSell,
			Price:     latest,
			Score:     (avg1 - latest) / avg1,
		}
	} else {
		return nil
	}
}

func (b *Beacon) Add(trades []exchange.TradeDetail) {
	b.lock.Lock()
	defer b.lock.Unlock()

	for _, t := range trades {
		b.Timestamps = append(b.Timestamps, t.Timestamp)
		price, _ := t.Price.Float64()
		b.Prices = append(b.Prices, price)
		amount, _ := t.Amount.Float64()
		b.Amount = append(b.Amount, amount)
	}
	l := len(b.Timestamps)
	log.Printf("ticks length: %d", l)
	if l > b.MaxLength {
		b.Timestamps = b.Timestamps[l-b.MaxLength:]
		b.Prices = b.Prices[l-b.MaxLength:]
		b.Amount = b.Amount[l-b.MaxLength:]
	}
}
