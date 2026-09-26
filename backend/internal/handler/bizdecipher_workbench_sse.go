package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/gin-gonic/gin"
)

func (h *BizDecipherHandler) StreamWorkbenchRunEvents(c *gin.Context) {
	identity, ok := h.workbenchIdentity(c)
	if !ok {
		return
	}
	runID := workbench.RunID(strings.TrimSpace(c.Param("id")))
	if runID == "" {
		response.BadRequest(c, "run id is required")
		return
	}
	cursor, err := workbenchStreamCursor(c, runID)
	if err != nil {
		response.BadRequest(c, "Last-Event-ID must be an event sequence")
		return
	}
	seen := cursor.Seq
	first, err := h.workbench.Events(c.Request.Context(), workbench.EventsCommand{Identity: identity, RunID: runID, Cursor: cursor, Limit: 100})
	if writeWorkbenchError(c, err) {
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
	if err := writeWorkbenchSSE(c, first, &seen); err != nil {
		return
	}
	if first.Terminal {
		return
	}
	wake, cancel := workbench.SubscribeRun(runID)
	defer cancel()
	second, err := h.workbench.Events(c.Request.Context(), workbench.EventsCommand{Identity: identity, RunID: runID, Cursor: workbench.StreamCursor{RunID: runID, Seq: seen}, Limit: 100})
	if writeWorkbenchError(c, err) {
		return
	}
	if err := writeWorkbenchSSE(c, second, &seen); err != nil {
		return
	}
	if second.Terminal {
		return
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-wake:
		}
		next, nextErr := h.workbench.Events(c.Request.Context(), workbench.EventsCommand{Identity: identity, RunID: runID, Cursor: workbench.StreamCursor{RunID: runID, Seq: seen}, Limit: 100})
		if writeWorkbenchError(c, nextErr) {
			return
		}
		if err := writeWorkbenchSSE(c, next, &seen); err != nil {
			return
		}
		if next.Terminal {
			return
		}
	}
}

func workbenchStreamCursor(c *gin.Context, runID workbench.RunID) (workbench.StreamCursor, error) {
	cursor := workbench.StreamCursor{RunID: runID}
	lastEventID := strings.TrimSpace(c.GetHeader("Last-Event-ID"))
	if lastEventID == "" {
		lastEventID = strings.TrimSpace(c.Query("cursor"))
	}
	if lastEventID == "" {
		return cursor, nil
	}
	seq, err := strconv.ParseUint(lastEventID, 10, 64)
	if err != nil {
		return workbench.StreamCursor{}, err
	}
	cursor.Seq = seq
	return cursor, nil
}

func writeWorkbenchSSE(c *gin.Context, stream workbench.EventStreamResult, seen *uint64) error {
	for _, event := range stream.Events {
		if event.Seq <= *seen {
			continue
		}
		payload, err := json.Marshal(event)
		if err != nil {
			writeWorkbenchError(c, err)
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "id: %d\nevent: %s\ndata: %s\n\n", event.Seq, event.Kind, payload); err != nil {
			return err
		}
		*seen = event.Seq
		if flusher, ok := c.Writer.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	return nil
}
