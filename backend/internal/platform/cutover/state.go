package cutover

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidAttemptID      = errors.New("invalid cutover attempt id")
	ErrAttemptReuse          = errors.New("replacement cutover attempt must be new")
	ErrInvalidEpoch          = errors.New("invalid authority epoch")
	ErrInvalidVersion        = errors.New("invalid cutover version")
	ErrIllegalTransition     = errors.New("illegal cutover phase transition")
	ErrStaleAuthority        = errors.New("stale cutover authority")
	ErrLegacyRuntimeFenced   = errors.New("legacy runtime is fenced")
	ErrCanonicalEffectClosed = errors.New("canonical effects are not open")
)

type Phase string

const (
	PhaseLegacyOpen              Phase = "legacy_open"
	PhaseDraining                Phase = "draining"
	PhaseFenced                  Phase = "fenced"
	PhaseImporting               Phase = "importing"
	PhaseVerified                Phase = "verified"
	PhaseCanonicalOpen           Phase = "canonical_open"
	PhaseCanonicalRecovering     Phase = "canonical_recovering"
	PhaseComplete                Phase = "complete"
	PhaseAbortedBeforeActivation Phase = "aborted_before_activation"
)

type Authority struct {
	AttemptID            string
	Phase                Phase
	Epoch                int64
	Version              int64
	FirstCanonicalEffect bool
}

type Expectation struct {
	AttemptID string
	Phase     Phase
	Epoch     int64
	Version   int64
}

type AbortResult struct {
	Failed      Authority
	Replacement Authority
}

func NewAuthority(attemptID string, epoch int64) (Authority, error) {
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return Authority{}, ErrInvalidAttemptID
	}
	if epoch <= 0 {
		return Authority{}, ErrInvalidEpoch
	}
	return Authority{AttemptID: attemptID, Phase: PhaseLegacyOpen, Epoch: epoch, Version: 1}, nil
}

func (a Authority) Expectation() Expectation {
	return Expectation{AttemptID: a.AttemptID, Phase: a.Phase, Epoch: a.Epoch, Version: a.Version}
}

func (a Authority) Transition(expected Expectation, next Phase) (Authority, error) {
	if err := a.matches(expected); err != nil {
		return Authority{}, err
	}
	if !legalTransition(a.Phase, next) {
		return Authority{}, fmt.Errorf("%w: %s -> %s", ErrIllegalTransition, a.Phase, next)
	}
	a.Phase = next
	if next == PhaseFenced {
		a.Epoch++
	}
	a.Version++
	return a, nil
}

func (a Authority) AbortAfterFence(expected Expectation, replacementAttemptID string) (AbortResult, error) {
	if err := a.matches(expected); err != nil {
		return AbortResult{}, err
	}
	if a.Phase != PhaseFenced && a.Phase != PhaseImporting && a.Phase != PhaseVerified {
		return AbortResult{}, fmt.Errorf("%w: %s -> %s", ErrIllegalTransition, a.Phase, PhaseAbortedBeforeActivation)
	}
	replacementAttemptID = strings.TrimSpace(replacementAttemptID)
	if replacementAttemptID == a.AttemptID {
		return AbortResult{}, ErrAttemptReuse
	}
	failed := a
	failed.Phase = PhaseAbortedBeforeActivation
	failed.Version++
	replacement, err := NewAuthority(replacementAttemptID, a.Epoch+1)
	if err != nil {
		return AbortResult{}, err
	}
	replacement.Version = failed.Version
	return AbortResult{Failed: failed, Replacement: replacement}, nil
}

func (a Authority) MarkFirstCanonicalEffect(expected Expectation) (Authority, error) {
	if err := a.matches(expected); err != nil {
		return Authority{}, err
	}
	if a.Phase != PhaseCanonicalOpen {
		return Authority{}, ErrCanonicalEffectClosed
	}
	if a.FirstCanonicalEffect {
		return a, nil
	}
	a.FirstCanonicalEffect = true
	a.Version++
	return a, nil
}

func (a Authority) AuthorizeLegacyRuntimeWrite(epoch int64) error {
	if epoch != a.Epoch {
		return ErrStaleAuthority
	}
	if a.Phase != PhaseLegacyOpen && a.Phase != PhaseDraining {
		return ErrLegacyRuntimeFenced
	}
	return nil
}

func (a Authority) matches(expected Expectation) error {
	if expected.AttemptID != a.AttemptID || expected.Phase != a.Phase || expected.Epoch != a.Epoch || expected.Version != a.Version {
		return ErrStaleAuthority
	}
	return nil
}

func legalTransition(from, to Phase) bool {
	switch from {
	case PhaseLegacyOpen:
		return to == PhaseDraining
	case PhaseDraining:
		return to == PhaseFenced || to == PhaseLegacyOpen
	case PhaseFenced:
		return to == PhaseImporting
	case PhaseImporting:
		return to == PhaseVerified
	case PhaseVerified:
		return to == PhaseCanonicalOpen
	case PhaseCanonicalOpen:
		return to == PhaseCanonicalRecovering || to == PhaseComplete
	case PhaseCanonicalRecovering:
		return to == PhaseCanonicalOpen || to == PhaseComplete
	case PhaseComplete, PhaseAbortedBeforeActivation:
		return false
	default:
		return false
	}
}
