package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.MarketplaceOnboardingRepository = (*marketplaceRepository)(nil)

func (r *marketplaceRepository) GetMarketplaceOnboarding(ctx context.Context, userID int64, version int) (*service.MarketplaceOnboarding, error) {
	result := &service.MarketplaceOnboarding{Version: version}
	err := r.db.QueryRowContext(ctx, `SELECT intent, completed_at FROM marketplace_onboarding WHERE user_id=$1 AND version=$2`, userID, version).Scan(&result.Intent, &result.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *marketplaceRepository) SaveMarketplaceOnboarding(ctx context.Context, userID int64, version int, intent string) (*service.MarketplaceOnboarding, error) {
	result := &service.MarketplaceOnboarding{Version: version}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO marketplace_onboarding(user_id,version,intent) VALUES($1,$2,$3)
		ON CONFLICT(user_id,version) DO UPDATE SET intent=EXCLUDED.intent
		RETURNING intent,completed_at`, userID, version, intent).Scan(&result.Intent, &result.CompletedAt)
	if err != nil {
		return nil, err
	}
	return result, nil
}
