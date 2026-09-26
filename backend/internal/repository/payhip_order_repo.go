package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type payhipOrderRepository struct {
	db *sql.DB
}

func NewPayhipOrderRepository(db *sql.DB) service.PayhipOrderRepository {
	return &payhipOrderRepository{db: db}
}

const payhipOrderColumns = `
	id, transaction_id, buyer_email, currency, price_cents, credit_amount,
	product_key, product_name, product_link, payment_type, status, redeem_code,
	claimed_by, claimed_at, refunded_at, amount_refunded, raw_payload, created_at, updated_at
`

func (r *payhipOrderRepository) CreatePaidOrder(ctx context.Context, order *service.PayhipOrder) (*service.PayhipOrder, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, errors.New("payhip repository not configured")
	}
	if order == nil {
		return nil, false, errors.New("payhip order is required")
	}
	query := fmt.Sprintf(`
		INSERT INTO payhip_webhook_orders (
			transaction_id, buyer_email, currency, price_cents, credit_amount,
			product_key, product_name, product_link, payment_type, status, redeem_code,
			raw_payload, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW())
		ON CONFLICT (transaction_id) DO NOTHING
		RETURNING %s`, payhipOrderColumns)
	row := r.db.QueryRowContext(ctx, query,
		order.TransactionID,
		order.BuyerEmail,
		order.Currency,
		order.PriceCents,
		order.CreditAmount,
		order.ProductKey,
		order.ProductName,
		order.ProductLink,
		order.PaymentType,
		order.Status,
		order.RedeemCode,
		order.RawPayload,
		order.CreatedAt,
	)
	created, err := service.ScanPayhipOrderForRepository(row)
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	existing, getErr := r.GetByTransactionID(ctx, order.TransactionID)
	return existing, false, getErr
}

func (r *payhipOrderRepository) MarkRefunded(ctx context.Context, transactionID string, amountRefunded int64, rawPayload string, refundedAt time.Time) (*service.PayhipOrder, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("payhip repository not configured")
	}
	query := fmt.Sprintf(`
		UPDATE payhip_webhook_orders
		SET status = $2, amount_refunded = $3, raw_payload = $4, refunded_at = $5, updated_at = NOW()
		WHERE transaction_id = $1
		RETURNING %s`, payhipOrderColumns)
	order, err := service.ScanPayhipOrderForRepository(r.db.QueryRowContext(ctx, query, transactionID, service.PayhipOrderStatusRefunded, amountRefunded, rawPayload, refundedAt))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPayhipOrderNotFound
	}
	return order, err
}

func (r *payhipOrderRepository) GetByTransactionID(ctx context.Context, transactionID string) (*service.PayhipOrder, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("payhip repository not configured")
	}
	query := fmt.Sprintf(`SELECT %s FROM payhip_webhook_orders WHERE transaction_id = $1`, payhipOrderColumns)
	order, err := service.ScanPayhipOrderForRepository(r.db.QueryRowContext(ctx, query, transactionID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPayhipOrderNotFound
	}
	return order, err
}

func (r *payhipOrderRepository) MarkClaimed(ctx context.Context, transactionID string, userID int64) (*service.PayhipOrder, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("payhip repository not configured")
	}
	query := fmt.Sprintf(`
		UPDATE payhip_webhook_orders
		SET status = CASE WHEN status = $3 THEN status ELSE $2 END,
		    claimed_by = COALESCE(claimed_by, $4),
		    claimed_at = COALESCE(claimed_at, NOW()),
		    updated_at = NOW()
		WHERE transaction_id = $1
		RETURNING %s`, payhipOrderColumns)
	order, err := service.ScanPayhipOrderForRepository(r.db.QueryRowContext(ctx, query, transactionID, service.PayhipOrderStatusClaimed, service.PayhipOrderStatusRefunded, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPayhipOrderNotFound
	}
	return order, err
}
