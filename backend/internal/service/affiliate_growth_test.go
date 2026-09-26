package service

import (
	"math"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestAffiliateGrowthThresholds(t *testing.T) {
	for _, tt := range []struct {
		count int
		direct, indirect float64
	}{
		{0, 5, 1}, {4, 5, 1}, {5, 8, 2}, {19, 8, 2}, {20, 12, 3}, {100, 12, 3},
	} {
		growth := resolveAffiliateGrowth(tt.count, nil)
		require.Equal(t, tt.direct, growth.DirectPercent)
		require.Equal(t, tt.indirect, growth.IndirectPercent)
		require.Equal(t, 15.0, growth.MaxCombinedPercent)
	}
}

func TestAffiliateGrowthOverridesCannotExceedCombinedCap(t *testing.T) {
	for _, override := range []float64{-1, 0, 8, 12, 15, 100} {
		growth := resolveAffiliateGrowth(20, &override)
		require.GreaterOrEqual(t, growth.DirectPercent, 0.0)
		require.LessOrEqual(t, growth.DirectPercent+growth.IndirectPercent, 15.0)
	}
	for _, override := range []float64{math.NaN(), math.Inf(1)} {
		require.Equal(t, 8.0, resolveAffiliateGrowth(5, &override).DirectPercent)
	}
}

func TestAffiliateGrowthTruncationRespectsBudgetAcrossTiers(t *testing.T) {
	for _, base := range []float64{0.00000001, 0.00000006, 0.12345678, 100, 999999.99999999} {
		budget := decimal.NewFromFloat(base).Mul(decimal.NewFromFloat(0.15))
		for _, directTier := range affiliateGrowthTiers() {
			for _, indirectTier := range affiliateGrowthTiers() {
				direct := affiliatePercentageAmount(base, directTier.DirectPercent)
				indirect := affiliatePercentageAmount(base, indirectTier.IndirectPercent)
				total := decimal.NewFromFloat(direct).Add(decimal.NewFromFloat(indirect))
				require.True(t, total.LessThanOrEqual(budget), "base=%v total=%s", base, total)
			}
		}
	}
}
