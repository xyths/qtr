package trigger

import (
	"github.com/xyths/hs/exchange"
	"testing"
	"time"
)

func TestPeriod(t *testing.T) {
	tests := []time.Duration{
		exchange.MIN1,
		exchange.MIN5,
		exchange.MIN15,
		exchange.MIN30,
		exchange.HOUR1,
		exchange.HOUR4,
		exchange.DAY1,
		exchange.WEEK1,
		exchange.MON1,
	}
	now := time.Now()
	t.Logf("now is %s", now.String())
	for _, tt := range tests {
		t.Logf("next time is %s", now.Truncate(tt).Add(tt))
	}
	/*
	    TestPeriod: trigger_test.go:22: now is 2020-10-29 14:07:22.223828 +0800 CST m=+0.005287311
	    TestPeriod: trigger_test.go:24: next time is 2020-10-29 14:08:00 +0800 CST
	    TestPeriod: trigger_test.go:24: next time is 2020-10-29 14:10:00 +0800 CST
	    TestPeriod: trigger_test.go:24: next time is 2020-10-29 14:15:00 +0800 CST
	    TestPeriod: trigger_test.go:24: next time is 2020-10-29 14:30:00 +0800 CST
	    TestPeriod: trigger_test.go:24: next time is 2020-10-29 15:00:00 +0800 CST
	    TestPeriod: trigger_test.go:24: next time is 2020-10-29 16:00:00 +0800 CST
	    TestPeriod: trigger_test.go:24: next time is 2020-10-30 08:00:00 +0800 CST
	    TestPeriod: trigger_test.go:24: next time is 2020-11-02 08:00:00 +0800 CST
	    TestPeriod: trigger_test.go:24: next time is 2020-11-02 08:00:00 +0800 CST
	 */
}
