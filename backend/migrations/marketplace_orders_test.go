package migrations_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarketplaceOrdersMigration_tracksLifecycleWithoutPaymentOrCustody(t *testing.T) {
	contents, err := os.ReadFile("256_marketplace_orders.sql")
	require.NoError(t, err)
	sql := string(contents)

	require.Contains(t, sql, "CHECK (status IN ('quoted', 'confirmed', 'delivered', 'accepted', 'settled', 'canceled', 'disputed'))")
	require.Contains(t, sql, "CHECK (buyer_user_id <> seller_user_id)")
	require.Contains(t, sql, "CHECK (revision_limit BETWEEN 0 AND 10)")
	require.Contains(t, sql, "UNIQUE (order_id, author_user_id)")
	require.Contains(t, sql, "marketplace_order_events")
	require.Contains(t, sql, "marketplace_order_reviews")
	require.Contains(t, sql, "-- This migration deliberately creates no payment, custody or frozen-balance state.")
	require.NotContains(t, sql, "CREATE TABLE marketplace_payments")
}

func TestMarketplaceOrderDedupeMigration_preventsDuplicateActiveOrders(t *testing.T) {
	contents, err := os.ReadFile("257_marketplace_order_active_inquiry.sql")
	require.NoError(t, err)
	sql := string(contents)

	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS idx_marketplace_orders_active_inquiry")
	require.Contains(t, sql, "WHERE status <> 'canceled'")
}
