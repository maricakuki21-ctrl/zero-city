package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SharedPoolUsageReviewActionRelease      = "release"
	SharedPoolUsageReviewActionCaptureHold  = "capture_hold"
	SharedPoolUsageReviewActionSettleAmount = "settle_amount"
)

var (
	ErrSharedPoolUsageReviewNotFound = infraerrors.NotFound(
		"SHARED_POOL_USAGE_REVIEW_NOT_FOUND", "shared pool usage review not found")
	ErrSharedPoolUsageReviewConflict = infraerrors.Conflict(
		"SHARED_POOL_USAGE_REVIEW_CONFLICT", "shared pool usage review conflicts with an existing resolution")
	ErrSharedPoolUsageReviewFinalized = infraerrors.Conflict(
		"SHARED_POOL_USAGE_REVIEW_FINALIZED", "shared pool usage review is already finalized")
)

type SharedPoolUsageReview struct {
	ReservationID       int64      `json:"reservation_id"`
	RequestID           string     `json:"request_id"`
	AccessKeyID         int64      `json:"-"`
	PoolID              int64      `json:"pool_id"`
	PoolName            string     `json:"pool_name"`
	AccountID           int64      `json:"-"`
	UserID              int64      `json:"user_id"`
	UserLabel           string     `json:"user_label"`
	Model               string     `json:"model"`
	EndpointType        string     `json:"endpoint_type"`
	PricingSource       string     `json:"pricing_source"`
	HoldAmount          float64    `json:"hold_amount"`
	ReportedAmount      float64    `json:"reported_amount"`
	SettledAmount       float64    `json:"settled_amount"`
	ReservationStatus   string     `json:"reservation_status"`
	TriggerReason       string     `json:"trigger_reason"`
	ReservedAt          time.Time  `json:"reserved_at"`
	ForwardStartedAt    *time.Time `json:"forward_started_at,omitempty"`
	ExpiresAt           time.Time  `json:"expires_at"`
	FinalizedAt         *time.Time `json:"finalized_at,omitempty"`
	ResolutionID        int64      `json:"resolution_id,omitempty"`
	ResolutionAdminID   int64      `json:"resolution_admin_id,omitempty"`
	ResolutionAction    string     `json:"resolution_action,omitempty"`
	ResolutionAmount    float64    `json:"resolution_amount,omitempty"`
	ResolutionNote      string     `json:"resolution_note,omitempty"`
	ResolutionOperation string     `json:"resolution_operation_id,omitempty"`
	ResolvedAt          *time.Time `json:"resolved_at,omitempty"`
}

type SharedPoolUsageReviewPage struct {
	Items        []SharedPoolUsageReview            `json:"items"`
	NextBeforeID int64                              `json:"next_before_id,omitempty"`
	HasMore      bool                               `json:"has_more"`
	Summary      *SharedPoolUsageReviewQueueSummary `json:"summary,omitempty"`
}

type SharedPoolUsageReviewGroupSummary struct {
	PoolID         int64      `json:"pool_id"`
	PoolName       string     `json:"pool_name"`
	Model          string     `json:"model"`
	TriggerReason  string     `json:"trigger_reason"`
	PendingCount   int64      `json:"pending_count"`
	PendingHold    float64    `json:"pending_hold"`
	OldestReserved *time.Time `json:"oldest_reserved_at,omitempty"`
}

type SharedPoolUsageReviewQueueSummary struct {
	PendingCount   int64                               `json:"pending_count"`
	PendingHold    float64                             `json:"pending_hold"`
	OldestReserved *time.Time                          `json:"oldest_reserved_at,omitempty"`
	Groups         []SharedPoolUsageReviewGroupSummary `json:"groups"`
}

type ResolveSharedPoolUsageReviewInput struct {
	ReservationID int64
	AdminUserID   int64
	Action        string
	Amount        *float64
	Note          string
	OperationID   string
}

type PreparedSharedPoolUsageReviewResolution struct {
	Review *SharedPoolUsageReview `json:"review"`
	Usage  *SharedPoolUsageInput  `json:"-"`
	Replay bool                   `json:"replay"`
}

type BatchResolveSharedPoolUsageReviewsInput struct {
	ReservationIDs []int64
	AdminUserID    int64
	Action         string
	Note           string
	OperationID    string
}

