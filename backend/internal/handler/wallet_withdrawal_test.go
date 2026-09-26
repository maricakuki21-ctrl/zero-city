package handler

import (
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestWithdrawalAdminIdentityFailsClosed(t *testing.T) {
	for _, role := range []string{"", "user", "admin"} {
		t.Run(role, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			if role != "" {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
				c.Set(string(middleware.ContextKeyUserRole), role)
			}
			id, ok := withdrawalIdentity(c, true)
			if role == "admin" {
				require.True(t, ok)
				require.Equal(t, int64(7), id)
			} else {
				require.False(t, ok)
				require.Contains(t, []int{401, 403}, w.Code)
			}
		})
	}
}
