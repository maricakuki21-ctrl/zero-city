package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrCommunityPollInvalid        = infraerrors.BadRequest("COMMUNITY_POLL_INVALID", "投票内容无效")
	ErrCommunityPollOptionsInvalid = infraerrors.BadRequest("COMMUNITY_POLL_OPTIONS_INVALID", "投票需要 2 至 8 个不同选项")
	ErrCommunityPollNotFound       = infraerrors.NotFound("COMMUNITY_POLL_NOT_FOUND", "投票不存在")
	ErrCommunityPollClosed         = infraerrors.Conflict("COMMUNITY_POLL_CLOSED", "投票已经结束")
	ErrCommunityPollAlreadyVoted   = infraerrors.Conflict("COMMUNITY_POLL_ALREADY_VOTED", "你已经提交过其他选项")
	ErrCommunityPollForbidden      = infraerrors.Forbidden("COMMUNITY_POLL_FORBIDDEN", "只有发起人或管理员可以结束投票")
	ErrCommunityPollUnavailable    = infraerrors.ServiceUnavailable("COMMUNITY_POLL_UNAVAILABLE", "投票服务暂不可用")
)

type CommunityPollInput struct {
	ProposalKind   string     `json:"proposal_kind"`
	MinimumVotes   int        `json:"-"`
	SupportPercent int        `json:"-"`
	DisplayDays    int        `json:"-"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	Options        []string   `json:"options"`
	ClosesAt       *time.Time `json:"closes_at,omitempty"`
}

type CommunityPollOption struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`
	Position  int    `json:"position"`
	VoteCount int    `json:"vote_count"`
}

type CommunityPoll struct {
	ProposalKind   string                `json:"proposal_kind"`
	MinimumVotes   int                   `json:"minimum_votes"`
	SupportPercent int                   `json:"support_percent"`
	DisplayDays    int                   `json:"display_days"`
	Decision       string                `json:"decision,omitempty"`
	PublishedAt    *time.Time            `json:"published_at,omitempty"`
	ExpiresAt      *time.Time            `json:"expires_at,omitempty"`
	ID             int64                 `json:"id"`
	PostID         int64                 `json:"post_id"`
	OwnerUserID    int64                 `json:"owner_user_id"`
	Author         string                `json:"author"`
	Title          string                `json:"title"`
	Body           string                `json:"body"`
	Status         string                `json:"status"`
	Options        []CommunityPollOption `json:"options"`
	TotalVotes     int                   `json:"total_votes"`
	ViewerOptionID int64                 `json:"viewer_option_id"`
	CanClose       bool                  `json:"can_close"`
	ClosesAt       *time.Time            `json:"closes_at,omitempty"`
	ClosedAt       *time.Time            `json:"closed_at,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
}

type communityPollRepository interface {
	ListCommunityPolls(ctx context.Context, viewerID int64, viewerIsAdmin bool, limit int) ([]CommunityPoll, error)
	CreateCommunityPollTx(ctx context.Context, userID int64, input CommunityPollInput) (*CommunityPoll, error)
	VoteCommunityPollTx(ctx context.Context, pollID, userID, optionID int64) (*CommunityPoll, error)
	CloseCommunityPollTx(ctx context.Context, pollID, actorID int64, allowAdmin bool) (*CommunityPoll, error)
}

func (s *BizDecipherService) pollRepository() (communityPollRepository, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCommunityPollUnavailable
	}
	repo, ok := s.repo.(communityPollRepository)
	if !ok {
		return nil, ErrCommunityPollUnavailable
	}
	return repo, nil
}

func (s *BizDecipherService) ListCommunityPolls(ctx context.Context, viewerID int64, viewerIsAdmin bool, limit int) ([]CommunityPoll, error) {
	repo, err := s.pollRepository()
	if err != nil {
		return nil, err
	}
	if viewerID < 0 {
		viewerID = 0
	}
	if limit <= 0 || limit > 50 {
		limit = 30
	}
	return repo.ListCommunityPolls(ctx, viewerID, viewerIsAdmin, limit)
}

func (s *BizDecipherService) CreateCommunityPoll(ctx context.Context, userID int64, input CommunityPollInput) (*CommunityPoll, error) {
	if userID <= 0 {
		return nil, ErrCommunityPollInvalid
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	input.ProposalKind = strings.TrimSpace(input.ProposalKind)
	if input.ProposalKind == "" {
		input.ProposalKind = "general"
	}
	switch input.ProposalKind {
	case "general", "announcement", "activity", "improvement", "rule":
	default:
		return nil, ErrCommunityPollInvalid
	}
	policy := PlayerAnnouncementPolicy()
	input.MinimumVotes, input.SupportPercent, input.DisplayDays = policy.MinimumVotes, policy.SupportPercent, policy.DisplayDays
	if input.ProposalKind == "announcement" {
		input.Options = []string{"支持发布", "暂不发布"}
		deadline := time.Now().Add(time.Duration(policy.VotingHours) * time.Hour)
		input.ClosesAt = &deadline
	} else if input.ClosesAt != nil && !input.ClosesAt.After(time.Now()) {
		return nil, ErrCommunityPollInvalid
	}
	input.Options = normalizeCommunityPollOptions(input.Options)
	if input.Title == "" || input.Body == "" || len([]rune(input.Title)) > 180 || len([]rune(input.Body)) > 4000 {
		return nil, ErrCommunityPollInvalid
	}
	if len(input.Options) < 2 || len(input.Options) > 8 {
		return nil, ErrCommunityPollOptionsInvalid
	}
	if err := s.requireCommunityParticipation(ctx, userID, "governance", "votes", false); err != nil {
		return nil, err
	}
	repo, err := s.pollRepository()
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.EnsureProfile(ctx, userID); err != nil {
		return nil, err
	}
	return repo.CreateCommunityPollTx(ctx, userID, input)
}

func (s *BizDecipherService) VoteCommunityPoll(ctx context.Context, pollID, userID, optionID int64) (*CommunityPoll, error) {
	if pollID <= 0 || userID <= 0 || optionID <= 0 {
		return nil, ErrCommunityPollInvalid
	}
	if err := s.requireCommunityParticipation(ctx, userID, "governance", "votes", false); err != nil {
		return nil, err
	}
	repo, err := s.pollRepository()
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.EnsureProfile(ctx, userID); err != nil {
		return nil, err
	}
	return repo.VoteCommunityPollTx(ctx, pollID, userID, optionID)
}

func (s *BizDecipherService) CloseCommunityPoll(ctx context.Context, pollID, actorID int64, allowAdmin bool) (*CommunityPoll, error) {
	if pollID <= 0 || actorID <= 0 {
		return nil, ErrCommunityPollInvalid
	}
	repo, err := s.pollRepository()
	if err != nil {
		return nil, err
	}
	return repo.CloseCommunityPollTx(ctx, pollID, actorID, allowAdmin)
}

func normalizeCommunityPollOptions(options []string) []string {
	cleaned := make([]string, 0, len(options))
	seen := make(map[string]struct{}, len(options))
	for _, option := range options {
		option = strings.TrimSpace(option)
		key := strings.ToLower(option)
		if option == "" || len([]rune(option)) > 180 {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		cleaned = append(cleaned, option)
	}
	return cleaned
}
