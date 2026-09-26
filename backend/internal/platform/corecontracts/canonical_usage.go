package corecontracts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const CanonicalUsageFinalizedSchemaVersion = 1

var (
	ErrBillingPolicyInvalid              = errors.New("invalid billing policy")
	ErrCanonicalUsageIdentityRequired    = errors.New("canonical usage identity is required")
	ErrCanonicalUsageCommitStateRequired = errors.New("canonical usage commit state is required")
	ErrCanonicalUsageCommitStateInvalid  = errors.New("invalid canonical usage commit state")
	ErrCanonicalUsageUnitsInvalid        = errors.New("canonical usage units must be non-negative")
	ErrCanonicalUsageTimestampInvalid    = errors.New("canonical usage timestamps are invalid")
	ErrCanonicalUsageSchemaInvalid       = errors.New("invalid canonical usage schema")
)

type BillingPolicy string

const (
	BillingPolicyNativeSub2        BillingPolicy = "native-sub2"
	BillingPolicyBizDecipherLedger BillingPolicy = "bizdecipher-ledger"
)

func ResolveBillingPolicy(policy BillingPolicy) (BillingPolicy, error) {
	switch policy {
	case "", BillingPolicyNativeSub2:
		return BillingPolicyNativeSub2, nil
	case BillingPolicyBizDecipherLedger:
		return BillingPolicyBizDecipherLedger, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrBillingPolicyInvalid, policy)
	}
}

func (p BillingPolicy) UsesNativeUserBalance() bool {
	return p == BillingPolicyNativeSub2 || p == ""
}

type CanonicalUsageCommitState string

const (
	CanonicalUsageCommitStateUsageCommitted       CanonicalUsageCommitState = "usage-committed"
	CanonicalUsageCommitStateMediaTerminalUnknown CanonicalUsageCommitState = "media-terminal-unknown"
)

type ExactUsageUnits struct {
	InputTokens          int64 `json:"input_tokens"`
	OutputTokens         int64 `json:"output_tokens"`
	CacheCreationTokens  int64 `json:"cache_creation_tokens"`
	CacheReadTokens      int64 `json:"cache_read_tokens"`
	ImageInputTokens     int64 `json:"image_input_tokens"`
	ImageOutputTokens    int64 `json:"image_output_tokens"`
	ImageCount           int64 `json:"image_count"`
	VideoCount           int64 `json:"video_count"`
	VideoDurationSeconds int64 `json:"video_duration_seconds"`
}

func (u ExactUsageUnits) validate() error {
	if u.InputTokens < 0 || u.OutputTokens < 0 || u.CacheCreationTokens < 0 || u.CacheReadTokens < 0 ||
		u.ImageInputTokens < 0 || u.ImageOutputTokens < 0 || u.ImageCount < 0 ||
		u.VideoCount < 0 || u.VideoDurationSeconds < 0 {
		return ErrCanonicalUsageUnitsInvalid
	}
	return nil
}

type CanonicalUsageFinalizedInput struct {
	Policy         BillingPolicy
	RequestID      string
	UsageReference string
	UserID         int64
	AccountID      int64
	GroupID        int64
	Model          string
	Protocol       string
	Units          ExactUsageUnits
	CommitState    CanonicalUsageCommitState
	CommittedAt    time.Time
	FinalizedAt    time.Time
}

type CanonicalUsageFinalized struct {
	eventID        string
	requestID      string
	usageReference string
	userID         int64
	accountID      int64
	groupID        int64
	model          string
	protocol       string
	units          ExactUsageUnits
	commitState    CanonicalUsageCommitState
	committedAt    time.Time
	finalizedAt    time.Time
}

type canonicalUsageWire struct {
	SchemaVersion  int                       `json:"schema_version"`
	Policy         BillingPolicy             `json:"billing_policy"`
	EventID        string                    `json:"event_id"`
	RequestID      string                    `json:"request_id"`
	UsageReference string                    `json:"usage_reference"`
	UserID         int64                     `json:"user_id"`
	AccountID      int64                     `json:"account_id"`
	GroupID        int64                     `json:"group_id"`
	Model          string                    `json:"model"`
	Protocol       string                    `json:"protocol"`
	Units          ExactUsageUnits           `json:"units"`
	CommitState    CanonicalUsageCommitState `json:"commit_state"`
	CommittedAt    time.Time                 `json:"committed_at"`
	FinalizedAt    time.Time                 `json:"finalized_at"`
}

