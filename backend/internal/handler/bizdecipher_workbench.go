package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type workbenchRunRequest struct {
	WorkspaceToken        string `json:"workspace_token"`
	CapabilityID          string `json:"capability_id"`
	CapabilityVersion     string `json:"capability_version"`
	CapabilityDigest      string `json:"capability_digest"`
	CanonicalModelID      string `json:"canonical_model_id"`
	CanonicalModelVersion string `json:"canonical_model_version"`
	AcceptedQuoteID       string `json:"accepted_quote_id"`
	AcceptedQuoteSHA      string `json:"accepted_quote_sha"`
	Intent                string `json:"intent"`
	Locale                string `json:"locale"`
	RetryOfRunID          string `json:"retry_of_run_id"`
	RequestID             string `json:"request_id"`
}

type workbenchSaveRequest struct {
	Label string `json:"label"`
}

type workbenchCancelRequest struct {
	Reason string `json:"reason"`
}

func (h *BizDecipherHandler) SetWorkbenchAdapter(adapter workbench.CanonicalAdapter) {
	if adapter != nil {
		h.workbench = adapter
	}
}

func (h *BizDecipherHandler) workbenchIdentity(c *gin.Context) (workbench.Identity, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return workbench.Identity{}, false
	}
	return workbench.Identity{ActorID: workbench.ActorID(subject.UserID)}, true
}

func workbenchIdempotencyKey(c *gin.Context, bodyValue string) string {
	if value := strings.TrimSpace(c.GetHeader("Idempotency-Key")); value != "" {
		return value
	}
	return strings.TrimSpace(bodyValue)
}

func writeWorkbenchError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	var stale *workbench.StaleCursorError
	if errors.As(err, &stale) {
		c.JSON(http.StatusGone, response.Response{
			Code: http.StatusGone, Message: "workbench stream cursor is stale",
			Reason: "WORKBENCH_STREAM_CURSOR_STALE", Data: stale.Reset,
		})
		return true
	}
	status := http.StatusInternalServerError
	reason := "WORKBENCH_INTERNAL_ERROR"
	message := "workbench request failed"
	switch {
	case errors.Is(err, service.ErrWorkbenchCatalogUnavailable):
		status = http.StatusServiceUnavailable
		reason = "WORKBENCH_CATALOG_UNAVAILABLE"
		message = "模型资源不可用或授权已过期，请检查密钥后刷新选择。"
	case errors.Is(err, service.ErrWorkbenchCatalogMismatch):
		status = http.StatusConflict
		reason = "WORKBENCH_CATALOG_CHANGED"
		message = "模型资源已变化，请刷新后重新确认。"
	case errors.Is(err, service.ErrWorkbenchRuntimeUnavailable):
		status = http.StatusServiceUnavailable
		reason = "WORKBENCH_RUNTIME_UNAVAILABLE"
		message = "模型运行服务暂不可用，请稍后重试。"
	case errors.Is(err, workbench.ErrCanonicalRuntimeUnavailable):
		status = http.StatusServiceUnavailable
		reason = "WORKBENCH_RUNTIME_UNAVAILABLE"
		message = "workbench runtime is unavailable"
	case errors.Is(err, workbench.ErrRunNotFound),
		errors.Is(err, workbench.ErrArtifactNotFound),
		errors.Is(err, workbench.ErrSnapshotNotFound),
		errors.Is(err, workbench.ErrOwnershipMismatch):
		status = http.StatusNotFound
		reason = "WORKBENCH_NOT_FOUND"
		message = "workbench resource not found"
	case errors.Is(err, workbench.ErrIdempotencyConflict):
		status = http.StatusConflict
		reason = "WORKBENCH_IDEMPOTENCY_CONFLICT"
		message = "workbench idempotency key conflicts with an existing operation"
	case errors.Is(err, workbench.ErrInvalidTransition):
		status = http.StatusUnprocessableEntity
		reason = "WORKBENCH_INVALID_TRANSITION"
		message = "workbench run cannot transition to the requested state"
	case errors.Is(err, workbench.ErrInvalidCommand),
		errors.Is(err, workbench.ErrInvalidDecimal),
		errors.Is(err, workbench.ErrCursorRunMismatch),
		errors.Is(err, workbench.ErrInvalidRecovery):
		status = http.StatusBadRequest
		reason = "WORKBENCH_INVALID_REQUEST"
		message = "workbench request is invalid"
	case errors.Is(err, workbench.ErrStaleCursor):
		status = http.StatusGone
		reason = "WORKBENCH_STREAM_CURSOR_STALE"
		message = "workbench stream cursor is stale"
	default:
		return response.ErrorFrom(c, err)
	}
	response.ErrorWithDetails(c, status, message, reason, nil)
	return true
}

