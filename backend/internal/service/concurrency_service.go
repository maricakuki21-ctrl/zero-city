package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// ConcurrencyCache 定义并发控制的缓存接口
// 使用有序集合存储槽位，按时间戳清理过期条目
type ConcurrencyCache interface {
	// 账号槽位管理
	// 键格式: concurrency:account:{accountID}（有序集合，成员为 requestID）
	AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error)
	ReleaseAccountSlot(ctx context.Context, accountID int64, requestID string) error
	GetAccountConcurrency(ctx context.Context, accountID int64) (int, error)
	GetAccountConcurrencyBatch(ctx context.Context, accountIDs []int64) (map[int64]int, error)

	// 账号等待队列（账号级）
	IncrementAccountWaitCount(ctx context.Context, accountID int64, maxWait int) (bool, error)
	DecrementAccountWaitCount(ctx context.Context, accountID int64) error
	GetAccountWaitingCount(ctx context.Context, accountID int64) (int, error)

	// 用户槽位管理
	// 键格式: concurrency:user:{userID}（有序集合，成员为 requestID）
	AcquireUserSlot(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error)
	ReleaseUserSlot(ctx context.Context, userID int64, requestID string) error
	GetUserConcurrency(ctx context.Context, userID int64) (int, error)

	// 等待队列计数（每次入队都会刷新 TTL，避免长时间排队时计数提前过期）
	IncrementWaitCount(ctx context.Context, userID int64, maxWait int) (bool, error)
	DecrementWaitCount(ctx context.Context, userID int64) error

	// 批量负载查询（只读）
	GetAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error)
	GetUsersLoadBatch(ctx context.Context, users []UserWithConcurrency) (map[int64]*UserLoadInfo, error)

	// 清理过期槽位（后台任务）
	CleanupExpiredAccountSlots(ctx context.Context, accountID int64) error
	CleanupExpiredAccountSlotKeys(ctx context.Context) error

	// 启动时清理旧进程遗留槽位与等待计数
	CleanupStaleProcessSlots(ctx context.Context, activeRequestPrefix string) error
}

// ConcurrencySlotLeaseCache refreshes an existing distributed slot without
// acquiring capacity. A missing member must return owned=false and must never
// recreate it, even when capacity is available.
type ConcurrencySlotLeaseCache interface {
	RefreshAccountSlot(ctx context.Context, accountID int64, requestID string) (owned bool, err error)
	RefreshUserSlot(ctx context.Context, userID int64, requestID string) (owned bool, err error)
}

type APIKeyConcurrencyCache interface {
	TrackAPIKeySlot(ctx context.Context, apiKeyID int64, requestID string) error
	ReleaseAPIKeySlot(ctx context.Context, apiKeyID int64, requestID string) error
	GetAPIKeyConcurrencyBatch(ctx context.Context, apiKeyIDs []int64) (map[int64]int, error)
}

// OpenAIWSIngressLeaseCache owns the short-lived distributed lease used to
// bound live client WebSocket sessions. It is deliberately independent of the
// request-slot namespace: idle ingress connections do not occupy turn slots.
type OpenAIWSIngressLeaseCache interface {
	AcquireOpenAIWSIngressLease(ctx context.Context, apiKeyID int64, maxConnections int, leaseID string) (bool, error)
	RefreshOpenAIWSIngressLease(ctx context.Context, apiKeyID int64, leaseID string) (bool, error)
	ReleaseOpenAIWSIngressLease(ctx context.Context, apiKeyID int64, leaseID string) error
}

const (
	openAIWSIngressLeaseTTL             = 60 * time.Second
	openAIWSIngressLeaseRefreshInterval = 20 * time.Second
	openAIWSIngressLeaseOperationTO     = 2 * time.Second
)

var ErrOpenAIWSIngressLeaseLost = errors.New("openai websocket ingress lease lost")

var ErrConcurrencySlotLeaseLost = errors.New("concurrency slot lease lost")

type concurrencySlotLeaseCancelContextKey struct{}

// WithConcurrencySlotLeaseCancellation installs the request-scoped cancel
// owner used by account/user slot heartbeats. All slots acquired for the same
// request should share this context so losing any distributed slot terminates
// the actual upstream request.
func WithConcurrencySlotLeaseCancellation(ctx context.Context) (context.Context, context.CancelCauseFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	leaseCtx, cancel := context.WithCancelCause(ctx)
	return context.WithValue(leaseCtx, concurrencySlotLeaseCancelContextKey{}, cancel), cancel
}

func concurrencySlotLeaseCancelFromContext(ctx context.Context) (context.CancelCauseFunc, bool) {
	if ctx == nil {
		return nil, false
	}
	cancel, ok := ctx.Value(concurrencySlotLeaseCancelContextKey{}).(context.CancelCauseFunc)
	return cancel, ok && cancel != nil
}

