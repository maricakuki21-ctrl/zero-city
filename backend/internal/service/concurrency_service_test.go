//go:build unit

package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// stubConcurrencyCacheForTest 用于并发服务单元测试的缓存桩
type stubConcurrencyCacheForTest struct {
	acquireResult        bool
	acquireErr           error
	releaseErr           error
	concurrency          int
	concurrencyErr       error
	waitAllowed          bool
	waitErr              error
	waitCount            int
	waitCountErr         error
	loadBatch            map[int64]*AccountLoadInfo
	loadBatchErr         error
	usersLoadBatch       map[int64]*UserLoadInfo
	usersLoadErr         error
	cleanupErr           error
	apiKeyTrackErr       error
	apiKeyReleaseErr     error
	apiKeyConcurrency    map[int64]int
	apiKeyConcurrencyErr error

	// 记录调用
	releasedAccountIDs       []int64
	releasedRequestIDs       []string
	loadBatchCalls           atomic.Int64
	trackedAPIKeyIDs         []int64
	trackedAPIKeyRequestIDs  []string
	releasedAPIKeyIDs        []int64
	releasedAPIKeyRequestIDs []string
}

type ingressLeaseCacheForTest struct {
	stubConcurrencyCacheForTest
	acquireIngressResult bool
	acquireIngressErr    error
	acquireIngressFn     func(context.Context, int64, int, string) (bool, error)
	refreshIngressResult bool
	refreshIngressErr    error
	refreshIngressFn     func(context.Context, int64, string) (bool, error)
	releaseIngressErr    error
	releaseIngressFn     func(context.Context, int64, string) error
	acquireIngressCalls  int
	refreshIngressCalls  int
	releaseIngressCalls  int
}

type heartbeatSlotCallForTest struct {
	id             int64
	maxConcurrency int
	requestID      string
}

type heartbeatConcurrencyCacheForTest struct {
	stubConcurrencyCacheForTest

	mu                 sync.Mutex
	accountAcquireFn   func(context.Context, int64, int, string) (bool, error)
	accountRefreshFn   func(context.Context, int64, string) (bool, error)
	accountAcquires    []heartbeatSlotCallForTest
	userAcquires       []heartbeatSlotCallForTest
	accountRefreshes   []heartbeatSlotCallForTest
	userRefreshes      []heartbeatSlotCallForTest
	accountReleases    []heartbeatSlotCallForTest
	userReleases       []heartbeatSlotCallForTest
	accountRefreshLost bool
	userRefreshLost    bool
	accountRefreshErr  error
	userRefreshErr     error
}

func (c *heartbeatConcurrencyCacheForTest) AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
	c.mu.Lock()
	c.accountAcquires = append(c.accountAcquires, heartbeatSlotCallForTest{id: accountID, maxConcurrency: maxConcurrency, requestID: requestID})
	fn := c.accountAcquireFn
	c.mu.Unlock()
	if fn != nil {
		return fn(ctx, accountID, maxConcurrency, requestID)
	}
	return true, nil
}

func (c *heartbeatConcurrencyCacheForTest) RefreshAccountSlot(ctx context.Context, accountID int64, requestID string) (bool, error) {
	c.mu.Lock()
	c.accountRefreshes = append(c.accountRefreshes, heartbeatSlotCallForTest{id: accountID, requestID: requestID})
	fn := c.accountRefreshFn
	refreshErr := c.accountRefreshErr
	refreshLost := c.accountRefreshLost
	c.mu.Unlock()
	if fn != nil {
		return fn(ctx, accountID, requestID)
	}
	if refreshErr != nil {
		return false, refreshErr
	}
	return !refreshLost, nil
}

func (c *heartbeatConcurrencyCacheForTest) ReleaseAccountSlot(_ context.Context, accountID int64, requestID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accountReleases = append(c.accountReleases, heartbeatSlotCallForTest{id: accountID, requestID: requestID})
	return nil
}

func (c *heartbeatConcurrencyCacheForTest) AcquireUserSlot(_ context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userAcquires = append(c.userAcquires, heartbeatSlotCallForTest{id: userID, maxConcurrency: maxConcurrency, requestID: requestID})
	return true, nil
}

func (c *heartbeatConcurrencyCacheForTest) RefreshUserSlot(_ context.Context, userID int64, requestID string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userRefreshes = append(c.userRefreshes, heartbeatSlotCallForTest{id: userID, requestID: requestID})
	if c.userRefreshErr != nil {
		return false, c.userRefreshErr
	}
	return !c.userRefreshLost, nil
}

func (c *heartbeatConcurrencyCacheForTest) ReleaseUserSlot(_ context.Context, userID int64, requestID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userReleases = append(c.userReleases, heartbeatSlotCallForTest{id: userID, requestID: requestID})
	return nil
}

func (c *heartbeatConcurrencyCacheForTest) accountSnapshot() ([]heartbeatSlotCallForTest, []heartbeatSlotCallForTest, []heartbeatSlotCallForTest) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]heartbeatSlotCallForTest(nil), c.accountAcquires...), append([]heartbeatSlotCallForTest(nil), c.accountRefreshes...), append([]heartbeatSlotCallForTest(nil), c.accountReleases...)
}

func (c *heartbeatConcurrencyCacheForTest) userSnapshot() ([]heartbeatSlotCallForTest, []heartbeatSlotCallForTest, []heartbeatSlotCallForTest) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]heartbeatSlotCallForTest(nil), c.userAcquires...), append([]heartbeatSlotCallForTest(nil), c.userRefreshes...), append([]heartbeatSlotCallForTest(nil), c.userReleases...)
}

