package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SharedPoolMediaTaskSubmitted      = "submitted"
	SharedPoolMediaTaskProcessing     = "processing"
	SharedPoolMediaTaskSucceeded      = "succeeded"
	SharedPoolMediaTaskFailed         = "failed"
	SharedPoolMediaTaskReviewRequired = "review_required"

	sharedPoolVideoTaskTTL = 24 * time.Hour
)

var (
	ErrSharedPoolMediaTaskNotFound = errors.New("shared pool media task was not found")
	ErrSharedPoolMediaTaskConflict = errors.New("shared pool media task conflicts with existing state")
)

// SharedPoolMediaTask is a credential-free ownership and settlement record for
// an asynchronous provider task. PriceSnapshot is the immutable quote accepted
// at submission; no status request is allowed to re-price historical work.
type SharedPoolMediaTask struct {
	ID                       int64
	ReservationID            int64
	AccessKeyID              int64
	PoolID                   int64
	AccountID                int64
	UserID                   int64
	PriceVersionID           int64
	EndpointType             string
	Provider                 string
	ModelSnapshot            string
	UpstreamModelSnapshot    string
	UpstreamRequestID        string
	ReservationRequestID     string
	RequestedResolution      string
	RequestedDurationSeconds int
	Status                   string
	LastUpstreamStatus       string
	LastHTTPStatus           *int
	PricingSource            string
	PriceSnapshot            json.RawMessage
	ReservationStatus        string
	ExpiresAt                time.Time
	CompletedAt              *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type CreateSharedPoolMediaTaskInput struct {
	ReservationID            int64
	AccessKeyID              int64
	PoolID                   int64
	AccountID                int64
	UserID                   int64
	PriceVersionID           int64
	Provider                 string
	ModelSnapshot            string
	UpstreamModelSnapshot    string
	UpstreamRequestID        string
	ReservationRequestID     string
	RequestedResolution      string
	RequestedDurationSeconds int
	ExpiresAt                time.Time
}

type UpdateSharedPoolMediaTaskStateInput struct {
	TaskID             int64
	AccessKeyID        int64
	UpstreamRequestID  string
	Status             string
	LastUpstreamStatus string
	LastHTTPStatus     *int
	Reason             string
}

type sharedPoolMediaTaskRepository interface {
	CreateSharedPoolMediaTask(ctx context.Context, input CreateSharedPoolMediaTaskInput) (*SharedPoolMediaTask, error)
	GetSharedPoolMediaTaskByAPIKeyID(ctx context.Context, apiKeyID int64, upstreamRequestID string) (*SharedPoolMediaTask, error)
	GetSharedPoolAccessKeyForMediaTask(ctx context.Context, apiKeyID, taskID int64) (*SharedPoolAccessKey, error)
	UpdateSharedPoolMediaTaskStateTx(ctx context.Context, input UpdateSharedPoolMediaTaskStateInput) (*SharedPoolMediaTask, error)
}

type sharedPoolExpiredMediaTaskRepository interface {
	RecoverExpiredSharedPoolMediaTasksTx(ctx context.Context, now time.Time, limit int) (int, error)
}

func (s *BizDecipherService) CreateSharedPoolVideoTask(ctx context.Context, input CreateSharedPoolMediaTaskInput) (*SharedPoolMediaTask, error) {
	repo, err := sharedPoolMediaTaskRepositoryFromService(s)
	if err != nil {
		return nil, err
	}
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	if input.Provider == "xai" {
		input.Provider = PlatformGrok
	}
	input.ModelSnapshot = strings.TrimSpace(input.ModelSnapshot)
	input.UpstreamModelSnapshot = strings.TrimSpace(input.UpstreamModelSnapshot)
	if input.UpstreamModelSnapshot == "" {
		input.UpstreamModelSnapshot = input.ModelSnapshot
	}
	input.UpstreamRequestID = strings.TrimSpace(input.UpstreamRequestID)
	input.ReservationRequestID = strings.TrimSpace(input.ReservationRequestID)
	input.RequestedResolution = NormalizeVideoBillingResolutionOrDefault(input.RequestedResolution)
	if input.ReservationID <= 0 || input.AccessKeyID <= 0 || input.PoolID <= 0 || input.AccountID < 0 ||
		input.UserID <= 0 || input.PriceVersionID <= 0 || input.Provider != PlatformGrok ||
		input.ModelSnapshot == "" || input.UpstreamModelSnapshot == "" || len(input.UpstreamModelSnapshot) > 200 ||
		input.UpstreamRequestID == "" || len(input.UpstreamRequestID) > 200 ||
		input.ReservationRequestID == "" || len(input.ReservationRequestID) > 160 ||
		input.RequestedDurationSeconds < VideoBillingMinDurationSeconds || input.RequestedDurationSeconds > VideoBillingMaxDurationSeconds {
		return nil, errors.New("shared pool video task identity is invalid")
	}
	if input.ExpiresAt.IsZero() {
		input.ExpiresAt = time.Now().UTC().Add(sharedPoolVideoTaskTTL)
	}
	return repo.CreateSharedPoolMediaTask(ctx, input)
}

// RecoverExpiredSharedPoolMediaTasks never guesses a provider result and never
// refunds forwarded money. Unknown asynchronous tasks are moved, together with
// their reservation, to review_required so governance can resolve them without
// leaving an invisible perpetual forwarding hold.
func (s *BizDecipherService) RecoverExpiredSharedPoolMediaTasks(ctx context.Context, now time.Time, limit int) (int, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("shared pool media task service is unavailable")
	}
	repo, ok := s.repo.(sharedPoolExpiredMediaTaskRepository)
	if !ok || repo == nil {
		return 0, errors.New("shared pool media task repository is unavailable")
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
	return repo.RecoverExpiredSharedPoolMediaTasksTx(ctx, now.UTC(), limit)
}

func (s *BizDecipherService) GetSharedPoolVideoTask(ctx context.Context, apiKeyID int64, upstreamRequestID string) (*SharedPoolMediaTask, error) {
	repo, err := sharedPoolMediaTaskRepositoryFromService(s)
	if err != nil {
		return nil, err
	}
	upstreamRequestID = strings.TrimSpace(upstreamRequestID)
	if apiKeyID <= 0 || upstreamRequestID == "" || len(upstreamRequestID) > 200 {
		return nil, ErrSharedPoolMediaTaskNotFound
	}
	task, err := repo.GetSharedPoolMediaTaskByAPIKeyID(ctx, apiKeyID, upstreamRequestID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrSharedPoolMediaTaskNotFound
	}
	return task, nil
}

func (s *BizDecipherService) GetSharedPoolVideoTaskRuntime(ctx context.Context, apiKeyID int64, task *SharedPoolMediaTask) (*SharedPoolAccessKey, error) {
	repo, err := sharedPoolMediaTaskRepositoryFromService(s)
	if err != nil {
		return nil, err
	}
	if task == nil || task.ID <= 0 || apiKeyID <= 0 {
		return nil, ErrSharedPoolMediaTaskNotFound
	}
	accessKey, err := repo.GetSharedPoolAccessKeyForMediaTask(ctx, apiKeyID, task.ID)
	if err != nil {
		return nil, err
	}
	if accessKey == nil || accessKey.ID != task.AccessKeyID || accessKey.PoolID != task.PoolID || accessKey.AccountID != task.AccountID {
		return nil, ErrSharedPoolMediaTaskNotFound
	}
	accessKey, err = s.hydrateSharedPoolAccessKeyCredentials(accessKey)
	if err != nil {
		return nil, err
	}
	var quote SharedPoolPriceQuote
	if len(task.PriceSnapshot) == 0 || json.Unmarshal(task.PriceSnapshot, &quote) != nil {
		return nil, fmt.Errorf("%w: frozen media price snapshot is invalid", ErrSharedPoolPricingNotConfigured)
	}
	FinalizeSharedPoolPriceQuote(&quote)
	if err := ValidateSharedPoolPriceQuote(&quote); err != nil || quote.PriceVersionID != task.PriceVersionID ||
		quote.PoolID != task.PoolID || normalizeSharedPoolEndpoint(quote.EndpointType) != SharedPoolEndpointVideo {
		return nil, fmt.Errorf("%w: frozen media price snapshot does not match task", ErrSharedPoolPricingNotConfigured)
	}
	accessKey.PriceQuote = &quote
	accessKey.PublishedModelName = task.ModelSnapshot
	return accessKey, nil
}

func (s *BizDecipherService) UpdateSharedPoolVideoTaskState(ctx context.Context, input UpdateSharedPoolMediaTaskStateInput) (*SharedPoolMediaTask, error) {
	repo, err := sharedPoolMediaTaskRepositoryFromService(s)
	if err != nil {
		return nil, err
	}
	input.UpstreamRequestID = strings.TrimSpace(input.UpstreamRequestID)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.LastUpstreamStatus = strings.ToLower(strings.TrimSpace(input.LastUpstreamStatus))
	input.Reason = strings.TrimSpace(input.Reason)
	if input.TaskID <= 0 || input.AccessKeyID <= 0 || input.UpstreamRequestID == "" {
		return nil, errors.New("shared pool media task identity is invalid")
	}
	switch input.Status {
	case SharedPoolMediaTaskSubmitted, SharedPoolMediaTaskProcessing, SharedPoolMediaTaskSucceeded,
		SharedPoolMediaTaskFailed, SharedPoolMediaTaskReviewRequired:
	default:
		return nil, errors.New("shared pool media task status is invalid")
	}
	if input.LastHTTPStatus != nil && (*input.LastHTTPStatus < 100 || *input.LastHTTPStatus > 599) {
		return nil, errors.New("shared pool media task HTTP status is invalid")
	}
	if len(input.LastUpstreamStatus) > 64 {
		input.LastUpstreamStatus = input.LastUpstreamStatus[:64]
	}
	if len(input.Reason) > 160 {
		input.Reason = input.Reason[:160]
	}
	return repo.UpdateSharedPoolMediaTaskStateTx(ctx, input)
}

func sharedPoolMediaTaskRepositoryFromService(s *BizDecipherService) (sharedPoolMediaTaskRepository, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("shared pool media task service is unavailable")
	}
	repo, ok := s.repo.(sharedPoolMediaTaskRepository)
	if !ok || repo == nil {
		return nil, errors.New("shared pool media task repository is unavailable")
	}
	return repo, nil
}

