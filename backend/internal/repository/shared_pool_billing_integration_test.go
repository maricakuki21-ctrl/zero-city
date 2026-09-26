//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolBillingRejectsUsageThatWouldMakeBalanceNegative(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 0.40)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 0)
	accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, userID)
	seatID := mustCreateSharedPoolBillingSeat(t, poolID, userID, 1, 0, time.Now())

	err := repo.RecordSharedPoolUsageTx(ctx, service.SharedPoolUsageInput{
		AccessKeyID:   accessKeyID,
		PoolID:        poolID,
		UserID:        userID,
		Cost:          1.00,
		RequestID:     fmt.Sprintf("req-insufficient-%d", time.Now().UnixNano()),
		Success:       true,
		Model:         "gpt-test",
		PriceSnapshot: json.RawMessage(`{}`),
	})
	require.ErrorIs(t, err, service.ErrInsufficientBalance)

	balance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, userID)
	require.InDelta(t, 0.40, balance, 1e-9)

	ledger := sharedPoolBillingLedgerStatus(t, userID, "share_pool_usage")
	require.Equal(t, "reversed", ledger.status)
	require.InDelta(t, 0.0, ledger.amount, 1e-9)

	seatStatus := sharedPoolBillingString(t, `SELECT status FROM pool_seat_bindings WHERE id = $1`, seatID)
	require.Equal(t, "released", seatStatus)

	keyStatus := sharedPoolBillingString(t, `SELECT status FROM shared_pool_access_keys WHERE id = $1`, accessKeyID)
	require.Equal(t, service.StatusDisabled, keyStatus)

	poolUsers := sharedPoolBillingInt(t, `SELECT current_users FROM shared_pools WHERE id = $1`, poolID)
	require.Equal(t, 0, poolUsers)
}

func TestSharedPoolBillingUnreservedRequestIDConflictDoesNotDoubleCharge(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 0)
	accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, userID)
	requestID := fmt.Sprintf("req-idempotent-%d", time.Now().UnixNano())
	input := service.SharedPoolUsageInput{
		AccessKeyID:   accessKeyID,
		PoolID:        poolID,
		UserID:        userID,
		Cost:          1.25,
		RequestID:     requestID,
		Success:       true,
		Model:         "gpt-test",
		PriceSnapshot: json.RawMessage(`{}`),
	}

	require.NoError(t, repo.RecordSharedPoolUsageTx(ctx, input))
	require.ErrorIs(t, repo.RecordSharedPoolUsageTx(ctx, input), service.ErrSharedPoolReservationConflict)

	balance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, userID)
	require.InDelta(t, 8.75, balance, 1e-9)

	ledgerCount := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_balance_ledger WHERE user_id = $1 AND source_type = 'share_pool_usage' AND source_id = $2`, userID, requestID)
	require.Equal(t, 1, ledgerCount)
	usageDestination := sharedPoolBillingString(t, `SELECT settlement_destination FROM shared_pool_balance_ledger WHERE user_id = $1 AND source_type = 'share_pool_usage' AND source_id = $2`, userID, requestID)
	require.Equal(t, "user_balance", usageDestination)

	totalUsed := sharedPoolBillingFloat(t, `SELECT total_used::double precision FROM shared_pool_access_keys WHERE id = $1`, accessKeyID)
	require.InDelta(t, 1.25, totalUsed, 1e-9)

	poolCalls := sharedPoolBillingInt(t, `SELECT total_calls FROM shared_pools WHERE id = $1`, poolID)
	require.Equal(t, 1, poolCalls)
}

func TestSharedPoolAccessKeyRequiresActiveSeat(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 0, 0)

	result, err := repo.CreateSharedPoolAccessKeyTx(ctx, poolID, userID, "seat required", fmt.Sprintf("sk-share-seat-%d", time.Now().UnixNano()))
	require.ErrorIs(t, err, service.ErrPoolSeatRequired)
	require.Nil(t, result)

	keyCount := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_access_keys WHERE pool_id = $1 AND user_id = $2`, poolID, userID)
	require.Equal(t, 0, keyCount)
}