func NewCanonicalUsageFinalized(input CanonicalUsageFinalizedInput) (CanonicalUsageFinalized, error) {
	policy, err := ResolveBillingPolicy(input.Policy)
	if err != nil {
		return CanonicalUsageFinalized{}, err
	}
	if policy != BillingPolicyBizDecipherLedger {
		return CanonicalUsageFinalized{}, fmt.Errorf("%w: canonical ledger event requires %q", ErrBillingPolicyInvalid, BillingPolicyBizDecipherLedger)
	}

	requestID := strings.TrimSpace(input.RequestID)
	usageReference := strings.TrimSpace(input.UsageReference)
	model := strings.TrimSpace(input.Model)
	protocol := strings.TrimSpace(input.Protocol)
	if requestID == "" || usageReference == "" || input.UserID <= 0 || input.AccountID <= 0 || input.GroupID <= 0 || model == "" || protocol == "" {
		return CanonicalUsageFinalized{}, ErrCanonicalUsageIdentityRequired
	}
	if err := input.Units.validate(); err != nil {
		return CanonicalUsageFinalized{}, err
	}
	if input.CommitState == "" {
		return CanonicalUsageFinalized{}, ErrCanonicalUsageCommitStateRequired
	}
	switch input.CommitState {
	case CanonicalUsageCommitStateUsageCommitted, CanonicalUsageCommitStateMediaTerminalUnknown:
	default:
		return CanonicalUsageFinalized{}, fmt.Errorf("%w: %q", ErrCanonicalUsageCommitStateInvalid, input.CommitState)
	}
	if input.CommittedAt.IsZero() || input.FinalizedAt.IsZero() || input.FinalizedAt.Before(input.CommittedAt) {
		return CanonicalUsageFinalized{}, ErrCanonicalUsageTimestampInvalid
	}

	return CanonicalUsageFinalized{
		eventID:        canonicalUsageEventID(requestID, usageReference),
		requestID:      requestID,
		usageReference: usageReference,
		userID:         input.UserID,
		accountID:      input.AccountID,
		groupID:        input.GroupID,
		model:          model,
		protocol:       protocol,
		units:          input.Units,
		commitState:    input.CommitState,
		committedAt:    input.CommittedAt.UTC(),
		finalizedAt:    input.FinalizedAt.UTC(),
	}, nil
}

func canonicalUsageEventID(requestID, usageReference string) string {
	sum := sha256.Sum256([]byte("canonical-usage-finalized|" + requestID + "|" + usageReference))
	return "cue_" + hex.EncodeToString(sum[:])
}

func (e CanonicalUsageFinalized) EventID() string                        { return e.eventID }
func (e CanonicalUsageFinalized) RequestID() string                      { return e.requestID }
func (e CanonicalUsageFinalized) UsageReference() string                 { return e.usageReference }
func (e CanonicalUsageFinalized) UserID() int64                          { return e.userID }
func (e CanonicalUsageFinalized) AccountID() int64                       { return e.accountID }
func (e CanonicalUsageFinalized) GroupID() int64                         { return e.groupID }
func (e CanonicalUsageFinalized) Model() string                          { return e.model }
func (e CanonicalUsageFinalized) Protocol() string                       { return e.protocol }
func (e CanonicalUsageFinalized) Units() ExactUsageUnits                 { return e.units }
func (e CanonicalUsageFinalized) CommitState() CanonicalUsageCommitState { return e.commitState }
func (e CanonicalUsageFinalized) CommittedAt() time.Time                 { return e.committedAt }
func (e CanonicalUsageFinalized) FinalizedAt() time.Time                 { return e.finalizedAt }

func (e CanonicalUsageFinalized) MarshalJSON() ([]byte, error) {
	return json.Marshal(canonicalUsageWire{
		SchemaVersion:  CanonicalUsageFinalizedSchemaVersion,
		Policy:         BillingPolicyBizDecipherLedger,
		EventID:        e.eventID,
		RequestID:      e.requestID,
		UsageReference: e.usageReference,
		UserID:         e.userID,
		AccountID:      e.accountID,
		GroupID:        e.groupID,
		Model:          e.model,
		Protocol:       e.protocol,
		Units:          e.units,
		CommitState:    e.commitState,
		CommittedAt:    e.committedAt,
		FinalizedAt:    e.finalizedAt,
	})
}

func (e *CanonicalUsageFinalized) UnmarshalJSON(data []byte) error {
	var wire canonicalUsageWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.SchemaVersion != CanonicalUsageFinalizedSchemaVersion {
		return fmt.Errorf("%w: version %d", ErrCanonicalUsageSchemaInvalid, wire.SchemaVersion)
	}
	parsed, err := NewCanonicalUsageFinalized(CanonicalUsageFinalizedInput{
		Policy:         wire.Policy,
		RequestID:      wire.RequestID,
		UsageReference: wire.UsageReference,
		UserID:         wire.UserID,
		AccountID:      wire.AccountID,
		GroupID:        wire.GroupID,
		Model:          wire.Model,
		Protocol:       wire.Protocol,
		Units:          wire.Units,
		CommitState:    wire.CommitState,
		CommittedAt:    wire.CommittedAt,
		FinalizedAt:    wire.FinalizedAt,
	})
	if err != nil {
		return err
	}
	if wire.EventID != parsed.EventID() {
		return fmt.Errorf("%w: event_id does not match immutable identity", ErrCanonicalUsageIdentityRequired)
	}
	*e = parsed
	return nil
}