func (c *ingressLeaseCacheForTest) AcquireOpenAIWSIngressLease(ctx context.Context, apiKeyID int64, maxConnections int, leaseID string) (bool, error) {
	c.acquireIngressCalls++
	if c.acquireIngressFn != nil {
		return c.acquireIngressFn(ctx, apiKeyID, maxConnections, leaseID)
	}
	return c.acquireIngressResult, c.acquireIngressErr
}

func (c *ingressLeaseCacheForTest) RefreshOpenAIWSIngressLease(ctx context.Context, apiKeyID int64, leaseID string) (bool, error) {
	c.refreshIngressCalls++
	if c.refreshIngressFn != nil {
		return c.refreshIngressFn(ctx, apiKeyID, leaseID)
	}
	return c.refreshIngressResult, c.refreshIngressErr
}

func (c *ingressLeaseCacheForTest) ReleaseOpenAIWSIngressLease(ctx context.Context, apiKeyID int64, leaseID string) error {
	c.releaseIngressCalls++
	if c.releaseIngressFn != nil {
		return c.releaseIngressFn(ctx, apiKeyID, leaseID)
	}
	return c.releaseIngressErr
}

var _ ConcurrencyCache = (*stubConcurrencyCacheForTest)(nil)
var _ OpenAIWSIngressLeaseCache = (*ingressLeaseCacheForTest)(nil)
var _ ConcurrencyCache = (*heartbeatConcurrencyCacheForTest)(nil)
var _ ConcurrencySlotLeaseCache = (*heartbeatConcurrencyCacheForTest)(nil)

func (c *stubConcurrencyCacheForTest) AcquireAccountSlot(_ context.Context, _ int64, _ int, _ string) (bool, error) {
	return c.acquireResult, c.acquireErr
}
func (c *stubConcurrencyCacheForTest) ReleaseAccountSlot(_ context.Context, accountID int64, requestID string) error {
	c.releasedAccountIDs = append(c.releasedAccountIDs, accountID)
	c.releasedRequestIDs = append(c.releasedRequestIDs, requestID)
	return c.releaseErr
}
func (c *stubConcurrencyCacheForTest) GetAccountConcurrency(_ context.Context, _ int64) (int, error) {
	return c.concurrency, c.concurrencyErr
}
func (c *stubConcurrencyCacheForTest) GetAccountConcurrencyBatch(_ context.Context, accountIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int, len(accountIDs))
	for _, accountID := range accountIDs {
		if c.concurrencyErr != nil {
			return nil, c.concurrencyErr
		}
		result[accountID] = c.concurrency
	}
	return result, nil
}
func (c *stubConcurrencyCacheForTest) IncrementAccountWaitCount(_ context.Context, _ int64, _ int) (bool, error) {
	return c.waitAllowed, c.waitErr
}
func (c *stubConcurrencyCacheForTest) DecrementAccountWaitCount(_ context.Context, _ int64) error {
	return nil
}
func (c *stubConcurrencyCacheForTest) GetAccountWaitingCount(_ context.Context, _ int64) (int, error) {
	return c.waitCount, c.waitCountErr
}
func (c *stubConcurrencyCacheForTest) AcquireUserSlot(_ context.Context, _ int64, _ int, _ string) (bool, error) {
	return c.acquireResult, c.acquireErr
}
func (c *stubConcurrencyCacheForTest) ReleaseUserSlot(_ context.Context, _ int64, _ string) error {
	return c.releaseErr
}
func (c *stubConcurrencyCacheForTest) GetUserConcurrency(_ context.Context, _ int64) (int, error) {
	return c.concurrency, c.concurrencyErr
}
func (c *stubConcurrencyCacheForTest) TrackAPIKeySlot(_ context.Context, apiKeyID int64, requestID string) error {
	c.trackedAPIKeyIDs = append(c.trackedAPIKeyIDs, apiKeyID)
	c.trackedAPIKeyRequestIDs = append(c.trackedAPIKeyRequestIDs, requestID)
	return c.apiKeyTrackErr
}
func (c *stubConcurrencyCacheForTest) ReleaseAPIKeySlot(_ context.Context, apiKeyID int64, requestID string) error {
	c.releasedAPIKeyIDs = append(c.releasedAPIKeyIDs, apiKeyID)
	c.releasedAPIKeyRequestIDs = append(c.releasedAPIKeyRequestIDs, requestID)
	return c.apiKeyReleaseErr
}
func (c *stubConcurrencyCacheForTest) GetAPIKeyConcurrencyBatch(_ context.Context, apiKeyIDs []int64) (map[int64]int, error) {
	if c.apiKeyConcurrencyErr != nil {
		return nil, c.apiKeyConcurrencyErr
	}
	result := make(map[int64]int, len(apiKeyIDs))
	for _, apiKeyID := range apiKeyIDs {
		result[apiKeyID] = c.apiKeyConcurrency[apiKeyID]
	}
	return result, nil
}
func (c *stubConcurrencyCacheForTest) IncrementWaitCount(_ context.Context, _ int64, _ int) (bool, error) {
	return c.waitAllowed, c.waitErr
}
func (c *stubConcurrencyCacheForTest) DecrementWaitCount(_ context.Context, _ int64) error {
	return nil
}
func (c *stubConcurrencyCacheForTest) GetAccountsLoadBatch(_ context.Context, _ []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	c.loadBatchCalls.Add(1)
	return c.loadBatch, c.loadBatchErr
}
func (c *stubConcurrencyCacheForTest) GetUsersLoadBatch(_ context.Context, _ []UserWithConcurrency) (map[int64]*UserLoadInfo, error) {
	return c.usersLoadBatch, c.usersLoadErr
}
func (c *stubConcurrencyCacheForTest) CleanupExpiredAccountSlots(_ context.Context, _ int64) error {
	return c.cleanupErr
}

