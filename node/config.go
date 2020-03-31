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
	APIKeyPair gateio.APIKeyPair `json:"APIKeyPair"`
}

type Config struct {
	User  User
	Mongo MongoConf
}
