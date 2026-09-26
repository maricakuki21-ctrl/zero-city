package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type creditSystemRepository struct {
	db *sql.DB
}

func NewCreditSystemRepository(db *sql.DB) service.CreditSystemRepository {
	return &creditSystemRepository{db: db}
}

func (r *creditSystemRepository) GetCreditSystemOverview(ctx context.Context) (*service.CreditSystemOverview, error) {
	overview := &service.CreditSystemOverview{}
	if err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(credit_balance), 0)::double precision,
       COUNT(*) FILTER (WHERE credit_balance > 0)
FROM users
WHERE deleted_at IS NULL`).Scan(&overview.TotalCreditBalance, &overview.PositiveCreditUsers); err != nil {
		return nil, fmt.Errorf("query credit users overview: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM credit_ledger`).Scan(&overview.LedgerEntryCount); err != nil {
		return nil, fmt.Errorf("query credit ledger count: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0)::double precision,
       COALESCE(SUM(CASE WHEN amount < 0 THEN ABS(amount) ELSE 0 END), 0)::double precision
FROM credit_ledger
WHERE created_at >= date_trunc('day', NOW())`).Scan(&overview.TodayGrantedCredits, &overview.TodayConsumedCredits); err != nil {
		return nil, fmt.Errorf("query today credit ledger overview: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT source_type,
       COUNT(*)::bigint,
       COALESCE(SUM(amount), 0)::double precision
FROM credit_ledger
GROUP BY source_type
ORDER BY COUNT(*) DESC, source_type ASC
LIMIT 20`)
	if err != nil {
		return nil, fmt.Errorf("query credit source distribution: %w", err)
	}
	defer func() { _ = rows.Close() }()
	overview.SourceTypeDistribution = make([]service.CreditSourceDistribution, 0)
	for rows.Next() {
		var item service.CreditSourceDistribution
		if err := rows.Scan(&item.SourceType, &item.Count, &item.Amount); err != nil {
			return nil, err
		}
		overview.SourceTypeDistribution = append(overview.SourceTypeDistribution, item)
	}
	return overview, rows.Err()
}

func (r *creditSystemRepository) ListCreditLedger(ctx context.Context, filter service.CreditLedgerFilter) ([]service.BizCreditLedgerEntry, int64, error) {
	where, args := buildCreditLedgerWhere(filter)
	countSQL := "SELECT COUNT(*) FROM credit_ledger cl JOIN users u ON u.id = cl.user_id " + where
	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count credit ledger: %w", err)
	}

	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.db.QueryContext(ctx, `
SELECT cl.id, cl.user_id, cl.source_type, cl.source_id, cl.amount::double precision, cl.balance_after::double precision, cl.status, cl.note, cl.created_by, cl.created_at, cl.posted_at
FROM credit_ledger cl
JOIN users u ON u.id = cl.user_id
`+where+`
ORDER BY cl.created_at DESC
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list credit ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items, err := scanCreditRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *creditSystemRepository) GetCreditUserSummary(ctx context.Context, userID int64) (*service.CreditUserSummary, error) {
	var out service.CreditUserSummary
	if err := r.db.QueryRowContext(ctx, `
SELECT id, COALESCE(email, ''), COALESCE(username, ''), credit_balance::double precision
FROM users
WHERE id = $1 AND deleted_at IS NULL
LIMIT 1`, userID).Scan(&out.UserID, &out.Email, &out.Username, &out.CreditBalance); err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrUserNotFound
		}
		return nil, fmt.Errorf("query credit user summary: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, source_type, source_id, amount::double precision, balance_after::double precision, status, note, created_by, created_at, posted_at
FROM credit_ledger
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 50`, userID)
	if err != nil {
		return nil, fmt.Errorf("query credit user ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()
	ledger, err := scanCreditRows(rows)
	if err != nil {
		return nil, err
	}
	out.Ledger = ledger
	return &out, nil
}

func (r *creditSystemRepository) GrantCredit(ctx context.Context, input service.BizCreditGrantInput) (*service.BizCreditLedgerEntry, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var balance float64
	if err := tx.QueryRowContext(ctx, `
UPDATE users
SET credit_balance = credit_balance + $1,
    updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL
RETURNING credit_balance`, input.Amount, input.UserID).Scan(&balance); err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `
INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, created_by, posted_at)
VALUES ($1, $2, $3, $4, $5, 'posted', $6, $7, NOW())
RETURNING id, user_id, source_type, source_id, amount::double precision, balance_after::double precision, status, note, created_by, created_at, posted_at`, input.UserID, input.SourceType, input.SourceID, input.Amount, balance, input.Note, input.CreatedBy)
	entry, err := scanCredit(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return entry, nil
}

func buildCreditLedgerWhere(filter service.CreditLedgerFilter) (string, []any) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	if filter.UserID > 0 {
		args = append(args, filter.UserID)
		clauses = append(clauses, fmt.Sprintf("cl.user_id = $%d", len(args)))
	}
	if filter.SourceType != "" {
		args = append(args, filter.SourceType)
		clauses = append(clauses, fmt.Sprintf("cl.source_type = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("cl.status = $%d", len(args)))
	}
	if filter.StartAt != nil {
		args = append(args, *filter.StartAt)
		clauses = append(clauses, fmt.Sprintf("cl.created_at >= $%d", len(args)))
	}
	if filter.EndAt != nil {
		args = append(args, *filter.EndAt)
		clauses = append(clauses, fmt.Sprintf("cl.created_at <= $%d", len(args)))
	}
	search := strings.TrimSpace(filter.Search)
	if search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		idx := len(args)
		clauses = append(clauses, fmt.Sprintf("(LOWER(u.email) LIKE $%d OR LOWER(u.username) LIKE $%d OR cl.user_id::text LIKE $%d OR LOWER(cl.source_type) LIKE $%d OR LOWER(cl.source_id) LIKE $%d OR LOWER(cl.note) LIKE $%d)", idx, idx, idx, idx, idx, idx))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}
