package repository

import (
	"fmt"
	"strings"
	"time"
)

// Daily shared-pool stability rewards (non-withdrawable credits).
// Settlement unit is one complete UTC day; reward_hour column stores day start.
const (
	sharedPoolStabilityBasePool1 = 120.0
	sharedPoolStabilityBasePool2 = 72.0
	sharedPoolStabilityBasePool3 = 36.0

	sharedPoolStabilityBonusAvailability  = 30.0
	sharedPoolStabilityBonusLowProbeFail  = 20.0
	sharedPoolStabilityBonusStableLatency = 20.0

	sharedPoolStabilityBonusRealCalls = 30.0
	sharedPoolStabilityBonusPaidSeats = 30.0

	sharedPoolStabilityOwnerQualityDaily   = 80.0
	sharedPoolStabilityOwnerCertifiedDaily = 150.0

	sharedPoolStabilityAvailabilityThreshold = 95.0
	// Stable latency: positive and not worse than this threshold (ms).
	sharedPoolStabilityLatencyMaxMS = 3000
)

// sharedPoolStabilityOwnerTier is the admin-controlled owner excellence tier.
// Values: none | quality | certified
type sharedPoolStabilityOwnerTier string

const (
	sharedPoolStabilityOwnerTierNone      sharedPoolStabilityOwnerTier = "none"
	sharedPoolStabilityOwnerTierQuality   sharedPoolStabilityOwnerTier = "quality"
	sharedPoolStabilityOwnerTierCertified sharedPoolStabilityOwnerTier = "certified"
)

type sharedPoolStabilityPoolMetrics struct {
	PoolID                   int64
	OwnerID                  int64
	TodayAvailability        float64
	SevenDayAvailability     float64
	AvgLatencyMS             int
	ConsecutiveProbeFailures int
	QualityScore             float64
	HasRealCalls             bool
	HasPaidOrSeatUsers       bool
}

type sharedPoolStabilityBreakdown struct {
	BaseAmount           float64
	BaseRank             int // 1-based rank among owner's eligible pools; 0 when not ranked
	AvailabilityBonus    float64
	LowProbeFailBonus    float64
	StableLatencyBonus   float64
	RealCallsBonus       float64
	PaidSeatsBonus       float64
	OwnerExcellenceBonus float64
	TotalAmount          float64
	NoteParts            []string
}

func previousCompleteUTCDay(now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	return now.UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour)
}

func sharedPoolStabilityBaseByRank(rank int) float64 {
	switch rank {
	case 1:
		return sharedPoolStabilityBasePool1
	case 2:
		return sharedPoolStabilityBasePool2
	case 3:
		return sharedPoolStabilityBasePool3
	default:
		return 0
	}
}

func normalizeSharedPoolStabilityOwnerTier(raw string) sharedPoolStabilityOwnerTier {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(sharedPoolStabilityOwnerTierQuality), "quality_owner", "quality-owner":
		return sharedPoolStabilityOwnerTierQuality
	case string(sharedPoolStabilityOwnerTierCertified), "certified_owner", "certified-owner":
		return sharedPoolStabilityOwnerTierCertified
	default:
		return sharedPoolStabilityOwnerTierNone
	}
}

func sharedPoolStabilityOwnerExcellenceAmount(tier sharedPoolStabilityOwnerTier) float64 {
	switch tier {
	case sharedPoolStabilityOwnerTierCertified:
		return sharedPoolStabilityOwnerCertifiedDaily
	case sharedPoolStabilityOwnerTierQuality:
		return sharedPoolStabilityOwnerQualityDaily
	default:
		return 0
	}
}

// betterSharedPoolStabilityCandidate returns true when a is a better base-tier
// candidate than b (higher quality first, then availability, then lower id).
func betterSharedPoolStabilityCandidate(a, b sharedPoolStabilityPoolMetrics) bool {
	if a.QualityScore != b.QualityScore {
		return a.QualityScore > b.QualityScore
	}
	aAvail := a.effectiveAvailability()
	bAvail := b.effectiveAvailability()
	if aAvail != bAvail {
		return aAvail > bAvail
	}
	return a.PoolID < b.PoolID
}

