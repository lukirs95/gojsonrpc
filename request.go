package gojsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket/wsjson"
)

func (rpc *JsonRPC) SendRequest(ctx context.Context, method Method, request any) (json.RawMessage, error) {
	params, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	responseChannel := make(ResponseChan, 1)
	message := rpc.newRequest(method, params, responseChannel)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	rpc.connMutex.Lock()
	conn := rpc.conn
	rpc.connMutex.Unlock()
	if conn == nil {
		rpc.deleteRequest(message.Id)
		return nil, fmt.Errorf("request was not done, websocket closed")
	}

	if err := wsjson.Write(ctx, conn, message); err != nil {
		rpc.deleteRequest(message.Id)
		return nil, err
	}

	select {
	case <-ctxWithTimeout.Done():
		rpc.deleteRequest(message.Id)
		return nil, fmt.Errorf("timeout exeeded")
	case response := <-responseChannel:
		switch response.ResponseType {
		case R_TYPE_ERROR:
			rpcErr := response.Error
			return nil, &rpcErr
		case R_TYPE_RESULT:
			return response.Result, nil
		case R_TYPE_DELETED:
			return nil, fmt.Errorf("request was not done, request was deleted")
		}
	}
	return nil, fmt.Errorf("request failed, select statement did not work")
}

func (jsonRPC *JsonRPC) newRequest(method Method, params json.RawMessage, responseChannel ResponseChan) *RpcRequest {
	id := jsonRPC.nextId()
	jsonRPC.request.push(id, responseChannel)
	return &RpcRequest{
		Version: VERSION,
		Method:  method,
		Params:  params,
		Id:      id,
	}
}

func (jsonRPC *JsonRPC) deleteRequest(id RequestId) error {
	responseChannel, err := jsonRPC.request.pop(id)
	if err != nil {
		return err
	}
	responseChannel <- RpcResponse{R_TYPE_DELETED, nil, Error{}}
	return nil
}
