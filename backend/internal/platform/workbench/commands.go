package workbench

type LaunchCommand struct {
	Identity               Identity
	CapabilityID           string
	CapabilityVersion      string
	CapabilityDigest       string
	CanonicalModelID       string
	CanonicalModelVersion  string
	AcceptedQuoteID        string
	AcceptedQuoteSHA       string
	Intent                 string
	IdempotencyKey         string
	RequiredProtocolFamily string
}

type RunCommand struct {
	Identity       Identity
	RunID          RunID
	IdempotencyKey string
}

type SaveCommand struct {
	Identity       Identity
	RunID          RunID
	ReplayLabel    string
	IdempotencyKey string
}

type ReplayCommand struct {
	Identity       Identity
	ReplayID       SnapshotID
	IdempotencyKey string
}

type ForkCommand struct {
	Identity       Identity
	RunID          RunID
	IdempotencyKey string
}

type CancelCommand struct {
	Identity       Identity
	RunID          RunID
	Reason         string
	IdempotencyKey string
}

type ArtifactCommand struct {
	Identity   Identity
	RunID      RunID
	ArtifactID ArtifactID
}

type EventsCommand struct {
	Identity Identity
	RunID    RunID
	Cursor   StreamCursor
	Limit    uint32
}

type LaunchResult struct {
	Run      Run  `json:"run"`
	Replayed bool `json:"replayed"`
}

type RunResult struct {
	Run Run `json:"run"`
}

type SaveResult struct {
	Run      Run           `json:"run"`
	Snapshot SavedSnapshot `json:"snapshot"`
	Replayed bool          `json:"replayed"`
}

type ReplayResult struct {
	Run              Run        `json:"run"`
	SourceSnapshotID SnapshotID `json:"source_snapshot_id"`
	Replayed         bool       `json:"replayed"`
}

type ForkResult struct {
	Run              Run        `json:"run"`
	SourceSnapshotID SnapshotID `json:"source_snapshot_id"`
	Replayed         bool       `json:"replayed"`
}

type CancelResult struct {
	Run      Run  `json:"run"`
	Replayed bool `json:"replayed"`
}
