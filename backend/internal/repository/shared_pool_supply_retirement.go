package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Retire the mapped runtime identity, retaining IDs and historical billing joins.
// A zero sourceID retires the whole pool; otherwise only its specified supply.
func retireSharedPoolSupplyTx(ctx context.Context, tx *sql.Tx, poolID, sourceID int64) error {
	_, err := tx.ExecContext(ctx, `WITH retired AS (
		UPDATE accounts a
		SET deleted_at=NOW(), status='disabled', schedulable=FALSE, updated_at=NOW()
		FROM shared_pool_supply_dispositions d
		WHERE d.canonical_account_id=a.id AND d.pool_id=$1
		  AND ($2::bigint=0 OR (d.source_kind='pool_account' AND d.source_id=$2))
		  AND a.deleted_at IS NULL
		RETURNING a.id
	)
	INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload)
	SELECT $3,id,NULL,NULL FROM retired`, poolID, sourceID, service.SchedulerOutboxEventAccountChanged)
	return err
}

// Deleting the product must retire its routing identity in the same transaction.
// Keep stable group IDs for bills; never expose the group as selectable supply.
func retireSharedPoolGroupTx(ctx context.Context, tx *sql.Tx, poolID int64) error {
	_, err := tx.ExecContext(ctx, `WITH binding AS (
		UPDATE shared_pool_sub2_bindings
		SET lifecycle='archived',updated_at=NOW()
		WHERE pool_id=$1 RETURNING canonical_group_id
	), retired_groups AS (
		UPDATE groups g SET status='disabled',updated_at=NOW()
		FROM binding b WHERE g.id=b.canonical_group_id
		RETURNING g.id
	)
	INSERT INTO scheduler_outbox(event_type,account_id,group_id,payload)
	SELECT $2,NULL,id,NULL FROM retired_groups`, poolID, service.SchedulerOutboxEventGroupChanged)
	return err
}