// OpenAIWSIngressLease keeps a Redis-backed ingress lease alive and cancels
// its context if Redis cannot confirm ownership for a full lease lifetime.
// Call Release on every handler exit to reclaim capacity immediately.
type OpenAIWSIngressLease struct {
	ctx      context.Context
	cancel   context.CancelCauseFunc
	cache    OpenAIWSIngressLeaseCache
	apiKeyID int64
	leaseID  string

	stopOnce    sync.Once
	stopCh      chan struct{}
	refreshDone chan struct{}
}

func (l *OpenAIWSIngressLease) Context() context.Context {
	if l == nil || l.ctx == nil {
		return context.Background()
	}
	return l.ctx
}

func (l *OpenAIWSIngressLease) Release() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() {
		if l.stopCh != nil {
			close(l.stopCh)
		}
		if l.cancel != nil {
			l.cancel(nil)
		}
		if l.refreshDone != nil {
			<-l.refreshDone
		}
		if l.cache == nil || l.apiKeyID <= 0 || l.leaseID == "" {
			return
		}
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), openAIWSIngressLeaseOperationTO)
		defer releaseCancel()
		if err := l.cache.ReleaseOpenAIWSIngressLease(releaseCtx, l.apiKeyID, l.leaseID); err != nil {
			logger.L().Warn("openai_ws_ingress_lease_release_failed",
				zap.Int64("api_key_id", l.apiKeyID),
				zap.Error(err),
			)
		}
	})
}

func (l *OpenAIWSIngressLease) refreshLoop() {
	defer func() {
		if l != nil && l.refreshDone != nil {
			close(l.refreshDone)
		}
	}()
	if l == nil || l.cache == nil {
		return
	}
	ticker := time.NewTicker(openAIWSIngressLeaseRefreshInterval)
	defer ticker.Stop()
	lastConfirmedAt := time.Now()
	for {
		select {
		case <-l.ctx.Done():
			return
		case <-l.stopCh:
			return
		case <-ticker.C:
			var lost bool
			lastConfirmedAt, lost = l.refresh(lastConfirmedAt)
			if lost {
				l.cancel(ErrOpenAIWSIngressLeaseLost)
				return
			}
		}
	}
}

// refresh confirms the lease is still owned. A missing member is an immediate
// lease loss; transient Redis errors are tolerated only for one full lease TTL.
func (l *OpenAIWSIngressLease) refresh(lastConfirmedAt time.Time) (time.Time, bool) {
	refreshCtx, refreshCancel := context.WithTimeout(context.Background(), openAIWSIngressLeaseOperationTO)
	owned, err := l.cache.RefreshOpenAIWSIngressLease(refreshCtx, l.apiKeyID, l.leaseID)
	refreshCancel()
	if err == nil && owned {
		return time.Now(), false
	}
	if err == nil {
		err = ErrOpenAIWSIngressLeaseLost
	}
	elapsed := time.Since(lastConfirmedAt)
	logger.L().Warn("openai_ws_ingress_lease_refresh_failed",
		zap.Int64("api_key_id", l.apiKeyID),
		zap.Duration("unconfirmed_for", elapsed),
		zap.Error(err),
	)
	if errors.Is(err, ErrOpenAIWSIngressLeaseLost) || elapsed >= openAIWSIngressLeaseTTL {
		logger.L().Error("openai_ws_ingress_lease_lost",
			zap.Int64("api_key_id", l.apiKeyID),
			zap.Duration("unconfirmed_for", elapsed),
			zap.Error(err),
		)
		return lastConfirmedAt, true
	}
	return lastConfirmedAt, false
}

var (
	requestIDPrefix  = initRequestIDPrefix()
	requestIDCounter atomic.Uint64
)

func initRequestIDPrefix() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		return "r" + strconv.FormatUint(binary.BigEndian.Uint64(b), 36)
	}
	fallback := uint64(time.Now().UnixNano()) ^ (uint64(os.Getpid()) << 16)
	return "r" + strconv.FormatUint(fallback, 36)
}

func RequestIDPrefix() string {
	return requestIDPrefix
}

func generateRequestID() string {
	seq := requestIDCounter.Add(1)
	return requestIDPrefix + "-" + strconv.FormatUint(seq, 36)
}

// CleanupStaleProcessSlots is destructive across process prefixes and is safe
// only during an explicitly verified single-runtime maintenance window. Normal
// application startup must rely on score/TTL cleanup instead.
func (s *ConcurrencyService) CleanupStaleProcessSlots(ctx context.Context) error {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.CleanupStaleProcessSlots(ctx, RequestIDPrefix())
}

const (
	// 默认等待队列额外槽位
	defaultExtraWaitSlots = 20

	defaultAccountLoadBatchCacheTTL = 200 * time.Millisecond
	accountLoadBatchFetchTimeout    = 3 * time.Second
	maxAccountLoadBatchCacheEntries = 256
	apiKeyConcurrencyFetchTimeout   = 3 * time.Second
	apiKeySlotTrackTimeout          = 2 * time.Second
	concurrencySlotOperationTimeout = 5 * time.Second
)

