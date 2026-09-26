package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type marketGuideApp struct {
	MarketplaceApplication
	userID int64
}

func (s *marketGuideApp) GetOnboarding(_ context.Context, id int64) (*service.MarketplaceOnboarding, error) {
	s.userID = id
	return &service.MarketplaceOnboarding{Version: 1}, nil
}
func (s *marketGuideApp) CompleteOnboarding(_ context.Context, id int64, intent string) (*service.MarketplaceOnboarding, error) {
	s.userID = id
	return &service.MarketplaceOnboarding{Version: 1, Intent: intent}, nil
}
func TestMarketplaceOnboardingHandlerUsesAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := &marketGuideApp{}
	h := &MarketplaceHandler{app: app}
	for _, signedIn := range []bool{false, true} {
		for _, method := range []gin.HandlerFunc{h.GetOnboarding, h.CompleteOnboarding} {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("PUT", "/", strings.NewReader(`{"user_id":999,"intent":"hire"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			if signedIn {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 31})
			}
			method(c)
			if signedIn {
				require.Equal(t, 200, w.Code)
				require.EqualValues(t, 31, app.userID)
			} else {
				require.Equal(t, 401, w.Code)
			}
		}
	}
}
