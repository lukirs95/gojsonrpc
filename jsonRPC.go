package gojsonrpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var (
	ErrOnDial = errors.New("connection with websocket failed")
	// ErrConnectionClosed is returned by SendRequest when there is no connection
	// or the connection closed before the response arrived.
	ErrConnectionClosed = errors.New("request was not done, websocket closed")
)

const (
	defaultReadLimit      = 2048
	defaultRequestTimeout = 2 * time.Second
	defaultDialTimeout    = 10 * time.Second
)

type JsonRPC struct {
	idCounter          atomic.Int32
	request            requestResponseMap
	subscriberRegistry *subscriberRegistry
	readLimit          atomic.Int64
	requestTimeout     atomic.Int64 // time.Duration
	dialTimeout        atomic.Int64 // time.Duration
	connMutex          sync.Mutex
	conn               *websocket.Conn
	once               atomic.Bool
}

func NewJsonRPC() *JsonRPC {
	jsonRPC := &JsonRPC{
		request: requestResponseMap{
			store: make(map[RequestId]ResponseChan),
		},
		subscriberRegistry: newSubscriberRegistry(),
		connMutex:          sync.Mutex{},
		conn:               nil,
	}
	jsonRPC.readLimit.Store(defaultReadLimit)
	jsonRPC.requestTimeout.Store(int64(defaultRequestTimeout))
	jsonRPC.dialTimeout.Store(int64(defaultDialTimeout))
	return jsonRPC
}

// SetReadLimit sets the maximum size in bytes of a received message
// (default 2048). It takes effect on the next Connect.
func (jsonRPC *JsonRPC) SetReadLimit(newLimit int64) {
	jsonRPC.readLimit.Store(newLimit)
}

// SetRequestTimeout sets how long SendRequest waits for a response
// (default 2s).
func (jsonRPC *JsonRPC) SetRequestTimeout(timeout time.Duration) {
	jsonRPC.requestTimeout.Store(int64(timeout))
}

// SetDialTimeout sets how long Connect waits for the websocket handshake
// (default 10s). It takes effect on the next Connect.
func (jsonRPC *JsonRPC) SetDialTimeout(timeout time.Duration) {
	jsonRPC.dialTimeout.Store(int64(timeout))
}

// SubscribeMethod delivers notifications for method to the notification channel.
// Delivery never blocks: a notification is dropped if the channel is not ready
// to receive or ctx is done. Use a buffered channel to absorb bursts.
func (jsonRPC *JsonRPC) SubscribeMethod(ctx context.Context, method Method, notification chan Notification) {
	jsonRPC.subscriberRegistry.push(method, &Subscriber{notification, ctx})
}

func (jsonRPC *JsonRPC) UnsubscribeMethod(method Method) (*Subscriber, error) {
	return jsonRPC.subscriberRegistry.pop(method)
}

func (jsonRPC *JsonRPC) nextId() RequestId {
	return RequestId(jsonRPC.idCounter.Add(1))
}

func (jsonRPC *JsonRPC) handleMessage(message *UnknownMessage) error {
	switch message.messageType {
	case M_TYPE_REQUEST:
		return fmt.Errorf("message type \"request\" currently not supported")
	case M_TYPE_NOTIFY:
		subscriber, ok := jsonRPC.subscriberRegistry.get(message.Notification.Method)
		if !ok || subscriber.ctx.Err() != nil {
			return nil
		}
		// Never block the read loop: if the subscriber is not ready to receive,
		// the notification is dropped.
		select {
		case subscriber.Notification <- Notification{subscriber.ctx, message.Notification.Params}:
		default:
		}
		return nil
	case M_TYPE_RESPONSE:
		responseChannel, err := jsonRPC.request.pop(message.Response.Id)
		if err != nil {
			// The request already timed out or was never sent by us. Dropping the
			// response keeps the connection alive.
			return nil
		}
		// responseChannel has a buffer of 1 and only the owner of the popped
		// entry sends on it, so this never blocks.
		if message.Response.Result != nil {
			responseChannel <- RpcResponse{R_TYPE_RESULT, message.Response.Result, message.Response.Error}
		} else {
			responseChannel <- RpcResponse{R_TYPE_ERROR, message.Response.Result, message.Response.Error}
		}
		return nil
	}
	return fmt.Errorf("received unsupported message type: %d", message.messageType)
}

func (jsonRPC *JsonRPC) Connect(parentCtx context.Context, address string, wsOptions *websocket.DialOptions) error {
	if !jsonRPC.once.CompareAndSwap(false, true) {
		return fmt.Errorf("already connected")
	}
	defer jsonRPC.once.Store(false)

	withTimeout, cancel := context.WithTimeout(parentCtx, time.Duration(jsonRPC.dialTimeout.Load()))
	defer cancel()
	c, _, err := websocket.Dial(withTimeout, address, wsOptions)
	if err != nil {
		return err
	}

	c.SetReadLimit(jsonRPC.readLimit.Load())
	jsonRPC.connMutex.Lock()
	jsonRPC.conn = c
	jsonRPC.connMutex.Unlock()
	defer func() {
		jsonRPC.connMutex.Lock()
		jsonRPC.conn = nil
		jsonRPC.connMutex.Unlock()
		// Fail pending requests now instead of letting them run into the timeout.
		for _, responseChannel := range jsonRPC.request.popAll() {
			responseChannel <- RpcResponse{R_TYPE_DELETED, nil, Error{}}
		}
		c.Close(websocket.StatusNormalClosure, "")
	}()

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
