package gojsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// serverHandler is called by the test server for every request it receives.
// It runs in its own goroutine, so it may block or answer late.
type serverHandler func(ctx context.Context, conn *websocket.Conn, req RpcRequest)

// newTestServer starts a websocket server that decodes incoming JSON-RPC
// requests and passes them to handle. It returns the ws:// address.
func newTestServer(t *testing.T, handle serverHandler) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer conn.CloseNow()
		ctx := r.Context()
		for {
			var req RpcRequest
			if err := wsjson.Read(ctx, conn, &req); err != nil {
				return
			}
			go handle(ctx, conn, req)
		}
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

// connectClient runs Connect in the background and waits until the client is
// ready to send. The returned channel receives the result of Connect and is
// closed afterwards, so it can be read more than once.
func connectClient(t *testing.T, address string) (*JsonRPC, <-chan error) {
	t.Helper()
	rpc := NewJsonRPC()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- rpc.Connect(ctx, address, nil)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		rpc.connMutex.Lock()
		ready := rpc.conn != nil
		rpc.connMutex.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("client did not connect in time")
		}
		time.Sleep(5 * time.Millisecond)
	}

	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("Connect did not return after cancel")
		}
	})
	return rpc, done
}

func writeResult(ctx context.Context, conn *websocket.Conn, id RequestId, result any) {
	raw, _ := json.Marshal(result)
	msg := json.RawMessage(raw)
	wsjson.Write(ctx, conn, RpcServerResponse{Version: VERSION, Result: &msg, Id: id})
}

func writeError(ctx context.Context, conn *websocket.Conn, id RequestId, rpcErr Error) {
	wsjson.Write(ctx, conn, RpcServerResponse{Version: VERSION, Error: &rpcErr, Id: id})
}

func writeNotification(ctx context.Context, conn *websocket.Conn, method Method, params any) {
	raw, _ := json.Marshal(params)
	wsjson.Write(ctx, conn, RpcNotification{Version: VERSION, Method: method, Params: raw})
}

// assertStillConnected fails if Connect has returned, i.e. the connection was closed.
func assertStillConnected(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("connection closed unexpectedly: %v", err)
	default:
	}
}

func TestSendRequestResult(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		writeResult(ctx, conn, req.Id, req.Params) // echo
	})
	rpc, _ := connectClient(t, addr)

	result, err := rpc.SendRequest(context.Background(), "echo", map[string]int{"a": 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != `{"a":1}` {
		t.Fatalf("got result %s, want {\"a\":1}", result)
	}
}

func TestSendRequestSequentialIds(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		writeResult(ctx, conn, req.Id, req.Id)
	})
	rpc, _ := connectClient(t, addr)

	for want := 1; want <= 3; want++ {
		result, err := rpc.SendRequest(context.Background(), "id", nil)
		if err != nil {
			t.Fatalf("request %d: %v", want, err)
		}
		var got int
		if err := json.Unmarshal(result, &got); err != nil || got != want {
			t.Fatalf("request %d: server saw id %s", want, result)
		}
	}
}

// Problem 3: the error object of a response must reach the caller completely.
func TestSendRequestErrorKeepsCodeAndData(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		writeError(ctx, conn, req.Id, Error{
			Code:    -32000,
			Message: "boom",
			Data:    json.RawMessage(`{"detail":"disk full"}`),
		})
	})
	rpc, _ := connectClient(t, addr)

	_, err := rpc.SendRequest(context.Background(), "fail", nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	var rpcErr *Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error %q (%T) is not a *Error", err, err)
	}
	if rpcErr.Code != -32000 || rpcErr.Message != "boom" {
		t.Errorf("got code %d message %q, want -32000 \"boom\"", rpcErr.Code, rpcErr.Message)
	}
	if string(rpcErr.Data) != `{"detail":"disk full"}` {
		t.Errorf("got data %s, want {\"detail\":\"disk full\"}", rpcErr.Data)
	}
}

func TestSendRequestTimeout(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		// never answer
	})
	rpc, _ := connectClient(t, addr)
	rpc.SetRequestTimeout(100 * time.Millisecond)

	start := time.Now()
	_, err := rpc.SendRequest(context.Background(), "silent", nil)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("timeout took %v", elapsed)
	}
}

