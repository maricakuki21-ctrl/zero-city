package service

import (
	"context"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrCommunityParticipationRequired    = infraerrors.Forbidden("COMMUNITY_PARTICIPATION_REQUIRED", "此板块需要居民资格；你可以先在闲聊广场发帖，或提交贡献与作品供管理员确认")
	ErrCommunityLocationInvalid          = infraerrors.BadRequest("COMMUNITY_LOCATION_INVALID", "请选择有效的城区与频道")
	ErrCommunityParticipationUnavailable = infraerrors.ServiceUnavailable("COMMUNITY_PARTICIPATION_UNAVAILABLE", "参与资格暂时无法读取，请稍后重试")
	ErrCommunityStaffOnly                = infraerrors.Forbidden("COMMUNITY_STAFF_ONLY", "公告与规则公示仅限管理员发布")
)

type CommunityParticipation struct {
	UserID  int64  `json:"user_id"`
	Level   int    `json:"level"`
	IsAdmin bool   `json:"is_admin"`
	Reason  string `json:"reason"`
}

type communityParticipationRepository interface {
	GetCommunityParticipation(context.Context, int64) (*CommunityParticipation, error)
	SetCommunityParticipation(context.Context, int64, int64, int, string) (*CommunityParticipation, error)
	GetCommunityPostLocation(context.Context, int64) (string, string, error)
}

func (s *BizDecipherService) participationRepository() (communityParticipationRepository, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCommunityParticipationUnavailable
	}
	r, ok := s.repo.(communityParticipationRepository)
	if !ok {
		return nil, ErrCommunityParticipationUnavailable
	}
	return r, nil
}

func (s *BizDecipherService) GetCommunityParticipation(ctx context.Context, userID int64) (*CommunityParticipation, error) {
	r, err := s.participationRepository()
	if err != nil {
		return nil, err
	}
	return r.GetCommunityParticipation(ctx, userID)
}

func (s *BizDecipherService) SetCommunityParticipation(ctx context.Context, actorID, userID int64, level int, reason string) (*CommunityParticipation, error) {
	reason = strings.TrimSpace(reason)
	if userID <= 0 || level < 0 || level > 1 || reason == "" || len([]rune(reason)) > 300 {
		return nil, infraerrors.BadRequest("COMMUNITY_PARTICIPATION_INVALID", "请填写用户编号、L0 或 L1 资格及确认依据")
	}
	actor, err := s.GetCommunityParticipation(ctx, actorID)
	if err != nil {
		return nil, err
	}
	if !actor.IsAdmin {
		return nil, ErrCommunityStaffOnly
	}
	r, err := s.participationRepository()
	if err != nil {
		return nil, err
	}
	return r.SetCommunityParticipation(ctx, actorID, userID, level, reason)
}

// Empty locations from older clients are canonicalized to the open plaza.
func normalizeCommunityLocation(district, channel string) (string, string, error) {
	district, channel = strings.TrimSpace(district), strings.TrimSpace(channel)
	if district == "" && channel == "" {
		return "tavern", "chat-hall", nil
	}
	channels := map[string][]string{
		"tavern":     {"chat-hall", "life-break", "deals"},
		"workshop":   {"tech-share", "news-radar", "help-desk"},
		"market":     {"capability-showcase", "demand-posting", "delivery-certification"},
		"governance": {"votes", "badges", "rules"},
	}
	for _, allowed := range channels[district] {
		if channel == allowed {
			return district, channel, nil
		}
	}
	return "", "", ErrCommunityLocationInvalid
}

func (s *BizDecipherService) requireCommunityParticipation(ctx context.Context, userID int64, district, channel string, announcement bool) error {
	access, err := s.GetCommunityParticipation(ctx, userID)
	if err != nil {
		return err
	}
	if access.IsAdmin {
		return nil
	}
	if announcement || (district == "governance" && channel == "rules") {
		return ErrCommunityStaffOnly
	}
	if district != "tavern" && access.Level < 1 {
		return ErrCommunityParticipationRequired
	}
	return nil
}
