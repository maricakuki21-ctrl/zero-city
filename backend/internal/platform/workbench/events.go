package workbench

import (
	"fmt"
	"strconv"
	"time"
)

type EventKind string

const (
	EventRunQueued       EventKind = "run_queued"
	EventRunStarted      EventKind = "run_started"
	EventCancelRequested EventKind = "cancel_requested"
	EventRunSucceeded    EventKind = "run_succeeded"
	EventRunFailed       EventKind = "run_failed"
	EventRunCancelled    EventKind = "run_cancelled"
	EventArtifactAdded   EventKind = "artifact_added"
	EventSnapshotSaved   EventKind = "snapshot_saved"
	EventRunReplayed     EventKind = "run_replayed"
	EventRunForked       EventKind = "run_forked"
)

type Event struct {
	ID          EventID     `json:"id"`
	RunID       RunID       `json:"run_id"`
	Seq         uint64      `json:"seq"`
	Kind        EventKind   `json:"kind"`
	State       RunState    `json:"state,omitempty"`
	ArtifactID  ArtifactID  `json:"artifact_id,omitempty"`
	SnapshotID  SnapshotID  `json:"snapshot_id,omitempty"`
	SourceRunID RunID       `json:"source_run_id,omitempty"`
	Failure     *RunFailure `json:"failure,omitempty"`
	Message     string      `json:"message,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

type StreamCursor struct {
	RunID RunID  `json:"run_id"`
	Seq   uint64 `json:"seq"`
}

func (c StreamCursor) String() string { return strconv.FormatUint(c.Seq, 10) }

type EventRead struct {
	Run                Run
	Events             []Event
	Cursor             StreamCursor
	OldestAvailableSeq uint64
}

type StreamReset struct {
	Run    Run          `json:"run"`
	Cursor StreamCursor `json:"cursor"`
	Reason string       `json:"reason"`
}

type EventStreamResult struct {
	Events   []Event      `json:"events"`
	Cursor   StreamCursor `json:"cursor"`
	Terminal bool         `json:"terminal"`
}

func ReadEventStream(read EventRead) (EventStreamResult, error) {
	if read.Run.ID == "" {
		return EventStreamResult{}, fmt.Errorf("%w: event stream requires run id", ErrInvalidCommand)
	}
	cursor := read.Cursor
	if cursor.RunID == "" {
		cursor.RunID = read.Run.ID
	}
	if cursor.RunID != read.Run.ID {
		return EventStreamResult{}, ErrCursorRunMismatch
	}
	oldest := read.OldestAvailableSeq
	if oldest == 0 {
		oldest = 1
	}
	if oldest > 1 && cursor.Seq < oldest-1 {
		latest := uint64(0)
		if read.Run.NextEventSeq > 0 {
			latest = read.Run.NextEventSeq - 1
		}
		return EventStreamResult{}, &StaleCursorError{
			Cursor: cursor,
			Reset: StreamReset{
				Run: read.Run.Clone(), Cursor: StreamCursor{RunID: read.Run.ID, Seq: latest}, Reason: "retention_window_advanced",
			},
		}
	}

	selected := make([]Event, 0, len(read.Events))
	previous := uint64(0)
	for _, event := range read.Events {
		if event.RunID != read.Run.ID || event.Seq == 0 || event.Seq <= previous {
			return EventStreamResult{}, fmt.Errorf("%w: invalid persisted event sequence", ErrInvalidCommand)
		}
		previous = event.Seq
		if event.Seq > cursor.Seq {
			selected = append(selected, event)
			cursor.Seq = event.Seq
		}
	}
	return EventStreamResult{Events: selected, Cursor: cursor, Terminal: read.Run.State.Terminal()}, nil
}
