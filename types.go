package gojsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
)

// Method is the name of a JSON-RPC method.
type Method string

// ErrorCode is the code of a JSON-RPC error.
type ErrorCode int

// Subscription is a channel that receives notifications, see Client.Subscribe.
type Subscription chan Notification

// Notification is a notification received from the server.
type Notification struct {
	// Ctx is the context that was passed to Client.Subscribe.
	Ctx    context.Context
	Params json.RawMessage
}

// Error is the JSON-RPC 2.0 error object. SendRequest returns it as *Error
// when the server answers with an error, so callers can read Code and Data
// via errors.As.
type Error struct {
	Code    ErrorCode       `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("response error %d: %s", e.Code, e.Message)
}

type requestID int32

type rpcRequest struct {
	Version string          `json:"jsonrpc"`
	Method  Method          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      requestID       `json:"id"`
}

type rpcNotification struct {
	Version string          `json:"jsonrpc"`
	Method  Method          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	Version string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *Error          `json:"error"`
	ID      requestID       `json:"id"`
}

// response is what a pending SendRequest receives on its channel.
type response struct {
	result json.RawMessage
	err    error
}