func TestSendRequestWithoutConnection(t *testing.T) {
	rpc := NewJsonRPC()
	if _, err := rpc.SendRequest(context.Background(), "echo", nil); !errors.Is(err, ErrConnectionClosed) {
		t.Fatalf("got %v, want ErrConnectionClosed", err)
	}
	if len(rpc.request.store) != 0 {
		t.Fatal("request was not removed from the pending map")
	}
}

// Problem 2: a response that arrives after the request timed out must not
// close the connection.
func TestLateResponseKeepsConnection(t *testing.T) {
	answered := make(chan struct{})
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		if req.Method == "slow" {
			time.Sleep(300 * time.Millisecond) // longer than the client timeout
			writeResult(ctx, conn, req.Id, "late")
			close(answered)
			return
		}
		writeResult(ctx, conn, req.Id, "ok")
	})
	rpc, done := connectClient(t, addr)
	rpc.SetRequestTimeout(100 * time.Millisecond)

	if _, err := rpc.SendRequest(context.Background(), "slow", nil); err == nil {
		t.Fatal("expected the slow request to time out")
	}
	<-answered
	time.Sleep(100 * time.Millisecond) // let the client read the late response

	assertStillConnected(t, done)
	if _, err := rpc.SendRequest(context.Background(), "fast", nil); err != nil {
		t.Fatalf("request after late response failed: %v", err)
	}
}

// Problem 2: a response with an id the client never sent must not close the
// connection either.
func TestUnknownResponseIdKeepsConnection(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		writeResult(ctx, conn, 9999, "stray")
		writeResult(ctx, conn, req.Id, "ok")
	})
	rpc, done := connectClient(t, addr)

	if _, err := rpc.SendRequest(context.Background(), "echo", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	assertStillConnected(t, done)
	if _, err := rpc.SendRequest(context.Background(), "echo", nil); err != nil {
		t.Fatalf("second request failed: %v", err)
	}
}

func TestNotificationDelivered(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		writeNotification(ctx, conn, "event", map[string]string{"hello": "world"})
		writeResult(ctx, conn, req.Id, "ok")
	})
	rpc, _ := connectClient(t, addr)

	notifications := make(chan Notification, 1)
	subCtx := context.WithValue(context.Background(), struct{}{}, "marker")
	rpc.SubscribeMethod(subCtx, "event", notifications)

	if _, err := rpc.SendRequest(context.Background(), "trigger", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case n := <-notifications:
		if string(n.Params) != `{"hello":"world"}` {
			t.Errorf("got params %s", n.Params)
		}
		if n.Ctx != subCtx {
			t.Error("notification does not carry the subscriber context")
		}
	case <-time.After(time.Second):
		t.Fatal("notification not delivered")
	}
}

func TestNotificationWithoutSubscriberIsIgnored(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		writeNotification(ctx, conn, "nobody-listens", nil)
		writeResult(ctx, conn, req.Id, "ok")
	})
	rpc, done := connectClient(t, addr)

	if _, err := rpc.SendRequest(context.Background(), "trigger", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertStillConnected(t, done)
}

func TestUnsubscribeMethod(t *testing.T) {
	rpc := NewJsonRPC()
	ch := make(chan Notification)
	rpc.SubscribeMethod(context.Background(), "event", ch)

	sub, err := rpc.UnsubscribeMethod("event")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub.Notification != Subscription(ch) {
		t.Error("returned subscriber has a different channel")
	}
	if _, err := rpc.UnsubscribeMethod("event"); err == nil {
		t.Error("expected an error when unsubscribing twice")
	}
}

// Problem 1: a subscriber that does not read its channel must not block the
// processing of responses.
func TestBlockedSubscriberDoesNotBlockResponses(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		writeNotification(ctx, conn, "event", nil)
		writeResult(ctx, conn, req.Id, "ok")
	})
	rpc, _ := connectClient(t, addr)

	rpc.SubscribeMethod(context.Background(), "event", make(chan Notification)) // never read

	if _, err := rpc.SendRequest(context.Background(), "trigger", nil); err != nil {
		t.Fatalf("request blocked by subscriber: %v", err)
	}
}

func TestNotificationDroppedAfterSubscriberContextDone(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		writeNotification(ctx, conn, "event", nil)
		writeResult(ctx, conn, req.Id, "ok")
	})
	rpc, _ := connectClient(t, addr)

	subCtx, cancel := context.WithCancel(context.Background())
	cancel()
	notifications := make(chan Notification, 1)
	rpc.SubscribeMethod(subCtx, "event", notifications)

	if _, err := rpc.SendRequest(context.Background(), "trigger", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case <-notifications:
		t.Fatal("notification delivered to subscriber with done context")
	default:
	}
}

