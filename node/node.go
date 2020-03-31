package node

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

type Node struct {
	config Config
	db     *mongo.Database
}

func (n *Node) Init(ctx context.Context, cfg Config) {
	n.config = cfg
	n.initMongo(ctx, cfg)
}

func (n *Node) initMongo(ctx context.Context, config Config) {
	clientOpts := options.Client().ApplyURI(config.Mongo.URI)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		log.Fatal("Error when connect to mongo:", err)
	}
	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("Error when ping to mongo:", err)
	}
	n.db = client.Database(config.Mongo.Database)
}

func (n *Node) Grid(ctx context.Context) {

}
func (n *Node) History(ctx context.Context) {

}
