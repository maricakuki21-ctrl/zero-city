package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMatchSharedPoolPriceSnapshotKnownProviderNeverFallsBackToBareModel(t *testing.T) {
	t.Parallel()

	otherVendorPrice := 9.9
	otherVendor := &service.SharedPoolPriceSnapshot{BillingMode: "per_request", PerRequestPrice: &otherVendorPrice}
	priceMap := map[string]*service.SharedPoolPriceSnapshot{
		"same-model": otherVendor,
	}

	got := matchSharedPoolPriceSnapshot(priceMap, "vendor-b", "same-model", nil)
	require.Nil(t, got)
}

func TestMatchSharedPoolPriceSnapshotScopesAliasesToKnownProvider(t *testing.T) {
	t.Parallel()

	vendorAPrice := 1.1
	vendorBPrice := 2.2
	vendorA := &service.SharedPoolPriceSnapshot{BillingMode: "per_request", PerRequestPrice: &vendorAPrice}
	vendorB := &service.SharedPoolPriceSnapshot{BillingMode: "per_request", PerRequestPrice: &vendorBPrice}
	priceMap := map[string]*service.SharedPoolPriceSnapshot{
		"shared-alias":          vendorA,
		"vendor-b:shared-alias": vendorB,
	}

	got := matchSharedPoolPriceSnapshot(priceMap, " Vendor-B ", "missing-primary", []string{" Shared-Alias "})
	require.NotNil(t, got)
	require.NotNil(t, got.PerRequestPrice)
	require.InDelta(t, 2.2, *got.PerRequestPrice, 1e-12)
	require.NotSame(t, vendorB, got)
}

func TestMatchSharedPoolPriceSnapshotAllowsBareKeyOnlyWithoutProvider(t *testing.T) {
	t.Parallel()

	legacyPrice := 3.3
	legacy := &service.SharedPoolPriceSnapshot{BillingMode: "per_request", PerRequestPrice: &legacyPrice}
	priceMap := map[string]*service.SharedPoolPriceSnapshot{
		"legacy-model": legacy,
	}

	got := matchSharedPoolPriceSnapshot(priceMap, "", "legacy-model", nil)
	require.NotNil(t, got)
	require.NotNil(t, got.PerRequestPrice)
	require.InDelta(t, 3.3, *got.PerRequestPrice, 1e-12)
	require.NotSame(t, legacy, got)
}
