package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type marketplaceAdminQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func requireMarketplaceAdmin(ctx context.Context, db marketplaceAdminQuerier, adminID int64) error {
	var allowed bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND role = 'admin' AND status = 'active' AND deleted_at IS NULL)`, adminID).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return service.ErrMarketplaceOrderRoleDenied
	}
	return nil
}

func (r *marketplaceRepository) ListMarketplaceDisputes(ctx context.Context, adminID int64, q service.MarketplacePageQuery) (service.MarketplaceOrderPage, error) {
	if err := requireMarketplaceAdmin(ctx, r.db, adminID); err != nil {
		return service.MarketplaceOrderPage{}, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+marketplaceOrderColumns+marketplaceOrderFrom()+`
		WHERE o.status = 'disputed' AND ($1 = 0 OR o.id < $1) ORDER BY o.id DESC LIMIT $2`, q.Cursor, q.Limit+1)
	if err != nil {
		return service.MarketplaceOrderPage{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.MarketplaceOrder, 0)
	for rows.Next() {
		order, err := scanMarketplaceOrder(rows)
		if err != nil {
			return service.MarketplaceOrderPage{}, err
		}
		order.ViewerRole = "admin"
		order.AvailableActions = []string{}
		items = append(items, *order)
	}
	if err := rows.Err(); err != nil {
		return service.MarketplaceOrderPage{}, err
	}
	page := service.MarketplaceOrderPage{Items: items}
	if len(items) > q.Limit {
		page.Items = items[:q.Limit]
		id := page.Items[len(page.Items)-1].ID
		page.NextCursor = &id
	}
	return page, nil
}

func (r *marketplaceRepository) GetMarketplaceDispute(ctx context.Context, orderID, adminID int64) (*service.MarketplaceOrder, error) {
	if err := requireMarketplaceAdmin(ctx, r.db, adminID); err != nil {
		return nil, err
	}
	order, err := scanMarketplaceOrder(r.db.QueryRowContext(ctx, `SELECT `+marketplaceOrderColumns+marketplaceOrderFrom()+`
		WHERE o.id = $1 AND (o.status = 'disputed' OR EXISTS (
			SELECT 1 FROM marketplace_order_events e WHERE e.order_id = o.id AND e.event = 'admin_resolve_dispute'))`, orderID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	order.ViewerRole = "admin"
	order.AvailableActions = []string{}
	if err := r.loadMarketplaceOrderEvents(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (r *marketplaceRepository) ResolveMarketplaceDispute(ctx context.Context, orderID, adminID int64, in service.MarketplaceDisputeResolutionInput) (*service.MarketplaceOrder, error) {
	if (in.Outcome != "confirmed" && in.Outcome != "canceled") || strings.TrimSpace(in.Reason) == "" || utf8.RuneCountInString(in.Reason) > 1000 {
		return nil, service.ErrMarketplaceOrderInvalidState
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	buyer, seller, err := lockMarketplaceParticipants(ctx, tx, orderID, adminID, true)
	if err != nil {
		return nil, err
	}
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM marketplace_orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != service.MarketplaceOrderStatusDisputed {
		return nil, service.ErrMarketplaceOrderInvalidState
	}
	action := "reopen"
	if in.Outcome == "canceled" {
		action = "cancel"
	}
	if err := applyMarketplaceFundsTransition(ctx, tx, orderID, buyer, seller, adminID, action); err != nil {
		return nil, err
	}
	// Reopening keeps held funds and requires fresh delivery and acceptance.
	_, err = tx.ExecContext(ctx, `UPDATE marketplace_orders SET status = $2::varchar, updated_at = NOW(),
		canceled_at = CASE WHEN $2::varchar = 'canceled'::varchar THEN NOW() ELSE NULL END,
		delivered_at = CASE WHEN $2::varchar = 'confirmed'::varchar THEN NULL ELSE delivered_at END,
		accepted_at = CASE WHEN $2::varchar = 'confirmed'::varchar THEN NULL ELSE accepted_at END
		WHERE id = $1 AND status = 'disputed'`, orderID, in.Outcome)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO marketplace_order_events
		(order_id, actor_user_id, event, from_status, to_status, note)
		VALUES ($1, $2, 'admin_resolve_dispute', 'disputed', $3, $4)`, orderID, adminID, in.Outcome, in.Reason)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetMarketplaceDispute(ctx, orderID, adminID)
}
