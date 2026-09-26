package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type sharedPoolOwnerPauseRepository interface {
	SetSharedPoolOwnerPauseTx(context.Context, int64, int64, int64, bool) (*SharedPool, error)
}

func (s *BizDecipherService) SetSharedPoolOwnerPause(ctx context.Context, poolID, ownerID, version int64, paused bool) (*SharedPool, error) {
	if poolID <= 0 || ownerID <= 0 || version <= 0 {
		return nil, infraerrors.BadRequest("INVALID_POOL_VERSION", "请刷新共享池后重试")
	}
	repo, ok := s.repo.(sharedPoolOwnerPauseRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("POOL_PAUSE_UNAVAILABLE", "共享池停用服务暂不可用")
	}
	pool, err := repo.SetSharedPoolOwnerPauseTx(ctx, poolID, ownerID, version, paused)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_owner_pause")
	return pool, nil
}
