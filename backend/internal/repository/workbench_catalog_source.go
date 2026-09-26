package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const workbenchSelectAcceptedQuoteSQL = `SELECT quote_id, price_version_id, model, protocol, asset, canonical_snapshot,
	snapshot_sha256, maximum_authorized_cost::text, accepted_at, expires_at, source_epoch
	FROM bizdecipher_accepted_price_quotes WHERE quote_id = $1`

const workbenchListAcceptedQuotesSQL = `SELECT quote_id, price_version_id, model, protocol, asset, canonical_snapshot,
	snapshot_sha256, maximum_authorized_cost::text, accepted_at, expires_at, source_epoch
	FROM bizdecipher_accepted_price_quotes WHERE expires_at > NOW() ORDER BY accepted_at DESC`

type workbenchUserLookup interface {
	GetByID(ctx context.Context, id int64) (*service.User, error)
}

type workbenchAPIKeyLookup interface {
	ListByUserID(ctx context.Context, userID int64, params pagination.PaginationParams, filters service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error)
}

type workbenchCatalogSource struct {
	db    *sql.DB
	users workbenchUserLookup
	keys  workbenchAPIKeyLookup
}

func ProvideWorkbenchCatalogSource(db *sql.DB, users service.UserRepository, keys service.APIKeyRepository) service.WorkbenchCatalogSource {
	return &workbenchCatalogSource{db: db, users: users, keys: keys}
}

func (s *workbenchCatalogSource) ListWorkbenchCapabilities(ctx context.Context, identity workbench.Identity) ([]workbench.Capability, error) {
	if _, _, err := s.activeActor(ctx, identity); err != nil {
		return nil, err
	}
	quotes, err := s.listAcceptedQuotes(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(quotes))
	capabilities := make([]workbench.Capability, 0, len(quotes))
	for _, quote := range quotes {
		capability, capErr := workbenchCapabilityFromQuote(quote)
		if capErr != nil {
			return nil, capErr
		}
		key := string(capability.ID) + "|" + capability.Version
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		capabilities = append(capabilities, capability)
	}
	return capabilities, nil
}

func (s *workbenchCatalogSource) ResolveWorkbenchCatalog(
	ctx context.Context,
	identity workbench.Identity,
	_ workbench.CapabilityID,
	quoteID workbench.QuoteID,
) (service.WorkbenchCatalogRecord, error) {
	user, apiKey, err := s.activeActor(ctx, identity)
	if err != nil {
		return service.WorkbenchCatalogRecord{}, err
	}
	quote, err := s.loadAcceptedQuote(ctx, string(quoteID))
	if err != nil {
		return service.WorkbenchCatalogRecord{}, err
	}
	if err := quote.ValidAt(time.Now().UTC()); err != nil {
		return service.WorkbenchCatalogRecord{}, service.ErrWorkbenchCatalogUnavailable
	}
	capability, err := workbenchCapabilityFromQuote(quote)
	if err != nil {
		return service.WorkbenchCatalogRecord{}, err
	}
	accepted, err := workbenchAcceptedQuote(quote, *apiKey.GroupID)
	if err != nil {
		return service.WorkbenchCatalogRecord{}, err
	}
	return service.WorkbenchCatalogRecord{Capability: capability, Quote: accepted, User: user, APIKey: apiKey}, nil
}

func (s *workbenchCatalogSource) activeActor(ctx context.Context, identity workbench.Identity) (*service.User, *service.APIKey, error) {
	if err := identity.Validate(); err != nil {
		return nil, nil, err
	}
	user, err := s.users.GetByID(ctx, int64(identity.ActorID))
	if err != nil || user == nil || !user.IsActive() {
		return nil, nil, service.ErrWorkbenchCatalogUnavailable
	}
	keys, _, err := s.keys.ListByUserID(ctx, user.ID, pagination.PaginationParams{Page: 1, PageSize: 100}, service.APIKeyListFilters{Status: service.StatusAPIKeyActive})
	if err != nil {
		return nil, nil, fmt.Errorf("list workbench api keys: %w", err)
	}
	for i := range keys {
		key := keys[i]
		if key.IsActive() && key.GroupID != nil && *key.GroupID > 0 {
			key.User = user
			return user, &key, nil
		}
	}
	return nil, nil, service.ErrWorkbenchCatalogUnavailable
}