func (c *stubConcurrencyCacheForTest) CleanupExpiredAccountSlotKeys(_ context.Context) error {
	return c.cleanupErr
}

func (c *stubConcurrencyCacheForTest) CleanupStaleProcessSlots(_ context.Context, _ string) error {
	return c.cleanupErr
}

type trackingConcurrencyCache struct {
	stubConcurrencyCacheForTest
	cleanupPrefix string
}

func (c *trackingConcurrencyCache) CleanupStaleProcessSlots(_ context.Context, prefix string) error {
	c.cleanupPrefix = prefix
	return c.cleanupErr
}

func TestCleanupStaleProcessSlots_NilCache(t *testing.T) {
	svc := &ConcurrencyService{cache: nil}
	require.NoError(t, svc.CleanupStaleProcessSlots(context.Background()))
}

func TestCleanupStaleProcessSlots_DelegatesPrefix(t *testing.T) {
	cache := &trackingConcurrencyCache{}
	svc := NewConcurrencyService(cache)
	require.NoError(t, svc.CleanupStaleProcessSlots(context.Background()))
	require.Equal(t, RequestIDPrefix(), cache.cleanupPrefix)
}

func TestProvideConcurrencyServiceNeverRunsPrefixCleanupDuringRuntimeOverlap(t *testing.T) {
	cfg := &config.Config{}
	activeCache := &trackingConcurrencyCache{}
	standbyCache := &trackingConcurrencyCache{}
	active := config.ProvideBackgroundRuntime(&config.Config{BackgroundRuntimeRole: config.BackgroundRuntimeRoleActive})
	standby := config.ProvideBackgroundRuntime(&config.Config{BackgroundRuntimeRole: config.BackgroundRuntimeRoleStandby})

	ProvideConcurrencyService(activeCache, nil, cfg, active)
	ProvideConcurrencyService(standbyCache, nil, cfg, standby)

	require.Empty(t, activeCache.cleanupPrefix, "active startup must preserve slots owned by overlapping HTTP runtimes")
	require.Empty(t, standbyCache.cleanupPrefix)
}

func TestProvideConcurrencyServiceInjectsSlotHeartbeatFromTTL(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.ConcurrencySlotTTLMinutes = 12
	standby := config.ProvideBackgroundRuntime(&config.Config{BackgroundRuntimeRole: config.BackgroundRuntimeRoleStandby})

	svc := ProvideConcurrencyService(&stubConcurrencyCacheForTest{}, nil, cfg, standby)

	require.Equal(t, 4*time.Minute, time.Duration(svc.slotHeartbeatInterval.Load()))
}

func TestAcquireAccountSlot_Success(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: true}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAccountSlot(context.Background(), 1, 5)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.NotNil(t, result.ReleaseFunc)
}

func TestAcquireAccountSlot_Failure(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: false}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAccountSlot(context.Background(), 1, 5)
	require.NoError(t, err)
	require.False(t, result.Acquired)
	require.Nil(t, result.ReleaseFunc)
}

func TestAcquireAccountSlot_UnlimitedConcurrency(t *testing.T) {
	svc := NewConcurrencyService(&stubConcurrencyCacheForTest{})

	for _, maxConcurrency := range []int{0, -1} {
		result, err := svc.AcquireAccountSlot(context.Background(), 1, maxConcurrency)
		require.NoError(t, err)
		require.True(t, result.Acquired, "maxConcurrency=%d 应无限制通过", maxConcurrency)
		require.NotNil(t, result.ReleaseFunc, "ReleaseFunc 应为 no-op 函数")
	}
}

func TestAcquireAccountSlot_CacheError(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireErr: errors.New("redis down")}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAccountSlot(context.Background(), 1, 5)
	require.Error(t, err)
	require.Nil(t, result)
}

func TestAcquireAccountSlot_ReleaseDecrements(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: true}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAccountSlot(context.Background(), 42, 5)
	require.NoError(t, err)
	require.True(t, result.Acquired)

	// 调用 ReleaseFunc 应释放槽位
	result.ReleaseFunc()

	require.Len(t, cache.releasedAccountIDs, 1)
	require.Equal(t, int64(42), cache.releasedAccountIDs[0])
	require.Len(t, cache.releasedRequestIDs, 1)
	require.NotEmpty(t, cache.releasedRequestIDs[0], "requestID 不应为空")
}

func TestAcquireUserSlot_IndependentFromAccount(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: true}
	svc := NewConcurrencyService(cache)

	// 用户槽位获取应独立于账户槽位
	result, err := svc.AcquireUserSlot(context.Background(), 100, 3)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.NotNil(t, result.ReleaseFunc)
}

func TestAcquireUserSlot_UnlimitedConcurrency(t *testing.T) {
	svc := NewConcurrencyService(&stubConcurrencyCacheForTest{})

	result, err := svc.AcquireUserSlot(context.Background(), 1, 0)
	require.NoError(t, err)
	require.True(t, result.Acquired)
}

