package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const MarketplaceOnboardingVersion = 1

type MarketplaceOnboarding struct {
	Version     int        `json:"version"`
	Intent      string     `json:"intent"`
	CompletedAt *time.Time `json:"completed_at"`
}

type MarketplaceOnboardingRepository interface {
	GetMarketplaceOnboarding(context.Context, int64, int) (*MarketplaceOnboarding, error)
	SaveMarketplaceOnboarding(context.Context, int64, int, string) (*MarketplaceOnboarding, error)
}

func (s *MarketplaceService) GetOnboarding(ctx context.Context, userID int64) (*MarketplaceOnboarding, error) {
	if userID <= 0 {
		return nil, badMarketplace("invalid user")
	}
	repo, ok := s.repo.(MarketplaceOnboardingRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("MARKET_GUIDE_UNAVAILABLE", "marketplace guide is unavailable")
	}
	return repo.GetMarketplaceOnboarding(ctx, userID, MarketplaceOnboardingVersion)
}

func (s *MarketplaceService) CompleteOnboarding(ctx context.Context, userID int64, intent string) (*MarketplaceOnboarding, error) {
	if userID <= 0 {
		return nil, badMarketplace("invalid user")
	}
	intent = strings.TrimSpace(intent)
	if intent != "hire" && intent != "sell" && intent != "browse" {
		return nil, badMarketplace("invalid onboarding intent")
	}
	repo, ok := s.repo.(MarketplaceOnboardingRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("MARKET_GUIDE_UNAVAILABLE", "marketplace guide is unavailable")
	}
	return repo.SaveMarketplaceOnboarding(ctx, userID, MarketplaceOnboardingVersion, intent)
}
