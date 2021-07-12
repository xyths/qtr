package reaper

import (
	"context"
	"github.com/xyths/hs/exchange"
	"log"
	"sync"
)

// startExecutor start a exchange executor service to place orders.
func (r *Reaper) startExecutor(ctx context.Context) {
	r.Sugar.Info("executor service started")
	var direction exchange.Direction
	for {
		select {
		case <-ctx.Done():
			return
		case signal, ok := <-r.ch:
			if !ok {
				r.Sugar.Info("signal channel closed")
				return
			}
			if signal.Direction != direction {
				direction = signal.Direction
				r.Sugar.Debugf("executor got signal: %s, %f, %f%%", direction, signal.Price, signal.Score*100)
			}
		}
	}
}

func (r *Reaper) subscribeTrade() {
	r.ex.SubscribeTrade(r.symbol, "reaper", r.handleTradeUpdate)
	r.Sugar.Infof("subscribe to %s's trade history", r.symbol)
}

func (r *Reaper) unsubscribeTrade() {
	r.ex.UnsubscribeTrade(r.symbol, "reaper")
	r.Sugar.Infof("unsubscribe to %s's trade history", r.symbol)
}

func (r *Reaper) handleTradeUpdate(trades []exchange.TradeDetail) {
	//for _, t := range trades {
	//	r.Sugar.Debugf("frame:%d,%s,%s", t.Timestamp, t.Price.String(), t.Amount)
	//}
	// add trades
	r.beacon.Add(trades)
	// test signal
	if signal := r.beacon.Signal(); signal != nil {
		r.Sugar.Debugf("got signal: %s, %f, %f%%", signal.Direction, signal.Price, signal.Score*100)
		r.ch <- *signal
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
		price, _ := t.Price.Float64()
		amount, _ := t.Amount.Float64()
		l := len(b.Timestamps)
		if len(b.Timestamps) > 0 && b.Timestamps[l-1] == t.Timestamp {
			if b.Prices[l-1] == price {
				b.Amount[l-1] += amount
			} else {
				total := b.Amount[l-1]*b.Prices[l-1] + amount*price
				b.Amount[l-1] += amount
				b.Prices[l-1] = total / b.Amount[l-1] // average price
			}
		} else {
			if l > 0 {
				log.Printf("frame:%d,%f,%f", b.Timestamps[l-1], b.Prices[l-1], b.Amount[l-1])
			}

			b.Timestamps = append(b.Timestamps, t.Timestamp)
			b.Prices = append(b.Prices, price)
			b.Amount = append(b.Amount, amount)
		}

	}
	l := len(b.Timestamps)
	//log.Printf("ticks length: %d", l)
	if l > b.MaxLength {
		b.Timestamps = b.Timestamps[l-b.MaxLength:]
		b.Prices = b.Prices[l-b.MaxLength:]
		b.Amount = b.Amount[l-b.MaxLength:]
	}
}
