package service

import (
	"context"
	"errors"
)

type sharedPoolAdminInspectionRepository interface {
	GetSharedPoolOwnerForAdmin(ctx context.Context, poolID int64) (int64, error)
}

// AdminListSharedPoolEndpointPricing is used only by the admin-authenticated
// route. It resolves the owner server-side without accepting a viewer override.
func (s *BizDecipherService) AdminListSharedPoolEndpointPricing(ctx context.Context, poolID int64) ([]SharedPoolModelEndpointPricing, error) {
	if s == nil || s.repo == nil || poolID <= 0 {
		return nil, errors.New("invalid shared pool inspection request")
	}
	repo, ok := s.repo.(sharedPoolAdminInspectionRepository)
	if !ok {
		return nil, errors.New("shared pool inspection unavailable")
	}
	ownerID, err := repo.GetSharedPoolOwnerForAdmin(ctx, poolID)
	if err != nil {
		return nil, err
	}
	if ownerID <= 0 {
		return nil, ErrPoolForbidden
	}
	return s.ListSharedPoolModelEndpointPricing(ctx, poolID, ownerID)
}
