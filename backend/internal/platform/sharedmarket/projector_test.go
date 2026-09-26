package sharedmarket

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type runtimeStub struct {
	facts RuntimeFacts
	err   error
}

func (s *runtimeStub) LoadRuntime(context.Context, int64, int64) (RuntimeFacts, error) {
	return s.facts, s.err
}

type financialStub struct {
	facts FinancialFacts
	err   error
}

func (s *financialStub) LoadFinancial(context.Context, int64) (FinancialFacts, error) {
	return s.facts, s.err
}

func TestProjectorPreservesExactDecimalsAndSeparatesRuntimeFromProduct(t *testing.T) {
	observedAt := time.Date(2026, time.September, 2, 9, 0, 0, 0, time.UTC)
	productEvidence := Evidence{Source: SourceBizPriceSnapshot, ObservedAt: observedAt, Freshness: FreshnessLive, Confidence: ConfidenceHigh}
	runtimeEvidence := Evidence{Source: SourceSub2UsageLog, ObservedAt: observedAt, Freshness: FreshnessRecent, Confidence: ConfidenceHigh}
	rate, err := NewDecimalString("0.0001")
	require.NoError(t, err)
	zero, err := NewDecimalString("0")
	require.NoError(t, err)

	projector := NewProjector(
		&runtimeStub{facts: RuntimeFacts{
			GroupID:       Fact[string]{Value: "group-91", Evidence: runtimeEvidence},
			Models:        Fact[[]Model]{Value: []Model{{Name: "gpt-5", Executable: true}}, Evidence: runtimeEvidence},
			LatencyMS:     Fact[DecimalString]{Value: DecimalString("12.50"), Evidence: runtimeEvidence},
			ThroughputRPM: Fact[DecimalString]{Value: DecimalString("3.25"), Evidence: runtimeEvidence},
		}},
		&financialStub{facts: FinancialFacts{
			AvailableBalance: Fact[DecimalString]{Value: rate, Evidence: Evidence{Source: SourceBizLedger, ObservedAt: observedAt, Freshness: FreshnessLive, Confidence: ConfidenceHigh}},
		}},
	)
	projection, err := projector.Project(context.Background(), Input{
		PoolID: 42, CanonicalGroupID: 91,
		Product: ProductFacts{Pricing: PricingFacts{
			RateMultiplier: Fact[DecimalString]{Value: rate, Evidence: productEvidence},
			MinimumBalance: Fact[DecimalString]{Value: zero, Evidence: productEvidence},
		}},
		OfficialServiceStatus: Fact[string]{Value: "operational", Evidence: Evidence{Source: SourceOfficialStatus, ObservedAt: observedAt, Freshness: FreshnessLive, Confidence: ConfidenceHigh}},
	})

	require.NoError(t, err)
	require.Equal(t, DecimalString("0.0001"), projection.Product.Pricing.RateMultiplier.Value)
	require.Equal(t, SourceSub2UsageLog, projection.Runtime.LatencyMS.Evidence.Source)
	require.Equal(t, SourceBizLedger, projection.Product.Financial.AvailableBalance.Evidence.Source)
	require.Empty(t, projection.Errors)
}

func TestProjectorKeepsLastVerifiedFactsAndOfficialStatusWhenDependenciesFail(t *testing.T) {
	runtime := &runtimeStub{facts: RuntimeFacts{Health: Fact[Health]{Value: Health{State: "healthy", Confidence: ConfidenceHigh}}}}
	financial := &financialStub{facts: FinancialFacts{OwnerNet: Fact[DecimalString]{Value: DecimalString("12.3400")}}}
	projector := NewProjector(runtime, financial)
	input := Input{PoolID: 7, CanonicalGroupID: 8, OfficialServiceStatus: Fact[string]{Value: "degraded"}}

	first, err := projector.Project(context.Background(), input)
	require.NoError(t, err)
	require.False(t, first.Stale)

	runtime.err = errors.New("timeout")
	financial.err = errors.New("projection lag")
	second, err := projector.Project(context.Background(), input)
	require.NoError(t, err)
	require.True(t, second.Stale)
	require.Equal(t, "healthy", second.Runtime.Health.Value.State)
	require.Equal(t, FreshnessLastVerified, second.Runtime.Health.Evidence.Freshness)
	require.Equal(t, DecimalString("12.3400"), second.Product.Financial.OwnerNet.Value)
	require.Equal(t, FreshnessLastVerified, second.Product.Financial.OwnerNet.Evidence.Freshness)
	require.Equal(t, "degraded", second.OfficialServiceStatus.Value)
	require.Len(t, second.Errors, 2)
	require.True(t, second.Errors[0].Retryable)
	require.NotEmpty(t, second.Errors[0].Action)
}

func TestNewDecimalStringRejectsAmbiguousOrExponentForms(t *testing.T) {
	for _, value := range []string{"", ".1", "01", "1e-4", "NaN"} {
		_, err := NewDecimalString(value)
		require.ErrorIs(t, err, ErrInvalidDecimalString)
	}
}
