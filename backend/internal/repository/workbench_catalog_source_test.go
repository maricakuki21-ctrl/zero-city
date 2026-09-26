package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type workbenchUserLookupStub struct{ user *service.User }

func (s workbenchUserLookupStub) GetByID(context.Context, int64) (*service.User, error) {
	return s.user, nil
}

type workbenchAPIKeyLookupStub struct{ keys []service.APIKey }

func (s workbenchAPIKeyLookupStub) ListByUserID(context.Context, int64, pagination.PaginationParams, service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error) {
	return append([]service.APIKey(nil), s.keys...), &pagination.PaginationResult{Total: int64(len(s.keys)), Page: 1, PageSize: 100, Pages: 1}, nil
}

func TestWorkbenchRuntimeCatalogSource_ResolveWorkbenchCatalog_restoresPersistedQuoteAndActiveKey(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	quote := mustWorkbenchQuote(t)
	groupID := int64(17)
	source := &workbenchCatalogSource{
		db:    db,
		users: workbenchUserLookupStub{user: &service.User{ID: 41, Status: service.StatusActive}},
		keys:  workbenchAPIKeyLookupStub{keys: []service.APIKey{{ID: 71, UserID: 41, GroupID: &groupID, Status: service.StatusAPIKeyActive}}},
	}
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectAcceptedQuoteSQL)).WithArgs(quote.QuoteID()).WillReturnRows(workbenchQuoteRows(quote))

	// When
	record, err := source.ResolveWorkbenchCatalog(context.Background(), workbench.Identity{ActorID: 41}, "cap_ignored", workbench.QuoteID(quote.QuoteID()))

	// Then
	require.NoError(t, err)
	require.Equal(t, quote.Model(), record.Capability.CanonicalModelID)
	require.Equal(t, "2024-10-22", record.Capability.CanonicalModelVersion)
	require.Equal(t, record.Quote.ID, record.Capability.AcceptedQuoteID)
	require.Equal(t, record.Quote.SHA256, record.Capability.AcceptedQuoteSHA)
	require.Equal(t, workbench.QuoteID(quote.QuoteID()), record.Quote.ID)
	require.Equal(t, workbench.Digest(quote.SnapshotSHA256()), record.Quote.SHA256)
	require.Equal(t, groupID, record.Quote.GroupID)
	require.Equal(t, int64(41), record.User.ID)
	require.Equal(t, int64(71), record.APIKey.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchRuntimeCatalogSource_ResolveWorkbenchCatalog_rejectsMissingQuote(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	groupID := int64(17)
	source := &workbenchCatalogSource{
		db:    db,
		users: workbenchUserLookupStub{user: &service.User{ID: 41, Status: service.StatusActive}},
		keys:  workbenchAPIKeyLookupStub{keys: []service.APIKey{{ID: 71, UserID: 41, GroupID: &groupID, Status: service.StatusAPIKeyActive}}},
	}
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectAcceptedQuoteSQL)).WithArgs("quote_missing").WillReturnRows(sqlmock.NewRows([]string{
		"quote_id", "price_version_id", "model", "protocol", "asset", "canonical_snapshot", "snapshot_sha256", "maximum_authorized_cost", "accepted_at", "expires_at", "source_epoch",
	}))

	// When
	_, err = source.ResolveWorkbenchCatalog(context.Background(), workbench.Identity{ActorID: 41}, "cap_text", "quote_missing")

	// Then
	require.ErrorIs(t, err, service.ErrWorkbenchCatalogUnavailable)
	require.NoError(t, mock.ExpectationsWereMet())
}

func mustWorkbenchQuote(t *testing.T) billingcontract.AcceptedQuote {
	t.Helper()
	accepted := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
	expires := accepted.Add(time.Hour)
	snapshot := []byte(fmt.Sprintf(
		`{"model":"claude-3-5-sonnet-20241022","version":"2024-10-22","protocol":"chat","asset":"balance","unit_prices":{"input":"0.0001","output":"0.0002"},"multiplier":"1","fee_policy":"v1","max_authorized_cost":"0.0003","expires_at":"%s","source_epoch":3}`,
		expires.Format(time.RFC3339),
	))
	cost, err := billingcontract.ParseDecimal("0.0003")
	require.NoError(t, err)
	raw := sha256.Sum256(snapshot)
	quote, err := billingcontract.NewAcceptedQuote(billingcontract.AcceptedQuoteInput{
		QuoteID: "quote_1", PriceVersionID: "2024-10-22", Model: "claude-3-5-sonnet-20241022",
		Protocol: billingcontract.ProtocolChat, Asset: billingcontract.AssetBalance, CanonicalSnapshotJSON: snapshot,
		SnapshotSHA256:        hex.EncodeToString(raw[:]),
		MaximumAuthorizedCost: cost, AcceptedAt: accepted, ExpiresAt: expires, SourceEpoch: 3,
	})
	require.NoError(t, err)
	return quote
}

func workbenchQuoteRows(quote billingcontract.AcceptedQuote) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"quote_id", "price_version_id", "model", "protocol", "asset", "canonical_snapshot", "snapshot_sha256",
		"maximum_authorized_cost", "accepted_at", "expires_at", "source_epoch",
	}).AddRow(
		quote.QuoteID(), quote.PriceVersionID(), quote.Model(), string(quote.Protocol()), string(quote.Asset()),
		quote.CanonicalSnapshotJSON(), quote.SnapshotSHA256(), quote.MaximumAuthorizedCost().String(),
		quote.AcceptedAt(), quote.ExpiresAt(), int64(quote.SourceEpoch()),
	)
}
