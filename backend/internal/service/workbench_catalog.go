package service

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/workbench"
)

var (
	ErrWorkbenchCatalogUnavailable = errors.New("workbench canonical catalog is unavailable")
	ErrWorkbenchCatalogMismatch    = errors.New("workbench canonical catalog assertion mismatch")
)

type WorkbenchClock interface {
	Now() time.Time
}

type systemWorkbenchClock struct{}

func (systemWorkbenchClock) Now() time.Time { return time.Now() }

type WorkbenchAcceptedQuote struct {
	ID           workbench.QuoteID
	SHA256       workbench.Digest
	ModelID      string
	ModelVersion string
	GroupID      int64
	ExpiresAt    time.Time
	Estimate     *workbench.Money
}

type WorkbenchCatalogRecord struct {
	Capability workbench.Capability
	Quote      WorkbenchAcceptedQuote
	User       *User
	APIKey     *APIKey
}

type WorkbenchCatalogSource interface {
	ListWorkbenchCapabilities(ctx context.Context, identity workbench.Identity) ([]workbench.Capability, error)
	ResolveWorkbenchCatalog(
		ctx context.Context,
		identity workbench.Identity,
		capabilityID workbench.CapabilityID,
		quoteID workbench.QuoteID,
	) (WorkbenchCatalogRecord, error)
}

type ResolvedWorkbenchLaunch struct {
	Capability workbench.Capability
	Quote      WorkbenchAcceptedQuote
	User       *User
	APIKey     *APIKey
	GroupID    int64
}

type WorkbenchCatalog struct {
	source WorkbenchCatalogSource
	clock  WorkbenchClock
}

func NewWorkbenchCatalog(source WorkbenchCatalogSource, clock WorkbenchClock) (*WorkbenchCatalog, error) {
	if source == nil {
		return nil, ErrWorkbenchCatalogUnavailable
	}
	if clock == nil {
		clock = systemWorkbenchClock{}
	}
	return &WorkbenchCatalog{source: source, clock: clock}, nil
}

func (c *WorkbenchCatalog) Capabilities(ctx context.Context, identity workbench.Identity) ([]workbench.Capability, error) {
	if err := identity.Validate(); err != nil {
		return nil, err
	}
	capabilities, err := c.source.ListWorkbenchCapabilities(ctx, identity)
	if err != nil {
		return nil, fmt.Errorf("list workbench canonical capabilities: %w", err)
	}
	return append([]workbench.Capability{}, capabilities...), nil
}

func (c *WorkbenchCatalog) ResolveLaunch(ctx context.Context, command workbench.LaunchCommand) (ResolvedWorkbenchLaunch, error) {
	if err := validateWorkbenchLaunchCommand(command); err != nil {
		return ResolvedWorkbenchLaunch{}, err
	}
	record, err := c.source.ResolveWorkbenchCatalog(ctx, command.Identity, workbench.CapabilityID(command.CapabilityID), workbench.QuoteID(command.AcceptedQuoteID))
	if err != nil {
		return ResolvedWorkbenchLaunch{}, fmt.Errorf("resolve workbench canonical catalog: %w", err)
	}
	if err := validateWorkbenchCatalogRecord(record, command.Identity, c.clock.Now()); err != nil {
		return ResolvedWorkbenchLaunch{}, err
	}
	if !workbenchCatalogAssertionsMatch(record, command) {
		return ResolvedWorkbenchLaunch{}, ErrWorkbenchCatalogMismatch
	}
	if command.RequiredProtocolFamily == "text" && record.Capability.Protocol != "chat" && record.Capability.Protocol != "responses" {
		return ResolvedWorkbenchLaunch{}, workbench.ErrInvalidCommand
	}
	return ResolvedWorkbenchLaunch{
		Capability: record.Capability,
		Quote:      cloneWorkbenchQuote(record.Quote),
		User:       record.User,
		APIKey:     record.APIKey,
		GroupID:    record.Quote.GroupID,
	}, nil
}

func validateWorkbenchLaunchCommand(command workbench.LaunchCommand) error {
	if err := command.Identity.Validate(); err != nil {
		return err
	}
	values := []string{
		command.CapabilityID, command.CapabilityVersion, command.CapabilityDigest,
		command.CanonicalModelID, command.CanonicalModelVersion, command.AcceptedQuoteID,
		command.AcceptedQuoteSHA, command.Intent, command.IdempotencyKey,
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return fmt.Errorf("%w: launch assertions must be non-empty canonical values", workbench.ErrInvalidCommand)
		}
	}
	return nil
}

func validateWorkbenchCatalogRecord(record WorkbenchCatalogRecord, identity workbench.Identity, now time.Time) error {
	capability := record.Capability
	quote := record.Quote
	if capability.ID == "" || capability.Version == "" || !validWorkbenchDigest(capability.Digest) ||
		strings.TrimSpace(capability.CanonicalModelID) == "" || strings.TrimSpace(capability.CanonicalModelVersion) == "" ||
		quote.ID == "" || !validWorkbenchDigest(quote.SHA256) || quote.GroupID <= 0 || quote.ExpiresAt.IsZero() || !now.Before(quote.ExpiresAt) ||
		quote.ModelID != capability.CanonicalModelID || quote.ModelVersion != capability.CanonicalModelVersion {
		return ErrWorkbenchCatalogUnavailable
	}
	if record.User == nil || record.APIKey == nil || record.User.ID != int64(identity.ActorID) || record.APIKey.UserID != record.User.ID ||
		record.APIKey.GroupID == nil || *record.APIKey.GroupID != quote.GroupID || !record.User.IsActive() || !record.APIKey.IsActive() {
		return ErrWorkbenchCatalogUnavailable
	}
	return nil
}

func workbenchCatalogAssertionsMatch(record WorkbenchCatalogRecord, command workbench.LaunchCommand) bool {
	return string(record.Capability.ID) == command.CapabilityID &&
		record.Capability.Version == command.CapabilityVersion && string(record.Capability.Digest) == command.CapabilityDigest &&
		record.Capability.CanonicalModelID == command.CanonicalModelID && record.Capability.CanonicalModelVersion == command.CanonicalModelVersion &&
		string(record.Quote.ID) == command.AcceptedQuoteID && string(record.Quote.SHA256) == command.AcceptedQuoteSHA
}

func validWorkbenchDigest(value workbench.Digest) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(string(value))
	return err == nil && strings.ToLower(string(value)) == string(value)
}

func cloneWorkbenchQuote(value WorkbenchAcceptedQuote) WorkbenchAcceptedQuote {
	if value.Estimate != nil {
		estimate := *value.Estimate
		value.Estimate = &estimate
	}
	return value
}
