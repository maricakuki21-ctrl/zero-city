package runneradapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/mediatask"
	"github.com/stretchr/testify/require"
)

var errVideoBindingLookup = errors.New("video binding lookup failed")

type videoBindingResolverFake struct {
	binding  mediatask.Binding
	err      error
	requests []VideoRecoveryRequest
}

func (f *videoBindingResolverFake) ResolveCanonicalVideo(_ context.Context, request VideoRecoveryRequest) (mediatask.Binding, error) {
	f.requests = append(f.requests, request)
	return f.binding, f.err
}

type videoProviderCall struct {
	operation string
	origin    VideoOrigin
}

type videoProviderFake struct {
	observation mediatask.Observation
	content     VideoContent
	err         error
	calls       []videoProviderCall
	createCount int
}

func (f *videoProviderFake) Status(_ context.Context, origin VideoOrigin) (mediatask.Observation, error) {
	f.calls = append(f.calls, videoProviderCall{operation: "status", origin: origin})
	return f.observation, f.err
}

func (f *videoProviderFake) Cancel(_ context.Context, origin VideoOrigin) (mediatask.Observation, error) {
	f.calls = append(f.calls, videoProviderCall{operation: "cancel", origin: origin})
	return mediatask.Observation{State: mediatask.StateCancelled}, f.err
}

func (f *videoProviderFake) Content(_ context.Context, origin VideoOrigin) (VideoContent, error) {
	f.calls = append(f.calls, videoProviderCall{operation: "content", origin: origin})
	return f.content, f.err
}

func (f *videoProviderFake) Create() {
	f.createCount++
}

type videoArtifactStoreFake struct {
	mu        sync.Mutex
	artifacts map[string]RecoveredVideoArtifact
}

func newVideoArtifactStoreFake() *videoArtifactStoreFake {
	return &videoArtifactStoreFake{artifacts: make(map[string]RecoveredVideoArtifact)}
}

func (f *videoArtifactStoreFake) Store(_ context.Context, artifact RecoveredVideoArtifact) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.artifacts[artifact.MediaTaskID]
	if ok && existing.SHA256 != artifact.SHA256 {
		return ErrVideoArtifactConflict
	}
	artifact.Body = bytes.Clone(artifact.Body)
	f.artifacts[artifact.MediaTaskID] = artifact
	return nil
}

type fixtureVideoRenderer struct{}

func (fixtureVideoRenderer) Render() VideoContent {
	return VideoContent{
		MediaType: "video/mp4",
		Body: []byte{
			0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm',
			0, 0, 2, 0, 'i', 's', 'o', 'm', 'i', 's', 'o', '2',
			0, 0, 0, 8, 'm', 'd', 'a', 't',
		},
	}
}

func acceptedVideoBinding(t *testing.T) mediatask.Binding {
	t.Helper()
	createContext, err := mediatask.NewCreateContext("media:event-1", "client-key-1", []byte(`{"prompt":"clip"}`))
	require.NoError(t, err)
	return mediatask.Binding{
		Context: createContext, APIKeyID: 11, UserID: 22, GroupID: 33, AccountID: 44,
		Endpoint: "videos_generations", State: mediatask.BindingAccepted, UpstreamTaskID: "video-task-1",
	}
}

func newVideoRecoveryForTest(t *testing.T, resolver VideoBindingResolver, provider VideoProvider, store VideoArtifactStore, maxBytes int) *VideoRecovery {
	t.Helper()
	recovery, err := NewVideoRecovery(VideoRecoveryConfig{
		Bindings: resolver, Provider: provider, Artifacts: store, MaxArtifactBytes: maxBytes,
	})
	require.NoError(t, err)
	return recovery
}

