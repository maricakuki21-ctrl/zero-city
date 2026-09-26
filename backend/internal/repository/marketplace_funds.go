package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const marketplaceFundsFields = `order_id,amount::text,currency,status,operation_id,paid_at,released_at,refunded_at`

func scanMarketplaceFunds(row scanner) (*service.MarketplaceFunds, error) {
	f := &service.MarketplaceFunds{}
	err := row.Scan(&f.OrderID, &f.Amount, &f.Currency, &f.Status, &f.OperationID, &f.PaidAt, &f.ReleasedAt, &f.RefundedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return f, err
}
func (r *marketplaceRepository) GetMarketplaceFunds(ctx context.Context, id, actor int64) (*service.MarketplaceFunds, error) {
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM marketplace_orders o WHERE o.id=$1 AND
 (o.buyer_user_id=$2 OR o.seller_user_id=$2 OR EXISTS(SELECT 1 FROM users WHERE id=$2 AND role='admin' AND status='active' AND deleted_at IS NULL)))`, id, actor).Scan(&allowed)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, service.ErrMarketplaceOrderForbidden
	}
	return scanMarketplaceFunds(r.db.QueryRowContext(ctx, `SELECT `+marketplaceFundsFields+` FROM marketplace_order_funds WHERE order_id=$1`, id))
}
func (r *marketplaceRepository) GetMarketplaceFundsPolicy(ctx context.Context) (*service.MarketplaceFundsPolicy, error) {
	p := &service.MarketplaceFundsPolicy{Currency: "USD"}
	err := r.db.QueryRowContext(ctx, `SELECT enabled FROM marketplace_funds_policy WHERE id=TRUE`).Scan(&p.Enabled)
	return p, err
}
func (r *marketplaceRepository) SetMarketplaceFundsPolicy(ctx context.Context, actor int64, in service.MarketplaceFundsPolicyInput) (*service.MarketplaceFundsPolicy, error) {
	if strings.TrimSpace(in.Reason) == "" || len([]rune(in.Reason)) > 1000 {
		return nil, service.ErrMarketplaceFundsInvalid
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
	if !users[actor].active || users[actor].role != service.RoleAdmin {
		return nil, service.ErrMarketplaceOrderRoleDenied
	}
	if _, err = tx.ExecContext(ctx, `UPDATE marketplace_funds_policy SET enabled=$1,updated_at=NOW() WHERE id=TRUE`, in.Enabled); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO marketplace_funds_policy_audit(actor_user_id,enabled,reason) VALUES($1,$2,$3)`, actor, in.Enabled, strings.TrimSpace(in.Reason)); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &service.MarketplaceFundsPolicy{Enabled: in.Enabled, Currency: "USD"}, nil
}

