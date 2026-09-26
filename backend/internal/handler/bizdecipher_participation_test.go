package handler

import (
	"net/http"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCommunityParticipationPostReturnsForbiddenBeforeInsert(t *testing.T) {
	h, cleanup := newBizDecipherHandlerWithMockRepo(t, func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery(`SELECT COALESCE\(p.level, 0\)`).
			WithArgs(int64(303)).
			WillReturnRows(sqlmock.NewRows([]string{"level", "admin", "reason"}).AddRow(0, false, ""))
	})
	defer cleanup()
	w, c := newCommunityActionContext(http.MethodPost, "/biz/community/posts", `{"kind":"card","title":"title","body":"body","district":"workshop","channel":"help-desk"}`, "", 303)
	h.CreateCommunityPost(c)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "COMMUNITY_PARTICIPATION_REQUIRED")
}

func TestCommunityParticipationRequiresAuth(t *testing.T) {
	h, cleanup := newBizDecipherHandlerWithMockRepo(t)
	defer cleanup()
	w, c := newCommunityActionContext(http.MethodGet, "/biz/community/participation", "", "", 0)
	h.GetCommunityParticipation(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
