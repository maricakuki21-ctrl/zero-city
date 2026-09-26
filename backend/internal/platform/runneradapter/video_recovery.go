package runneradapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/platform/mediatask"
)

var (
	ErrVideoRecoveryInvalid  = errors.New("runneradapter: invalid video recovery configuration or request")
	ErrVideoBindingMissing   = errors.New("runneradapter: canonical video binding is missing")
	ErrVideoBindingPending   = errors.New("runneradapter: canonical video binding is pending")
	ErrVideoBindingConflict  = errors.New("runneradapter: canonical video binding conflicts with recovery request")
	ErrVideoBindingUnknown   = errors.New("runneradapter: canonical video binding state is unknown")
	ErrVideoRecoveryPending  = errors.New("runneradapter: video recovery is pending")
	ErrVideoRecoveryTerminal = errors.New("runneradapter: video task terminated without an artifact")
	ErrVideoStatusUnknown    = errors.New("runneradapter: video provider status is unknown")
	ErrVideoArtifactInvalid  = errors.New("runneradapter: recovered video artifact is invalid")
	ErrVideoArtifactTooLarge = errors.New("runneradapter: recovered video artifact exceeds the configured limit")
	ErrVideoArtifactConflict = errors.New("runneradapter: recovered video artifact conflicts with stored artifact")
)

type VideoRecoveryRequest struct {
	APIKeyID       int64
	UserID         int64
	GroupID        int64
	UpstreamTaskID string
}

type VideoOrigin struct {
	AccountID      int64
	UpstreamTaskID string
}

type VideoContent struct {
	MediaType string
	Body      []byte
}

type RecoveredVideoArtifact struct {
	MediaTaskID string
	AccountID   int64
	MediaType   string
	SHA256      string
	Body        []byte
}

type VideoBindingResolver interface {
	ResolveCanonicalVideo(context.Context, VideoRecoveryRequest) (mediatask.Binding, error)
}

type VideoProvider interface {
	Status(context.Context, VideoOrigin) (mediatask.Observation, error)
	Cancel(context.Context, VideoOrigin) (mediatask.Observation, error)
	Content(context.Context, VideoOrigin) (VideoContent, error)
}

type VideoArtifactStore interface {
	Store(context.Context, RecoveredVideoArtifact) error
}

type VideoRecoveryConfig struct {
	Bindings         VideoBindingResolver
	Provider         VideoProvider
	Artifacts        VideoArtifactStore
	MaxArtifactBytes int
}

type VideoRecovery struct {
	bindings         VideoBindingResolver
	provider         VideoProvider
	artifacts        VideoArtifactStore
	maxArtifactBytes int
}

func NewVideoRecovery(config VideoRecoveryConfig) (*VideoRecovery, error) {
	if config.Bindings == nil || config.Provider == nil || config.Artifacts == nil || config.MaxArtifactBytes <= 0 {
		return nil, ErrVideoRecoveryInvalid
	}
	return &VideoRecovery{
		bindings: config.Bindings, provider: config.Provider,
		artifacts: config.Artifacts, maxArtifactBytes: config.MaxArtifactBytes,
	}, nil
}

func (r *VideoRecovery) Resume(ctx context.Context, request VideoRecoveryRequest) (RecoveredVideoArtifact, error) {
	binding, err := r.resolve(ctx, request)
	if err != nil {
		return RecoveredVideoArtifact{}, err
	}
	observation, err := r.status(ctx, binding)
	if err != nil {
		return RecoveredVideoArtifact{}, err
	}
	switch observation.State {
	case mediatask.StateCreating, mediatask.StateAccepted, mediatask.StateProcessing:
		return RecoveredVideoArtifact{}, ErrVideoRecoveryPending
	case mediatask.StateSucceeded:
		return r.content(ctx, binding)
	case mediatask.StateFailed, mediatask.StateCancelled:
		return RecoveredVideoArtifact{}, ErrVideoRecoveryTerminal
	default:
		return RecoveredVideoArtifact{}, ErrVideoStatusUnknown
	}
}

func (r *VideoRecovery) Status(ctx context.Context, request VideoRecoveryRequest) (mediatask.Observation, error) {
	binding, err := r.resolve(ctx, request)
	if err != nil {
		return mediatask.Observation{}, err
	}
	return r.status(ctx, binding)
}

func (r *VideoRecovery) Cancel(ctx context.Context, request VideoRecoveryRequest) (mediatask.Observation, error) {
	binding, err := r.resolve(ctx, request)
	if err != nil {
		return mediatask.Observation{}, err
	}
	observation, err := r.provider.Cancel(ctx, videoOrigin(binding))
	if err != nil {
		return mediatask.Observation{}, fmt.Errorf("cancel original video task: %w", err)
	}
	if err := validateVideoObservation(observation); err != nil {
		return mediatask.Observation{}, err
	}
	return observation, nil
}

func (r *VideoRecovery) Content(ctx context.Context, request VideoRecoveryRequest) (RecoveredVideoArtifact, error) {
	binding, err := r.resolve(ctx, request)
	if err != nil {
		return RecoveredVideoArtifact{}, err
	}
	return r.content(ctx, binding)
}

