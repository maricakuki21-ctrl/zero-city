package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const marketplaceOrderColumns = `
	o.id, o.inquiry_id, o.listing_id, l.title,
	o.buyer_user_id,
	COALESCE(NULLIF(bp.display_name, ''), NULLIF(bu.username, ''), split_part(bu.email, '@', 1), 'User'),
	o.seller_user_id,
	COALESCE(NULLIF(sp.display_name, ''), NULLIF(su.username, ''), split_part(su.email, '@', 1), 'User'),
	o.status, o.scope_text, o.amount_text, o.delivery_text, o.revision_limit,
	o.delivery_note, o.accept_note, o.dispute_note, o.cancel_reason,
	o.quoted_at, o.confirmed_at, o.delivered_at, o.accepted_at, o.settled_at, o.canceled_at,
	o.created_at, o.updated_at, o.legacy_cooperation`

func marketplaceOrderFrom() string {
	return ` FROM marketplace_orders o
		JOIN marketplace_listings l ON l.id = o.listing_id
		JOIN users bu ON bu.id = o.buyer_user_id
		LEFT JOIN biz_profiles bp ON bp.user_id = o.buyer_user_id
		JOIN users su ON su.id = o.seller_user_id
		LEFT JOIN biz_profiles sp ON sp.user_id = o.seller_user_id`
}

func scanMarketplaceOrder(row scanner) (*service.MarketplaceOrder, error) {
	var order service.MarketplaceOrder
	var confirmedAt, deliveredAt, acceptedAt, settledAt, canceledAt sql.NullTime
	if err := row.Scan(
		&order.ID, &order.InquiryID, &order.ListingID, &order.ListingTitle,
		&order.BuyerUserID, &order.BuyerDisplayName,
		&order.SellerUserID, &order.SellerDisplayName,
		&order.Status, &order.ScopeText, &order.AmountText, &order.DeliveryText, &order.RevisionLimit,
		&order.DeliveryNote, &order.AcceptNote, &order.DisputeNote, &order.CancelReason,
		&order.QuotedAt, &confirmedAt, &deliveredAt, &acceptedAt, &settledAt, &canceledAt,
		&order.CreatedAt, &order.UpdatedAt, &order.LegacyCooperation,
	); err != nil {
		return nil, err
	}
	order.ConfirmedAt = nullTimePtr(confirmedAt)
	order.DeliveredAt = nullTimePtr(deliveredAt)
	order.AcceptedAt = nullTimePtr(acceptedAt)
	order.SettledAt = nullTimePtr(settledAt)
	order.CanceledAt = nullTimePtr(canceledAt)
	return &order, nil
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

func (r *marketplaceRepository) CreateMarketplaceOrder(ctx context.Context, inquiryID, sellerID int64, in service.MarketplaceOrderQuoteInput) (*service.MarketplaceOrder, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var listingID, buyerID, ownerID int64
	err = tx.QueryRowContext(ctx, `SELECT i.listing_id, i.initiator_user_id, i.listing_owner_user_id
		FROM marketplace_inquiries i WHERE i.id = $1`, inquiryID).Scan(&listingID, &buyerID, &ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceInquiryUnavailable
	}
	if err != nil {
		return nil, err
	}
	// Only the seller quotes, and only on their own inquiry.
	if ownerID != sellerID {
		return nil, service.ErrMarketplaceOrderForbidden
	}
	var orderID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO marketplace_orders
		(inquiry_id, listing_id, buyer_user_id, seller_user_id, status, scope_text, amount_text, delivery_text, revision_limit, legacy_cooperation)
		VALUES ($1, $2, $3, $4, 'quoted', $5, $6, $7, $8, FALSE) RETURNING id`,
		inquiryID, listingID, buyerID, sellerID, in.ScopeText, in.AmountText, in.DeliveryText, in.RevisionLimit).Scan(&orderID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, service.ErrMarketplaceOrderExists
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO marketplace_order_events
		(order_id, actor_user_id, event, from_status, to_status, note)
		VALUES ($1, $2, 'quote', '', 'quoted', $3)`, orderID, sellerID, truncateMarketplaceOrderNote(in.ScopeText)); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.getMarketplaceOrder(ctx, orderID, sellerID, true)
}

