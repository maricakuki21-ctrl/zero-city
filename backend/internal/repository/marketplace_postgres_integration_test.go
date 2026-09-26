package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceRepositoryPostgres_participantPrivacyAndTakedown(t *testing.T) {
	// Given
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	require.NoError(t, db.PingContext(ctx))
	repo := NewMarketplaceRepository(db)
	suffix := time.Now().UnixNano()
	ownerID := insertMarketplaceTestUser(t, db, fmt.Sprintf("owner-%d@example.test", suffix))
	initiatorID := insertMarketplaceTestUser(t, db, fmt.Sprintf("initiator-%d@example.test", suffix))
	outsiderID := insertMarketplaceTestUser(t, db, fmt.Sprintf("outsider-%d@example.test", suffix))
	assetID := insertMarketplaceTestAsset(t, db, ownerID, suffix)
	foreignAssetID := insertMarketplaceTestAsset(t, db, outsiderID, suffix+1)
	_, err = repo.CreateListing(ctx, ownerID, service.MarketplaceListingInput{
		Kind: "service", CanonicalAssetID: &foreignAssetID, Title: "Invalid asset owner",
		Summary: "Must be rejected", Category: "review",
	})
	require.True(t, infraerrors.IsBadRequest(err))

	// When
	listing, err := repo.CreateListing(ctx, ownerID, service.MarketplaceListingInput{
		Kind: "demand", CanonicalAssetID: &assetID, Title: "Need a reviewer",
		Summary: "Private inquiry test", Category: "review", Tags: []string{"go"},
	})
	require.NoError(t, err)
	inquiry, err := repo.CreateInquiry(ctx, listing.ID, initiatorID)
	require.NoError(t, err)
	sameKey := postMarketplaceMessagesConcurrently(ctx, repo, inquiry.ID, initiatorID, []service.MarketplaceMessageInput{
		{ClientMessageID: "concurrent-same", Body: "Can you help?"},
		{ClientMessageID: "concurrent-same", Body: "Can you help?"},
	})
	differentKeys := postMarketplaceMessagesConcurrently(ctx, repo, inquiry.ID, initiatorID, []service.MarketplaceMessageInput{
		{ClientMessageID: "concurrent-a", Body: "First distinct message"},
		{ClientMessageID: "concurrent-b", Body: "Second distinct message"},
	})

	// Then
	require.NoError(t, sameKey[0].err)
	require.NoError(t, sameKey[1].err)
	require.Equal(t, sameKey[0].message.ID, sameKey[1].message.ID)
	require.NoError(t, differentKeys[0].err)
	require.NoError(t, differentKeys[1].err)
	require.NotEqual(t, differentKeys[0].message.ID, differentKeys[1].message.ID)
	_, err = repo.PostMessage(ctx, inquiry.ID, initiatorID, service.MarketplaceMessageInput{
		ClientMessageID: "concurrent-same", Body: "Different retry",
	})
	require.True(t, infraerrors.IsConflict(err))
	_, err = repo.ListMessages(ctx, inquiry.ID, outsiderID, service.MarketplacePageQuery{Limit: 20})
	require.True(t, infraerrors.IsForbidden(err))
	_, err = repo.CreateInquiry(ctx, listing.ID, ownerID)
	require.ErrorIs(t, err, service.ErrMarketplaceSelfInquiry)
	_, err = repo.AdminSetListingStatus(ctx, listing.ID, service.MarketplaceListingStatusTakenDown)
	require.NoError(t, err)
	_, err = repo.UpdateListing(ctx, listing.ID, ownerID, service.MarketplaceListingInput{
		Kind: "demand", CanonicalAssetID: &assetID, Title: "Owner restore attempt",
		Summary: "Must remain blocked", Category: "review", Tags: []string{"go"},
	})
	require.ErrorIs(t, err, service.ErrMarketplaceListingLocked)
	_, err = repo.GetListing(ctx, listing.ID, outsiderID)
	require.True(t, infraerrors.IsNotFound(err))
	history, err := repo.ListMessages(ctx, inquiry.ID, initiatorID, service.MarketplacePageQuery{Limit: 20})
	require.NoError(t, err)
	require.Len(t, history.Items, 3)
	_, err = repo.AdminSetListingStatus(ctx, listing.ID, service.MarketplaceListingStatusPublished)
	require.NoError(t, err)
	restored, err := repo.GetListing(ctx, listing.ID, outsiderID)
	require.NoError(t, err)
	require.Equal(t, service.MarketplaceListingStatusPublished, restored.Status)
}

