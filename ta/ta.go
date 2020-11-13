package ta

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	indicator "github.com/xyths/go-indicators"
	"github.com/xyths/hs"
	"github.com/xyths/hs/exchange"
	"github.com/xyths/hs/exchange/gateio"
	"github.com/xyths/hs/exchange/huobi"
	"github.com/xyths/qtr/ta/grid"
	"github.com/xyths/qtr/ta/natr"
	"github.com/xyths/qtr/ta/squeeze"
	"github.com/xyths/qtr/ta/supertrend"
	"go.uber.org/zap"
	"os"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Exchange hs.ExchangeConf
	Log      hs.LogConf
}

type Agent struct {
	config Config

	Sugar *zap.SugaredLogger
	ex    exchange.RestAPIExchange
}

func NewAgent(cfg Config) *Agent {
	return &Agent{
		config: cfg,
	}
}

func (a *Agent) Init() error {
	l, err := hs.NewZapLogger(a.config.Log)
	if err != nil {
		return err
	}
	a.Sugar = l.Sugar()
	a.Sugar.Info("Logger initialized")

	switch a.config.Exchange.Name {
	case "huobi":
		a.ex, err = huobi.New(a.config.Exchange.Label, a.config.Exchange.Key, a.config.Exchange.Secret, a.config.Exchange.Host)
		if err != nil {
			return err
		}
	case "gate":
		a.ex = gateio.New(a.config.Exchange.Key, a.config.Exchange.Secret, a.config.Exchange.Host)
	default:
		return errors.New("exchange not supported")
	}
	a.Sugar.Infof("exchange %s initialized", a.config.Exchange.Name)
	return nil
}

func (a *Agent) NATR(ctx context.Context, symbols []string, start, end time.Time, period time.Duration, output string) error {
	symbols, err := a.fillSymbols(ctx, symbols)
	if err != nil {
		return err
	}
	r, err := natr.NATR(a.ex, symbols, start, end, period)
	if err != nil {
		a.Sugar.Errorf("get natr error: %s", err)
		return err
	}
	// write to csv
	return natr.WriteToCsv(r, output)
}

// just download to csv, no indicators call
func (a *Agent) DownloadFrom(ctx context.Context, symbol string, from, to time.Time, period time.Duration, output string) error {
	a.Sugar.Infof("calculate SuperTrend in details for %s", symbol)
	candle, err := a.ex.CandleFrom(symbol, "1111", period, from, to)
	if err != nil {
		a.Sugar.Errorf("get candle error: %s", err)
		return err
	}
	// write to csv
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	// this is mplfinance default header
	header := []string{"Date", "Open", "High", "Low", "Close", "Volume"}
	_ = w.Write(header)
	for i := 0; i < candle.Length(); i++ {
		line := []string{fmt.Sprintf("%d", candle.Timestamp[i])}
		line = append(line, fmt.Sprintf("%f", candle.Open[i]))
		line = append(line, fmt.Sprintf("%f", candle.High[i]))
		line = append(line, fmt.Sprintf("%f", candle.Low[i]))
		line = append(line, fmt.Sprintf("%f", candle.Close[i]))
		line = append(line, fmt.Sprintf("%f", candle.Volume[i]))
		_ = w.Write(line)
	}
	return nil
}

func (a *Agent) SuperTrendSingle(ctx context.Context, symbol string, size int64, period time.Duration, output string) error {
	a.Sugar.Infof("calculate SuperTrend in details for %s", symbol)
	candle, err := a.ex.CandleBySize(symbol, period, int(size))
	if err != nil {
		a.Sugar.Errorf("get candle error: %s", err)
		return err
	}
	upper, lower, tsl, trend := indicator.SuperTrendDetail(3, 7, candle.High, candle.Low, candle.Close)
	// write to csv
	f, err := os.Create(output)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{"Date", "Open", "High", "Low", "Close", "Volume", "upper", "lower", "tsl", "trend"}
	_ = w.Write(header)
	for i := 0; i < candle.Length(); i++ {
		line := []string{fmt.Sprintf("%d", candle.Timestamp[i])}
		line = append(line, fmt.Sprintf("%f", candle.Open[i]))
		line = append(line, fmt.Sprintf("%f", candle.High[i]))
		line = append(line, fmt.Sprintf("%f", candle.Low[i]))
		line = append(line, fmt.Sprintf("%f", candle.Close[i]))
		line = append(line, fmt.Sprintf("%f", candle.Volume[i]))
		line = append(line, fmt.Sprintf("%f", upper[i]))
		line = append(line, fmt.Sprintf("%f", lower[i]))
		line = append(line, fmt.Sprintf("%f", tsl[i]))
		line = append(line, fmt.Sprintf("%v", trend[i]))
		_ = w.Write(line)
	}
	return nil
}