func (r *marketplaceRepository) ListMarketplaceOrders(ctx context.Context, userID int64, query service.MarketplacePageQuery) (service.MarketplaceOrderPage, error) {
	args := []any{userID}
	where := `(o.buyer_user_id = $1 OR o.seller_user_id = $1)`
	if query.Cursor > 0 {
		args = append(args, query.Cursor)
		where += fmt.Sprintf(" AND o.id < $%d", len(args))
	}
	args = append(args, query.Limit+1)
	rows, err := r.db.QueryContext(ctx, `SELECT `+marketplaceOrderColumns+marketplaceOrderFrom()+
		` WHERE `+where+fmt.Sprintf(" ORDER BY o.id DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return service.MarketplaceOrderPage{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.MarketplaceOrder, 0, query.Limit+1)
	for rows.Next() {
		order, scanErr := scanMarketplaceOrder(rows)
		if scanErr != nil {
			return service.MarketplaceOrderPage{}, scanErr
		}
		order.ViewerRole = service.MarketplaceOrderViewerRole(order, userID)
		items = append(items, *order)
	}
	if err := rows.Err(); err != nil {
		return service.MarketplaceOrderPage{}, err
	}
	page := service.MarketplaceOrderPage{Items: items}
	if len(items) > query.Limit {
		page.Items = items[:query.Limit]
		cursor := page.Items[len(page.Items)-1].ID
		page.NextCursor = &cursor
	}
	if err := r.attachMarketplaceOrderReviews(ctx, userID, page.Items); err != nil {
		return service.MarketplaceOrderPage{}, err
	}
	for i := range page.Items {
		page.Items[i].AvailableActions = service.MarketplaceOrderActions(&page.Items[i], userID)
	}
	return page, nil
}

// attachMarketplaceOrderReviews fetches the page's reviews in one query so the
// available-actions计算 does not fan out into one query per row.
func (r *marketplaceRepository) attachMarketplaceOrderReviews(ctx context.Context, userID int64, orders []service.MarketplaceOrder) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(orders))
	index := make(map[int64]int, len(orders))
	for i := range orders {
		ids = append(ids, orders[i].ID)
		index[orders[i].ID] = i
	}
	rows, err := r.db.QueryContext(ctx, `SELECT r.id, r.order_id, r.author_user_id,
		COALESCE(NULLIF(bp.display_name, ''), NULLIF(u.username, ''), split_part(u.email, '@', 1), 'User'),
		r.rating, r.body, r.created_at
		FROM marketplace_order_reviews r
		JOIN users u ON u.id = r.author_user_id
		LEFT JOIN biz_profiles bp ON bp.user_id = r.author_user_id
		WHERE r.order_id = ANY($1) ORDER BY r.id`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var review service.MarketplaceOrderReview
		if err := rows.Scan(&review.ID, &review.OrderID, &review.AuthorUserID, &review.AuthorName, &review.Rating, &review.Body, &review.CreatedAt); err != nil {
			return err
		}
		if position, ok := index[review.OrderID]; ok {
			orders[position].Reviews = append(orders[position].Reviews, review)
		}
	}
	return rows.Err()
}

func (r *marketplaceRepository) GetMarketplaceOrder(ctx context.Context, orderID, userID int64) (*service.MarketplaceOrder, error) {
	return r.getMarketplaceOrder(ctx, orderID, userID, true)
}

