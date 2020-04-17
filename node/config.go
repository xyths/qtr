package node

import (
	"github.com/xyths/qtr/exchange"
)

type MongoConf struct {
	URI         string `json:"uri"`
	Database    string `json:"database"`
	MaxPoolSize uint64 `json:"maxPoolSize"`
	MinPoolSize uint64 `json:"minPoolSize"`
	AppName     string `json:"appName"`
}

type MySQLConf struct {
	URI   string `json:"uri"`
	Table string
}

type User struct {
	Exchange   string
	Label      string
	Pair       string // 交易对
	APIKeyPair exchange.APIKeyPair `json:"APIKeyPair"`
}

type History struct {
	Prefix   string
	Interval string `json:"interval"`
}

type GridConf struct {
	Interval   string
	Percentage float64
	Fund       float64
	MaxGrid    int
}
type Config struct {
	Users   []User
	Mongo   MongoConf
	MySQL   MySQLConf
	History History
	Grid    GridConf
}
