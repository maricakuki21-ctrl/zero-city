package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	platformledger "github.com/Wei-Shaw/sub2api/internal/platform/ledger"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const workbenchSelectCanonicalUsageOutboxSQL = `SELECT event_id, request_id, api_key_id, account_id, group_id, payload, payload_sha256
	FROM canonical_usage_outbox WHERE request_id = $1 AND api_key_id = $2`

type workbenchSettlementSource struct {
	db     *sql.DB
	ledger *BizDecipherLedgerRepository
}

type workbenchLedgerWriter struct {
	repo *BizDecipherLedgerRepository
}

func ProvideWorkbenchSettlementSource(db *sql.DB, ledger *BizDecipherLedgerRepository) service.WorkbenchSettlementSource {
	return &workbenchSettlementSource{db: db, ledger: ledger}
}

func ProvideWorkbenchLedgerWriter(repo *BizDecipherLedgerRepository) service.WorkbenchLedgerWriter {
	return workbenchLedgerWriter{repo: repo}
}

func (s *workbenchSettlementSource) PreflightCanonicalUsage(ctx context.Context, _ service.WorkbenchCanonicalRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Until finalized pricing is bound to the accepted snapshot, reject before
	// provider dispatch. A post-usage rejection alone would allow unpaid runs.
	return unavailableCanonicalFinalizedCost()
}

func unavailableCanonicalFinalizedCost() error {
	return fmt.Errorf("%w: authoritative finalized cost and supported pricing policy are unavailable", service.ErrWorkbenchCanonicalSettlement)
}

func (w workbenchLedgerWriter) PostCanonicalUsage(ctx context.Context, event corecontracts.CanonicalUsageFinalized, journal platformledger.Journal) (service.WorkbenchLedgerPostResult, error) {
	result, err := w.repo.PostCanonicalUsage(ctx, event, journal)
	if err != nil {
		return service.WorkbenchLedgerPostResult{}, err
	}
	return service.WorkbenchLedgerPostResult{JournalID: result.JournalID, Replayed: result.Replayed}, nil
}

func (s *workbenchSettlementSource) ClaimCanonicalUsage(ctx context.Context, claim service.WorkbenchCanonicalUsageClaim) (service.WorkbenchCanonicalSettlement, error) {
	var eventID, requestID, payloadSHA string
	var apiKeyID, accountID, groupID int64
	var payload []byte
	err := s.db.QueryRowContext(ctx, workbenchSelectCanonicalUsageOutboxSQL, claim.RequestID, claim.APIKeyID).
		Scan(&eventID, &requestID, &apiKeyID, &accountID, &groupID, &payload, &payloadSHA)
	if errors.Is(err, sql.ErrNoRows) {
		return service.WorkbenchCanonicalSettlement{}, service.ErrWorkbenchCanonicalSettlement
	}
	if err != nil {
		return service.WorkbenchCanonicalSettlement{}, fmt.Errorf("claim canonical usage outbox: %w", err)
	}
	var event corecontracts.CanonicalUsageFinalized
	if err := json.Unmarshal(payload, &event); err != nil {
		return service.WorkbenchCanonicalSettlement{}, fmt.Errorf("decode canonical usage event: %w", err)
	}
	if event.EventID() != eventID || event.RequestID() != claim.RequestID || event.UserID() != claim.UserID ||
		event.AccountID() != claim.AccountID || event.GroupID() != claim.GroupID || accountID != claim.AccountID ||
		groupID != claim.GroupID || apiKeyID != claim.APIKeyID {
		return service.WorkbenchCanonicalSettlement{}, service.ErrWorkbenchCanonicalSettlement
	}
	actualPayloadSHA, err := platformledger.CanonicalUsagePayloadSHA256(event)
	if err != nil || actualPayloadSHA != payloadSHA || requestID != claim.RequestID {
		return service.WorkbenchCanonicalSettlement{}, service.ErrWorkbenchCanonicalSettlement
	}
	quote, err := scanAcceptedQuote(s.db.QueryRowContext(ctx, workbenchSelectAcceptedQuoteSQL, string(claim.QuoteID)))
	if errors.Is(err, sql.ErrNoRows) {
		return service.WorkbenchCanonicalSettlement{}, service.ErrWorkbenchCanonicalSettlement
	}
	if err != nil {
		return service.WorkbenchCanonicalSettlement{}, fmt.Errorf("load settlement quote: %w", err)
	}
	if quote.SnapshotSHA256() != string(claim.QuoteSHA) || quote.Model() != event.Model() {
		return service.WorkbenchCanonicalSettlement{}, service.ErrWorkbenchCanonicalSettlement
	}
	journal, actual, err := s.canonicalUsageJournal(ctx, event, quote)
	if err != nil {
		return service.WorkbenchCanonicalSettlement{}, err
	}
	return service.WorkbenchCanonicalSettlement{Event: event, Journal: journal, ActualCost: actual}, nil
}

func (s *workbenchSettlementSource) canonicalUsageJournal(
	ctx context.Context,
	event corecontracts.CanonicalUsageFinalized,
	quote billingcontract.AcceptedQuote,
) (platformledger.Journal, *workbench.Money, error) {
	// Schema v1 supplies units but no finalized amount or pricing provenance.
	// Quote unit names/fee_policy have no enforced settlement semantics, including
	// fixed-price semantics. Authorization is a ceiling, never evidence of cost.
	// Fail before creating accounts or posting any entries, even for zero units.
	return platformledger.Journal{}, nil, unavailableCanonicalFinalizedCost()
}

var (
	_ service.WorkbenchSettlementSource = (*workbenchSettlementSource)(nil)
	_ service.WorkbenchLedgerWriter     = workbenchLedgerWriter{}
)
