package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// Migration 237 fences every legacy shared-pool runtime table behind a
// transaction-scoped authority epoch: a guarded write is rejected unless
// `bizdecipher.shared_pool_authority_epoch` matches
// `shared_pool_cutover_control.authority_epoch` and the control phase still
// admits the legacy runtime. The Go runtime never declared that epoch, so every
// shared-pool settlement write failed at the trigger. Declare the current epoch
// at the start of each transaction that writes a guarded table.
//
// The epoch is read inside the same transaction so a concurrent cutover can
// never leave a transaction writing under a stale epoch: the guard still
// re-validates the value it is given.
func declareSharedPoolLegacyRuntimeEpochTx(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return nil
	}
	var epoch int64
	err := tx.QueryRowContext(ctx,
		`SELECT authority_epoch FROM shared_pool_cutover_control WHERE singleton = TRUE`,
	).Scan(&epoch)
	if errors.Is(err, sql.ErrNoRows) {
		// Cutover control is optional in minimal/test schemas; without a control
		// row the guard function is not installed either.
		return nil
	}
	if err != nil {
		return err
	}
	// set_config(..., true) is transaction-local and cleared on commit/rollback.
	if _, err := tx.ExecContext(ctx,
		`SELECT set_config('bizdecipher.shared_pool_authority_epoch', $1, TRUE)`,
		strconv.FormatInt(epoch, 10),
	); err != nil {
		logger.LegacyPrintf("repository.shared_pool", "[SharedPool] failed to declare legacy runtime epoch %d: %v", epoch, err)
		return err
	}
	return nil
}
