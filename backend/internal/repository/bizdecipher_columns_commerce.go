package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const columnPurchaseFields = `id,column_id,buyer_user_id,owner_user_id,operation_id,column_title,
 amount::text,platform_fee::text,creator_amount::text,status,created_at,refunded_at,refund_reason`

func scanColumnPurchase(row communityPollScanner) (*service.ColumnPurchase, error) {
	p := &service.ColumnPurchase{}
	err := row.Scan(&p.ID, &p.ColumnID, &p.BuyerUserID, &p.OwnerUserID, &p.OperationID, &p.ColumnTitle,
		&p.Amount, &p.PlatformFee, &p.CreatorAmount, &p.Status, &p.CreatedAt, &p.RefundedAt, &p.RefundReason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCreatorColumnNotFound
	}
	return p, err
}
func (r *bizDecipherRepository) GetColumnCommercePolicy(ctx context.Context) (*service.ColumnCommercePolicy, error) {
	p := &service.ColumnCommercePolicy{Currency: "USD", PlatformFee: "0.00000000"}
	err := r.db.QueryRowContext(ctx, `SELECT enabled FROM creator_column_commerce_policy WHERE id=TRUE`).Scan(&p.Enabled)
	return p, err
}
func (r *bizDecipherRepository) SetColumnCommercePolicy(ctx context.Context, actor int64, enabled bool, reason string) (*service.ColumnCommercePolicy, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 1000 {
		return nil, service.ErrColumnCommerceInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = columnCommerceAdmin(ctx, tx, actor); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE creator_column_commerce_policy SET enabled=$1,updated_at=NOW() WHERE id=TRUE`, enabled); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creator_column_commerce_audit(actor_user_id,action,detail)
 VALUES($1,'policy',jsonb_build_object('enabled',$2::boolean,'reason',$3::text))`, actor, enabled, reason); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &service.ColumnCommercePolicy{Enabled: enabled, Currency: "USD", PlatformFee: "0.00000000"}, nil
}
func columnCommerceAdmin(ctx context.Context, tx *sql.Tx, actor int64) error {
	var role string
	err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL FOR NO KEY UPDATE`, actor).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) || err == nil && role != service.RoleAdmin {
		return service.ErrColumnCommerceForbidden
	}
	return err
}
func (r *bizDecipherRepository) SetColumnPricing(ctx context.Context, id, actor int64, input service.ColumnPricingInput) (*service.CreatorColumn, error) {
	in, err := service.NormalizeColumnPricing(input)
	if err != nil {
		return nil, err
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
		return nil, service.ErrColumnCommerceForbidden
	}
	result, err := tx.ExecContext(ctx, `UPDATE creator_columns SET mode=$3,price=$4::numeric,updated_at=NOW()
 WHERE id=$1 AND owner_user_id=$2 AND status='active'`, id, actor, in.Mode, in.Price)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, service.ErrColumnCommerceForbidden
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO creator_column_commerce_audit(actor_user_id,column_id,action,detail)
 VALUES($1,$2,'pricing',jsonb_build_object('mode',$3::text,'price',$4::text,'platform_fee','0.00000000'))`, actor, id, in.Mode, in.Price); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetCreatorColumn(ctx, id, service.CreatorColumnQuery{ViewerID: actor})
}

type columnCommerceUser struct {
	role   string
	active bool
}

