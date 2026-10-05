package gojsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type (
	Version      string
	Method       string
	ErrorCode    int
	ErrorMessage string
	RequestId    int32
	ResponseChan chan RpcResponse
	Subscription chan Notification
	MessageType  int
	ResponseType int

	// Error is the JSON-RPC 2.0 error object. SendRequest returns it as *Error
	// when the server answers with an error, so callers can read Code and Data
	// via errors.As.
	Error struct {
		Code    ErrorCode       `json:"code"`
		Message ErrorMessage    `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	}

	subscriberRegistry struct {
		subscriber map[Method]*Subscriber
		sync.RWMutex
	}

	Subscriber struct {
		Notification Subscription
		ctx          context.Context
	}

	Notification struct {
		Ctx    context.Context
		Params json.RawMessage
	}

	RpcRequest struct {
		Version Version         `json:"jsonrpc"`
		Method  Method          `json:"method"`
		Params  json.RawMessage `json:"params"`
		Id      RequestId       `json:"id"`
	}

	RpcNotification struct {
		Version Version         `json:"jsonrpc"`
		Method  Method          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}

	RpcRawResponse struct {
		Version Version         `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
		Error   Error           `json:"error"`
		Id      RequestId       `json:"id"`
	}

	RpcResponse struct {
		ResponseType ResponseType
		Result       json.RawMessage `json:"result"`
		Error        Error           `json:"error"`
	}

	RpcServerResponse struct {
		Version Version          `json:"jsonrpc"`
		Result  *json.RawMessage `json:"result,omitempty"`
		Error   *Error           `json:"error,omitempty"`
		Id      RequestId        `json:"id"`
	}

	// This struct is for unmarshalling any jsonrpc message received by the client. It needs to be further processed by HandleMessage method in order to call subscribers or resolve requests.
	UnknownMessage struct {
		messageType  MessageType
		Request      RpcRequest
		Notification RpcNotification
		Response     RpcRawResponse
	}
)

func (e *Error) Error() string {
	return fmt.Sprintf("response error %d: %s", e.Code, e.Message)
}
