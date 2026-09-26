package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/stretchr/testify/require"
)

type workbenchCatalogSourceStub struct {
	record WorkbenchCatalogRecord
}

type emptyWorkbenchCatalogSource struct{ workbenchCatalogSourceStub }

func (emptyWorkbenchCatalogSource) ListWorkbenchCapabilities(context.Context, workbench.Identity) ([]workbench.Capability, error) {
	return nil, nil
}

func TestWorkbenchCatalogEmptyResourcesAreArray(t *testing.T) {
	catalog, err := NewWorkbenchCatalog(emptyWorkbenchCatalogSource{}, nil)
	require.NoError(t, err)
	resources, err := catalog.Capabilities(context.Background(), workbench.Identity{ActorID: 41})
	require.NoError(t, err)
	require.NotNil(t, resources)
	require.Empty(t, resources)
}

func (s workbenchCatalogSourceStub) ListWorkbenchCapabilities(context.Context, workbench.Identity) ([]workbench.Capability, error) {
	return []workbench.Capability{s.record.Capability}, nil
}

func (s workbenchCatalogSourceStub) ResolveWorkbenchCatalog(
	context.Context,
	workbench.Identity,
	workbench.CapabilityID,
	workbench.QuoteID,
) (WorkbenchCatalogRecord, error) {
	return s.record, nil
}

func TestWorkbenchCatalogCanonicalResolutionAcceptsMatchingAssertions(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 3, 2, 0, 0, 0, time.UTC)
	record := canonicalWorkbenchCatalogRecord(now)
	catalog, err := NewWorkbenchCatalog(workbenchCatalogSourceStub{record: record}, fixedWorkbenchClock{now: now})
	require.NoError(t, err)

	// When
	resolved, err := catalog.ResolveLaunch(context.Background(), canonicalWorkbenchLaunchCommand())

	// Then
	require.NoError(t, err)
	require.Equal(t, record.Capability, resolved.Capability)
	require.Equal(t, int64(17), resolved.GroupID)
	require.Equal(t, int64(41), resolved.User.ID)
	require.Equal(t, int64(71), resolved.APIKey.ID)
}

func TestWorkbenchCatalogCanonicalResolutionRejectsClientQuoteMismatch(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 3, 2, 0, 0, 0, time.UTC)
	catalog, err := NewWorkbenchCatalog(workbenchCatalogSourceStub{record: canonicalWorkbenchCatalogRecord(now)}, fixedWorkbenchClock{now: now})
	require.NoError(t, err)
	command := canonicalWorkbenchLaunchCommand()
	command.AcceptedQuoteSHA = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	// When
	_, err = catalog.ResolveLaunch(context.Background(), command)

	// Then
	require.ErrorIs(t, err, ErrWorkbenchCatalogMismatch)
}

func TestWorkbenchCatalogCanonicalResolutionRejectsExpiredQuote(t *testing.T) {
	// Given
	now := time.Date(2026, time.September, 3, 2, 0, 0, 0, time.UTC)
	record := canonicalWorkbenchCatalogRecord(now)
	record.Quote.ExpiresAt = now.Add(-time.Second)
	catalog, err := NewWorkbenchCatalog(workbenchCatalogSourceStub{record: record}, fixedWorkbenchClock{now: now})
	require.NoError(t, err)

	// When
	_, err = catalog.ResolveLaunch(context.Background(), canonicalWorkbenchLaunchCommand())

	// Then
	require.ErrorIs(t, err, ErrWorkbenchCatalogUnavailable)
}

func canonicalWorkbenchCatalogRecord(now time.Time) WorkbenchCatalogRecord {
	groupID := int64(17)
	return WorkbenchCatalogRecord{
		Capability: workbench.Capability{
			ID: "cap_text", Version: "v1", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			CanonicalModelID: "claude-3-5-sonnet-20241022", CanonicalModelVersion: "2024-10-22", Title: "Text", Summary: "Text generation",
		},
		Quote: WorkbenchAcceptedQuote{
			ID: "quote_1", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			ModelID: "claude-3-5-sonnet-20241022", ModelVersion: "2024-10-22", GroupID: groupID, ExpiresAt: now.Add(time.Minute),
		},
		User:   &User{ID: 41, Status: StatusActive},
		APIKey: &APIKey{ID: 71, UserID: 41, GroupID: &groupID, Status: StatusAPIKeyActive, User: &User{ID: 41}},
	}
}

func canonicalWorkbenchLaunchCommand() workbench.LaunchCommand {
	return workbench.LaunchCommand{
		Identity: workbench.Identity{ActorID: 41}, CapabilityID: "cap_text", CapabilityVersion: "v1",
		CapabilityDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CanonicalModelID: "claude-3-5-sonnet-20241022", CanonicalModelVersion: "2024-10-22",
		AcceptedQuoteID: "quote_1", AcceptedQuoteSHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Intent: "summarize the release", IdempotencyKey: "launch-once",
	}
}

type fixedWorkbenchClock struct{ now time.Time }

func (c fixedWorkbenchClock) Now() time.Time { return c.now }

var _ = errors.Is
