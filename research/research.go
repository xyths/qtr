package research

import (
	indicator "github.com/xyths/go-indicators"
	"github.com/xyths/hs"
	"github.com/xyths/qtr/types"
	"go.uber.org/zap"
	"time"
)

type Research struct {
	config Config
	Sugar  *zap.SugaredLogger
}

func NewResearch(cfg Config) *Research {
	return &Research{
		config: cfg,
	}
}

func (r *Research) Init() error {
	l, err := hs.NewZapLogger(r.config.Log)
	if err != nil {
		return err
	}
	r.Sugar = l.Sugar()
	r.Sugar.Info("Logger initialized")

	r.Sugar.Info("Research initialized")
	return nil
}

func (r *Research) SuperTrend(input string, factor float64, period int, start, end time.Time, initial float64, output string) error {
	timestamp, open, high, low, close_ := readData(input, true)
	var results []SuperTrendReturn
	for i := 0.1; i <= 10; i = i + 0.1 {
		for j := 1; j <= 30; j++ {
			final, rate, annual := r.superTrend(i, j, start, end, initial, timestamp, open, high, low, close_)
			results = append(results, SuperTrendReturn{
				Factor: i,
				Period: j,
				Final:  final,
				Rate:   rate,
				Annual: annual,
			})
		}
	}
	return writeResult(results, output)
}

func (r *Research) superTrend(factor float64, period int, start, end time.Time, initial float64,
	timestamp []int64, open, high, low, close_ []float64) (final, rate, annualizedRate float64) {
	tsl, trend := indicator.SuperTrend(factor, period, high, low, close_)

	signal := make([]int, len(trend))
	cash := initial
	coin := 0.0
	for i := 0; i < len(trend); i++ {
		realtime := time.Unix(timestamp[i], 0)
		timeStr := types.TimestampToDate(timestamp[i])
		r.Sugar.Debugf("%s %f %f %f %f, %f %v", timeStr, open[i], high[i], low[i], close_[i], tsl[i], trend[i])
		if !realtime.Before(start) && !realtime.After(end) {
			if trend[i] && !trend[i-1] {
				signal[i] = 1
				amount := cash / close_[i]
				coin += amount
				cash = 0
				r.Sugar.Infow("[Signal] Buy", "time", timeStr, "price", close_[i], "amount", amount, "cash", cash, "coin", coin)
			} else if !trend[i] && trend[i-1] {
				signal[i] = -1
				amount := coin
				cash += coin * close_[i]
				coin = 0
				r.Sugar.Infow("[Signal] Sell", "time", timeStr, "price", close_[i], "amount", amount, "cash", cash, "coin", coin)
			}
		}
	}
	final = cash + coin*close_[len(close_)-1]
	rate = (final - initial) / initial
	annualizedRate = rate * (24 * 365 / end.Sub(start).Hours())
	r.Sugar.Infof("Factor: %f, Period: %d, Initial: %f, Final: %f, Rate: %.4f / %.4f",
		factor, period, initial, final, rate, annualizedRate)
	return
}
