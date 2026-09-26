package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type adminPoolInspectionRepo struct {
	BizDecipherRepository
	sharedPoolPricingRepository
	pool  *SharedPool
	err   error
	calls int
	owner int64
}

func (r *adminPoolInspectionRepo) GetSharedPoolOwnerForAdmin(context.Context, int64) (int64, error) {
	if r.pool == nil || r.pool.OwnerID == nil {
		return 0, r.err
	}
	return *r.pool.OwnerID, r.err
}
func (r *adminPoolInspectionRepo) ListSharedPoolModelEndpointPricing(_ context.Context, _ int64, ownerID int64) ([]SharedPoolModelEndpointPricing, error) {
	r.calls++
	r.owner = ownerID
	return []SharedPoolModelEndpointPricing{{ModelName: "private-model", EndpointType: SharedPoolEndpointChat}}, nil
}

func TestAdminPoolInspectionResolvesActualOwnerForUnlistedPool(t *testing.T) {
	owner := int64(17)
	repo := &adminPoolInspectionRepo{pool: &SharedPool{ID: 3, OwnerID: &owner, Listed: false}}
	items, err := NewBizDecipherService(repo, nil, nil).AdminListSharedPoolEndpointPricing(context.Background(), 3)
	require.NoError(t, err)
	require.Equal(t, owner, repo.owner)
	require.Len(t, items, 1)
	require.Equal(t, "private-model", items[0].ModelName)
}

func TestAdminPoolInspectionDoesNotReadPricesOnMissingOwnerOrLookupFailure(t *testing.T) {
	for _, repo := range []*adminPoolInspectionRepo{
		{pool: &SharedPool{ID: 3}},
		{err: errors.New("database unavailable")},
	} {
		_, err := NewBizDecipherService(repo, nil, nil).AdminListSharedPoolEndpointPricing(context.Background(), 3)
		require.Error(t, err)
		require.Zero(t, repo.calls)
	}
}
