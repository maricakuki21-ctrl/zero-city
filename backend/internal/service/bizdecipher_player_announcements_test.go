package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPlayerAnnouncementDecision(t *testing.T) {
	now := time.Now()
	deadline := now.Add(-time.Hour)
	for _, tc := range []struct {
		name, want                string
		votes, support            int
		ended, withdrawn, expired bool
	}{
		{"voting", "voting", 3, 3, false, false, false},
		{"quorum", "rejected", 2, 2, true, false, false},
		{"support", "rejected", 3, 1, true, false, false},
		{"passed", "published", 3, 2, true, false, false},
		{"exact threshold", "published", 5, 3, true, false, false},
		{"withdrawn", "withdrawn", 3, 3, true, true, false},
		{"expired", "expired", 3, 3, true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			closeAt := deadline
			if !tc.ended {
				closeAt = now.Add(time.Hour)
			}
			if tc.expired {
				closeAt = now.Add(-7 * 24 * time.Hour)
			}
			poll := CommunityPoll{ProposalKind: "announcement", MinimumVotes: 3, SupportPercent: 60, DisplayDays: 7, ClosesAt: &closeAt, TotalVotes: tc.votes,
				Options: []CommunityPollOption{{Position: 1, VoteCount: tc.support}}}
			if tc.withdrawn {
				poll.ClosedAt = &deadline
			}
			ResolvePlayerAnnouncement(&poll, now)
			require.Equal(t, tc.want, poll.Decision)
		})
	}
}

func TestPlayerAnnouncementLocksPolicyAndOptions(t *testing.T) {
	t.Setenv("CITY_ANNOUNCEMENT_MIN_VOTES", "5")
	t.Setenv("CITY_ANNOUNCEMENT_VOTING_HOURS", "48")
	repo := &pollServiceRepoStub{poll: &CommunityPoll{ID: 1}}
	svc := NewBizDecipherService(repo, nil, nil)
	forged := time.Now().Add(-time.Hour)
	_, err := svc.CreateCommunityPoll(context.Background(), 31, CommunityPollInput{
		Title: "公告", Body: "固定内容", ProposalKind: "announcement", ClosesAt: &forged,
		Options: []string{"伪造通过"}, MinimumVotes: 1, SupportPercent: 1, DisplayDays: 999,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"支持发布", "暂不发布"}, repo.createdInput.Options)
	require.Equal(t, 5, repo.createdInput.MinimumVotes)
	require.Equal(t, 60, repo.createdInput.SupportPercent)
	require.Equal(t, 7, repo.createdInput.DisplayDays)
	require.WithinDuration(t, time.Now().Add(48*time.Hour), *repo.createdInput.ClosesAt, time.Second)
}

func TestPlayerAnnouncementL0DiscussionNotVoting(t *testing.T) {
	repo := &participationTestRepo{district: "governance", channel: "votes"}
	svc := NewBizDecipherService(repo, nil, nil)
	_, err := svc.CreateCommunityComment(context.Background(), 10, 7, CommunityCommentInput{Body: "新人建议"})
	require.NoError(t, err)
	_, err = svc.VoteCommunityPoll(context.Background(), 10, 7, 20)
	require.ErrorIs(t, err, ErrCommunityParticipationRequired)
	_, err = svc.CreateCommunityPoll(context.Background(), 7, CommunityPollInput{Title: "公告", Body: "内容", ProposalKind: "announcement"})
	require.ErrorIs(t, err, ErrCommunityParticipationRequired)
}
