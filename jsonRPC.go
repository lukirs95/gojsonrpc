package jsonrpc

import (
	"context"
	"fmt"
	"sync"

	"github.com/lukirs95/websocket"
	"github.com/lukirs95/websocket/wsjson"
)

type JsonRPC struct {
	idCounter          Id
	callStackMutex     sync.Mutex
	callStack          *callStack
	subscriberRegistry *subscriberRegistry
	connMutex          sync.Mutex
	conn               *websocket.Conn
	OnConnect          func() error
	OnDisconnect       func()
}

func NewJsonRPC() *JsonRPC {
	return &JsonRPC{
		idCounter:          Id(0),
		callStackMutex:     sync.Mutex{},
		callStack:          newCallStack(),
		subscriberRegistry: newSubscriberRegistry(),
		connMutex:          sync.Mutex{},
		conn:               nil,
		OnConnect:          func() error { return nil },
		OnDisconnect:       func() {},
	}
}

func (jsonRPC *JsonRPC) SubscribeMethod(ctx context.Context, method Method, notification chan Notification) {
	jsonRPC.subscriberRegistry.push(method, &Subscriber{notification, ctx})
}

func (jsonRPC *JsonRPC) UnsubscribeMethod(method Method) (*Subscriber, error) {
	return jsonRPC.subscriberRegistry.pop(method)
}

func (jsonRPC *JsonRPC) nextId() Id {
	jsonRPC.idCounter++
	return jsonRPC.idCounter
}

func (jsonRPC *JsonRPC) HandleMessage(message *UnknownMessage) error {
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
	case M_TYPE_RESPONSE:
		if !jsonRPC.callStack.empty() {
			jsonRPC.callStackMutex.Lock()
			responseChannel, err := jsonRPC.callStack.pop(message.Response.Id)
			jsonRPC.callStackMutex.Unlock()
			if err != nil {
				return err
			}
			if message.Response.Result != nil {
				*responseChannel <- RpcResponse{R_TYPE_RESULT, message.Response.Result, message.Response.Error}
			} else {
				*responseChannel <- RpcResponse{R_TYPE_ERROR, message.Response.Result, message.Response.Error}
			}
			return nil
		} else {
			return fmt.Errorf("no request found for response with id %d, callstack is empty", message.Response.Id)
		}
	}
	return fmt.Errorf("received unsupported message type: %d", message.messageType)
}

func (jsonRPC *JsonRPC) Listen(ctx context.Context, address string) error {
	c, _, dialErr := websocket.Dial(ctx, address, nil)
	if dialErr != nil {
		return dialErr
	}

	jsonRPC.conn = c

	if err := jsonRPC.OnConnect(); err != nil {
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			break
		}

		rpcMessage := &UnknownMessage{}
		if err := wsjson.Read(ctx, c, rpcMessage); err != nil {
			dialErr = err
			break
		}

		if err := jsonRPC.HandleMessage(rpcMessage); err != nil {
			dialErr = err
			break
		}
	}

	jsonRPC.OnDisconnect()
	return dialErr
}
