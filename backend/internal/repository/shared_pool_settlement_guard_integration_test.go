//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestSharedPoolSettlementWritePassesLegacyRuntimeGuard pins the root-cause fix
// for broken shared-pool accounting: migration 237 fences every legacy runtime
// table behind a transaction-scoped authority epoch, and the runtime never
// declared one, so RecordSharedPoolUsageTx aborted at the usage-window trigger.
// A real settlement must now debit the member and credit the owner wallet.
func TestSharedPoolSettlementWritePassesLegacyRuntimeGuard(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	memberID := mustCreateSharedPoolBillingUser(t, 5)
	poolID := guardTestCreatePool(t, ownerID)
	accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, memberID)

	requestID := fmt.Sprintf("req-guard-%d", time.Now().UnixNano())
	err := repo.RecordSharedPoolUsageTx(ctx, service.SharedPoolUsageInput{
		AccessKeyID:    accessKeyID,
		PoolID:         poolID,
		UserID:         memberID,
		Cost:           0.0002,
		Model:          "guard-probe-model",
		RequestID:      requestID,
		Success:        true,
		PriceVersionID: 1,
		PricingSource:  service.SharedPoolPricingSourceOwner,
		PriceSnapshot:  json.RawMessage(`{"probe":true}`),
	})
	require.NoError(t, err, "settlement must pass the migration-237 runtime guard")

	memberBalance := sharedPoolBillingFloat(t,
		`SELECT balance::double precision FROM users WHERE id = $1`, memberID)
	require.InDelta(t, 5.0-0.0002, memberBalance, 1e-9)

	ownerWallet := sharedPoolBillingFloat(t,
		`SELECT available_amount::double precision FROM shared_pool_owner_wallets WHERE owner_id = $1`, ownerID)
	require.InDelta(t, 0.00018, ownerWallet, 1e-9)

	ledgerCount := sharedPoolBillingInt(t,
		`SELECT COUNT(*) FROM shared_pool_balance_ledger WHERE user_id = $1 AND source_id = $2`,
		memberID, requestID)
	require.Equal(t, 1, ledgerCount)

	usageWindowCount := sharedPoolBillingInt(t,
		`SELECT COUNT(*) FROM shared_pool_model_usage_windows WHERE pool_id = $1`, poolID)
	require.GreaterOrEqual(t, usageWindowCount, 1)
}

// guardTestCreatePool creates a shared pool and removes it under a declared
// authority epoch. Deleting a pool cascades into guarded runtime tables, so the
// shared fixture cleanup would otherwise fail even when the test body passes.
func guardTestCreatePool(t *testing.T, ownerID int64) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
INSERT INTO shared_pools (owner_id, owner_label, name, description, status, listed, max_users, current_users, min_balance_admission, hourly_seat_fee, owner_share_percent)
VALUES ($1, 'guard-test-owner', $2, 'guard integration test', 'healthy', TRUE, 5, 1, 0, 0, 90)
RETURNING id`, ownerID, fmt.Sprintf("Guard Pool %d", time.Now().UnixNano())).Scan(&id))

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		tx, err := integrationDB.BeginTx(cleanupCtx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		var epoch int64
		require.NoError(t, tx.QueryRowContext(cleanupCtx,
			`SELECT authority_epoch FROM shared_pool_cutover_control WHERE singleton = TRUE`).Scan(&epoch))
		_, err = tx.ExecContext(cleanupCtx,
			`SELECT set_config('bizdecipher.shared_pool_authority_epoch', $1, TRUE)`,
			strconv.FormatInt(epoch, 10))
		require.NoError(t, err)

		statements := []string{
			`DELETE FROM shared_pool_balance_ledger WHERE pool_id = $1`,
			`DELETE FROM shared_pool_media_endpoint_probes WHERE pool_id = $1`,
			`DELETE FROM shared_pool_model_usage_windows WHERE pool_id = $1`,
			`DELETE FROM shared_pool_price_versions WHERE pool_id = $1`,
			`DELETE FROM shared_pool_model_endpoints WHERE pool_model_id IN (SELECT id FROM shared_pool_models WHERE pool_id = $1)`,
			`DELETE FROM shared_pool_models WHERE pool_id = $1`,
			`DELETE FROM shared_pool_settlement_rule_versions WHERE pool_id = $1`,
			`DELETE FROM shared_pool_archive_events WHERE pool_id = $1`,
			`DELETE FROM shared_pools WHERE id = $1`,
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(cleanupCtx, statement, id); err != nil {
				t.Logf("guard pool cleanup statement failed: %v", err)
			}
		}
		require.NoError(t, tx.Commit())
	})
	return id
}
