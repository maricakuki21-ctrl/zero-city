package billingcontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

var (
	ErrInvalidQuote = errors.New("invalid accepted quote")
	ErrExpiredQuote = errors.New("accepted quote is expired")
)

const sha256TextLength = 64

func validSHA256Text(raw string) bool {
	if len(raw) != sha256TextLength {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

type Asset string

const (
	AssetBalance Asset = "balance"
	AssetCredits Asset = "credits"
)

type Protocol string

const (
	ProtocolChat      Protocol = "chat"
	ProtocolResponses Protocol = "responses"
	ProtocolWebSocket Protocol = "websocket"
	ProtocolImage     Protocol = "image"
	ProtocolVideo     Protocol = "video"
)

// CanonicalQuoteSnapshot is the complete immutable pricing context accepted before upstream dispatch.
type CanonicalQuoteSnapshot struct {
	Model             string             `json:"model"`
	Version           string             `json:"version"`
	Protocol          Protocol           `json:"protocol"`
	Asset             Asset              `json:"asset"`
	UnitPrices        map[string]Decimal `json:"unit_prices"`
	Multiplier        Decimal            `json:"multiplier"`
	FeePolicy         string             `json:"fee_policy"`
	MaxAuthorizedCost Decimal            `json:"max_authorized_cost"`
	ExpiresAt         time.Time          `json:"expires_at"`
	SourceEpoch       uint64             `json:"source_epoch"`
}

func ParseCanonicalQuoteSnapshot(raw []byte) (CanonicalQuoteSnapshot, error) {
	if !hasRequiredQuoteFields(raw, "model", "version", "protocol", "asset", "unit_prices", "multiplier", "fee_policy", "max_authorized_cost", "expires_at", "source_epoch") {
		return CanonicalQuoteSnapshot{}, ErrInvalidQuote
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var snapshot CanonicalQuoteSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return CanonicalQuoteSnapshot{}, ErrInvalidQuote
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return CanonicalQuoteSnapshot{}, ErrInvalidQuote
	}
	if err := snapshot.valid(); err != nil {
		return CanonicalQuoteSnapshot{}, err
	}
	return snapshot, nil
}

func (s CanonicalQuoteSnapshot) CanonicalJSON() ([]byte, error) {
	if err := s.valid(); err != nil {
		return nil, err
	}
	canonical, err := json.Marshal(s)
	if err != nil {
		return nil, ErrInvalidQuote
	}
	return canonical, nil
}

func (s CanonicalQuoteSnapshot) valid() error {
	if strings.TrimSpace(s.Model) == "" || strings.TrimSpace(s.Version) == "" ||
		!s.Protocol.valid() || !s.Asset.valid() || len(s.UnitPrices) == 0 ||
		strings.TrimSpace(s.FeePolicy) == "" || s.Multiplier.IsNegative() ||
		s.MaxAuthorizedCost.IsNegative() || s.ExpiresAt.IsZero() || s.SourceEpoch == 0 {
		return ErrInvalidQuote
	}
	for unit, price := range s.UnitPrices {
		if strings.TrimSpace(unit) == "" || price.IsNegative() {
			return ErrInvalidQuote
		}
	}
	return nil
}

type AcceptedQuoteInput struct {
	QuoteID               string
	PriceVersionID        string
	Model                 string
	Protocol              Protocol
	Asset                 Asset
	CanonicalSnapshotJSON []byte
	SnapshotSHA256        string
	MaximumAuthorizedCost Decimal
	AcceptedAt            time.Time
	ExpiresAt             time.Time
	SourceEpoch           uint64
}

type AcceptedQuote struct {
	quoteID               string
	priceVersionID        string
	model                 string
	protocol              Protocol
	asset                 Asset
	canonicalSnapshotJSON []byte
	snapshotSHA256        string
	maximumAuthorizedCost Decimal
	acceptedAt            time.Time
	expiresAt             time.Time
	sourceEpoch           uint64
}

func NewAcceptedQuote(input AcceptedQuoteInput) (AcceptedQuote, error) {
	if err := input.valid(); err != nil {
		return AcceptedQuote{}, err
	}
	canonical, err := canonicalQuoteSnapshot(input)
	if err != nil {
		return AcceptedQuote{}, err
	}
	canonicalDigest := sha256.Sum256(canonical)
	canonicalDigestText := hex.EncodeToString(canonicalDigest[:])
	rawDigest := sha256.Sum256(input.CanonicalSnapshotJSON)
	rawDigestText := hex.EncodeToString(rawDigest[:])
	if !validSHA256Text(input.SnapshotSHA256) || (!strings.EqualFold(input.SnapshotSHA256, canonicalDigestText) &&
		!strings.EqualFold(input.SnapshotSHA256, rawDigestText)) {
		return AcceptedQuote{}, ErrInvalidQuote
	}
	return AcceptedQuote{
		quoteID: strings.TrimSpace(input.QuoteID), priceVersionID: strings.TrimSpace(input.PriceVersionID),
		model: strings.TrimSpace(input.Model), protocol: input.Protocol, asset: input.Asset,
		canonicalSnapshotJSON: canonical, snapshotSHA256: canonicalDigestText,
		maximumAuthorizedCost: input.MaximumAuthorizedCost, acceptedAt: input.AcceptedAt,
		expiresAt: input.ExpiresAt, sourceEpoch: input.SourceEpoch,
	}, nil
}

func (input AcceptedQuoteInput) valid() error {
	if strings.TrimSpace(input.QuoteID) == "" || strings.TrimSpace(input.PriceVersionID) == "" ||
		strings.TrimSpace(input.Model) == "" || !input.Protocol.valid() || !input.Asset.valid() ||
		input.MaximumAuthorizedCost.IsNegative() || input.AcceptedAt.IsZero() || !input.ExpiresAt.After(input.AcceptedAt) ||
		input.SourceEpoch == 0 || !json.Valid(input.CanonicalSnapshotJSON) {
		return ErrInvalidQuote
	}
	return nil
}

func canonicalQuoteSnapshot(input AcceptedQuoteInput) ([]byte, error) {
	snapshot, err := ParseCanonicalQuoteSnapshot(input.CanonicalSnapshotJSON)
	if err == nil {
		if snapshot.Model != strings.TrimSpace(input.Model) || snapshot.Version != strings.TrimSpace(input.PriceVersionID) ||
			snapshot.Protocol != input.Protocol || snapshot.Asset != input.Asset || !snapshot.MaxAuthorizedCost.Equal(input.MaximumAuthorizedCost) ||
			!snapshot.ExpiresAt.Equal(input.ExpiresAt) || snapshot.SourceEpoch != input.SourceEpoch {
			return nil, ErrInvalidQuote
		}
		canonical, marshalErr := snapshot.CanonicalJSON()
		if marshalErr != nil {
			return nil, marshalErr
		}
		return canonical, nil
	}
	legacy, legacyErr := parseLegacyQuoteSnapshot(input.CanonicalSnapshotJSON)
	if legacyErr != nil {
		return nil, ErrInvalidQuote
	}
	if legacy.Model != strings.TrimSpace(input.Model) {
		return nil, ErrInvalidQuote
	}
	canonical, canonicalErr := CanonicalQuoteSnapshot{
		Model: strings.TrimSpace(input.Model), Version: strings.TrimSpace(input.PriceVersionID), Protocol: input.Protocol,
		Asset: input.Asset, UnitPrices: map[string]Decimal{"default": legacy.UnitPrice}, Multiplier: legacy.Multiplier,
		FeePolicy: legacy.FeePolicy, MaxAuthorizedCost: input.MaximumAuthorizedCost, ExpiresAt: input.ExpiresAt,
		SourceEpoch: input.SourceEpoch,
	}.CanonicalJSON()
	if canonicalErr != nil {
		return nil, canonicalErr
	}
	return canonical, nil
}

type legacyQuoteSnapshot struct {
	Model      string  `json:"model"`
	UnitPrice  Decimal `json:"unit_price"`
	Multiplier Decimal `json:"multiplier"`
	FeePolicy  string  `json:"fee_policy"`
}

func parseLegacyQuoteSnapshot(raw []byte) (legacyQuoteSnapshot, error) {
	if !hasRequiredQuoteFields(raw, "model", "unit_price", "multiplier", "fee_policy") {
		return legacyQuoteSnapshot{}, ErrInvalidQuote
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var snapshot legacyQuoteSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return legacyQuoteSnapshot{}, ErrInvalidQuote
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || strings.TrimSpace(snapshot.Model) == "" ||
		snapshot.UnitPrice.IsNegative() || snapshot.Multiplier.IsNegative() || strings.TrimSpace(snapshot.FeePolicy) == "" {
		return legacyQuoteSnapshot{}, ErrInvalidQuote
	}
	return snapshot, nil
}

func hasRequiredQuoteFields(raw []byte, required ...string) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || len(fields) == 0 {
		return false
	}
	for _, name := range required {
		if _, ok := fields[name]; !ok {
			return false
		}
	}
	return true
}

func (q AcceptedQuote) ValidAt(at time.Time) error {
	if q.snapshotSHA256 == "" {
		return ErrInvalidQuote
	}
	if !at.Before(q.expiresAt) {
		return ErrExpiredQuote
	}
	return nil
}

func (q AcceptedQuote) QuoteID() string                { return q.quoteID }
func (q AcceptedQuote) PriceVersionID() string         { return q.priceVersionID }
func (q AcceptedQuote) Model() string                  { return q.model }
func (q AcceptedQuote) Protocol() Protocol             { return q.protocol }
func (q AcceptedQuote) Asset() Asset                   { return q.asset }
func (q AcceptedQuote) SnapshotSHA256() string         { return q.snapshotSHA256 }
func (q AcceptedQuote) MaximumAuthorizedCost() Decimal { return q.maximumAuthorizedCost }
func (q AcceptedQuote) AcceptedAt() time.Time          { return q.acceptedAt }
func (q AcceptedQuote) ExpiresAt() time.Time           { return q.expiresAt }
func (q AcceptedQuote) SourceEpoch() uint64            { return q.sourceEpoch }
func (q AcceptedQuote) CanonicalSnapshotJSON() []byte {
	return append([]byte(nil), q.canonicalSnapshotJSON...)
}

func (a Asset) valid() bool {
	switch a {
	case AssetBalance, AssetCredits:
		return true
	default:
		return false
	}
}

func (p Protocol) valid() bool {
	switch p {
	case ProtocolChat, ProtocolResponses, ProtocolWebSocket, ProtocolImage, ProtocolVideo:
		return true
	default:
		return false
	}
}
