package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandlerSharedPoolDefaultPlatformFeePercentRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{}}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 1}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	getSettings := func() map[string]any {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
		handler.GetSettings(ctx)
		require.Equal(t, http.StatusOK, recorder.Code)
		var resp response.Response
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
		data, ok := resp.Data.(map[string]any)
		require.True(t, ok)
		return data
	}

	require.Equal(t, service.SharedPoolPlatformFeePercentDefault, getSettings()["shared_pool_default_platform_fee_percent"])

	body, err := json.Marshal(map[string]any{"shared_pool_default_platform_fee_percent": 5})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateSettings(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "5.00000000", repo.values[service.SettingKeySharedPoolDefaultPlatformFeePercent])
	require.Equal(t, 5.0, getSettings()["shared_pool_default_platform_fee_percent"])
}

func TestSettingHandlerRejectsOutOfRangeSharedPoolDefaultPlatformFeePercent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, value := range []float64{-0.1, 100.1} {
		t.Run(http.StatusText(http.StatusBadRequest), func(t *testing.T) {
			repo := &settingHandlerRepoStub{values: map[string]string{
				service.SettingKeySharedPoolDefaultPlatformFeePercent: "10",
			}}
			svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 1}})
			handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)
			body, err := json.Marshal(map[string]any{"shared_pool_default_platform_fee_percent": value})
			require.NoError(t, err)

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			handler.UpdateSettings(ctx)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Equal(t, "10", repo.values[service.SettingKeySharedPoolDefaultPlatformFeePercent])
		})
	}
}