// ConcurrencyService 管理账号和用户的并发限制。
type ConcurrencyService struct {
	cache ConcurrencyCache

	slotHeartbeatInterval atomic.Int64
	slotLeaseTTL          atomic.Int64
	accountLoadCacheTTL   atomic.Int64
	accountLoadCacheMu    sync.RWMutex
	accountLoadCache      map[string]cachedAccountLoadBatch
	accountLoadGroup      singleflight.Group
}

type cachedAccountLoadBatch struct {
	loadMap   map[int64]*AccountLoadInfo
	expiresAt time.Time
}

// NewConcurrencyService 创建并发控制服务。
func NewConcurrencyService(cache ConcurrencyCache) *ConcurrencyService {
	svc := &ConcurrencyService{
		cache:            cache,
		accountLoadCache: make(map[string]cachedAccountLoadBatch),
	}
	svc.SetAccountLoadBatchCacheTTL(defaultAccountLoadBatchCacheTTL)
	return svc
}

// SetSlotHeartbeatInterval configures how often an acquired account/user slot
// is refreshed. A non-positive duration disables heartbeats; production wiring
// derives this from the configured slot TTL.
func (s *ConcurrencyService) SetSlotHeartbeatInterval(interval time.Duration) {
	if s == nil {
		return
	}
	s.slotHeartbeatInterval.Store(int64(interval))
	if interval > 0 {
		s.slotLeaseTTL.Store(int64(3 * interval))
	} else {
		s.slotLeaseTTL.Store(0)
	}
}

// SetSlotLeaseTTL sets how long transient Redis refresh errors may leave slot
// ownership unconfirmed. A confirmed missing member is always lost immediately.
func (s *ConcurrencyService) SetSlotLeaseTTL(ttl time.Duration) {
	if s == nil {
		return
	}
	s.slotLeaseTTL.Store(int64(ttl))
}

type concurrencySlotHeartbeat struct {
	cancel  context.CancelFunc
	state   atomic.Int32
	workers sync.WaitGroup

	deadlineMu      sync.RWMutex
	confirmedAt     time.Time
	deadline        time.Time
	deadlineChanged chan struct{}
}

const (
	concurrencySlotHeartbeatActive int32 = iota
	concurrencySlotHeartbeatStopping
	concurrencySlotHeartbeatLost
)

func (h *concurrencySlotHeartbeat) stopAndWait() {
	if h == nil {
		return
	}
	// Whichever side wins this transition owns the outcome. If a normal
	// release starts first, neither the refresh worker nor the watchdog may
	// subsequently cancel the request as a lost lease.
	h.state.CompareAndSwap(concurrencySlotHeartbeatActive, concurrencySlotHeartbeatStopping)
	h.cancel()
	h.workers.Wait()
}

func (h *concurrencySlotHeartbeat) markLost() bool {
	return h != nil && h.state.CompareAndSwap(concurrencySlotHeartbeatActive, concurrencySlotHeartbeatLost)
}

func (h *concurrencySlotHeartbeat) leaseWindow() (time.Time, time.Time) {
	if h == nil {
		return time.Time{}, time.Time{}
	}
	h.deadlineMu.RLock()
	defer h.deadlineMu.RUnlock()
	return h.confirmedAt, h.deadline
}

func (h *concurrencySlotHeartbeat) confirmAt(startedAt time.Time, leaseTTL time.Duration) {
	if h == nil {
		return
	}
	h.deadlineMu.Lock()
	h.confirmedAt = startedAt
	h.deadline = startedAt.Add(leaseTTL)
	h.deadlineMu.Unlock()
	select {
	case h.deadlineChanged <- struct{}{}:
	default:
	}
}

