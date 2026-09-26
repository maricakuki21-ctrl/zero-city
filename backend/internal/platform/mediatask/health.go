package mediatask

import "errors"

var ErrPaidGenerationHealthProbe = errors.New("paid media generation cannot be used as health evidence")

type HealthEvidenceSource string

const (
	HealthEvidenceSignedMetadata HealthEvidenceSource = "signed_metadata"
	HealthEvidenceFreeCapability HealthEvidenceSource = "free_capability"
	HealthEvidenceRealExecution  HealthEvidenceSource = "real_user_execution"
)

type HealthEvidence struct {
	Source     HealthEvidenceSource
	Confidence uint8
}

func ValidateHealthEvidence(evidence HealthEvidence) error {
	switch evidence.Source {
	case HealthEvidenceSignedMetadata, HealthEvidenceFreeCapability, HealthEvidenceRealExecution:
		if evidence.Confidence <= 100 {
			return nil
		}
	}
	return ErrPaidGenerationHealthProbe
}

func PaidGenerationProbeCount() uint64 { return 0 }
