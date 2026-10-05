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
	// ErrConnectionClosed is returned by SendRequest when there is no connection
	// or the connection closed before the response arrived.
	ErrConnectionClosed = errors.New("request was not done, websocket closed")
	// ErrAlreadyConnected is returned by Connect while the client is connected.
	ErrAlreadyConnected = errors.New("already connected")
	// ErrNotSubscribed is returned by Unsubscribe if the method has no subscriber.
	ErrNotSubscribed = errors.New("method is not subscribed")
)

const (
	defaultReadLimit      = 2048
	defaultRequestTimeout = 2 * time.Second
	defaultDialTimeout    = 10 * time.Second
)

// Client is a JSON-RPC 2.0 client over a websocket. It is safe for concurrent
// use. Connect runs the connection; SendRequest and Subscribe may be called
// from other goroutines while it runs.
type Client struct {
	nextID         atomic.Int32
	pending        requestResponseMap
	subscribers    subscriberRegistry
	readLimit      atomic.Int64
	requestTimeout atomic.Int64 // time.Duration
	dialTimeout    atomic.Int64 // time.Duration
	connected      atomic.Bool
	connMutex      sync.Mutex
	conn           *websocket.Conn
}

// NewClient returns a client that is not connected yet, see Connect.
func NewClient() *Client {
	c := &Client{
		pending:     requestResponseMap{store: make(map[requestID]chan response)},
		subscribers: subscriberRegistry{subscribers: make(map[Method]subscriber)},
	}
	c.readLimit.Store(defaultReadLimit)
	c.requestTimeout.Store(int64(defaultRequestTimeout))
	c.dialTimeout.Store(int64(defaultDialTimeout))
	return c
}

// SetReadLimit sets the maximum size in bytes of a received message
// (default 2048). It takes effect on the next Connect.
func (c *Client) SetReadLimit(newLimit int64) {
	c.readLimit.Store(newLimit)
}

// SetRequestTimeout sets how long SendRequest waits for a response
// (default 2s).
func (c *Client) SetRequestTimeout(timeout time.Duration) {
	c.requestTimeout.Store(int64(timeout))
}

// SetDialTimeout sets how long Connect waits for the websocket handshake
// (default 10s). It takes effect on the next Connect.
func (c *Client) SetDialTimeout(timeout time.Duration) {
	c.dialTimeout.Store(int64(timeout))
}

// Subscribe delivers notifications for method to the notifications channel,
// replacing any previous subscriber of method.
// Delivery never blocks: a notification is dropped if the channel is not ready
// to receive or ctx is done. Use a buffered channel to absorb bursts.
func (c *Client) Subscribe(ctx context.Context, method Method, notifications chan<- Notification) {
	c.subscribers.push(method, subscriber{notifications, ctx})
}

// Unsubscribe removes the subscriber of method. It returns ErrNotSubscribed if
// there is none. The channel is not closed.
func (c *Client) Unsubscribe(method Method) error {
	if !c.subscribers.pop(method) {
		return ErrNotSubscribed
	}
	return nil
}

// Connect dials address and reads messages until ctx is cancelled or the
// connection fails. It blocks for the lifetime of the connection and returns
// nil when ctx is cancelled. Pending requests fail with ErrConnectionClosed
// when it returns. Only one Connect may run at a time.
func (c *Client) Connect(ctx context.Context, address string, wsOptions *websocket.DialOptions) error {
	if !c.connected.CompareAndSwap(false, true) {
		return ErrAlreadyConnected
	}
	defer c.connected.Store(false)

	dialCtx, cancel := context.WithTimeout(ctx, time.Duration(c.dialTimeout.Load()))
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, address, wsOptions)
	if err != nil {
		return err
	}

	conn.SetReadLimit(c.readLimit.Load())
	c.connMutex.Lock()
	c.conn = conn
	c.connMutex.Unlock()
	defer func() {
		c.connMutex.Lock()
		c.conn = nil
		c.connMutex.Unlock()
		// Fail pending requests now instead of letting them run into the timeout.
		for _, responseChannel := range c.pending.popAll() {
			responseChannel <- response{err: ErrConnectionClosed}
		}
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		message := &incomingMessage{}
		if err := wsjson.Read(ctx, conn, message); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		if err := c.handleMessage(message); err != nil {
			return err
		}
	}
}

func (c *Client) handleMessage(message *incomingMessage) error {
	switch message.messageType {
	case messageTypeRequest:
		return fmt.Errorf("message type \"request\" currently not supported")
	case messageTypeNotification:
		s, ok := c.subscribers.get(message.notification.Method)
		if !ok || s.ctx.Err() != nil {
			return nil
		}
		// Never block the read loop: if the subscriber is not ready to receive,
		// the notification is dropped.
		select {
		case s.notifications <- Notification{s.ctx, message.notification.Params}:
		default:
		}
		return nil
	case messageTypeResponse:
		responseChannel, ok := c.pending.pop(message.response.ID)
		if !ok {
			// The request already timed out or was never sent by us. Dropping the
			// response keeps the connection alive.
			return nil
		}
		// responseChannel has a buffer of 1 and only the owner of the popped
		// entry sends on it, so this never blocks.
		res := response{result: message.response.Result}
		if message.response.Result == nil && message.response.Error != nil {
			res.err = message.response.Error
		}
		responseChannel <- res
		return nil
	}
	return fmt.Errorf("received unsupported message type: %d", message.messageType)
}
