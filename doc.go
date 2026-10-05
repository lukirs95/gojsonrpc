// Package gojsonrpc is a JSON-RPC 2.0 client over websockets.
//
// Create a client with NewClient and run Connect in its own goroutine. Connect
// blocks for the lifetime of the connection; while it runs, SendRequest sends
// requests and Subscribe receives notifications from the server.
//
//	rpc := gojsonrpc.NewClient()
//	go rpc.Connect(ctx, "ws://localhost:8080/jsonrpc", nil)
//	result, err := rpc.SendRequest(ctx, "sum", []int{1, 2})
//
// Until the websocket handshake has completed, SendRequest fails with
// ErrConnectionClosed. There is no connected signal yet, so retry a first
// request that fails with ErrConnectionClosed.
//
// # Requests
//
// SendRequest marshals the params to JSON, waits for the response and returns
// its raw result. It fails with
//   - *Error if the server answered with a JSON-RPC error (use errors.As to
//     read Code, Message and Data),
//   - ErrConnectionClosed if there is no connection or it closed while waiting,
//   - an error wrapping context.DeadlineExceeded after the request timeout
//     (SetRequestTimeout, default 2s).
//
// A response that arrives after its request timed out is ignored.
//
// # Notifications
//
// Subscribe registers a channel for the notifications of one method. Delivery
// never blocks the connection: a notification is dropped if the channel is not
// ready to receive or the context passed to Subscribe is done. Use a buffered
// channel sized for your bursts.
//
// # Connection
//
// Only one Connect may run at a time per client. It returns nil when its
// context is cancelled and an error when the connection fails. When it
// returns, all pending requests fail with ErrConnectionClosed; call Connect
// again to reconnect. Subscriptions are kept across reconnects.
//
// The client does not support requests sent by the server; receiving one
// closes the connection.
//
// All methods of Client are safe for concurrent use.
package gojsonrpc
