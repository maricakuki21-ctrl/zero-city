package service

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func sub2PoolForTest(t *testing.T, capacity int) (*openAIWSConnPool, openAIWSAcquireRequest) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = capacity
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = capacity
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(&openAIWSFakeDialer{})
	t.Cleanup(pool.Close)
	return pool, openAIWSAcquireRequest{
		Account: &Account{ID: 920, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
		WSURL:   "wss://example.test/v1/responses",
	}
}

func TestSub2PoolCanceledWaiterReturnsToken(t *testing.T) {
	for _, topologyWait := range []bool{false, true} {
		conn := newOpenAIWSConn("cancel", 1, &openAIWSFakeConn{}, nil)
		defer conn.close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for range 100 {
			var err error
			if topologyWait {
				err = conn.acquireOrPoolChanged(ctx, make(chan struct{}))
			} else {
				err = conn.acquire(ctx)
			}
			require.ErrorIs(t, err, context.Canceled)
			require.True(t, conn.tryAcquire(), "canceled waiter must not strand the lease")
			conn.release()
		}
	}
}

func TestSub2PoolWaiterUsesAnyReleasedConnection(t *testing.T) {
	pool, req := sub2PoolForTest(t, 2)
	first, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	defer first.Release()
	second, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	defer second.Release()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result := make(chan *openAIWSConnLease, 1)
	errs := make(chan error, 1)
	go func() {
		lease, err := pool.Acquire(ctx, req)
		result <- lease
		errs <- err
	}()
	require.Eventually(t, func() bool {
		return first.conn.waiters.Load()+second.conn.waiters.Load() == 1
	}, time.Second, time.Millisecond)
	released := second
	if second.conn.waiters.Load() > 0 {
		released = first
	}
	released.Release()
	select {
	case lease := <-result:
		require.NoError(t, <-errs)
		require.NotNil(t, lease)
		defer lease.Release()
		require.Equal(t, released.ConnID(), lease.ConnID())
		require.Greater(t, lease.QueueWaitDuration(), time.Duration(0))
	case <-ctx.Done():
		t.Fatal("waiter did not reselect a free connection")
	}
	require.Zero(t, first.conn.waiters.Load()+second.conn.waiters.Load())
}

type sub2PoolPausedDialer struct {
	started chan struct{}
	resume  chan struct{}
	conn    *openAIWSFakeConn
	calls   atomic.Int32
}

func (d *sub2PoolPausedDialer) Dial(ctx context.Context, _ string, _ http.Header, _ string) (openAIWSClientConn, int, http.Header, error) {
	d.calls.Add(1)
	close(d.started)
	select {
	case <-ctx.Done():
		return nil, 0, nil, ctx.Err()
	case <-d.resume:
		return d.conn, 0, nil, nil
	}
}

func TestSub2PoolClearRejectsInflightOldCredentials(t *testing.T) {
	pool, req := sub2PoolForTest(t, 1)
	dialer := &sub2PoolPausedDialer{started: make(chan struct{}), resume: make(chan struct{}), conn: &openAIWSFakeConn{}}
	pool.setClientDialerForTest(dialer)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		lease, err := pool.Acquire(ctx, req)
		if lease != nil {
			lease.Release()
		}
		done <- err
	}()
	select {
	case <-dialer.started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	pool.ClearAccount(req.Account.ID)
	close(dialer.resume)
	select {
	case err := <-done:
		require.ErrorIs(t, err, errOpenAIWSConnClosed)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.Equal(t, int32(1), dialer.calls.Load())
	ap := pool.getOrCreateAccountPool(req.Account.ID)
	ap.mu.Lock()
	count, creating, last := len(ap.conns), ap.creating, ap.lastAcquire
	ap.mu.Unlock()
	require.Zero(t, count)
	require.Zero(t, creating)
	require.Nil(t, last)
}

type sub2PoolProbeConn struct {
	openAIWSFakeConn
	started chan struct{}
	finish  chan struct{}
}

func (c *sub2PoolProbeConn) Ping(ctx context.Context) error {
	close(c.started)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.finish:
		return errors.New("probe failed")
	}
}

func TestSub2PoolProbeDoesNotEvictConcurrentBorrower(t *testing.T) {
	pool, req := sub2PoolForTest(t, 1)
	wire := &sub2PoolProbeConn{started: make(chan struct{}), finish: make(chan struct{})}
	conn := newOpenAIWSConn("probe", req.Account.ID, wire, nil)
	ap := pool.getOrCreateAccountPool(req.Account.ID)
	ap.mu.Lock()
	ap.conns[conn.id] = conn
	ap.mu.Unlock()
	done := make(chan struct{})
	go func() { pool.runBackgroundPingSweep(); close(done) }()
	select {
	case <-wire.started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	lease, err := pool.Acquire(context.Background(), req)
	require.NoError(t, err)
	defer lease.Release()
	close(wire.finish)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("probe did not finish")
	}
	select {
	case <-conn.closedCh:
		t.Fatal("probe closed a borrowed connection")
	default:
	}
	require.NoError(t, lease.WriteJSON(map[string]any{"type": "response.create"}, time.Second))
}