func TestCanonicalVideoCreateIsNeverInvokedByVideoRecoveryAcceptedResume(t *testing.T) {
	// Given
	binding := acceptedVideoBinding(t)
	resolver := &videoBindingResolverFake{binding: binding}
	provider := &videoProviderFake{observation: mediatask.Observation{State: mediatask.StateSucceeded}, content: fixtureVideoRenderer{}.Render()}
	store := newVideoArtifactStoreFake()
	recovery := newVideoRecoveryForTest(t, resolver, provider, store, 1024)
	request := VideoRecoveryRequest{APIKeyID: 11, UserID: 22, GroupID: 33, UpstreamTaskID: "video-task-1"}

	// When
	first, err := recovery.Resume(context.Background(), request)
	require.NoError(t, err)
	second, err := recovery.Resume(context.Background(), request)

	// Then
	require.NoError(t, err)
	require.Equal(t, first.SHA256, second.SHA256)
	require.NotEmpty(t, first.SHA256)
	require.Zero(t, provider.createCount)
	require.Len(t, provider.calls, 4)
	for _, call := range provider.calls {
		require.Equal(t, VideoOrigin{AccountID: 44, UpstreamTaskID: "video-task-1"}, call.origin)
	}
	require.Equal(t, []string{"status", "content", "status", "content"}, []string{
		provider.calls[0].operation, provider.calls[1].operation, provider.calls[2].operation, provider.calls[3].operation,
	})
}

func TestAcceptedStatusVideoRecoveryUsesOriginalTaskAndAccountForStatusCancelContent(t *testing.T) {
	// Given
	binding := acceptedVideoBinding(t)
	resolver := &videoBindingResolverFake{binding: binding}
	provider := &videoProviderFake{observation: mediatask.Observation{State: mediatask.StateProcessing}, content: fixtureVideoRenderer{}.Render()}
	recovery := newVideoRecoveryForTest(t, resolver, provider, newVideoArtifactStoreFake(), 1024)
	request := VideoRecoveryRequest{APIKeyID: 11, UserID: 22, GroupID: 33, UpstreamTaskID: "video-task-1"}

	// When
	_, statusErr := recovery.Status(context.Background(), request)
	_, cancelErr := recovery.Cancel(context.Background(), request)
	_, contentErr := recovery.Content(context.Background(), request)

	// Then
	require.NoError(t, statusErr)
	require.NoError(t, cancelErr)
	require.NoError(t, contentErr)
	require.Len(t, provider.calls, 3)
	for _, call := range provider.calls {
		require.Equal(t, VideoOrigin{AccountID: 44, UpstreamTaskID: "video-task-1"}, call.origin)
	}
}

func TestVideoRecoveryFailsClosedForUnsafeBindingOrState(t *testing.T) {
	base := acceptedVideoBinding(t)
	tests := []struct {
		name    string
		binding mediatask.Binding
		err     error
		wantErr error
	}{
		{name: "missing binding", wantErr: ErrVideoBindingMissing},
		{name: "lookup failure", err: errVideoBindingLookup, wantErr: errVideoBindingLookup},
		{name: "pending binding", binding: withVideoBindingState(base, mediatask.BindingPending), wantErr: ErrVideoBindingPending},
		{name: "unknown binding state", binding: withVideoBindingState(base, mediatask.BindingState("unknown")), wantErr: ErrVideoBindingUnknown},
		{name: "conflicting account", binding: withVideoBindingAccount(base, 0), wantErr: ErrVideoBindingConflict},
		{name: "conflicting task", binding: withVideoBindingTask(base, "other-task"), wantErr: ErrVideoBindingConflict},
		{name: "non-video endpoint", binding: withVideoBindingEndpoint(base, "images_generations"), wantErr: ErrVideoBindingConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			resolver := &videoBindingResolverFake{binding: tt.binding, err: tt.err}
			provider := &videoProviderFake{observation: mediatask.Observation{State: mediatask.StateSucceeded}, content: fixtureVideoRenderer{}.Render()}
			recovery := newVideoRecoveryForTest(t, resolver, provider, newVideoArtifactStoreFake(), 1024)

			// When
			_, err := recovery.Resume(context.Background(), VideoRecoveryRequest{APIKeyID: 11, UserID: 22, GroupID: 33, UpstreamTaskID: "video-task-1"})

			// Then
			require.ErrorIs(t, err, tt.wantErr)
			require.Empty(t, provider.calls)
			require.Zero(t, provider.createCount)
		})
	}
}

