package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGovernanceAdminHandlersRejectOrdinaryUsers(t *testing.T) {
	h := &BizDecipherHandler{}
	for _, name := range []string{"grant", "revoke_badge", "revoke_rule"} {
		t.Run(name, func(t *testing.T) {
			w, c := governanceTestContext(12)
			switch name {
			case "grant": h.AdminGrantCommunityBadge(c)
			case "revoke_badge": h.AdminRevokeCommunityBadge(c)
			case "revoke_rule": h.AdminRevokeGovernanceRule(c)
			}
			require.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}

func TestGovernanceAdminHandlerRejectsAnonymous(t *testing.T) {
	w, c := governanceTestContext(0)
	(&BizDecipherHandler{}).AdminGrantCommunityBadge(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func governanceTestContext(userID int64) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	if userID > 0 {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
	}
	return w, c
}
