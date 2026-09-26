package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type sharedPoolUsageReviewHandlerRepoStub struct {
	service.BizDecipherRepository
	prepareCalls int
}

func (*sharedPoolUsageReviewHandlerRepoStub) ListSharedPoolUsageReviews(context.Context, string, int64, int) (*service.SharedPoolUsageReviewPage, error) {
	return &service.SharedPoolUsageReviewPage{}, nil
}

func (r *sharedPoolUsageReviewHandlerRepoStub) PrepareSharedPoolUsageReviewResolutionTx(context.Context, service.ResolveSharedPoolUsageReviewInput) (*service.PreparedSharedPoolUsageReviewResolution, error) {
	r.prepareCalls++
	return &service.PreparedSharedPoolUsageReviewResolution{}, nil
}

func TestAdminResolveSharedPoolUsageReviewHTTPRejectsRetiredChargeActions(t *testing.T) {
	for _, action := range []string{service.SharedPoolUsageReviewActionCaptureHold, service.SharedPoolUsageReviewActionSettleAmount} {
		t.Run(action, func(t *testing.T) {
			repo := &sharedPoolUsageReviewHandlerRepoStub{}
			handler := NewBizDecipherHandler(service.NewBizDecipherService(repo, nil, nil))
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/biz/shared-pool-usage-reviews/7/resolve", bytes.NewBufferString(`{"action":"`+action+`","amount":0.75,"note":"legacy charge disabled","operation_id":"review-http-disabled"}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Params = gin.Params{{Key: "id", Value: "7"}}
			ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 9})

			handler.AdminResolveSharedPoolUsageReview(ctx)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Equal(t, "SHARED_POOL_USAGE_REVIEW_INVALID", responseReason(t, recorder))
			require.Zero(t, repo.prepareCalls)
		})
	}
}

func TestAdminBatchResolveSharedPoolUsageReviewsHTTPRejectsRetiredChargeActions(t *testing.T) {
	for _, action := range []string{service.SharedPoolUsageReviewActionCaptureHold, service.SharedPoolUsageReviewActionSettleAmount} {
		t.Run(action, func(t *testing.T) {
			repo := &sharedPoolUsageReviewHandlerRepoStub{}
			handler := NewBizDecipherHandler(service.NewBizDecipherService(repo, nil, nil))
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/biz/shared-pool-usage-reviews/batch-resolve", bytes.NewBufferString(`{"reservation_ids":[7,8],"action":"`+action+`","note":"legacy charge disabled","operation_id":"review-batch-http-disabled"}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 9})

			handler.AdminBatchResolveSharedPoolUsageReviews(ctx)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Equal(t, "SHARED_POOL_USAGE_REVIEW_INVALID", responseReason(t, recorder))
			require.Zero(t, repo.prepareCalls)
		})
	}
}

func responseReason(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload.Reason
}
