package handler

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAssetExecutionRequiresAuthentication(t *testing.T) {
	h := &BizDecipherHandler{}
	for _, action := range []func(*gin.Context){h.GetAssetExecutionPlan, h.LaunchAssetExecution} {
		w, c := newPollHandlerContext(http.MethodPost, "/", `{}`, "41", 0)
		action(c)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
}

func TestAssetExecutionConclusiveRejectionsPreserveReason(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
		reason string
	}{
		{service.ErrAssetCommerceInvalid, http.StatusBadRequest, "ASSET_COMMERCE_INVALID"},
		{service.ErrAssetCommerceForbidden, http.StatusForbidden, "ASSET_COMMERCE_FORBIDDEN"},
		{service.ErrAssetCommerceConflict, http.StatusConflict, "ASSET_COMMERCE_CONFLICT"},
	} {
		w, c := newPollHandlerContext(http.MethodPost, "/", `{}`, "41", 0)
		require.True(t, handleAssetExecutionPackageError(c, test.err))
		require.Equal(t, test.status, w.Code)
		require.Contains(t, w.Body.String(), test.reason)
	}
}
