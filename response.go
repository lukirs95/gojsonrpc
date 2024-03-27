package jsonrpc

import "encoding/json"

func NewResponseResult(id Id, result json.RawMessage) *RpcServerResponse {
	var res json.RawMessage = result
	return &RpcServerResponse{Version: VERSION, Id: id, Result: &res}
}

func NewResponseError(id Id, err Error) *RpcServerResponse {
	return &RpcServerResponse{Version: VERSION, Id: id, Error: &err}
}
