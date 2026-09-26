package repository

import (
	"math"
	"testing"
	"time"
)

func TestPreviousCompleteUTCDay(t *testing.T) {
	now := time.Date(2026, 7, 14, 3, 15, 0, 0, time.UTC)
	got := previousCompleteUTCDay(now)
	want := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("previousCompleteUTCDay = %s, want %s", got, want)
	}
}

func TestSharedPoolStabilityBaseByRank(t *testing.T) {
	cases := map[int]float64{1: 120, 2: 72, 3: 36, 4: 0, 0: 0, 99: 0}
	for rank, want := range cases {
		if got := sharedPoolStabilityBaseByRank(rank); got != want {
			t.Fatalf("rank %d: got %.0f want %.0f", rank, got, want)
		}
	}
}

func TestComputeSharedPoolStabilityPoolBreakdownFullQuality(t *testing.T) {
	metrics := sharedPoolStabilityPoolMetrics{
		PoolID:                   1,
		TodayAvailability:        98,
		SevenDayAvailability:     97,
		AvgLatencyMS:             1200,
		ConsecutiveProbeFailures: 0,
		HasRealCalls:             true,
		HasPaidOrSeatUsers:       true,
	}
	got := computeSharedPoolStabilityPoolBreakdown(metrics, 1)
	// 120 + 30 + 20 + 20 + 30 + 30 = 250
	want := 250.0
	if math.Abs(got.TotalAmount-want) > 0.001 {
		t.Fatalf("total=%.2f want %.2f breakdown=%+v", got.TotalAmount, want, got)
	}
	if got.BaseAmount != 120 || got.AvailabilityBonus != 30 || got.LowProbeFailBonus != 20 ||
		got.StableLatencyBonus != 20 || got.RealCallsBonus != 30 || got.PaidSeatsBonus != 30 {
		t.Fatalf("unexpected component breakdown: %+v", got)
	}
}

func TestComputeSharedPoolStabilityPoolBreakdownFourthPoolNoBase(t *testing.T) {
	metrics := sharedPoolStabilityPoolMetrics{
		PoolID:                   9,
		TodayAvailability:        99,
		AvgLatencyMS:             800,
		ConsecutiveProbeFailures: 0,
		HasRealCalls:             false,
		HasPaidOrSeatUsers:       false,
	}
	got := computeSharedPoolStabilityPoolBreakdown(metrics, 4)
	// base 0 + 30 + 20 + 20 = 70
	if math.Abs(got.TotalAmount-70) > 0.001 {
		t.Fatalf("total=%.2f want 70", got.TotalAmount)
	}
	if got.BaseAmount != 0 {
		t.Fatalf("base should be 0 for rank 4, got %.0f", got.BaseAmount)
	}
}

func TestRankSharedPoolStabilityPools(t *testing.T) {
	pools := []sharedPoolStabilityPoolMetrics{
		{PoolID: 10, QualityScore: 50, TodayAvailability: 90},
		{PoolID: 11, QualityScore: 80, TodayAvailability: 95},
		{PoolID: 12, QualityScore: 80, TodayAvailability: 99},
	}
	ranks := rankSharedPoolStabilityPools(pools)
	if ranks[12] != 1 || ranks[11] != 2 || ranks[10] != 3 {
		t.Fatalf("unexpected ranks: %+v", ranks)
	}
}

func TestNormalizeSharedPoolStabilityOwnerTier(t *testing.T) {
	if normalizeSharedPoolStabilityOwnerTier("certified") != sharedPoolStabilityOwnerTierCertified {
		t.Fatal("certified")
	}
	if normalizeSharedPoolStabilityOwnerTier("quality_owner") != sharedPoolStabilityOwnerTierQuality {
		t.Fatal("quality_owner")
	}
	if sharedPoolStabilityOwnerExcellenceAmount(sharedPoolStabilityOwnerTierCertified) != 150 {
		t.Fatal("certified amount")
	}
	if sharedPoolStabilityOwnerExcellenceAmount(sharedPoolStabilityOwnerTierQuality) != 80 {
		t.Fatal("quality amount")
	}
	if sharedPoolStabilityOwnerExcellenceAmount(sharedPoolStabilityOwnerTierNone) != 0 {
		t.Fatal("none amount")
	}
}
