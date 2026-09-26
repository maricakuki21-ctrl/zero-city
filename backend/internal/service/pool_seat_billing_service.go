package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	poolSeatBillingTimeout = 60 * time.Second
	// runOnce currently has seven sequential timeout-bounded operations:
	// probe aggregation, media-gate expiry, charging, idle-seat release,
	// stability rewards, usage-reservation recovery, and media-task recovery.
	poolSeatBillingSerialStepCount  = 7
	poolSeatBillingMaxSweepDuration = poolSeatBillingSerialStepCount * poolSeatBillingTimeout

	sharedPoolProbeJobPollInterval = time.Second
)

const (
	// poolSeatBillingLeaderLockKey gates the periodic seat-billing sweep so that
	// only one instance charges seats per cycle. Charging money must never run on
	// multiple instances concurrently.
	poolSeatBillingLeaderLockKey = "pool:seat:billing:leader"
	// poolSeatBillingLeaderLockTTL must comfortably exceed the sum of every
	// serial step timeout so another instance cannot enter while a slow sweep is
	// still charging or reconciling money.
	poolSeatBillingLeaderLockTTL = 15 * time.Minute
)

type poolSeatBillingBizService interface {
	RunSharedPoolProbeAggregation(ctx context.Context, now time.Time, limit int) (*SharedPoolProbeAggregationSummary, error)
	ChargeDueSeats(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error)
	ReleaseIdleSharedPoolSeats(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error)
	GrantSharedPoolStabilityRewards(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error)
}

type sharedPoolProbeJobRunner interface {
	RunSharedPoolProbeJobs(ctx context.Context, workerID string, limit int) (int, error)
}

type canonicalSharedPoolSettler interface {
	SettlePendingCanonicalSharedPoolUsage(context.Context, int) (*CanonicalSharedPoolSettlementSummary, error)
}

type sharedPoolUsageReservationRecoverer interface {
	RecoverExpiredSharedPoolUsageReservations(ctx context.Context, now time.Time, limit int) (*SharedPoolUsageReservationRecoverySummary, error)
}

type sharedPoolMediaGateExpirer interface {
	ExpireSharedPoolMediaEndpointGates(ctx context.Context, now time.Time, limit int) (int, error)
}

type sharedPoolMediaTaskRecoverer interface {
	RecoverExpiredSharedPoolMediaTasks(ctx context.Context, now time.Time, limit int) (int, error)
}

// PoolSeatBillingService periodically charges hourly seat fees for active shared
// pool seats and credits pool owners. Idempotency is enforced at the repository
// layer (unique per seat+billing-hour), and a leader lock prevents concurrent
// instances from double-charging.
type PoolSeatBillingService struct {
	bizSvc   poolSeatBillingBizService
	interval time.Duration
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

func NewPoolSeatBillingService(bizSvc poolSeatBillingBizService, interval time.Duration) *PoolSeatBillingService {
	return &PoolSeatBillingService{
		bizSvc:     bizSvc,
		interval:   interval,
		stopCh:     make(chan struct{}),
		instanceID: uuid.NewString(),
	}
}

// SetLeaderLock injects the leader-lock cache and DB used to elect a single
// instance for the periodic billing sweep. When both are nil the job runs
// ungated (single-instance / test behavior).
func (s *PoolSeatBillingService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *PoolSeatBillingService) Start() {
	if s == nil || s.bizSvc == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()

	// Final usage must reach wallets promptly, independently of slow probes and
	// hourly seat maintenance. Ledger uniqueness makes overlapping drains safe.
	if settler, ok := s.bizSvc.(canonicalSharedPoolSettler); ok {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			defer close(done)
			go func() {
				select {
				case <-s.stopCh:
					cancel()
				case <-done:
				}
			}()
			for {
				s.runCanonicalSettlementOnce(ctx, settler)
				select {
				case <-ticker.C:
				case <-s.stopCh:
					return
				}
			}
		}()
	}

	// Probe jobs use their own durable DB lease, so every instance may poll;
	// only the claimant performs the expensive upstream work. This loop is
	// independent from the five-minute money/billing leader lock.
	if runner, ok := s.bizSvc.(sharedPoolProbeJobRunner); ok {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(sharedPoolProbeJobPollInterval)
			defer ticker.Stop()
			s.runProbeJobsOnce(runner)
			for {
				select {
				case <-ticker.C:
					s.runProbeJobsOnce(runner)
				case <-s.stopCh:
					return
				}
			}
		}()
	}
}