func TestSharedPoolBillingRecordsUsageLedgerSplitAndOwnerPayout(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 2)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 0)
	accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, userID)
	requestID := fmt.Sprintf("req-ledger-split-%d", time.Now().UnixNano())

	require.NoError(t, repo.RecordSharedPoolUsageTx(ctx, service.SharedPoolUsageInput{
		AccessKeyID:   accessKeyID,
		PoolID:        poolID,
		UserID:        userID,
		Cost:          2.00,
		RequestID:     requestID,
		Success:       true,
		Model:         "gpt-test",
		PriceSnapshot: json.RawMessage(`{}`),
	}))

	userBalance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, userID)
	require.InDelta(t, 8.0, userBalance, 1e-9)
	ownerBalance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, ownerID)
	require.InDelta(t, 2.0, ownerBalance, 1e-9)
	ownerWallet := sharedPoolBillingFloat(t, `SELECT available_amount::double precision FROM shared_pool_owner_wallets WHERE owner_id = $1`, ownerID)
	require.InDelta(t, 1.8, ownerWallet, 1e-9)

	var amount, platformFee, ownerPayout float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT amount::double precision, platform_fee_amount::double precision, owner_payout_amount::double precision
		FROM shared_pool_balance_ledger
		WHERE user_id = $1 AND pool_id = $2 AND source_type = 'share_pool_usage' AND source_id = $3`, userID, poolID, requestID).
		Scan(&amount, &platformFee, &ownerPayout))
	require.InDelta(t, -2.0, amount, 1e-9)
	require.InDelta(t, 0.2, platformFee, 1e-9)
	require.InDelta(t, 1.8, ownerPayout, 1e-9)

	payout := sharedPoolBillingFloat(t, `SELECT amount::double precision FROM shared_pool_balance_ledger
		WHERE user_id = $1 AND pool_id = $2 AND source_type = 'share_pool_payout' AND source_id = $3`, ownerID, poolID, requestID)
	require.InDelta(t, 1.8, payout, 1e-9)
}

func TestSharedPoolBillingUsesEffectivePlatformFeeVersion(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)
	now := time.Now().UTC()

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 0)
	accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, userID)

	_, err := integrationDB.ExecContext(ctx, `INSERT INTO shared_pool_settlement_rule_versions
		(pool_id, hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, effective_from, source)
		VALUES
			($1, 0, 0, 20, $2, 'admin_governance'),
			($1, 0, 0, 35, $3, 'admin_governance')`,
		poolID, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)

	require.NoError(t, repo.RecordSharedPoolUsageTx(ctx, service.SharedPoolUsageInput{
		AccessKeyID:   accessKeyID,
		PoolID:        poolID,
		UserID:        userID,
		Cost:          2.00,
		RequestID:     fmt.Sprintf("req-versioned-fee-%d", time.Now().UnixNano()),
		Success:       true,
		Model:         "gpt-test",
		PriceSnapshot: json.RawMessage(`{}`),
	}))

	ownerBalance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, ownerID)
	require.InDelta(t, 0.0, ownerBalance, 1e-9)
	ownerWallet := sharedPoolBillingFloat(t, `SELECT available_amount::double precision FROM shared_pool_owner_wallets WHERE owner_id = $1`, ownerID)
	require.InDelta(t, 1.60, ownerWallet, 1e-9)
	var platformFee, ownerPayout float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT platform_fee_amount::double precision, owner_payout_amount::double precision
		FROM shared_pool_balance_ledger
		WHERE user_id = $1 AND pool_id = $2 AND source_type = 'share_pool_usage'
		ORDER BY id DESC LIMIT 1`, userID, poolID).Scan(&platformFee, &ownerPayout))
	require.InDelta(t, 0.40, platformFee, 1e-9)
	require.InDelta(t, 1.60, ownerPayout, 1e-9)
}

