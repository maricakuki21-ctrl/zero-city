package service

import (
	"context"
	"math"

	"github.com/shopspring/decimal"
)

const affiliateGrowthMaxDirectPercent = 12.0

type AffiliateGrowthTier struct {
	Name                  string  `json:"name"`
	QualifiedInvitees      int     `json:"qualified_invitees"`
	DirectPercent         float64 `json:"direct_percent"`
	IndirectPercent       float64 `json:"indirect_percent"`
}

type AffiliateGrowth struct {
	QualifiedPaidInvitees int                   `json:"qualified_paid_invitees"`
	CurrentTier          AffiliateGrowthTier   `json:"current_tier"`
	Tiers                []AffiliateGrowthTier `json:"tiers"`
	DirectPercent        float64               `json:"direct_percent"`
	IndirectPercent      float64               `json:"indirect_percent"`
	MaxCombinedPercent   float64               `json:"max_combined_percent"`
}

func affiliateGrowthTiers() []AffiliateGrowthTier {
	return []AffiliateGrowthTier{
		{Name: "伙伴", QualifiedInvitees: 0, DirectPercent: 5, IndirectPercent: 1},
		{Name: "共创者", QualifiedInvitees: 5, DirectPercent: 8, IndirectPercent: 2},
		{Name: "城市合伙人", QualifiedInvitees: 20, DirectPercent: 12, IndirectPercent: 3},
	}
}

func resolveAffiliateGrowth(count int, override *float64) AffiliateGrowth {
	tiers := affiliateGrowthTiers()
	current := tiers[0]
	for _, tier := range tiers {
		if count >= tier.QualifiedInvitees {
			current = tier
		}
	}
	direct := current.DirectPercent
	if override != nil && !math.IsNaN(*override) && !math.IsInf(*override, 0) {
		direct = math.Max(0, math.Min(affiliateGrowthMaxDirectPercent, *override))
	}
	return AffiliateGrowth{
		QualifiedPaidInvitees: count, CurrentTier: current, Tiers: tiers,
		DirectPercent: direct, IndirectPercent: current.IndirectPercent, MaxCombinedPercent: 15,
	}
}

func (s *AffiliateService) affiliateGrowth(ctx context.Context, summary *AffiliateSummary) (*AffiliateGrowth, error) {
	overview, err := s.repo.GetAffiliateUserOverview(ctx, summary.UserID)
	if err != nil {
		return nil, err
	}
	if overview == nil {
		return nil, ErrAffiliateProfileNotFound
	}
	growth := resolveAffiliateGrowth(overview.QualifiedPaidInvitees, summary.AffRebateRatePercent)
	return &growth, nil
}

func affiliatePercentageAmount(base, percent float64) float64 {
	// Truncation ensures two independently rounded rewards never exceed the cap.
	result, _ := decimal.NewFromFloat(base).Mul(decimal.NewFromFloat(percent)).
		Div(decimal.NewFromInt(100)).Truncate(8).Float64()
	return result
}