func TestMarketplaceRepositoryPostgres_orderLifecycleReviewsAndNotes(t *testing.T) {
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	require.NoError(t, db.PingContext(ctx))
	repo := NewMarketplaceRepository(db)
	suffix := time.Now().UnixNano()
	buyerID := insertMarketplaceTestUser(t, db, fmt.Sprintf("order-buyer-%d@example.test", suffix))
	sellerID := insertMarketplaceTestUser(t, db, fmt.Sprintf("order-seller-%d@example.test", suffix))
	outsiderID := insertMarketplaceTestUser(t, db, fmt.Sprintf("order-outsider-%d@example.test", suffix))
	listing, err := repo.CreateListing(ctx, sellerID, service.MarketplaceListingInput{
		Kind: "service", Title: "Order lifecycle", Summary: "Track scope and delivery",
		Category: "review", PriceText: "quote", Tags: []string{"go"},
	})
	require.NoError(t, err)
	inquiry, err := repo.CreateInquiry(ctx, listing.ID, buyerID)
	require.NoError(t, err)

	order, err := repo.CreateMarketplaceOrder(ctx, inquiry.ID, sellerID, service.MarketplaceOrderQuoteInput{
		ScopeText: "Deliver the verified implementation", AmountText: "100 credits", DeliveryText: "2 days", RevisionLimit: 1,
	})
	require.NoError(t, err)
	require.Equal(t, service.MarketplaceOrderStatusQuoted, order.Status)
	require.Equal(t, buyerID, order.BuyerUserID)
	require.Equal(t, sellerID, order.SellerUserID)
	require.Equal(t, []string{"cancel"}, order.AvailableActions)
	// Fixture represents a real pre-migration cooperation-only order.
	_, err = db.ExecContext(ctx, `UPDATE marketplace_orders SET legacy_cooperation=TRUE WHERE id=$1`, order.ID)
	require.NoError(t, err)
	_, err = repo.CreateMarketplaceOrder(ctx, inquiry.ID, sellerID, service.MarketplaceOrderQuoteInput{
		ScopeText: "重复报价",
	})
	require.ErrorIs(t, err, service.ErrMarketplaceOrderExists)
	buyerView, err := repo.GetMarketplaceOrder(ctx, order.ID, buyerID)
	require.NoError(t, err)
	require.Equal(t, []string{"confirm", "cancel"}, buyerView.AvailableActions)

	order, err = repo.TransitionMarketplaceOrder(ctx, service.MarketplaceOrderTransition{
		OrderID: order.ID, ActorID: buyerID, Action: "confirm",
		FromStatuses:    []string{service.MarketplaceOrderStatusQuoted},
		ToStatus:        service.MarketplaceOrderStatusConfirmed,
		TimestampColumn: "confirmed_at",
	})
	require.NoError(t, err)
	require.Equal(t, service.MarketplaceOrderStatusConfirmed, order.Status)
	require.NotNil(t, order.ConfirmedAt)

	order, err = repo.TransitionMarketplaceOrder(ctx, service.MarketplaceOrderTransition{
		OrderID: order.ID, ActorID: sellerID, Action: "deliver",
		FromStatuses:    []string{service.MarketplaceOrderStatusConfirmed},
		ToStatus:        service.MarketplaceOrderStatusDelivered,
		TimestampColumn: "delivered_at", NoteColumn: "delivery_note", Note: "验收包已经上传",
	})
	require.NoError(t, err)
	require.Equal(t, "验收包已经上传", order.DeliveryNote)
	require.NotNil(t, order.DeliveredAt)

	order, err = repo.TransitionMarketplaceOrder(ctx, service.MarketplaceOrderTransition{
		OrderID: order.ID, ActorID: buyerID, Action: "accept",
		FromStatuses:    []string{service.MarketplaceOrderStatusDelivered},
		ToStatus:        service.MarketplaceOrderStatusAccepted,
		TimestampColumn: "accepted_at", NoteColumn: "accept_note", Note: "范围与验收一致",
	})
	require.NoError(t, err)
	require.Equal(t, service.MarketplaceOrderStatusAccepted, order.Status)
	require.Equal(t, "范围与验收一致", order.AcceptNote)

	_, err = repo.CreateMarketplaceOrderReview(ctx, order.ID, buyerID, 5, "交付清楚")
	require.NoError(t, err)
	_, err = repo.CreateMarketplaceOrderReview(ctx, order.ID, buyerID, 4, "重复评价")
	require.ErrorIs(t, err, service.ErrMarketplaceOrderReviewExists)

	order, err = repo.TransitionMarketplaceOrder(ctx, service.MarketplaceOrderTransition{
		OrderID: order.ID, ActorID: sellerID, Action: "settle",
		FromStatuses:    []string{service.MarketplaceOrderStatusAccepted},
		ToStatus:        service.MarketplaceOrderStatusSettled,
		TimestampColumn: "settled_at",
	})
	require.NoError(t, err)
	require.Equal(t, service.MarketplaceOrderStatusSettled, order.Status)
	require.Len(t, order.Events, 5)
	require.Len(t, order.Reviews, 1)

	_, err = repo.GetMarketplaceOrder(ctx, order.ID, outsiderID)
	require.ErrorIs(t, err, service.ErrMarketplaceOrderForbidden)
	buyerPage, err := repo.ListMarketplaceOrders(ctx, buyerID, service.MarketplacePageQuery{Limit: 20})
	require.NoError(t, err)
	require.Len(t, buyerPage.Items, 1)
	require.Empty(t, buyerPage.Items[0].AvailableActions)
}

