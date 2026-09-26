package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestWithdrawalPostgresConcurrentDebitAndRefund(t *testing.T) {
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	var name string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&name))
	require.True(t, strings.HasPrefix(name, "bizdecipher_columns_acceptance_test_") || strings.HasPrefix(name, "bizdecipher_withdrawal_test_"), "requires a disposable acceptance database")
	require.NoError(t, ApplyMigrations(ctx, db))
	owner := insertMarketplaceTestUser(t, db, fmt.Sprintf("withdrawal-%d@example.test", time.Now().UnixNano()))
	r := &bizDecipherRepository{db: db}
	_, err = db.ExecContext(ctx, `INSERT INTO shared_pool_owner_wallets(owner_id,available_amount) VALUES($1,1)`, owner)
	require.NoError(t, err)
	require.NoError(t, r.SaveWithdrawalPolicy(ctx, owner, service.WithdrawalPolicy{Enabled: true, Channels: []string{"test-bank"}, Instructions: "test only"}))
	in := service.WithdrawalInput{OperationID: "same_withdrawal_token_123", Amount: "0.7", Channel: "test-bank", Recipient: "private-test"}
	var results [2]*service.Withdrawal
	var errs [2]error
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = r.CreateWithdrawal(ctx, owner, in) }(i)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Equal(t, results[0].ID, results[1].ID)
	var available string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT available_amount::text FROM shared_pool_owner_wallets WHERE owner_id=$1`, owner).Scan(&available))
	require.Equal(t, "0.300000000000", available)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = r.ActWithdrawal(ctx, owner, results[0].ID, false, service.WithdrawalAction{Action: "cancelled"})
		}(i)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.NoError(t, db.QueryRowContext(ctx, `SELECT available_amount::text FROM shared_pool_owner_wallets WHERE owner_id=$1`, owner).Scan(&available))
	require.Equal(t, "1.000000000000", available)
	// Different tokens cannot overdraw even when submitted simultaneously.
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			input := in
			input.OperationID = fmt.Sprintf("different_withdrawal_token_%d", i)
			results[i], errs[i] = r.CreateWithdrawal(ctx, owner, input)
		}(i)
	}
	wg.Wait()
	successes := 0
	for _, e := range errs {
		if e == nil {
			successes++
		} else {
			require.ErrorIs(t, e, service.ErrInsufficientBalance)
		}
	}
	require.Equal(t, 1, successes)
	var returns int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM shared_pool_owner_earnings_ledger WHERE owner_id=$1 AND event_type='withdrawal_return'`, owner).Scan(&returns))
	require.Equal(t, 1, returns)
	outsider := insertMarketplaceTestUser(t, db, fmt.Sprintf("withdrawal-outsider-%d@example.test", time.Now().UnixNano()))
	private, err := r.ListWithdrawals(ctx, outsider, 0)
	require.NoError(t, err)
	require.Empty(t, private)
}
