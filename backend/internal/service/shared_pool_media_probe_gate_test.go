package service

import (
	"testing"
	"time"
)

func TestNormalizeSharedPoolMediaEndpointProbeResultRequiresConfigFences(t *testing.T) {
	base := SharedPoolMediaEndpointProbeResult{
		PoolID:                7,
		ModelName:             "grok-imagine-video",
		EndpointType:          SharedPoolEndpointVideo,
		OperationID:           "probe:test:fences",
		PoolConfigVersion:     2,
		ProbePlanVersion:      3,
		EndpointConfigVersion: 4,
		Success:               true,
		OutputObserved:        true,
		AsyncTerminalObserved: true,
		CheckedAt:             time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC),
	}

	if _, err := NormalizeSharedPoolMediaEndpointProbeResult(base); err != nil {
		t.Fatalf("valid pool-level probe rejected: %v", err)
	}

	missingPoolFence := base
	missingPoolFence.PoolConfigVersion = 0
	if _, err := NormalizeSharedPoolMediaEndpointProbeResult(missingPoolFence); err == nil {
		t.Fatal("expected missing pool config fence to be rejected")
	}

	missingAccountFence := base
	missingAccountFence.AccountID = 11
	if _, err := NormalizeSharedPoolMediaEndpointProbeResult(missingAccountFence); err == nil {
		t.Fatal("expected account probe without account config fence to be rejected")
	}

	poolProbeWithAccountFence := base
	poolProbeWithAccountFence.AccountConfigVersion = 1
	if _, err := NormalizeSharedPoolMediaEndpointProbeResult(poolProbeWithAccountFence); err == nil {
		t.Fatal("expected pool-level probe carrying an account fence to be rejected")
	}
}

func TestNormalizeSharedPoolMediaEndpointProbeResultRequiresObservedTerminalVideo(t *testing.T) {
	input := SharedPoolMediaEndpointProbeResult{
		PoolID:                7,
		AccountID:             11,
		ModelName:             "grok-imagine-video",
		EndpointType:          SharedPoolEndpointVideo,
		OperationID:           "probe:test:terminal",
		PoolConfigVersion:     2,
		AccountConfigVersion:  5,
		ProbePlanVersion:      3,
		EndpointConfigVersion: 4,
		Success:               true,
		OutputObserved:        true,
		CheckedAt:             time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC),
	}

	normalized, err := NormalizeSharedPoolMediaEndpointProbeResult(input)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.ResultStatus != "failed" {
		t.Fatalf("expected incomplete async evidence to fail, got %q", normalized.ResultStatus)
	}
	if normalized.ErrorType != "media_probe_evidence_incomplete" {
		t.Fatalf("unexpected error type %q", normalized.ErrorType)
	}
}