func (r *marketplaceRepository) getMarketplaceOrder(ctx context.Context, orderID, userID int64, withDetail bool) (*service.MarketplaceOrder, error) {
	order, err := scanMarketplaceOrder(r.db.QueryRowContext(ctx, `SELECT `+marketplaceOrderColumns+marketplaceOrderFrom()+` WHERE o.id = $1`, orderID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	order.ViewerRole = service.MarketplaceOrderViewerRole(order, userID)
	if order.ViewerRole == "" {
		return nil, service.ErrMarketplaceOrderForbidden
	}
	if withDetail {
		if err := r.loadMarketplaceOrderEvents(ctx, order); err != nil {
			return nil, err
		}
		batch := []service.MarketplaceOrder{*order}
		if err := r.attachMarketplaceOrderReviews(ctx, userID, batch); err != nil {
			return nil, err
		}
		order.Reviews = batch[0].Reviews
	}
	order.AvailableActions = service.MarketplaceOrderActions(order, userID)
	return order, nil
}

func (r *marketplaceRepository) loadMarketplaceOrderEvents(ctx context.Context, order *service.MarketplaceOrder) error {
	rows, err := r.db.QueryContext(ctx, `SELECT e.id, e.order_id, e.actor_user_id,
		COALESCE(NULLIF(bp.display_name, ''), NULLIF(u.username, ''), split_part(u.email, '@', 1), 'User'),
		e.event, e.from_status, e.to_status, e.note, e.created_at
		FROM marketplace_order_events e
		JOIN users u ON u.id = e.actor_user_id
		LEFT JOIN biz_profiles bp ON bp.user_id = e.actor_user_id
		WHERE e.order_id = $1 ORDER BY e.id`, order.ID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	events := make([]service.MarketplaceOrderEvent, 0, 8)
	for rows.Next() {
		var event service.MarketplaceOrderEvent
		if err := rows.Scan(&event.ID, &event.OrderID, &event.ActorUserID, &event.ActorName, &event.Event, &event.FromStatus, &event.ToStatus, &event.Note, &event.CreatedAt); err != nil {
			return err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	order.Events = events
	return nil
}

func (r *marketplaceRepository) TransitionMarketplaceOrder(ctx context.Context, transition service.MarketplaceOrderTransition) (*service.MarketplaceOrder, error) {
	column, ok := allowedOrderTimestampColumn(transition.TimestampColumn)
	if !ok || transition.NoteColumn != "" && !allowedOrderNoteColumn(transition.NoteColumn) {
		return nil, service.ErrMarketplaceOrderInvalidState
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	buyer, seller, err := lockMarketplaceParticipants(ctx, tx, transition.OrderID, transition.ActorID, false)
	if err != nil {
		return nil, err
	}
	var fromStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM marketplace_orders WHERE id = $1 FOR UPDATE`, transition.OrderID).Scan(&fromStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrMarketplaceOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if !containsOrderStatus(transition.FromStatuses, fromStatus) {
		return nil, service.ErrMarketplaceOrderInvalidState
	}
	// Recheck role under the same locks as state/money, not only in the service.
	role := service.MarketplaceOrderViewerRole(&service.MarketplaceOrder{BuyerUserID: buyer, SellerUserID: seller}, transition.ActorID)
	if (transition.Action == "accept" || transition.Action == "confirm") && role != "buyer" || transition.Action == "deliver" && role != "seller" {
		return nil, service.ErrMarketplaceOrderRoleDenied
	}
	if err := applyMarketplaceFundsTransition(ctx, tx, transition.OrderID, buyer, seller, transition.ActorID, transition.Action); err != nil {
		return nil, err
	}
	assignments := []string{"status = $2", "updated_at = NOW()"}
	args := []any{transition.OrderID, transition.ToStatus}
	if column != "" {
		assignments = append(assignments, column+" = NOW()")
	}
	if transition.NoteColumn != "" {
		args = append(args, transition.Note)
		assignments = append(assignments, fmt.Sprintf("%s = $%d", transition.NoteColumn, len(args)))
	}
	statusArgument := len(args) + 1
	args = append(args, fromStatus)
	result, err := tx.ExecContext(ctx, `UPDATE marketplace_orders SET `+strings.Join(assignments, ", ")+
		fmt.Sprintf(" WHERE id = $1 AND status = $%d", statusArgument), args...)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, service.ErrMarketplaceOrderInvalidState
	}
	note := truncateMarketplaceOrderNote(transition.Note)
	if _, err := tx.ExecContext(ctx, `INSERT INTO marketplace_order_events
		(order_id, actor_user_id, event, from_status, to_status, note)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		transition.OrderID, transition.ActorID, transition.Action, fromStatus, transition.ToStatus, note); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.getMarketplaceOrder(ctx, transition.OrderID, transition.ActorID, true)
}

func (r *marketplaceRepository) CreateMarketplaceOrderReview(ctx context.Context, orderID, authorID int64, rating int, body string) (*service.MarketplaceOrderReview, error) {
	var review service.MarketplaceOrderReview
	err := r.db.QueryRowContext(ctx, `INSERT INTO marketplace_order_reviews (order_id, author_user_id, rating, body)
		VALUES ($1, $2, $3, $4)
		RETURNING id, order_id, author_user_id, rating, body, created_at`,
		orderID, authorID, rating, body).
		Scan(&review.ID, &review.OrderID, &review.AuthorUserID, &review.Rating, &review.Body, &review.CreatedAt)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, service.ErrMarketplaceOrderReviewExists
		}
		return nil, err
	}
	return &review, nil
}

func allowedOrderTimestampColumn(column string) (string, bool) {
	switch column {
	case "confirmed_at", "delivered_at", "accepted_at", "settled_at", "canceled_at":
		return column, true
	case "":
		return "", true
	default:
		return "", false
	}
}

func allowedOrderNoteColumn(column string) bool {
	switch column {
	case "delivery_note", "accept_note", "dispute_note", "cancel_reason":
		return true
	default:
		return false
	}
}

func containsOrderStatus(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func truncateMarketplaceOrderNote(note string) string {
	if utf8.RuneCountInString(note) <= 1000 {
		return note
	}
	return string([]rune(note)[:1000])
}