func TestConcurrencySlotHeartbeatRefreshesAndStopsBeforeIdempotentRelease(t *testing.T) {
	const heartbeatInterval = 5 * time.Millisecond

	for _, tc := range []struct {
		name     string
		acquire  func(*ConcurrencyService, context.Context) (*AcquireResult, error)
		snapshot func(*heartbeatConcurrencyCacheForTest) ([]heartbeatSlotCallForTest, []heartbeatSlotCallForTest, []heartbeatSlotCallForTest)
		wantID   int64
		wantMax  int
	}{
		{
			name: "account",
			acquire: func(svc *ConcurrencyService, ctx context.Context) (*AcquireResult, error) {
				return svc.AcquireAccountSlot(ctx, 41, 3)
			},
			snapshot: func(cache *heartbeatConcurrencyCacheForTest) ([]heartbeatSlotCallForTest, []heartbeatSlotCallForTest, []heartbeatSlotCallForTest) {
				return cache.accountSnapshot()
			},
			wantID:  41,
			wantMax: 3,
		},
		{
			name: "user",
			acquire: func(svc *ConcurrencyService, ctx context.Context) (*AcquireResult, error) {
				return svc.AcquireUserSlot(ctx, 52, 4)
			},
			snapshot: func(cache *heartbeatConcurrencyCacheForTest) ([]heartbeatSlotCallForTest, []heartbeatSlotCallForTest, []heartbeatSlotCallForTest) {
				return cache.userSnapshot()
			},
			wantID:  52,
			wantMax: 4,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := &heartbeatConcurrencyCacheForTest{}
			svc := NewConcurrencyService(cache)
			svc.SetSlotHeartbeatInterval(heartbeatInterval)
			ctx, cancel := WithConcurrencySlotLeaseCancellation(context.Background())
			defer cancel(nil)

			result, err := tc.acquire(svc, ctx)
			require.NoError(t, err)
			require.True(t, result.Acquired)
			require.Eventually(t, func() bool {
				_, refreshes, _ := tc.snapshot(cache)
				return len(refreshes) >= 2
			}, time.Second, time.Millisecond, "long request should refresh its slot more than once")

			acquires, refreshes, _ := tc.snapshot(cache)
			require.Len(t, acquires, 1, "heartbeat must never reuse the capacity-acquire operation")
			require.NotEmpty(t, acquires[0].requestID)
			require.Equal(t, tc.wantID, acquires[0].id)
			require.Equal(t, tc.wantMax, acquires[0].maxConcurrency)
			for _, call := range refreshes {
				require.Equal(t, tc.wantID, call.id)
				require.Equal(t, acquires[0].requestID, call.requestID, "heartbeat must refresh the original member")
			}

			result.ReleaseFunc()
			result.ReleaseFunc()
			_, refreshesAfterRelease, releases := tc.snapshot(cache)
			require.Len(t, releases, 1, "release must be idempotent")
			require.Equal(t, tc.wantID, releases[0].id)
			require.Equal(t, acquires[0].requestID, releases[0].requestID)

			time.Sleep(4 * heartbeatInterval)
			_, finalRefreshes, finalReleases := tc.snapshot(cache)
			require.Len(t, finalRefreshes, len(refreshesAfterRelease), "heartbeat must stop before release returns")
			require.Len(t, finalReleases, 1)
		})
	}
}

func TestConcurrencySlotHeartbeatStopsWhenRequestContextIsCanceled(t *testing.T) {
	const heartbeatInterval = 5 * time.Millisecond
	cache := &heartbeatConcurrencyCacheForTest{}
	svc := NewConcurrencyService(cache)
	svc.SetSlotHeartbeatInterval(heartbeatInterval)
	ctx, cancel := WithConcurrencySlotLeaseCancellation(context.Background())

	result, err := svc.AcquireAccountSlot(ctx, 61, 2)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, refreshes, _ := cache.accountSnapshot()
		return len(refreshes) >= 1
	}, time.Second, time.Millisecond)

	cancel(nil)
	time.Sleep(2 * heartbeatInterval)
	_, refreshesAfterCancel, _ := cache.accountSnapshot()
	time.Sleep(3 * heartbeatInterval)
	_, finalRefreshes, _ := cache.accountSnapshot()
	require.Len(t, finalRefreshes, len(refreshesAfterCancel), "canceled request must not retain a heartbeat goroutine")

	result.ReleaseFunc()
	_, _, releases := cache.accountSnapshot()
	require.Len(t, releases, 1)
}

func TestConcurrencySlotHeartbeatRefreshErrorDoesNotPreventRelease(t *testing.T) {
	cache := &heartbeatConcurrencyCacheForTest{accountRefreshErr: errors.New("redis unavailable")}
	svc := NewConcurrencyService(cache)
	svc.SetSlotHeartbeatInterval(5 * time.Millisecond)
	svc.SetSlotLeaseTTL(time.Second)
	ctx, cancel := WithConcurrencySlotLeaseCancellation(context.Background())
	defer cancel(nil)

	result, err := svc.AcquireAccountSlot(ctx, 71, 2)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, refreshes, _ := cache.accountSnapshot()
		return len(refreshes) >= 2
	}, time.Second, time.Millisecond, "refresh errors should be retried without terminating the process")
	require.NoError(t, ctx.Err(), "a transient Redis failure below the lease TTL must not cancel the request")

	require.NotPanics(t, result.ReleaseFunc)
	_, _, releases := cache.accountSnapshot()
	require.Len(t, releases, 1)
}

