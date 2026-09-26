package workbench

import (
	"errors"
	"testing"
)

func TestIdempotency_SameKeyAndFingerprintReplaysTypedResult(t *testing.T) {
	t.Parallel()

	claim := OperationClaim{RunID: "wbr_one", Key: "operation-1", Kind: OperationCreate, Fingerprint: FingerprintLaunch(LaunchCommand{Intent: "hello"})}
	existing := &OperationRecord{Claim: claim, Result: OperationResult{RunID: "wbr_one", State: StateQueued}}

	decision, err := ResolveIdempotency(existing, claim)
	if err != nil {
		t.Fatalf("ResolveIdempotency() error = %v", err)
	}
	if decision.Disposition != IdempotencyReplay || decision.Result.RunID != "wbr_one" {
		t.Fatalf("ResolveIdempotency() = %#v", decision)
	}
}

func TestIdempotency_SameKeyDifferentFingerprintConflicts(t *testing.T) {
	t.Parallel()

	existingClaim := OperationClaim{RunID: "wbr_one", Key: "operation-1", Kind: OperationCreate, Fingerprint: FingerprintLaunch(LaunchCommand{Intent: "hello"})}
	requested := existingClaim
	requested.Fingerprint = FingerprintLaunch(LaunchCommand{Intent: "different"})

	_, err := ResolveIdempotency(&OperationRecord{Claim: existingClaim}, requested)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("ResolveIdempotency() error = %v, want ErrIdempotencyConflict", err)
	}
	var conflict *IdempotencyConflictError
	if !errors.As(err, &conflict) || conflict.Key != requested.Key {
		t.Fatalf("ResolveIdempotency() typed conflict = %#v", conflict)
	}
}
