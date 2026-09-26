package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type pollHandlerRepoStub struct {
	service.BizDecipherRepository
	voterID  int64
	optionID int64
	closeErr error
}

func (r *pollHandlerRepoStub) GetCommunityParticipation(_ context.Context, userID int64) (*service.CommunityParticipation, error) {
	return &service.CommunityParticipation{UserID: userID, Level: 1}, nil
}
func (r *pollHandlerRepoStub) SetCommunityParticipation(context.Context, int64, int64, int, string) (*service.CommunityParticipation, error) {
	return nil, nil
}
func (r *pollHandlerRepoStub) GetCommunityPostLocation(context.Context, int64) (string, string, error) {
	return "governance", "votes", nil
}

func (r *pollHandlerRepoStub) EnsureProfile(_ context.Context, userID int64) (*service.BizProfile, error) {
	return &service.BizProfile{UserID: userID}, nil
}
func (r *pollHandlerRepoStub) ListCommunityPolls(context.Context, int64, bool, int) ([]service.CommunityPoll, error) {
	return []service.CommunityPoll{}, nil
}
func (r *pollHandlerRepoStub) CreateCommunityPollTx(context.Context, int64, service.CommunityPollInput) (*service.CommunityPoll, error) {
	return &service.CommunityPoll{}, nil
}
func (r *pollHandlerRepoStub) VoteCommunityPollTx(_ context.Context, pollID, userID, optionID int64) (*service.CommunityPoll, error) {
	r.voterID, r.optionID = userID, optionID
	return &service.CommunityPoll{ID: pollID, ViewerOptionID: optionID}, nil
}
func (r *pollHandlerRepoStub) CloseCommunityPollTx(context.Context, int64, int64, bool) (*service.CommunityPoll, error) {
	return nil, r.closeErr
}

func TestVoteCommunityPoll_derivesVoterFromJWTContext(t *testing.T) {
	repo := &pollHandlerRepoStub{}
	h := NewBizDecipherHandler(service.NewBizDecipherService(repo, nil, nil))
	w, c := newPollHandlerContext(http.MethodPost, "/biz/community/polls/9/vote", `{"option_id":77,"user_id":999}`, "9", 31)

	h.VoteCommunityPoll(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, int64(31), repo.voterID)
	require.Equal(t, int64(77), repo.optionID)
}

func TestCloseCommunityPoll_returnsForbiddenForWrongOwner(t *testing.T) {
	repo := &pollHandlerRepoStub{closeErr: service.ErrCommunityPollForbidden}
	h := NewBizDecipherHandler(service.NewBizDecipherService(repo, nil, nil))
	w, c := newPollHandlerContext(http.MethodPost, "/biz/community/polls/9/close", `{}`, "9", 42)

	h.CloseCommunityPoll(c)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func newPollHandlerContext(method, target, body, id string, userID int64) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: id}}
	if userID > 0 {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	}
	return w, c
}
