package service

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
)

var ErrSharedPoolDecimalBillingUnavailable = errors.New("shared pool decimal billing is unavailable")

type decimalBillingRepository interface {
	PersistAcceptedQuoteAndHold(context.Context, billingcontract.AcceptedQuote, billingcontract.Hold) error
}

type DecimalReservationInput struct {
	Quote            billingcontract.AcceptedQuote
	ReservationID    string
	BusinessEventID  string
	RequestID        string
	Amount           billingcontract.Decimal
	AvailableBalance billingcontract.Decimal
	LeaseOwner       string
	ActiveEpoch      uint64
	CreatedAt        time.Time
}

type SharedPoolDecimalBilling struct {
	repo decimalBillingRepository
}

func NewSharedPoolDecimalBilling(repo decimalBillingRepository) *SharedPoolDecimalBilling {
	return &SharedPoolDecimalBilling{repo: repo}
}

func (s *SharedPoolDecimalBilling) ReserveBeforeDispatch(ctx context.Context, input DecimalReservationInput) (billingcontract.Hold, error) {
	if s == nil || s.repo == nil {
		return billingcontract.Hold{}, ErrSharedPoolDecimalBillingUnavailable
	}
	hold, err := billingcontract.NewHold(billingcontract.NewHoldInput{
		ReservationID: input.ReservationID, BusinessEventID: input.BusinessEventID, RequestID: input.RequestID,
		Quote: input.Quote, Amount: input.Amount, AvailableBalance: input.AvailableBalance,
		LeaseOwner: input.LeaseOwner, ActiveEpoch: input.ActiveEpoch, CreatedAt: input.CreatedAt,
	})
	if err != nil {
		return billingcontract.Hold{}, err
	}
	if err := s.repo.PersistAcceptedQuoteAndHold(ctx, input.Quote, hold); err != nil {
		return billingcontract.Hold{}, err
	}
	return hold, nil
}
