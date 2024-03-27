package jsonrpc

import "encoding/json"

func NewNotification(method Method, params json.RawMessage) *RpcNotification {
	return &RpcNotification{Version: VERSION, Method: method, Params: params}
}
