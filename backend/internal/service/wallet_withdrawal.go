package service

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

var ErrWithdrawalInvalid = errors.New("invalid withdrawal request")
var ErrWithdrawalConflict = errors.New("withdrawal state or idempotency conflict")
var ErrWithdrawalClosed = errors.New("manual withdrawals are not enabled")

type WithdrawalPolicy struct {
	Enabled      bool     `json:"enabled"`
	Channels     []string `json:"channels"`
	Instructions string   `json:"instructions"`
}
type WithdrawalInput struct {
	OperationID string `json:"operation_id"`
	Amount      string `json:"amount"`
	Channel     string `json:"channel"`
	Recipient   string `json:"recipient"`
}
type Withdrawal struct {
	ProcessingBy int64 `json:"processing_by"`
	ID           int64 `json:"id"`
	OwnerID      int64 `json:"owner_id"`
	WithdrawalInput
	Status    string    `json:"status"`
	Reference string    `json:"reference"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type WithdrawalAction struct {
	NoPaymentConfirmed bool   `json:"no_payment_confirmed"`
	Action             string `json:"action"`
	Reason             string `json:"reason"`
	Reference          string `json:"reference"`
}
type withdrawalRepository interface {
	WithdrawalPolicy(context.Context) (*WithdrawalPolicy, error)
	SaveWithdrawalPolicy(context.Context, int64, WithdrawalPolicy) error
	ListWithdrawals(context.Context, int64, int64) ([]Withdrawal, error)
	CreateWithdrawal(context.Context, int64, WithdrawalInput) (*Withdrawal, error)
	ActWithdrawal(context.Context, int64, int64, bool, WithdrawalAction) (*Withdrawal, error)
}

func (s *BizDecipherService) WithdrawalRepo() (withdrawalRepository, error) {
	r, ok := s.repo.(withdrawalRepository)
	if !ok {
		return nil, errors.New("withdrawal repository unavailable")
	}
	return r, nil
}

var withdrawalAmountPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,11})(\.[0-9]{1,12})?$`)
var withdrawalTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,100}$`)

// Normalize without floating point; the database uses NUMERIC(24,12).
func NormalizeWithdrawalInput(in WithdrawalInput) (WithdrawalInput, error) {
	if !withdrawalAmountPattern.MatchString(in.Amount) || !withdrawalTokenPattern.MatchString(in.OperationID) {
		return in, ErrWithdrawalInvalid
	}
	parts := strings.Split(in.Amount, ".")
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	fraction += strings.Repeat("0", 12-len(fraction))
	in.Amount = parts[0] + "." + fraction
	if strings.Trim(in.Amount, "0.") == "" {
		return in, ErrWithdrawalInvalid
	}
	in.Channel = strings.TrimSpace(in.Channel)
	in.Recipient = strings.TrimSpace(in.Recipient)
	if len(in.Channel) == 0 || len(in.Channel) > 80 || len(in.Recipient) == 0 || len(in.Recipient) > 500 {
		return in, ErrWithdrawalInvalid
	}
	return in, nil
}
func ValidateWithdrawalPolicy(p WithdrawalPolicy) error {
	if len(p.Instructions) > 2000 || len(p.Channels) > 20 || (p.Enabled && len(p.Channels) == 0) {
		return ErrWithdrawalInvalid
	}
	seen := map[string]bool{}
	for _, c := range p.Channels {
		if c != strings.TrimSpace(c) || len(c) == 0 || len(c) > 80 || seen[c] {
			return ErrWithdrawalInvalid
		}
		seen[c] = true
	}
	return nil
}
