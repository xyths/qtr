package huobi

import (
	"errors"
	"fmt"
	"github.com/huobirdcenter/huobi_golang/pkg/model/market"
	"github.com/xyths/hs"
)

func CandlestickHandler(resp interface{}) (*hs.Ticker, *hs.Candle, error) {
	candlestickResponse, ok := resp.(market.SubscribeCandlestickResponse)
	if ok {
		if &candlestickResponse != nil {
			if candlestickResponse.Tick != nil {
				tick := candlestickResponse.Tick
				//logger.Sugar.Debugf("Tick, id: %d, count: %v, amount: %v, volume: %v, OHLC[%v, %v, %v, %v]",
				//	tick.Id, tick.Count, tick.Amount, tick.Vol, tick.Open, tick.High, tick.Low, tick.Close)
				ticker := hs.Ticker{
					Timestamp: tick.Id,
				}
				ticker.Open, _ = tick.Open.Float64()
				ticker.High, _ = tick.High.Float64()
				ticker.Low, _ = tick.Low.Float64()
				ticker.Close, _ = tick.Close.Float64()
				ticker.Volume, _ = tick.Vol.Float64()
				return &ticker, nil, nil
			}

			if candlestickResponse.Data != nil {
				candle := hs.NewCandle(len(candlestickResponse.Data))
				//logger.Sugar.Debugf("Candlestick(candle) update, last timestamp: %d", candlestickResponse.Data[len(candlestickResponse.Data)-1].Id)
				for _, tick := range candlestickResponse.Data {
					//logger.Sugar.Infof("Candlestick data[%d], id: %d, count: %v, volume: %v, OHLC[%v, %v, %v, %v]",
					//	i, tick.Id, tick.Count, tick.Vol, tick.Open, tick.High, tick.Low, tick.Close)
					ticker := hs.Ticker{
						Timestamp: tick.Id,
					}
					ticker.Open, _ = tick.Open.Float64()
					ticker.High, _ = tick.High.Float64()
					ticker.Low, _ = tick.Low.Float64()
					ticker.Close, _ = tick.Close.Float64()
					ticker.Volume, _ = tick.Vol.Float64()
					candle.Append(ticker)
				}
				return nil, &candle, nil
			}
		}
	} else {
		return nil, nil, errors.New(fmt.Sprintf("Unknown response: %v", resp))
	}
	return nil, nil, nil
}
