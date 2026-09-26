package billingcontract

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidHold         = errors.New("invalid hold")
	ErrInsufficientFunds   = errors.New("insufficient funds before dispatch")
	ErrStaleVersion        = errors.New("stale hold version")
	ErrStaleEpoch          = errors.New("stale authority epoch")
	ErrInvalidTransition   = errors.New("invalid hold state transition")
	ErrHoldFinalized       = errors.New("hold is already finalized")
	ErrMediaTaskIDConflict = errors.New("accepted media task id conflicts with retained id")
)

type HoldState string

const (
	HoldReserved          HoldState = "reserved"
	HoldDispatching       HoldState = "dispatching"
	HoldSettlementPending HoldState = "settlement_pending"
	HoldUnknown           HoldState = "unknown"
	HoldCaptured          HoldState = "captured"
	HoldReleased          HoldState = "released"
)

type NewHoldInput struct {
	ReservationID    string
	BusinessEventID  string
	RequestID        string
	Quote            AcceptedQuote
	Amount           Decimal
	AvailableBalance Decimal
	LeaseOwner       string
	ActiveEpoch      uint64
	CreatedAt        time.Time
}

type Hold struct {
	reservationID       string
	businessEventID     string
	requestID           string
	quoteSHA256         string
	amount              Decimal
	asset               Asset
	originalExpiresAt   time.Time
	sourceEpoch         uint64
	acceptedMediaTaskID string
	state               HoldState
	leaseOwner          string
	leaseEpoch          uint64
	version             uint64
	updatedAt           time.Time
}

func NewHold(input NewHoldInput) (Hold, error) {
	if strings.TrimSpace(input.ReservationID) == "" || strings.TrimSpace(input.BusinessEventID) == "" ||
		strings.TrimSpace(input.RequestID) == "" || strings.TrimSpace(input.LeaseOwner) == "" ||
		input.ActiveEpoch == 0 || input.CreatedAt.IsZero() {
		return Hold{}, ErrInvalidHold
	}
	if input.Amount.IsNegative() || input.AvailableBalance.IsNegative() {
		return Hold{}, ErrNegativeAmount
	}
	if err := input.Quote.ValidAt(input.CreatedAt); err != nil {
		return Hold{}, err
	}
	if input.Quote.MaximumAuthorizedCost().LessThan(input.Amount) {
		return Hold{}, ErrInvalidHold
	}
	if input.AvailableBalance.LessThan(input.Amount) {
		return Hold{}, ErrInsufficientFunds
	}
	return Hold{
		reservationID: strings.TrimSpace(input.ReservationID), businessEventID: strings.TrimSpace(input.BusinessEventID),
		requestID: strings.TrimSpace(input.RequestID), quoteSHA256: input.Quote.SnapshotSHA256(),
		amount: input.Amount, asset: input.Quote.Asset(), originalExpiresAt: input.Quote.ExpiresAt(),
		sourceEpoch: input.Quote.SourceEpoch(), state: HoldReserved, leaseOwner: strings.TrimSpace(input.LeaseOwner),
		leaseEpoch: input.ActiveEpoch, version: 1, updatedAt: input.CreatedAt,
	}, nil
}

type TransitionCommand struct {
	To              HoldState
	ExpectedVersion uint64
	ActiveEpoch     uint64
	UpdatedAt       time.Time
}

type Decision struct {
	hold    Hold
	applied bool
}

func DecideTransition(current Hold, command TransitionCommand) (Decision, error) {
	if command.ActiveEpoch != current.leaseEpoch {
		return Decision{}, ErrStaleEpoch
	}
	if current.state == command.To && current.state.terminal() {
		return Decision{hold: current}, nil
	}
	if command.ExpectedVersion != current.version {
		return Decision{}, ErrStaleVersion
	}
	if current.state.terminal() {
		return Decision{}, ErrHoldFinalized
	}
	if command.UpdatedAt.Before(current.updatedAt) || !transitionAllowed(current.state, command.To) {
		return Decision{}, ErrInvalidTransition
	}
	next := current
	next.state = command.To
	next.version++
	next.updatedAt = command.UpdatedAt
	return Decision{hold: next, applied: true}, nil
}

