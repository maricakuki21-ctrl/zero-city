package repository

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCapabilityAssetCommercePostgres(t *testing.T) {
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
	s := service.NewBizDecipherService(r, nil, nil)
	newUser := func(name string) int64 {
		id := insertMarketplaceTestUser(t, db, fmt.Sprintf("asset-%s-%d@example.test", name, time.Now().UnixNano()))
		_, e := db.ExecContext(ctx, `UPDATE users SET balance=100 WHERE id=$1`, id)
		require.NoError(t, e)
		return id
	}
	owner, buyer, other, admin := newUser("owner"), newUser("buyer"), newUser("other"), newUser("admin")
	_, err = db.ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, admin)
	require.NoError(t, err)
	newAsset := func(user int64) int64 {
		a, e := r.CreateCapabilityAsset(ctx, user, fmt.Sprintf("commerce-%d", user), service.CapabilityAssetInput{
			Title: "Hosted paid tool", Summary: "test", Description: "test", AssetType: "tool", Status: "draft",
			PricingType: "free", PrimaryActionType: "download", Tags: []string{}, ScenarioTags: []string{},
			IntegrationTags: []string{}, ScreenshotURLs: []string{},
		})
		require.NoError(t, e)
		_, e = s.ImportCapabilityAssetPackage(ctx, a.ID, user, service.ImportCapabilityAssetPackageInput{
			Version: "v1", RuntimeKind: "workflow", Manifest: []byte(`{"secret":"paid-manifest"}`), Finalize: true,
			Files: []service.CapabilityAssetFileInput{{Path: "entry.txt", ContentType: "text/plain", ContentBase64: base64.StdEncoding.EncodeToString([]byte("paid package bytes"))}},
		})
		require.NoError(t, e)
		_, e = s.PublishCapabilityAssetVersion(ctx, a.ID, user, "v1")
		require.NoError(t, e)
		_, e = db.ExecContext(ctx, `UPDATE capability_assets SET status='listed' WHERE id=$1`, a.ID)
		require.NoError(t, e)
		return a.ID
	}
	id := newAsset(owner)
	policy, err := r.GetAssetCommercePolicy(ctx)
	require.NoError(t, err)
	require.False(t, policy.Enabled)
	_, err = s.DownloadCapabilityAssetPackage(ctx, id, buyer, "v1")
	require.NoError(t, err)
	_, err = r.SetAssetPricing(ctx, id, buyer, service.ColumnPricingInput{Mode: "paid", Price: "10.12345678"})
	require.ErrorIs(t, err, service.ErrAssetCommerceForbidden)
	_, err = r.SetAssetPricing(ctx, id, owner, service.ColumnPricingInput{Mode: "paid", Price: "10.12345678"})
	require.NoError(t, err)
	in := service.ColumnPurchaseInput{OperationID: "asset_purchase_token_1", ExpectedPrice: "10.12345678"}
	_, err = r.PurchaseAsset(ctx, id, buyer, in)
	require.ErrorIs(t, err, service.ErrAssetCommerceClosed)
	_, err = r.SetAssetCommercePolicy(ctx, admin, true, "isolated acceptance")
	require.NoError(t, err)
	_, err = r.PurchaseAsset(ctx, id, owner, in)
	require.ErrorIs(t, err, service.ErrAssetCommerceForbidden)
	stale := in
	stale.ExpectedPrice = "1"
	_, err = r.PurchaseAsset(ctx, id, buyer, stale)
	require.ErrorIs(t, err, service.ErrAssetCommerceConflict)
	_, err = s.DownloadCapabilityAssetPackage(ctx, id, buyer, "v1")
	require.ErrorIs(t, err, service.ErrCapabilityAssetPackageForbidden)
	_, err = s.ReuseCapabilityAssetPackage(ctx, id, buyer, "v1", "asset_reuse_before_1")
	require.ErrorIs(t, err, service.ErrCapabilityAssetPackageForbidden)
	versions, err := s.ListCapabilityAssetVersions(ctx, id, buyer)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	require.JSONEq(t, `{}`, string(versions[0].Manifest))
	var results [2]*service.AssetPurchase
	var errs [2]error
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = r.PurchaseAsset(ctx, id, buyer, in) }(i)
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
	stats, err := r.GetCapabilityAssetStats(ctx, owner)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.True(t, stats[0].RevenueConnected)
	pkg, err := s.DownloadCapabilityAssetPackage(ctx, id, buyer, "v1")
	require.NoError(t, err)
	require.Equal(t, "paid package bytes", string(pkg.Files[0].Content))
	pkg, err = s.ReuseCapabilityAssetPackage(ctx, id, buyer, "v1", "asset_reuse_after_1")
	require.NoError(t, err)
	require.JSONEq(t, `{"secret":"paid-manifest"}`, string(pkg.Version.Manifest))
	_, err = s.CreateCapabilityAssetVersion(ctx, id, owner, service.CreateCapabilityAssetVersionInput{Version: "v2", RuntimeKind: "workflow"})
	require.NoError(t, err)
	_, err = s.DownloadCapabilityAssetPackage(ctx, id, buyer, "v2")
	require.ErrorIs(t, err, service.ErrCapabilityAssetPackageForbidden)
	_, err = db.ExecContext(ctx, `UPDATE capability_assets SET status='delisted' WHERE id=$1`, id)
	require.NoError(t, err)
	_, err = s.DownloadCapabilityAssetPackage(ctx, id, buyer, "v1")
	require.ErrorIs(t, err, service.ErrCapabilityAssetPackageNotFound)
	_, err = db.ExecContext(ctx, `UPDATE capability_assets SET status='listed' WHERE id=$1`, id)
	require.NoError(t, err)
	refund := service.ColumnRefundInput{OperationID: "asset_refund_token_1", Reason: "verified dispute"}
	_, err = r.RefundAssetPurchase(ctx, id, p.ID, buyer, refund)
	require.ErrorIs(t, err, service.ErrAssetCommerceForbidden)
	_, err = db.ExecContext(ctx, `UPDATE shared_pool_owner_wallets SET available_amount=0 WHERE owner_id=$1`, owner)
	require.NoError(t, err)
	_, err = r.RefundAssetPurchase(ctx, id, p.ID, admin, refund)
	require.ErrorIs(t, err, service.ErrAssetCommerceFunds)
	_, err = db.ExecContext(ctx, `UPDATE shared_pool_owner_wallets SET available_amount=$2::numeric WHERE owner_id=$1`, owner, p.CreatorAmount)
	require.NoError(t, err)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = r.RefundAssetPurchase(ctx, id, p.ID, admin, refund)
		}(i)
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Equal(t, "refunded", results[0].Status)
	_, err = s.DownloadCapabilityAssetPackage(ctx, id, buyer, "v1")
	require.ErrorIs(t, err, service.ErrCapabilityAssetPackageForbidden)
	_, err = s.ReuseCapabilityAssetPackage(ctx, id, buyer, "v1", "asset_reuse_refunded_1")
	require.ErrorIs(t, err, service.ErrCapabilityAssetPackageForbidden)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, buyer).Scan(&balance))
	require.Equal(t, "100.00000000", balance)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT available_amount::text FROM shared_pool_owner_wallets WHERE owner_id=$1`, owner).Scan(&wallet))
	require.Equal(t, "0.000000000000", wallet)
	entries, err := r.ListSharedPoolOwnerEarningsPage(ctx, owner, 0, 0, 20)
	require.NoError(t, err)
	require.Len(t, entries.Items, 2)
	var reversed int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM shared_pool_owner_earnings_ledger WHERE owner_id=$1 AND status='reversed' AND metadata->>'source_type'='capability_asset_purchase'`, owner).Scan(&reversed))
	require.Equal(t, 1, reversed)
	hidden, err := r.ListAssetPurchases(ctx, id, service.CreatorColumnQuery{ViewerID: other, Limit: 20})
	require.NoError(t, err)
	require.Empty(t, hidden)
	id2 := newAsset(buyer)
	for _, pair := range [][2]int64{{id, owner}, {id2, buyer}} {
		_, err = r.SetAssetPricing(ctx, pair[0], pair[1], service.ColumnPricingInput{Mode: "paid", Price: "1"})
		require.NoError(t, err)
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errs[0] = r.PurchaseAsset(ctx, id2, owner, service.ColumnPurchaseInput{OperationID: "asset_reciprocal_1", ExpectedPrice: "1"})
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = r.PurchaseAsset(ctx, id, buyer, service.ColumnPurchaseInput{OperationID: "asset_reciprocal_2", ExpectedPrice: "1"})
	}()
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	_, err = db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE user_balance_ledger ADD CONSTRAINT asset_commerce_failure CHECK(user_id<>%d OR source_type<>'capability_asset_purchase')`, other))
	require.NoError(t, err)
	_, err = r.PurchaseAsset(ctx, id, other, service.ColumnPurchaseInput{OperationID: "asset_rollback_test_1", ExpectedPrice: "1"})
	require.Error(t, err)
	_, err = db.ExecContext(ctx, `ALTER TABLE user_balance_ledger DROP CONSTRAINT asset_commerce_failure`)
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance::text FROM users WHERE id=$1`, other).Scan(&balance))
	require.Equal(t, "100.00000000", balance)
	info, err := r.GetAssetCommerce(ctx, id, other, false)
	require.NoError(t, err)
	require.False(t, info.CanDownload)
	_, err = s.RevokeCapabilityAssetVersion(ctx, id, owner, "v1")
	require.NoError(t, err)
	_, err = s.DownloadCapabilityAssetPackage(ctx, id, buyer, "v1")
	require.ErrorIs(t, err, service.ErrCapabilityAssetPackageNotFound)
}
