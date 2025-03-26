package jsonrpc

import (
	"context"
	"encoding/json"
)

type (
	Version      string
	Method       string
	ErrorCode    int
	ErrorMessage string
	Id           int32
	Response     chan RpcResponse
	Subscription chan Notification
	MessageType  int
	ResponseType int

	Error struct {
		Code    ErrorCode       `json:"code"`
		Message ErrorMessage    `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	}

	callStack struct {
		callStack map[Id]*Response
	}

	subscriberRegistry struct {
		subscriber map[Method]*Subscriber
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
		Id      Id              `json:"id"`
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
		Id      Id              `json:"id"`
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
		Id      Id               `json:"id"`
	}

	// This struct is for unmarshalling any jsonrpc message received by the client. It needs to be further processed by HandleMessage method in order to call subscribers or resolve requests.
	UnknownMessage struct {
		messageType  MessageType
		Request      RpcRequest
		Notification RpcNotification
		Response     RpcRawResponse
	}
)