func TestVideoRecoveryFailsClosedForPendingTerminalOrUnknownProviderState(t *testing.T) {
	tests := []struct {
		state   mediatask.State
		wantErr error
	}{
		{state: mediatask.StateCreating, wantErr: ErrVideoRecoveryPending},
		{state: mediatask.StateAccepted, wantErr: ErrVideoRecoveryPending},
		{state: mediatask.StateProcessing, wantErr: ErrVideoRecoveryPending},
		{state: mediatask.StateFailed, wantErr: ErrVideoRecoveryTerminal},
		{state: mediatask.StateCancelled, wantErr: ErrVideoRecoveryTerminal},
		{state: mediatask.State("unknown"), wantErr: ErrVideoStatusUnknown},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			// Given
			provider := &videoProviderFake{observation: mediatask.Observation{State: tt.state}, content: fixtureVideoRenderer{}.Render()}
			recovery := newVideoRecoveryForTest(t, &videoBindingResolverFake{binding: acceptedVideoBinding(t)}, provider, newVideoArtifactStoreFake(), 1024)

			// When
			_, err := recovery.Resume(context.Background(), VideoRecoveryRequest{APIKeyID: 11, UserID: 22, GroupID: 33, UpstreamTaskID: "video-task-1"})

			// Then
			require.ErrorIs(t, err, tt.wantErr)
			require.Len(t, provider.calls, 1)
			require.Equal(t, "status", provider.calls[0].operation)
		})
	}
}

func TestArtifactVerificationRejectsUnsafeVideoAndStoreConflict(t *testing.T) {
	valid := fixtureVideoRenderer{}.Render()
	tests := []struct {
		name     string
		content  VideoContent
		maxBytes int
		wantErr  error
	}{
		{name: "empty", content: VideoContent{MediaType: "video/mp4"}, maxBytes: 1024, wantErr: ErrVideoArtifactInvalid},
		{name: "wrong media type", content: VideoContent{MediaType: "text/plain", Body: valid.Body}, maxBytes: 1024, wantErr: ErrVideoArtifactInvalid},
		{name: "oversize", content: valid, maxBytes: len(valid.Body) - 1, wantErr: ErrVideoArtifactTooLarge},
		{name: "malformed mp4", content: VideoContent{MediaType: "video/mp4", Body: []byte("not-an-mp4")}, maxBytes: 1024, wantErr: ErrVideoArtifactInvalid},
		{name: "truncated 12-byte ftyp", content: VideoContent{MediaType: "video/mp4", Body: []byte{0, 0, 0, 12, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}}, maxBytes: 1024, wantErr: ErrVideoArtifactInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &videoProviderFake{content: tt.content}
			recovery := newVideoRecoveryForTest(t, &videoBindingResolverFake{binding: acceptedVideoBinding(t)}, provider, newVideoArtifactStoreFake(), tt.maxBytes)

			_, err := recovery.Content(context.Background(), VideoRecoveryRequest{APIKeyID: 11, UserID: 22, GroupID: 33, UpstreamTaskID: "video-task-1"})

			require.ErrorIs(t, err, tt.wantErr)
		})
	}

	store := newVideoArtifactStoreFake()
	store.artifacts["video-task-1"] = RecoveredVideoArtifact{MediaTaskID: "video-task-1", SHA256: "different"}
	recovery := newVideoRecoveryForTest(t, &videoBindingResolverFake{binding: acceptedVideoBinding(t)}, &videoProviderFake{content: valid}, store, 1024)
	_, err := recovery.Content(context.Background(), VideoRecoveryRequest{APIKeyID: 11, UserID: 22, GroupID: 33, UpstreamTaskID: "video-task-1"})
	require.ErrorIs(t, err, ErrVideoArtifactConflict)
}

func withVideoBindingState(binding mediatask.Binding, state mediatask.BindingState) mediatask.Binding {
	binding.State = state
	return binding
}

func withVideoBindingAccount(binding mediatask.Binding, accountID int64) mediatask.Binding {
	binding.AccountID = accountID
	return binding
}

func withVideoBindingTask(binding mediatask.Binding, taskID string) mediatask.Binding {
	binding.UpstreamTaskID = taskID
	return binding
}