func TestConcurrencySlotHeartbeatMissingMemberCancelsRequestWithoutReacquiring(t *testing.T) {
	const heartbeatInterval = 5 * time.Millisecond

	for _, tc := range []struct {
		name     string
		cache    *heartbeatConcurrencyCacheForTest
		acquire  func(*ConcurrencyService, context.Context) (*AcquireResult, error)
		snapshot func(*heartbeatConcurrencyCacheForTest) ([]heartbeatSlotCallForTest, []heartbeatSlotCallForTest, []heartbeatSlotCallForTest)
	}{
		{
			name:  "account member removed while capacity is held by another request",
			cache: &heartbeatConcurrencyCacheForTest{accountRefreshLost: true},
			acquire: func(svc *ConcurrencyService, ctx context.Context) (*AcquireResult, error) {
				return svc.AcquireAccountSlot(ctx, 81, 1)
			},
			snapshot: func(cache *heartbeatConcurrencyCacheForTest) ([]heartbeatSlotCallForTest, []heartbeatSlotCallForTest, []heartbeatSlotCallForTest) {
				return cache.accountSnapshot()
			},
		},
		{
			name:  "user member removed while capacity is held by another request",
			cache: &heartbeatConcurrencyCacheForTest{userRefreshLost: true},
			acquire: func(svc *ConcurrencyService, ctx context.Context) (*AcquireResult, error) {
				return svc.AcquireUserSlot(ctx, 82, 1)
			},
			snapshot: func(cache *heartbeatConcurrencyCacheForTest) ([]heartbeatSlotCallForTest, []heartbeatSlotCallForTest, []heartbeatSlotCallForTest) {
				return cache.userSnapshot()
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewConcurrencyService(tc.cache)
			svc.SetSlotHeartbeatInterval(heartbeatInterval)
			requestCtx, cancel := WithConcurrencySlotLeaseCancellation(context.Background())
			defer cancel(nil)

			result, err := tc.acquire(svc, requestCtx)
			require.NoError(t, err)
			require.True(t, result.Acquired)
			defer result.ReleaseFunc()

			select {
			case <-requestCtx.Done():
			case <-time.After(time.Second):
				t.Fatal("missing slot member did not cancel the upstream request context")
			}
			require.ErrorIs(t, context.Cause(requestCtx), ErrConcurrencySlotLeaseLost)

			acquires, refreshes, _ := tc.snapshot(tc.cache)
			require.Len(t, acquires, 1, "heartbeat must not enter the acquire path to reclaim a lost slot")
			require.Len(t, refreshes, 1, "confirmed missing ownership must stop heartbeat immediately")
			require.Equal(t, acquires[0].requestID, refreshes[0].requestID)
		})
	}
}

func TestConcurrencySlotHeartbeatFailsClosedWithoutRequestCancelOwner(t *testing.T) {
	svc := NewConcurrencyService(&heartbeatConcurrencyCacheForTest{})
	svc.SetSlotHeartbeatInterval(5 * time.Millisecond)

	result, err := svc.AcquireAccountSlot(context.Background(), 91, 1)
	require.Nil(t, result)
	require.ErrorContains(t, err, "request cancellation context is unavailable")
}

func TestConcurrencySlotHeartbeatInitialDeadlineUsesAcquireCallStart(t *testing.T) {
	const leaseTTL = 40 * time.Millisecond
	acquireStarted := make(chan struct{})
	finishAcquire := make(chan struct{})
	cache := &heartbeatConcurrencyCacheForTest{}
	cache.accountAcquireFn = func(context.Context, int64, int, string) (bool, error) {
		close(acquireStarted)
		<-finishAcquire
		return true, nil
	}

	svc := NewConcurrencyService(cache)
	svc.SetSlotHeartbeatInterval(time.Hour)
	svc.SetSlotLeaseTTL(leaseTTL)
	requestCtx, cancel := WithConcurrencySlotLeaseCancellation(context.Background())
	defer cancel(nil)

	type acquireOutcome struct {
		result *AcquireResult
		err    error
	}
	resultCh := make(chan acquireOutcome, 1)
	go func() {
		result, err := svc.AcquireAccountSlot(requestCtx, 92, 1)
		resultCh <- acquireOutcome{result: result, err: err}
	}()

	<-acquireStarted
	time.Sleep(2 * leaseTTL)
	close(finishAcquire)
	outcome := <-resultCh
	require.NoError(t, outcome.err)
	require.NotNil(t, outcome.result)
	defer outcome.result.ReleaseFunc()

	select {
	case <-requestCtx.Done():
		require.ErrorIs(t, context.Cause(requestCtx), ErrConcurrencySlotLeaseLost)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("lease deadline was incorrectly restarted after the acquire call returned")
	}
}

