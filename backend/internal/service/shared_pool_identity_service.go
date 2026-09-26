package service

import "context"

func (s *BizDecipherService) ResolveSharedPoolCanonicalIdentity(
	ctx context.Context,
	query SharedPoolIdentityQuery,
) (*SharedPoolCanonicalIdentity, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSharedPoolIdentityQueryInvalid
	}
	return s.repo.ResolveSharedPoolCanonicalIdentity(ctx, query)
}
