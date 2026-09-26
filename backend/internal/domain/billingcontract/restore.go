package billingcontract

import (
	"strings"
	"time"
)

type RestoreHoldInput struct {
	ReservationID       string
	BusinessEventID     string
	RequestID           string
	QuoteSHA256         string
	Amount              Decimal
	Asset               Asset
	OriginalExpiresAt   time.Time
	SourceEpoch         uint64
	AcceptedMediaTaskID string
	State               HoldState
	LeaseOwner          string
	LeaseEpoch          uint64
	Version             uint64
	UpdatedAt           time.Time
}

func RestoreHold(input RestoreHoldInput) (Hold, error) {
	if strings.TrimSpace(input.ReservationID) == "" || strings.TrimSpace(input.BusinessEventID) == "" ||
		strings.TrimSpace(input.RequestID) == "" || !validSHA256Text(input.QuoteSHA256) ||
		input.Amount.IsNegative() || !input.Asset.valid() || input.OriginalExpiresAt.IsZero() ||
		input.SourceEpoch == 0 || !input.State.valid() || strings.TrimSpace(input.LeaseOwner) == "" ||
		input.LeaseEpoch == 0 || input.Version == 0 || input.UpdatedAt.IsZero() {
		return Hold{}, ErrInvalidHold
	}
	return Hold{
		reservationID: strings.TrimSpace(input.ReservationID), businessEventID: strings.TrimSpace(input.BusinessEventID),
		requestID: strings.TrimSpace(input.RequestID), quoteSHA256: strings.ToLower(input.QuoteSHA256),
		amount: input.Amount, asset: input.Asset, originalExpiresAt: input.OriginalExpiresAt,
		sourceEpoch: input.SourceEpoch, acceptedMediaTaskID: strings.TrimSpace(input.AcceptedMediaTaskID),
		state: input.State, leaseOwner: strings.TrimSpace(input.LeaseOwner), leaseEpoch: input.LeaseEpoch,
		version: input.Version, updatedAt: input.UpdatedAt,
	}, nil
}

func (s HoldState) valid() bool {
	switch s {
	case HoldReserved, HoldDispatching, HoldSettlementPending, HoldUnknown, HoldCaptured, HoldReleased:
		return true
	default:
		return false
	}
}