func TestMarketplaceRepositoryPostgres_disputeConcurrentResolution(t *testing.T) {
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	repo := NewMarketplaceRepository(db)
	svc := service.NewMarketplaceService(repo)
	suffix := time.Now().UnixNano()
	buyer := insertMarketplaceTestUser(t, db, fmt.Sprintf("dispute-buyer-%d@example.test", suffix))
	seller := insertMarketplaceTestUser(t, db, fmt.Sprintf("dispute-seller-%d@example.test", suffix))
	admin := insertMarketplaceTestUser(t, db, fmt.Sprintf("dispute-admin-%d@example.test", suffix))
	_, err = db.ExecContext(ctx, "UPDATE users SET role = 'admin' WHERE id = $1", admin)
	require.NoError(t, err)
	listing, err := repo.CreateListing(ctx, seller, service.MarketplaceListingInput{Kind: "service", Title: "Dispute", Summary: "Delivery", Category: "test"})
	require.NoError(t, err)
	inquiry, err := repo.CreateInquiry(ctx, listing.ID, buyer)
	require.NoError(t, err)
	order, err := svc.QuoteOrder(ctx, inquiry.ID, seller, service.MarketplaceOrderQuoteInput{ScopeText: "Deliver", AmountText: "Text only"})
	require.NoError(t, err)
	// Regression fixture for pre-migration unpaid cooperation disputes.
	_, err = db.ExecContext(ctx, `UPDATE marketplace_orders SET legacy_cooperation=TRUE WHERE id=$1`, order.ID)
	require.NoError(t, err)
	for _, action := range []struct {
		actor        int64
		action, note string
	}{
		{buyer, "confirm", ""}, {seller, "deliver", "old delivery"}, {buyer, "accept", "old acceptance"}, {buyer, "dispute", "incomplete"},
	} {
		_, err = svc.ApplyOrderAction(ctx, order.ID, action.actor, service.MarketplaceOrderActionInput{Action: action.action, Note: action.note})
		require.NoError(t, err)
	}
	_, err = svc.ResolveDispute(ctx, order.ID, buyer, service.MarketplaceDisputeResolutionInput{Outcome: "confirmed", Reason: "unauthorized"})
	require.ErrorIs(t, err, service.ErrMarketplaceOrderRoleDenied)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.ResolveDispute(ctx, order.ID, admin, service.MarketplaceDisputeResolutionInput{Outcome: "confirmed", Reason: "new delivery agreed"})
		}(i)
	}
	wg.Wait()
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, service.ErrMarketplaceOrderInvalidState)
		}
	}
	require.Equal(t, 1, successes)
	resolved, err := svc.GetOrder(ctx, order.ID, buyer)
	require.NoError(t, err)
	require.Equal(t, "confirmed", resolved.Status)
	require.Nil(t, resolved.DeliveredAt)
	require.Nil(t, resolved.AcceptedAt)
	require.Equal(t, "old delivery", resolved.DeliveryNote)
	require.Equal(t, "old acceptance", resolved.AcceptNote)
	require.Equal(t, "Text only", resolved.AmountText)
	resolutionCount := 0
	for _, event := range resolved.Events {
		if event.Event == "admin_resolve_dispute" {
			resolutionCount++
			require.Equal(t, admin, event.ActorUserID)
			require.Equal(t, "new delivery agreed", event.Note)
		}
	}
	require.Equal(t, 1, resolutionCount)
	_, err = svc.ApplyOrderAction(ctx, order.ID, buyer, service.MarketplaceOrderActionInput{Action: "settle"})
	require.ErrorIs(t, err, service.ErrMarketplaceOrderInvalidState)
}

type marketplacePostResult struct {
	message *service.MarketplaceMessage
	err     error
}

func postMarketplaceMessagesConcurrently(
	ctx context.Context,
	repo service.MarketplaceRepository,
	inquiryID, senderID int64,
	inputs []service.MarketplaceMessageInput,
) []marketplacePostResult {
	start := make(chan struct{})
	results := make([]marketplacePostResult, len(inputs))
	var wait sync.WaitGroup
	wait.Add(len(inputs))
	for index, input := range inputs {
		go func() {
			defer wait.Done()
			<-start
			results[index].message, results[index].err = repo.PostMessage(ctx, inquiryID, senderID, input)
		}()
	}
	close(start)
	wait.Wait()
	return results
}

func insertMarketplaceTestUser(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`INSERT INTO users (email, password_hash, role, status) VALUES ($1, 'test', 'user', 'active') RETURNING id`, email).Scan(&id)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO biz_profiles (user_id, display_name) VALUES ($1, $2)`, id, email)
	require.NoError(t, err)
	return id
}

func insertMarketplaceTestAsset(t *testing.T, db *sql.DB, ownerID, suffix int64) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`INSERT INTO capability_assets (user_id, title, slug, summary)
		VALUES ($1, 'Test asset', $2, 'Integration fixture') RETURNING id`, ownerID, fmt.Sprintf("marketplace-%d", suffix)).Scan(&id)
	require.NoError(t, err)
	return id
}
