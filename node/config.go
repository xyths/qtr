package node

import "github.com/xyths/qtr/gateio"

type MongoConf struct {
	URI         string `json:"uri"`
	Database    string `json:"database"`
	MaxPoolSize uint64 `json:"maxPoolSize"`
	MinPoolSize uint64 `json:"minPoolSize"`
	AppName     string `json:"appName"`
}

type User struct {
	Exchange   string
	Label      string
	Pair       string // 交易对
	APIKeyPair gateio.APIKeyPair `json:"APIKeyPair"`
}

type History struct {
	Prefix   string
	Interval string `json:"interval"`
}

type Config struct {
	Users   []User
	Mongo   MongoConf
	History History
}