func lockColumnCommerceUsers(ctx context.Context, tx *sql.Tx, ids ...int64) (map[int64]columnCommerceUser, error) {
	// Same users-before-wallet ordering as withdrawals. NO KEY UPDATE also
	// permits concurrent article inserts' FK key-share locks on their author.
	rows, err := tx.QueryContext(ctx, `SELECT id,role,(status='active' AND deleted_at IS NULL)
 FROM users WHERE id=ANY($1) ORDER BY id FOR NO KEY UPDATE`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := map[int64]columnCommerceUser{}
	for rows.Next() {
		var id int64
		var u columnCommerceUser
		if err = rows.Scan(&id, &u.role, &u.active); err != nil {
			return nil, err
		}
		users[id] = u
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, ok := users[id]; !ok {
			return nil, service.ErrColumnCommerceForbidden
		}
	}
	return users, nil
}

func (r *bizDecipherRepository) PurchaseColumn(ctx context.Context, id, buyer int64, input service.ColumnPurchaseInput) (*service.ColumnPurchase, error) {
	in, err := service.NormalizeColumnPurchase(input)
	if err != nil {
		return nil, err
	}
	// Column authorship is immutable. Resolve identities before taking the
	// ordered user locks; status and price are rechecked under the column lock.
	var owner int64
	err = r.db.QueryRowContext(ctx, `SELECT owner_user_id FROM creator_columns WHERE id=$1`, id).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCreatorColumnNotFound
	}
	if err != nil {
		return nil, err
	}
	if owner == buyer {
		return nil, service.ErrColumnCommerceForbidden
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
		return nil, service.ErrColumnCommerceForbidden
	}
	existing, err := scanColumnPurchase(tx.QueryRowContext(ctx, `SELECT `+columnPurchaseFields+` FROM creator_column_purchases WHERE buyer_user_id=$1 AND operation_id=$2`, buyer, in.OperationID))
	if err == nil {
		if existing.ColumnID != id || existing.Amount != in.ExpectedPrice {
			return nil, service.ErrColumnCommerceConflict
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !errors.Is(err, service.ErrCreatorColumnNotFound) {
		return nil, err
	}
	var enabled bool
	if err = tx.QueryRowContext(ctx, `SELECT enabled FROM creator_column_commerce_policy WHERE id=TRUE FOR SHARE`).Scan(&enabled); err != nil {
		return nil, err
	}
	if !enabled {
		return nil, service.ErrColumnCommerceClosed
	}
	var mode, status, title, price string
	if err = tx.QueryRowContext(ctx, `SELECT mode,status,title,price::text FROM creator_columns WHERE id=$1 FOR SHARE`, id).Scan(&mode, &status, &title, &price); err != nil {
		return nil, err
	}
	if status != "active" || mode != "paid" {
		return nil, service.ErrColumnCommerceForbidden
	}
	if price != in.ExpectedPrice {
		return nil, service.ErrColumnCommerceConflict
	}
	var already bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM creator_column_purchases WHERE column_id=$1 AND buyer_user_id=$2 AND status='active')`, id, buyer).Scan(&already); err != nil {
		return nil, err
	}
	if already {
		return nil, service.ErrColumnCommerceConflict
	}
	var balanceAfter string
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance-$2::numeric,updated_at=NOW()
 WHERE id=$1 AND balance >= $2::numeric RETURNING balance::text`, buyer, price).Scan(&balanceAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrColumnCommerceFunds
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_owner_wallets(owner_id) VALUES($1) ON CONFLICT(owner_id) DO NOTHING`, owner); err != nil {
		return nil, err
	}
	var walletAfter string
	if err = tx.QueryRowContext(ctx, `UPDATE shared_pool_owner_wallets SET available_amount=available_amount+$2::numeric,
 version=version+1,updated_at=NOW() WHERE owner_id=$1 RETURNING available_amount::text`, owner, price).Scan(&walletAfter); err != nil {
		return nil, err
	}
	p, err := scanColumnPurchase(tx.QueryRowContext(ctx, `INSERT INTO creator_column_purchases(column_id,buyer_user_id,owner_user_id,operation_id,column_title,amount,platform_fee,creator_amount)
 VALUES($1,$2,$3,$4,$5,$6::numeric,0,$6::numeric) RETURNING `+columnPurchaseFields, id, buyer, owner, in.OperationID, title, price))
	if err != nil {
		return nil, err
	}
	if err = columnCommerceLedgers(ctx, tx, p, false, buyer, balanceAfter, walletAfter); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return p, nil
}

func columnCommerceLedgers(ctx context.Context, tx *sql.Tx, p *service.ColumnPurchase, refund bool, actor int64, balanceAfter, walletAfter string) error {
	source, kind, userDelta, walletDelta := "creator_column_purchase", "earning", "-"+p.Amount, p.CreatorAmount
	walletStatus := "available"
	if refund {
		source, kind, userDelta, walletDelta = "creator_column_refund", "reversal", p.Amount, "-"+p.CreatorAmount
		walletStatus = "settled"
	}
	operation := source + "_" + strconv.FormatInt(p.ID, 10)
	_, err := tx.ExecContext(ctx, `INSERT INTO user_balance_ledger(user_id,source_type,source_id,amount,balance_after,status,note,created_by,posted_at)
 VALUES($1,$2,$3,$4::numeric,$5::numeric,'posted',$6,$7,NOW())`, p.BuyerUserID, source, operation, userDelta, balanceAfter, p.ColumnTitle, actor)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_owner_earnings_ledger(owner_id,event_type,operation_id,
 gross_amount,platform_fee_amount,net_amount,wallet_delta,available_after,status,available_at,metadata,posted_at)
 VALUES($1,$2,$3,$4::numeric,$5::numeric,$6::numeric,$7::numeric,$8::numeric,$9,NOW(),
 jsonb_build_object('source_type',$10::text,'column_id',$11::bigint,'column_title',$12::text,'purchase_id',$13::bigint,'actor_id',$14::bigint),NOW())`,
		p.OwnerUserID, kind, operation, p.Amount, p.PlatformFee, p.CreatorAmount, walletDelta, walletAfter, walletStatus, source, p.ColumnID, p.ColumnTitle, p.ID, actor)
	return err
}

