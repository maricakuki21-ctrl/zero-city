package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sub2PreemptPausedCache struct {
	openAIWSSessionPreemptCacheStub
	claimed chan struct{}
	resume  chan struct{}
	calls   atomic.Int32
}

func (c *sub2PreemptPausedCache) ClaimOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, hash string, owner []byte, ttl time.Duration) ([]byte, error) {
	call := c.calls.Add(1)
	previous, err := c.openAIWSSessionPreemptCacheStub.ClaimOpenAIResponsesSessionWindow(ctx, groupID, hash, owner, ttl)
	if call == 1 {
		close(c.claimed)
		select {
		case <-c.resume:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return previous, err
}

func TestSub2PreemptionSerializesRemoteAndLocalRegistration(t *testing.T) {
	cache := &sub2PreemptPausedCache{claimed: make(chan struct{}), resume: make(chan struct{})}
	svc := &OpenAIGatewayService{cache: cache}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	key := openAIWSSessionPreemptKey{groupID: 7, apiKeyID: 11, sessionHash: "scope"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type registration struct {
		ctx     context.Context
		cleanup func()
		armed   bool
	}
	begin := func(out chan<- registration) {
		next, cleanup, armed, _ := svc.beginOpenAIWSSessionPreemptContext(ctx, account, 7, 11, "scope", false, nil)
		out <- registration{ctx: next, cleanup: cleanup, armed: armed}
	}
	first := make(chan registration, 1)
	second := make(chan registration, 1)
	go begin(first)
	select {
	case <-cache.claimed:
	case <-ctx.Done():
		t.Fatal("first claim never reached the cache")
	}
	go begin(second)
	require.Eventually(t, func() bool {
		svc.openaiWSSessionPreemptions.mu.Lock()
		defer svc.openaiWSSessionPreemptions.mu.Unlock()
		start := svc.openaiWSSessionPreemptions.starts[key]
		return start != nil && start.refs == 2
	}, time.Second, time.Millisecond)
	require.EqualValues(t, 1, cache.calls.Load(), "second claim must wait for the first local registration")
	close(cache.resume)
	var a, b registration
	select {
	case a = <-first:
	case <-ctx.Done():
		t.Fatal("first registration stuck")
	}
	defer a.cleanup()
	select {
	case b = <-second:
	case <-ctx.Done():
		t.Fatal("replacement registration stuck")
	}
	defer b.cleanup()
	require.True(t, a.armed)
	require.True(t, b.armed)
	require.True(t, isOpenAIWSSessionPreempted(a.ctx))
	require.NoError(t, b.ctx.Err())
	a.cleanup()
	svc.openaiWSSessionPreemptions.mu.Lock()
	lockCount := len(svc.openaiWSSessionPreemptions.starts)
	entryCount := len(svc.openaiWSSessionPreemptions.active)
	svc.openaiWSSessionPreemptions.mu.Unlock()
	require.Zero(t, lockCount, "registration locks must not accumulate per session")
	require.Equal(t, 1, entryCount, "stale cleanup must preserve the replacement")
}

func TestSub2PreemptionLateCallbackPreservesSuccessorState(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	svc := &OpenAIGatewayService{openaiWSStateStore: store}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	key := openAIWSSessionPreemptKey{groupID: 7, apiKeyID: 11, sessionHash: "scope"}
	_, firstCleanup, armed, _ := svc.beginOpenAIWSSessionPreemptContext(context.Background(), account, 7, 11, "scope", false, nil)
	require.True(t, armed)
	svc.openaiWSSessionPreemptions.mu.Lock()
	latePreempt := svc.openaiWSSessionPreemptions.active[key].cancel
	svc.openaiWSSessionPreemptions.mu.Unlock()
	firstCleanup()
	secondCtx, secondCleanup, armed, _ := svc.beginOpenAIWSSessionPreemptContext(context.Background(), account, 7, 11, "scope", false, nil)
	require.True(t, armed)
	defer secondCleanup()
	store.BindSessionTurnState(7, "scope", "new-turn", time.Hour)
	store.BindSessionConn(7, "scope", "new-connection", time.Hour)

	// A remote watch that was already in flight may report loss after cleanup.
	latePreempt()
	turn, ok := store.GetSessionTurnState(7, "scope")
	require.True(t, ok)
	require.Equal(t, "new-turn", turn)
	conn, ok := store.GetSessionConn(7, "scope")
	require.True(t, ok)
	require.Equal(t, "new-connection", conn)
	require.NoError(t, secondCtx.Err())
}

func TestSub2PreemptionRemoteOwnerLossCancelsOnlyOldInstance(t *testing.T) {
	cache := &openAIWSSessionPreemptCacheStub{}
	firstService := &OpenAIGatewayService{cache: cache}
	secondService := &OpenAIGatewayService{cache: cache}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	firstCtx, firstCleanup, armed, _ := firstService.beginOpenAIWSSessionPreemptContext(context.Background(), account, 7, 11, "scope", false, nil)
	require.True(t, armed)
	defer firstCleanup()
	secondCtx, secondCleanup, armed, _ := secondService.beginOpenAIWSSessionPreemptContext(context.Background(), account, 7, 11, "scope", false, nil)
	require.True(t, armed)
	defer secondCleanup()
	select {
	case <-firstCtx.Done():
		require.True(t, isOpenAIWSSessionPreempted(firstCtx))
	case <-time.After(2 * openAIWSSessionPreemptWatchInterval):
		t.Fatal("old service did not observe ownership loss")
	}
	require.NoError(t, secondCtx.Err())
}

func TestSub2PreemptionFencesInflightAndLateStateWrites(t *testing.T) {
	store := NewOpenAIWSStateStore(nil)
	svc := &OpenAIGatewayService{openaiWSStateStore: store}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	firstCtx, firstCleanup, armed, _ := svc.beginOpenAIWSSessionPreemptContext(ctx, account, 7, 11, "scope", false, nil)
	require.True(t, armed)
	defer firstCleanup()
	writeEntered := make(chan struct{})
	finishWrite := make(chan struct{})
	writeDone := make(chan bool, 1)
	go func() {
		writeDone <- withOpenAIWSSessionStateWrite(firstCtx, func() {
			close(writeEntered)
			select {
			case <-finishWrite:
			case <-ctx.Done():
			}
			store.BindSessionTurnState(7, "scope", "old-turn", time.Hour)
			store.BindSessionConn(7, "scope", "old-connection", time.Hour)
		})
	}()
	select {
	case <-writeEntered:
	case <-ctx.Done():
		t.Fatal("old write did not start")
	}
	type registration struct {
		ctx     context.Context
		cleanup func()
	}
	replacement := make(chan registration, 1)
	go func() {
		next, cleanup, _, _ := svc.beginOpenAIWSSessionPreemptContext(ctx, account, 7, 11, "scope", false, nil)
		replacement <- registration{next, cleanup}
	}()
	require.Eventually(t, func() bool {
		svc.openaiWSSessionPreemptions.mu.Lock()
		defer svc.openaiWSSessionPreemptions.mu.Unlock()
		return svc.openaiWSSessionPreemptions.next == 2
	}, time.Second, time.Millisecond)
	select {
	case <-replacement:
		t.Fatal("successor started before the old state write was fenced")
	default:
	}
	close(finishWrite)
	require.True(t, <-writeDone)
	var second registration
	select {
	case second = <-replacement:
	case <-ctx.Done():
		t.Fatal("replacement remained blocked")
	}
	defer second.cleanup()
	_, oldTurnExists := store.GetSessionTurnState(7, "scope")
	require.False(t, oldTurnExists, "successor must clear the completed old write")
	require.True(t, withOpenAIWSSessionStateWrite(second.ctx, func() {
		store.BindSessionTurnState(7, "scope", "new-turn", time.Hour)
		store.BindSessionConn(7, "scope", "new-connection", time.Hour)
	}))
	require.False(t, withOpenAIWSSessionStateWrite(firstCtx, func() {
		store.BindSessionTurnState(7, "scope", "stale-turn", time.Hour)
		store.BindSessionConn(7, "scope", "stale-connection", time.Hour)
	}))
	turn, ok := store.GetSessionTurnState(7, "scope")
	require.True(t, ok)
	require.Equal(t, "new-turn", turn)
	conn, ok := store.GetSessionConn(7, "scope")
	require.True(t, ok)
	require.Equal(t, "new-connection", conn)
}
