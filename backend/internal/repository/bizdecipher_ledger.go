package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	platformledger "github.com/Wei-Shaw/sub2api/internal/platform/ledger"
)

type BizDecipherLedgerRepository struct {
	db *sql.DB
}

func NewBizDecipherLedgerRepository(db *sql.DB) *BizDecipherLedgerRepository {
	return &BizDecipherLedgerRepository{db: db}
}

type PostResult struct {
	JournalID string
	Replayed  bool
}

func (r *BizDecipherLedgerRepository) EnsureAccount(ctx context.Context, account platformledger.Account) error {
	validated, err := platformledger.NewAccount(account)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ledger account: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO bizdecipher_ledger_accounts
        (id, owner_id, asset, purpose) VALUES ($1, $2, $3, $4)
        ON CONFLICT (id) DO NOTHING`, validated.ID, validated.OwnerID, validated.Asset, validated.Purpose); err != nil {
		return fmt.Errorf("insert ledger account: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bizdecipher_ledger_projections (account_id)
        VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, validated.ID); err != nil {
		return fmt.Errorf("insert ledger projection: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ledger account: %w", err)
	}
	return nil
}

func (r *BizDecipherLedgerRepository) Post(ctx context.Context, journal platformledger.Journal) (PostResult, error) {
	validated, err := platformledger.NewJournal(journal)
	if err != nil {
		return PostResult{}, err
	}
	return r.postValidated(ctx, validated)
}

func (r *BizDecipherLedgerRepository) postValidated(ctx context.Context, journal platformledger.Journal) (PostResult, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return PostResult{}, fmt.Errorf("begin ledger post: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := r.postJournalTx(ctx, tx, journal)
	if err != nil {
		return PostResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return PostResult{}, fmt.Errorf("commit ledger post: %w", err)
	}
	return result, nil
}

func (r *BizDecipherLedgerRepository) postJournalTx(
	ctx context.Context,
	tx *sql.Tx,
	journal platformledger.Journal,
) (PostResult, error) {
	var insertedID string
	err := tx.QueryRowContext(ctx, `INSERT INTO bizdecipher_ledger_journals
        (id, event_id, idempotency_key, payload_sha256, journal_type, reversal_of, posted_at)
        VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
        ON CONFLICT DO NOTHING RETURNING id`,
		journal.ID, journal.EventID, journal.IdempotencyKey, journal.PayloadSHA256,
		journal.Kind, journal.ReversalOf, journal.PostedAt).Scan(&insertedID)
	if errors.Is(err, sql.ErrNoRows) {
		return r.replayResultTx(ctx, tx, journal)
	}
	if err != nil {
		return PostResult{}, fmt.Errorf("insert ledger journal: %w", err)
	}
	for lineNo, entry := range journal.Entries {
		if _, err := tx.ExecContext(ctx, `INSERT INTO bizdecipher_ledger_entries
            (journal_id, line_no, account_id, amount) VALUES ($1, $2, $3, $4::numeric)`,
			journal.ID, lineNo+1, entry.AccountID, entry.Amount.String()); err != nil {
			return PostResult{}, fmt.Errorf("insert ledger entry %d: %w", lineNo+1, err)
		}
		updateResult, err := tx.ExecContext(ctx, `UPDATE bizdecipher_ledger_projections AS projection
			SET balance = projection.balance + $2::numeric, version = projection.version + 1, updated_at = $3
			FROM bizdecipher_ledger_accounts AS account
			WHERE projection.account_id = $1
			  AND account.id = projection.account_id
			  AND (account.purpose <> 'user_balance'
			       OR projection.balance + $2::numeric >= projection.reserved_amount)`,
			entry.AccountID, entry.Amount.String(), journal.PostedAt)
		if err != nil {
			return PostResult{}, fmt.Errorf("update ledger projection %s: %w", entry.AccountID, err)
		}
		affected, err := updateResult.RowsAffected()
		if err != nil {
			return PostResult{}, fmt.Errorf("count ledger projection update %s: %w", entry.AccountID, err)
		}
		if affected != 1 {
			var purpose string
			err := tx.QueryRowContext(ctx, `SELECT account.purpose
				FROM bizdecipher_ledger_accounts AS account
				JOIN bizdecipher_ledger_projections AS projection ON projection.account_id = account.id
				WHERE account.id = $1`, entry.AccountID).Scan(&purpose)
			if errors.Is(err, sql.ErrNoRows) {
				return PostResult{}, fmt.Errorf("%w: missing projection for account %s", platformledger.ErrInvalidJournal, entry.AccountID)
			}
			if err != nil {
				return PostResult{}, fmt.Errorf("read rejected ledger projection %s: %w", entry.AccountID, err)
			}
			if purpose == string(platformledger.PurposeUserBalance) {
				return PostResult{}, fmt.Errorf("%w: account %s", platformledger.ErrInsufficientFunds, entry.AccountID)
			}
			return PostResult{}, fmt.Errorf("%w: projection update rejected for account %s", platformledger.ErrInvalidJournal, entry.AccountID)
		}
	}
	return PostResult{JournalID: insertedID}, nil
}

func (r *BizDecipherLedgerRepository) replayResultTx(
	ctx context.Context,
	tx *sql.Tx,
	journal platformledger.Journal,
) (PostResult, error) {
	var existingID, eventID, payloadSHA, idempotencyKey string
	if err := tx.QueryRowContext(ctx, `SELECT id, event_id, payload_sha256, idempotency_key
		FROM bizdecipher_ledger_journals WHERE event_id = $1 OR idempotency_key = $2`, journal.EventID, journal.IdempotencyKey).
		Scan(&existingID, &eventID, &payloadSHA, &idempotencyKey); err != nil {
		return PostResult{}, fmt.Errorf("read existing ledger event: %w", err)
	}
	if eventID != journal.EventID || payloadSHA != journal.PayloadSHA256 || idempotencyKey != journal.IdempotencyKey {
		return PostResult{}, platformledger.ErrIdempotencyConflict
	}
	return PostResult{JournalID: existingID, Replayed: true}, nil
}

type HoldRequest struct {
	ReservationID  string
	EventID        string
	AccountID      string
	Amount         billingcontract.Decimal
	AuthorityEpoch uint64
}

func (r *BizDecipherLedgerRepository) PlaceHold(ctx context.Context, request HoldRequest) (bool, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return false, fmt.Errorf("begin ledger hold: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var balance, reserved billingcontract.Decimal
	if err := tx.QueryRowContext(ctx, `SELECT balance, reserved_amount FROM bizdecipher_ledger_projections
        WHERE account_id = $1 FOR UPDATE`, request.AccountID).Scan(&balance, &reserved); err != nil {
		return false, fmt.Errorf("lock ledger projection: %w", err)
	}
	var existingEvent, existingAccount string
	var existingAmount billingcontract.Decimal
	var existingEpoch uint64
	err = tx.QueryRowContext(ctx, `SELECT event_id, account_id, amount, authority_epoch FROM bizdecipher_ledger_holds
		WHERE reservation_id = $1`, request.ReservationID).
		Scan(&existingEvent, &existingAccount, &existingAmount, &existingEpoch)
	if err == nil {
		if existingEvent != request.EventID || existingAccount != request.AccountID ||
			!existingAmount.Equal(request.Amount) || existingEpoch != request.AuthorityEpoch {
			return false, platformledger.ErrIdempotencyConflict
		}
		return false, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("read ledger hold: %w", err)
	}
	nextReserved, err := platformledger.Reserve(balance, reserved, request.Amount)
	if err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bizdecipher_ledger_holds
        (reservation_id, event_id, account_id, amount, state, authority_epoch, version)
        VALUES ($1, $2, $3, $4::numeric, 'reserved', $5, 1)`,
		request.ReservationID, request.EventID, request.AccountID, request.Amount.String(), request.AuthorityEpoch); err != nil {
		return false, fmt.Errorf("insert ledger hold: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE bizdecipher_ledger_projections
        SET reserved_amount = reserved_amount + $2::numeric, version = version + 1, updated_at = NOW()
		WHERE account_id = $1`, request.AccountID, nextReserved.Sub(reserved).String()); err != nil {
		return false, fmt.Errorf("reserve ledger balance: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit ledger hold: %w", err)
	}
	return true, nil
}

func (r *BizDecipherLedgerRepository) RebuildProjections(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `SELECT bizdecipher_rebuild_ledger_projections()`); err != nil {
		return fmt.Errorf("rebuild ledger projections: %w", err)
	}
	return nil
}

func (r *BizDecipherLedgerRepository) PostCanonicalUsage(
	ctx context.Context,
	event corecontracts.CanonicalUsageFinalized,
	journal platformledger.Journal,
) (PostResult, error) {
	if event.EventID() != journal.EventID {
		return PostResult{}, platformledger.ErrIdempotencyConflict
	}
	payloadSHA, err := platformledger.CanonicalUsagePayloadSHA256(event)
	if err != nil {
		return PostResult{}, err
	}
	if payloadSHA != journal.PayloadSHA256 {
		return PostResult{}, platformledger.ErrIdempotencyConflict
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return PostResult{}, fmt.Errorf("begin canonical usage ledger post: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, event.EventID()); err != nil {
		return PostResult{}, fmt.Errorf("lock canonical usage event: %w", err)
	}
	result, found, err := r.canonicalUsageReceipt(ctx, tx, event.EventID(), payloadSHA)
	if err != nil {
		return PostResult{}, err
	}
	if found {
		if err := tx.Commit(); err != nil {
			return PostResult{}, fmt.Errorf("commit canonical usage replay: %w", err)
		}
		return result, nil
	}
	result, found, err = r.canonicalUsageJournal(ctx, tx, event.EventID(), payloadSHA)
	if err != nil {
		return PostResult{}, err
	}
	if !found {
		validated, err := platformledger.NewJournal(journal)
		if err != nil {
			return PostResult{}, err
		}
		result, err = r.postJournalTx(ctx, tx, validated)
		if err != nil {
			return PostResult{}, err
		}
	}
	if err := r.insertCanonicalUsageReceipt(ctx, tx, event.EventID(), payloadSHA, result.JournalID); err != nil {
		return PostResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return PostResult{}, fmt.Errorf("commit canonical usage ledger post: %w", err)
	}
	return result, nil
}

func (r *BizDecipherLedgerRepository) canonicalUsageReceipt(
	ctx context.Context,
	tx *sql.Tx,
	eventID string,
	payloadSHA string,
) (PostResult, bool, error) {
	var existingPayloadSHA, journalID string
	err := tx.QueryRowContext(ctx, `SELECT receipt.payload_sha256, receipt.journal_id
        FROM bizdecipher_ledger_outbox_receipts receipt
        JOIN bizdecipher_ledger_journals journal ON journal.id = receipt.journal_id
        WHERE receipt.event_id = $1
          AND journal.event_id = receipt.event_id
          AND journal.payload_sha256 = receipt.payload_sha256`, eventID).
		Scan(&existingPayloadSHA, &journalID)
	if errors.Is(err, sql.ErrNoRows) {
		return PostResult{}, false, nil
	}
	if err != nil {
		return PostResult{}, false, fmt.Errorf("read canonical usage outbox receipt: %w", err)
	}
	if existingPayloadSHA != payloadSHA {
		return PostResult{}, false, platformledger.ErrIdempotencyConflict
	}
	return PostResult{JournalID: journalID, Replayed: true}, true, nil
}

func (r *BizDecipherLedgerRepository) canonicalUsageJournal(
	ctx context.Context,
	tx *sql.Tx,
	eventID string,
	payloadSHA string,
) (PostResult, bool, error) {
	var journalID, existingPayloadSHA string
	err := tx.QueryRowContext(ctx, `SELECT id, payload_sha256
        FROM bizdecipher_ledger_journals WHERE event_id = $1`, eventID).
		Scan(&journalID, &existingPayloadSHA)
	if errors.Is(err, sql.ErrNoRows) {
		return PostResult{}, false, nil
	}
	if err != nil {
		return PostResult{}, false, fmt.Errorf("read canonical usage journal: %w", err)
	}
	if existingPayloadSHA != payloadSHA {
		return PostResult{}, false, platformledger.ErrIdempotencyConflict
	}
	return PostResult{JournalID: journalID, Replayed: true}, true, nil
}

func (r *BizDecipherLedgerRepository) insertCanonicalUsageReceipt(
	ctx context.Context,
	tx *sql.Tx,
	eventID string,
	payloadSHA string,
	journalID string,
) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO bizdecipher_ledger_outbox_receipts
        (event_id, payload_sha256, journal_id, processed_at)
        VALUES ($1, $2, $3, NOW())`, eventID, payloadSHA, journalID); err != nil {
		return fmt.Errorf("insert canonical usage outbox receipt: %w", err)
	}
	return nil
}

func (r *BizDecipherLedgerRepository) OwnerEarnings(ctx context.Context, accountID string) (platformledger.Projection, error) {
	var projection platformledger.Projection
	projection.AccountID = accountID
	if err := r.db.QueryRowContext(ctx, `SELECT p.balance, p.reserved_amount
        FROM bizdecipher_ledger_projections p
        JOIN bizdecipher_ledger_accounts a ON a.id = p.account_id
        WHERE p.account_id = $1 AND a.purpose = 'owner_earnings'`, accountID).
		Scan(&projection.Balance, &projection.Reserved); err != nil {
		return platformledger.Projection{}, fmt.Errorf("read owner earnings projection: %w", err)
	}
	return projection, nil
}