func (s *ConcurrencyService) startSlotHeartbeat(
	ctx context.Context,
	slotKind string,
	slotID int64,
	requestID string,
	acquireStartedAt time.Time,
	refresh func(context.Context) (bool, error),
) *concurrencySlotHeartbeat {
	interval := time.Duration(s.slotHeartbeatInterval.Load())
	if interval <= 0 || refresh == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	leaseTTL := time.Duration(s.slotLeaseTTL.Load())
	if acquireStartedAt.IsZero() {
		acquireStartedAt = time.Now()
	}
	heartbeat := &concurrencySlotHeartbeat{
		cancel:          cancel,
		confirmedAt:     acquireStartedAt,
		deadline:        acquireStartedAt.Add(leaseTTL),
		deadlineChanged: make(chan struct{}, 1),
	}
	leaseCancel, _ := concurrencySlotLeaseCancelFromContext(ctx)
	heartbeat.workers.Add(2)
	go func() {
		defer heartbeat.workers.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				refreshStartedAt := time.Now()
				_, leaseDeadline := heartbeat.leaseWindow()
				if !refreshStartedAt.Before(leaseDeadline) {
					if heartbeat.markLost() {
						logger.LegacyPrintf("service.concurrency", "Error: lost %s slot lease for %d (req=%s, deadline exceeded before refresh)", slotKind, slotID, requestID)
						leaseCancel(ErrConcurrencySlotLeaseLost)
					}
					return
				}

				// Never let a refresh call outlive the last lease we can prove.
				// The watchdog below remains independent and enforces the same
				// absolute deadline even if a cache implementation ignores ctx.
				refreshDeadline := leaseDeadline
				if operationDeadline := refreshStartedAt.Add(concurrencySlotOperationTimeout); operationDeadline.Before(refreshDeadline) {
					refreshDeadline = operationDeadline
				}
				refreshCtx, refreshCancel := context.WithDeadline(heartbeatCtx, refreshDeadline)
				refreshed, err := refresh(refreshCtx)
				refreshCancel()
				if heartbeatCtx.Err() != nil || heartbeat.state.Load() != concurrencySlotHeartbeatActive {
					return
				}
				if err == nil && refreshed {
					// Redis may have executed at any point after the call began, so
					// only the pre-call time is a safe basis for the next deadline.
					heartbeat.confirmAt(refreshStartedAt, leaseTTL)
					continue
				}

				lastConfirmedAt, currentDeadline := heartbeat.leaseWindow()
				unconfirmedFor := time.Since(lastConfirmedAt)
				if err != nil {
					logger.LegacyPrintf("service.concurrency", "Warning: failed to refresh %s slot for %d (req=%s, unconfirmed=%s): %v", slotKind, slotID, requestID, unconfirmedFor, err)
					if time.Now().Before(currentDeadline) {
						continue
					}
				}

				if heartbeat.markLost() {
					logger.LegacyPrintf("service.concurrency", "Error: lost %s slot lease for %d (req=%s, unconfirmed=%s)", slotKind, slotID, requestID, unconfirmedFor)
					leaseCancel(ErrConcurrencySlotLeaseLost)
				}
				return
			}
		}
	}()
	go func() {
		defer heartbeat.workers.Done()
		_, deadline := heartbeat.leaseWindow()
		delay := time.Until(deadline)
		if delay < 0 {
			delay = 0
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()

		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-heartbeat.deadlineChanged:
				_, deadline = heartbeat.leaseWindow()
				delay = time.Until(deadline)
				if delay <= 0 {
					if heartbeat.markLost() {
						logger.LegacyPrintf("service.concurrency", "Error: lost %s slot lease for %d (req=%s, watchdog deadline exceeded)", slotKind, slotID, requestID)
						leaseCancel(ErrConcurrencySlotLeaseLost)
					}
					return
				}
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(delay)
			case <-timer.C:
				_, deadline = heartbeat.leaseWindow()
				if delay = time.Until(deadline); delay > 0 {
					timer.Reset(delay)
					continue
				}
				if heartbeat.markLost() {
					logger.LegacyPrintf("service.concurrency", "Error: lost %s slot lease for %d (req=%s, watchdog deadline exceeded)", slotKind, slotID, requestID)
					leaseCancel(ErrConcurrencySlotLeaseLost)
				}
				return
			}
		}
	}()
	return heartbeat
}

func (s *ConcurrencyService) slotLeaseCache(leaseCtx context.Context) (ConcurrencySlotLeaseCache, error) {
	if time.Duration(s.slotHeartbeatInterval.Load()) <= 0 {
		return nil, nil
	}
	cache, ok := s.cache.(ConcurrencySlotLeaseCache)
	if !ok {
		return nil, errors.New("concurrency slot refresh-only cache is unavailable")
	}
	if _, ok := concurrencySlotLeaseCancelFromContext(leaseCtx); !ok {
		return nil, errors.New("concurrency slot request cancellation context is unavailable")
	}
	return cache, nil
}

func (s *ConcurrencyService) newSlotReleaseFunc(
	ctx context.Context,
	slotKind string,
	slotID int64,
	requestID string,
	acquireStartedAt time.Time,
	refresh func(context.Context) (bool, error),
	release func(context.Context) error,
) func() {
	heartbeat := s.startSlotHeartbeat(ctx, slotKind, slotID, requestID, acquireStartedAt, refresh)
	var releaseOnce sync.Once
	return func() {
		releaseOnce.Do(func() {
			// Stop and join the heartbeat before deleting the member. Otherwise an
			// in-flight refresh can recreate the slot immediately after release.
			heartbeat.stopAndWait()
			releaseCtx, cancel := context.WithTimeout(context.Background(), concurrencySlotOperationTimeout)
			defer cancel()
			if err := release(releaseCtx); err != nil {
				logger.LegacyPrintf("service.concurrency", "Warning: failed to release %s slot for %d (req=%s): %v", slotKind, slotID, requestID, err)
			}
		})
	}
}