func TestSharedPoolBillingAppliesRuleVersionAtEachBillingHour(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Hour)
	periodStart := now.Add(-2 * time.Hour)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 9)
	seatID := mustCreateSharedPoolBillingSeat(t, poolID, userID, 9, 0, periodStart)

	_, err := integrationDB.ExecContext(ctx, `INSERT INTO shared_pool_settlement_rule_versions
		(pool_id, hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, effective_from, source)
		VALUES
			($1, 1, 0, 10, $2, 'owner_update'),
			($1, 2, 0, 10, $3, 'owner_update')`,
		poolID, periodStart, periodStart.Add(time.Hour))
	require.NoError(t, err)

	summary, err := repo.ChargeDueSeats(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 2, summary.HoursCharged)
	require.InDelta(t, 3.0, summary.TotalCharged, 1e-9)

	var firstAmount, secondAmount float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT amount::double precision FROM pool_seat_charges WHERE seat_id = $1 AND billing_hour = $2`, seatID, periodStart).Scan(&firstAmount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT amount::double precision FROM pool_seat_charges WHERE seat_id = $1 AND billing_hour = $2`, seatID, periodStart.Add(time.Hour)).Scan(&secondAmount))
	require.InDelta(t, 1.0, firstAmount, 1e-9)
	require.InDelta(t, 2.0, secondAmount, 1e-9)
}

func TestSharedPoolBillingRecordsUsageWindowsWithPublishedModel(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 0)
	accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, userID)
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO shared_pool_models (pool_id, model_name, display_name, upstream_model_name, model_aliases, sort_order, enabled, model_open, rate_multiplier)
		VALUES ($1, 'published-model', 'published-model', 'upstream-billing-model', '["published-alias"]'::jsonb, 1, TRUE, TRUE, 1)
		ON CONFLICT (pool_id, model_name) DO UPDATE SET upstream_model_name = EXCLUDED.upstream_model_name, model_aliases = EXCLUDED.model_aliases, enabled = TRUE, model_open = TRUE`, poolID)
	require.NoError(t, err)

	require.NoError(t, repo.RecordSharedPoolUsageTx(ctx, service.SharedPoolUsageInput{
		AccessKeyID:   accessKeyID,
		PoolID:        poolID,
		UserID:        userID,
		Cost:          1.00,
		RequestID:     fmt.Sprintf("req-published-window-%d", time.Now().UnixNano()),
		Success:       true,
		Model:         "upstream-billing-model",
		PriceSnapshot: json.RawMessage(`{}`),
	}))

	canonicalRows := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_model_usage_windows WHERE pool_id = $1 AND model_name = 'published-model'`, poolID)
	require.Equal(t, 3, canonicalRows)
	upstreamRows := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_model_usage_windows WHERE pool_id = $1 AND model_name = 'upstream-billing-model'`, poolID)
	require.Equal(t, 0, upstreamRows)
}

func TestSharedPoolBillingAppliesContinuousUsageWaiver(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Hour)
	periodStart := now.Add(-time.Hour)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 1.00)
	seatID := mustCreateSharedPoolBillingSeat(t, poolID, userID, 1.00, 2.00, periodStart)

	_, err := integrationDB.ExecContext(ctx, `INSERT INTO shared_pool_balance_ledger
		(user_id, pool_id, source_type, source_id, amount, balance_after, status, note, posted_at, created_at)
		VALUES ($1, $2, 'share_pool_usage', $3, -1, 9, 'posted', 'waiver usage', $4, $4)`,
		userID, poolID, fmt.Sprintf("req-waiver-%d", time.Now().UnixNano()), periodStart.Add(30*time.Minute))
	require.NoError(t, err)

	summary, err := repo.ChargeDueSeats(ctx, now)
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Equal(t, 1, summary.HoursCharged)
	require.InDelta(t, 0.50, summary.TotalCharged, 1e-9)

	userBalance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, userID)
	require.InDelta(t, 9.50, userBalance, 1e-9)
	ownerBalance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, ownerID)
	require.InDelta(t, 0.0, ownerBalance, 1e-9)
	ownerWallet := sharedPoolBillingFloat(t, `SELECT available_amount::double precision FROM shared_pool_owner_wallets WHERE owner_id = $1`, ownerID)
	require.InDelta(t, 0.50, ownerWallet, 1e-9)

	var amount, usageAmount, waivedAmount, threshold float64
	var waiverApplied bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT amount::double precision, usage_amount::double precision,
		waived_amount::double precision, waiver_threshold::double precision, waiver_applied
		FROM pool_seat_charges WHERE seat_id = $1`, seatID).
		Scan(&amount, &usageAmount, &waivedAmount, &threshold, &waiverApplied))
	require.InDelta(t, 0.50, amount, 1e-9)
	require.InDelta(t, 1.00, usageAmount, 1e-9)
	require.InDelta(t, 0.50, waivedAmount, 1e-9)
	require.InDelta(t, 2.00, threshold, 1e-9)
	require.True(t, waiverApplied)
}

