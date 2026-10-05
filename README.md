# gojsonrpc

A JSON-RPC 2.0 client over websockets for Go.

```sh
go get github.com/lukirs95/gojsonrpc/v2
```

## Migrating from v1 to v2

v2 renames the client API to common Go names, hides internal types and fixes
several bugs that changed how the client behaves. Most code only needs a new
import path, three renames and a buffered notification channel.

### 1. Change the import path

```go
// v1
import jsonrpc "github.com/lukirs95/gojsonrpc"

// v2
import jsonrpc "github.com/lukirs95/gojsonrpc/v2"
```

### 2. Rename

| v1                                      | v2                                   |
|-----------------------------------------|--------------------------------------|
| `jsonrpc.JsonRPC`                       | `jsonrpc.Client`                     |
| `jsonrpc.NewJsonRPC()`                  | `jsonrpc.NewClient()`                |
| `SubscribeMethod(ctx, method, ch)`      | `Subscribe(ctx, method, ch)`         |
| `UnsubscribeMethod(method) (*Subscriber, error)` | `Unsubscribe(method) error` |
| `Error.Message` of type `ErrorMessage`  | `Error.Message` of type `string`     |

`Method`, `Subscription`, `Notification`, `Error`, `ErrorCode`, `Connect`,
`SendRequest` and `SetReadLimit` keep their names.

`Unsubscribe` no longer returns the subscriber. You already hold the channel
you passed to `Subscribe`; it is not closed by `Unsubscribe`.

### 3. Removed from the public API

These were internal and are no longer exported: `VERSION`, `M_TYPE_*`,
`R_TYPE_*`, `MessageType`, `ResponseType`, `RequestId`, `ResponseChan`,
`Version`, `ErrorMessage`, `Subscriber`, `RpcRequest`, `RpcNotification`,
`RpcRawResponse`, `RpcResponse`, `RpcServerResponse`, `UnknownMessage`,
`ErrDuplicateId` and `ErrIdNotFound`. `ErrOnDial` was never returned and is
removed.

### 4. Use a buffered channel for notifications

Notifications are now delivered without blocking. A notification is
**dropped** if your channel is not ready to receive it, or if the context
passed to `Subscribe` is done. In v1 a slow subscriber blocked the whole
connection, so responses stopped arriving and requests timed out.

With an unbuffered channel you lose every notification that arrives while your
receive loop is busy. Give the channel a buffer that fits your bursts:

```go
// v1
ch := make(jsonrpc.Subscription)
rpc.SubscribeMethod(ctx, "systems.update", ch)

// v2
ch := make(jsonrpc.Subscription, 64)
rpc.Subscribe(ctx, "systems.update", ch)
```

If you started a goroutine or an extra queue to work around the blocking in
v1, you can remove it.

### 5. Check errors with `errors.Is` and `errors.As`

Error messages changed, so do not compare error strings. Use these instead:

| Situation | v1 | v2 |
|---|---|---|
| Server answered with an error | `fmt` error with the message only, code and data lost | `*jsonrpc.Error` with `Code`, `Message` and `Data` |
| No connection, or connection closed while waiting | `"request was not done, websocket closed"` or a timeout | `jsonrpc.ErrConnectionClosed`, returned immediately |
| No response within the request timeout | `"timeout exeeded"` | error wrapping `context.DeadlineExceeded` |
| `Connect` while already connected | `"already connected"` | `jsonrpc.ErrAlreadyConnected` |
| `Unsubscribe` without subscriber | `fmt` error | `jsonrpc.ErrNotSubscribed` |

```go
result, err := rpc.SendRequest(ctx, "config", params)
var rpcErr *jsonrpc.Error
switch {
case errors.As(err, &rpcErr):
	log.Printf("server error %d: %s (%s)", rpcErr.Code, rpcErr.Message, rpcErr.Data)
case errors.Is(err, jsonrpc.ErrConnectionClosed):
	// reconnect
case errors.Is(err, context.DeadlineExceeded):
	// server too slow
case err != nil:
	return err
}
```

### 6. Behavior you can rely on now

- **Late responses no longer close the connection.** In v1 a response that
  arrived after its request timed out made `Connect` return an error and close
  the websocket. v2 ignores responses with unknown ids. If you reconnected to
  work around this, you no longer need to.
- **Pending requests fail on disconnect.** When `Connect` returns, every
  waiting `SendRequest` returns `ErrConnectionClosed` right away instead of
  running into the timeout.
- **Timeouts are configurable.** `SetRequestTimeout` (default 2s) and
  `SetDialTimeout` (default 10s) replace the fixed values.
- **The client is safe for concurrent use.** `SendRequest`, `Subscribe`,
  `Unsubscribe` and the setters may be called from any goroutine while
  `Connect` runs. v1 had data races here, e.g. two parallel requests could get
  the same id.

### Full example

```go
// v1
rpc := jsonrpc.NewJsonRPC()
rpc.SetReadLimit(1 << 20)
ch := make(jsonrpc.Subscription)
rpc.SubscribeMethod(ctx, "systems.update", ch)
go rpc.Connect(ctx, "ws://host/jsonrpc", nil)
defer rpc.UnsubscribeMethod("systems.update")

// v2
rpc := jsonrpc.NewClient()
rpc.SetReadLimit(1 << 20)
rpc.SetRequestTimeout(5 * time.Second) // optional
ch := make(jsonrpc.Subscription, 64)
rpc.Subscribe(ctx, "systems.update", ch)
go rpc.Connect(ctx, "ws://host/jsonrpc", nil)
defer rpc.Unsubscribe("systems.update")
```
