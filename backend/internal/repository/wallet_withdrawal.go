package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const withdrawalColumns = `id,owner_id,operation_id,amount::text,channel,recipient,status,reference,reason,created_at,updated_at,COALESCE(processing_by,0)`

func scanWithdrawal(s scanner) (*service.Withdrawal, error) {
	var w service.Withdrawal
	err := s.Scan(&w.ID, &w.OwnerID, &w.OperationID, &w.Amount, &w.Channel, &w.Recipient, &w.Status, &w.Reference, &w.Reason, &w.CreatedAt, &w.UpdatedAt, &w.ProcessingBy)
	return &w, err
}
func (r *bizDecipherRepository) WithdrawalPolicy(ctx context.Context) (*service.WithdrawalPolicy, error) {
	var p service.WithdrawalPolicy
	var channels []byte
	err := r.db.QueryRowContext(ctx, `SELECT enabled,channels,instructions FROM shared_pool_withdrawal_policy WHERE id=TRUE`).Scan(&p.Enabled, &channels, &p.Instructions)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(channels, &p.Channels)
	return &p, err
}
func (r *bizDecipherRepository) SaveWithdrawalPolicy(ctx context.Context, actor int64, p service.WithdrawalPolicy) error {
	if err := service.ValidateWithdrawalPolicy(p); err != nil {
		return err
	}
	if p.Channels == nil {
		p.Channels = []string{}
	}
	channels, _ := json.Marshal(p.Channels)
	snapshot, _ := json.Marshal(p)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE shared_pool_withdrawal_policy SET enabled=$1,channels=$2::jsonb,instructions=$3,updated_by=$4,updated_at=NOW() WHERE id=TRUE`, p.Enabled, string(channels), p.Instructions, actor); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_withdrawal_audit(actor_id,action,reason) VALUES($1,'policy',$2)`, actor, string(snapshot)); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *bizDecipherRepository) ListWithdrawals(ctx context.Context, owner, before int64) ([]service.Withdrawal, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+withdrawalColumns+` FROM shared_pool_withdrawals WHERE ($1::bigint=0 OR owner_id=$1) AND ($2::bigint=0 OR id<$2) ORDER BY id DESC LIMIT 100`, owner, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.Withdrawal{}
	for rows.Next() {
		w, e := scanWithdrawal(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, *w)
	}
	return out, rows.Err()
}
func withdrawalLedger(ctx context.Context, tx *sql.Tx, w *service.Withdrawal, kind, delta, after string, actor int64) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO shared_pool_owner_earnings_ledger(owner_id,event_type,operation_id,wallet_delta,available_after,status,metadata,posted_at)
 VALUES($1,$2,$3,$4::numeric,$5::numeric,'settled',jsonb_build_object('withdrawal_id',$6::bigint,'actor_id',$7::bigint),NOW())`,
		w.OwnerID, kind, "withdrawal_"+strconv.FormatInt(w.ID, 10), delta, after, w.ID, actor)
	return err
}
func (r *bizDecipherRepository) CreateWithdrawal(ctx context.Context, owner int64, input service.WithdrawalInput) (*service.Withdrawal, error) {
	in, err := service.NormalizeWithdrawalInput(input)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Serialize all withdrawals for an owner, including exact retries, against transfers.
	var userID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, owner).Scan(&userID); err != nil {
		return nil, err
	}
	var available string
	if err = tx.QueryRowContext(ctx, `SELECT available_amount::text FROM shared_pool_owner_wallets WHERE owner_id=$1 FOR UPDATE`, owner).Scan(&available); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrInsufficientBalance
		}
		return nil, err
	}
	existing, e := scanWithdrawal(tx.QueryRowContext(ctx, `SELECT `+withdrawalColumns+` FROM shared_pool_withdrawals WHERE owner_id=$1 AND operation_id=$2`, owner, in.OperationID))
	if e == nil {
		if existing.Amount != in.Amount || existing.Channel != in.Channel || existing.Recipient != in.Recipient {
			return nil, service.ErrWithdrawalConflict
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	var enabled bool
	var channels []byte
	if err = tx.QueryRowContext(ctx, `SELECT enabled,channels FROM shared_pool_withdrawal_policy WHERE id=TRUE FOR SHARE`).Scan(&enabled, &channels); err != nil {
		return nil, err
	}
	var allowed []string
	if err = json.Unmarshal(channels, &allowed); err != nil {
		return nil, err
	}
	found := false
	for _, c := range allowed {
		if c == in.Channel {
			found = true
		}
	}
	if !enabled {
		return nil, service.ErrWithdrawalClosed
	}
	if !found {
		return nil, service.ErrWithdrawalInvalid
	}
	if err = tx.QueryRowContext(ctx, `UPDATE shared_pool_owner_wallets SET available_amount=available_amount-$2::numeric,version=version+1,updated_at=NOW() WHERE owner_id=$1 AND available_amount >= $2::numeric RETURNING available_amount::text`, owner, in.Amount).Scan(&available); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrInsufficientBalance
		}
		return nil, err
	}
	w, err := scanWithdrawal(tx.QueryRowContext(ctx, `INSERT INTO shared_pool_withdrawals(owner_id,operation_id,amount,channel,recipient) VALUES($1,$2,$3::numeric,$4,$5) RETURNING `+withdrawalColumns, owner, in.OperationID, in.Amount, in.Channel, in.Recipient))
	if err != nil {
		return nil, err
	}
	if err = withdrawalLedger(ctx, tx, w, "withdrawal", "-"+in.Amount, available, owner); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_withdrawal_audit(withdrawal_id,actor_id,action) VALUES($1,$2,'pending')`, w.ID, owner); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return w, nil
}
func (r *bizDecipherRepository) ActWithdrawal(ctx context.Context, actor, id int64, admin bool, a service.WithdrawalAction) (*service.Withdrawal, error) {
	a.Reason = strings.TrimSpace(a.Reason)
	a.Reference = strings.TrimSpace(a.Reference)
	if len(a.Reason) > 1000 || len(a.Reference) > 200 || (!admin && a.Action != "cancelled") {
		return nil, service.ErrWithdrawalInvalid
	}
	if a.Action != "cancelled" && a.Action != "processing" && a.Action != "rejected" && a.Action != "paid" {
		return nil, service.ErrWithdrawalInvalid
	}
	if (a.Action == "paid" && a.Reference == "") || (a.Action == "rejected" && a.Reason == "") {
		return nil, service.ErrWithdrawalInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// Transfers lock users before wallets. Lock both the owner and audit actor
	// in deterministic order before the request/wallet to avoid FK lock cycles.
	locked, err := tx.QueryContext(ctx, `SELECT id FROM users WHERE id IN ($1,(SELECT owner_id FROM shared_pool_withdrawals WHERE id=$2 AND ($3::boolean OR owner_id=$1))) ORDER BY id FOR UPDATE`, actor, id, admin)
	if err != nil {
		return nil, err
	}
	for locked.Next() {
		var userID int64
		if err = locked.Scan(&userID); err != nil {
			_ = locked.Close()
			return nil, err
		}
	}
	err = locked.Err()
	_ = locked.Close()
	if err != nil {
		return nil, err
	}
	w, err := scanWithdrawal(tx.QueryRowContext(ctx, `SELECT `+withdrawalColumns+` FROM shared_pool_withdrawals WHERE id=$1 AND ($2::boolean OR owner_id=$3) FOR UPDATE`, id, admin, actor))
	if err != nil {
		return nil, err
	}
	// A processing claim belongs to one operator. Another operator must not
	// mistake a replay for a successful claim and send a second external payout.
	if w.ProcessingBy != 0 && w.ProcessingBy != actor {
		return nil, service.ErrWithdrawalConflict
	}
	if w.Status == a.Action {
		if w.Reason != a.Reason || w.Reference != a.Reference {
			return nil, service.ErrWithdrawalConflict
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return w, nil
	}
	allowed := w.Status == "pending" && (a.Action == "cancelled" || a.Action == "processing" || a.Action == "rejected")
	// Once a payout may have been sent, require affirmative verification and
	// an external investigation reference before returning the funds.
	allowed = allowed || (w.Status == "processing" && admin && a.Action == "paid")
	allowed = allowed || (w.Status == "processing" && admin && a.Action == "rejected" && a.NoPaymentConfirmed && a.Reference != "")
	if !allowed {
		return nil, service.ErrWithdrawalConflict
	}
	if a.Action == "cancelled" || a.Action == "rejected" {
		var after string
		if err = tx.QueryRowContext(ctx, `UPDATE shared_pool_owner_wallets SET available_amount=available_amount+$2::numeric,version=version+1,updated_at=NOW() WHERE owner_id=$1 RETURNING available_amount::text`, w.OwnerID, w.Amount).Scan(&after); err != nil {
			return nil, err
		}
		if err = withdrawalLedger(ctx, tx, w, "withdrawal_return", w.Amount, after, actor); err != nil {
			return nil, err
		}
	}
	w, err = scanWithdrawal(tx.QueryRowContext(ctx, `UPDATE shared_pool_withdrawals SET status=$2::varchar,reason=$3,reference=$4,processing_by=CASE WHEN $2::varchar='processing'::varchar THEN $5::bigint ELSE processing_by END,updated_at=NOW() WHERE id=$1 RETURNING `+withdrawalColumns, id, a.Action, a.Reason, a.Reference, actor))
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_withdrawal_audit(withdrawal_id,actor_id,action,reason,reference) VALUES($1,$2,$3,$4,$5)`, id, actor, a.Action, a.Reason, a.Reference); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return w, nil
}