func TestConcurrencySlotWatchdogCancelsWhileRefreshIsBlocked(t *testing.T) {
	const leaseTTL = 80 * time.Millisecond
	acquireObservedAt := make(chan time.Time, 1)
	refreshDeadline := make(chan time.Time, 1)
	refreshStarted := make(chan struct{})
	unblockRefresh := make(chan struct{})
	cache := &heartbeatConcurrencyCacheForTest{}
	cache.accountAcquireFn = func(context.Context, int64, int, string) (bool, error) {
		acquireObservedAt <- time.Now()
		return true, nil
	}
	cache.accountRefreshFn = func(ctx context.Context, _ int64, _ string) (bool, error) {
		deadline, _ := ctx.Deadline()
		refreshDeadline <- deadline
		close(refreshStarted)
		<-unblockRefresh // Deliberately ignore ctx to prove watchdog independence.
		return false, ctx.Err()
	}

	svc := NewConcurrencyService(cache)
	svc.SetSlotHeartbeatInterval(10 * time.Millisecond)
	svc.SetSlotLeaseTTL(leaseTTL)
	requestCtx, cancel := WithConcurrencySlotLeaseCancellation(context.Background())
	defer cancel(nil)

	result, err := svc.AcquireAccountSlot(requestCtx, 93, 1)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	<-refreshStarted
	gotRefreshDeadline := <-refreshDeadline
	require.False(t, gotRefreshDeadline.IsZero())
	require.False(t, gotRefreshDeadline.After((<-acquireObservedAt).Add(leaseTTL)), "refresh timeout must not outlive the current lease")

	select {
	case <-requestCtx.Done():
		require.ErrorIs(t, context.Cause(requestCtx), ErrConcurrencySlotLeaseLost)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("blocked refresh prevented the independent lease watchdog from canceling the request")
	}
	close(unblockRefresh)
	result.ReleaseFunc()
}

func TestConcurrencySlotReleaseStopsWatchdogAndSuccessfulRefreshUsesCallStart(t *testing.T) {
	const leaseTTL = 120 * time.Millisecond
	firstRefreshObservedAt := make(chan time.Time, 1)
	secondRefreshDeadline := make(chan time.Time, 1)
	finishFirstRefresh := make(chan struct{})
	finishSecondRefresh := make(chan struct{})
	var refreshCalls atomic.Int64
	cache := &heartbeatConcurrencyCacheForTest{}
	cache.accountRefreshFn = func(ctx context.Context, _ int64, _ string) (bool, error) {
		switch refreshCalls.Add(1) {
		case 1:
			firstRefreshObservedAt <- time.Now()
			<-finishFirstRefresh
			return true, nil
		default:
			deadline, _ := ctx.Deadline()
			secondRefreshDeadline <- deadline
			<-finishSecondRefresh // Ignore ctx so ReleaseFunc must join this worker.
			return false, ctx.Err()
		}
	}

	svc := NewConcurrencyService(cache)
	svc.SetSlotHeartbeatInterval(10 * time.Millisecond)
	svc.SetSlotLeaseTTL(leaseTTL)
	requestCtx, cancel := WithConcurrencySlotLeaseCancellation(context.Background())
	defer cancel(nil)

	result, err := svc.AcquireAccountSlot(requestCtx, 94, 1)
	require.NoError(t, err)
	firstStartedAt := <-firstRefreshObservedAt
	time.Sleep(30 * time.Millisecond)
	close(finishFirstRefresh)
	secondDeadline := <-secondRefreshDeadline
	require.False(t, secondDeadline.IsZero())
	require.False(t, secondDeadline.After(firstStartedAt.Add(leaseTTL)), "successful refresh must extend from its conservative pre-call time")

	releaseDone := make(chan struct{})
	go func() {
		result.ReleaseFunc()
		close(releaseDone)
	}()
	time.Sleep(leaseTTL)
	require.NoError(t, requestCtx.Err(), "normal release must disarm the watchdog before waiting for refresh")
	close(finishSecondRefresh)
	select {
	case <-releaseDone:
	case <-time.After(time.Second):
		t.Fatal("release did not finish after the blocked refresh returned")
	}
	require.NoError(t, requestCtx.Err(), "watchdog canceled the request after a normal release")
}

func TestTrackAPIKeySlot_ReleaseDecrements(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{}
	svc := NewConcurrencyService(cache)

	release := svc.TrackAPIKeySlot(context.Background(), 88)
	require.NotNil(t, release)
	require.Equal(t, []int64{88}, cache.trackedAPIKeyIDs)
	require.Len(t, cache.trackedAPIKeyRequestIDs, 1)
	require.NotEmpty(t, cache.trackedAPIKeyRequestIDs[0])

	release()

	require.Equal(t, []int64{88}, cache.releasedAPIKeyIDs)
	require.Equal(t, cache.trackedAPIKeyRequestIDs, cache.releasedAPIKeyRequestIDs)
}

func TestTrackAPIKeySlot_FailOpen(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{apiKeyTrackErr: errors.New("redis down")}
	svc := NewConcurrencyService(cache)

	release := svc.TrackAPIKeySlot(context.Background(), 88)
	require.NotNil(t, release)
	require.Equal(t, []int64{88}, cache.trackedAPIKeyIDs)

	require.NotPanics(t, release)
	require.Empty(t, cache.releasedAPIKeyIDs)
}

func TestGetAPIKeyConcurrencyBatch_Fallbacks(t *testing.T) {
	t.Run("nil cache returns zeroes", func(t *testing.T) {
		svc := &ConcurrencyService{cache: nil}

		counts, err := svc.GetAPIKeyConcurrencyBatch(context.Background(), []int64{1, 2})
		require.NoError(t, err)
		require.Equal(t, map[int64]int{1: 0, 2: 0}, counts)
	})

	t.Run("redis error returns zeroes", func(t *testing.T) {
		cache := &stubConcurrencyCacheForTest{apiKeyConcurrencyErr: errors.New("redis down")}
		svc := NewConcurrencyService(cache)

		counts, err := svc.GetAPIKeyConcurrencyBatch(context.Background(), []int64{1, 2})
		require.NoError(t, err)
		require.Equal(t, map[int64]int{1: 0, 2: 0}, counts)
	})

	t.Run("success returns counts", func(t *testing.T) {
		cache := &stubConcurrencyCacheForTest{apiKeyConcurrency: map[int64]int{1: 3, 2: 0}}
		svc := NewConcurrencyService(cache)

		counts, err := svc.GetAPIKeyConcurrencyBatch(context.Background(), []int64{1, 2})
		require.NoError(t, err)
		require.Equal(t, map[int64]int{1: 3, 2: 0}, counts)
	})
}