type MediaTaskCommand struct {
	TaskID          string
	ExpectedVersion uint64
	ActiveEpoch     uint64
	UpdatedAt       time.Time
}

func RetainAcceptedMediaTask(current Hold, command MediaTaskCommand) (Decision, error) {
	if command.ActiveEpoch != current.leaseEpoch {
		return Decision{}, ErrStaleEpoch
	}
	taskID := strings.TrimSpace(command.TaskID)
	if taskID == "" {
		return Decision{}, ErrInvalidHold
	}
	if current.acceptedMediaTaskID == taskID {
		return Decision{hold: current}, nil
	}
	if current.acceptedMediaTaskID != "" {
		return Decision{}, ErrMediaTaskIDConflict
	}
	if command.ExpectedVersion != current.version {
		return Decision{}, ErrStaleVersion
	}
	if command.UpdatedAt.Before(current.updatedAt) || current.state != HoldDispatching {
		return Decision{}, ErrInvalidTransition
	}
	next := current
	next.acceptedMediaTaskID = taskID
	next.version++
	next.updatedAt = command.UpdatedAt
	return Decision{hold: next, applied: true}, nil
}

type LeaseCommand struct {
	Owner           string
	LeaseEpoch      uint64
	ExpectedVersion uint64
	ActiveEpoch     uint64
	UpdatedAt       time.Time
}

func ReassignLease(current Hold, command LeaseCommand) (Decision, error) {
	if command.ActiveEpoch != current.leaseEpoch {
		return Decision{}, ErrStaleEpoch
	}
	if command.ExpectedVersion != current.version {
		return Decision{}, ErrStaleVersion
	}
	owner := strings.TrimSpace(command.Owner)
	if owner == "" || command.LeaseEpoch <= current.leaseEpoch || command.UpdatedAt.Before(current.updatedAt) {
		return Decision{}, ErrInvalidHold
	}
	next := current
	next.leaseOwner = owner
	next.leaseEpoch = command.LeaseEpoch
	next.version++
	next.updatedAt = command.UpdatedAt
	return Decision{hold: next, applied: true}, nil
}

func transitionAllowed(from, to HoldState) bool {
	switch from {
	case HoldReserved:
		return to == HoldDispatching || to == HoldReleased
	case HoldDispatching:
		return to == HoldSettlementPending || to == HoldUnknown || to == HoldReleased
	case HoldSettlementPending:
		return to == HoldCaptured || to == HoldUnknown
	case HoldUnknown:
		return to == HoldSettlementPending || to == HoldCaptured || to == HoldReleased
	case HoldCaptured, HoldReleased:
		return false
	default:
		return false
	}
}

func (s HoldState) terminal() bool { return s == HoldCaptured || s == HoldReleased }

func (d Decision) Hold() Hold    { return d.hold }
func (d Decision) Applied() bool { return d.applied }

func (h Hold) ReservationID() string        { return h.reservationID }
func (h Hold) BusinessEventID() string      { return h.businessEventID }
func (h Hold) RequestID() string            { return h.requestID }
func (h Hold) QuoteSHA256() string          { return h.quoteSHA256 }
func (h Hold) Amount() Decimal              { return h.amount }
func (h Hold) Asset() Asset                 { return h.asset }
func (h Hold) OriginalExpiresAt() time.Time { return h.originalExpiresAt }
func (h Hold) SourceEpoch() uint64          { return h.sourceEpoch }
func (h Hold) AcceptedMediaTaskID() string  { return h.acceptedMediaTaskID }
func (h Hold) State() HoldState             { return h.state }
func (h Hold) LeaseOwner() string           { return h.leaseOwner }
func (h Hold) LeaseEpoch() uint64           { return h.leaseEpoch }
func (h Hold) Version() uint64              { return h.version }
func (h Hold) UpdatedAt() time.Time         { return h.updatedAt }
