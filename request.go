package gojsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// SendRequest sends params marshalled to JSON as a request for method and
// waits for the response. If the client is not connected yet, it first waits
// for the connection. Both waits together are bounded by the request timeout
// (see SetRequestTimeout) and ctx.
//
// It returns the raw result, an *Error if the server answered with an error,
// ErrConnectionClosed if no connection was established in time or it closed
// while waiting, or an error wrapping context.DeadlineExceeded if the server
// did not answer in time.
func (c *Client) SendRequest(ctx context.Context, method Method, params any) (json.RawMessage, error) {
	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.requestTimeout.Load()))
	defer cancel()

	conn, err := c.waitForConnection(ctx)
	if err != nil {
		return nil, err
	}

	id := requestID(c.nextID.Add(1))
	responseChannel := make(chan response, 1)
	c.pending.push(id, responseChannel)
	// If the connection closed before we registered, popAll has missed this
	// request. Otherwise a later disconnect fails it via popAll.
	c.connMutex.Lock()
	stillConnected := c.conn == conn
	c.connMutex.Unlock()
	if !stillConnected {
		c.pending.pop(id)
		return nil, ErrConnectionClosed
	}

	request := rpcRequest{Version: jsonrpcVersion, Method: method, Params: rawParams, ID: id}
	if err := wsjson.Write(ctx, conn, request); err != nil {
		c.pending.pop(id)
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.pending.pop(id)
		return nil, fmt.Errorf("request %s: %w", method, ctx.Err())
	case res := <-responseChannel:
		return res.result, res.err
	}
}

// waitForConnection returns the current connection, waiting for one until ctx
// is done.
func (c *Client) waitForConnection(ctx context.Context) (*websocket.Conn, error) {
	for {
		c.connMutex.Lock()
		conn, connected := c.conn, c.connectedCh
		c.connMutex.Unlock()
		if conn != nil {
			return conn, nil
		}
		select {
		case <-connected:
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %w", ErrConnectionClosed, ctx.Err())
		}
	}
}