// AcquireOpenAIWSIngressLease atomically reserves one live ingress connection
// for an API key. A non-positive limit explicitly disables this protection.
func (s *ConcurrencyService) AcquireOpenAIWSIngressLease(ctx context.Context, apiKeyID int64, maxConnections int) (*OpenAIWSIngressLease, bool, error) {
	if maxConnections <= 0 {
		return nil, true, nil
	}
	if s == nil || s.cache == nil || apiKeyID <= 0 {
		return nil, false, errors.New("openai websocket ingress lease cache is unavailable")
	}
	cache, ok := s.cache.(OpenAIWSIngressLeaseCache)
	if !ok {
		return nil, false, errors.New("openai websocket ingress lease cache is unsupported")
	}
	leaseID := generateRequestID()
	baseCtx := context.Background()
	if ctx != nil {
		baseCtx = context.WithoutCancel(ctx)
	}
	acquireCtx, acquireCancel := context.WithTimeout(baseCtx, openAIWSIngressLeaseOperationTO)
	acquired, err := cache.AcquireOpenAIWSIngressLease(acquireCtx, apiKeyID, maxConnections, leaseID)
	acquireCancel()
	if err != nil || !acquired {
		return nil, acquired, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	leaseCtx, leaseCancel := context.WithCancelCause(ctx)
	lease := &OpenAIWSIngressLease{
		ctx:         leaseCtx,
		cancel:      leaseCancel,
		cache:       cache,
		apiKeyID:    apiKeyID,
		leaseID:     leaseID,
		stopCh:      make(chan struct{}),
		refreshDone: make(chan struct{}),
	}
	go lease.refreshLoop()
	return lease, true, nil
}

// SetAccountLoadBatchCacheTTL 设置账号负载批量读取的极短 TTL 缓存；非正数表示禁用缓存。
func (s *ConcurrencyService) SetAccountLoadBatchCacheTTL(ttl time.Duration) {
	if s == nil {
		return
	}
	s.accountLoadCacheTTL.Store(int64(ttl))
	if ttl <= 0 {
		s.accountLoadCacheMu.Lock()
		s.accountLoadCache = make(map[string]cachedAccountLoadBatch)
		s.accountLoadCacheMu.Unlock()
	}
}

// AcquireResult represents the result of acquiring a concurrency slot
type AcquireResult struct {
	Acquired    bool
	ReleaseFunc func() // Must be called when done (typically via defer)
}

type AccountWithConcurrency struct {
	ID             int64
	MaxConcurrency int
}

type UserWithConcurrency struct {
	ID             int64
	MaxConcurrency int
}

type AccountLoadInfo struct {
	AccountID          int64
	CurrentConcurrency int
	WaitingCount       int
	LoadRate           int // 0-100+ (percent)
}

type UserLoadInfo struct {
	UserID             int64
	CurrentConcurrency int
	WaitingCount       int
	LoadRate           int // 0-100+ (percent)
}

// AcquireAccountSlot attempts to acquire a concurrency slot for an account.
// If the account is at max concurrency, it waits until a slot is available or timeout.
// Returns a release function that MUST be called when the request completes.
func (s *ConcurrencyService) AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int) (*AcquireResult, error) {
	return s.acquireAccountSlot(ctx, ctx, accountID, maxConcurrency)
}

// AcquireAccountSlotWithLeaseContext separates the bounded Redis acquisition
// context from the context that owns the acquired slot. This is required by
// wait loops: their timeout context is canceled as soon as acquisition returns,
// while the slot heartbeat must live for the entire request.
func (s *ConcurrencyService) AcquireAccountSlotWithLeaseContext(acquireCtx, leaseCtx context.Context, accountID int64, maxConcurrency int) (*AcquireResult, error) {
	return s.acquireAccountSlot(acquireCtx, leaseCtx, accountID, maxConcurrency)
}

func (s *ConcurrencyService) acquireAccountSlot(acquireCtx, leaseCtx context.Context, accountID int64, maxConcurrency int) (*AcquireResult, error) {
	// If maxConcurrency is 0 or negative, no limit
	if maxConcurrency <= 0 {
		return &AcquireResult{
			Acquired:    true,
			ReleaseFunc: func() {}, // no-op
		}, nil
	}
	leaseCache, err := s.slotLeaseCache(leaseCtx)
	if err != nil {
		return nil, err
	}

	// Generate unique request ID for this slot
	requestID := generateRequestID()

	acquireStartedAt := time.Now()
	acquired, err := s.cache.AcquireAccountSlot(acquireCtx, accountID, maxConcurrency, requestID)
	if err != nil {
		return nil, err
	}

	if acquired {
		return &AcquireResult{
			Acquired: true,
			ReleaseFunc: s.newSlotReleaseFunc(
				leaseCtx,
				"account",
				accountID,
				requestID,
				acquireStartedAt,
				func(refreshCtx context.Context) (bool, error) {
					if leaseCache == nil {
						return true, nil
					}
					return leaseCache.RefreshAccountSlot(refreshCtx, accountID, requestID)
				},
				func(releaseCtx context.Context) error {
					return s.cache.ReleaseAccountSlot(releaseCtx, accountID, requestID)
				},
			),
		}, nil
	}

	return &AcquireResult{
		Acquired:    false,
		ReleaseFunc: nil,
	}, nil
}

