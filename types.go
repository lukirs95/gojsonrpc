package jsonrpc

import (
	"context"
	"encoding/json"
)

type Version string
type Method string
type ErrorCode int
type ErrorMessage string
type Id int32
type Response chan RpcResponse
type Subscription chan Notification
type MessageType int
type ResponseType int

type Error struct {
	Code    ErrorCode       `json:"code"`
	Message ErrorMessage    `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type callStack struct {
	callStack map[Id]*Response
}

type subscriberRegistry struct {
	subscriber map[Method]*Subscriber
}

type Subscriber struct {
	Notification Subscription
	ctx          context.Context
}

type Notification struct {
	Ctx    context.Context
	Params json.RawMessage
}

type RpcRequest struct {
	Version Version         `json:"jsonrpc"`
	Method  Method          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Id      Id              `json:"id"`
}

type RpcNotification struct {
	Version Version         `json:"jsonrpc"`
	Method  Method          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type RpcRawResponse struct {
	Version Version         `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   Error           `json:"error"`
	Id      Id              `json:"id"`
}

type RpcResponse struct {
	ResponseType ResponseType
	Result       json.RawMessage `json:"result"`
	Error        Error           `json:"error"`
}

type RpcServerResponse struct {
	Version Version          `json:"jsonrpc"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *Error           `json:"error,omitempty"`
	Id      Id               `json:"id"`
}

// This struct is for unmarshalling any jsonrpc message received by the client. It needs to be further processed by HandleMessage method in order to call subscribers or resolve requests.
type UnknownMessage struct {
	messageType  MessageType
	Request      RpcRequest
	Notification RpcNotification
	Response     RpcRawResponse
}
