package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type sharedPoolIdentityResolver struct {
	db *sql.DB
}

func (r sharedPoolIdentityResolver) ResolveSharedPoolCanonicalIdentity(
	ctx context.Context,
	query service.SharedPoolIdentityQuery,
) (*service.SharedPoolCanonicalIdentity, error) {
	if r.db == nil || query.PoolID <= 0 || query.OwnerID <= 0 {
		return nil, service.ErrSharedPoolIdentityQueryInvalid
	}

	sourceKind := ""
	sourceID := int64(0)
	if query.Supply != nil {
		switch query.Supply.Kind {
		case service.SharedPoolSupplyPoolDefault, service.SharedPoolSupplyPoolAccount:
			sourceKind = string(query.Supply.Kind)
		default:
			return nil, service.ErrSharedPoolIdentityQueryInvalid
		}
		if query.Supply.ID <= 0 {
			return nil, service.ErrSharedPoolIdentityQueryInvalid
		}
		sourceID = query.Supply.ID
	}

	var (
		identity  service.SharedPoolCanonicalIdentity
		accountID sql.NullInt64
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT
			binding.pool_id,
			binding.owner_id,
			binding.canonical_group_id,
			disposition.canonical_account_id,
			binding.lifecycle
		FROM shared_pool_sub2_bindings binding
		JOIN groups canonical_group
		  ON canonical_group.id = binding.canonical_group_id
		 AND canonical_group.deleted_at IS NULL
		LEFT JOIN shared_pool_supply_dispositions disposition
		  ON disposition.pool_id = binding.pool_id
		 AND disposition.source_kind = $3
		 AND disposition.source_id = $4
		 AND disposition.disposition = 'mapped'
		LEFT JOIN accounts canonical_account
		  ON canonical_account.id = disposition.canonical_account_id
		 AND canonical_account.deleted_at IS NULL
		LEFT JOIN account_groups membership
		  ON membership.account_id = canonical_account.id
		 AND membership.group_id = binding.canonical_group_id
		WHERE binding.pool_id = $1
		  AND binding.owner_id = $2
		  AND ($3 = '' OR membership.account_id IS NOT NULL)
	`, query.PoolID, query.OwnerID, sourceKind, sourceID).Scan(
		&identity.PoolID,
		&identity.OwnerID,
		&identity.GroupID,
		&accountID,
		&identity.Lifecycle,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrSharedPoolIdentityUnmapped
	}
	if err != nil {
		return nil, fmt.Errorf("resolve shared pool canonical identity: %w", err)
	}
	if identity.Lifecycle != "active" {
		return nil, service.ErrSharedPoolIdentityQuarantined
	}
	if accountID.Valid {
		canonicalAccountID := service.CanonicalAccountID(accountID.Int64)
		identity.AccountID = &canonicalAccountID
	}
	return &identity, nil
}
