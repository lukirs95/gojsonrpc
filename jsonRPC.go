package wsjsonrpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var (
	ErrOnDial = errors.New("connection with websocket failed")
)

type JsonRPC struct {
	idCounter          RequestId
	request            requestResponseMap
	subscriberRegistry *subscriberRegistry
	readLimit          int64
	connMutex          sync.Mutex
	conn               *websocket.Conn
}

func NewJsonRPC() *JsonRPC {
	return &JsonRPC{
		idCounter:          RequestId(0),
		request:            requestResponseMap{},
		subscriberRegistry: newSubscriberRegistry(),
		readLimit:          2048,
		connMutex:          sync.Mutex{},
		conn:               nil,
	}
}

func (jsonRPC *JsonRPC) SetReadLimit(newLimit int64) {
	jsonRPC.readLimit = newLimit
}

func (jsonRPC *JsonRPC) SubscribeMethod(ctx context.Context, method Method, notification chan Notification) {
	jsonRPC.subscriberRegistry.push(method, &Subscriber{notification, ctx})
}

func (jsonRPC *JsonRPC) UnsubscribeMethod(method Method) (*Subscriber, error) {
	return jsonRPC.subscriberRegistry.pop(method)
}

func (jsonRPC *JsonRPC) nextId() RequestId {
	jsonRPC.idCounter++
	return jsonRPC.idCounter
}

func (jsonRPC *JsonRPC) handleMessage(message *UnknownMessage) error {
	switch message.messageType {
	case M_TYPE_REQUEST:
		return fmt.Errorf("message type \"request\" currently not supported")
	case M_TYPE_NOTIFY:
		if !jsonRPC.subscriberRegistry.empty() {
			if subscriber, ok := jsonRPC.subscriberRegistry.subscriber[message.Notification.Method]; ok {
				subscriber.Notification <- Notification{subscriber.ctx, message.Notification.Params}
				return nil
			}
		}
		return nil
	case M_TYPE_RESPONSE:
		if !jsonRPC.request.empty() {
			responseChannel, err := jsonRPC.request.pop(message.Response.Id)
			if err != nil {
				return err
			}
			if message.Response.Result != nil {
				responseChannel <- RpcResponse{R_TYPE_RESULT, message.Response.Result, message.Response.Error}
			} else {
				responseChannel <- RpcResponse{R_TYPE_ERROR, message.Response.Result, message.Response.Error}
			}
			return nil
		} else {
			return fmt.Errorf("no request found for response with id %d, callstack is empty", message.Response.Id)
		}
	}
	return fmt.Errorf("received unsupported message type: %d", message.messageType)
}

func (jsonRPC *JsonRPC) Connect(parentCtx context.Context, address string, wsOptions *websocket.DialOptions) error {
	withTimeout, cancel := context.WithTimeout(parentCtx, time.Second*10)
	defer cancel()
	c, _, err := websocket.Dial(withTimeout, address, wsOptions)
	if err != nil {
		return err
	}

	jsonRPC.conn = c
	defer func() {
		c.Close(websocket.StatusNormalClosure, "")
	}()
	jsonRPC.conn.SetReadLimit(jsonRPC.readLimit)

	for {
		rpcMessage := &UnknownMessage{}

		if err := wsjson.Read(parentCtx, c, rpcMessage); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		if err := jsonRPC.handleMessage(rpcMessage); err != nil {
			return err
		}
	}
}