// AcquireUserSlot attempts to acquire a concurrency slot for a user.
// If the user is at max concurrency, it waits until a slot is available or timeout.
// Returns a release function that MUST be called when the request completes.
func (s *ConcurrencyService) AcquireUserSlot(ctx context.Context, userID int64, maxConcurrency int) (*AcquireResult, error) {
	return s.acquireUserSlot(ctx, ctx, userID, maxConcurrency)
}

// AcquireUserSlotWithLeaseContext is the user-slot counterpart of
// AcquireAccountSlotWithLeaseContext.
func (s *ConcurrencyService) AcquireUserSlotWithLeaseContext(acquireCtx, leaseCtx context.Context, userID int64, maxConcurrency int) (*AcquireResult, error) {
	return s.acquireUserSlot(acquireCtx, leaseCtx, userID, maxConcurrency)
}

func (s *ConcurrencyService) acquireUserSlot(acquireCtx, leaseCtx context.Context, userID int64, maxConcurrency int) (*AcquireResult, error) {
	// If maxConcurrency is 0 or negative, no limit
	if maxConcurrency <= 0 {
		return &AcquireResult{
			Acquired:    true,
			ReleaseFunc: func() {}, // no-op
		}, nil
	}
	leaseCache, err := s.slotLeaseCache(leaseCtx)
	if err != nil {
		return nil, err
	}

	// Generate unique request ID for this slot
	requestID := generateRequestID()

	acquireStartedAt := time.Now()
	acquired, err := s.cache.AcquireUserSlot(acquireCtx, userID, maxConcurrency, requestID)
	if err != nil {
		return nil, err
	}

	if acquired {
		return &AcquireResult{
			Acquired: true,
			ReleaseFunc: s.newSlotReleaseFunc(
				leaseCtx,
				"user",
				userID,
				requestID,
				acquireStartedAt,
				func(refreshCtx context.Context) (bool, error) {
					if leaseCache == nil {
						return true, nil
					}
					return leaseCache.RefreshUserSlot(refreshCtx, userID, requestID)
				},
				func(releaseCtx context.Context) error {
					return s.cache.ReleaseUserSlot(releaseCtx, userID, requestID)
				},
			),
		}, nil
	}

	return &AcquireResult{
		Acquired:    false,
		ReleaseFunc: nil,
	}, nil
}

// TrackAPIKeySlot records one active request slot for an API key without
// applying key-level concurrency limits. It is fail-open: Redis errors are
// logged and return a no-op release function.
func (s *ConcurrencyService) TrackAPIKeySlot(ctx context.Context, apiKeyID int64) func() {
	if s == nil || s.cache == nil || apiKeyID <= 0 {
		return func() {}
	}
	cache, ok := s.cache.(APIKeyConcurrencyCache)
	if !ok {
		return func() {}
	}

	requestID := generateRequestID()
	baseCtx := context.Background()
	if ctx != nil {
		baseCtx = context.WithoutCancel(ctx)
	}
	trackCtx, cancel := context.WithTimeout(baseCtx, apiKeySlotTrackTimeout)
	err := cache.TrackAPIKeySlot(trackCtx, apiKeyID, requestID)
	cancel()
	if err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: failed to track api key slot for %d (req=%s): %v", apiKeyID, requestID, err)
		return func() {}
	}

	return func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cache.ReleaseAPIKeySlot(bgCtx, apiKeyID, requestID); err != nil {
			logger.LegacyPrintf("service.concurrency", "Warning: failed to release api key slot for %d (req=%s): %v", apiKeyID, requestID, err)
		}
	}
}

// GetAPIKeyConcurrencyBatch gets real-time active request counts for API keys.
// Stats are best-effort: missing Redis support or Redis errors return zeroes.
func (s *ConcurrencyService) GetAPIKeyConcurrencyBatch(ctx context.Context, apiKeyIDs []int64) (map[int64]int, error) {
	result := zeroAPIKeyConcurrencyMap(apiKeyIDs)
	if len(apiKeyIDs) == 0 {
		return result, nil
	}
	if s == nil || s.cache == nil {
		return result, nil
	}
	cache, ok := s.cache.(APIKeyConcurrencyCache)
	if !ok {
		return result, nil
	}

	redisCtx, cancel := context.WithTimeout(context.Background(), apiKeyConcurrencyFetchTimeout)
	defer cancel()

	counts, err := cache.GetAPIKeyConcurrencyBatch(redisCtx, apiKeyIDs)
	if err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: get api key concurrency batch failed: %v", err)
		return result, nil
	}
	for _, apiKeyID := range apiKeyIDs {
		result[apiKeyID] = counts[apiKeyID]
	}
	return result, nil
}

func zeroAPIKeyConcurrencyMap(apiKeyIDs []int64) map[int64]int {
	result := make(map[int64]int, len(apiKeyIDs))
	for _, apiKeyID := range apiKeyIDs {
		result[apiKeyID] = 0
	}
	return result
}

