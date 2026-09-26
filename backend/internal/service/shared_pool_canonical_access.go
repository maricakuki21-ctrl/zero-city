package service

import "context"

type canonicalSharedPoolAccessRepository interface {
	GetCanonicalSharedPoolAccessKeyByAPIKeyID(context.Context, int64, string) (*SharedPoolAccessKey, error)
}

func (s *BizDecipherService) getCanonicalSharedPoolAccessKey(ctx context.Context, apiKeyID int64, model string) (*SharedPoolAccessKey, error) {
	repo, ok := s.repo.(canonicalSharedPoolAccessRepository)
	if !ok {
		return nil, ErrSharedPoolIdentityUnmapped
	}
	return repo.GetCanonicalSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, model)
}
