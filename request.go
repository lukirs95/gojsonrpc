package gojsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket/wsjson"
)

// SendRequest sends params marshalled to JSON as a request for method and
// waits for the response. It returns the raw result, an *Error if the server
// answered with an error, ErrConnectionClosed if there is no connection or it
// closed, or an error wrapping context.DeadlineExceeded after the request
// timeout (see SetRequestTimeout).
func (c *Client) SendRequest(ctx context.Context, method Method, params any) (json.RawMessage, error) {
	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.requestTimeout.Load()))
	defer cancel()

	// Register before reading conn, so a disconnect in between still fails
	// this request via popAll.
	id := requestID(c.nextID.Add(1))
	responseChannel := make(chan response, 1)
	c.pending.push(id, responseChannel)

	c.connMutex.Lock()
	conn := c.conn
	c.connMutex.Unlock()
	if conn == nil {
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
