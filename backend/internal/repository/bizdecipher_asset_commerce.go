package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const assetPurchaseFields = `id,asset_id,buyer_user_id,owner_user_id,operation_id,asset_title,amount::text,
 platform_fee::text,creator_amount::text,status,created_at,refunded_at,refund_reason`
const assetSellablePackage = `EXISTS(SELECT 1 FROM capability_asset_versions v WHERE v.asset_id=a.id
 AND v.status='published' AND v.file_count>0 AND v.runtime_kind<>'external_api')`

func scanAssetPurchase(row communityPollScanner) (*service.AssetPurchase, error) {
	p := &service.AssetPurchase{}
	err := row.Scan(&p.ID, &p.AssetID, &p.BuyerUserID, &p.OwnerUserID, &p.OperationID, &p.AssetTitle,
		&p.Amount, &p.PlatformFee, &p.CreatorAmount, &p.Status, &p.CreatedAt, &p.RefundedAt, &p.RefundReason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCapabilityAssetPackageNotFound
	}
	return p, err
}
func (r *bizDecipherRepository) GetAssetCommercePolicy(ctx context.Context) (*service.ColumnCommercePolicy, error) {
	p := &service.ColumnCommercePolicy{Currency: "USD", PlatformFee: "0.00000000"}
	err := r.db.QueryRowContext(ctx, `SELECT enabled FROM capability_asset_commerce_policy WHERE id=TRUE`).Scan(&p.Enabled)
	return p, err
}
func (r *bizDecipherRepository) SetAssetCommercePolicy(ctx context.Context, actor int64, enabled bool, reason string) (*service.ColumnCommercePolicy, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 1000 {
		return nil, service.ErrAssetCommerceInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = columnCommerceAdmin(ctx, tx, actor); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE capability_asset_commerce_policy SET enabled=$1,updated_at=NOW() WHERE id=TRUE`, enabled); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO capability_asset_commerce_audit(actor_user_id,action,detail)
 VALUES($1,'policy',jsonb_build_object('enabled',$2::boolean,'reason',$3::text))`, actor, enabled, reason); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &service.ColumnCommercePolicy{Enabled: enabled, Currency: "USD", PlatformFee: "0.00000000"}, nil
}
func (r *bizDecipherRepository) GetAssetCommerce(ctx context.Context, id, viewer int64, admin bool) (*service.AssetCommerceInfo, error) {
	p := &service.AssetCommerceInfo{}
	err := r.db.QueryRowContext(ctx, `SELECT a.id,a.user_id,a.pricing_type,a.commerce_price::text,
 ((a.status='listed' OR a.user_id=$2) AND (a.pricing_type IN('free','open_source') OR a.user_id=$2 OR EXISTS(
 SELECT 1 FROM capability_asset_purchases p WHERE p.asset_id=a.id AND p.buyer_user_id=$2 AND p.status='active'))),
 COALESCE((SELECT p.id FROM capability_asset_purchases p WHERE p.asset_id=a.id AND p.buyer_user_id=$2 AND p.status='active'),0),
 `+assetSellablePackage+`,a.status FROM capability_assets a
 WHERE a.id=$1 AND a.deleted_at IS NULL AND (a.status='listed' OR a.user_id=$2 OR $3::boolean)`, id, viewer, admin).
		Scan(&p.AssetID, &p.OwnerUserID, &p.PricingType, &p.Price, &p.CanDownload, &p.PurchaseID, &p.HasPackage, &p.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCapabilityAssetPackageNotFound
	}
	return p, err
}
func (r *bizDecipherRepository) SetAssetPricing(ctx context.Context, id, actor int64, input service.ColumnPricingInput) (*service.AssetCommerceInfo, error) {
	in, err := service.NormalizeColumnPricing(input)
	if err != nil {
		return nil, service.ErrAssetCommerceInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	users, err := lockColumnCommerceUsers(ctx, tx, actor)
	if err != nil {
		return nil, err
	}
	if !users[actor].active {
		return nil, service.ErrAssetCommerceForbidden
	}
	var eligible bool
	err = tx.QueryRowContext(ctx, `SELECT (`+assetSellablePackage+` AND a.demo_url='' AND a.source_url='' AND a.template_url='')
 FROM capability_assets a WHERE a.id=$1 AND a.user_id=$2 AND a.deleted_at IS NULL
 AND a.status NOT IN('delisted','archived') FOR UPDATE`, id, actor).Scan(&eligible)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAssetCommerceForbidden
	}
	if err != nil {
		return nil, err
	}
	if in.Mode == "paid" && !eligible {
		return nil, service.ErrAssetCommerceInvalid
	}
	if _, err = tx.ExecContext(ctx, `UPDATE capability_assets SET pricing_type=$2,commerce_price=$3::numeric,updated_at=NOW() WHERE id=$1`, id, in.Mode, in.Price); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO capability_asset_commerce_audit(actor_user_id,asset_id,action,detail)
 VALUES($1,$2,'pricing',jsonb_build_object('mode',$3::text,'price',$4::text,'platform_fee','0.00000000'))`, actor, id, in.Mode, in.Price); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetAssetCommerce(ctx, id, actor, false)
}
func (r *bizDecipherRepository) PurchaseAsset(ctx context.Context, id, buyer int64, input service.ColumnPurchaseInput) (*service.AssetPurchase, error) {
	in, err := service.NormalizeColumnPurchase(input)
	if err != nil {
		return nil, service.ErrAssetCommerceInvalid
	}
	var owner int64
	err = r.db.QueryRowContext(ctx, `SELECT user_id FROM capability_assets WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCapabilityAssetPackageNotFound
	}
	if err != nil {
		return nil, err
	}
	if owner == buyer {
		return nil, service.ErrAssetCommerceForbidden
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	users, err := lockColumnCommerceUsers(ctx, tx, buyer, owner)
	if err != nil {
		return nil, err
	}
	if !users[buyer].active || !users[owner].active {
		return nil, service.ErrAssetCommerceForbidden
	}
	existing, err := scanAssetPurchase(tx.QueryRowContext(ctx, `SELECT `+assetPurchaseFields+` FROM capability_asset_purchases WHERE buyer_user_id=$1 AND operation_id=$2`, buyer, in.OperationID))
	if err == nil {
		if existing.AssetID != id || existing.Amount != in.ExpectedPrice {
			return nil, service.ErrAssetCommerceConflict
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !errors.Is(err, service.ErrCapabilityAssetPackageNotFound) {
		return nil, err
	}
	var enabled bool
	if err = tx.QueryRowContext(ctx, `SELECT enabled FROM capability_asset_commerce_policy WHERE id=TRUE FOR SHARE`).Scan(&enabled); err != nil {
		return nil, err
	}
	if !enabled {
		return nil, service.ErrAssetCommerceClosed
	}
	var price, title, status, pricing string
	var eligible bool
	err = tx.QueryRowContext(ctx, `SELECT a.commerce_price::text,a.title,a.status,a.pricing_type,
 (`+assetSellablePackage+` AND a.demo_url='' AND a.source_url='' AND a.template_url='')
 FROM capability_assets a WHERE a.id=$1 AND a.deleted_at IS NULL FOR SHARE OF a`, id).Scan(&price, &title, &status, &pricing, &eligible)
	if err != nil {
		return nil, err
	}
	if status != "listed" || pricing != "paid" || !eligible {
		return nil, service.ErrAssetCommerceForbidden
	}
	if price != in.ExpectedPrice {
		return nil, service.ErrAssetCommerceConflict
	}
	var deliverableVersion int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM capability_asset_versions WHERE asset_id=$1 AND status='published'
 AND file_count>0 AND runtime_kind<>'external_api' ORDER BY id DESC LIMIT 1 FOR SHARE`, id).Scan(&deliverableVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAssetCommerceForbidden
	}
	if err != nil {
		return nil, err
	}
	var already bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM capability_asset_purchases WHERE asset_id=$1 AND buyer_user_id=$2 AND status='active')`, id, buyer).Scan(&already); err != nil {
		return nil, err
	}
	if already {
		return nil, service.ErrAssetCommerceConflict
	}
	var balanceAfter, walletAfter string
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance-$2::numeric,updated_at=NOW()
 WHERE id=$1 AND balance >= $2::numeric RETURNING balance::text`, buyer, price).Scan(&balanceAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAssetCommerceFunds
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_owner_wallets(owner_id) VALUES($1) ON CONFLICT(owner_id) DO NOTHING`, owner); err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, `UPDATE shared_pool_owner_wallets SET available_amount=available_amount+$2::numeric,
 version=version+1,updated_at=NOW() WHERE owner_id=$1 RETURNING available_amount::text`, owner, price).Scan(&walletAfter); err != nil {
		return nil, err
	}
	p, err := scanAssetPurchase(tx.QueryRowContext(ctx, `INSERT INTO capability_asset_purchases(asset_id,buyer_user_id,owner_user_id,operation_id,asset_title,amount,platform_fee,creator_amount)
 VALUES($1,$2,$3,$4,$5,$6::numeric,0,$6::numeric) RETURNING `+assetPurchaseFields, id, buyer, owner, in.OperationID, title, price))
	if err != nil {
		return nil, err
	}
	if err = assetCommerceLedgers(ctx, tx, p, false, buyer, balanceAfter, walletAfter); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return p, nil
}
func assetCommerceLedgers(ctx context.Context, tx *sql.Tx, p *service.AssetPurchase, refund bool, actor int64, balanceAfter, walletAfter string) error {
	source, event, userDelta, walletDelta, state := "capability_asset_purchase", "earning", "-"+p.Amount, p.CreatorAmount, "available"
	if refund {
		source, event, userDelta, walletDelta, state = "capability_asset_refund", "reversal", p.Amount, "-"+p.CreatorAmount, "settled"
	}
	operation := source + "_" + strconv.FormatInt(p.ID, 10)
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_balance_ledger(user_id,source_type,source_id,amount,balance_after,status,note,created_by,posted_at)
 VALUES($1,$2,$3,$4::numeric,$5::numeric,'posted',$6,$7,NOW())`, p.BuyerUserID, source, operation, userDelta, balanceAfter, p.AssetTitle, actor); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO shared_pool_owner_earnings_ledger(owner_id,event_type,operation_id,gross_amount,platform_fee_amount,
 net_amount,wallet_delta,available_after,status,available_at,metadata,posted_at)
 VALUES($1,$2,$3,$4::numeric,0,$4::numeric,$5::numeric,$6::numeric,$7,NOW(),
 jsonb_build_object('source_type',$8::text,'asset_id',$9::bigint,'asset_title',$10::text,'purchase_id',$11::bigint,'actor_id',$12::bigint),NOW())`,
		p.OwnerUserID, event, operation, p.Amount, walletDelta, walletAfter, state, source, p.AssetID, p.AssetTitle, p.ID, actor)
	return err
}
func (r *bizDecipherRepository) RefundAssetPurchase(ctx context.Context, id, purchaseID, actor int64, input service.ColumnRefundInput) (*service.AssetPurchase, error) {
	in, err := service.NormalizeColumnRefund(input)
	if err != nil {
		return nil, service.ErrAssetCommerceInvalid
	}
	p, err := scanAssetPurchase(r.db.QueryRowContext(ctx, `SELECT `+assetPurchaseFields+` FROM capability_asset_purchases WHERE id=$1 AND asset_id=$2`, purchaseID, id))
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	users, err := lockColumnCommerceUsers(ctx, tx, actor, p.BuyerUserID, p.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if !users[actor].active || users[actor].role != service.RoleAdmin {
		return nil, service.ErrAssetCommerceForbidden
	}
	p, err = scanAssetPurchase(tx.QueryRowContext(ctx, `SELECT `+assetPurchaseFields+` FROM capability_asset_purchases WHERE id=$1 FOR UPDATE`, purchaseID))
	if err != nil {
		return nil, err
	}
	if p.Status == "refunded" {
		var operation string
		var admin int64
		if err = tx.QueryRowContext(ctx, `SELECT refund_operation_id,refunded_by FROM capability_asset_purchases WHERE id=$1`, p.ID).Scan(&operation, &admin); err != nil {
			return nil, err
		}
		if admin != actor || operation != in.OperationID || p.RefundReason != in.Reason {
			return nil, service.ErrAssetCommerceConflict
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return p, nil
	}
	var reused bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM capability_asset_purchases WHERE refunded_by=$1 AND refund_operation_id=$2)`, actor, in.OperationID).Scan(&reused); err != nil {
		return nil, err
	}
	if reused {
		return nil, service.ErrAssetCommerceConflict
	}
	var walletAfter, balanceAfter string
	err = tx.QueryRowContext(ctx, `UPDATE shared_pool_owner_wallets SET available_amount=available_amount-$2::numeric,
 version=version+1,updated_at=NOW() WHERE owner_id=$1 AND available_amount >= $2::numeric RETURNING available_amount::text`, p.OwnerUserID, p.CreatorAmount).Scan(&walletAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAssetCommerceFunds
	}
	if err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+$2::numeric,updated_at=NOW() WHERE id=$1 RETURNING balance::text`, p.BuyerUserID, p.Amount).Scan(&balanceAfter); err != nil {
		return nil, err
	}
	p, err = scanAssetPurchase(tx.QueryRowContext(ctx, `UPDATE capability_asset_purchases SET status='refunded',refund_operation_id=$2,
 refund_reason=$3,refunded_by=$4,refunded_at=NOW() WHERE id=$1 RETURNING `+assetPurchaseFields, p.ID, in.OperationID, in.Reason, actor))
	if err != nil {
		return nil, err
	}
	if err = assetCommerceLedgers(ctx, tx, p, true, actor, balanceAfter, walletAfter); err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE shared_pool_owner_earnings_ledger SET status='reversed'
 WHERE owner_id=$1 AND event_type='earning' AND operation_id=$2 AND status IN('available','settled')`, p.OwnerUserID, "capability_asset_purchase_"+strconv.FormatInt(p.ID, 10))
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, service.ErrAssetCommerceConflict
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return p, nil
}
func (r *bizDecipherRepository) ListAssetPurchases(ctx context.Context, id int64, q service.CreatorColumnQuery) ([]service.AssetPurchase, error) {
	if q.ViewerID <= 0 || q.Limit < 1 || q.Limit > 50 || q.Cursor < 0 {
		return nil, service.ErrAssetCommerceInvalid
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+assetPurchaseFields+` FROM capability_asset_purchases
 WHERE asset_id=$1 AND ($2::boolean OR owner_user_id=$3 OR buyer_user_id=$3)
 AND ($4::bigint=0 OR id<$4) ORDER BY id DESC LIMIT $5`, id, q.Admin, q.ViewerID, q.Cursor, q.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.AssetPurchase{}
	for rows.Next() {
		p, e := scanAssetPurchase(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}
