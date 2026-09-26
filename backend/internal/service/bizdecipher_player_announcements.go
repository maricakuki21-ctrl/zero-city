package service

import (
	"context"
	"os"
	"strconv"
	"time"
)

type CommunityAnnouncementPolicy struct {
	MinimumVotes   int `json:"minimum_votes"`
	SupportPercent int `json:"support_percent"`
	VotingHours    int `json:"voting_hours"`
	DisplayDays    int `json:"display_days"`
}

// Limits are operator-configurable; each new proposal stores its own snapshot.
func PlayerAnnouncementPolicy() CommunityAnnouncementPolicy {
	read := func(key string, fallback, minimum, maximum int) int {
		value, err := strconv.Atoi(os.Getenv(key))
		if err != nil || value < minimum || value > maximum {
			return fallback
		}
		return value
	}
	return CommunityAnnouncementPolicy{
		MinimumVotes:   read("CITY_ANNOUNCEMENT_MIN_VOTES", 3, 1, 100000),
		SupportPercent: read("CITY_ANNOUNCEMENT_SUPPORT_PERCENT", 60, 51, 100),
		VotingHours:    read("CITY_ANNOUNCEMENT_VOTING_HOURS", 24, 1, 720),
		DisplayDays:    read("CITY_ANNOUNCEMENT_DISPLAY_DAYS", 7, 1, 30),
	}
}

// No publication job or duplicate post: immutable ballots + deadline are the
// publication fact. Reads and retries cannot create duplicate announcements.
func ResolvePlayerAnnouncement(poll *CommunityPoll, now time.Time) {
	if poll.ProposalKind != "announcement" {
		return
	}
	poll.Decision = "voting"
	if poll.ClosedAt != nil {
		poll.Decision = "withdrawn"
		return
	}
	if poll.ClosesAt == nil || now.Before(*poll.ClosesAt) {
		return
	}
	poll.Decision = "rejected"
	support := 0
	for _, option := range poll.Options {
		if option.Position == 1 {
			support = option.VoteCount
		}
	}
	if poll.TotalVotes < poll.MinimumVotes || int64(support)*100 < int64(poll.TotalVotes)*int64(poll.SupportPercent) {
		return
	}
	poll.PublishedAt = poll.ClosesAt
	expires := poll.ClosesAt.Add(time.Duration(poll.DisplayDays) * 24 * time.Hour)
	poll.ExpiresAt = &expires
	poll.Decision = "published"
	if !now.Before(expires) {
		poll.Decision = "expired"
	}
}

func (s *BizDecipherService) ListPlayerAnnouncements(ctx context.Context) ([]CommunityPoll, error) {
	repo, ok := s.repo.(interface {
		ListPlayerAnnouncements(context.Context) ([]CommunityPoll, error)
	})
	if !ok {
		return nil, ErrCommunityPollUnavailable
	}
	return repo.ListPlayerAnnouncements(ctx)
}

func (s *BizDecipherService) GetCommunityPoll(ctx context.Context, pollID, viewerID int64, isAdmin bool) (*CommunityPoll, error) {
	repo, ok := s.repo.(interface {
		GetCommunityPoll(context.Context, int64, int64, bool) (*CommunityPoll, error)
	})
	if !ok {
		return nil, ErrCommunityPollUnavailable
	}
	return repo.GetCommunityPoll(ctx, pollID, viewerID, isAdmin)
}
