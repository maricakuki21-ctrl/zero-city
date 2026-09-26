package workbench

import (
	"errors"
	"testing"
	"time"
)

func TestEvent_ReadAfterCursorReturnsPersistedSequenceOnce(t *testing.T) {
	t.Parallel()

	run := Run{ID: "wbr_events", State: StateSucceeded, NextEventSeq: 4}
	events := []Event{
		{ID: "wbe_1", RunID: run.ID, Seq: 1, Kind: EventRunQueued},
		{ID: "wbe_2", RunID: run.ID, Seq: 2, Kind: EventRunStarted},
		{ID: "wbe_3", RunID: run.ID, Seq: 3, Kind: EventRunSucceeded},
	}

	stream, err := ReadEventStream(EventRead{Run: run, Events: events, Cursor: StreamCursor{RunID: run.ID, Seq: 1}, OldestAvailableSeq: 1})
	if err != nil {
		t.Fatalf("ReadEventStream() error = %v", err)
	}
	if len(stream.Events) != 2 || stream.Events[0].Seq != 2 || stream.Events[1].Seq != 3 {
		t.Fatalf("events = %#v", stream.Events)
	}
	if stream.Cursor.Seq != 3 || !stream.Terminal {
		t.Fatalf("stream metadata = %#v", stream)
	}
}

func TestEvent_StaleCursorReturnsTypedReset(t *testing.T) {
	t.Parallel()

	run := Run{ID: "wbr_events", State: StateRunning, NextEventSeq: 8, UpdatedAt: time.Now()}
	_, err := ReadEventStream(EventRead{Run: run, Cursor: StreamCursor{RunID: run.ID, Seq: 2}, OldestAvailableSeq: 6})
	if !errors.Is(err, ErrStaleCursor) {
		t.Fatalf("ReadEventStream() error = %v, want ErrStaleCursor", err)
	}
	var stale *StaleCursorError
	if !errors.As(err, &stale) || stale.Reset.Cursor.Seq != 7 || stale.Reset.Run.ID != run.ID {
		t.Fatalf("stale cursor reset = %#v", stale)
	}
}
