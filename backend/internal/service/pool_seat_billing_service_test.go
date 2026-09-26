package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type poolSeatBillingBizServiceStub struct {
	probeErr     error
	mediaGateErr error
	mediaTaskErr error
	chargeErr    error
	idleErr      error
	rewardErr    error

	probeCalls     int
	mediaGateCalls int
	mediaTaskCalls int
	chargeCalls    int
	idleCalls      int
	rewardCalls    int
	probeCtx       context.Context
	mediaGateCtx   context.Context
	mediaGateNow   time.Time
	mediaGateLimit int
	mediaTaskCtx   context.Context
	mediaTaskNow   time.Time
	mediaTaskLimit int
	chargeCtx      context.Context
	idleCtx        context.Context
	rewardCtx      context.Context
}

type promptSettlementStub struct {
	poolSeatBillingBizServiceStub
	started   chan struct{}
	cancelled chan struct{}
}

func (s *promptSettlementStub) SettlePendingCanonicalSharedPoolUsage(ctx context.Context, limit int) (*CanonicalSharedPoolSettlementSummary, error) {
	close(s.started)
	<-ctx.Done()
	close(s.cancelled)
	return nil, ctx.Err()
}

func TestPoolSeatBillingSettlementStartsImmediatelyAndStops(t *testing.T) {
	stub := &promptSettlementStub{started: make(chan struct{}), cancelled: make(chan struct{})}
	svc := NewPoolSeatBillingService(stub, time.Hour)
	svc.Start()
	select {
	case <-stub.started:
	case <-time.After(2 * time.Second):
		svc.Stop()
		t.Fatal("settlement waited for the hourly maintenance sweep")
	}
	stopped := make(chan struct{})
	go func() { svc.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("settlement did not stop with the service")
	}
	select {
	case <-stub.cancelled:
	default:
		t.Fatal("active settlement context was not cancelled")
	}
}

func (s *poolSeatBillingBizServiceStub) ExpireSharedPoolMediaEndpointGates(ctx context.Context, now time.Time, limit int) (int, error) {
	s.mediaGateCalls++
	s.mediaGateCtx = ctx
	s.mediaGateNow = now
	s.mediaGateLimit = limit
	return 2, s.mediaGateErr
}

func (s *poolSeatBillingBizServiceStub) RecoverExpiredSharedPoolMediaTasks(ctx context.Context, now time.Time, limit int) (int, error) {
	s.mediaTaskCalls++
	s.mediaTaskCtx = ctx
	s.mediaTaskNow = now
	s.mediaTaskLimit = limit
	return 3, s.mediaTaskErr
}

func (s *poolSeatBillingBizServiceStub) RunSharedPoolProbeAggregation(ctx context.Context, now time.Time, limit int) (*SharedPoolProbeAggregationSummary, error) {
	s.probeCalls++
	s.probeCtx = ctx
	return nil, s.probeErr
}

func (s *poolSeatBillingBizServiceStub) ChargeDueSeats(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error) {
	s.chargeCalls++
	s.chargeCtx = ctx
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return nil, s.chargeErr
}

func (s *poolSeatBillingBizServiceStub) ReleaseIdleSharedPoolSeats(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error) {
	s.idleCalls++
	s.idleCtx = ctx
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &ChargeSeatsSummary{IdleSeatsReleased: 1}, s.idleErr
}

func (s *poolSeatBillingBizServiceStub) GrantSharedPoolStabilityRewards(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error) {
	s.rewardCalls++
	s.rewardCtx = ctx
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &ChargeSeatsSummary{StabilityRewards: 2, TotalStabilityCredit: 60}, s.rewardErr
}

func TestPoolSeatBillingLeaderLockTTLCoversWorstCaseSweep(t *testing.T) {
	if poolSeatBillingLeaderLockTTL < 15*time.Minute {
		t.Fatalf("leader lock TTL must remain at least 15 minutes, got %s", poolSeatBillingLeaderLockTTL)
	}
	if poolSeatBillingLeaderLockTTL <= poolSeatBillingMaxSweepDuration {
		t.Fatalf("leader lock TTL %s must exceed worst-case serial sweep duration %s", poolSeatBillingLeaderLockTTL, poolSeatBillingMaxSweepDuration)
	}
}

