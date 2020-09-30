package ws

import (
	"fmt"
)

type BaseTrader struct {
}

func GetClientOrderId(sep, prefix string, short, long, unique int64) string {
	return fmt.Sprintf("%[2]s%[1]s%[3]d%[1]s%[4]d%[1]s%[5]d", sep, prefix, short, long, unique)
}
