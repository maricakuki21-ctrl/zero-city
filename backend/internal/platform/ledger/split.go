package ledger

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
)

type GrossSplitAccounts struct {
	UserDebit      string
	OwnerEarnings  string
	PlatformFee    string
	ProcessorFee   string
	RoundingResidue string
}

func (s GrossSplit) PostingEntries(accounts GrossSplitAccounts) ([]Entry, error) {
	validated, err := NewGrossSplit(s)
	if err != nil {
		return nil, err
	}
	zero, err := billingcontract.ParseDecimal("0")
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, 5)
	seen := make(map[string]struct{}, 5)
	appendEntry := func(accountID string, amount billingcontract.Decimal) error {
		if amount.Equal(zero) {
			return nil
		}
		accountID = strings.TrimSpace(accountID)
		if accountID == "" {
			return ErrInvalidJournal
		}
		if _, exists := seen[accountID]; exists {
			return ErrInvalidJournal
		}
		seen[accountID] = struct{}{}
		entries = append(entries, Entry{AccountID: accountID, Amount: amount})
		return nil
	}
	if err := appendEntry(accounts.UserDebit, zero.Sub(validated.Gross)); err != nil {
		return nil, err
	}
	for _, component := range []struct {
		accountID string
		amount    billingcontract.Decimal
	}{
		{accountID: accounts.OwnerEarnings, amount: validated.OwnerNet},
		{accountID: accounts.PlatformFee, amount: validated.PlatformFee},
		{accountID: accounts.ProcessorFee, amount: validated.ProcessorFee},
		{accountID: accounts.RoundingResidue, amount: validated.RoundingResidue},
	} {
		if err := appendEntry(component.accountID, component.amount); err != nil {
			return nil, err
		}
	}
	return entries, nil
}
