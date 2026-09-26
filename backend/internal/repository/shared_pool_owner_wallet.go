package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type sharedPoolOwnerEarningCredit struct {
	OwnerID        int64
	PoolID         int64
	AccountID      int64
	PriceVersionID int64
	OperationID    string
	RequestID      string
	Model          string
	PricingSource  string
	GrossAmount    float64
	PlatformFee    float64
	NetAmount      float64
	SourceType     string
	Note           string
}

type sharedPoolOwnerEarningCreditResult struct {
	LedgerID        int64
	BalanceLedgerID int64
	WalletAfter     float64
	AlreadyPosted   bool
}

func creditSharedPoolOwnerWalletTx(
	ctx context.Context,
	tx *sql.Tx,
	input sharedPoolOwnerEarningCredit,
) (*sharedPoolOwnerEarningCreditResult, error) {
	if tx == nil {
		return nil, errors.New("owner wallet transaction is unavailable")
	}
	input.OperationID = strings.TrimSpace(input.OperationID)
	input.SourceType = strings.TrimSpace(input.SourceType)
	if input.OwnerID <= 0 || input.PoolID <= 0 || input.OperationID == "" || input.NetAmount <= 0 {
		return nil, errors.New("invalid shared pool owner earning")
	}
	if len(input.OperationID) > 160 {
		return nil, errors.New("owner earning operation id is too long")
	}

	var poolName, ownerLabel string
	if err := tx.QueryRowContext(ctx,
		`SELECT name, owner_label FROM shared_pools WHERE id = $1`,
		input.PoolID,
	).Scan(&poolName, &ownerLabel); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO shared_pool_owner_wallets (owner_id)
		 VALUES ($1)
		 ON CONFLICT (owner_id) DO NOTHING`,
		input.OwnerID,
	); err != nil {
		return nil, err
	}

	var ledgerID int64
	var currentAvailable float64
	claimErr := tx.QueryRowContext(ctx, `
		INSERT INTO shared_pool_owner_earnings_ledger (
			owner_id, pool_id, account_id, price_version_id,
			event_type, operation_id, request_id,
			pool_name_snapshot, owner_label_snapshot, model_snapshot, pricing_source_snapshot,
			gross_amount, platform_fee_amount, net_amount,
			wallet_delta, available_after, status, metadata
		)
		SELECT
			$1, $2, NULLIF($3, 0), NULLIF($4, 0),
			'earning', $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13,
			0, wallet.available_amount, 'pending', jsonb_build_object('source_type', $14::text)
		FROM shared_pool_owner_wallets wallet
		WHERE wallet.owner_id = $1
		ON CONFLICT (owner_id, event_type, operation_id) WHERE operation_id <> '' DO NOTHING
		RETURNING id, available_after`,
		input.OwnerID, input.PoolID, input.AccountID, input.PriceVersionID,
		input.OperationID, strings.TrimSpace(input.RequestID),
		poolName, ownerLabel, strings.TrimSpace(input.Model), strings.TrimSpace(input.PricingSource),
		input.GrossAmount, input.PlatformFee, input.NetAmount, input.SourceType,
	).Scan(&ledgerID, &currentAvailable)
	if errors.Is(claimErr, sql.ErrNoRows) {
		var existingPoolID, existingAccountID, existingPriceVersionID int64
		var existingRequestID, existingModel, existingPricingSource, existingSourceType string
		var existingGrossAmount, existingPlatformFee, existingNetAmount float64
		if err := tx.QueryRowContext(ctx, `
			SELECT
				id,
				available_after,
				COALESCE(pool_id, 0),
				COALESCE(account_id, 0),
				COALESCE(price_version_id, 0),
				request_id,
				model_snapshot,
				pricing_source_snapshot,
				gross_amount,
				platform_fee_amount,
				net_amount,
				COALESCE(
					NULLIF(metadata->>'source_type', ''),
					(
						SELECT balance.source_type
						FROM shared_pool_balance_ledger balance
						WHERE balance.owner_earnings_ledger_id = earning.id
							AND balance.user_id = earning.owner_id
							AND balance.source_id = earning.operation_id
						ORDER BY balance.id
						LIMIT 1
					),
					''
				)
			FROM shared_pool_owner_earnings_ledger earning
			WHERE owner_id = $1
				AND event_type = 'earning'
				AND operation_id = $2`,
			input.OwnerID, input.OperationID,
		).Scan(
			&ledgerID,
			&currentAvailable,
			&existingPoolID,
			&existingAccountID,
			&existingPriceVersionID,
			&existingRequestID,
			&existingModel,
			&existingPricingSource,
			&existingGrossAmount,
			&existingPlatformFee,
			&existingNetAmount,
			&existingSourceType,
		); err != nil {
			return nil, err
		}
		// An operation id identifies one immutable earning event. Returning the
		// first result for a different pool, route, pricing snapshot, request, or
		// amount would silently hide a billing corruption from both the owner and
		// the platform ledger.
		if existingPoolID != input.PoolID ||
			existingAccountID != input.AccountID ||
			existingPriceVersionID != input.PriceVersionID ||
			existingRequestID != strings.TrimSpace(input.RequestID) ||
			existingModel != strings.TrimSpace(input.Model) ||
			existingPricingSource != strings.TrimSpace(input.PricingSource) ||
			existingSourceType != input.SourceType ||
			math.Abs(existingGrossAmount-input.GrossAmount) > 1e-12 ||
			math.Abs(existingPlatformFee-input.PlatformFee) > 1e-12 ||
			math.Abs(existingNetAmount-input.NetAmount) > 1e-12 {
			return nil, service.ErrIdempotencyKeyConflict
		}
		return &sharedPoolOwnerEarningCreditResult{
			LedgerID:      ledgerID,
			WalletAfter:   currentAvailable,
			AlreadyPosted: true,
		}, nil
	}
	if claimErr != nil {
		return nil, claimErr
	}

	var walletAfter float64
	if err := tx.QueryRowContext(ctx, `
		UPDATE shared_pool_owner_wallets
		SET available_amount = available_amount + $2,
			version = version + 1,
			updated_at = NOW()
		WHERE owner_id = $1
		RETURNING available_amount`,
		input.OwnerID, input.NetAmount,
	).Scan(&walletAfter); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE shared_pool_owner_earnings_ledger
		SET wallet_delta = $2,
			available_after = $3,
			status = 'available',
			available_at = NOW(),
			posted_at = NOW()
		WHERE id = $1`,
		ledgerID, input.NetAmount, walletAfter,
	); err != nil {
		return nil, err
	}

	if input.SourceType != "" {
		note := strings.TrimSpace(input.Note)
		if note == "" {
			note = fmt.Sprintf("共享池收益 · %s", poolName)
		}
		var balanceLedgerID int64
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO shared_pool_balance_ledger (
				user_id, pool_id, account_id, source_type, source_id,
				amount, balance_after, status, note, posted_at,
				settlement_destination, pool_name_snapshot, owner_label_snapshot,
				model_snapshot, price_version_id, owner_earnings_ledger_id
			)
			VALUES (
				$1, $2, NULLIF($3, 0), $4, $5,
				$6, $7, 'posted', $8, NOW(),
				'owner_wallet', $9, $10, $11, NULLIF($12, 0), $13
			)
			ON CONFLICT (user_id, source_type, source_id) WHERE source_id <> '' DO NOTHING
			RETURNING id`,
			input.OwnerID, input.PoolID, input.AccountID, input.SourceType, input.OperationID,
			input.NetAmount, walletAfter, note,
			poolName, ownerLabel, strings.TrimSpace(input.Model), input.PriceVersionID, ledgerID,
		).Scan(&balanceLedgerID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// An existing balance-ledger row cannot be silently attached to a new
				// wallet earning. It may be a legacy payout that already reached
				// users.balance, so reusing it could pay the owner twice.
				return nil, service.ErrIdempotencyKeyConflict
			}
			return nil, err
		}
		return &sharedPoolOwnerEarningCreditResult{
			LedgerID:        ledgerID,
			BalanceLedgerID: balanceLedgerID,
			WalletAfter:     walletAfter,
		}, nil
	}

	return &sharedPoolOwnerEarningCreditResult{
		LedgerID:    ledgerID,
		WalletAfter: walletAfter,
	}, nil
}

func scanSharedPoolOwnerEarningsEntry(s scanner) (*service.SharedPoolOwnerEarningsEntry, error) {
	return scanSharedPoolOwnerEarningsEntryWithExtras(s)
}

func scanSharedPoolOwnerEarningsEntryWithExtras(s scanner, extras ...any) (*service.SharedPoolOwnerEarningsEntry, error) {
	var entry service.SharedPoolOwnerEarningsEntry
	var poolID, accountID, priceVersionID sql.NullInt64
	var availableAt, postedAt sql.NullTime
	var metadata []byte
	destinations := []any{
		&entry.ID, &entry.OwnerID, &poolID, &accountID, &priceVersionID,
		&entry.EventType, &entry.OperationID, &entry.RequestID,
		&entry.PoolNameSnapshot, &entry.OwnerLabelSnapshot, &entry.ModelSnapshot, &entry.PricingSourceSnapshot,
		&entry.GrossAmount, &entry.PlatformFeeAmount, &entry.NetAmount,
		&entry.WalletDelta, &entry.AvailableAfter, &entry.Status,
		&availableAt, &metadata, &entry.CreatedAt, &postedAt,
	}
	destinations = append(destinations, extras...)
	if err := s.Scan(destinations...); err != nil {
		return nil, err
	}
	if poolID.Valid {
		entry.PoolID = &poolID.Int64
	}
	if accountID.Valid {
		entry.AccountID = &accountID.Int64
	}
	if priceVersionID.Valid {
		entry.PriceVersionID = &priceVersionID.Int64
	}
	if availableAt.Valid {
		entry.AvailableAt = &availableAt.Time
	}
	if postedAt.Valid {
		entry.PostedAt = &postedAt.Time
	}
	if len(metadata) == 0 {
		metadata = []byte(`{}`)
	}
	entry.Metadata = json.RawMessage(metadata)
	return &entry, nil
}

func (r *bizDecipherRepository) TransferSharedPoolOwnerEarningsTx(
	ctx context.Context,
	ownerID int64,
	amount float64,
	operationID string,
) (*service.SharedPoolOwnerWalletTransferResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var userID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		ownerID,
	).Scan(&userID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO shared_pool_owner_wallets (owner_id)
		 VALUES ($1)
		 ON CONFLICT (owner_id) DO NOTHING`,
		ownerID,
	); err != nil {
		return nil, err
	}

	var available float64
	if err := tx.QueryRowContext(ctx,
		`SELECT available_amount FROM shared_pool_owner_wallets WHERE owner_id = $1 FOR UPDATE`,
		ownerID,
	).Scan(&available); err != nil {
		return nil, err
	}

	var existingDelta, existingWalletAfter float64
	var existingMetadata []byte
	existingErr := tx.QueryRowContext(ctx, `
		SELECT wallet_delta, available_after, metadata
		FROM shared_pool_owner_earnings_ledger
		WHERE owner_id = $1
			AND event_type = 'transfer_to_balance'
			AND operation_id = $2`,
		ownerID, operationID,
	).Scan(&existingDelta, &existingWalletAfter, &existingMetadata)
	if existingErr == nil {
		// Idempotency replays are valid only for the exact same transfer. A
		// caller reusing an operation id with a different amount must get an
		// explicit conflict instead of a misleading success response carrying
		// the first transfer's values.
		if math.Abs((-existingDelta)-amount) > 1e-12 {
			return nil, service.ErrIdempotencyKeyConflict
		}
		var metadata struct {
			BalanceAfter float64 `json:"balance_after"`
		}
		_ = json.Unmarshal(existingMetadata, &metadata)
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &service.SharedPoolOwnerWalletTransferResult{
			OperationID:  operationID,
			Amount:       -existingDelta,
			WalletAfter:  existingWalletAfter,
			BalanceAfter: metadata.BalanceAfter,
			AlreadyDone:  true,
		}, nil
	}
	if !errors.Is(existingErr, sql.ErrNoRows) {
		return nil, existingErr
	}
	if available+1e-12 < amount {
		return nil, service.ErrInsufficientBalance
	}

	var walletAfter float64
	if err := tx.QueryRowContext(ctx, `
		UPDATE shared_pool_owner_wallets
		SET available_amount = available_amount - $2,
			transferred_amount = transferred_amount + $2,
			version = version + 1,
			updated_at = NOW()
		WHERE owner_id = $1 AND available_amount >= $2
		RETURNING available_amount`,
		ownerID, amount,
	).Scan(&walletAfter); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrInsufficientBalance
		}
		return nil, err
	}
	var balanceAfter float64
	if err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance + $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING balance`,
		ownerID, amount,
	).Scan(&balanceAfter); err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(map[string]any{
		"balance_after": balanceAfter,
		"destination":   "users.balance",
	})
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO shared_pool_owner_earnings_ledger (
			owner_id, event_type, operation_id,
			gross_amount, platform_fee_amount, net_amount,
			wallet_delta, available_after, status, metadata, posted_at
		)
		VALUES ($1, 'transfer_to_balance', $2, 0, 0, 0, $3, $4, 'settled', $5::jsonb, NOW())`,
		ownerID, operationID, -amount, walletAfter, string(metadata),
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_balance_ledger (
			user_id, source_type, source_id, amount, balance_after,
			status, note, posted_at
		)
		VALUES (
			$1, 'shared_pool_owner_wallet_transfer', $2, $3, $4,
			'posted', '共享池池主收益转入站点余额', NOW()
		)`,
		ownerID, operationID, amount, balanceAfter,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.SharedPoolOwnerWalletTransferResult{
		OperationID:  operationID,
		Amount:       amount,
		WalletAfter:  walletAfter,
		BalanceAfter: balanceAfter,
	}, nil
}

func emptySharedPoolOwnerWallet(ownerID int64) service.SharedPoolOwnerWallet {
	return service.SharedPoolOwnerWallet{
		OwnerID:   ownerID,
		UpdatedAt: time.Time{},
	}
}