func TestPoolSeatBillingRunOnceContinuesToRewardsWhenEarlierStepsFail(t *testing.T) {
	bizSvc := &poolSeatBillingBizServiceStub{
		probeErr:  errors.New("probe timeout"),
		chargeErr: errors.New("charge failed"),
		idleErr:   errors.New("idle release failed"),
	}
	svc := NewPoolSeatBillingService(bizSvc, time.Minute)

	svc.runOnce()

	if bizSvc.probeCalls != 1 {
		t.Fatalf("expected probe to run once, got %d", bizSvc.probeCalls)
	}
	if bizSvc.chargeCalls != 1 {
		t.Fatalf("expected charge to run once after probe error, got %d", bizSvc.chargeCalls)
	}
	if bizSvc.idleCalls != 1 {
		t.Fatalf("expected idle release to run once after charge error, got %d", bizSvc.idleCalls)
	}
	if bizSvc.rewardCalls != 1 {
		t.Fatalf("expected stability rewards to run after probe/charge/idle errors, got %d", bizSvc.rewardCalls)
	}
}

func TestPoolSeatBillingRunOnceExpiresMediaGatesUnderLeaderSweep(t *testing.T) {
	bizSvc := &poolSeatBillingBizServiceStub{}
	svc := NewPoolSeatBillingService(bizSvc, time.Minute)

	svc.runOnce()

	if bizSvc.mediaGateCalls != 1 {
		t.Fatalf("expected media gate expiry once, got %d", bizSvc.mediaGateCalls)
	}
	if bizSvc.mediaGateCtx == nil {
		t.Fatal("expected media gate expiry context")
	}
	if bizSvc.mediaGateNow.IsZero() {
		t.Fatal("expected media gate expiry timestamp")
	}
	if bizSvc.mediaGateLimit != 100 {
		t.Fatalf("expected bounded media gate expiry limit 100, got %d", bizSvc.mediaGateLimit)
	}
}

func TestPoolSeatBillingRunOnceMovesExpiredMediaTasksToReview(t *testing.T) {
	bizSvc := &poolSeatBillingBizServiceStub{}
	svc := NewPoolSeatBillingService(bizSvc, time.Minute)

	svc.runOnce()

	if bizSvc.mediaTaskCalls != 1 {
		t.Fatalf("expected media task recovery once, got %d", bizSvc.mediaTaskCalls)
	}
	if bizSvc.mediaTaskCtx == nil {
		t.Fatal("expected media task recovery context")
	}
	if bizSvc.mediaTaskNow.IsZero() {
		t.Fatal("expected media task recovery timestamp")
	}
	if bizSvc.mediaTaskLimit != 100 {
		t.Fatalf("expected bounded media task recovery limit 100, got %d", bizSvc.mediaTaskLimit)
	}
}

func TestPoolSeatBillingRunOnceUsesFreshContextPerStep(t *testing.T) {
	bizSvc := &poolSeatBillingBizServiceStub{}
	svc := NewPoolSeatBillingService(bizSvc, time.Minute)

	svc.runOnce()

	if bizSvc.probeCtx == nil || bizSvc.chargeCtx == nil || bizSvc.idleCtx == nil || bizSvc.rewardCtx == nil {
		t.Fatal("expected every billing step to receive a context")
	}
	if bizSvc.probeCtx == bizSvc.chargeCtx {
		t.Fatal("charge step must not reuse probe context")
	}
	if bizSvc.chargeCtx == bizSvc.idleCtx {
		t.Fatal("idle step must not reuse charge context")
	}
	if bizSvc.idleCtx == bizSvc.rewardCtx {
		t.Fatal("reward step must not reuse idle context")
	}
	if bizSvc.probeCtx == bizSvc.rewardCtx {
		t.Fatal("reward step must not reuse probe context")
	}
}
