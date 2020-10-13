package ws

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/xyths/qtr/executor"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sync"
	"testing"
	"time"
)

func TestClientIdManager_LongAdd(t *testing.T) {
	client, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	require.NoError(t, err)
	collection := client.Database("test").Collection("state")
	clientIdManager := executor.ClientIdManager{}
	clientIdManager.Init("+", collection)

	worker := func(id int, wg *sync.WaitGroup) {
		defer wg.Done()
		fmt.Printf("Worker %d starting\n", id)
		clientIdManager.LongAdd(ctx)
		fmt.Printf("Worker %d done\n", id)
	}
	var wg sync.WaitGroup
	long := 5
	for i := 1; i <= long; i++ {
		wg.Add(1)
		go worker(i, &wg)
	}
	wg.Wait()
	want := "t+0+5+1"
	got, err := clientIdManager.GetClientOrderId(ctx, "t")
	if got != want {
		t.Errorf("want %s, got %s", want, got)
	}
}