type BatchResolveSharedPoolUsageReviewItem struct {
	ReservationID int64                  `json:"reservation_id"`
	Success       bool                   `json:"success"`
	Review        *SharedPoolUsageReview `json:"review,omitempty"`
	Replay        bool                   `json:"replay,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

type BatchResolveSharedPoolUsageReviewsResult struct {
	Items     []BatchResolveSharedPoolUsageReviewItem `json:"items"`
	Succeeded int                                     `json:"succeeded"`
	Failed    int                                     `json:"failed"`
}

type sharedPoolUsageReviewRepository interface {
	ListSharedPoolUsageReviews(ctx context.Context, state string, beforeID int64, limit int) (*SharedPoolUsageReviewPage, error)
	PrepareSharedPoolUsageReviewResolutionTx(ctx context.Context, input ResolveSharedPoolUsageReviewInput) (*PreparedSharedPoolUsageReviewResolution, error)
}

func (s *BizDecipherService) AdminListSharedPoolUsageReviews(
	ctx context.Context,
	state string,
	beforeID int64,
	limit int,
) (*SharedPoolUsageReviewPage, error) {
	repo, ok := sharedPoolUsageReviewRepositoryFromService(s)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("SHARED_POOL_USAGE_REVIEW_UNAVAILABLE", "shared pool usage review is temporarily unavailable")
	}
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "" {
		state = "pending"
	}
	switch state {
	case "pending", "processing", "resolved", "all":
	default:
		return nil, invalidSharedPoolUsageReview("invalid shared pool usage review state")
	}
	if beforeID < 0 {
		return nil, invalidSharedPoolUsageReview("invalid shared pool usage review cursor")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return repo.ListSharedPoolUsageReviews(ctx, state, beforeID, limit)
}

func (s *BizDecipherService) AdminResolveSharedPoolUsageReview(
	ctx context.Context,
	input ResolveSharedPoolUsageReviewInput,
) (*PreparedSharedPoolUsageReviewResolution, error) {
	repo, ok := sharedPoolUsageReviewRepositoryFromService(s)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("SHARED_POOL_USAGE_REVIEW_UNAVAILABLE", "shared pool usage review is temporarily unavailable")
	}
	if input.ReservationID <= 0 || input.AdminUserID <= 0 {
		return nil, invalidSharedPoolUsageReview("invalid shared pool usage review identity")
	}
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	switch input.Action {
	case SharedPoolUsageReviewActionRelease:
		if input.Amount != nil && *input.Amount != 0 {
			return nil, invalidSharedPoolUsageReview("this shared pool usage review action does not accept an amount")
		}
		input.Amount = nil
	case SharedPoolUsageReviewActionCaptureHold, SharedPoolUsageReviewActionSettleAmount:
		return nil, invalidSharedPoolUsageReview(fmt.Sprintf("%s is retired; shared pool review only supports release", input.Action))
	default:
		return nil, invalidSharedPoolUsageReview("invalid shared pool usage review action")
	}
	input.Note = strings.TrimSpace(input.Note)
	if input.Note == "" {
		return nil, invalidSharedPoolUsageReview("a review note is required")
	}
	if len([]rune(input.Note)) > 500 {
		return nil, invalidSharedPoolUsageReview("the review note is too long")
	}
	input.OperationID = strings.TrimSpace(input.OperationID)
	if input.OperationID == "" || len([]rune(input.OperationID)) > 160 {
		return nil, invalidSharedPoolUsageReview("a valid idempotency operation id is required")
	}

	prepared, err := repo.PrepareSharedPoolUsageReviewResolutionTx(ctx, input)
	if err != nil {
		return nil, err
	}
	if prepared == nil || prepared.Review == nil {
		return nil, infraerrors.InternalServer("SHARED_POOL_USAGE_REVIEW_EMPTY_RESULT", "shared pool usage review returned no result")
	}
	return prepared, nil
}

func (s *BizDecipherService) AdminBatchResolveSharedPoolUsageReviews(
	ctx context.Context,
	input BatchResolveSharedPoolUsageReviewsInput,
) (*BatchResolveSharedPoolUsageReviewsResult, error) {
	if input.AdminUserID <= 0 {
		return nil, invalidSharedPoolUsageReview("invalid shared pool usage review administrator")
	}
	input.Action = strings.ToLower(strings.TrimSpace(input.Action))
	if input.Action != SharedPoolUsageReviewActionRelease {
		return nil, invalidSharedPoolUsageReview(fmt.Sprintf("%s is retired; batch shared pool review only supports release", input.Action))
	}
	input.Note = strings.TrimSpace(input.Note)
	if input.Note == "" || len([]rune(input.Note)) > 500 {
		return nil, invalidSharedPoolUsageReview("a review note between 1 and 500 characters is required")
	}
	input.OperationID = strings.TrimSpace(input.OperationID)
	if input.OperationID == "" || len([]rune(input.OperationID)) > 100 {
		return nil, invalidSharedPoolUsageReview("a valid batch idempotency operation id is required")
	}
	if len(input.ReservationIDs) == 0 || len(input.ReservationIDs) > SharedPoolUsageReviewBatchLimit {
		return nil, invalidSharedPoolUsageReview(fmt.Sprintf("batch review requires between 1 and %d reservations", SharedPoolUsageReviewBatchLimit))
	}

	uniqueIDs := make([]int64, 0, len(input.ReservationIDs))
	seen := make(map[int64]struct{}, len(input.ReservationIDs))
	for _, reservationID := range input.ReservationIDs {
		if reservationID <= 0 {
			return nil, invalidSharedPoolUsageReview("batch review contains an invalid reservation id")
		}
		if _, exists := seen[reservationID]; exists {
			continue
		}
		seen[reservationID] = struct{}{}
		uniqueIDs = append(uniqueIDs, reservationID)
	}

	result := &BatchResolveSharedPoolUsageReviewsResult{
		Items: make([]BatchResolveSharedPoolUsageReviewItem, 0, len(uniqueIDs)),
	}
	for _, reservationID := range uniqueIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		prepared, err := s.AdminResolveSharedPoolUsageReview(ctx, ResolveSharedPoolUsageReviewInput{
			ReservationID: reservationID,
			AdminUserID:   input.AdminUserID,
			Action:        input.Action,
			Note:          input.Note,
			OperationID:   fmt.Sprintf("%s:%d", input.OperationID, reservationID),
		})
		item := BatchResolveSharedPoolUsageReviewItem{ReservationID: reservationID}
		if err != nil {
			item.Error = err.Error()
			result.Failed++
		} else {
			item.Success = true
			item.Review = prepared.Review
			item.Replay = prepared.Replay
			result.Succeeded++
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func sharedPoolUsageReviewRepositoryFromService(s *BizDecipherService) (sharedPoolUsageReviewRepository, bool) {
	if s == nil || s.repo == nil {
		return nil, false
	}
	repo, ok := s.repo.(sharedPoolUsageReviewRepository)
	return repo, ok && repo != nil
}

func invalidSharedPoolUsageReview(message string) error {
	return infraerrors.BadRequest("SHARED_POOL_USAGE_REVIEW_INVALID", message)
}
