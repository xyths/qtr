package snapshot

import (
	"context"
	"github.com/xyths/hs"
	. "github.com/xyths/hs/log"
)

type Config struct {
	Exchange hs.ExchangeConf
	Mongo    hs.MongoConf
}

type Snapshot struct {
	config Config

	//ex exchange
}

func New(configFilename string) *Snapshot {
	cfg := Config{}
	if err := hs.ParseJsonConfig(configFilename, &cfg); err != nil {
		Sugar.Fatal(err)
	}
	s := &Snapshot{
		config: cfg,
	}
	switch cfg.Exchange.Name {
	case "huobi":

	}
	return s
}

func (s *Snapshot) Do(ctx context.Context) {

}