// ============================================
// Wait Queue Count Methods
// ============================================

// IncrementWaitCount attempts to increment the wait queue counter for a user.
// Returns true if successful, false if the wait queue is full.
// maxWait should be user.Concurrency + defaultExtraWaitSlots
func (s *ConcurrencyService) IncrementWaitCount(ctx context.Context, userID int64, maxWait int) (bool, error) {
	if s.cache == nil {
		// Redis not available, allow request
		return true, nil
	}

	result, err := s.cache.IncrementWaitCount(ctx, userID, maxWait)
	if err != nil {
		// On error, allow the request to proceed (fail open)
		logger.LegacyPrintf("service.concurrency", "Warning: increment wait count failed for user %d: %v", userID, err)
		return true, nil
	}
	return result, nil
}

// DecrementWaitCount decrements the wait queue counter for a user.
// Should be called when a request completes or exits the wait queue.
func (s *ConcurrencyService) DecrementWaitCount(ctx context.Context, userID int64) {
	if s.cache == nil {
		return
	}

	// Use background context to ensure decrement even if original context is cancelled
	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.cache.DecrementWaitCount(bgCtx, userID); err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: decrement wait count failed for user %d: %v", userID, err)
	}
}

// IncrementAccountWaitCount increments the wait queue counter for an account.
func (s *ConcurrencyService) IncrementAccountWaitCount(ctx context.Context, accountID int64, maxWait int) (bool, error) {
	if s.cache == nil {
		return true, nil
	}

	result, err := s.cache.IncrementAccountWaitCount(ctx, accountID, maxWait)
	if err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: increment wait count failed for account %d: %v", accountID, err)
		return true, nil
	}
	return result, nil
}

// DecrementAccountWaitCount decrements the wait queue counter for an account.
func (s *ConcurrencyService) DecrementAccountWaitCount(ctx context.Context, accountID int64) {
	if s.cache == nil {
		return
	}

	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.cache.DecrementAccountWaitCount(bgCtx, accountID); err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: decrement wait count failed for account %d: %v", accountID, err)
	}
}

// GetAccountWaitingCount gets current wait queue count for an account.
func (s *ConcurrencyService) GetAccountWaitingCount(ctx context.Context, accountID int64) (int, error) {
	if s.cache == nil {
		return 0, nil
	}
	return s.cache.GetAccountWaitingCount(ctx, accountID)
}

// CalculateMaxWait calculates the maximum wait queue size for a user
// maxWait = userConcurrency + defaultExtraWaitSlots
func CalculateMaxWait(userConcurrency int) int {
	if userConcurrency <= 0 {
		userConcurrency = 1
	}
	return userConcurrency + defaultExtraWaitSlots
}

// GetAccountsLoadBatch 批量获取账号负载信息。
func (s *ConcurrencyService) GetAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	return s.getAccountsLoadBatch(ctx, accounts, true)
}

// GetAccountsLoadBatchFresh 绕过极短 TTL 缓存，用于抢槽失败后的实时刷新兜底。
func (s *ConcurrencyService) GetAccountsLoadBatchFresh(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	return s.getAccountsLoadBatch(ctx, accounts, false)
}

func (s *ConcurrencyService) getAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency, allowCache bool) (map[int64]*AccountLoadInfo, error) {
	if len(accounts) == 0 {
		return map[int64]*AccountLoadInfo{}, nil
	}
	if s.cache == nil {
		return map[int64]*AccountLoadInfo{}, nil
	}

	ttl := time.Duration(s.accountLoadCacheTTL.Load())
	if !allowCache || ttl <= 0 {
		return s.fetchAccountsLoadBatch(ctx, accounts)
	}

	key := accountLoadBatchCacheKey(accounts)
	if cached, ok := s.getCachedAccountLoadBatch(key, time.Now()); ok {
		return cached, nil
	}

	value, err, _ := s.accountLoadGroup.Do(key, func() (any, error) {
		now := time.Now()
		if cached, ok := s.getCachedAccountLoadBatch(key, now); ok {
			return cached, nil
		}
		loadMap, fetchErr := s.fetchAccountsLoadBatch(ctx, accounts)
		if fetchErr != nil {
			return nil, fetchErr
		}
		cached := cloneAccountLoadMap(loadMap)
		s.storeCachedAccountLoadBatch(key, cached, now.Add(ttl))
		return cached, nil
	})
	if err != nil {
		return nil, err
	}
	loadMap, _ := value.(map[int64]*AccountLoadInfo)
	if loadMap == nil {
		return map[int64]*AccountLoadInfo{}, nil
	}
	return loadMap, nil
}

func (s *ConcurrencyService) fetchAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	if s.cache == nil {
		return map[int64]*AccountLoadInfo{}, nil
	}
	baseCtx := context.Background()
	if ctx != nil {
		baseCtx = context.WithoutCancel(ctx)
	}
	redisCtx, cancel := context.WithTimeout(baseCtx, accountLoadBatchFetchTimeout)
	defer cancel()
	return s.cache.GetAccountsLoadBatch(redisCtx, accounts)
}

