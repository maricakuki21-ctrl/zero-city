package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type sharedPoolProbeJobHandlerRepoStub struct {
	service.BizDecipherRepository
	pool         service.SharedPool
	enqueueInput service.SharedPoolProbeJobEnqueueInput
	job          *service.SharedPoolProbeJob
	getJobID     string
	getOwnerID   int64
}

func (r *sharedPoolProbeJobHandlerRepoStub) ListMySharedPools(context.Context, int64) ([]service.SharedPool, error) {
	return []service.SharedPool{r.pool}, nil
}

func (r *sharedPoolProbeJobHandlerRepoStub) EnqueueSharedPoolProbeJob(_ context.Context, input service.SharedPoolProbeJobEnqueueInput) (*service.SharedPoolProbeJob, bool, error) {
	r.enqueueInput = input
	return &service.SharedPoolProbeJob{
		ID:            "00000000-0000-4000-8000-000000000001",
		OperationID:   input.OperationID,
		PoolID:        input.PoolID,
		OwnerID:       input.OwnerID,
		ModelName:     input.ModelName,
		ProbeType:     input.ProbeType,
		CheckLevel:    input.CheckLevel,
		ConfigVersion: 1,
		Status:        service.SharedPoolProbeJobQueued,
	}, true, nil
}

func (r *sharedPoolProbeJobHandlerRepoStub) GetSharedPoolProbeJob(_ context.Context, jobID string, ownerID int64) (*service.SharedPoolProbeJob, error) {
	r.getJobID = jobID
	r.getOwnerID = ownerID
	return r.job, nil
}

func (r *sharedPoolProbeJobHandlerRepoStub) ClaimSharedPoolProbeJobs(context.Context, string, int, time.Duration) ([]service.SharedPoolProbeJob, error) {
	return nil, nil
}

func (r *sharedPoolProbeJobHandlerRepoStub) HeartbeatSharedPoolProbeJob(context.Context, string, string, time.Duration) error {
	return nil
}

func (r *sharedPoolProbeJobHandlerRepoStub) CompleteSharedPoolProbeJob(context.Context, service.SharedPoolProbeJobCompletion) (*service.SharedPoolProbeJob, error) {
	return nil, nil
}

func TestProbeSharedPoolUpstreamAcceptsDurableJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ownerID := int64(19)
	repo := &sharedPoolProbeJobHandlerRepoStub{pool: service.SharedPool{
		ID: 7, OwnerID: &ownerID, Models: []string{"custom-model"},
	}}
	handler := NewBizDecipherHandler(service.NewBizDecipherService(repo, nil, nil))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/biz/upstream/probe",
		bytes.NewBufferString(`{"pool_id":7,"probe_type":"manual","probe_model":"custom-model"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Idempotency-Key", "operation-handler-1")
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: ownerID})

	handler.ProbeSharedPoolUpstream(ctx)

	require.Equal(t, http.StatusAccepted, recorder.Code)
	var payload struct {
		Code int `json:"code"`
		Data struct {
			ID          string `json:"id"`
			OperationID string `json:"operation_id"`
			Status      string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "00000000-0000-4000-8000-000000000001", payload.Data.ID)
	require.Equal(t, "operation-handler-1", payload.Data.OperationID)
	require.Equal(t, service.SharedPoolProbeJobQueued, payload.Data.Status)
	require.Equal(t, "operation-handler-1", repo.enqueueInput.OperationID)
	require.Equal(t, "full", repo.enqueueInput.CheckLevel)
}

func TestProbeSharedPoolUpstreamRejectsConflictingIdempotencyKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &sharedPoolProbeJobHandlerRepoStub{}
	handler := NewBizDecipherHandler(service.NewBizDecipherService(repo, nil, nil))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/biz/upstream/probe",
		bytes.NewBufferString(`{"pool_id":7,"operation_id":"body-operation"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Idempotency-Key", "header-operation")
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 19})

	handler.ProbeSharedPoolUpstream(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGetSharedPoolProbeJobUsesAuthenticatedOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const jobID = "00000000-0000-4000-8000-000000000001"
	repo := &sharedPoolProbeJobHandlerRepoStub{
		job: &service.SharedPoolProbeJob{
			ID:          jobID,
			OperationID: "operation-handler-1",
			PoolID:      7,
			OwnerID:     19,
			Status:      service.SharedPoolProbeJobRunning,
		},
	}
	handler := NewBizDecipherHandler(service.NewBizDecipherService(repo, nil, nil))
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/biz/upstream/probe-jobs/"+jobID, nil)
	ctx.Params = gin.Params{{Key: "jobId", Value: jobID}}
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 19})

	handler.GetSharedPoolProbeJob(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, jobID, repo.getJobID)
	require.Equal(t, int64(19), repo.getOwnerID)
	var payload struct {
		Code int `json:"code"`
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, jobID, payload.Data.ID)
	require.Equal(t, service.SharedPoolProbeJobRunning, payload.Data.Status)
}