func (h *BizDecipherHandler) GetWorkbenchWorkspace(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	value, err := h.workbench.Workspace(c.Request.Context(), identity)
	if writeWorkbenchError(c, err) {
		return
	}
	response.Success(c, value)
}

func (h *BizDecipherHandler) CreateWorkbenchRun(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	var request workbenchRunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	key := workbenchIdempotencyKey(c, request.RequestID)
	if strings.TrimSpace(request.CapabilityID) == "" || strings.TrimSpace(request.CapabilityVersion) == "" ||
		strings.TrimSpace(request.CapabilityDigest) == "" || strings.TrimSpace(request.CanonicalModelID) == "" ||
		strings.TrimSpace(request.CanonicalModelVersion) == "" || strings.TrimSpace(request.AcceptedQuoteID) == "" ||
		strings.TrimSpace(request.AcceptedQuoteSHA) == "" || strings.TrimSpace(request.Intent) == "" || key == "" {
		response.BadRequest(c, "canonical capability, model, accepted quote, intent and idempotency key are required")
		return
	}
	value, err := h.workbench.Launch(c.Request.Context(), workbench.LaunchCommand{
		Identity: identity, CapabilityID: strings.TrimSpace(request.CapabilityID),
		CapabilityVersion: strings.TrimSpace(request.CapabilityVersion), CapabilityDigest: strings.TrimSpace(request.CapabilityDigest),
		CanonicalModelID: strings.TrimSpace(request.CanonicalModelID), CanonicalModelVersion: strings.TrimSpace(request.CanonicalModelVersion),
		AcceptedQuoteID: strings.TrimSpace(request.AcceptedQuoteID), AcceptedQuoteSHA: strings.TrimSpace(request.AcceptedQuoteSHA),
		Intent: strings.TrimSpace(request.Intent), IdempotencyKey: key,
	})
	if writeWorkbenchError(c, err) {
		return
	}
	response.Success(c, value)
}

func (h *BizDecipherHandler) GetWorkbenchRun(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	runID := workbench.RunID(strings.TrimSpace(c.Param("id")))
	if runID == "" {
		response.BadRequest(c, "run id is required")
		return
	}
	value, err := h.workbench.Run(c.Request.Context(), workbench.RunCommand{Identity: identity, RunID: runID})
	if writeWorkbenchError(c, err) {
		return
	}
	response.Success(c, value)
}

func (h *BizDecipherHandler) CancelWorkbenchRun(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	var request workbenchCancelRequest
	if err := c.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	command := workbench.CancelCommand{Identity: identity, RunID: workbench.RunID(strings.TrimSpace(c.Param("id"))), Reason: strings.TrimSpace(request.Reason), IdempotencyKey: workbenchIdempotencyKey(c, c.GetHeader("X-Request-ID"))}
	if command.RunID == "" || command.IdempotencyKey == "" {
		response.BadRequest(c, "run id and idempotency key are required")
		return
	}
	value, err := h.workbench.Cancel(c.Request.Context(), command)
	if writeWorkbenchError(c, err) {
		return
	}
	response.Success(c, value)
}

