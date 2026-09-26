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

func TestMarketplaceFundsPostgres(t *testing.T) {
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
	r := &marketplaceRepository{db: db}
	newUser := func(name string) int64 {
		return insertMarketplaceTestUser(t, db, fmt.Sprintf("funds-%s-%d@example.test", name, time.Now().UnixNano()))
	}
	buyer, seller, outsider, admin := newUser("buyer"), newUser("seller"), newUser("outsider"), newUser("admin")
	_, err = db.ExecContext(ctx, `UPDATE users SET balance=100 WHERE id=$1`, buyer)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, admin)
	require.NoError(t, err)
	newOrder := func() *service.MarketplaceOrder {
		l, e := r.CreateListing(ctx, seller, service.MarketplaceListingInput{Kind: "service", Title: "Design", Category: "design", Summary: "paid order test"})
		require.NoError(t, e)
		i, e := r.CreateInquiry(ctx, l.ID, buyer)
		require.NoError(t, e)
		o, e := r.CreateMarketplaceOrder(ctx, i.ID, seller, service.MarketplaceOrderQuoteInput{ScopeText: "Design delivery", AmountText: "Do not infer CNY 900", DeliveryText: "Tomorrow"})
		require.NoError(t, e)
		return o
	}
	order := newOrder()
	require.False(t, order.LegacyCooperation)
	svc := service.NewMarketplaceService(r)
	_, err = svc.ApplyOrderAction(ctx, order.ID, buyer, service.MarketplaceOrderActionInput{Action: "confirm"})
	require.ErrorIs(t, err, service.ErrMarketplaceFundsInvalid)
	buyerOrder, err := r.GetMarketplaceOrder(ctx, order.ID, buyer)
	require.NoError(t, err)
	require.NotContains(t, buyerOrder.AvailableActions, "confirm")
	for _, action := range []string{"confirm", "deliver", "accept", "settle"} {
		actor := buyer
		if action == "deliver" {
			actor = seller
		}
		_, err = r.TransitionMarketplaceOrder(ctx, service.MarketplaceOrderTransition{OrderID: order.ID, ActorID: actor, Action: action, FromStatuses: []string{"quoted"}, ToStatus: "confirmed"})
		require.ErrorIs(t, err, service.ErrMarketplaceFundsInvalid, action)
	}
	f, err := r.GetMarketplaceFunds(ctx, order.ID, buyer)
	require.NoError(t, err)
	require.Nil(t, f)
	_, err = r.GetMarketplaceFunds(ctx, order.ID, outsider)
	require.Error(t, err)
	_, err = r.SetMarketplaceFunds(ctx, order.ID, buyer, service.MarketplaceFundsInput{Amount: "10", Currency: "USD"})
	require.Error(t, err)
	f, err = r.SetMarketplaceFunds(ctx, order.ID, seller, service.MarketplaceFundsInput{Amount: "10.12345678", Currency: "USD"})
	require.NoError(t, err)
	require.Equal(t, "10.12345678", f.Amount)
	_, err = r.SetMarketplaceFunds(ctx, order.ID, seller, service.MarketplaceFundsInput{Amount: "12", Currency: "USD"})
	require.Error(t, err)
	policy, err := r.GetMarketplaceFundsPolicy(ctx)
	require.NoError(t, err)
	require.False(t, policy.Enabled)
	input := service.MarketplacePaymentInput{ExpectedAmount: f.Amount, Currency: "USD", OperationID: "marketplace_payment_0001"}
	_, err = r.PayMarketplaceOrder(ctx, order.ID, buyer, input)
	require.ErrorIs(t, err, service.ErrMarketplaceFundsClosed)
	_, err = r.SetMarketplaceFundsPolicy(ctx, buyer, service.MarketplaceFundsPolicyInput{Enabled: true, Reason: "not admin"})
	require.Error(t, err)
	_, err = r.SetMarketplaceFundsPolicy(ctx, admin, service.MarketplaceFundsPolicyInput{Enabled: true, Reason: "isolated QA"})
	require.NoError(t, err)
	_, err = r.PayMarketplaceOrder(ctx, order.ID, outsider, input)
	require.Error(t, err)
	stale := input
	stale.ExpectedAmount = "9"
	_, err = r.PayMarketplaceOrder(ctx, order.ID, buyer, stale)
	require.Error(t, err)
	// A paid quote cannot silently use the legacy confirm path.
	_, err = r.TransitionMarketplaceOrder(ctx, service.MarketplaceOrderTransition{OrderID: order.ID, ActorID: buyer, Action: "confirm", FromStatuses: []string{"quoted"}, ToStatus: "confirmed", TimestampColumn: "confirmed_at"})
	require.ErrorIs(t, err, service.ErrMarketplaceFundsInvalid)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); _, errs[i] = r.PayMarketplaceOrder(ctx, order.ID, buyer, input) }(i)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	var balance string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, buyer).Scan(&balance))
	require.Equal(t, "89.87654322", balance)
	var wallets int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM shared_pool_owner_wallets WHERE owner_id=$1`, seller).Scan(&wallets))
	require.Zero(t, wallets)
	move := func(o *service.MarketplaceOrder, actor int64, action, from, to string) error {
		_, e := r.TransitionMarketplaceOrder(ctx, service.MarketplaceOrderTransition{OrderID: o.ID, ActorID: actor, Action: action, FromStatuses: []string{from}, ToStatus: to})
		return e
	}
	require.NoError(t, move(order, seller, "deliver", "confirmed", "delivered"))
	require.Error(t, move(order, seller, "accept", "delivered", "accepted"))
	for i := range errs {
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs[i] = move(order, buyer, "accept", "delivered", "accepted") }(i)
	}
	wg.Wait()
	require.True(t, (errs[0] == nil) != (errs[1] == nil))
	var wallet string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT available_amount::text FROM shared_pool_owner_wallets WHERE owner_id=$1`, seller).Scan(&wallet))
	require.Equal(t, "10.123456780000", wallet)
	require.ErrorIs(t, move(order, buyer, "dispute", "accepted", "disputed"), service.ErrMarketplaceFundsInvalid)
	f, err = r.PayMarketplaceOrder(ctx, order.ID, buyer, input)
	require.NoError(t, err)
	require.Equal(t, "released", f.Status)
	refund := newOrder()
	_, err = r.SetMarketplaceFunds(ctx, refund.ID, seller, service.MarketplaceFundsInput{Amount: "20", Currency: "USD"})
	require.NoError(t, err)
	input = service.MarketplacePaymentInput{ExpectedAmount: "20", Currency: "USD", OperationID: "marketplace_payment_0002"}
	_, err = r.PayMarketplaceOrder(ctx, refund.ID, buyer, input)
	require.NoError(t, err)
	require.NoError(t, move(refund, buyer, "dispute", "confirmed", "disputed"))
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = r.ResolveMarketplaceDispute(ctx, refund.ID, admin, service.MarketplaceDisputeResolutionInput{Outcome: "canceled", Reason: "non delivery"})
		}(i)
	}
	wg.Wait()
	require.True(t, (errs[0] == nil) != (errs[1] == nil))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, buyer).Scan(&balance))
	require.Equal(t, "89.87654322", balance)
	f, err = r.GetMarketplaceFunds(ctx, refund.ID, buyer)
	require.NoError(t, err)
	require.Equal(t, "refunded", f.Status)
	unaffordable := newOrder()
	_, err = r.SetMarketplaceFunds(ctx, unaffordable.ID, seller, service.MarketplaceFundsInput{Amount: "1000", Currency: "USD"})
	require.NoError(t, err)
	_, err = r.PayMarketplaceOrder(ctx, unaffordable.ID, buyer, service.MarketplacePaymentInput{ExpectedAmount: "1000", Currency: "USD", OperationID: "marketplace_payment_0003"})
	require.ErrorIs(t, err, service.ErrMarketplaceFundsInsufficient)
	f, err = r.GetMarketplaceFunds(ctx, unaffordable.ID, buyer)
	require.NoError(t, err)
	require.Equal(t, "unpaid", f.Status)
	legacy := newOrder()
	_, err = db.ExecContext(ctx, `UPDATE marketplace_orders SET legacy_cooperation=TRUE WHERE id=$1`, legacy.ID)
	require.NoError(t, err)
	require.NoError(t, move(legacy, buyer, "confirm", "quoted", "confirmed"))
	f, err = r.GetMarketplaceFunds(ctx, legacy.ID, buyer)
	require.NoError(t, err)
	require.Nil(t, f)
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM user_balance_ledger WHERE user_id=$1 AND source_type IN ('marketplace_order_payment','marketplace_order_refund')`, buyer).Scan(&count))
	require.Equal(t, 3, count)
	_, err = r.SetMarketplaceFundsPolicy(ctx, admin, service.MarketplaceFundsPolicyInput{Enabled: false, Reason: "QA done"})
	require.NoError(t, err)
}
