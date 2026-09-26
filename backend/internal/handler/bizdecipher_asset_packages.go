package handler

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type createCapabilityAssetVersionRequest struct {
	Version     string          `json:"version"`
	RuntimeKind string          `json:"runtime_kind"`
	Manifest    json.RawMessage `json:"manifest"`
}

type putCapabilityAssetFileRequest struct {
	Path          string `json:"path"`
	ContentType   string `json:"content_type"`
	ContentBase64 string `json:"content_base64"`
}

type importCapabilityAssetPackageFileRequest struct {
	Path          string `json:"path"`
	ContentType   string `json:"content_type"`
	ContentBase64 string `json:"content_base64"`
}

type importCapabilityAssetPackageRequest struct {
	RuntimeKind string                                    `json:"runtime_kind"`
	Manifest    json.RawMessage                           `json:"manifest"`
	Files       []importCapabilityAssetPackageFileRequest `json:"files"`
	Finalize    *bool                                     `json:"finalize"`
}

func capabilityAssetIDParam(c *gin.Context) (int64, bool) {
	assetID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assetID <= 0 {
		response.BadRequest(c, "Invalid capability asset id")
		return 0, false
	}
	return assetID, true
}

func capabilityAssetVersionParam(c *gin.Context) (string, bool) {
	version := strings.TrimSpace(c.Param("version"))
	if version == "" {
		response.BadRequest(c, "Invalid capability asset version")
		return "", false
	}
	return version, true
}

func handleCapabilityAssetPackageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrCapabilityAssetPackageNotFound), errors.Is(err, sql.ErrNoRows):
		response.NotFound(c, "Capability asset package not found")
	case errors.Is(err, service.ErrCapabilityAssetPackageForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "Capability asset package operation forbidden"})
	case errors.Is(err, service.ErrCapabilityAssetPackageConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "Capability asset package state conflict"})
	case errors.Is(err, service.ErrCapabilityAssetPackageInvalid):
		response.BadRequest(c, err.Error())
	default:
		response.ErrorFrom(c, err)
	}
}

func (h *BizDecipherHandler) ListCapabilityAssetVersions(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	assetID, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	viewerID := int64(0)
	if subject, exists := middleware2.GetAuthSubjectFromContext(c); exists {
		viewerID = subject.UserID
	}
	versions, err := h.bizService.ListCapabilityAssetVersions(c.Request.Context(), assetID, viewerID)
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	response.Success(c, gin.H{"items": versions})
}

func (h *BizDecipherHandler) CreateCapabilityAssetVersion(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	var req createCapabilityAssetVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	version, err := h.bizService.CreateCapabilityAssetVersion(c.Request.Context(), assetID, subject.UserID, service.CreateCapabilityAssetVersionInput{
		Version:     req.Version,
		RuntimeKind: req.RuntimeKind,
		Manifest:    req.Manifest,
	})
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	response.Success(c, version)
}

func (h *BizDecipherHandler) PutCapabilityAssetFile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	version, ok := capabilityAssetVersionParam(c)
	if !ok {
		return
	}
	var req putCapabilityAssetFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	updated, err := h.bizService.PutCapabilityAssetFile(c.Request.Context(), assetID, subject.UserID, version, service.CapabilityAssetFileInput{
		Path:          req.Path,
		ContentType:   req.ContentType,
		ContentBase64: req.ContentBase64,
	})
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	response.Success(c, updated)
}

// ImportCapabilityAssetPackage lets a local agent or CLI push a whole package
// in one request. The body is capped so an oversized upload is rejected before
// it is buffered, and every file is validated by the service before any write.
func (h *BizDecipherHandler) ImportCapabilityAssetPackage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	version, ok := capabilityAssetVersionParam(c)
	if !ok {
		return
	}
	const maxImportBody = int64(2)*service.CapabilityAssetMaxPackageSize + (1 << 20)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportBody)
	var req importCapabilityAssetPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	files := make([]service.CapabilityAssetFileInput, 0, len(req.Files))
	for _, file := range req.Files {
		files = append(files, service.CapabilityAssetFileInput{
			Path:          file.Path,
			ContentType:   file.ContentType,
			ContentBase64: file.ContentBase64,
		})
	}
	finalize := true
	if req.Finalize != nil {
		finalize = *req.Finalize
	}
	item, err := h.bizService.ImportCapabilityAssetPackage(c.Request.Context(), assetID, subject.UserID, service.ImportCapabilityAssetPackageInput{
		Version:     version,
		RuntimeKind: req.RuntimeKind,
		Manifest:    req.Manifest,
		Files:       files,
		Finalize:    finalize,
	})
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *BizDecipherHandler) FinalizeCapabilityAssetVersion(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	version, ok := capabilityAssetVersionParam(c)
	if !ok {
		return
	}
	updated, err := h.bizService.FinalizeCapabilityAssetVersion(c.Request.Context(), assetID, subject.UserID, version)
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *BizDecipherHandler) PublishCapabilityAssetVersion(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	version, ok := capabilityAssetVersionParam(c)
	if !ok {
		return
	}
	updated, err := h.bizService.PublishCapabilityAssetVersion(c.Request.Context(), assetID, subject.UserID, version)
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *BizDecipherHandler) RevokeCapabilityAssetVersion(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	version, ok := capabilityAssetVersionParam(c)
	if !ok {
		return
	}
	updated, err := h.bizService.RevokeCapabilityAssetVersion(c.Request.Context(), assetID, subject.UserID, version)
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *BizDecipherHandler) DownloadCapabilityAssetPackage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, ok := capabilityAssetIDParam(c)
	if !ok {
		return
	}
	version, ok := capabilityAssetVersionParam(c)
	if !ok {
		return
	}
	pkg, err := h.bizService.DownloadCapabilityAssetPackage(c.Request.Context(), assetID, subject.UserID, version)
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	manifest, err := json.Marshal(pkg.Version)
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	header, err := writer.Create("manifest.json")
	if err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	if _, err := header.Write(manifest); err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	for _, file := range pkg.Files {
		entry, err := writer.Create("package/" + file.Path)
		if err != nil {
			handleCapabilityAssetPackageError(c, err)
			return
		}
		if _, err := entry.Write(file.Content); err != nil {
			handleCapabilityAssetPackageError(c, err)
			return
		}
	}
	if err := writer.Close(); err != nil {
		handleCapabilityAssetPackageError(c, err)
		return
	}
	filename := fmt.Sprintf("asset-%d-%s.zip", assetID, version)
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/zip", archive.Bytes())
}