func (s *workbenchCatalogSource) loadAcceptedQuote(ctx context.Context, quoteID string) (billingcontract.AcceptedQuote, error) {
	quote, err := scanAcceptedQuote(s.db.QueryRowContext(ctx, workbenchSelectAcceptedQuoteSQL, quoteID))
	if errors.Is(err, sql.ErrNoRows) {
		return billingcontract.AcceptedQuote{}, service.ErrWorkbenchCatalogUnavailable
	}
	if err != nil {
		return billingcontract.AcceptedQuote{}, fmt.Errorf("load accepted quote: %w", err)
	}
	return quote, nil
}

func (s *workbenchCatalogSource) listAcceptedQuotes(ctx context.Context) ([]billingcontract.AcceptedQuote, error) {
	rows, err := s.db.QueryContext(ctx, workbenchListAcceptedQuotesSQL)
	if err != nil {
		return nil, fmt.Errorf("list accepted quotes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	quotes := make([]billingcontract.AcceptedQuote, 0)
	for rows.Next() {
		quote, scanErr := scanAcceptedQuote(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		quotes = append(quotes, quote)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accepted quotes: %w", err)
	}
	return quotes, nil
}

func scanAcceptedQuote(row scanner) (billingcontract.AcceptedQuote, error) {
	var quoteID, priceVersionID, model, protocol, asset, snapshotSHA, maxCost string
	var snapshot []byte
	var acceptedAt, expiresAt time.Time
	var sourceEpoch int64
	if err := row.Scan(&quoteID, &priceVersionID, &model, &protocol, &asset, &snapshot, &snapshotSHA, &maxCost, &acceptedAt, &expiresAt, &sourceEpoch); err != nil {
		return billingcontract.AcceptedQuote{}, err
	}
	cost, err := billingcontract.ParseDecimal(maxCost)
	if err != nil {
		return billingcontract.AcceptedQuote{}, err
	}
	return billingcontract.NewAcceptedQuote(billingcontract.AcceptedQuoteInput{
		QuoteID: quoteID, PriceVersionID: priceVersionID, Model: model, Protocol: billingcontract.Protocol(protocol),
		Asset: billingcontract.Asset(asset), CanonicalSnapshotJSON: snapshot, SnapshotSHA256: snapshotSHA,
		MaximumAuthorizedCost: cost, AcceptedAt: acceptedAt, ExpiresAt: expiresAt, SourceEpoch: uint64(sourceEpoch),
	})
}

func workbenchCapabilityFromQuote(quote billingcontract.AcceptedQuote) (workbench.Capability, error) {
	snapshot, err := billingcontract.ParseCanonicalQuoteSnapshot(quote.CanonicalSnapshotJSON())
	if err != nil {
		return workbench.Capability{}, service.ErrWorkbenchCatalogUnavailable
	}
	model := quote.Model()
	version := snapshot.Version
	digest := sha256.Sum256([]byte(model + "\n" + version))
	amount, err := workbench.NewDecimal(quote.MaximumAuthorizedCost().String())
	if err != nil {
		return workbench.Capability{}, err
	}
	return workbench.Capability{
		ID: workbench.CapabilityID("cap_" + workbenchCapabilitySlug(model)), Version: version,
		Digest: workbench.Digest(hex.EncodeToString(digest[:])), CanonicalModelID: model, CanonicalModelVersion: version,
		Title: model, Summary: model, Estimate: &workbench.Money{Currency: "USD", Amount: amount},
		AcceptedQuoteID: workbench.QuoteID(quote.QuoteID()), AcceptedQuoteSHA: workbench.Digest(quote.SnapshotSHA256()),
		Protocol: string(quote.Protocol()),
	}, nil
}

func workbenchAcceptedQuote(quote billingcontract.AcceptedQuote, groupID int64) (service.WorkbenchAcceptedQuote, error) {
	amount, err := workbench.NewDecimal(quote.MaximumAuthorizedCost().String())
	if err != nil {
		return service.WorkbenchAcceptedQuote{}, err
	}
	snapshot, err := billingcontract.ParseCanonicalQuoteSnapshot(quote.CanonicalSnapshotJSON())
	if err != nil {
		return service.WorkbenchAcceptedQuote{}, service.ErrWorkbenchCatalogUnavailable
	}
	return service.WorkbenchAcceptedQuote{
		ID: workbench.QuoteID(quote.QuoteID()), SHA256: workbench.Digest(quote.SnapshotSHA256()),
		ModelID: quote.Model(), ModelVersion: snapshot.Version, GroupID: groupID, ExpiresAt: quote.ExpiresAt(),
		Estimate: &workbench.Money{Currency: "USD", Amount: amount},
	}, nil
}

func workbenchCapabilitySlug(model string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(model) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	slug := b.String()
	if slug == "" {
		return "model"
	}
	return slug
}

var _ service.WorkbenchCatalogSource = (*workbenchCatalogSource)(nil)
