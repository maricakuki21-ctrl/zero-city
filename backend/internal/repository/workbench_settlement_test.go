package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	platformledger "github.com/Wei-Shaw/sub2api/internal/platform/ledger"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestWorkbenchSettlementSourcePreflightRejectsWithoutDatabaseOrLedger(t *testing.T) {
	source := ProvideWorkbenchSettlementSource(nil, nil)
	err := source.PreflightCanonicalUsage(context.Background(), service.WorkbenchCanonicalRequest{})
	require.ErrorIs(t, err, service.ErrWorkbenchCanonicalSettlement)
	require.ErrorContains(t, err, "authoritative finalized cost")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, source.PreflightCanonicalUsage(ctx, service.WorkbenchCanonicalRequest{}), context.Canceled)
}

func TestWorkbenchSettlementSource_ClaimCanonicalUsage_rejectsUnpricedUsageWithoutWrites(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	quote := mustWorkbenchQuote(t)
	event := mustWorkbenchUsageEvent(t, "req_1", 41, 81, 17, quote.Model())
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	payloadSHA, err := platformledger.CanonicalUsagePayloadSHA256(event)
	require.NoError(t, err)
	source := ProvideWorkbenchSettlementSource(db, NewBizDecipherLedgerRepository(db))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectCanonicalUsageOutboxSQL)).WithArgs("req_1", int64(71)).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "request_id", "api_key_id", "account_id", "group_id", "payload", "payload_sha256"}).
			AddRow(event.EventID(), "req_1", int64(71), int64(81), int64(17), payload, payloadSHA))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectAcceptedQuoteSQL)).WithArgs(quote.QuoteID()).WillReturnRows(workbenchQuoteRows(quote))

	// When
	got, err := source.ClaimCanonicalUsage(context.Background(), service.WorkbenchCanonicalUsageClaim{
		RequestID: "req_1", APIKeyID: 71, UserID: 41, AccountID: 81, GroupID: 17,
		QuoteID: workbench.QuoteID(quote.QuoteID()), QuoteSHA: workbench.Digest(quote.SnapshotSHA256()),
	})

	// Then
	require.ErrorIs(t, err, service.ErrWorkbenchCanonicalSettlement)
	require.ErrorContains(t, err, "authoritative finalized cost")
	require.Empty(t, got.Journal.Entries)
	require.Nil(t, got.ActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchSettlementSource_ClaimCanonicalUsage_rejectsQuoteSHAMismatch(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	quote := mustWorkbenchQuote(t)
	event := mustWorkbenchUsageEvent(t, "req_2", 41, 81, 17, quote.Model())
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	payloadSHA, err := platformledger.CanonicalUsagePayloadSHA256(event)
	require.NoError(t, err)
	source := ProvideWorkbenchSettlementSource(db, NewBizDecipherLedgerRepository(db))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectCanonicalUsageOutboxSQL)).WithArgs("req_2", int64(71)).
		WillReturnRows(sqlmock.NewRows([]string{"event_id", "request_id", "api_key_id", "account_id", "group_id", "payload", "payload_sha256"}).
			AddRow(event.EventID(), "req_2", int64(71), int64(81), int64(17), payload, payloadSHA))
	mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectAcceptedQuoteSQL)).WithArgs(quote.QuoteID()).WillReturnRows(workbenchQuoteRows(quote))

	// When
	_, err = source.ClaimCanonicalUsage(context.Background(), service.WorkbenchCanonicalUsageClaim{
		RequestID: "req_2", APIKeyID: 71, UserID: 41, AccountID: 81, GroupID: 17,
		QuoteID: workbench.QuoteID(quote.QuoteID()), QuoteSHA: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	})

	// Then
	require.ErrorIs(t, err, service.ErrWorkbenchCanonicalSettlement)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkbenchSettlementSource_ClaimCanonicalUsage_rejectsCorruptOutbox(t *testing.T) {
	for _, mismatch := range []string{"hash", "request_id"} {
		t.Run(mismatch, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			quote := mustWorkbenchQuote(t)
			event := mustWorkbenchUsageEvent(t, "req_integrity", 41, 81, 17, quote.Model())
			payload, err := json.Marshal(event)
			require.NoError(t, err)
			payloadSHA, err := platformledger.CanonicalUsagePayloadSHA256(event)
			require.NoError(t, err)
			requestID := event.RequestID()
			if mismatch == "hash" {
				payloadSHA = "corrupt"
			} else {
				requestID = "other_request"
			}
			mock.ExpectQuery(regexp.QuoteMeta(workbenchSelectCanonicalUsageOutboxSQL)).
				WithArgs(event.RequestID(), int64(71)).
				WillReturnRows(sqlmock.NewRows([]string{"event_id", "request_id", "api_key_id", "account_id", "group_id", "payload", "payload_sha256"}).
					AddRow(event.EventID(), requestID, int64(71), int64(81), int64(17), payload, payloadSHA))
			source := ProvideWorkbenchSettlementSource(db, nil)
			result, err := source.ClaimCanonicalUsage(context.Background(), service.WorkbenchCanonicalUsageClaim{
				RequestID: event.RequestID(), APIKeyID: 71, UserID: 41, AccountID: 81, GroupID: 17,
				QuoteID: workbench.QuoteID(quote.QuoteID()), QuoteSHA: workbench.Digest(quote.SnapshotSHA256()),
			})
			require.ErrorIs(t, err, service.ErrWorkbenchCanonicalSettlement)
			require.Nil(t, result.ActualCost)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkbenchSettlementSource_NoFinalizedPriceNeverUsesAuthorizationCeiling(t *testing.T) {
	for _, units := range []corecontracts.ExactUsageUnits{
		{},
		{InputTokens: 1, OutputTokens: 1},
		{InputTokens: 1000000, OutputTokens: 1000000},
		{ImageCount: 1},
	} {
		t.Run(fmt.Sprintf("%+v", units), func(t *testing.T) {
			now := time.Now()
			quote := mustWorkbenchQuote(t)
			event, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
				Policy: corecontracts.BillingPolicyBizDecipherLedger, RequestID: "request", UsageReference: "usage",
				UserID: 41, AccountID: 81, GroupID: 17, Model: quote.Model(), Protocol: "/v1/messages:messages",
				Units: units, CommitState: corecontracts.CanonicalUsageCommitStateUsageCommitted,
				CommittedAt: now, FinalizedAt: now,
			})
			require.NoError(t, err)
			// A nil ledger also proves no account creation is attempted.
			source := &workbenchSettlementSource{}
			journal, actual, err := source.canonicalUsageJournal(context.Background(), event, quote)
			require.ErrorIs(t, err, service.ErrWorkbenchCanonicalSettlement)
			require.Empty(t, journal.Entries)
			require.Nil(t, actual)
		})
	}
}

func mustWorkbenchUsageEvent(t *testing.T, requestID string, userID, accountID, groupID int64, model string) corecontracts.CanonicalUsageFinalized {
	t.Helper()
	now := time.Date(2026, time.September, 3, 3, 0, 0, 0, time.UTC)
	event, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy: corecontracts.BillingPolicyBizDecipherLedger, RequestID: requestID, UsageReference: "sub2-usage:71:" + requestID,
		UserID: userID, AccountID: accountID, GroupID: groupID, Model: model, Protocol: "/v1/messages:messages",
		CommitState: corecontracts.CanonicalUsageCommitStateUsageCommitted, CommittedAt: now, FinalizedAt: now,
	})
	require.NoError(t, err)
	return event
}

func expectWorkbenchLedgerAccount(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO bizdecipher_ledger_accounts`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO bizdecipher_ledger_projections`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}
