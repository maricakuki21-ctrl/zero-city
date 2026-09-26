package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type pollServiceRepoStub struct {
	communityParticipationRepository
	BizDecipherRepository
	ensuredUserID int64
	createdInput  CommunityPollInput
	votedUserID   int64
	votedOptionID int64
	closedAdmin   bool
	poll          *CommunityPoll
}

func (r *pollServiceRepoStub) GetCommunityParticipation(_ context.Context, userID int64) (*CommunityParticipation, error) {
	return &CommunityParticipation{UserID: userID, Level: 1}, nil
}

func (r *pollServiceRepoStub) EnsureProfile(_ context.Context, userID int64) (*BizProfile, error) {
	r.ensuredUserID = userID
	return &BizProfile{UserID: userID}, nil
}

func (r *pollServiceRepoStub) ListCommunityPolls(context.Context, int64, bool, int) ([]CommunityPoll, error) {
	return []CommunityPoll{*r.poll}, nil
}

func (r *pollServiceRepoStub) CreateCommunityPollTx(_ context.Context, userID int64, input CommunityPollInput) (*CommunityPoll, error) {
	r.createdInput = input
	r.poll.OwnerUserID = userID
	return r.poll, nil
}

func (r *pollServiceRepoStub) VoteCommunityPollTx(_ context.Context, _ int64, userID, optionID int64) (*CommunityPoll, error) {
	r.votedUserID, r.votedOptionID = userID, optionID
	return r.poll, nil
}

func (r *pollServiceRepoStub) CloseCommunityPollTx(_ context.Context, _ int64, _ int64, allowAdmin bool) (*CommunityPoll, error) {
	r.closedAdmin = allowAdmin
	return r.poll, nil
}

func TestCreateCommunityPoll_usesExistingProfilePermissionAndNormalizesOptions(t *testing.T) {
	closeAt := time.Now().Add(time.Hour)
	repo := &pollServiceRepoStub{poll: &CommunityPoll{ID: 1}}
	svc := NewBizDecipherService(repo, nil, nil)

	poll, err := svc.CreateCommunityPoll(context.Background(), 31, CommunityPollInput{
		Title: "  先做哪个功能？  ", Body: "  请选择一个方向。  ",
		Options: []string{"  搜索  ", "分享", "搜索"}, ClosesAt: &closeAt,
	})

	require.NoError(t, err)
	require.Equal(t, int64(31), repo.ensuredUserID)
	require.Equal(t, []string{"搜索", "分享"}, repo.createdInput.Options)
	require.Equal(t, "先做哪个功能？", repo.createdInput.Title)
	require.Equal(t, int64(31), poll.OwnerUserID)
}

func TestCreateCommunityPoll_rejectsFewerThanTwoDistinctOptions(t *testing.T) {
	svc := NewBizDecipherService(&pollServiceRepoStub{poll: &CommunityPoll{}}, nil, nil)

	_, err := svc.CreateCommunityPoll(context.Background(), 31, CommunityPollInput{
		Title: "方向", Body: "说明", Options: []string{"同一项", "同一项"},
	})

	require.ErrorIs(t, err, ErrCommunityPollOptionsInvalid)
}

func TestVoteCommunityPoll_derivesVoterAndDelegatesOneOption(t *testing.T) {
	repo := &pollServiceRepoStub{poll: &CommunityPoll{ID: 9}}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.VoteCommunityPoll(context.Background(), 9, 31, 77)

	require.NoError(t, err)
	require.Equal(t, int64(31), repo.ensuredUserID)
	require.Equal(t, int64(31), repo.votedUserID)
	require.Equal(t, int64(77), repo.votedOptionID)
}

func TestCloseCommunityPoll_passesAdminAuthorityExplicitly(t *testing.T) {
	repo := &pollServiceRepoStub{poll: &CommunityPoll{ID: 9}}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.CloseCommunityPoll(context.Background(), 9, 1, true)

	require.NoError(t, err)
	require.True(t, repo.closedAdmin)
}
