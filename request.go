package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lukirs95/websocket/wsjson"
)

type Request interface {
	Marshal() ([]byte, error)
	Method() Method
}

func (jsonRPC *JsonRPC) NewRequest(method Method, params json.RawMessage, responseChannel *Response) *RpcRequest {
	jsonRPC.callStackMutex.Lock()
	defer jsonRPC.callStackMutex.Unlock()
	jsonRPC.callStack.push(jsonRPC.nextId(), responseChannel)
	return &RpcRequest{
		Version: VERSION,
		Method:  method,
		Params:  params,
		Id:      jsonRPC.idCounter,
	}
}

func (jsonRPC *JsonRPC) DeleteRequest(id Id) error {
	jsonRPC.callStackMutex.Lock()
	defer jsonRPC.callStackMutex.Unlock()
	responseChannel, err := jsonRPC.callStack.pop(id)
	if err != nil {
		return err
	}
	*responseChannel <- RpcResponse{R_TYPE_DELETED, nil, Error{}}
	return nil
}

func (rpc *JsonRPC) SendRequest(ctx context.Context, request Request) (json.RawMessage, error) {
	params, err := request.Marshal()
	if err != nil {
		return nil, err
	}

	responseChannel := make(Response, 1)
	message := rpc.NewRequest(request.Method(), params, &responseChannel)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	rpc.connMutex.Lock()
	if rpc.conn == nil {
		rpc.connMutex.Unlock()
		return nil, fmt.Errorf("request was not done, websocket closed")
	}
	rpc.connMutex.Unlock()

	if err := wsjson.Write(ctx, rpc.conn, message); err != nil {
		rpc.DeleteRequest(message.Id)
		close(responseChannel)
		return nil, err
	}

	select {
	case <-ctxWithTimeout.Done():
		rpc.DeleteRequest(message.Id)
		close(responseChannel)
		return nil, fmt.Errorf("timeout exeeded")
	case response := <-responseChannel:
		switch response.ResponseType {
		case R_TYPE_ERROR:
			return nil, fmt.Errorf("response error, %s", response.Error.Message)
		case R_TYPE_RESULT:
			close(responseChannel)
			return response.Result, nil
		case R_TYPE_DELETED:
			return nil, fmt.Errorf("request was not done, request was deleted")
		}
	}
	return nil, fmt.Errorf("request failed, select statement did not work")
}