func (m sharedPoolStabilityPoolMetrics) effectiveAvailability() float64 {
	// Prefer seven-day when present; fall back to today.
	if m.SevenDayAvailability > 0 {
		return m.SevenDayAvailability
	}
	return m.TodayAvailability
}

func computeSharedPoolStabilityPoolBreakdown(metrics sharedPoolStabilityPoolMetrics, baseRank int) sharedPoolStabilityBreakdown {
	out := sharedPoolStabilityBreakdown{
		BaseRank:   baseRank,
		BaseAmount: sharedPoolStabilityBaseByRank(baseRank),
	}
	if out.BaseAmount > 0 {
		out.NoteParts = append(out.NoteParts, fmt.Sprintf("base_rank=%d:%.0f", baseRank, out.BaseAmount))
	} else if baseRank > 0 {
		out.NoteParts = append(out.NoteParts, fmt.Sprintf("base_rank=%d:0", baseRank))
	}

	if metrics.effectiveAvailability() >= sharedPoolStabilityAvailabilityThreshold {
		out.AvailabilityBonus = sharedPoolStabilityBonusAvailability
		out.NoteParts = append(out.NoteParts, fmt.Sprintf("availability>=%.0f:+%.0f", sharedPoolStabilityAvailabilityThreshold, out.AvailabilityBonus))
	}
	if metrics.ConsecutiveProbeFailures <= 0 {
		out.LowProbeFailBonus = sharedPoolStabilityBonusLowProbeFail
		out.NoteParts = append(out.NoteParts, fmt.Sprintf("low_probe_fail:+%.0f", out.LowProbeFailBonus))
	}
	if metrics.AvgLatencyMS > 0 && metrics.AvgLatencyMS <= sharedPoolStabilityLatencyMaxMS {
		out.StableLatencyBonus = sharedPoolStabilityBonusStableLatency
		out.NoteParts = append(out.NoteParts, fmt.Sprintf("stable_latency:+%.0f", out.StableLatencyBonus))
	}
	if metrics.HasRealCalls {
		out.RealCallsBonus = sharedPoolStabilityBonusRealCalls
		out.NoteParts = append(out.NoteParts, fmt.Sprintf("real_calls:+%.0f", out.RealCallsBonus))
	}
	if metrics.HasPaidOrSeatUsers {
		out.PaidSeatsBonus = sharedPoolStabilityBonusPaidSeats
		out.NoteParts = append(out.NoteParts, fmt.Sprintf("paid_seats:+%.0f", out.PaidSeatsBonus))
	}

	out.TotalAmount = out.BaseAmount +
		out.AvailabilityBonus +
		out.LowProbeFailBonus +
		out.StableLatencyBonus +
		out.RealCallsBonus +
		out.PaidSeatsBonus
	return out
}

func rankSharedPoolStabilityPools(pools []sharedPoolStabilityPoolMetrics) map[int64]int {
	// Stable insertion sort by betterSharedPoolStabilityCandidate.
	ordered := make([]sharedPoolStabilityPoolMetrics, len(pools))
	copy(ordered, pools)
	for i := 1; i < len(ordered); i++ {
		j := i
		for j > 0 && betterSharedPoolStabilityCandidate(ordered[j], ordered[j-1]) {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
			j--
		}
	}
	ranks := make(map[int64]int, len(ordered))
	for i, p := range ordered {
		ranks[p.PoolID] = i + 1
	}
	return ranks
}

func formatSharedPoolStabilityNote(poolID int64, rewardDay time.Time, parts []string, ownerBonus float64) string {
	day := rewardDay.UTC().Format("2006-01-02")
	base := fmt.Sprintf("Shared pool daily stability reward: pool %d day %s", poolID, day)
	if len(parts) == 0 && ownerBonus <= 0 {
		return base
	}
	joined := strings.Join(parts, "; ")
	if ownerBonus > 0 {
		if joined != "" {
			joined += "; "
		}
		joined += fmt.Sprintf("owner_excellence:+%.0f", ownerBonus)
	}
	return base + " | " + joined
}