func TestSharedPoolBillingReleasesSeatWhenHourlyFeeWouldMakeBalanceNegative(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Hour)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 0.50)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 1)
	seatID := mustCreateSharedPoolBillingSeat(t, poolID, userID, 1, 0, now.Add(-2*time.Hour))

	summary, err := repo.ChargeDueSeats(ctx, now)
	require.NoError(t, err)
	require.NotNil(t, summary)

	balance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, userID)
	require.InDelta(t, 0.50, balance, 1e-9)

	seatStatus := sharedPoolBillingString(t, `SELECT status FROM pool_seat_bindings WHERE id = $1`, seatID)
	require.Equal(t, "released", seatStatus)

	poolUsers := sharedPoolBillingInt(t, `SELECT current_users FROM shared_pools WHERE id = $1`, poolID)
	require.Equal(t, 0, poolUsers)

	ledger := sharedPoolBillingLedgerStatus(t, userID, "pool_seat_fee")
	require.Equal(t, "reversed", ledger.status)
	require.InDelta(t, 0.0, ledger.amount, 1e-9)

	chargeAmount := sharedPoolBillingFloat(t, `SELECT amount::double precision FROM pool_seat_charges WHERE seat_id = $1 LIMIT 1`, seatID)
	require.InDelta(t, 0.0, chargeAmount, 1e-9)
}

func TestSharedPoolBillingReleasesNonServiceablePoolSeatWithoutCharging(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	cases := []struct {
		name       string
		updatePool string
	}{
		{name: "banned governance", updatePool: `UPDATE shared_pools SET governance_status = 'banned' WHERE id = $1`},
		{name: "unlisted", updatePool: `UPDATE shared_pools SET listed = FALSE WHERE id = $1`},
		{name: "offline status", updatePool: `UPDATE shared_pools SET status = 'offline' WHERE id = $1`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Hour)

			ownerID := mustCreateSharedPoolBillingUser(t, 2.75)
			userID := mustCreateSharedPoolBillingUser(t, 10)
			poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 1.50)
			accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, userID)
			seatID := mustCreateSharedPoolBillingSeat(t, poolID, userID, 1.50, 0, now.Add(-2*time.Hour))

			var apiKeyID int64
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT api_key_id FROM shared_pool_access_keys WHERE id = $1`, accessKeyID).Scan(&apiKeyID))
			_, err := integrationDB.ExecContext(ctx, tc.updatePool, poolID)
			require.NoError(t, err)

			summary, err := repo.ChargeDueSeats(ctx, now)
			require.NoError(t, err)
			require.NotNil(t, summary)
			require.Equal(t, 0, summary.SeatsProcessed)
			require.Equal(t, 0, summary.HoursCharged)
			require.InDelta(t, 0.0, summary.TotalCharged, 1e-9)

			userBalance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, userID)
			require.InDelta(t, 10.0, userBalance, 1e-9)
			ownerBalance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, ownerID)
			require.InDelta(t, 2.75, ownerBalance, 1e-9)

			seatStatus := sharedPoolBillingString(t, `SELECT status FROM pool_seat_bindings WHERE id = $1`, seatID)
			require.Equal(t, "released", seatStatus)
			poolUsers := sharedPoolBillingInt(t, `SELECT current_users FROM shared_pools WHERE id = $1`, poolID)
			require.Equal(t, 0, poolUsers)
			accessKeyStatus := sharedPoolBillingString(t, `SELECT status FROM shared_pool_access_keys WHERE id = $1`, accessKeyID)
			require.Equal(t, service.StatusDisabled, accessKeyStatus)
			apiKeyStatus := sharedPoolBillingString(t, `SELECT status FROM api_keys WHERE id = $1`, apiKeyID)
			require.Equal(t, service.StatusDisabled, apiKeyStatus)

			chargeCount := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM pool_seat_charges WHERE seat_id = $1`, seatID)
			require.Equal(t, 0, chargeCount)
			feeLedgerCount := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_balance_ledger WHERE pool_id = $1 AND source_type = 'pool_seat_fee'`, poolID)
			require.Equal(t, 0, feeLedgerCount)
			payoutLedgerCount := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_balance_ledger WHERE pool_id = $1 AND source_type = 'pool_owner_payout'`, poolID)
			require.Equal(t, 0, payoutLedgerCount)
		})
	}
}