func (h *BizDecipherHandler) SaveWorkbenchRun(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	var request workbenchSaveRequest
	if err := c.ShouldBindJSON(&request); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	command := workbench.SaveCommand{Identity: identity, RunID: workbench.RunID(strings.TrimSpace(c.Param("id"))), ReplayLabel: strings.TrimSpace(request.Label), IdempotencyKey: workbenchIdempotencyKey(c, "")}
	if command.RunID == "" || command.IdempotencyKey == "" {
		response.BadRequest(c, "run id and idempotency key are required")
		return
	}
	value, err := h.workbench.Save(c.Request.Context(), command)
	if writeWorkbenchError(c, err) {
		return
	}
	response.Success(c, value)
}

func (h *BizDecipherHandler) ReplayWorkbenchRun(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	command := workbench.ReplayCommand{Identity: identity, ReplayID: workbench.SnapshotID(strings.TrimSpace(c.Param("id"))), IdempotencyKey: workbenchIdempotencyKey(c, "")}
	if command.ReplayID == "" || command.IdempotencyKey == "" {
		response.BadRequest(c, "replay id and idempotency key are required")
		return
	}
	value, err := h.workbench.Replay(c.Request.Context(), command)
	if writeWorkbenchError(c, err) {
		return
	}
	response.Success(c, value)
}

func (h *BizDecipherHandler) ForkWorkbenchRun(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	command := workbench.ForkCommand{Identity: identity, RunID: workbench.RunID(strings.TrimSpace(c.Param("id"))), IdempotencyKey: workbenchIdempotencyKey(c, "")}
	if command.RunID == "" || command.IdempotencyKey == "" {
		response.BadRequest(c, "run id and idempotency key are required")
		return
	}
	value, err := h.workbench.Fork(c.Request.Context(), command)
	if writeWorkbenchError(c, err) {
		return
	}
	response.Success(c, value)
}

func (h *BizDecipherHandler) GetWorkbenchArtifactMetadata(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	command := workbench.ArtifactCommand{Identity: identity, RunID: workbench.RunID(strings.TrimSpace(c.Param("id"))), ArtifactID: workbench.ArtifactID(strings.TrimSpace(c.Param("artifactId")))}
	if command.RunID == "" || command.ArtifactID == "" {
		response.BadRequest(c, "run id and artifact id are required")
		return
	}
	value, err := h.workbench.Artifact(c.Request.Context(), command)
	if writeWorkbenchError(c, err) {
		return
	}
	response.Success(c, value)
}

func (h *BizDecipherHandler) GetWorkbenchArtifactContent(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	command := workbench.ArtifactCommand{Identity: identity, RunID: workbench.RunID(strings.TrimSpace(c.Param("id"))), ArtifactID: workbench.ArtifactID(strings.TrimSpace(c.Param("artifactId")))}
	if command.RunID == "" || command.ArtifactID == "" {
		response.BadRequest(c, "run id and artifact id are required")
		return
	}
	artifact, err := h.workbench.Artifact(c.Request.Context(), command)
	if writeWorkbenchError(c, err) {
		return
	}
	contentType := strings.TrimSpace(artifact.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Data(http.StatusOK, contentType, artifact.Body)
}

func (h *BizDecipherHandler) GetWorkbenchArtifact(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	runID := workbench.RunID(strings.TrimSpace(c.Param("id")))
	if runID == "" {
		response.BadRequest(c, "run id is required")
		return
	}
	run, err := h.workbench.Run(c.Request.Context(), workbench.RunCommand{Identity: identity, RunID: runID})
	if writeWorkbenchError(c, err) {
		return
	}
	if len(run.Run.Artifacts) == 0 || run.Run.Artifacts[0].ArtifactID == "" {
		writeWorkbenchError(c, workbench.ErrArtifactNotFound)
		return
	}
	artifact, err := h.workbench.Artifact(c.Request.Context(), workbench.ArtifactCommand{Identity: identity, RunID: runID, ArtifactID: run.Run.Artifacts[0].ArtifactID})
	if writeWorkbenchError(c, err) {
		return
	}
	contentType := strings.TrimSpace(artifact.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Data(http.StatusOK, contentType, artifact.Body)
}
