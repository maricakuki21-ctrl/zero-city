package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrGovernanceInvalid          = infraerrors.BadRequest("GOVERNANCE_INVALID", "治理内容无效")
	ErrGovernanceRuleNotFound     = infraerrors.NotFound("GOVERNANCE_RULE_NOT_FOUND", "规则不存在")
	ErrGovernanceRuleConflict     = infraerrors.Conflict("GOVERNANCE_RULE_CONFLICT", "该投票已经有生效中的规则")
	ErrGovernancePollNotClosed    = infraerrors.Conflict("GOVERNANCE_POLL_NOT_CLOSED", "投票尚未结束，不能采纳为规则")
	ErrGovernanceForbidden        = infraerrors.Forbidden("GOVERNANCE_FORBIDDEN", "只有管理员可以采纳全站规则")
	ErrGovernanceBadgeNotFound    = infraerrors.NotFound("GOVERNANCE_BADGE_NOT_FOUND", "徽章不存在")
	ErrGovernanceBadgeNotGranted  = infraerrors.NotFound("GOVERNANCE_BADGE_NOT_GRANTED", "该用户当前没有这枚徽章")
	ErrGovernanceBadgeAlreadyHeld = infraerrors.Conflict("GOVERNANCE_BADGE_ALREADY_HELD", "该用户已经持有这枚徽章")
	ErrGovernanceUnavailable      = infraerrors.ServiceUnavailable("GOVERNANCE_UNAVAILABLE", "治理服务暂不可用")
)

// GovernanceRule is an adopted city rule. It records who adopted it and keeps
// the vote tally that existed at adoption time.
type GovernanceRule struct {
	ID               int64          `json:"id"`
	SourcePollID     *int64         `json:"source_poll_id,omitempty"`
	Title            string         `json:"title"`
	Body             string         `json:"body"`
	Status           string         `json:"status"`
	TallySnapshot    map[string]any `json:"tally_snapshot"`
	AdoptedByUserID  *int64         `json:"adopted_by_user_id,omitempty"`
	AdoptedBy        string         `json:"adopted_by"`
	AdoptedAt        time.Time      `json:"adopted_at"`
	RevokedByUserID  *int64         `json:"revoked_by_user_id,omitempty"`
	RevokedAt        *time.Time     `json:"revoked_at,omitempty"`
	RevokedReason    string         `json:"revoked_reason,omitempty"`
}

// CommunityBadge is a badge definition. Grants live in CommunityBadgeGrant.
type CommunityBadge struct {
	ID          int64  `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

// CommunityBadgeGrant is one grant of one badge to one user. A revoked grant
// stays visible so the grant and its later revocation remain auditable.
type CommunityBadgeGrant struct {
	GrantID      int64      `json:"grant_id"`
	BadgeKey     string     `json:"badge_key"`
	BadgeName    string     `json:"badge_name"`
	Description  string     `json:"description"`
	UserID       int64      `json:"user_id"`
	Reason       string     `json:"reason,omitempty"`
	GrantedBy    *int64     `json:"granted_by_user_id,omitempty"`
	GrantedAt    time.Time  `json:"granted_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	RevokedReason string    `json:"revoked_reason,omitempty"`
}

type communityGovernanceRepository interface {
	ListGovernanceRules(ctx context.Context, limit int) ([]GovernanceRule, error)
	AdoptCommunityPollAsRuleTx(ctx context.Context, pollID, actorID int64, allowAdmin bool) (*GovernanceRule, error)
	RevokeGovernanceRuleTx(ctx context.Context, ruleID, actorID int64, reason string) (*GovernanceRule, error)
	ListCommunityBadges(ctx context.Context) ([]CommunityBadge, error)
	ListUserBadges(ctx context.Context, userID int64) ([]CommunityBadgeGrant, error)
	GrantCommunityBadgeTx(ctx context.Context, badgeKey string, userID, actorID int64, reason string) (*CommunityBadgeGrant, error)
	RevokeCommunityBadgeTx(ctx context.Context, badgeKey string, userID, actorID int64, reason string) (*CommunityBadgeGrant, error)
}