func TestSharedPoolBillingChargeDueSeatsIsIdempotentPerBillingHour(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Hour)
	lastCharged := now.Add(-time.Hour)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 1.50)
	seatID := mustCreateSharedPoolBillingSeat(t, poolID, userID, 1.50, 0, lastCharged)

	first, err := repo.ChargeDueSeats(ctx, now)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, 1, first.SeatsProcessed)
	require.Equal(t, 1, first.HoursCharged)
	require.InDelta(t, 1.50, first.TotalCharged, 1e-9)

	balanceAfterFirst := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, userID)
	require.InDelta(t, 8.50, balanceAfterFirst, 1e-9)
	ownerBalanceAfterFirst := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, ownerID)
	require.InDelta(t, 0.0, ownerBalanceAfterFirst, 1e-9)
	ownerWalletAfterFirst := sharedPoolBillingFloat(t, `SELECT available_amount::double precision FROM shared_pool_owner_wallets WHERE owner_id = $1`, ownerID)
	require.InDelta(t, 1.50, ownerWalletAfterFirst, 1e-9)

	chargeCountAfterFirst := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM pool_seat_charges WHERE seat_id = $1`, seatID)
	require.Equal(t, 1, chargeCountAfterFirst)
	feeLedgerCountAfterFirst := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_balance_ledger WHERE user_id = $1 AND source_type = 'pool_seat_fee'`, userID)
	require.Equal(t, 1, feeLedgerCountAfterFirst)
	feeDestination := sharedPoolBillingString(t, `SELECT settlement_destination FROM shared_pool_balance_ledger WHERE user_id = $1 AND source_type = 'pool_seat_fee' ORDER BY id DESC LIMIT 1`, userID)
	require.Equal(t, "user_balance", feeDestination)
	payoutLedgerCountAfterFirst := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_balance_ledger WHERE user_id = $1 AND source_type = 'pool_owner_payout'`, ownerID)
	require.Equal(t, 1, payoutLedgerCountAfterFirst)
	totalChargedAfterFirst := sharedPoolBillingFloat(t, `SELECT total_charged::double precision FROM pool_seat_bindings WHERE id = $1`, seatID)
	require.InDelta(t, 1.50, totalChargedAfterFirst, 1e-9)

	_, err = integrationDB.ExecContext(ctx, `UPDATE pool_seat_bindings SET last_charged_at = $2 WHERE id = $1`, seatID, lastCharged)
	require.NoError(t, err)

	second, err := repo.ChargeDueSeats(ctx, now)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, 1, second.SeatsProcessed)
	require.Equal(t, 1, second.HoursCharged)
	require.InDelta(t, 0.0, second.TotalCharged, 1e-9)

	balanceAfterSecond := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, userID)
	require.InDelta(t, balanceAfterFirst, balanceAfterSecond, 1e-9)
	ownerBalanceAfterSecond := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, ownerID)
	require.InDelta(t, ownerBalanceAfterFirst, ownerBalanceAfterSecond, 1e-9)
	ownerWalletAfterSecond := sharedPoolBillingFloat(t, `SELECT available_amount::double precision FROM shared_pool_owner_wallets WHERE owner_id = $1`, ownerID)
	require.InDelta(t, ownerWalletAfterFirst, ownerWalletAfterSecond, 1e-9)

	chargeCountAfterSecond := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM pool_seat_charges WHERE seat_id = $1`, seatID)
	require.Equal(t, chargeCountAfterFirst, chargeCountAfterSecond)
	feeLedgerCountAfterSecond := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_balance_ledger WHERE user_id = $1 AND source_type = 'pool_seat_fee'`, userID)
	require.Equal(t, feeLedgerCountAfterFirst, feeLedgerCountAfterSecond)
	payoutLedgerCountAfterSecond := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_balance_ledger WHERE user_id = $1 AND source_type = 'pool_owner_payout'`, ownerID)
	require.Equal(t, payoutLedgerCountAfterFirst, payoutLedgerCountAfterSecond)
	totalChargedAfterSecond := sharedPoolBillingFloat(t, `SELECT total_charged::double precision FROM pool_seat_bindings WHERE id = $1`, seatID)
	require.InDelta(t, totalChargedAfterFirst, totalChargedAfterSecond, 1e-9)
}

