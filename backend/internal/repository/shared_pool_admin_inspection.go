package repository

import (
	"context"
	"database/sql"
)

// This projection is intentionally separate from the public market query.
// No upstream credentials or public visibility predicates are involved.
func (r *bizDecipherRepository) GetSharedPoolOwnerForAdmin(ctx context.Context, poolID int64) (int64, error) {
	var owner sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT owner_id FROM shared_pools WHERE id = $1`, poolID).Scan(&owner)
	return owner.Int64, err
}
