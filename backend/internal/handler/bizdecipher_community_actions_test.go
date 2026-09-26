package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpdateCommunityPostActionRequiresAuth(t *testing.T) {
	h, cleanup := newBizDecipherHandlerWithMockRepo(t)
	defer cleanup()

	w, c := newCommunityActionContext(http.MethodPatch, "/biz/community/posts/101/action", `{"action":"mark_resolved"}`, "101", 0)
	h.UpdateCommunityPostAction(c)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.JSONEq(t, `{"code":401,"message":"User not authenticated"}`, w.Body.String())
}

func TestUpdateCommunityPostActionRejectsUserModerationStatus(t *testing.T) {
	h, cleanup := newBizDecipherHandlerWithMockRepo(t)
	defer cleanup()

	w, c := newCommunityActionContext(http.MethodPatch, "/biz/community/posts/101/action", `{"action":"set_status","status":"hidden"}`, "101", 303)
	h.UpdateCommunityPostAction(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "invalid owner post status")
}

func TestUpdateCommunityPostActionOwnerCanAcceptComment(t *testing.T) {
	h, cleanup := newBizDecipherHandlerWithMockRepo(t,
		func(mock sqlmock.Sqlmock) {
			postID := int64(101)
			commentID := int64(202)
			ownerID := int64(303)

			mock.ExpectBegin()
			mock.ExpectQuery("SELECT user_id\\s+FROM community_posts").
				WithArgs(postID).
				WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(ownerID))
			mock.ExpectQuery("SELECT EXISTS\\(").
				WithArgs(commentID, postID).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			mock.ExpectExec("UPDATE community_comments\\s+SET status = CASE").
				WithArgs(postID, commentID).
				WillReturnResult(sqlmock.NewResult(0, 2))
			mock.ExpectQuery("UPDATE community_posts\\s+SET status = CASE").
				WithArgs(postID).
				WillReturnRows(newHandlerCommunityPostRows().AddRow(
					postID, ownerID, "", "support", "Need help", "Body", []byte(`["qa"]`), "tavern", "chat-hall", false,
					"answered", false, 0, 2, 9, "", "", "incident_support", "", "", "", "ask_help",
					[]byte(`[]`), []byte(`{}`), time.Now(), time.Now(),
				))
			mock.ExpectCommit()
			mock.ExpectQuery("SELECT c\\.id, c\\.post_id, c\\.user_id,").
				WithArgs(postID, 5).
				WillReturnRows(newHandlerCommunityCommentRows().
					AddRow(commentID, postID, int64(404), "helper", "Accepted answer", "operator", false, "accepted", time.Now(), time.Now()))
		},
	)
	defer cleanup()

	w, c := newCommunityActionContext(http.MethodPatch, "/biz/community/posts/101/action", `{"action":"accept_comment","comment_id":202}`, "101", 303)
	h.UpdateCommunityPostAction(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Code int `json:"code"`
		Data struct {
			ID       int64  `json:"id"`
			UserID   int64  `json:"user_id"`
			Status   string `json:"status"`
			Comments []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"comments"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, int64(101), body.Data.ID)
	require.Equal(t, int64(303), body.Data.UserID)
	require.Equal(t, "answered", body.Data.Status)
	require.Len(t, body.Data.Comments, 1)
	require.Equal(t, "accepted", body.Data.Comments[0].Status)
}

func TestUpdateCommunityPostActionNonOwnerAcceptReturnsNotFound(t *testing.T) {
	h, cleanup := newBizDecipherHandlerWithMockRepo(t,
		func(mock sqlmock.Sqlmock) {
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT user_id\\s+FROM community_posts").
				WithArgs(int64(101)).
				WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(999)))
			mock.ExpectRollback()
		},
	)
	defer cleanup()

	w, c := newCommunityActionContext(http.MethodPatch, "/biz/community/posts/101/action", `{"action":"accept_comment","comment_id":202}`, "101", 303)
	h.UpdateCommunityPostAction(c)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "Community post or comment not found")
}

func TestAdminUpdateCommunityPostStatusCanPinAndConfirm(t *testing.T) {
	h, cleanup := newBizDecipherHandlerWithMockRepo(t,
		func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery("UPDATE community_posts\\s+SET status = \\$2, pinned = \\$3").
				WithArgs(int64(101), "confirmed", true).
				WillReturnRows(newHandlerCommunityPostRows().AddRow(
					int64(101), int64(303), "", "support", "Need help", "Body", []byte(`["qa"]`), "tavern", "chat-hall", false,
					"confirmed", true, 0, 2, 9, "", "", "incident_support", "", "", "", "ask_help",
					[]byte(`[]`), []byte(`{}`), time.Now(), time.Now(),
				))
		},
	)
	defer cleanup()

	w, c := newCommunityActionContext(http.MethodPatch, "/admin/biz/community/posts/101/status", `{"status":"confirmed","pinned":true}`, "101", 1)
	h.AdminUpdateCommunityPostStatus(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Code int `json:"code"`
		Data struct {
			Status string `json:"status"`
			Pinned bool   `json:"pinned"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "confirmed", body.Data.Status)
	require.True(t, body.Data.Pinned)
}

func TestAdminUpdateCommunityCommentStatusCanConfirmOfficial(t *testing.T) {
	h, cleanup := newBizDecipherHandlerWithMockRepo(t,
		func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery("UPDATE community_comments\\s+SET status = \\$2, official = \\$3").
				WithArgs(int64(202), "confirmed", true).
				WillReturnRows(newHandlerCommunityCommentRows().AddRow(
					int64(202), int64(101), int64(404), "", "Confirmed answer", "official", true, "confirmed", time.Now(), time.Now(),
				))
		},
	)
	defer cleanup()

	w, c := newCommunityActionContext(http.MethodPatch, "/admin/biz/community/comments/202/status", `{"status":"confirmed","official":true}`, "202", 1)
	h.AdminUpdateCommunityCommentStatus(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Code int `json:"code"`
		Data struct {
			Status   string `json:"status"`
			Official bool   `json:"official"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "confirmed", body.Data.Status)
	require.True(t, body.Data.Official)
}

func newBizDecipherHandlerWithMockRepo(t *testing.T, configure ...func(sqlmock.Sqlmock)) (*BizDecipherHandler, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	for _, fn := range configure {
		fn(mock)
	}
	repo := repository.NewBizDecipherRepository(db)
	svc := service.NewBizDecipherService(repo, nil, nil)
	h := NewBizDecipherHandler(svc)
	return h, func() {
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	}
}

func newCommunityActionContext(method, target, body, id string, userID int64) (*httptest.ResponseRecorder, *gin.Context) {
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

func newHandlerCommunityPostRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "user_id", "author", "kind", "title", "body", "tags", "district", "channel", "private", "status", "pinned", "catches", "replies", "views",
		"source_type", "source_id", "scenario", "subject_type", "subject_id", "subject_title", "action_type", "evidence", "trust_signals", "created_at", "updated_at",
	})
}

func newHandlerCommunityCommentRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "post_id", "user_id", "author", "body", "helper_role", "official", "status", "created_at", "updated_at",
	})
}

var _ service.BizDecipherRepository = repository.NewBizDecipherRepository(nil)
