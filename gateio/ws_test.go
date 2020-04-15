package gateio

import (
	"fmt"
	"golang.org/x/net/websocket"
	"log"
	"testing"
)

func TestGatePing(t *testing.T) {
	origin := "http://localhost/"
	url := "wss://ws.gateio.life/v3/"
	ws, err := websocket.Dial(url, "", origin)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := ws.Write([]byte(`{"id":12312, "method":"server.ping", "params":[]}\n`)); err != nil {
		log.Fatal(err)
	}
	var msg = make([]byte, 512)
	var n int
	if n, err = ws.Read(msg); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Received: %s.\n", msg[:n])
}
