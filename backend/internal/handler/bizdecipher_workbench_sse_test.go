package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWorkbenchSSEReconnectFromCursorEmitsNextSequenceOnce(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	event := workbench.Event{ID: "wbe_2", RunID: "wbr_stream", Seq: 2, Kind: workbench.EventRunStarted, State: workbench.StateRunning}
	stub := &workbenchHTTPStub{events: []workbench.EventStreamResult{{Events: []workbench.Event{event}, Cursor: workbench.StreamCursor{RunID: "wbr_stream", Seq: 2}, Terminal: true}}}
	rec, c := newWorkbenchHTTPContext(t, http.MethodGet, "/api/v1/biz/workbench/runs/wbr_stream/events?cursor=1", 41)
	c.Params = gin.Params{{Key: "id", Value: "wbr_stream"}}
	c.Request.Header.Set("Last-Event-ID", "1")

	// When
	NewBizDecipherHandler(nil, stub).StreamWorkbenchRunEvents(c)

	// Then
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Equal(t, 1, strings.Count(rec.Body.String(), "id: 2"))
	require.Equal(t, 1, stub.eventN)
}

func TestWorkbenchSSESecondReadClosesSubscribeRace(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	event := workbench.Event{ID: "wbe_2", RunID: "wbr_race", Seq: 2, Kind: workbench.EventRunSucceeded, State: workbench.StateSucceeded}
	stub := &workbenchHTTPStub{events: []workbench.EventStreamResult{
		{Events: nil, Cursor: workbench.StreamCursor{RunID: "wbr_race", Seq: 1}, Terminal: false},
		{Events: []workbench.Event{event}, Cursor: workbench.StreamCursor{RunID: "wbr_race", Seq: 2}, Terminal: true},
	}}
	rec, c := newWorkbenchHTTPContext(t, http.MethodGet, "/api/v1/biz/workbench/runs/wbr_race/events", 41)
	c.Params = gin.Params{{Key: "id", Value: "wbr_race"}}

	// When
	NewBizDecipherHandler(nil, stub).StreamWorkbenchRunEvents(c)

	// Then
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 2, stub.eventN)
	require.Equal(t, 1, strings.Count(rec.Body.String(), "id: 2"))
}

func TestWorkbenchSSEStaleCursorReturnsTypedReset(t *testing.T) {
	// Given
	gin.SetMode(gin.TestMode)
	reset := workbench.StreamReset{Run: workbench.Run{ID: "wbr_stale", OwnerID: 41}, Cursor: workbench.StreamCursor{RunID: "wbr_stale", Seq: 9}, Reason: "retention_window_advanced"}
	stub := &workbenchHTTPStub{eventErrs: []error{&workbench.StaleCursorError{Cursor: workbench.StreamCursor{RunID: "wbr_stale", Seq: 1}, Reset: reset}}}
	rec, c := newWorkbenchHTTPContext(t, http.MethodGet, "/api/v1/biz/workbench/runs/wbr_stale/events", 41)
	c.Params = gin.Params{{Key: "id", Value: "wbr_stale"}}
	c.Request.Header.Set("Last-Event-ID", "1")

	// When
	NewBizDecipherHandler(nil, stub).StreamWorkbenchRunEvents(c)

	// Then
	require.Equal(t, http.StatusGone, rec.Code)
	var body response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "WORKBENCH_STREAM_CURSOR_STALE", body.Reason)
	payload, err := json.Marshal(body.Data)
	require.NoError(t, err)
	require.Contains(t, string(payload), "retention_window_advanced")
	require.Contains(t, string(payload), "wbr_stale")
}