type sharedPoolBillingLedgerRow struct {
	status string
	amount float64
}

func TestSharedPoolBillingReleasesIdleSeatAfterTwoHours(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)
	now := time.Now().UTC()
	idleAt := now.Add(-2*time.Hour - time.Minute)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 1.00)
	accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, userID)
	seatID := mustCreateSharedPoolBillingSeat(t, poolID, userID, 1.00, 0, idleAt)

	// Keep activity clock old enough while last_charged is also overdue.
	_, err := integrationDB.ExecContext(ctx, `UPDATE pool_seat_bindings SET last_activity_at = $2, last_charged_at = $2 WHERE id = $1`, seatID, idleAt)
	require.NoError(t, err)

	summary, err := repo.ReleaseIdleSharedPoolSeats(ctx, now)
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Equal(t, 1, summary.IdleSeatsReleased)

	seatStatus := sharedPoolBillingString(t, `SELECT status FROM pool_seat_bindings WHERE id = $1`, seatID)
	require.Equal(t, "released", seatStatus)

	poolUsers := sharedPoolBillingInt(t, `SELECT current_users FROM shared_pools WHERE id = $1`, poolID)
	require.Equal(t, 0, poolUsers)

	accessKeyStatus := sharedPoolBillingString(t, `SELECT status FROM shared_pool_access_keys WHERE id = $1`, accessKeyID)
	require.Equal(t, service.StatusDisabled, accessKeyStatus)

	// Fresh seat must not be released.
	activeUserID := mustCreateSharedPoolBillingUser(t, 10)
	activeSeatID := mustCreateSharedPoolBillingSeat(t, poolID, activeUserID, 1.00, 0, now)
	_, err = integrationDB.ExecContext(ctx, `UPDATE shared_pools SET current_users = 1 WHERE id = $1`, poolID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE pool_seat_bindings SET last_activity_at = $2 WHERE id = $1`, activeSeatID, now.Add(-30*time.Minute))
	require.NoError(t, err)

	second, err := repo.ReleaseIdleSharedPoolSeats(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 0, second.IdleSeatsReleased)
	activeStatus := sharedPoolBillingString(t, `SELECT status FROM pool_seat_bindings WHERE id = $1`, activeSeatID)
	require.Equal(t, "active", activeStatus)
}

func TestLeaveSharedPoolKeepsHistoryAndRemovesActiveSeat(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 0)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 0)
	accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, userID)
	seatID := mustCreateSharedPoolBillingSeat(t, poolID, userID, 0, 0, time.Now())

	require.NoError(t, repo.LeaveSharedPoolTx(ctx, poolID, userID))
	require.NoError(t, repo.LeaveSharedPoolTx(ctx, poolID, userID))

	require.Equal(t, "released", sharedPoolBillingString(t, `SELECT status FROM pool_seat_bindings WHERE id = $1`, seatID))
	require.Equal(t, "user_leave", sharedPoolBillingString(t, `SELECT release_reason FROM pool_seat_bindings WHERE id = $1`, seatID))
	require.Equal(t, service.StatusDisabled, sharedPoolBillingString(t, `SELECT status FROM shared_pool_access_keys WHERE id = $1`, accessKeyID))
	require.Equal(t, 0, sharedPoolBillingInt(t, `SELECT current_users FROM shared_pools WHERE id = $1`, poolID))

	active, err := repo.ListSharedPoolMembers(ctx, poolID, ownerID, "active")
	require.NoError(t, err)
	require.Empty(t, active)
	history, err := repo.ListSharedPoolMembers(ctx, poolID, ownerID, "released")
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, seatID, history[0].ID)
	require.NotNil(t, history[0].ReleasedAt)
	require.Equal(t, "user_leave", history[0].ReleaseReason)
}

func TestSharedPoolOwnerWalletTransferIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := NewBizDecipherRepository(integrationDB)

	ownerID := mustCreateSharedPoolBillingUser(t, 2)
	userID := mustCreateSharedPoolBillingUser(t, 10)
	poolID := mustCreateSharedPoolBillingPool(t, ownerID, 1, 0)
	accessKeyID := mustCreateSharedPoolBillingAccessKey(t, poolID, userID)
	requestID := fmt.Sprintf("req-wallet-transfer-%d", time.Now().UnixNano())
	require.NoError(t, repo.RecordSharedPoolUsageTx(ctx, service.SharedPoolUsageInput{
		AccessKeyID:   accessKeyID,
		PoolID:        poolID,
		UserID:        userID,
		Cost:          2,
		RequestID:     requestID,
		Success:       true,
		Model:         "gpt-test",
		PriceSnapshot: json.RawMessage(`{}`),
	}))

	operationID := fmt.Sprintf("wallet-transfer-%d", time.Now().UnixNano())
	first, err := repo.TransferSharedPoolOwnerEarningsTx(ctx, ownerID, 1.25, operationID)
	require.NoError(t, err)
	require.False(t, first.AlreadyDone)
	require.InDelta(t, 0.55, first.WalletAfter, 1e-9)
	require.InDelta(t, 3.25, first.BalanceAfter, 1e-9)

	second, err := repo.TransferSharedPoolOwnerEarningsTx(ctx, ownerID, 1.25, operationID)
	require.NoError(t, err)
	require.True(t, second.AlreadyDone)
	require.InDelta(t, first.WalletAfter, second.WalletAfter, 1e-9)
	require.InDelta(t, first.BalanceAfter, second.BalanceAfter, 1e-9)

	ownerBalance := sharedPoolBillingFloat(t, `SELECT balance::double precision FROM users WHERE id = $1`, ownerID)
	require.InDelta(t, 3.25, ownerBalance, 1e-9)
	ownerWallet := sharedPoolBillingFloat(t, `SELECT available_amount::double precision FROM shared_pool_owner_wallets WHERE owner_id = $1`, ownerID)
	require.InDelta(t, 0.55, ownerWallet, 1e-9)
	transferEvents := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM shared_pool_owner_earnings_ledger WHERE owner_id = $1 AND event_type = 'transfer_to_balance' AND operation_id = $2`, ownerID, operationID)
	require.Equal(t, 1, transferEvents)
	balanceEvents := sharedPoolBillingInt(t, `SELECT COUNT(*) FROM user_balance_ledger WHERE user_id = $1 AND source_type = 'shared_pool_owner_wallet_transfer' AND source_id = $2`, ownerID, operationID)
	require.Equal(t, 1, balanceEvents)
}

