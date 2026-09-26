package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type participationTestRepo struct {
	BizDecipherRepository
	communityParticipationRepository
	level             int
	admin             bool
	district, channel string
	post              CommunityPostInput
	comment           CommunityCommentInput
}

func (r *participationTestRepo) GetCommunityParticipation(_ context.Context, id int64) (*CommunityParticipation, error) {
	return &CommunityParticipation{UserID: id, Level: r.level, IsAdmin: r.admin}, nil
}
func (r *participationTestRepo) EnsureProfile(_ context.Context, id int64) (*BizProfile, error) {
	return &BizProfile{UserID: id}, nil
}
func (r *participationTestRepo) CreateCommunityPost(_ context.Context, id int64, in CommunityPostInput) (*CommunityPost, error) {
	r.post = in
	return &CommunityPost{ID: 10, UserID: id}, nil
}
func (r *participationTestRepo) GetCommunityPostLocation(context.Context, int64) (string, string, error) {
	return r.district, r.channel, nil
}
func (r *participationTestRepo) CreateCommunityComment(_ context.Context, post, user int64, in CommunityCommentInput) (*CommunityComment, error) {
	r.comment = in
	return &CommunityComment{ID: 20}, nil
}

func TestCommunityParticipationPostingMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, district, channel, kind string
		level                         int
		admin                         bool
		want                          error
	}{
		{"legacy plaza", "", "", "card", 0, false, nil},
		{"open plaza", "tavern", "chat-hall", "support", 0, false, nil},
		{"workshop locked", "workshop", "help-desk", "support", 0, false, ErrCommunityParticipationRequired},
		{"workshop resident", "workshop", "help-desk", "support", 1, false, nil},
		{"market locked", "market", "demand-posting", "card", 0, false, ErrCommunityParticipationRequired},
		{"no announcement spoof", "tavern", "chat-hall", "announcement", 1, false, ErrCommunityStaffOnly},
		{"no rules spoof", "governance", "rules", "card", 1, false, ErrCommunityStaffOnly},
		{"admin rules", "governance", "rules", "announcement", 0, true, nil},
		{"invalid pair", "tavern", "help-desk", "card", 0, false, ErrCommunityLocationInvalid},
		{"channel without district", "", "help-desk", "card", 0, false, ErrCommunityLocationInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &participationTestRepo{level: tc.level, admin: tc.admin}
			s := NewBizDecipherService(r, nil, nil)
			_, err := s.CreateCommunityPost(context.Background(), 7, CommunityPostInput{
				Kind: tc.kind, Title: "title", Body: "body", District: tc.district, Channel: tc.channel,
				TrustSignals: json.RawMessage(`{"verified":true}`),
			})
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				require.Empty(t, r.post.Title)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, `{}`, string(r.post.TrustSignals))
			require.NotEmpty(t, r.post.District)
		})
	}
}

func TestCommunityParticipationRepliesAndPolls(t *testing.T) {
	r := &participationTestRepo{district: "workshop", channel: "help-desk"}
	s := NewBizDecipherService(r, nil, nil)
	_, err := s.CreateCommunityComment(context.Background(), 10, 7, CommunityCommentInput{Body: "reply", HelperRole: "admin"})
	require.ErrorIs(t, err, ErrCommunityParticipationRequired)
	require.Empty(t, r.comment.Body)
	_, err = s.CreateCommunityPoll(context.Background(), 7, CommunityPollInput{Title: "vote", Body: "body", Options: []string{"a", "b"}})
	require.ErrorIs(t, err, ErrCommunityParticipationRequired)
	r.level = 1
	_, err = s.CreateCommunityComment(context.Background(), 10, 7, CommunityCommentInput{Body: "reply", HelperRole: "admin"})
	require.NoError(t, err)
	require.Equal(t, "resident", r.comment.HelperRole)
	_, err = s.SetCommunityParticipation(context.Background(), 7, 8, 1, "external work")
	require.ErrorIs(t, err, ErrCommunityStaffOnly)
}
