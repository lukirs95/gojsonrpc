package gojsonrpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	jsonrpc "github.com/lukirs95/gojsonrpc/v2"
)

func Example() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rpc := jsonrpc.NewClient()
	rpc.SetRequestTimeout(5 * time.Second)

	// Subscribe before connecting, so no notification is missed.
	updates := make(jsonrpc.Subscription, 64)
	rpc.Subscribe(ctx, "status.update", updates)

	go func() {
		if err := rpc.Connect(ctx, "ws://localhost:8080/jsonrpc", nil); err != nil {
			log.Printf("connection lost: %v", err)
		}
	}()

	go func() {
		for n := range updates {
			log.Printf("status update: %s", n.Params)
		}
	}()

	// Connect runs in the background; retry until the handshake has completed.
	var raw json.RawMessage
	var err error
	for range 10 {
		raw, err = rpc.SendRequest(ctx, "sum", []int{1, 2, 3})
		if !errors.Is(err, jsonrpc.ErrConnectionClosed) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		log.Fatal(err)
	}
	var sum int
	if err := json.Unmarshal(raw, &sum); err != nil {
		log.Fatal(err)
	}
	log.Printf("sum: %d", sum)
}

func ExampleClient_SendRequest() {
	rpc := jsonrpc.NewClient()
	// ... Connect in a goroutine ...

	_, err := rpc.SendRequest(context.Background(), "config.set", map[string]any{"volume": 11})

	var rpcErr *jsonrpc.Error
	switch {
	case errors.As(err, &rpcErr):
		log.Printf("server error %d: %s, data: %s", rpcErr.Code, rpcErr.Message, rpcErr.Data)
	case errors.Is(err, jsonrpc.ErrConnectionClosed):
		log.Print("not connected")
	case errors.Is(err, context.DeadlineExceeded):
		log.Print("server did not answer in time")
	case err != nil:
		log.Print(err)
	}
}

func ExampleClient_Connect() {
	rpc := jsonrpc.NewClient()
	ctx := context.Background()

	// Connect blocks until the connection ends; call it again to reconnect.
	// Subscriptions are kept.
	for ctx.Err() == nil {
		if err := rpc.Connect(ctx, "ws://localhost:8080/jsonrpc", nil); err != nil {
			log.Printf("connection lost: %v, retrying", err)
			time.Sleep(time.Second)
		}
	}
}