// Run with -race: requests, subscriptions and notifications run concurrently.
func TestConcurrentRequestsAndSubscriptions(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		writeNotification(ctx, conn, "event", nil)
		writeResult(ctx, conn, req.Id, req.Params) // echo
	})
	rpc, _ := connectClient(t, addr)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := rpc.SendRequest(context.Background(), "echo", i)
			if err != nil {
				t.Errorf("request %d: %v", i, err)
				return
			}
			if string(result) != strconv.Itoa(i) {
				t.Errorf("request %d got result %s", i, result)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 50 {
			rpc.SubscribeMethod(context.Background(), "event", make(chan Notification, 1))
			rpc.UnsubscribeMethod("event")
		}
	}()
	wg.Wait()
}

func TestPendingRequestFailsOnDisconnect(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {
		conn.Close(websocket.StatusGoingAway, "bye") // close instead of answering
	})
	rpc, done := connectClient(t, addr)
	rpc.SetRequestTimeout(5 * time.Second)

	start := time.Now()
	_, err := rpc.SendRequest(context.Background(), "hang", nil)
	if !errors.Is(err, ErrConnectionClosed) {
		t.Fatalf("got %v, want ErrConnectionClosed", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("request waited %v instead of failing on disconnect", elapsed)
	}
	if err := <-done; err == nil {
		t.Fatal("Connect returned nil after the server closed the connection")
	}
	if _, err := rpc.SendRequest(context.Background(), "after", nil); !errors.Is(err, ErrConnectionClosed) {
		t.Fatalf("request after disconnect: got %v, want ErrConnectionClosed", err)
	}
}

func TestConnectTwice(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {})
	rpc, _ := connectClient(t, addr)

	if err := rpc.Connect(context.Background(), addr, nil); err == nil {
		t.Fatal("expected an error on second Connect")
	}
}

func TestConnectCancelReturnsNil(t *testing.T) {
	addr := newTestServer(t, func(ctx context.Context, conn *websocket.Conn, req RpcRequest) {})
	rpc := NewJsonRPC()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- rpc.Connect(ctx, addr, nil) }()

	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("got %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Connect did not return after cancel")
	}
}

func TestConnectDialError(t *testing.T) {
	rpc := NewJsonRPC()
	if err := rpc.Connect(context.Background(), "ws://127.0.0.1:1", nil); err == nil {
		t.Fatal("expected a dial error")
	}
}

func TestConnectDialTimeout(t *testing.T) {
	// A TCP listener that never answers the websocket handshake.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
		}
	}()

	rpc := NewJsonRPC()
	rpc.SetDialTimeout(100 * time.Millisecond)
	start := time.Now()
	if err := rpc.Connect(context.Background(), "ws://"+listener.Addr().String(), nil); err == nil {
		t.Fatal("expected a dial timeout")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("dial took %v", elapsed)
	}
}

func TestUnmarshalUnknownMessage(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    MessageType
		wantErr bool
	}{
		{"result", `{"jsonrpc":"2.0","result":{"a":1},"id":1}`, M_TYPE_RESPONSE, false},
		{"null result", `{"jsonrpc":"2.0","result":null,"id":1}`, M_TYPE_RESPONSE, false},
		{"error", `{"jsonrpc":"2.0","error":{"code":-1,"message":"x"},"id":1}`, M_TYPE_RESPONSE, false},
		{"notification", `{"jsonrpc":"2.0","method":"event","params":[1]}`, M_TYPE_NOTIFY, false},
		{"request", `{"jsonrpc":"2.0","method":"ping","params":[],"id":7}`, M_TYPE_REQUEST, false},
		{"unknown", `{"jsonrpc":"2.0","id":1}`, 0, true},
		{"invalid json", `{`, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m UnknownMessage
			err := json.Unmarshal([]byte(tt.raw), &m)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && m.messageType != tt.want {
				t.Fatalf("got message type %d, want %d", m.messageType, tt.want)
			}
		})
	}
}

func TestUnmarshalKeepsErrorData(t *testing.T) {
	var m UnknownMessage
	raw := `{"jsonrpc":"2.0","error":{"code":-32000,"message":"boom","data":{"x":1}},"id":3}`
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if m.Response.Id != 3 || m.Response.Error.Code != -32000 || string(m.Response.Error.Data) != `{"x":1}` {
		t.Fatalf("got %+v", m.Response)
	}
}