func (s *ConcurrencyService) getCachedAccountLoadBatch(key string, now time.Time) (map[int64]*AccountLoadInfo, bool) {
	s.accountLoadCacheMu.RLock()
	cached, ok := s.accountLoadCache[key]
	s.accountLoadCacheMu.RUnlock()
	if !ok {
		return nil, false
	}
	if !now.Before(cached.expiresAt) {
		s.accountLoadCacheMu.Lock()
		if current, exists := s.accountLoadCache[key]; exists && !now.Before(current.expiresAt) {
			delete(s.accountLoadCache, key)
		}
		s.accountLoadCacheMu.Unlock()
		return nil, false
	}
	return cached.loadMap, true
}

func (s *ConcurrencyService) storeCachedAccountLoadBatch(key string, loadMap map[int64]*AccountLoadInfo, expiresAt time.Time) {
	s.accountLoadCacheMu.Lock()
	if s.accountLoadCache == nil {
		s.accountLoadCache = make(map[string]cachedAccountLoadBatch)
	}
	if len(s.accountLoadCache) >= maxAccountLoadBatchCacheEntries {
		now := time.Now()
		for cacheKey, cached := range s.accountLoadCache {
			if !now.Before(cached.expiresAt) {
				delete(s.accountLoadCache, cacheKey)
			}
		}
		for len(s.accountLoadCache) >= maxAccountLoadBatchCacheEntries {
			for cacheKey := range s.accountLoadCache {
				delete(s.accountLoadCache, cacheKey)
				break
			}
		}
	}
	s.accountLoadCache[key] = cachedAccountLoadBatch{
		loadMap:   loadMap,
		expiresAt: expiresAt,
	}
	s.accountLoadCacheMu.Unlock()
}

func accountLoadBatchCacheKey(accounts []AccountWithConcurrency) string {
	hash := sha256.New()
	var buf [16]byte
	for _, account := range accounts {
		binary.LittleEndian.PutUint64(buf[:8], uint64(account.ID))
		binary.LittleEndian.PutUint64(buf[8:], uint64(int64(account.MaxConcurrency)))
		_, _ = hash.Write(buf[:])
	}
	sum := hash.Sum(nil)
	return strconv.Itoa(len(accounts)) + ":" + hex.EncodeToString(sum)
}

func cloneAccountLoadMap(loadMap map[int64]*AccountLoadInfo) map[int64]*AccountLoadInfo {
	if len(loadMap) == 0 {
		return map[int64]*AccountLoadInfo{}
	}
	clone := make(map[int64]*AccountLoadInfo, len(loadMap))
	for accountID, loadInfo := range loadMap {
		if loadInfo == nil {
			clone[accountID] = nil
			continue
		}
		copied := *loadInfo
		clone[accountID] = &copied
	}
	return clone
}

// GetUsersLoadBatch returns load info for multiple users.
func (s *ConcurrencyService) GetUsersLoadBatch(ctx context.Context, users []UserWithConcurrency) (map[int64]*UserLoadInfo, error) {
	if s.cache == nil {
		return map[int64]*UserLoadInfo{}, nil
	}
	return s.cache.GetUsersLoadBatch(ctx, users)
}

// CleanupExpiredAccountSlots removes expired slots for one account (background task).
func (s *ConcurrencyService) CleanupExpiredAccountSlots(ctx context.Context, accountID int64) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.CleanupExpiredAccountSlots(ctx, accountID)
}

// StartSlotCleanupWorker starts a background cleanup worker for expired account slots.
func (s *ConcurrencyService) StartSlotCleanupWorker(_ AccountRepository, interval time.Duration) {
	if s == nil || s.cache == nil || interval <= 0 {
		return
	}

	runCleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := s.cache.CleanupExpiredAccountSlotKeys(cleanupCtx)
		cancel()
		if err != nil {
			logger.LegacyPrintf("service.concurrency", "Warning: cleanup expired account slots failed: %v", err)
			return
		}
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		runCleanup()
		for range ticker.C {
			runCleanup()
		}
	}()
}

// GetAccountConcurrencyBatch gets current concurrency counts for multiple accounts.
// Uses a detached context with timeout to prevent HTTP request cancellation from
// causing the entire batch to fail (which would show all concurrency as 0).
func (s *ConcurrencyService) GetAccountConcurrencyBatch(ctx context.Context, accountIDs []int64) (map[int64]int, error) {
	if len(accountIDs) == 0 {
		return map[int64]int{}, nil
	}
	if s.cache == nil {
		result := make(map[int64]int, len(accountIDs))
		for _, accountID := range accountIDs {
			result[accountID] = 0
		}
		return result, nil
	}

	// Use a detached context so that a cancelled HTTP request doesn't cause
	// the Redis pipeline to fail and return all-zero concurrency counts.
	redisCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return s.cache.GetAccountConcurrencyBatch(redisCtx, accountIDs)
}
