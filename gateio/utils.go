package gateio

import "github.com/shopspring/decimal"

func Fee(fee, feeCoin, gtFee, pointFee string) (feeMap map[string]string) {
	feeMap = make(map[string]string)
	if df := decimal.RequireFromString(fee); !df.IsZero() {
		feeMap[feeCoin] = df.Neg().String()
	}
	if gf := decimal.RequireFromString(gtFee); !gf.IsZero() {
		feeMap["gt"] = gf.Neg().String()
	}
	if pf := decimal.RequireFromString(pointFee); !pf.IsZero() {
		feeMap["point"] = pf.Neg().String()
	}
	return
}

func Buy1(bids []Quote) (price, amount float64) {
	if len(bids) == 0 {
		return
	}
	price = bids[0][0]
	amount = bids[0][1]
	return
}

func Sell1(asks []Quote) (price, amount float64) {
	l := len(asks)
	if l == 0 {
		return
	}

	price = asks[l-1][0]
	amount = asks[l-1][1]
	return
}
