package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
)

type canonicalGatewayRuntimeContextKey struct{}

type CanonicalGatewayRuntimeInput struct {
	GroupID       int64
	AccountID     int64
	BillingPolicy corecontracts.BillingPolicy
	AcceptedQuote *SharedPoolPriceQuote
}

type CanonicalGatewayRuntime struct {
	groupID       int64
	accountID     int64
	billingPolicy corecontracts.BillingPolicy
	acceptedQuote *SharedPoolPriceQuote
	coordinator   *AttemptCoordinator
	observer      *AttemptObserver
}

func NewCanonicalGatewayRuntime(input CanonicalGatewayRuntimeInput, classifier RetryClassifier) (*CanonicalGatewayRuntime, error) {
	coordinator, err := NewAttemptCoordinator(classifier)
	if err != nil {
		return nil, err
	}
	policy, err := corecontracts.ResolveBillingPolicy(input.BillingPolicy)
	if err != nil {
		return nil, err
	}
	return &CanonicalGatewayRuntime{
		groupID:       input.GroupID,
		accountID:     input.AccountID,
		billingPolicy: policy,
		acceptedQuote: cloneSharedPoolPriceQuote(input.AcceptedQuote),
		coordinator:   coordinator,
		observer:      newAttemptObserver(),
	}, nil
}

func WithCanonicalGatewayRuntime(ctx context.Context, runtime *CanonicalGatewayRuntime) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if runtime == nil {
		return ctx
	}
	return context.WithValue(ctx, canonicalGatewayRuntimeContextKey{}, runtime)
}

func CanonicalGatewayRuntimeFromContext(ctx context.Context) (*CanonicalGatewayRuntime, bool) {
	if ctx == nil {
		return nil, false
	}
	runtime, ok := ctx.Value(canonicalGatewayRuntimeContextKey{}).(*CanonicalGatewayRuntime)
	return runtime, ok && runtime != nil
}

func ResolveRequestBillingPolicy(ctx context.Context, explicit corecontracts.BillingPolicy) (corecontracts.BillingPolicy, error) {
	if explicit != "" {
		return corecontracts.ResolveBillingPolicy(explicit)
	}
	if runtime, ok := CanonicalGatewayRuntimeFromContext(ctx); ok {
		return runtime.billingPolicy, nil
	}
	return corecontracts.ResolveBillingPolicy("")
}

func (r *CanonicalGatewayRuntime) GroupID() int64 {
	if r == nil {
		return 0
	}
	return r.groupID
}

func (r *CanonicalGatewayRuntime) AccountID() (int64, bool) {
	if r == nil || r.accountID <= 0 {
		return 0, false
	}
	return r.accountID, true
}

func (r *CanonicalGatewayRuntime) AcceptedQuote() *SharedPoolPriceQuote {
	if r == nil {
		return nil
	}
	return cloneSharedPoolPriceQuote(r.acceptedQuote)
}

func cloneSharedPoolPriceQuote(quote *SharedPoolPriceQuote) *SharedPoolPriceQuote {
	if quote == nil {
		return nil
	}
	cloned := *quote
	cloned.BasePrice = cloneSharedPoolPriceComponents(quote.BasePrice)
	cloned.UserPrice = cloneSharedPoolPriceComponents(quote.UserPrice)
	if quote.OwnerPrice != nil {
		owner := cloneSharedPoolPriceComponents(*quote.OwnerPrice)
		cloned.OwnerPrice = &owner
	}
	return &cloned
}

func cloneSharedPoolPriceComponents(components SharedPoolPriceComponents) SharedPoolPriceComponents {
	return SharedPoolPriceComponents{
		BillingMode:      components.BillingMode,
		Currency:         components.Currency,
		InputPrice:       cloneSharedPoolPriceValue(components.InputPrice),
		OutputPrice:      cloneSharedPoolPriceValue(components.OutputPrice),
		CacheReadPrice:   cloneSharedPoolPriceValue(components.CacheReadPrice),
		CacheWritePrice:  cloneSharedPoolPriceValue(components.CacheWritePrice),
		ImageItemPrice:   cloneSharedPoolPriceValue(components.ImageItemPrice),
		VideoSecondPrice: cloneSharedPoolPriceValue(components.VideoSecondPrice),
		PerRequestPrice:  cloneSharedPoolPriceValue(components.PerRequestPrice),
		MinimumCharge:    cloneSharedPoolPriceValue(components.MinimumCharge),
		MaximumCharge:    cloneSharedPoolPriceValue(components.MaximumCharge),
	}
}

func cloneSharedPoolPriceValue(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func (r *CanonicalGatewayRuntime) Commit() error {
	if r == nil || r.observer == nil {
		return nil
	}
	return r.observer.Commit()
}

func (r *CanonicalGatewayRuntime) AcceptTask() error {
	if r == nil || r.observer == nil {
		return nil
	}
	return r.observer.AcceptTask()
}

func (r *CanonicalGatewayRuntime) CanRetry(err error) bool {
	if r == nil || r.coordinator == nil || r.observer == nil {
		return false
	}
	return r.coordinator.canRetry(r.observer.State(), err)
}
