package handler

import (
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func handleAssetExecutionPackageError(c *gin.Context, err error) bool {
	if errors.Is(err, service.ErrAssetCommerceInvalid) || errors.Is(err, service.ErrAssetCommerceConflict) ||
		errors.Is(err, service.ErrAssetCommerceForbidden) {
		response.ErrorFrom(c, err)
		return true
	}
	if errors.Is(err, service.ErrCapabilityAssetPackageInvalid) || errors.Is(err, service.ErrCapabilityAssetPackageConflict) ||
		errors.Is(err, service.ErrCapabilityAssetPackageNotFound) || errors.Is(err, service.ErrCapabilityAssetPackageForbidden) {
		handleCapabilityAssetPackageError(c, err)
		return true
	}
	return false
}
func (h *BizDecipherHandler) GetAssetExecutionPlan(c *gin.Context) {
	actor, ok := creatorActor(c)
	if !ok {
		return
	}
	id, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	version, ok := capabilityAssetVersionParam(c)
	if !ok {
		return
	}
	if c.Query("history") == "1" {
		history, err := h.bizService.ListAssetWorkflows(c.Request.Context(), actor, id, version)
		if err != nil {
			if !handleAssetExecutionPackageError(c, err) {
				writeWorkbenchError(c, err)
			}
			return
		}
		c.Header("Cache-Control", "private, no-store")
		response.Success(c, history)
		return
	}
	if requestID := c.Query("request_id"); requestID != "" {
		state, err := h.bizService.GetAssetWorkflow(c.Request.Context(), actor, id, version, requestID)
		if err != nil {
			if !handleAssetExecutionPackageError(c, err) {
				writeWorkbenchError(c, err)
			}
			return
		}
		c.Header("Cache-Control", "private, no-store")
		response.Success(c, state)
		return
	}
	plan, err := h.bizService.GetAssetExecutionPlan(c.Request.Context(), id, actor, version)
	if err != nil {
		if !handleAssetExecutionPackageError(c, err) {
			writeWorkbenchError(c, err)
		}
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, plan)
}
func (h *BizDecipherHandler) LaunchAssetExecution(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	id, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	version, ok := capabilityAssetVersionParam(c)
	if !ok {
		return
	}
	var in struct {
		workbenchRunRequest
		PlanDigest  string   `json:"plan_digest"`
		Permissions []string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		response.BadRequest(c, "Invalid asset execution request")
		return
	}
	result, err := h.bizService.AdvanceAssetExecution(c.Request.Context(), h.workbench, id, version, in.PlanDigest, in.Permissions,
		workbench.LaunchCommand{Identity: identity, CapabilityID: in.CapabilityID, CapabilityVersion: in.CapabilityVersion,
			CapabilityDigest: in.CapabilityDigest, CanonicalModelID: in.CanonicalModelID, CanonicalModelVersion: in.CanonicalModelVersion,
			AcceptedQuoteID: in.AcceptedQuoteID, AcceptedQuoteSHA: in.AcceptedQuoteSHA, Intent: in.Intent,
			IdempotencyKey: workbenchIdempotencyKey(c, in.RequestID)})
	if err != nil {
		if handleAssetExecutionPackageError(c, err) {
			return
		}
		writeWorkbenchError(c, err)
		return
	}
	response.Success(c, result)
}