func (s *BizDecipherService) governanceRepository() (communityGovernanceRepository, error) {
	if s == nil || s.repo == nil {
		return nil, ErrGovernanceUnavailable
	}
	repo, ok := s.repo.(communityGovernanceRepository)
	if !ok {
		return nil, ErrGovernanceUnavailable
	}
	return repo, nil
}

func (s *BizDecipherService) ListGovernanceRules(ctx context.Context, limit int) ([]GovernanceRule, error) {
	repo, err := s.governanceRepository()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return repo.ListGovernanceRules(ctx, limit)
}

func (s *BizDecipherService) AdoptCommunityPollAsRule(ctx context.Context, pollID, actorID int64, allowAdmin bool) (*GovernanceRule, error) {
	if !allowAdmin {
		return nil, ErrGovernanceForbidden
	}
	if pollID <= 0 || actorID <= 0 {
		return nil, ErrGovernanceInvalid
	}
	repo, err := s.governanceRepository()
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.EnsureProfile(ctx, actorID); err != nil {
		return nil, err
	}
	return repo.AdoptCommunityPollAsRuleTx(ctx, pollID, actorID, allowAdmin)
}

func (s *BizDecipherService) RevokeGovernanceRule(ctx context.Context, ruleID, actorID int64, reason string) (*GovernanceRule, error) {
	if ruleID <= 0 || actorID <= 0 {
		return nil, ErrGovernanceInvalid
	}
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > 300 {
		return nil, ErrGovernanceInvalid
	}
	repo, err := s.governanceRepository()
	if err != nil {
		return nil, err
	}
	return repo.RevokeGovernanceRuleTx(ctx, ruleID, actorID, reason)
}

func (s *BizDecipherService) ListCommunityBadges(ctx context.Context) ([]CommunityBadge, error) {
	repo, err := s.governanceRepository()
	if err != nil {
		return nil, err
	}
	return repo.ListCommunityBadges(ctx)
}

func (s *BizDecipherService) ListUserBadges(ctx context.Context, userID int64) ([]CommunityBadgeGrant, error) {
	if userID <= 0 {
		return nil, ErrGovernanceInvalid
	}
	repo, err := s.governanceRepository()
	if err != nil {
		return nil, err
	}
	return repo.ListUserBadges(ctx, userID)
}

func (s *BizDecipherService) GrantCommunityBadge(ctx context.Context, badgeKey string, userID, actorID int64, reason string) (*CommunityBadgeGrant, error) {
	badgeKey = strings.ToLower(strings.TrimSpace(badgeKey))
	reason = strings.TrimSpace(reason)
	if badgeKey == "" || userID <= 0 || actorID <= 0 || len([]rune(reason)) > 300 {
		return nil, ErrGovernanceInvalid
	}
	repo, err := s.governanceRepository()
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.EnsureProfile(ctx, userID); err != nil {
		return nil, err
	}
	return repo.GrantCommunityBadgeTx(ctx, badgeKey, userID, actorID, reason)
}

func (s *BizDecipherService) RevokeCommunityBadge(ctx context.Context, badgeKey string, userID, actorID int64, reason string) (*CommunityBadgeGrant, error) {
	badgeKey = strings.ToLower(strings.TrimSpace(badgeKey))
	reason = strings.TrimSpace(reason)
	if badgeKey == "" || userID <= 0 || actorID <= 0 || len([]rune(reason)) > 300 {
		return nil, ErrGovernanceInvalid
	}
	repo, err := s.governanceRepository()
	if err != nil {
		return nil, err
	}
	return repo.RevokeCommunityBadgeTx(ctx, badgeKey, userID, actorID, reason)
}