func (s *OpenAIGatewayService) CreateSharedPoolVideoTask(ctx context.Context, input CreateSharedPoolMediaTaskInput) (*SharedPoolMediaTask, error) {
	if s == nil || s.bizDecipherService == nil {
		return nil, errors.New("shared pool media task service is unavailable")
	}
	return s.bizDecipherService.CreateSharedPoolVideoTask(ctx, input)
}

func (s *OpenAIGatewayService) GetSharedPoolVideoTask(ctx context.Context, apiKeyID int64, upstreamRequestID string) (*SharedPoolMediaTask, error) {
	if s == nil || s.bizDecipherService == nil {
		return nil, errors.New("shared pool media task service is unavailable")
	}
	return s.bizDecipherService.GetSharedPoolVideoTask(ctx, apiKeyID, upstreamRequestID)
}

func (s *OpenAIGatewayService) GetSharedPoolVideoTaskRuntime(ctx context.Context, apiKeyID int64, task *SharedPoolMediaTask) (*SharedPoolAccessKey, error) {
	if s == nil || s.bizDecipherService == nil {
		return nil, errors.New("shared pool media task service is unavailable")
	}
	return s.bizDecipherService.GetSharedPoolVideoTaskRuntime(ctx, apiKeyID, task)
}

func (s *OpenAIGatewayService) UpdateSharedPoolVideoTaskState(ctx context.Context, input UpdateSharedPoolMediaTaskStateInput) (*SharedPoolMediaTask, error) {
	if s == nil || s.bizDecipherService == nil {
		return nil, errors.New("shared pool media task service is unavailable")
	}
	return s.bizDecipherService.UpdateSharedPoolVideoTaskState(ctx, input)
}
