package utils

import (
	"encoding/json"
	"github.com/urfave/cli/v2"
	"github.com/xyths/qtr/node"
	"log"
	"os"
)

func parseConfig(filename string) (c node.Config) {
	configFile, err := os.Open(filename)
	defer func() {
		_ = configFile.Close()
	}()
	if err != nil {
		log.Fatal(err)
	}
	err = json.NewDecoder(configFile).Decode(&c)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func GetNode(ctx *cli.Context) node.Node {
	c := parseConfig(ctx.String(ConfigFlag.Name))
	n := node.Node{}
	n.Init(ctx.Context, c)
	return n
}

func GetWsNode(ctx *cli.Context) node.WsNode {
	c := parseConfig(ctx.String(ConfigFlag.Name))
	n := node.WsNode{}
	n.Init(ctx.Context, c)
	return n
}