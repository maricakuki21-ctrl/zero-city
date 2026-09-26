package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) SetSharedPoolOwnerPauseTx(ctx context.Context, poolID, ownerID, expectedVersion int64, paused bool) (*service.SharedPool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var version int64
	var current bool
	err = tx.QueryRowContext(ctx, `SELECT config_version,owner_paused FROM shared_pools WHERE id=$1 AND owner_id=$2 AND lifecycle_state<>'archived' FOR UPDATE`, poolID, ownerID).Scan(&version, &current)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPoolForbidden
	}
	if err != nil {
		return nil, err
	}
	if current != paused {
		if version != expectedVersion {
			return nil, service.ErrSharedPoolConcurrentUpdate
		}
		// Existing native activation revocation preserves credentials while stopping
		// dispatch. Resuming clears intent only: readiness/pricing must be rechecked.
		if _, err = tx.ExecContext(ctx, `UPDATE shared_pools SET owner_paused=$3,listed=FALSE,status='maintenance',lifecycle_state='draft',config_version=config_version+1,updated_at=NOW() WHERE id=$1 AND owner_id=$2`, poolID, ownerID, paused); err != nil {
			return nil, err
		}
	}
	result, err := scanSharedPool(tx.QueryRowContext(ctx, `SELECT `+sharedPoolColumns+` FROM shared_pools WHERE id=$1 AND owner_id=$2`, poolID, ownerID))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
