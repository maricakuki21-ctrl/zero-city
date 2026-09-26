package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminPoolInspectionRejectsUnauthorizedAndInvalidRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, role, id string
		user           int64
		status         int
	}{
		{"anonymous", "", "3", 0, http.StatusUnauthorized},
		{"ordinary user", "user", "3", 7, http.StatusForbidden},
		{"missing role", "", "3", 7, http.StatusForbidden},
		{"bad id", "admin", "bad", 7, http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/biz/shared-pools/"+test.id+"/pricing", nil)
			c.Params = gin.Params{{Key: "id", Value: test.id}}
			if test.user > 0 {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: test.user})
			}
			c.Set(string(middleware.ContextKeyUserRole), test.role)
			(&BizDecipherHandler{}).AdminInspectSharedPoolPricing(c)
			require.Equal(t, test.status, rec.Code)
		})
	}
}
