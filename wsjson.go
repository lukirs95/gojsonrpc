// This is a plane copy of github.com/coder/websocket/wsjson
// with the only difference that on failed json unmarshalling
// the connection doesn't get closed!
package wsjsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/coder/websocket"
)

var bpool = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

// Get returns a buffer from the pool or creates a new one if
// the pool is empty.
func bpoolGet() *bytes.Buffer {
	b := bpool.Get()
	return b.(*bytes.Buffer)
}

// Put returns a buffer into the pool.
func bpoolPut(b *bytes.Buffer) {
	b.Reset()
	bpool.Put(b)
}

func wsjsonread(ctx context.Context, c *websocket.Conn, v interface{}) (err error) {
	_, r, err := c.Reader(ctx)
	if err != nil {
		return err
	}

	b := bpoolGet()
	defer bpoolPut(b)

	_, err = b.ReadFrom(r)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b.Bytes(), v)
	if err != nil {
		c.Close(websocket.StatusInvalidFramePayloadData, "failed to unmarshal JSON")
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// WriterFunc is used to implement one off io.Writers.
type jsonWriterFunc func(p []byte) (int, error)

func (f jsonWriterFunc) Write(p []byte) (int, error) {
	return f(p)
}

func wsjsonwrite(ctx context.Context, c *websocket.Conn, v interface{}) (err error) {
	// json.Marshal cannot reuse buffers between calls as it has to return
	// a copy of the byte slice but Encoder does as it directly writes to w.
	err = json.NewEncoder(jsonWriterFunc(func(p []byte) (int, error) {
		err := c.Write(ctx, websocket.MessageText, p)
		if err != nil {
			return 0, err
		}
		return len(p), nil
	})).Encode(v)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return nil
}