func withVideoBindingEndpoint(binding mediatask.Binding, endpoint string) mediatask.Binding {
	binding.Endpoint = endpoint
	return binding
}

func TestRunnerFixedSourceHashMatchesApprovedAdapterAssets(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "adapter.go", want: "f5eb88ca802c31e82724893688b6e1231e69cbd0c32d575746f30cb9734af094"},
		{path: "adapter_test.go", want: "e597cc349e65120275028655049fb17b93787c5375667a270cab726dae18c67b"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			body, err := os.ReadFile(tt.path)
			require.NoError(t, err)
			digest := sha256.Sum256(body)
			require.Equal(t, tt.want, hex.EncodeToString(digest[:]))
		})
	}
}

type sandboxSidecar struct {
	result       SidecarResult
	executeCount int
}

func (s *sandboxSidecar) Execute(context.Context, Request) (SidecarResult, error) {
	s.executeCount++
	return s.result, nil
}

func (s *sandboxSidecar) Cancel(context.Context, string) error { return nil }

func (s *sandboxSidecar) Resume(context.Context, string) (SidecarResult, error) {
	return s.result, nil
}

func TestRunnerSandboxPolicyFailsClosedForEgressHostPathOversizeMalformedSpecAndMissingArtifact(t *testing.T) {
	now := time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC)
	request := validRequest()
	request.Kind = KindVideo

	t.Run("egress artifact URI", func(t *testing.T) {
		sidecar := &sandboxSidecar{result: SidecarResult{State: StateCompleted, Artifacts: []Artifact{{
			ID: "video-1", RequestID: request.RequestID, URI: "https://example.test/video.mp4",
			SHA256: strings.Repeat("a", 64), CreatedAt: now,
		}}}}
		adapter := newSandboxAdapter(t, sidecar, now)
		_, err := adapter.Start(context.Background(), request)
		require.ErrorIs(t, err, ErrStaleArtifact)
	})

	t.Run("host path artifact URI", func(t *testing.T) {
		sidecar := &sandboxSidecar{result: SidecarResult{State: StateCompleted, Artifacts: []Artifact{{
			ID: "video-1", RequestID: request.RequestID, URI: "file:///C:/runner/video.mp4",
			SHA256: strings.Repeat("a", 64), CreatedAt: now,
		}}}}
		adapter := newSandboxAdapter(t, sidecar, now)
		_, err := adapter.Start(context.Background(), request)
		require.ErrorIs(t, err, ErrStaleArtifact)
	})

	t.Run("oversize request", func(t *testing.T) {
		sidecar := &sandboxSidecar{result: SidecarResult{State: StateRunning}}
		adapter := newSandboxAdapter(t, sidecar, now)
		oversize := request
		oversize.Prompt = strings.Repeat("x", 32*1024+1)
		_, err := adapter.Start(context.Background(), oversize)
		require.ErrorIs(t, err, ErrInvalidRequest)
		require.Zero(t, sidecar.executeCount)
	})

	t.Run("malformed sidecar manifest", func(t *testing.T) {
		_, err := New(SidecarSpec{
			Name: "video", Version: "1", Digest: "malformed", Kinds: []MediaKind{KindVideo},
		}, &sandboxSidecar{})
		require.ErrorIs(t, err, ErrInvalidRequest)
	})

	t.Run("completed without artifact", func(t *testing.T) {
		adapter := newSandboxAdapter(t, &sandboxSidecar{result: SidecarResult{State: StateCompleted}}, now)
		_, err := adapter.Start(context.Background(), request)
		require.True(t, errors.Is(err, ErrMisleadingSuccess))
	})
}

func newSandboxAdapter(t *testing.T, sidecar Sidecar, now time.Time) *Adapter {
	t.Helper()
	adapter, err := New(SidecarSpec{
		Name: "media", Version: "1", Digest: strings.Repeat("a", 64), Kinds: []MediaKind{KindVideo},
	}, sidecar)
	require.NoError(t, err)
	adapter.now = func() time.Time { return now }
	return adapter
}