func mustCreateSharedPoolBillingUser(t *testing.T, balance float64) int64 {
	t.Helper()
	var id int64
	email := fmt.Sprintf("shared-pool-billing-%d@example.com", time.Now().UnixNano())
	err := integrationDB.QueryRowContext(context.Background(), `
INSERT INTO users (email, password_hash, role, balance, concurrency, status)
VALUES ($1, 'hash', 'user', $2, 5, 'active')
RETURNING id`, email, balance).Scan(&id)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM shared_pool_owner_earnings_ledger WHERE owner_id = $1`, id)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM shared_pool_owner_wallets WHERE owner_id = $1`, id)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func mustCreateSharedPoolBillingPool(t *testing.T, ownerID int64, currentUsers int, hourlySeatFee float64) int64 {
	t.Helper()
	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
INSERT INTO shared_pools (owner_id, owner_label, name, description, status, listed, max_users, current_users, min_balance_admission, hourly_seat_fee, owner_share_percent)
VALUES ($1, 'billing-test-owner', $2, 'billing integration test', 'healthy', TRUE, 20, $3, 0, $4, 90)
RETURNING id`, ownerID, fmt.Sprintf("Billing Pool %d", time.Now().UnixNano()), currentUsers, hourlySeatFee).Scan(&id)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		tx, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		var epoch int64
		require.NoError(t, tx.QueryRowContext(ctx,
			`SELECT authority_epoch FROM shared_pool_cutover_control WHERE singleton = TRUE`).Scan(&epoch))
		_, err = tx.ExecContext(ctx,
			`SELECT set_config('bizdecipher.shared_pool_authority_epoch', $1, TRUE)`,
			fmt.Sprintf("%d", epoch))
		require.NoError(t, err)

		statements := []string{
			`DELETE FROM shared_pool_balance_ledger WHERE pool_id = $1`,
			`DELETE FROM shared_pool_media_endpoint_probes WHERE pool_id = $1`,
			`DELETE FROM shared_pool_model_usage_windows WHERE pool_id = $1`,
			`DELETE FROM shared_pool_usage_traces WHERE pool_id = $1`,
			`DELETE FROM shared_pool_probe_job_items WHERE job_id IN (SELECT id FROM shared_pool_probe_jobs WHERE pool_id = $1)`,
			`DELETE FROM shared_pool_probe_jobs WHERE pool_id = $1`,
			`DELETE FROM shared_pool_probe_histories WHERE pool_id = $1`,
			`DELETE FROM shared_pool_model_endpoints WHERE pool_model_id IN (SELECT id FROM shared_pool_models WHERE pool_id = $1)`,
			`DELETE FROM shared_pool_models WHERE pool_id = $1`,
			`DELETE FROM shared_pool_settlement_rule_versions WHERE pool_id = $1`,
			`DELETE FROM shared_pool_archive_events WHERE pool_id = $1`,
			`DELETE FROM shared_pools WHERE id = $1`,
		}
		for _, statement := range statements {
			_, err = tx.ExecContext(ctx, statement, id)
			require.NoError(t, err)
		}
		require.NoError(t, tx.Commit())
	})
	return id
}

func mustCreateSharedPoolBillingAccessKey(t *testing.T, poolID, userID int64) int64 {
	t.Helper()
	ctx := context.Background()
	var apiKeyID int64
	apiKey := fmt.Sprintf("sk-share-test-%d", time.Now().UnixNano())
	err := integrationDB.QueryRowContext(ctx, `
INSERT INTO api_keys (user_id, key, name, status)
VALUES ($1, $2, 'shared pool billing test', 'active')
RETURNING id`, userID, apiKey).Scan(&apiKeyID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM api_keys WHERE id = $1`, apiKeyID)
	})

	var accessKeyID int64
	err = integrationDB.QueryRowContext(ctx, `
INSERT INTO shared_pool_access_keys (pool_id, user_id, api_key_id, name, status, allowed_models)
VALUES ($1, $2, $3, 'billing test access key', 'active', '["gpt-test"]'::jsonb)
RETURNING id`, poolID, userID, apiKeyID).Scan(&accessKeyID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM shared_pool_access_keys WHERE id = $1`, accessKeyID)
	})
	return accessKeyID
}

func mustCreateSharedPoolBillingSeat(t *testing.T, poolID, userID int64, hourlyFee, waiver float64, lastChargedAt time.Time) int64 {
	t.Helper()
	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
INSERT INTO pool_seat_bindings (pool_id, user_id, status, hourly_seat_fee, hourly_min_usage_waiver, joined_at, last_charged_at, last_activity_at)
VALUES ($1, $2, 'active', $3, $4, NOW(), $5, $5)
RETURNING id`, poolID, userID, hourlyFee, waiver, lastChargedAt).Scan(&id)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM pool_seat_charges WHERE seat_id = $1`, id)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM pool_seat_bindings WHERE id = $1`, id)
	})
	return id
}

func sharedPoolBillingLedgerStatus(t *testing.T, userID int64, sourceType string) sharedPoolBillingLedgerRow {
	t.Helper()
	var row sharedPoolBillingLedgerRow
	err := integrationDB.QueryRowContext(context.Background(), `
SELECT status, amount::double precision
FROM shared_pool_balance_ledger
WHERE user_id = $1 AND source_type = $2
ORDER BY created_at DESC
LIMIT 1`, userID, sourceType).Scan(&row.status, &row.amount)
	require.NoError(t, err)
	return row
}

func sharedPoolBillingFloat(t *testing.T, query string, args ...any) float64 {
	t.Helper()
	var value float64
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), query, args...).Scan(&value))
	return value
}

func sharedPoolBillingInt(t *testing.T, query string, args ...any) int {
	t.Helper()
	var value int
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), query, args...).Scan(&value))
	return value
}

func sharedPoolBillingString(t *testing.T, query string, args ...any) string {
	t.Helper()
	var value string
	err := integrationDB.QueryRowContext(context.Background(), query, args...).Scan(&value)
	if errors.Is(err, context.Canceled) {
		t.Fatalf("query canceled: %v", err)
	}
	require.NoError(t, err)
	return value
}
