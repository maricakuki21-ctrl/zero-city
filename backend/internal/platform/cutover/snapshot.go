package cutover

import (
	"encoding/json"
	"errors"
	"strings"
)

var (
	ErrDuplicateSnapshotIdentity = errors.New("duplicate handoff snapshot identity")
	ErrInvalidSnapshotIdentity   = errors.New("invalid handoff snapshot identity")
	ErrSnapshotClosed            = errors.New("handoff snapshot is only available while fenced")
)

type SnapshotItem struct {
	Identity string          `json:"identity"`
	Payload  json.RawMessage `json:"payload"`
}

type HandoffSnapshot struct {
	Authority            Authority      `json:"authority"`
	ActiveNonMedia       []SnapshotItem `json:"active_non_media"`
	Holds                []SnapshotItem `json:"holds"`
	AcceptedMediaTaskIDs []SnapshotItem `json:"accepted_media_task_ids"`
	TerminalUnknown      []SnapshotItem `json:"terminal_unknown"`
	UnconsumedOutbox     []SnapshotItem `json:"unconsumed_outbox"`
}

func (s HandoffSnapshot) Validate() error {
	seen := make(map[string]struct{})
	collections := [][]SnapshotItem{
		s.ActiveNonMedia,
		s.Holds,
		s.AcceptedMediaTaskIDs,
		s.TerminalUnknown,
		s.UnconsumedOutbox,
	}
	for _, items := range collections {
		for _, item := range items {
			identity := strings.TrimSpace(item.Identity)
			if identity == "" {
				return ErrInvalidSnapshotIdentity
			}
			if _, exists := seen[identity]; exists {
				return ErrDuplicateSnapshotIdentity
			}
			seen[identity] = struct{}{}
		}
	}
	return nil
}
