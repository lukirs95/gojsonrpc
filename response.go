package wsjsonrpc

import "encoding/json"

func NewResponseResult(id RequestId, result json.RawMessage) *RpcServerResponse {
	var res json.RawMessage = result
	return &RpcServerResponse{Version: VERSION, Id: id, Result: &res}
}

func NewResponseError(id RequestId, err Error) *RpcServerResponse {
	return &RpcServerResponse{Version: VERSION, Id: id, Error: &err}
}