func (a *Agent) SuperTrend(ctx context.Context, symbols []string, size int64, monthly, weekly, daily, hour4, hour1 bool, output string) error {
	symbols, err := a.fillSymbols(ctx, symbols)
	if err != nil {
		return err
	}
	symbols, _ = a.sortByVol24h(symbols)
	s := supertrend.SuperTrend{
		Sugar: a.Sugar, Ex: a.ex,
	}
	r, err := s.Scan(ctx, symbols, size, monthly, weekly, daily, hour4, hour1)
	if err != nil {
		a.Sugar.Errorf("scan by SuperTrend error: %s", err)
		return err
	}
	// write to csv
	return s.WriteToCsv(ctx, r, output)
}

func (a *Agent) Squeeze(ctx context.Context, symbols []string, size int64, monthly, weekly, daily, hour4, hour1 bool, output string) error {
	symbols, err := a.fillSymbols(ctx, symbols)
	if err != nil {
		return err
	}
	symbols, _ = a.sortByVol24h(symbols)
	s := squeeze.Squeeze{
		Sugar: a.Sugar, Ex: a.ex,
	}
	r, err := s.Scan(ctx, symbols, size, monthly, weekly, daily, hour4, hour1)
	if err != nil {
		a.Sugar.Errorf("scan by SuperTrend error: %s", err)
		return err
	}
	// write to csv
	return s.WriteToCsv(ctx, r, output)
}

func (a *Agent) Grid(ctx context.Context, symbols []string, size int64, output string) error {
	symbols, err := a.fillSymbols(ctx, symbols)
	if err != nil {
		return err
	}
	symbols, _ = a.sortByVol24h(symbols)
	g := grid.Scanner{
		Sugar: a.Sugar, Ex: a.ex,
	}
	r, err := g.Scan(ctx, symbols, size)
	if err != nil {
		a.Sugar.Errorf("scan by Grid error: %s", err)
		return err
	}

	// write to csv
	return g.WriteToCsv(ctx, r, output)
}

func (a *Agent) fillSymbols(ctx context.Context, symbols []string) ([]string, error) {
	// if no symbols, use all symbols available in the exchange
	if len(symbols) == 0 {
		a.Sugar.Info("no symbols in command line")
		if len(a.config.Exchange.Symbols) > 0 {
			a.Sugar.Info("use symbols in config file")
			symbols = a.config.Exchange.Symbols
			a.Sugar.Infof("get symbols from config: %v", symbols)
		} else {
			a.Sugar.Info("no symbols in config file")
			a.Sugar.Info("use all symbols from exchange online")
			var err error
			symbols, err = a.allSymbols(ctx)
			if err != nil {
				return symbols, err
			}
			a.Sugar.Infof("get all symbols from exchange online: %v", symbols)
		}
	}
	return symbols, nil
}

func (a *Agent) allSymbols(ctx context.Context) ([]string, error) {
	symbols, err := a.ex.AllSymbols(ctx)
	if err != nil {
		return nil, err
	}
	var ret []string
	for _, s := range symbols {
		select {
		case <-ctx.Done():
			return ret, ctx.Err()
		default:
			// filter by quote currency "usdt"
			if !s.Disabled && strings.HasSuffix(s.Symbol, "usdt") {
				ret = append(ret, s.Symbol)
			}
		}
	}
	return ret, nil
}

type volume struct {
	Symbol string
	Vol    float64
}
type symbolVolume []volume

func (sv symbolVolume) Len() int {
	return len(sv)
}
func (sv symbolVolume) Swap(i, j int) {
	sv[i], sv[j] = sv[j], sv[i]
}
func (sv symbolVolume) Less(i, j int) bool {
	return sv[i].Vol < sv[j].Vol
}

// sort symbols by 24h trade volume
func (a *Agent) sortByVol24h(symbols []string) ([]string, error) {
	var vols symbolVolume
	for _, s := range symbols {
		vol1, err := a.ex.Last24hVolume(s)
		if err != nil {
			a.Sugar.Errorf("get %s last 24h volume error: %s", s, err)
			continue
		}
		if !vol1.IsPositive() {
			a.Sugar.Debugf("%s has no vol for last 24 hours", s)
			continue
		}
		vol2, _ := vol1.Float64()
		vols = append(vols, volume{
			Symbol: s,
			Vol:    vol2,
		})
	}
	sort.Sort(sort.Reverse(vols))
	ret := make([]string, len(vols))
	for i := 0; i < len(vols); i++ {
		a.Sugar.Debugf("[%d] %s: %f", i, vols[i].Symbol, vols[i].Vol)
		ret[i] = vols[i].Symbol
	}
	return ret, nil
}
