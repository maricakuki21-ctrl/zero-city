package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrNativeActivationInvalid  = infraerrors.New(http.StatusUnprocessableEntity, "native_activation_invalid", "Native pool billing prerequisites are not satisfied")
	ErrNativeActivationStale    = infraerrors.Conflict("native_activation_stale", "The pool configuration changed; refresh before activating")
	ErrNativeProbeUsesReadiness = infraerrors.Conflict("native_readiness_required", "Use the native readiness check for owner-native supply")
)

type SharedPoolNativeActivationInput struct {
	PoolID                int64
	OwnerID               int64
	OperationID           string
	ExpectedConfigVersion int64
	RequestFingerprint    string
}

type SharedPoolNativeActivation struct {
	PoolID                 int64      `json:"pool_id"`
	State                  string     `json:"state"`
	ConfigVersion          int64      `json:"config_version"`
	ActivatedConfigVersion int64      `json:"activated_config_version"`
	ActivatedAt            *time.Time `json:"activated_at,omitempty"`
	AlreadyActive          bool       `json:"already_active"`
}

type SharedPoolNativeActivationRepository interface {
	ActivateNativePoolBilling(context.Context, SharedPoolNativeActivationInput) (*SharedPoolNativeActivation, error)
}

func NewSharedPoolNativeActivationInput(poolID, ownerID int64, operationID string, expectedVersion int64) (SharedPoolNativeActivationInput, error) {
	operationID = strings.TrimSpace(operationID)
	if poolID <= 0 || ownerID <= 0 || operationID == "" || len(operationID) > 160 || expectedVersion <= 0 {
		return SharedPoolNativeActivationInput{}, ErrOwnerNativeFieldRejected
	}
	return SharedPoolNativeActivationInput{
		PoolID: poolID, OwnerID: ownerID, OperationID: operationID, ExpectedConfigVersion: expectedVersion,
		RequestFingerprint: nativeSHA(fmt.Sprintf("%d\x00%d\x00%d", poolID, ownerID, expectedVersion)),
	}, nil
}

func (s *BizDecipherService) ActivateSharedPoolNativeBilling(ctx context.Context, input SharedPoolNativeActivationInput) (*SharedPoolNativeActivation, error) {
	if s == nil || s.repo == nil {
		return nil, ErrNativeOnboardingUnavailable
	}
	repo, ok := s.repo.(SharedPoolNativeActivationRepository)
	if !ok {
		return nil, ErrNativeOnboardingUnavailable
	}
	return repo.ActivateNativePoolBilling(ctx, input)
}

func isNativeBillingActive(pool *SharedPool) bool {
	return pool != nil && pool.NativeOnboardingState == SharedPoolOnboardingBillingActive
}