func (r *bizDecipherRepository) RefundColumnPurchase(ctx context.Context, columnID, purchaseID, actor int64, input service.ColumnRefundInput) (*service.ColumnPurchase, error) {
	in, err := service.NormalizeColumnRefund(input)
	if err != nil {
		return nil, err
	}
	p, err := scanColumnPurchase(r.db.QueryRowContext(ctx, `SELECT `+columnPurchaseFields+` FROM creator_column_purchases WHERE id=$1 AND column_id=$2`, purchaseID, columnID))
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	users, err := lockColumnCommerceUsers(ctx, tx, p.BuyerUserID, p.OwnerUserID, actor)
	if err != nil {
		return nil, err
	}
	if !users[actor].active || users[actor].role != service.RoleAdmin {
		return nil, service.ErrColumnCommerceForbidden
	}
	p, err = scanColumnPurchase(tx.QueryRowContext(ctx, `SELECT `+columnPurchaseFields+` FROM creator_column_purchases WHERE id=$1 FOR UPDATE`, purchaseID))
	if err != nil {
		return nil, err
	}
	if p.Status == "refunded" {
		var operation string
		var admin int64
		if err = tx.QueryRowContext(ctx, `SELECT refund_operation_id,refunded_by FROM creator_column_purchases WHERE id=$1`, p.ID).Scan(&operation, &admin); err != nil {
			return nil, err
		}
		if operation != in.OperationID || admin != actor || p.RefundReason != in.Reason {
			return nil, service.ErrColumnCommerceConflict
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return p, nil
	}
	var reused bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM creator_column_purchases WHERE refunded_by=$1 AND refund_operation_id=$2)`, actor, in.OperationID).Scan(&reused); err != nil {
		return nil, err
	}
	if reused {
		return nil, service.ErrColumnCommerceConflict
	}
	var walletAfter string
	err = tx.QueryRowContext(ctx, `UPDATE shared_pool_owner_wallets SET available_amount=available_amount-$2::numeric,
 version=version+1,updated_at=NOW() WHERE owner_id=$1 AND available_amount >= $2::numeric RETURNING available_amount::text`, p.OwnerUserID, p.CreatorAmount).Scan(&walletAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrColumnCommerceFunds
	}
	if err != nil {
		return nil, err
	}
	var balanceAfter string
	if err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+$2::numeric,updated_at=NOW() WHERE id=$1 RETURNING balance::text`, p.BuyerUserID, p.Amount).Scan(&balanceAfter); err != nil {
		return nil, err
	}
	p, err = scanColumnPurchase(tx.QueryRowContext(ctx, `UPDATE creator_column_purchases SET status='refunded',refund_operation_id=$2,
 refund_reason=$3,refunded_by=$4,refunded_at=NOW() WHERE id=$1 RETURNING `+columnPurchaseFields, p.ID, in.OperationID, in.Reason, actor))
	if err != nil {
		return nil, err
	}
	if err = columnCommerceLedgers(ctx, tx, p, true, actor, balanceAfter, walletAfter); err != nil {
		return nil, err
	}
	// Existing wallet summaries exclude reversed earnings. Keep the original
	// receipt and add the reversal while making the same summary accurate.
	result, err := tx.ExecContext(ctx, `UPDATE shared_pool_owner_earnings_ledger SET status='reversed'
 WHERE owner_id=$1 AND event_type='earning' AND operation_id=$2 AND status='available'`,
		p.OwnerUserID, "creator_column_purchase_"+strconv.FormatInt(p.ID, 10))
	if err != nil {
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, service.ErrColumnCommerceConflict
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *bizDecipherRepository) ListColumnPurchases(ctx context.Context, id int64, q service.CreatorColumnQuery) ([]service.ColumnPurchase, error) {
	if q.ViewerID <= 0 || q.Limit < 1 || q.Limit > 50 || q.Cursor < 0 {
		return nil, service.ErrColumnCommerceInvalid
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+columnPurchaseFields+` FROM creator_column_purchases
 WHERE column_id=$1 AND ($2::boolean OR owner_user_id=$3 OR buyer_user_id=$3)
 AND ($4::bigint=0 OR id<$4) ORDER BY id DESC LIMIT $5`, id, q.Admin, q.ViewerID, q.Cursor, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("list column purchases: %w", err)
	}
	defer rows.Close()
	items := []service.ColumnPurchase{}
	for rows.Next() {
		p, e := scanColumnPurchase(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, *p)
	}
	return items, rows.Err()
}