func (s *PoolSeatBillingService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *PoolSeatBillingService) runOnce() {
	// Multi-instance guard: only the leader charges seats per cycle.
	lockCtx, lockCancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, poolSeatBillingLeaderLockKey, s.instanceID, poolSeatBillingLeaderLockTTL)
	lockCancel()
	if !ok {
		return
	}
	defer release()

	now := time.Now()

	probeCtx, probeCancel := context.WithTimeout(context.Background(), poolSeatBillingTimeout)
	probeSummary, probeErr := s.bizSvc.RunSharedPoolProbeAggregation(probeCtx, now, 50)
	probeCancel()
	if probeErr != nil {
		slog.Error("[PoolSeatBilling] failed to probe shared pools", "error", probeErr)
	}

	expiredMediaGates := 0
	var mediaGateErr error
	if expirer, ok := s.bizSvc.(sharedPoolMediaGateExpirer); ok {
		mediaGateCtx, mediaGateCancel := context.WithTimeout(context.Background(), poolSeatBillingTimeout)
		expiredMediaGates, mediaGateErr = expirer.ExpireSharedPoolMediaEndpointGates(mediaGateCtx, now.UTC(), 100)
		mediaGateCancel()
		if mediaGateErr != nil {
			slog.Error("[PoolSeatBilling] failed to expire shared pool media gates", "error", mediaGateErr)
		}
	}

	chargeCtx, chargeCancel := context.WithTimeout(context.Background(), poolSeatBillingTimeout)
	summary, chargeErr := s.bizSvc.ChargeDueSeats(chargeCtx, now)
	chargeCancel()
	if chargeErr != nil {
		slog.Error("[PoolSeatBilling] failed to charge due seats", "error", chargeErr)
	}

	idleCtx, idleCancel := context.WithTimeout(context.Background(), poolSeatBillingTimeout)
	idleSummary, idleErr := s.bizSvc.ReleaseIdleSharedPoolSeats(idleCtx, now)
	idleCancel()
	if idleErr != nil {
		slog.Error("[PoolSeatBilling] failed to release idle seats", "error", idleErr)
	}

	rewardCtx, rewardCancel := context.WithTimeout(context.Background(), poolSeatBillingTimeout)
	rewardSummary, rewardErr := s.bizSvc.GrantSharedPoolStabilityRewards(rewardCtx, now)
	rewardCancel()
	if rewardErr != nil {
		slog.Error("[PoolSeatBilling] failed to grant shared pool stability rewards", "error", rewardErr)
	}

	var reservationSummary *SharedPoolUsageReservationRecoverySummary
	var reservationErr error
	if recoverer, ok := s.bizSvc.(sharedPoolUsageReservationRecoverer); ok {
		reservationCtx, reservationCancel := context.WithTimeout(context.Background(), poolSeatBillingTimeout)
		reservationSummary, reservationErr = recoverer.RecoverExpiredSharedPoolUsageReservations(reservationCtx, now, 100)
		reservationCancel()
		if reservationErr != nil {
			slog.Error("[PoolSeatBilling] failed to recover shared pool usage reservations", "error", reservationErr)
		}
	}
	expiredMediaTasks := 0
	var mediaTaskErr error
	if recoverer, ok := s.bizSvc.(sharedPoolMediaTaskRecoverer); ok {
		mediaTaskCtx, mediaTaskCancel := context.WithTimeout(context.Background(), poolSeatBillingTimeout)
		expiredMediaTasks, mediaTaskErr = recoverer.RecoverExpiredSharedPoolMediaTasks(mediaTaskCtx, now.UTC(), 100)
		mediaTaskCancel()
		if mediaTaskErr != nil {
			slog.Error("[PoolSeatBilling] failed to recover expired shared pool media tasks", "error", mediaTaskErr)
		}
	}
	if summary == nil {
		summary = &ChargeSeatsSummary{}
	}
	if idleSummary != nil {
		summary.IdleSeatsReleased += idleSummary.IdleSeatsReleased
		summary.HoursCharged += idleSummary.HoursCharged
		summary.TotalCharged += idleSummary.TotalCharged
		summary.SeatsProcessed += idleSummary.SeatsProcessed
	}
	if rewardSummary != nil {
		summary.StabilityRewards += rewardSummary.StabilityRewards
		summary.TotalStabilityCredit += rewardSummary.TotalStabilityCredit
	}
	probed := 0
	probeSucceeded := 0
	probeFailed := 0
	if probeSummary != nil {
		probed = probeSummary.PoolsChecked
		probeSucceeded = probeSummary.Succeeded
		probeFailed = probeSummary.Failed
	}
	reservationsScanned := 0
	reservationsReleased := 0
	reservationsReviewRequired := 0
	reservationsAutoReleased := 0
	reservationsAutoReleaseFailed := 0
	pendingSettlementsScanned := 0
	reservationsSettled := 0
	reservationsFailed := 0
	if reservationSummary != nil {
		reservationsScanned = reservationSummary.Scanned
		reservationsReleased = reservationSummary.Released
		reservationsReviewRequired = reservationSummary.ReviewRequired
		reservationsAutoReleased = reservationSummary.AutoReleased
		reservationsAutoReleaseFailed = reservationSummary.AutoReleaseFailed
		pendingSettlementsScanned = reservationSummary.PendingScanned
		reservationsSettled = reservationSummary.Settled
		reservationsFailed = reservationSummary.Failed
	}
	if reservationsReviewRequired > 0 {
		slog.Error("[PoolSeatBilling] shared pool usage reservations require manual review; frozen funds were retained",
			"review_required", reservationsReviewRequired)
	}
	if reservationsFailed > 0 {
		slog.Error("[PoolSeatBilling] shared pool usage reservation recovery or settlement failed",
			"failed", reservationsFailed)
	}
	if reservationsAutoReleaseFailed > 0 {
		slog.Error("[PoolSeatBilling] shared pool low-value review auto-release found balance mismatches",
			"failed", reservationsAutoReleaseFailed)
	}
	if summary.SeatsProcessed > 0 || summary.IdleSeatsReleased > 0 || summary.StabilityRewards > 0 || probed > 0 || expiredMediaGates > 0 || expiredMediaTasks > 0 || reservationsScanned > 0 || pendingSettlementsScanned > 0 || probeErr != nil || mediaGateErr != nil || mediaTaskErr != nil || chargeErr != nil || idleErr != nil || rewardErr != nil || reservationErr != nil {
		slog.Info("[PoolSeatBilling] shared pool sweep completed",
			"probed", probed,
			"probe_succeeded", probeSucceeded,
			"probe_failed", probeFailed,
			"probe_error", probeErr != nil,
			"media_gates_expired", expiredMediaGates,
			"media_gate_error", mediaGateErr != nil,
			"media_tasks_review_required", expiredMediaTasks,
			"media_task_error", mediaTaskErr != nil,
			"charge_error", chargeErr != nil,
			"idle_error", idleErr != nil,
			"reward_error", rewardErr != nil,
			"seats", summary.SeatsProcessed,
			"hours", summary.HoursCharged,
			"total", summary.TotalCharged,
			"idle_released", summary.IdleSeatsReleased,
			"stability_rewards", summary.StabilityRewards,
			"stability_credit", summary.TotalStabilityCredit,
			"reservations_scanned", reservationsScanned,
			"reservations_released", reservationsReleased,
			"reservations_review_required", reservationsReviewRequired,
			"reservations_auto_released", reservationsAutoReleased,
			"reservations_auto_release_failed", reservationsAutoReleaseFailed,
			"pending_settlements_scanned", pendingSettlementsScanned,
			"reservations_settled", reservationsSettled,
			"reservations_failed", reservationsFailed,
			"reservation_error", reservationErr != nil)
	}
}

func (s *PoolSeatBillingService) runProbeJobsOnce(runner sharedPoolProbeJobRunner) {
	if s == nil || runner == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), sharedPoolProbeJobTimeout+15*time.Second)
	processed, err := runner.RunSharedPoolProbeJobs(ctx, s.instanceID, 1)
	cancel()
	if err != nil {
		slog.Error("[SharedPoolProbeJobs] worker cycle failed", "error", err)
		return
	}
	if processed > 0 {
		slog.Info("[SharedPoolProbeJobs] worker cycle completed", "processed", processed)
	}
}

func (s *PoolSeatBillingService) runCanonicalSettlementOnce(parent context.Context, settler canonicalSharedPoolSettler) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	summary, err := settler.SettlePendingCanonicalSharedPoolUsage(ctx, 100)
	if err != nil {
		if parent.Err() == nil {
			slog.Error("[SharedPoolSettlement] drain failed", "error", err)
		}
		return
	}
	if summary != nil && summary.Scanned > 0 {
		slog.Info("[SharedPoolSettlement] drain completed", "scanned", summary.Scanned,
			"settled", summary.Settled, "deferred", summary.Deferred, "failed", summary.Failed)
	}
}
