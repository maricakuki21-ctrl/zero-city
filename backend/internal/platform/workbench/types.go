package workbench

import (
	"fmt"
	"strings"
	"time"
)

type WorkspaceID string
type RunID string
type ArtifactID string
type SnapshotID string
type EventID string
type OperationID string
type CapabilityID string
type QuoteID string
type Digest string

type ActorID int64

type Identity struct {
	ActorID ActorID `json:"actor_id"`
}

func (i Identity) Validate() error {
	if i.ActorID <= 0 {
		return fmt.Errorf("%w: actor id must be positive", ErrInvalidCommand)
	}
	return nil
}

type Decimal string

func NewDecimal(value string) (Decimal, error) {
	if value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
		return "", ErrInvalidDecimal
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || len(parts[0]) > 12 || len(parts) == 2 && (parts[1] == "" || len(parts[1]) > 12) {
		return "", ErrInvalidDecimal
	}
	if len(parts[0]) > 1 && parts[0][0] == '0' {
		return "", ErrInvalidDecimal
	}
	for _, part := range parts {
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return "", ErrInvalidDecimal
			}
		}
	}
	return Decimal(value), nil
}

type Money struct {
	Currency string  `json:"currency"`
	Amount   Decimal `json:"amount"`
}

type Capability struct {
	ID                    CapabilityID `json:"id"`
	Version               string       `json:"version"`
	Digest                Digest       `json:"digest"`
	CanonicalModelID      string       `json:"canonical_model_id"`
	CanonicalModelVersion string       `json:"canonical_model_version"`
	Title                 string       `json:"title"`
	Summary               string       `json:"summary"`
	Estimate              *Money       `json:"estimate,omitempty"`
	AcceptedQuoteID       QuoteID      `json:"accepted_quote_id,omitempty"`
	AcceptedQuoteSHA      Digest       `json:"accepted_quote_sha,omitempty"`
	Protocol              string       `json:"protocol,omitempty"`
}

type InputSnapshot struct {
	Intent                string       `json:"intent"`
	CapabilityID          CapabilityID `json:"capability_id"`
	CapabilityVersion     string       `json:"capability_version"`
	CapabilityDigest      Digest       `json:"capability_digest"`
	CanonicalModelID      string       `json:"canonical_model_id"`
	CanonicalModelVersion string       `json:"canonical_model_version"`
	AcceptedQuoteID       QuoteID      `json:"accepted_quote_id"`
	AcceptedQuoteSHA      Digest       `json:"accepted_quote_sha"`
}

type Workspace struct {
	ID             WorkspaceID     `json:"id"`
	OwnerID        ActorID         `json:"owner_id"`
	Version        uint64          `json:"version"`
	Capabilities   []Capability    `json:"capabilities"`
	CurrentRun     *Run            `json:"current_run,omitempty"`
	SavedSnapshots []SavedSnapshot `json:"saved_snapshots"`
	Cursor         StreamCursor    `json:"cursor"`
	HydratedAt     time.Time       `json:"hydrated_at"`
}

type RunFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Run struct {
	ID                            RunID         `json:"id"`
	OwnerID                       ActorID       `json:"owner_id"`
	WorkspaceID                   WorkspaceID   `json:"workspace_id"`
	State                         RunState      `json:"state"`
	Input                         InputSnapshot `json:"input"`
	InputSHA256                   Digest        `json:"input_sha256"`
	Version                       uint64        `json:"version"`
	NextEventSeq                  uint64        `json:"next_event_seq"`
	ReplayOfRunID                 RunID         `json:"replay_of_run_id,omitempty"`
	ForkedFromRunID               RunID         `json:"forked_from_run_id,omitempty"`
	EstimatedCost                 *Money        `json:"estimated_cost,omitempty"`
	ActualCost                    *Money        `json:"actual_cost,omitempty"`
	Artifacts                     []Artifact    `json:"artifacts"`
	AcceptedQuoteID               QuoteID       `json:"accepted_quote_id,omitempty"`
	AcceptedQuoteSHA              Digest        `json:"accepted_quote_sha,omitempty"`
	CanonicalRequestID            string        `json:"canonical_request_id,omitempty"`
	CanonicalUsageEventID         string        `json:"canonical_usage_event_id,omitempty"`
	JournalID                     string        `json:"journal_id,omitempty"`
	MediaTaskID                   string        `json:"media_task_id,omitempty"`
	CanonicalMediaBusinessEventID string        `json:"canonical_media_business_event_id,omitempty"`
	RunnerJobID                   string        `json:"runner_job_id,omitempty"`
	Failure                       *RunFailure   `json:"failure,omitempty"`
	CreatedAt                     time.Time     `json:"created_at"`
	UpdatedAt                     time.Time     `json:"updated_at"`
	TerminalAt                    *time.Time    `json:"terminal_at,omitempty"`
}

func (r Run) Clone() Run {
	cloned := r
	cloned.EstimatedCost = cloneMoney(r.EstimatedCost)
	cloned.ActualCost = cloneMoney(r.ActualCost)
	cloned.Failure = cloneFailure(r.Failure)
	cloned.TerminalAt = cloneTime(r.TerminalAt)
	cloned.Artifacts = make([]Artifact, len(r.Artifacts))
	for index := range r.Artifacts {
		cloned.Artifacts[index] = r.Artifacts[index].Clone()
	}
	return cloned
}

func cloneMoney(value *Money) *Money {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneFailure(value *RunFailure) *RunFailure {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

type ArtifactKind string

const (
	ArtifactText  ArtifactKind = "text"
	ArtifactCode  ArtifactKind = "code"
	ArtifactImage ArtifactKind = "image"
	ArtifactVideo ArtifactKind = "video"
)

type Artifact struct {
	ArtifactID                    ArtifactID   `json:"artifact_id"`
	RunID                         RunID        `json:"run_id"`
	OwnerID                       ActorID      `json:"owner_id"`
	Kind                          ArtifactKind `json:"kind"`
	ContentType                   string       `json:"content_type"`
	Body                          []byte       `json:"-"`
	StorageURI                    string       `json:"storage_uri"`
	ByteSize                      int64        `json:"byte_size"`
	ArtifactDigest                Digest       `json:"artifact_digest"`
	AdapterDigest                 Digest       `json:"adapter_digest"`
	CapabilityDigest              Digest       `json:"capability_digest"`
	CanonicalRequestID            string       `json:"canonical_request_id"`
	CanonicalUsageEventID         string       `json:"canonical_usage_event_id"`
	AcceptedQuoteID               QuoteID      `json:"accepted_quote_id"`
	AcceptedQuoteSHA              Digest       `json:"accepted_quote_sha"`
	JournalID                     string       `json:"journal_id"`
	MediaTaskID                   string       `json:"media_task_id"`
	CanonicalMediaBusinessEventID string       `json:"canonical_media_business_event_id"`
	RunnerJobID                   string       `json:"runner_job_id"`
	UpstreamTaskID                string       `json:"upstream_task_id"`
	CreatedAt                     time.Time    `json:"created_at"`
}

func (a Artifact) Clone() Artifact {
	cloned := a
	cloned.Body = append([]byte(nil), a.Body...)
	return cloned
}
