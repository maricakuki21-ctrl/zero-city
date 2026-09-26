package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

type sub2ReaderWire struct {
	openAIWSFakeConn
	messages chan []byte
	closed   chan struct{}
	once     sync.Once
}

func newSub2ReaderWire() *sub2ReaderWire {
	return &sub2ReaderWire{messages: make(chan []byte), closed: make(chan struct{})}
}
func (*sub2ReaderWire) RequiresReaderLoop() bool { return true }
func (c *sub2ReaderWire) CloseNow() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}
func (c *sub2ReaderWire) Close() error { return c.CloseNow() }
func (c *sub2ReaderWire) ReadMessage(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, io.EOF
	case message := <-c.messages:
		return message, nil
	}
}

func TestSub2ReaderRejectsIdleData(t *testing.T) {
	pool, req := sub2PoolForTest(t, 1)
	wire := newSub2ReaderWire()
	conn := newOpenAIWSConn("dirty", req.Account.ID, wire, nil)
	defer conn.abort()
	ap := pool.getOrCreateAccountPool(req.Account.ID)
	ap.mu.Lock()
	ap.conns[conn.id] = conn
	ap.mu.Unlock()
	wire.messages <- []byte(`{"type":"response.output_text.delta","delta":"old result"}`)
	require.Eventually(t, func() bool { return len(conn.readerLoopResults) == 1 }, time.Second, time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	lease, err := pool.Acquire(ctx, req)
	require.NoError(t, err)
	defer lease.Release()
	require.NotEqual(t, "dirty", lease.ConnID())
	require.True(t, conn.isClosed())
}

func TestSub2ReaderCancellationClosesTransport(t *testing.T) {
	wire := newSub2ReaderWire()
	conn := newOpenAIWSConn("cancel-read", 1, wire, nil)
	defer conn.abort()
	require.True(t, conn.tryAcquire())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := conn.readMessage(ctx)
	require.ErrorIs(t, err, context.Canceled)
	select {
	case <-wire.closed:
	default:
		t.Fatal("cancellation did not stop the transport")
	}
	require.Eventually(t, func() bool {
		select {
		case _, ok := <-conn.readerLoopResults:
			return !ok
		default:
			return false
		}
	}, time.Second, time.Millisecond)
}

func TestSub2ReaderDrainsBufferedTerminalAfterPeerClose(t *testing.T) {
	wire := newSub2ReaderWire()
	conn := newOpenAIWSConn("drain", 1, wire, nil)
	defer conn.abort()
	require.True(t, conn.tryAcquire())
	terminal := []byte(`{"type":"response.completed"}`)
	wire.messages <- terminal
	require.Eventually(t, func() bool { return len(conn.readerLoopResults) == 1 }, time.Second, time.Millisecond)
	require.NoError(t, wire.CloseNow())
	require.Eventually(t, conn.isClosed, time.Second, time.Millisecond)
	got, err := conn.readMessageWithTimeout(time.Second)
	require.NoError(t, err)
	require.Equal(t, terminal, got)
	_, err = conn.readMessageWithTimeout(time.Second)
	require.ErrorIs(t, err, io.EOF)
}

func TestSub2ReaderIdleConnectionAnswersUpstreamPing(t *testing.T) {
	startPing := make(chan struct{})
	pingResult := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			pingResult <- err
			return
		}
		defer conn.CloseNow()
		readCtx := conn.CloseRead(r.Context())
		select {
		case <-startPing:
		case <-readCtx.Done():
			pingResult <- readCtx.Err()
			return
		}
		ctx, cancel := context.WithTimeout(readCtx, time.Second)
		defer cancel()
		pingResult <- conn.Ping(ctx)
		<-readCtx.Done()
	}))
	defer server.Close()
	pool, req := sub2PoolForTest(t, 1)
	defer pool.ClearAccount(req.Account.ID)
	pool.setClientDialerForTest(newDefaultOpenAIWSClientDialer())
	req.WSURL = "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	lease, err := pool.Acquire(ctx, req)
	require.NoError(t, err)
	id := lease.ConnID()
	lease.Release()
	close(startPing)
	select {
	case err := <-pingResult:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("idle client did not answer upstream ping")
	}
	reused, err := pool.Acquire(ctx, req)
	require.NoError(t, err)
	require.Equal(t, id, reused.ConnID())
	reused.Release()
	pool.ClearAccount(req.Account.ID)
}

func TestSub2ReaderPeerCloseEvictsIdlePoolEntry(t *testing.T) {
	pool, req := sub2PoolForTest(t, 1)
	wire := newSub2ReaderWire()
	conn := newOpenAIWSConn("peer-close", req.Account.ID, wire, nil)
	callback := func() { pool.evictConn(req.Account.ID, conn.id) }
	conn.onPeerClosed.Store(&callback)
	ap := pool.getOrCreateAccountPool(req.Account.ID)
	ap.mu.Lock()
	ap.conns[conn.id] = conn
	ap.mu.Unlock()
	require.NoError(t, wire.CloseNow())
	require.Eventually(t, func() bool {
		ap.mu.Lock()
		defer ap.mu.Unlock()
		return len(ap.conns) == 0
	}, time.Second, time.Millisecond)
	_, err := conn.readMessageWithTimeout(time.Second)
	require.True(t, errors.Is(err, io.EOF))
}
