package ledger

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
)

var (
	ErrInvalidJournal      = errors.New("invalid ledger journal")
	ErrUnbalancedJournal   = errors.New("ledger journal is not balanced")
	ErrIdempotencyConflict = errors.New("ledger idempotency payload conflict")
	ErrInsufficientFunds   = errors.New("ledger account has insufficient available balance")
)

type AccountPurpose string

const (
	PurposeUserBalance     AccountPurpose = "user_balance"
	PurposeOwnerEarnings   AccountPurpose = "owner_earnings"
	PurposePlatformFee     AccountPurpose = "platform_fee"
	PurposeProcessorFee    AccountPurpose = "processor_fee"
	PurposeRoundingResidue AccountPurpose = "rounding_residue"
	PurposeClearing        AccountPurpose = "clearing"
)

type Account struct {
	ID      string
	OwnerID string
	Asset   billingcontract.Asset
	Purpose AccountPurpose
}

func NewAccount(account Account) (Account, error) {
	if strings.TrimSpace(account.ID) == "" || strings.TrimSpace(account.OwnerID) == "" || !account.Purpose.valid() {
		return Account{}, ErrInvalidJournal
	}
	switch account.Asset {
	case billingcontract.AssetBalance, billingcontract.AssetCredits:
		return account, nil
	default:
		return Account{}, ErrInvalidJournal
	}
}

func (p AccountPurpose) valid() bool {
	switch p {
	case PurposeUserBalance, PurposeOwnerEarnings, PurposePlatformFee, PurposeProcessorFee, PurposeRoundingResidue, PurposeClearing:
		return true
	default:
		return false
	}
}

type Entry struct {
	AccountID string
	Amount    billingcontract.Decimal
}

type Journal struct {
	ID             string
	EventID        string
	IdempotencyKey string
	PayloadSHA256  string
	Kind           string
	ReversalOf     string
	PostedAt       time.Time
	Entries        []Entry
}

func NewJournal(input Journal) (Journal, error) {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.EventID) == "" ||
		strings.TrimSpace(input.IdempotencyKey) == "" || !validSHA256(input.PayloadSHA256) ||
		strings.TrimSpace(input.Kind) == "" || input.PostedAt.IsZero() || len(input.Entries) < 2 {
		return Journal{}, ErrInvalidJournal
	}
	zero, err := billingcontract.ParseDecimal("0")
	if err != nil {
		return Journal{}, fmt.Errorf("construct zero decimal: %w", err)
	}
	sum := zero
	entries := make([]Entry, len(input.Entries))
	for i, entry := range input.Entries {
		if strings.TrimSpace(entry.AccountID) == "" || entry.Amount.Equal(zero) {
			return Journal{}, ErrInvalidJournal
		}
		sum = sum.Add(entry.Amount)
		entries[i] = entry
	}
	if !sum.Equal(zero) {
		return Journal{}, ErrUnbalancedJournal
	}
	input.Entries = entries
	return input, nil
}

func validSHA256(raw string) bool {
	if len(raw) != 64 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

type ReversalRequest struct {
	JournalID      string
	EventID        string
	IdempotencyKey string
	PayloadSHA256  string
	PostedAt       time.Time
}

func Reverse(original Journal, request ReversalRequest) (Journal, error) {
	entries := make([]Entry, len(original.Entries))
	zero, err := billingcontract.ParseDecimal("0")
	if err != nil {
		return Journal{}, fmt.Errorf("construct zero decimal: %w", err)
	}
	for i, entry := range original.Entries {
		entries[i] = Entry{AccountID: entry.AccountID, Amount: zero.Sub(entry.Amount)}
	}
	return NewJournal(Journal{
		ID: request.JournalID, EventID: request.EventID, IdempotencyKey: request.IdempotencyKey,
		PayloadSHA256: request.PayloadSHA256, Kind: "reversal", ReversalOf: original.ID,
		PostedAt: request.PostedAt, Entries: entries,
	})
}

type Projection struct {
	AccountID string
	Balance   billingcontract.Decimal
	Reserved  billingcontract.Decimal
}

type GrossSplit struct {
	Gross           billingcontract.Decimal
	OwnerNet        billingcontract.Decimal
	PlatformFee     billingcontract.Decimal
	ProcessorFee    billingcontract.Decimal
	RoundingResidue billingcontract.Decimal
}

func NewGrossSplit(split GrossSplit) (GrossSplit, error) {
	components := split.OwnerNet.Add(split.PlatformFee).Add(split.ProcessorFee).Add(split.RoundingResidue)
	if !components.Equal(split.Gross) {
		return GrossSplit{}, ErrUnbalancedJournal
	}
	return split, nil
}

func Reserve(balance, reserved, amount billingcontract.Decimal) (billingcontract.Decimal, error) {
	zero, err := billingcontract.ParseDecimal("0")
	if err != nil {
		return billingcontract.Decimal{}, fmt.Errorf("construct zero decimal: %w", err)
	}
	if amount.IsNegative() || amount.Equal(zero) {
		return billingcontract.Decimal{}, billingcontract.ErrNegativeAmount
	}
	if balance.Sub(reserved).LessThan(amount) {
		return reserved, ErrInsufficientFunds
	}
	return reserved.Add(amount), nil
}

func Rebuild(entries []Entry) (map[string]billingcontract.Decimal, error) {
	balances := make(map[string]billingcontract.Decimal)
	for _, entry := range entries {
		if strings.TrimSpace(entry.AccountID) == "" {
			return nil, ErrInvalidJournal
		}
		balances[entry.AccountID] = balances[entry.AccountID].Add(entry.Amount)
	}
	return balances, nil
}