func (r *VideoRecovery) resolve(ctx context.Context, request VideoRecoveryRequest) (mediatask.Binding, error) {
	if err := validateVideoRecoveryRequest(request); err != nil {
		return mediatask.Binding{}, err
	}
	if err := ctx.Err(); err != nil {
		return mediatask.Binding{}, err
	}
	binding, err := r.bindings.ResolveCanonicalVideo(ctx, request)
	if err != nil {
		return mediatask.Binding{}, fmt.Errorf("resolve canonical video binding: %w", err)
	}
	if videoBindingMissing(binding) {
		return mediatask.Binding{}, ErrVideoBindingMissing
	}
	switch binding.State {
	case mediatask.BindingPending:
		return mediatask.Binding{}, ErrVideoBindingPending
	case mediatask.BindingAccepted:
	case mediatask.BindingState(""):
		return mediatask.Binding{}, ErrVideoBindingMissing
	default:
		return mediatask.Binding{}, ErrVideoBindingUnknown
	}
	if !binding.Accepted() || binding.APIKeyID != request.APIKeyID || binding.UserID != request.UserID ||
		binding.GroupID != request.GroupID || binding.AccountID <= 0 || binding.UpstreamTaskID != request.UpstreamTaskID ||
		!validVideoBindingEndpoint(binding.Endpoint) || strings.TrimSpace(binding.Context.BusinessEventID()) == "" ||
		strings.TrimSpace(binding.Context.IdempotencyKey()) == "" || !validDigest(binding.Context.RequestHash()) {
		return mediatask.Binding{}, ErrVideoBindingConflict
	}
	return binding, nil
}

func (r *VideoRecovery) status(ctx context.Context, binding mediatask.Binding) (mediatask.Observation, error) {
	observation, err := r.provider.Status(ctx, videoOrigin(binding))
	if err != nil {
		return mediatask.Observation{}, fmt.Errorf("status original video task: %w", err)
	}
	if err := validateVideoObservation(observation); err != nil {
		return mediatask.Observation{}, err
	}
	return observation, nil
}

func (r *VideoRecovery) content(ctx context.Context, binding mediatask.Binding) (RecoveredVideoArtifact, error) {
	content, err := r.provider.Content(ctx, videoOrigin(binding))
	if err != nil {
		return RecoveredVideoArtifact{}, fmt.Errorf("content original video task: %w", err)
	}
	mediaType, _, err := mime.ParseMediaType(content.MediaType)
	if err != nil || !strings.EqualFold(mediaType, "video/mp4") || len(content.Body) == 0 || !validMP4(content.Body) {
		return RecoveredVideoArtifact{}, ErrVideoArtifactInvalid
	}
	if len(content.Body) > r.maxArtifactBytes {
		return RecoveredVideoArtifact{}, ErrVideoArtifactTooLarge
	}
	body := bytes.Clone(content.Body)
	digest := sha256.Sum256(body)
	artifact := RecoveredVideoArtifact{
		MediaTaskID: binding.UpstreamTaskID, AccountID: binding.AccountID,
		MediaType: "video/mp4", SHA256: hex.EncodeToString(digest[:]), Body: body,
	}
	stored := artifact
	stored.Body = bytes.Clone(body)
	if err := r.artifacts.Store(ctx, stored); err != nil {
		return RecoveredVideoArtifact{}, fmt.Errorf("store verified video artifact: %w", err)
	}
	return artifact, nil
}

func validateVideoRecoveryRequest(request VideoRecoveryRequest) error {
	if request.APIKeyID <= 0 || request.UserID <= 0 || request.GroupID <= 0 ||
		request.UpstreamTaskID == "" || len(request.UpstreamTaskID) > 200 ||
		strings.TrimSpace(request.UpstreamTaskID) != request.UpstreamTaskID {
		return ErrVideoRecoveryInvalid
	}
	return nil
}

func validateVideoObservation(observation mediatask.Observation) error {
	switch observation.State {
	case mediatask.StateCreating, mediatask.StateAccepted, mediatask.StateProcessing,
		mediatask.StateSucceeded, mediatask.StateFailed, mediatask.StateCancelled:
		return nil
	default:
		return ErrVideoStatusUnknown
	}
}

func videoOrigin(binding mediatask.Binding) VideoOrigin {
	return VideoOrigin{AccountID: binding.AccountID, UpstreamTaskID: binding.UpstreamTaskID}
}

func videoBindingMissing(binding mediatask.Binding) bool {
	return binding.State == "" && binding.APIKeyID == 0 && binding.UserID == 0 && binding.GroupID == 0 &&
		binding.AccountID == 0 && binding.UpstreamTaskID == "" && binding.Context.BusinessEventID() == ""
}

func validVideoBindingEndpoint(endpoint string) bool {
	switch endpoint {
	case "videos_generations", "videos_edits", "videos_extensions":
		return true
	default:
		return false
	}
}

func validMP4(body []byte) bool {
	if len(body) < 16 || string(body[4:8]) != "ftyp" {
		return false
	}
	boxSize := binary.BigEndian.Uint32(body[:4])
	return boxSize >= 16 && uint64(boxSize) <= uint64(len(body))
}
