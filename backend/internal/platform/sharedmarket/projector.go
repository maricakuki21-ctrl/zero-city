package sharedmarket

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrInvalidProjectionInput = errors.New("shared-market projection input is invalid")
	ErrRuntimeUnavailable     = errors.New("canonical Sub2 runtime facts unavailable")
	ErrFinancialUnavailable   = errors.New("canonical ledger projection unavailable")
)

type RuntimeSource interface {
	LoadRuntime(ctx context.Context, poolID, canonicalGroupID int64) (RuntimeFacts, error)
}

type FinancialSource interface {
	LoadFinancial(ctx context.Context, poolID int64) (FinancialFacts, error)
}

type Input struct {
	PoolID                int64
	CanonicalGroupID      int64
	Product               ProductFacts
	OfficialServiceStatus Fact[string]
}

type cachedFacts struct {
	runtime   *RuntimeFacts
	financial *FinancialFacts
}

type Projector struct {
	runtime   RuntimeSource
	financial FinancialSource
	mu        sync.RWMutex
	cache     map[int64]cachedFacts
}

func NewProjector(runtime RuntimeSource, financial FinancialSource) *Projector {
	return &Projector{runtime: runtime, financial: financial, cache: make(map[int64]cachedFacts)}
}

func (p *Projector) Project(ctx context.Context, input Input) (PoolProjection, error) {
	if p == nil || input.PoolID <= 0 || input.CanonicalGroupID <= 0 {
		return PoolProjection{}, ErrInvalidProjectionInput
	}
	projection := PoolProjection{
		PoolID:                input.PoolID,
		Product:               input.Product,
		OfficialServiceStatus: input.OfficialServiceStatus,
		Errors:                []DependencyError{},
	}
	previous := p.loadCached(input.PoolID)

	if p.runtime == nil {
		projection.Runtime = lastVerifiedRuntime(previous.runtime)
		projection.recordFailure("sub2_runtime", ErrRuntimeUnavailable, "retry canonical Sub2 runtime projection")
	} else {
		runtime, err := p.runtime.LoadRuntime(ctx, input.PoolID, input.CanonicalGroupID)
		if err != nil {
			projection.Runtime = lastVerifiedRuntime(previous.runtime)
			projection.recordFailure("sub2_runtime", fmt.Errorf("%w: %v", ErrRuntimeUnavailable, err), "check Sub2 group/account health and retry")
		} else {
			projection.Runtime = &runtime
			previous.runtime = &runtime
		}
	}

	if p.financial == nil {
		projection.Product.Financial = lastVerifiedFinancial(previous.financial)
		projection.recordFailure("canonical_ledger", ErrFinancialUnavailable, "retry after the canonical ledger projection is available")
	} else {
		financial, err := p.financial.LoadFinancial(ctx, input.PoolID)
		if err != nil {
			projection.Product.Financial = lastVerifiedFinancial(previous.financial)
			projection.recordFailure("canonical_ledger", fmt.Errorf("%w: %v", ErrFinancialUnavailable, err), "inspect ledger projection lag and retry")
		} else {
			projection.Product.Financial = &financial
			previous.financial = &financial
		}
	}

	p.storeCached(input.PoolID, previous)
	return projection, nil
}

func lastVerifiedRuntime(runtime *RuntimeFacts) *RuntimeFacts {
	if runtime == nil {
		return nil
	}
	result := *runtime
	result.GroupID.Evidence.Freshness = FreshnessLastVerified
	result.AccountIDs.Evidence.Freshness = FreshnessLastVerified
	result.Models.Evidence.Freshness = FreshnessLastVerified
	result.GroupAvailability.Evidence.Freshness = FreshnessLastVerified
	result.AccountAvailability.Evidence.Freshness = FreshnessLastVerified
	result.Health.Evidence.Freshness = FreshnessLastVerified
	result.TodayAvailability.Evidence.Freshness = FreshnessLastVerified
	result.SevenDayAvailability.Evidence.Freshness = FreshnessLastVerified
	result.LatencyMS.Evidence.Freshness = FreshnessLastVerified
	result.ThroughputRPM.Evidence.Freshness = FreshnessLastVerified
	result.SuccessRatePercent.Evidence.Freshness = FreshnessLastVerified
	result.CanonicalUsage.Evidence.Freshness = FreshnessLastVerified
	return &result
}

func lastVerifiedFinancial(financial *FinancialFacts) *FinancialFacts {
	if financial == nil {
		return nil
	}
	result := *financial
	result.AvailableBalance.Evidence.Freshness = FreshnessLastVerified
	result.OwnerGross.Evidence.Freshness = FreshnessLastVerified
	result.OwnerNet.Evidence.Freshness = FreshnessLastVerified
	result.PlatformFee.Evidence.Freshness = FreshnessLastVerified
	result.ProcessorFee.Evidence.Freshness = FreshnessLastVerified
	result.Residue.Evidence.Freshness = FreshnessLastVerified
	return &result
}

func (p *Projector) loadCached(poolID int64) cachedFacts {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cache[poolID]
}

func (p *Projector) storeCached(poolID int64, facts cachedFacts) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cache[poolID] = facts
}

func (p *PoolProjection) recordFailure(dependency string, err error, action string) {
	p.Stale = true
	p.Errors = append(p.Errors, DependencyError{
		Dependency: dependency,
		Code:       dependency + "_unavailable",
		Message:    err.Error(),
		Retryable:  true,
		Action:     action,
	})
}
