package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

type RequestFingerprint string

type OperationKind string

const (
	OperationCreate OperationKind = "create"
	OperationCancel OperationKind = "cancel"
	OperationSave   OperationKind = "save"
	OperationReplay OperationKind = "replay"
	OperationFork   OperationKind = "fork"
)

type OperationState string

const (
	OperationClaimed   OperationState = "claimed"
	OperationCommitted OperationState = "committed"
	OperationFailed    OperationState = "failed"
)

type OperationClaim struct {
	ID          OperationID        `json:"id"`
	OwnerID     ActorID            `json:"owner_id"`
	RunID       RunID              `json:"run_id"`
	Key         string             `json:"key"`
	Kind        OperationKind      `json:"kind"`
	Fingerprint RequestFingerprint `json:"fingerprint"`
}

type OperationResult struct {
	RunID      RunID      `json:"run_id"`
	SnapshotID SnapshotID `json:"snapshot_id,omitempty"`
	State      RunState   `json:"state"`
}

type OperationRecord struct {
	Claim  OperationClaim  `json:"claim"`
	State  OperationState  `json:"state"`
	Result OperationResult `json:"result"`
}

type IdempotencyDisposition string

const (
	IdempotencyProceed IdempotencyDisposition = "proceed"
	IdempotencyReplay  IdempotencyDisposition = "replay"
)

type IdempotencyDecision struct {
	Disposition IdempotencyDisposition
	Result      OperationResult
}

func ResolveIdempotency(existing *OperationRecord, requested OperationClaim) (IdempotencyDecision, error) {
	if existing == nil || existing.Claim.RunID != requested.RunID || existing.Claim.Key != requested.Key {
		return IdempotencyDecision{Disposition: IdempotencyProceed}, nil
	}
	if existing.Claim.OwnerID != requested.OwnerID {
		return IdempotencyDecision{}, ErrOwnershipMismatch
	}
	if existing.Claim.Kind != requested.Kind || existing.Claim.Fingerprint != requested.Fingerprint {
		return IdempotencyDecision{}, &IdempotencyConflictError{
			Key: requested.Key, Existing: existing.Claim.Fingerprint, Requested: requested.Fingerprint,
		}
	}
	return IdempotencyDecision{Disposition: IdempotencyReplay, Result: existing.Result}, nil
}

func FingerprintLaunch(command LaunchCommand) RequestFingerprint {
	return fingerprintParts(
		string(OperationCreate), command.CapabilityID, command.CapabilityVersion, command.CapabilityDigest,
		command.CanonicalModelID, command.CanonicalModelVersion, command.AcceptedQuoteID,
		command.AcceptedQuoteSHA, command.Intent,
	)
}

func FingerprintSave(command SaveCommand) RequestFingerprint {
	return fingerprintParts(string(OperationSave), string(command.RunID), command.ReplayLabel)
}

func FingerprintReplay(command ReplayCommand) RequestFingerprint {
	return fingerprintParts(string(OperationReplay), string(command.ReplayID))
}

func FingerprintFork(command ForkCommand) RequestFingerprint {
	return fingerprintParts(string(OperationFork), string(command.RunID))
}

func FingerprintCancel(command CancelCommand) RequestFingerprint {
	return fingerprintParts(string(OperationCancel), string(command.RunID), command.Reason)
}

func fingerprintParts(parts ...string) RequestFingerprint {
	var canonical strings.Builder
	for _, part := range parts {
		canonical.WriteString(strconv.Itoa(len(part)))
		canonical.WriteByte(':')
		canonical.WriteString(part)
		canonical.WriteByte('|')
	}
	digest := sha256.Sum256([]byte(canonical.String()))
	return RequestFingerprint(hex.EncodeToString(digest[:]))
}
