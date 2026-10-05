# gojsonrpc

A JSON-RPC 2.0 client over websockets for Go, built on
[coder/websocket](https://github.com/coder/websocket).

- Requests with timeouts and typed JSON-RPC errors
- Notifications delivered to channels without ever blocking the connection
- Safe for concurrent use

```sh
go get github.com/lukirs95/gojsonrpc/v2
```

## Quick start

```go
import jsonrpc "github.com/lukirs95/gojsonrpc/v2"

rpc := jsonrpc.NewClient()

updates := make(jsonrpc.Subscription, 64)
rpc.Subscribe(ctx, "status.update", updates)

go func() {
	if err := rpc.Connect(ctx, "ws://localhost:8080/jsonrpc", nil); err != nil {
		log.Printf("connection lost: %v", err)
	}
}()

raw, err := rpc.SendRequest(ctx, "sum", []int{1, 2, 3})
```

## Connecting

`Connect` dials the websocket and then reads messages until its context is
cancelled (returns `nil`) or the connection fails (returns the error). Run it in
its own goroutine; everything else works while it runs.

- Only one `Connect` runs at a time per client, a second one returns
  `ErrAlreadyConnected`.
- To reconnect, call `Connect` again. Subscriptions are kept.
- When `Connect` returns, every waiting request fails with `ErrConnectionClosed`.
- You can send requests right after starting `Connect`: `SendRequest` waits
  for the connection (see [Requests](#requests)).
- `WaitConnected(ctx)` blocks until the client is connected or `ctx` is done.
  A failed dial does not end the wait, so give `ctx` a deadline.

```go
go rpc.Connect(ctx, address, nil)

waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
if err := rpc.WaitConnected(waitCtx); err != nil {
	return err // wraps ErrConnectionClosed and context.DeadlineExceeded
}
```

```go
for ctx.Err() == nil {
	if err := rpc.Connect(ctx, address, nil); err != nil {
		log.Printf("connection lost: %v, retrying", err)
		time.Sleep(time.Second)
	}
}
```

## Requests

`SendRequest` marshals the params to JSON, sends the request and waits for the
response. It returns the raw `result`, which you unmarshal yourself.

If the client is not connected yet, `SendRequest` first waits for the
connection. The request timeout (`SetRequestTimeout`, default 2s) covers both
waits together, and the context you pass can end them earlier.

```go
raw, err := rpc.SendRequest(ctx, "sum", []int{1, 2, 3})
if err != nil {
	return err
}
var sum int
err = json.Unmarshal(raw, &sum)
```

A response that arrives after its request timed out is ignored and does not
affect the connection.

### Errors

| Error | When |
|---|---|
| `*jsonrpc.Error` | The server answered with a JSON-RPC error. Holds `Code`, `Message` and `Data`. |
| `jsonrpc.ErrConnectionClosed` | No connection within the request timeout (also wraps `context.DeadlineExceeded`), or the connection closed while waiting. |
| wraps `context.DeadlineExceeded` | No response within the request timeout. |
| wraps `context.Canceled` | The context passed to `SendRequest` was cancelled. |

```go
var rpcErr *jsonrpc.Error
switch {
case errors.As(err, &rpcErr):
	log.Printf("server error %d: %s, data: %s", rpcErr.Code, rpcErr.Message, rpcErr.Data)
case errors.Is(err, jsonrpc.ErrConnectionClosed):
	// not connected in time, or connection lost
case errors.Is(err, context.DeadlineExceeded):
	// server too slow
}
```

## Notifications

`Subscribe` registers a channel for the notifications of one method. Each
`Notification` carries the raw `Params` and the context you passed to
`Subscribe`.

```go
updates := make(jsonrpc.Subscription, 64)
rpc.Subscribe(ctx, "status.update", updates)

for n := range updates {
	log.Printf("update: %s", n.Params)
}
```

Delivery never blocks the connection. A notification is **dropped** if the
channel is not ready to receive it or the subscription context is done. Use a
buffered channel sized for your bursts; with an unbuffered channel every
notification that arrives while your loop is busy is lost.

- One subscriber per method; subscribing again replaces it.
- `Unsubscribe(method)` removes it and returns `ErrNotSubscribed` if there was
  none. The channel is not closed.

## Configuration

| Method | Default | Applies |
|---|---|---|
| `SetRequestTimeout(d)` | 2s | to the next request, including waiting for the connection |
| `SetDialTimeout(d)` | 10s | on the next `Connect` |
| `SetReadLimit(bytes)` | 2048 | on the next `Connect` |

The read limit is small. Messages larger than it close the connection, so raise
it if your server sends bigger responses or notifications.

## Limitations

- Requests sent by the server to the client are not supported. Receiving one
  closes the connection.
- Batch requests are not supported.

## Migrating from v1

v2 fixes blocking notifications, lost error data, late responses that closed
the connection and several data races. To upgrade:

1. Import `github.com/lukirs95/gojsonrpc/v2`.
2. Rename `JsonRPC` → `Client`, `NewJsonRPC` → `NewClient`,
   `SubscribeMethod` → `Subscribe`, `UnsubscribeMethod` → `Unsubscribe`
   (now returns only `error`). `Error.Message` is a `string`.
3. **Buffer your notification channels.** Notifications that cannot be
   delivered immediately are dropped instead of blocking the connection.
   Workarounds for the old blocking (extra goroutines or queues) can go.
4. Check errors with `errors.Is`/`errors.As` (see [Errors](#errors)) instead
   of comparing strings; the messages changed.

Internal types and constants (`VERSION`, `M_TYPE_*`, `R_TYPE_*`, `Rpc*`,
`UnknownMessage`, `RequestId`, …) are no longer exported.