// Every financial transition locks user rows in ID order before orders/wallets.
func lockMarketplaceParticipants(ctx context.Context, tx *sql.Tx, id, actor int64, admin bool) (int64, int64, error) {
	var buyer, seller int64
	err := tx.QueryRowContext(ctx, `SELECT buyer_user_id,seller_user_id FROM marketplace_orders WHERE id=$1`, id).Scan(&buyer, &seller)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, service.ErrMarketplaceOrderNotFound
	}
	if err != nil {
		return 0, 0, err
	}
	users, err := lockColumnCommerceUsers(ctx, tx, buyer, seller, actor)
	if err != nil {
		return 0, 0, err
	}
	if !users[actor].active || admin && users[actor].role != service.RoleAdmin || !admin && actor != buyer && actor != seller {
		return 0, 0, service.ErrMarketplaceOrderRoleDenied
	}
	return buyer, seller, nil
}
func (r *marketplaceRepository) SetMarketplaceFunds(ctx context.Context, id, actor int64, in service.MarketplaceFundsInput) (*service.MarketplaceFunds, error) {
	amount, err := service.NormalizeColumnPrice(in.Amount, false)
	if err != nil || in.Currency != "USD" {
		return nil, service.ErrMarketplaceFundsInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, seller, err := lockMarketplaceParticipants(ctx, tx, id, actor, false)
	if err != nil {
		return nil, err
	}
	if actor != seller {
		return nil, service.ErrMarketplaceOrderRoleDenied
	}
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM marketplace_orders WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return nil, err
	}
	if status != "quoted" {
		return nil, service.ErrMarketplaceOrderInvalidState
	}
	f, err := scanMarketplaceFunds(tx.QueryRowContext(ctx, `SELECT `+marketplaceFundsFields+` FROM marketplace_order_funds WHERE order_id=$1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if f != nil {
		if f.Amount != amount {
			return nil, service.ErrMarketplaceFundsInvalid
		}
		return f, tx.Commit()
	}
	f, err = scanMarketplaceFunds(tx.QueryRowContext(ctx, `INSERT INTO marketplace_order_funds(order_id,amount) VALUES($1,$2::numeric) RETURNING `+marketplaceFundsFields, id, amount))
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO marketplace_order_events(order_id,actor_user_id,event,from_status,to_status,note)
 VALUES($1,$2,'price','quoted','quoted',$3)`, id, actor, "USD "+amount); err != nil {
		return nil, err
	}
	return f, tx.Commit()
}
func (r *marketplaceRepository) PayMarketplaceOrder(ctx context.Context, id, actor int64, input service.MarketplacePaymentInput) (*service.MarketplaceFunds, error) {
	in, err := service.NormalizeMarketplacePayment(input)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	buyer, seller, err := lockMarketplaceParticipants(ctx, tx, id, actor, false)
	if err != nil {
		return nil, err
	}
	if actor != buyer {
		return nil, service.ErrMarketplaceOrderRoleDenied
	}
	var sellerActive bool
	if err = tx.QueryRowContext(ctx, `SELECT status='active' AND deleted_at IS NULL FROM users WHERE id=$1`, seller).Scan(&sellerActive); err != nil {
		return nil, err
	}
	if !sellerActive {
		return nil, service.ErrMarketplaceOrderRoleDenied
	}
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM marketplace_orders WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return nil, err
	}
	f, err := scanMarketplaceFunds(tx.QueryRowContext(ctx, `SELECT `+marketplaceFundsFields+` FROM marketplace_order_funds WHERE order_id=$1 FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if f == nil || f.Amount != in.ExpectedAmount || f.Currency != in.Currency {
		return nil, service.ErrMarketplaceFundsInvalid
	}
	if f.Status != "unpaid" {
		if f.OperationID == nil || *f.OperationID != in.OperationID {
			return nil, service.ErrMarketplaceFundsInvalid
		}
		return f, tx.Commit()
	}
	if status != "quoted" {
		return nil, service.ErrMarketplaceOrderInvalidState
	}
	var enabled bool
	if err = tx.QueryRowContext(ctx, `SELECT enabled FROM marketplace_funds_policy WHERE id=TRUE FOR SHARE`).Scan(&enabled); err != nil {
		return nil, err
	}
	if !enabled {
		return nil, service.ErrMarketplaceFundsClosed
	}
	var balance string
	err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance-$2::numeric,updated_at=NOW() WHERE id=$1 AND balance >= $2::numeric RETURNING balance::text`, buyer, f.Amount).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceFundsInsufficient
	}
	if err != nil {
		return nil, err
	}
	if err = marketplaceBuyerLedger(ctx, tx, buyer, id, actor, "marketplace_order_payment", "-"+f.Amount, balance); err != nil {
		return nil, err
	}
	f, err = scanMarketplaceFunds(tx.QueryRowContext(ctx, `UPDATE marketplace_order_funds SET status='held',operation_id=$2,paid_at=NOW() WHERE order_id=$1 RETURNING `+marketplaceFundsFields, id, in.OperationID))
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE marketplace_orders SET status='confirmed',confirmed_at=NOW(),updated_at=NOW() WHERE id=$1`, id); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO marketplace_order_events(order_id,actor_user_id,event,from_status,to_status,note) VALUES($1,$2,'pay','quoted','confirmed',$3)`, id, actor, "USD "+f.Amount); err != nil {
		return nil, err
	}
	return f, tx.Commit()
}
func marketplaceBuyerLedger(ctx context.Context, tx *sql.Tx, buyer, id, actor int64, source, amount, balance string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO user_balance_ledger(user_id,source_type,source_id,amount,balance_after,status,note,created_by,posted_at)
 VALUES($1,$2,$3,$4::numeric,$5::numeric,'posted',$6,$7,NOW())`, buyer, source, source+"_"+strconv.FormatInt(id, 10), amount, balance, "Marketplace order #"+strconv.FormatInt(id, 10), actor)
	return err
}
func applyMarketplaceFundsTransition(ctx context.Context, tx *sql.Tx, id, buyer, seller, actor int64, action string) error {
	f, err := scanMarketplaceFunds(tx.QueryRowContext(ctx, `SELECT `+marketplaceFundsFields+` FROM marketplace_order_funds WHERE order_id=$1 FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if f == nil {
		if action == "cancel" {
			return nil
		}
		var legacy bool
		if err := tx.QueryRowContext(ctx, `SELECT legacy_cooperation FROM marketplace_orders WHERE id=$1`, id).Scan(&legacy); err != nil {
			return err
		}
		if legacy {
			return nil
		}
		return service.ErrMarketplaceFundsInvalid
	}
	if f.Status == "unpaid" {
		if action == "cancel" {
			return nil
		}
		return service.ErrMarketplaceFundsInvalid
	}
	if f.Status == "released" {
		if action == "settle" {
			return nil
		}
		return service.ErrMarketplaceFundsInvalid
	}
	if f.Status != "held" {
		return service.ErrMarketplaceFundsInvalid
	}
	if action == "accept" {
		if _, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_owner_wallets(owner_id) VALUES($1) ON CONFLICT(owner_id) DO NOTHING`, seller); err != nil {
			return err
		}
		var available string
		if err = tx.QueryRowContext(ctx, `UPDATE shared_pool_owner_wallets SET available_amount=available_amount+$2::numeric,version=version+1,updated_at=NOW() WHERE owner_id=$1 RETURNING available_amount::text`, seller, f.Amount).Scan(&available); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_owner_earnings_ledger(owner_id,event_type,operation_id,gross_amount,platform_fee_amount,net_amount,wallet_delta,available_after,status,available_at,metadata,posted_at)
 VALUES($1,'earning',$2,$3::numeric,0,$3::numeric,$3::numeric,$4::numeric,'available',NOW(),jsonb_build_object('source_type','marketplace_order_settlement','order_id',$5::bigint,'title',(SELECT l.title FROM marketplace_orders o JOIN marketplace_listings l ON l.id=o.listing_id WHERE o.id=$5)),NOW())`, seller, "marketplace_order_settlement_"+strconv.FormatInt(id, 10), f.Amount, available, id); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE marketplace_order_funds SET status='released',released_at=NOW() WHERE order_id=$1`, id)
		return err
	}
	if action == "cancel" {
		var balance string
		if err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance+$2::numeric,updated_at=NOW() WHERE id=$1 RETURNING balance::text`, buyer, f.Amount).Scan(&balance); err != nil {
			return err
		}
		if err = marketplaceBuyerLedger(ctx, tx, buyer, id, actor, "marketplace_order_refund", f.Amount, balance); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE marketplace_order_funds SET status='refunded',refunded_at=NOW() WHERE order_id=$1`, id)
		return err
	}
	return nil
}
