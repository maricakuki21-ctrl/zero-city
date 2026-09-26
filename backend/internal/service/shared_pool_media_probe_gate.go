package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

const sharedPoolMediaProbeDefaultValidity = 6 * time.Hour

var ErrSharedPoolMediaProbeConflict = errors.New("shared pool media probe conflicts with endpoint state")

// SharedPoolMediaEndpointProbeResult is endpoint-specific evidence. It cannot
// represent chat/full-check output, which prevents ordinary text probes from
// projecting image/video gates to passed.
type SharedPoolMediaEndpointProbeResult struct {
	PoolID                int64
	AccountID             int64
	ModelName             string
	EndpointType          string
	OperationID           string
	PoolConfigVersion     int64
	AccountConfigVersion  int64
	ProbePlanVersion      int64
	EndpointConfigVersion int64
	Success               bool
	OutputObserved        bool
	AsyncTerminalObserved bool
	UpstreamHTTPStatus    *int
	ErrorType             string
	ErrorMessage          string
	CheckedAt             time.Time
	ExpiresAt             time.Time
	ResultStatus          string
}

type sharedPoolMediaEndpointProbeRepository interface {
	ApplySharedPoolMediaEndpointProbeResultTx(ctx context.Context, input SharedPoolMediaEndpointProbeResult) error
	ExpireSharedPoolMediaEndpointGates(ctx context.Context, now time.Time, limit int) (int, error)
}

func NormalizeSharedPoolMediaEndpointProbeResult(input SharedPoolMediaEndpointProbeResult) (SharedPoolMediaEndpointProbeResult, error) {
	input.ModelName = strings.TrimSpace(input.ModelName)
	input.EndpointType = normalizeSharedPoolEndpoint(input.EndpointType)
	input.OperationID = strings.TrimSpace(input.OperationID)
	input.ErrorType = strings.TrimSpace(input.ErrorType)
	input.ErrorMessage = strings.TrimSpace(input.ErrorMessage)
	if input.PoolID <= 0 || input.AccountID < 0 || input.ModelName == "" || input.OperationID == "" ||
		len(input.OperationID) > 160 || input.PoolConfigVersion <= 0 ||
		input.ProbePlanVersion <= 0 || input.EndpointConfigVersion <= 0 ||
		(input.AccountID == 0 && input.AccountConfigVersion != 0) ||
		(input.AccountID > 0 && input.AccountConfigVersion <= 0) {
		return input, errors.New("shared pool media probe identity is invalid")
	}
	switch input.EndpointType {
	case SharedPoolEndpointImageGeneration, SharedPoolEndpointImageEdit:
		input.ResultStatus = "failed"
		if input.Success && input.OutputObserved {
			input.ResultStatus = "passed"
		}
	case SharedPoolEndpointVideo:
		input.ResultStatus = "failed"
		if input.Success && input.OutputObserved && input.AsyncTerminalObserved {
			input.ResultStatus = "passed"
		}
	default:
		return input, errors.New("only endpoint-specific image/video probes may update media gates")
	}
	if input.UpstreamHTTPStatus != nil && (*input.UpstreamHTTPStatus < 100 || *input.UpstreamHTTPStatus > 599) {
		return input, errors.New("shared pool media probe HTTP status is invalid")
	}
	if input.CheckedAt.IsZero() {
		input.CheckedAt = time.Now().UTC()
	} else {
		input.CheckedAt = input.CheckedAt.UTC()
	}
	if input.ExpiresAt.IsZero() {
		input.ExpiresAt = input.CheckedAt.Add(sharedPoolMediaProbeDefaultValidity)
	} else {
		input.ExpiresAt = input.ExpiresAt.UTC()
	}
	if !input.ExpiresAt.After(input.CheckedAt) || input.ExpiresAt.After(input.CheckedAt.Add(24*time.Hour)) {
		return input, errors.New("shared pool media probe expiry is invalid")
	}
	if input.ResultStatus == "failed" && input.ErrorType == "" {
		if input.Success {
			input.ErrorType = "media_probe_evidence_incomplete"
		} else {
			input.ErrorType = "media_probe_failed"
		}
	}
	if len(input.ErrorType) > 80 {
		input.ErrorType = input.ErrorType[:80]
	}
	if len(input.ErrorMessage) > 500 {
		input.ErrorMessage = input.ErrorMessage[:500]
	}
	return input, nil
}

func (s *BizDecipherService) ApplySharedPoolMediaEndpointProbeResult(ctx context.Context, input SharedPoolMediaEndpointProbeResult) error {
	if s == nil || s.repo == nil {
		return errors.New("shared pool media probe service is unavailable")
	}
	repo, ok := s.repo.(sharedPoolMediaEndpointProbeRepository)
	if !ok || repo == nil {
		return errors.New("shared pool media probe repository is unavailable")
	}
	normalized, err := NormalizeSharedPoolMediaEndpointProbeResult(input)
	if err != nil {
		return err
	}
	return repo.ApplySharedPoolMediaEndpointProbeResultTx(ctx, normalized)
}

func (s *BizDecipherService) ExpireSharedPoolMediaEndpointGates(ctx context.Context, now time.Time, limit int) (int, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("shared pool media probe service is unavailable")
	}
	repo, ok := s.repo.(sharedPoolMediaEndpointProbeRepository)
	if !ok || repo == nil {
		return 0, errors.New("shared pool media probe repository is unavailable")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return repo.ExpireSharedPoolMediaEndpointGates(ctx, now.UTC(), limit)
}
