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

func TestCreatorColumnCommercePostgres(t *testing.T) {
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var database string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&database))
	require.True(t, strings.HasPrefix(database, "bizdecipher_columns_acceptance_test_"), "disposable database only")
	require.NoError(t, ApplyMigrations(ctx, db))
	r := &bizDecipherRepository{db: db}
	newUser := func(name string) int64 {
		user := insertMarketplaceTestUser(t, db, fmt.Sprintf("commerce-%s-%d@example.test", name, time.Now().UnixNano()))
		_, e := db.ExecContext(ctx, `UPDATE users SET balance=100 WHERE id=$1`, user)
		require.NoError(t, e)
		return user
	}
	owner, buyer, other, admin := newUser("owner"), newUser("buyer"), newUser("other"), newUser("admin")
	_, err = db.ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, admin)
	require.NoError(t, err)
	title, body, published := "Commerce column", "Paid body", "published"
	c, err := r.CreateCreatorColumn(ctx, owner, service.CreatorColumnInput{Title: &title})
	require.NoError(t, err)
	a, err := r.SaveCreatorColumnArticle(ctx, c.ID, 0, owner, service.CreatorColumnArticleInput{Title: &title, Body: &body, Status: &published})
	require.NoError(t, err)
	draft, err := r.SaveCreatorColumnArticle(ctx, c.ID, 0, owner, service.CreatorColumnArticleInput{Title: &title, Body: &body})
	require.NoError(t, err)
	free, err := r.GetCreatorColumnArticle(ctx, c.ID, a.ID, service.CreatorColumnQuery{})
	require.NoError(t, err)
	require.Equal(t, body, free.Body)
	policy, err := r.GetColumnCommercePolicy(ctx)
	require.NoError(t, err)
	require.False(t, policy.Enabled)
	_, err = r.SetColumnCommercePolicy(ctx, buyer, true, "unprivileged")
	require.ErrorIs(t, err, service.ErrColumnCommerceForbidden)
	_, err = r.SetColumnPricing(ctx, c.ID, buyer, service.ColumnPricingInput{Mode: "paid", Price: "10.12345678"})
	require.ErrorIs(t, err, service.ErrColumnCommerceForbidden)
	c, err = r.SetColumnPricing(ctx, c.ID, owner, service.ColumnPricingInput{Mode: "paid", Price: "10.12345678"})
	require.NoError(t, err)
	input := service.ColumnPurchaseInput{OperationID: "commerce_purchase_token_1", ExpectedPrice: "10.12345678"}
	_, err = r.PurchaseColumn(ctx, c.ID, buyer, input)
	require.ErrorIs(t, err, service.ErrColumnCommerceClosed)
	_, err = r.SetColumnCommercePolicy(ctx, admin, true, "acceptance")
	require.NoError(t, err)
	_, err = r.PurchaseColumn(ctx, c.ID, owner, input)
	require.ErrorIs(t, err, service.ErrColumnCommerceForbidden)
	stale := input
	stale.ExpectedPrice = "10"
	_, err = r.PurchaseColumn(ctx, c.ID, buyer, stale)
	require.ErrorIs(t, err, service.ErrColumnCommerceConflict)
	_, err = r.GetCreatorColumnArticle(ctx, c.ID, a.ID, service.CreatorColumnQuery{ViewerID: buyer})
	require.ErrorIs(t, err, service.ErrCreatorColumnNotFound)
	// Concurrent exact retries debit and credit only once.
	var results [2]*service.ColumnPurchase
	var errs [2]error
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = r.PurchaseColumn(ctx, c.ID, buyer, input) }(i)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Equal(t, results[0].ID, results[1].ID)
	p := results[0]
	require.Equal(t, "0.00000000", p.PlatformFee)
	require.Equal(t, p.Amount, p.CreatorAmount)
	var balance, wallet string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, buyer).Scan(&balance))
	require.Equal(t, "89.87654322", balance)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT available_amount::text FROM shared_pool_owner_wallets WHERE owner_id=$1`, owner).Scan(&wallet))
	require.Equal(t, "10.123456780000", wallet)
	changed := input
	changed.OperationID = "commerce_new_operation_2"
	_, err = r.PurchaseColumn(ctx, c.ID, buyer, changed)
	require.ErrorIs(t, err, service.ErrColumnCommerceConflict)
	_, err = r.PurchaseColumn(ctx, c.ID, buyer, stale)
	require.ErrorIs(t, err, service.ErrColumnCommerceConflict)
	view, err := r.GetCreatorColumn(ctx, c.ID, service.CreatorColumnQuery{ViewerID: buyer})
	require.NoError(t, err)
	require.True(t, view.CanRead)
	require.Equal(t, p.ID, view.ViewerPurchaseID)
	_, err = r.GetCreatorColumnArticle(ctx, c.ID, a.ID, service.CreatorColumnQuery{ViewerID: buyer})
	require.NoError(t, err)
	_, err = r.GetCreatorColumnArticle(ctx, c.ID, draft.ID, service.CreatorColumnQuery{ViewerID: buyer})
	require.ErrorIs(t, err, service.ErrCreatorColumnNotFound)
	public, err := r.ListCreatorColumnArticles(ctx, c.ID, service.CreatorColumnQuery{ViewerID: other, Limit: 20})
	require.NoError(t, err)
	require.Len(t, public, 1)
	require.Empty(t, public[0].Body)
	_, err = r.ModerateCreatorColumn(ctx, c.ID, "suspended", "test")
	require.NoError(t, err)
	_, err = r.GetCreatorColumnArticle(ctx, c.ID, a.ID, service.CreatorColumnQuery{ViewerID: buyer})
	require.ErrorIs(t, err, service.ErrCreatorColumnNotFound)
	_, err = r.GetCreatorColumnArticle(ctx, c.ID, draft.ID, service.CreatorColumnQuery{ViewerID: buyer})
	require.ErrorIs(t, err, service.ErrCreatorColumnNotFound)
	_, err = r.ModerateCreatorColumn(ctx, c.ID, "active", "restore")
	require.NoError(t, err)
	_, err = r.SetColumnCommercePolicy(ctx, admin, false, "pause sales")
	require.NoError(t, err)
	_, err = r.GetCreatorColumnArticle(ctx, c.ID, a.ID, service.CreatorColumnQuery{ViewerID: buyer})
	require.NoError(t, err)
	replay, err := r.PurchaseColumn(ctx, c.ID, buyer, input)
	require.NoError(t, err)
	require.Equal(t, p.ID, replay.ID)
	refund := service.ColumnRefundInput{OperationID: "commerce_refund_token_1", Reason: "verified dispute"}
	_, err = r.RefundColumnPurchase(ctx, c.ID, p.ID, buyer, refund)
	require.ErrorIs(t, err, service.ErrColumnCommerceForbidden)
	require.NoError(t, r.SaveWithdrawalPolicy(ctx, admin, service.WithdrawalPolicy{Enabled: true, Channels: []string{"test"}}))
	w, err := r.CreateWithdrawal(ctx, owner, service.WithdrawalInput{OperationID: "commerce_withdrawal_1", Amount: p.CreatorAmount, Channel: "test", Recipient: "test only"})
	require.NoError(t, err)
	_, err = r.RefundColumnPurchase(ctx, c.ID, p.ID, admin, refund)
	require.ErrorIs(t, err, service.ErrColumnCommerceFunds)
	_, err = r.GetCreatorColumnArticle(ctx, c.ID, a.ID, service.CreatorColumnQuery{ViewerID: buyer})
	require.NoError(t, err)
	_, err = r.ActWithdrawal(ctx, owner, w.ID, false, service.WithdrawalAction{Action: "cancelled"})
	require.NoError(t, err)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = r.RefundColumnPurchase(ctx, c.ID, p.ID, admin, refund)
		}(i)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Equal(t, "refunded", results[0].Status)
	_, err = r.GetCreatorColumnArticle(ctx, c.ID, a.ID, service.CreatorColumnQuery{ViewerID: buyer})
	require.ErrorIs(t, err, service.ErrCreatorColumnNotFound)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, buyer).Scan(&balance))
	require.Equal(t, "100.00000000", balance)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT available_amount::text FROM shared_pool_owner_wallets WHERE owner_id=$1`, owner).Scan(&wallet))
	require.Equal(t, "0.000000000000", wallet)
	var settled string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COALESCE(SUM(net_amount),0)::text FROM shared_pool_owner_earnings_ledger
 WHERE owner_id=$1 AND event_type='earning' AND status IN ('available','settled')`, owner).Scan(&settled))
	require.Equal(t, "0", settled)
	entries, err := r.ListSharedPoolOwnerEarningsPage(ctx, owner, 0, 0, 20)
	require.NoError(t, err)
	require.Len(t, entries.Items, 4)
	hidden, err := r.ListColumnPurchases(ctx, c.ID, service.CreatorColumnQuery{ViewerID: other, Limit: 20})
	require.NoError(t, err)
	require.Empty(t, hidden)
	replay, err = r.PurchaseColumn(ctx, c.ID, buyer, input)
	require.NoError(t, err)
	require.Equal(t, "refunded", replay.Status)
	// Reciprocal purchases lock the same two users in the same order.
	c2, err := r.CreateCreatorColumn(ctx, buyer, service.CreatorColumnInput{Title: &title})
	require.NoError(t, err)
	_, err = r.SetColumnPricing(ctx, c2.ID, buyer, service.ColumnPricingInput{Mode: "paid", Price: "1"})
	require.NoError(t, err)
	_, err = r.SetColumnPricing(ctx, c.ID, owner, service.ColumnPricingInput{Mode: "paid", Price: "1"})
	require.NoError(t, err)
	_, err = r.SetColumnCommercePolicy(ctx, admin, true, "reopen")
	require.NoError(t, err)
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, errs[0] = r.PurchaseColumn(ctx, c2.ID, owner, service.ColumnPurchaseInput{OperationID: "reciprocal_purchase_1", ExpectedPrice: "1"})
	}()
	go func() {
		defer wg.Done()
		<-start
		_, errs[1] = r.PurchaseColumn(ctx, c.ID, buyer, service.ColumnPurchaseInput{OperationID: "reciprocal_purchase_2", ExpectedPrice: "1"})
	}()
	close(start)
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	// A downstream ledger failure must roll back balance, wallet and entitlement.
	_, err = db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE user_balance_ledger ADD CONSTRAINT commerce_test_failure CHECK (user_id <> %d OR source_type <> 'creator_column_purchase')`, other))
	require.NoError(t, err)
	_, err = r.PurchaseColumn(ctx, c.ID, other, service.ColumnPurchaseInput{OperationID: "rollback_purchase_123", ExpectedPrice: "1"})
	require.Error(t, err)
	_, err = db.ExecContext(ctx, `ALTER TABLE user_balance_ledger DROP CONSTRAINT commerce_test_failure`)
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, other).Scan(&balance))
	require.Equal(t, "100.00000000", balance)
	view, err = r.GetCreatorColumn(ctx, c.ID, service.CreatorColumnQuery{ViewerID: other})
	require.NoError(t, err)
	require.False(t, view.CanRead)
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM creator_column_purchases WHERE buyer_user_id=$1`, other).Scan(&count))
	require.Zero(t, count)
}