func TestAcquireOpenAIWSIngressLease(t *testing.T) {
	t.Run("zero value release is safe", func(t *testing.T) {
		var lease OpenAIWSIngressLease
		require.NotPanics(t, lease.Release)
	})

	t.Run("disabled", func(t *testing.T) {
		cache := &ingressLeaseCacheForTest{}
		lease, acquired, err := NewConcurrencyService(cache).AcquireOpenAIWSIngressLease(nil, 1, 0)
		require.NoError(t, err)
		require.True(t, acquired)
		require.Nil(t, lease)
		require.Zero(t, cache.acquireIngressCalls)
	})

	t.Run("unsupported cache fails closed", func(t *testing.T) {
		lease, acquired, err := NewConcurrencyService(&stubConcurrencyCacheForTest{}).AcquireOpenAIWSIngressLease(context.Background(), 1, 1)
		require.Error(t, err)
		require.False(t, acquired)
		require.Nil(t, lease)
	})

	t.Run("capacity rejected", func(t *testing.T) {
		cache := &ingressLeaseCacheForTest{acquireIngressResult: false}
		lease, acquired, err := NewConcurrencyService(cache).AcquireOpenAIWSIngressLease(context.Background(), 1, 1)
		require.NoError(t, err)
		require.False(t, acquired)
		require.Nil(t, lease)
	})

	t.Run("release returns capacity", func(t *testing.T) {
		cache := &ingressLeaseCacheForTest{acquireIngressResult: true, refreshIngressResult: true}
		lease, acquired, err := NewConcurrencyService(cache).AcquireOpenAIWSIngressLease(nil, 1, 1)
		require.NoError(t, err)
		require.True(t, acquired)
		require.NotNil(t, lease)
		lease.Release()
		lease.Release()
		require.Equal(t, 1, cache.releaseIngressCalls)
	})
}

func TestOpenAIWSIngressLeaseRefreshLoss(t *testing.T) {
	t.Run("missing lease is lost immediately", func(t *testing.T) {
		cache := &ingressLeaseCacheForTest{refreshIngressResult: false}
		lease := &OpenAIWSIngressLease{cache: cache, apiKeyID: 1, leaseID: "missing"}
		_, lost := lease.refresh(time.Now())
		require.True(t, lost)
		require.Equal(t, 1, cache.refreshIngressCalls)
	})

	t.Run("persistent redis errors lose lease after ttl", func(t *testing.T) {
		cache := &ingressLeaseCacheForTest{refreshIngressErr: errors.New("redis unavailable")}
		lease := &OpenAIWSIngressLease{cache: cache, apiKeyID: 1, leaseID: "unconfirmed"}
		_, lost := lease.refresh(time.Now().Add(-openAIWSIngressLeaseTTL))
		require.True(t, lost)
		require.Equal(t, 1, cache.refreshIngressCalls)
	})
}

func TestOpenAIWSIngressLeaseReleaseWaitsForInFlightRefresh(t *testing.T) {
	refreshStarted := make(chan struct{})
	allowRefresh := make(chan struct{})
	cache := &ingressLeaseCacheForTest{
		refreshIngressFn: func(context.Context, int64, string) (bool, error) {
			close(refreshStarted)
			<-allowRefresh
			return true, nil
		},
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	lease := &OpenAIWSIngressLease{
		ctx:         ctx,
		cancel:      cancel,
		cache:       cache,
		apiKeyID:    1,
		leaseID:     "in-flight-refresh",
		stopCh:      make(chan struct{}),
		refreshDone: make(chan struct{}),
	}
	go func() {
		defer close(lease.refreshDone)
		_, _ = lease.refresh(time.Now())
	}()
	<-refreshStarted

	released := make(chan struct{})
	go func() {
		lease.Release()
		close(released)
	}()

	select {
	case <-released:
		t.Fatal("release returned before the in-flight refresh completed")
	case <-time.After(20 * time.Millisecond):
	}
	require.Zero(t, cache.releaseIngressCalls)

	close(allowRefresh)
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("release did not complete after the refresh returned")
	}
	require.Equal(t, 1, cache.releaseIngressCalls)
}

func TestGenerateRequestID_UsesStablePrefixAndMonotonicCounter(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()
	require.NotEmpty(t, id1)
	require.NotEmpty(t, id2)

	p1 := strings.Split(id1, "-")
	p2 := strings.Split(id2, "-")
	require.Len(t, p1, 2)
	require.Len(t, p2, 2)
	require.Equal(t, p1[0], p2[0], "同一进程前缀应保持一致")

	n1, err := strconv.ParseUint(p1[1], 36, 64)
	require.NoError(t, err)
	n2, err := strconv.ParseUint(p2[1], 36, 64)
	require.NoError(t, err)
	require.Equal(t, n1+1, n2, "计数器应单调递增")
}

