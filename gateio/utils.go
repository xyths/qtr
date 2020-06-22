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
