package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) ResolveSharedPoolCanonicalIdentity(
	ctx context.Context,
	query service.SharedPoolIdentityQuery,
) (*service.SharedPoolCanonicalIdentity, error) {
	if r == nil {
		return nil, service.ErrSharedPoolIdentityQueryInvalid
	}
	return (sharedPoolIdentityResolver{db: r.db}).ResolveSharedPoolCanonicalIdentity(ctx, query)
}
