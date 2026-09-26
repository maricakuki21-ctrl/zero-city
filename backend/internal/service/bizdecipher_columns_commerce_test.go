package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestColumnCommerceExactAmounts(t *testing.T) {
	for _, value := range []string{"-1", "1e2", "NaN", "0", "0.000000001", "1000000000000", " 1", "01", "1."} {
		_, err := NormalizeColumnPrice(value, false)
		require.ErrorIs(t, err, ErrColumnCommerceInvalid, value)
	}
	value, err := NormalizeColumnPrice("10.12345678", false)
	require.NoError(t, err)
	require.Equal(t, "10.12345678", value)
	in, err := NormalizeColumnPurchase(ColumnPurchaseInput{OperationID: "stable_operation_123", ExpectedPrice: "1.2"})
	require.NoError(t, err)
	require.Equal(t, "1.20000000", in.ExpectedPrice)
	_, err = NormalizeColumnPricing(ColumnPricingInput{Mode: "free", Price: "1"})
	require.ErrorIs(t, err, ErrColumnCommerceInvalid)
	_, err = NormalizeColumnRefund(ColumnRefundInput{OperationID: "stable_operation_123", Reason: " "})
	require.ErrorIs(t, err, ErrColumnCommerceInvalid)
}

type columnCommerceServiceRepo struct {
	BizDecipherRepository
	columnCommerceRepository
	calls int
}

func (r *columnCommerceServiceRepo) PurchaseColumn(context.Context, int64, int64, ColumnPurchaseInput) (*ColumnPurchase, error) {
	r.calls++
	return &ColumnPurchase{ID: 1}, nil
}

type columnCommerceCacheStub struct {
	calls  int
	failAt int
}

func (c *columnCommerceCacheStub) InvalidateUserBalance(context.Context, int64) error {
	c.calls++
	if c.calls == c.failAt {
		return errors.New("cache down")
	}
	return nil
}
func (*columnCommerceCacheStub) InvalidateAPIKeyRateLimit(context.Context, int64) error { return nil }

type columnCommerceAuthRepo struct {
	APIKeyRepository
	calls int
}

func (r *columnCommerceAuthRepo) ListKeysByUserID(context.Context, int64) ([]string, error) {
	r.calls++
	return nil, nil
}
func TestColumnCommerceBalanceCacheFailureAndRetry(t *testing.T) {
	r := &columnCommerceServiceRepo{}
	cache := &columnCommerceCacheStub{failAt: 1}
	auth := &columnCommerceAuthRepo{}
	s := NewBizDecipherService(r, &APIKeyService{rateLimitCacheInvalid: cache, apiKeyRepo: auth}, nil)
	_, err := s.PurchaseColumn(context.Background(), 1, 2, ColumnPurchaseInput{})
	require.ErrorIs(t, err, ErrCreatorColumnUnavailable)
	require.Zero(t, r.calls)
	cache.calls = 0
	cache.failAt = 2
	_, err = s.PurchaseColumn(context.Background(), 1, 2, ColumnPurchaseInput{})
	require.ErrorIs(t, err, ErrCreatorColumnUnavailable)
	require.Equal(t, 1, r.calls)
	cache.failAt = 0
	_, err = s.PurchaseColumn(context.Background(), 1, 2, ColumnPurchaseInput{})
	require.NoError(t, err)
	require.Equal(t, 2, r.calls)
	require.Equal(t, 3, auth.calls)
}
