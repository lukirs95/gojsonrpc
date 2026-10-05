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
// SendRequest waits for the connection if Connect has not completed the
// handshake yet, so a request can be sent right after starting Connect. To
// wait explicitly, call WaitConnected with a deadline.
//
// # Requests
//
// SendRequest marshals the params to JSON, waits for the response and returns
// its raw result. Waiting for the connection and for the response together is
// bounded by the request timeout (SetRequestTimeout, default 2s). It fails with
//   - *Error if the server answered with a JSON-RPC error (use errors.As to
//     read Code, Message and Data),
//   - ErrConnectionClosed if no connection was established in time or it
//     closed while waiting,
//   - an error wrapping context.DeadlineExceeded if the server did not answer
//     in time.
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
