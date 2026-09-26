package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrSharedPoolCompactNotSupported means the selected shared-pool account has
// no current, durable proof that the Codex compact endpoint works. The request
// must fail before any reservation or upstream forwarding starts.
var ErrSharedPoolCompactNotSupported = errors.New("shared pool compact endpoint is not verified")

type sharedPoolCompactCapabilityRepository interface {
	HasSharedPoolOAuthCompactCapability(ctx context.Context, poolID, accountID int64, publishedModel, upstreamModel string) (bool, error)
}

func (s *BizDecipherService) HasSharedPoolOAuthCompactCapability(
	ctx context.Context,
	poolID int64,
	accountID int64,
	publishedModel string,
	upstreamModel string,
) (bool, error) {
	publishedModel = strings.TrimSpace(publishedModel)
	upstreamModel = strings.TrimSpace(upstreamModel)
	if s == nil || s.repo == nil || poolID <= 0 || accountID <= 0 || publishedModel == "" || upstreamModel == "" {
		return false, nil
	}
	repo, ok := s.repo.(sharedPoolCompactCapabilityRepository)
	if !ok {
		return false, errors.New("shared pool compact capability repository is unavailable")
	}
	return repo.HasSharedPoolOAuthCompactCapability(ctx, poolID, accountID, publishedModel, upstreamModel)
}

// ValidateSharedPoolCompactAccess deliberately rejects API-key accounts. Their
// shared-pool Responses path may fall back to Chat Completions, which cannot
// faithfully carry /responses/compact. OAuth accounts are allowed only when a
// successful compact item exists on a succeeded probe job for the pool's
// current configuration version.
func (s *OpenAIGatewayService) ValidateSharedPoolCompactAccess(
	ctx context.Context,
	accessKey *SharedPoolAccessKey,
) error {
	if s == nil || s.bizDecipherService == nil || accessKey == nil {
		return ErrSharedPoolCompactNotSupported
	}
	if !strings.EqualFold(strings.TrimSpace(accessKey.AuthType), AccountTypeOAuth) || accessKey.AccountID <= 0 {
		return ErrSharedPoolCompactNotSupported
	}
	publishedModel := strings.TrimSpace(accessKey.PublishedModelName)
	upstreamModel := strings.TrimSpace(accessKey.UpstreamModelName)
	if upstreamModel == "" {
		upstreamModel = publishedModel
	}
	if publishedModel == "" {
		return ErrSharedPoolCompactNotSupported
	}
	ready, err := s.bizDecipherService.HasSharedPoolOAuthCompactCapability(
		ctx,
		accessKey.PoolID,
		accessKey.AccountID,
		publishedModel,
		upstreamModel,
	)
	if err != nil {
		return fmt.Errorf("check shared pool compact capability: %w", err)
	}
	if !ready {
		return ErrSharedPoolCompactNotSupported
	}
	return nil
}
