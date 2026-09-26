package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type workbenchHTTPStub struct {
	workbench.UnavailableAdapter
	events    []workbench.EventStreamResult
	eventErrs []error
	eventN    int
	fork      workbench.ForkResult
	artifact  workbench.Artifact
	run       workbench.RunResult
}

func (s *workbenchHTTPStub) Events(context.Context, workbench.EventsCommand) (workbench.EventStreamResult, error) {
	idx := s.eventN
	s.eventN++
	if idx < len(s.eventErrs) && s.eventErrs[idx] != nil {
		return workbench.EventStreamResult{}, s.eventErrs[idx]
	}
	if idx < len(s.events) {
		return s.events[idx], nil
	}
	return workbench.EventStreamResult{Terminal: true}, nil
}

func (s *workbenchHTTPStub) Fork(context.Context, workbench.ForkCommand) (workbench.ForkResult, error) {
	return s.fork, nil
}

func (s *workbenchHTTPStub) Artifact(context.Context, workbench.ArtifactCommand) (workbench.Artifact, error) {
	if s.artifact.ArtifactID == "" {
		return workbench.Artifact{}, workbench.ErrArtifactNotFound
	}
	return s.artifact, nil
}

func (s *workbenchHTTPStub) Run(context.Context, workbench.RunCommand) (workbench.RunResult, error) {
	return s.run, nil
}

func TestWorkbenchForkHTTPCreatesDerivedRun(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	stub := &workbenchHTTPStub{fork: workbench.ForkResult{Run: workbench.Run{ID: "wbr_forked", OwnerID: 41, State: workbench.StateQueued}}}
	rec, c := newWorkbenchHTTPContext(t, http.MethodPost, "/api/v1/biz/workbench/runs/wbr_source/fork", 41)
	c.Params = gin.Params{{Key: "id", Value: "wbr_source"}}
	c.Request.Header.Set("Idempotency-Key", "fork-once")

	// When
	NewBizDecipherHandler(nil, stub).ForkWorkbenchRun(c)

	// Then
	require.Equal(t, http.StatusOK, rec.Code)
	var body response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	payload, err := json.Marshal(body.Data)
	require.NoError(t, err)
	require.Contains(t, string(payload), "wbr_forked")
}

func TestWorkbenchArtifactHTTPHidesCrossUserOwnership(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	stub := &workbenchHTTPStub{}
	rec, c := newWorkbenchHTTPContext(t, http.MethodGet, "/api/v1/biz/workbench/runs/wbr_private/artifacts/wba_private", 202)
	c.Params = gin.Params{{Key: "id", Value: "wbr_private"}, {Key: "artifactId", Value: "wba_private"}}

	// When
	NewBizDecipherHandler(nil, stub).GetWorkbenchArtifactMetadata(c)

	// Then
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.NotContains(t, rec.Body.String(), "wba_private")
}

func TestWorkbenchHTTPRequiresAuthenticatedOwner(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/biz/workbench/runs/wbr_one", nil)

	// When
	NewBizDecipherHandler(nil, &workbenchHTTPStub{}).GetWorkbenchRun(c)

	// Then
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func newWorkbenchHTTPContext(t *testing.T, method, target string, userID int64) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, target, strings.NewReader("{}"))
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	return rec, c
}