func TestGetAccountsLoadBatch_ReturnsCorrectData(t *testing.T) {
	expected := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, CurrentConcurrency: 3, WaitingCount: 0, LoadRate: 60},
		2: {AccountID: 2, CurrentConcurrency: 5, WaitingCount: 2, LoadRate: 100},
	}
	cache := &stubConcurrencyCacheForTest{loadBatch: expected}
	svc := NewConcurrencyService(cache)

	accounts := []AccountWithConcurrency{
		{ID: 1, MaxConcurrency: 5},
		{ID: 2, MaxConcurrency: 5},
	}
	result, err := svc.GetAccountsLoadBatch(context.Background(), accounts)
	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestGetAccountsLoadBatch_NilCache(t *testing.T) {
	svc := &ConcurrencyService{cache: nil}

	result, err := svc.GetAccountsLoadBatch(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetAccountsLoadBatch_UsesShortTTLCache(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{
		loadBatch: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, CurrentConcurrency: 1, LoadRate: 20},
		},
	}
	svc := NewConcurrencyService(cache)
	svc.SetAccountLoadBatchCacheTTL(time.Second)

	accounts := []AccountWithConcurrency{{ID: 1, MaxConcurrency: 5}}
	first, err := svc.GetAccountsLoadBatch(context.Background(), accounts)
	require.NoError(t, err)
	require.Equal(t, 1, first[int64(1)].CurrentConcurrency)

	cache.loadBatch[1] = &AccountLoadInfo{AccountID: 1, CurrentConcurrency: 4, LoadRate: 80}
	second, err := svc.GetAccountsLoadBatch(context.Background(), accounts)
	require.NoError(t, err)
	require.Equal(t, 1, second[int64(1)].CurrentConcurrency)
	require.Equal(t, int64(1), cache.loadBatchCalls.Load())
}

func TestGetAccountsLoadBatchFresh_BypassesShortTTLCache(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{
		loadBatch: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, CurrentConcurrency: 1, LoadRate: 20},
		},
	}
	svc := NewConcurrencyService(cache)
	svc.SetAccountLoadBatchCacheTTL(time.Second)

	accounts := []AccountWithConcurrency{{ID: 1, MaxConcurrency: 5}}
	_, err := svc.GetAccountsLoadBatch(context.Background(), accounts)
	require.NoError(t, err)

	cache.loadBatch[1] = &AccountLoadInfo{AccountID: 1, CurrentConcurrency: 4, LoadRate: 80}
	fresh, err := svc.GetAccountsLoadBatchFresh(context.Background(), accounts)
	require.NoError(t, err)
	require.Equal(t, 4, fresh[int64(1)].CurrentConcurrency)
	require.Equal(t, int64(2), cache.loadBatchCalls.Load())
}

func TestIncrementWaitCount_Success(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{waitAllowed: true}
	svc := NewConcurrencyService(cache)

	allowed, err := svc.IncrementWaitCount(context.Background(), 1, 25)
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestIncrementWaitCount_QueueFull(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{waitAllowed: false}
	svc := NewConcurrencyService(cache)

	allowed, err := svc.IncrementWaitCount(context.Background(), 1, 25)
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestIncrementWaitCount_FailOpen(t *testing.T) {
	// Redis 错误时应 fail-open（允许请求通过）
	cache := &stubConcurrencyCacheForTest{waitErr: errors.New("redis timeout")}
	svc := NewConcurrencyService(cache)

	allowed, err := svc.IncrementWaitCount(context.Background(), 1, 25)
	require.NoError(t, err, "Redis 错误不应传播")
	require.True(t, allowed, "Redis 错误时应 fail-open")
}

func TestIncrementWaitCount_NilCache(t *testing.T) {
	svc := &ConcurrencyService{cache: nil}

	allowed, err := svc.IncrementWaitCount(context.Background(), 1, 25)
	require.NoError(t, err)
	require.True(t, allowed, "nil cache 应 fail-open")
}

func TestCalculateMaxWait(t *testing.T) {
	tests := []struct {
		concurrency int
		expected    int
	}{
		{5, 25},  // 5 + 20
		{1, 21},  // 1 + 20
		{0, 21},  // min(1) + 20
		{-1, 21}, // min(1) + 20
		{10, 30}, // 10 + 20
	}
	for _, tt := range tests {
		result := CalculateMaxWait(tt.concurrency)
		require.Equal(t, tt.expected, result, "CalculateMaxWait(%d)", tt.concurrency)
	}
}

func TestGetAccountWaitingCount(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{waitCount: 5}
	svc := NewConcurrencyService(cache)

	count, err := svc.GetAccountWaitingCount(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 5, count)
}

func TestGetAccountWaitingCount_NilCache(t *testing.T) {
	svc := &ConcurrencyService{cache: nil}

	count, err := svc.GetAccountWaitingCount(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestGetAccountConcurrencyBatch(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{concurrency: 3}
	svc := NewConcurrencyService(cache)

	result, err := svc.GetAccountConcurrencyBatch(context.Background(), []int64{1, 2, 3})
	require.NoError(t, err)
	require.Len(t, result, 3)
	for _, id := range []int64{1, 2, 3} {
		require.Equal(t, 3, result[id])
	}
}

func TestIncrementAccountWaitCount_FailOpen(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{waitErr: errors.New("redis error")}
	svc := NewConcurrencyService(cache)

	allowed, err := svc.IncrementAccountWaitCount(context.Background(), 1, 10)
	require.NoError(t, err, "Redis 错误不应传播")
	require.True(t, allowed, "Redis 错误时应 fail-open")
}

func TestIncrementAccountWaitCount_NilCache(t *testing.T) {
	svc := &ConcurrencyService{cache: nil}

	allowed, err := svc.IncrementAccountWaitCount(context.Background(), 1, 10)
	require.NoError(t, err)
	require.True(t, allowed)
}
