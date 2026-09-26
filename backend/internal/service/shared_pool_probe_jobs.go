package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	sharedPoolProbeJobLease          = 4 * time.Minute
	sharedPoolProbeJobTimeout        = sharedPoolFullProbeTimeout + 20*time.Second
	sharedPoolProbeJobHeartbeatEvery = 20 * time.Second
)

const (
	SharedPoolProbeJobQueued    = "queued"
	SharedPoolProbeJobRunning   = "running"
	SharedPoolProbeJobSucceeded = "succeeded"
	SharedPoolProbeJobFailed    = "failed"
	SharedPoolProbeJobTimedOut  = "timed_out"
	SharedPoolProbeJobStale     = "stale"
	SharedPoolProbeJobCancelled = "cancelled"
)

// SharedPoolProbeJob is the durable control-plane representation of a probe.
// It intentionally contains no upstream credentials.
type SharedPoolProbeJob struct {
	ID                string                         `json:"id"`
	OperationID       string                         `json:"operation_id"`
	PoolID            int64                          `json:"pool_id"`
	AccountID         *int64                         `json:"account_id,omitempty"`
	OwnerID           int64                          `json:"owner_id"`
	ModelName         string                         `json:"model_name"`
	UpstreamModelName string                         `json:"upstream_model_name"`
	ProbeType         string                         `json:"probe_type"`
	CheckLevel        string                         `json:"check_level"`
	ConfigVersion     int64                          `json:"config_version"`
	Status            string                         `json:"status"`
	Attempt           int                            `json:"attempt"`
	MaxAttempts       int                            `json:"max_attempts"`
	LeaseOwner        string                         `json:"-"`
	LeaseExpiresAt    *time.Time                     `json:"lease_expires_at,omitempty"`
	HeartbeatAt       *time.Time                     `json:"heartbeat_at,omitempty"`
	StartedAt         *time.Time                     `json:"started_at,omitempty"`
	FinishedAt        *time.Time                     `json:"finished_at,omitempty"`
	Result            *SharedPoolUpstreamProbeResult `json:"result,omitempty"`
	ErrorType         string                         `json:"error_type,omitempty"`
	ErrorMessage      string                         `json:"error_message,omitempty"`
	CreatedAt         time.Time                      `json:"created_at"`
	UpdatedAt         time.Time                      `json:"updated_at"`
	Items             []SharedPoolProbeJobItem       `json:"items,omitempty"`
	Deduplicated      bool                           `json:"deduplicated,omitempty"`
}

type SharedPoolProbeJobItem struct {
	Index        int    `json:"index"`
	CheckID      string `json:"check_id"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	Required     bool   `json:"required"`
	Success      bool   `json:"success"`
	HTTPStatus   int    `json:"http_status"`
	LatencyMs    int    `json:"latency_ms"`
	ErrorType    string `json:"error_type,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	Evidence     string `json:"evidence,omitempty"`
}

type SharedPoolProbeJobEnqueueInput struct {
	OperationID       string
	PoolID            int64
	AccountID         int64
	OwnerID           int64
	ModelName         string
	UpstreamModelName string
	ProbeType         string
	CheckLevel        string
}

type SharedPoolProbeJobCompletion struct {
	JobID          string
	LeaseOwner     string
	TerminalStatus string
	History        SharedPoolProbeHistoryInput
	Result         *SharedPoolUpstreamProbeResult
}

// SharedPoolProbeJobRepository stays separate from BizDecipherRepository so
// existing mocks and unrelated repository consumers do not need a broad churn.
type SharedPoolProbeJobRepository interface {
	EnqueueSharedPoolProbeJob(ctx context.Context, input SharedPoolProbeJobEnqueueInput) (*SharedPoolProbeJob, bool, error)
	GetSharedPoolProbeJob(ctx context.Context, jobID string, ownerID int64) (*SharedPoolProbeJob, error)
	ClaimSharedPoolProbeJobs(ctx context.Context, leaseOwner string, limit int, lease time.Duration) ([]SharedPoolProbeJob, error)
	HeartbeatSharedPoolProbeJob(ctx context.Context, jobID, leaseOwner string, lease time.Duration) error
	CompleteSharedPoolProbeJob(ctx context.Context, completion SharedPoolProbeJobCompletion) (*SharedPoolProbeJob, error)
}

func (s *BizDecipherService) sharedPoolProbeJobRepository() (SharedPoolProbeJobRepository, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	repo, ok := s.repo.(SharedPoolProbeJobRepository)
	if !ok {
		return nil, errors.New("shared pool probe jobs are unavailable")
	}
	return repo, nil
}

func normalizeSharedPoolProbeJobType(probeType string) (string, error) {
	probeType = strings.ToLower(strings.TrimSpace(probeType))
	if probeType == "" {
		probeType = "manual"
	}
	switch probeType {
	case "manual", "publish_gate", "scheduled", "scheduled_full":
		return probeType, nil
	default:
		return "", errors.New("invalid shared pool probe type")
	}
}

func firstOpenSharedPoolProbeModel(configs []SharedPoolModelConfig) string {
	for _, cfg := range configs {
		if cfg.ModelOpen && strings.TrimSpace(cfg.ModelName) != "" {
			return strings.TrimSpace(cfg.ModelName)
		}
	}
	return ""
}

func sharedPoolUpstreamProbeModel(configs []SharedPoolModelConfig, model string) string {
	model = strings.TrimSpace(model)
	for _, cfg := range configs {
		if !cfg.ModelOpen {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(cfg.ModelName), model) || strings.EqualFold(strings.TrimSpace(cfg.UpstreamModelName), model) {
			return firstNonEmpty(strings.TrimSpace(cfg.UpstreamModelName), strings.TrimSpace(cfg.ModelName), model)
		}
	}
	return model
}

// EnqueueSharedPoolProbeJob validates ownership and configuration, then only
// persists work. No upstream call is made on the request context.
func (s *BizDecipherService) EnqueueSharedPoolProbeJob(ctx context.Context, input SharedPoolUpstreamProbeInput, operationID string) (*SharedPoolProbeJob, error) {
	repo, err := s.sharedPoolProbeJobRepository()
	if err != nil {
		return nil, err
	}
	if input.PoolID <= 0 || input.OwnerID <= 0 {
		return nil, errors.New("pool id and owner id are required")
	}
	probeType, err := normalizeSharedPoolProbeJobType(input.ProbeType)
	if err != nil {
		return nil, err
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		operationID = uuid.NewString()
	}
	if len(operationID) > 128 {
		return nil, errors.New("operation id is too long")
	}

	pool, err := s.findOwnedSharedPool(ctx, input.PoolID, input.OwnerID)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, ErrPoolForbidden
	}
	if isNativeR1Pool(pool) {
		return nil, ErrBillingActivationRequired
	}

	probeModel := strings.TrimSpace(input.ProbeModel)
	upstreamModel := probeModel
	accountID := input.AccountID
	if pool.AccountModeEnabled || accountID > 0 {
		accounts, listErr := s.ListSharedPoolAccounts(ctx, input.PoolID, input.OwnerID)
		if listErr != nil {
			return nil, listErr
		}
		var selected *SharedPoolAccount
		if accountID > 0 {
			for i := range accounts {
				if accounts[i].ID == accountID {
					selected = &accounts[i]
					break
				}
			}
			if selected == nil {
				return nil, ErrPoolForbidden
			}
			if !sharedPoolAccountEligibleForProbe(*selected, probeModel, time.Now()) {
				return nil, errors.New("selected account is not currently eligible for probing")
			}
		} else {
			selected, err = s.selectSharedPoolProbeAccount(ctx, input.PoolID, input.OwnerID, probeModel)
			if err != nil {
				return nil, err
			}
		}
		if selected != nil {
			accountID = selected.ID
			if probeModel == "" {
				probeModel = firstOpenSharedPoolProbeModel(selected.ModelConfigs)
			}
			upstreamModel = sharedPoolUpstreamProbeModel(selected.ModelConfigs, probeModel)
		}
	}
	if probeModel == "" && len(pool.Models) > 0 {
		probeModel = strings.TrimSpace(pool.Models[0])
		upstreamModel = probeModel
	}
	if probeModel == "" {
		return nil, errors.New("probe model is required; select an open model first")
	}

	job, created, err := repo.EnqueueSharedPoolProbeJob(ctx, SharedPoolProbeJobEnqueueInput{
		OperationID:       operationID,
		PoolID:            input.PoolID,
		AccountID:         accountID,
		OwnerID:           input.OwnerID,
		ModelName:         probeModel,
		UpstreamModelName: firstNonEmpty(upstreamModel, probeModel),
		ProbeType:         probeType,
		CheckLevel:        sharedPoolProbeCheckLevel(probeType),
	})
	if err != nil {
		return nil, err
	}
	if job != nil {
		job.Deduplicated = !created
	}
	return job, nil
}

func (s *BizDecipherService) GetSharedPoolProbeJob(ctx context.Context, jobID string, ownerID int64) (*SharedPoolProbeJob, error) {
	repo, err := s.sharedPoolProbeJobRepository()
	if err != nil {
		return nil, err
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" || ownerID <= 0 {
		return nil, errors.New("invalid probe job or owner id")
	}
	return repo.GetSharedPoolProbeJob(ctx, jobID, ownerID)
}

func sharedPoolProbeJobTerminalStatus(result *SharedPoolUpstreamProbeResult, probeErr error) string {
	if errors.Is(probeErr, context.DeadlineExceeded) {
		return SharedPoolProbeJobTimedOut
	}
	if errors.Is(probeErr, context.Canceled) {
		return SharedPoolProbeJobCancelled
	}
	if probeErr == nil && result != nil && result.OK {
		return SharedPoolProbeJobSucceeded
	}
	return SharedPoolProbeJobFailed
}

func sharedPoolProbeJobExecutionModel(job *SharedPoolProbeJob) string {
	if job == nil {
		return ""
	}
	return firstNonEmpty(strings.TrimSpace(job.UpstreamModelName), strings.TrimSpace(job.ModelName))
}

// RunSharedPoolProbeJobs claims durable work with a database lease. It is
// called by a background worker, never by the browser request goroutine.
func (s *BizDecipherService) RunSharedPoolProbeJobs(ctx context.Context, workerID string, limit int) (int, error) {
	repo, err := s.sharedPoolProbeJobRepository()
	if err != nil {
		return 0, err
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = "shared-pool-probe-" + uuid.NewString()
	}
	if limit <= 0 {
		limit = 1
	}
	if limit > 4 {
		limit = 4
	}
	jobs, err := repo.ClaimSharedPoolProbeJobs(ctx, workerID, limit, sharedPoolProbeJobLease)
	if err != nil {
		return 0, err
	}
	processed := 0
	var firstErr error
	for i := range jobs {
		job := jobs[i]
		if err := s.runSharedPoolProbeJob(ctx, repo, workerID, &job); err != nil && firstErr == nil {
			firstErr = err
		}
		processed++
	}
	return processed, firstErr
}

func (s *BizDecipherService) runSharedPoolProbeJob(parent context.Context, repo SharedPoolProbeJobRepository, workerID string, job *SharedPoolProbeJob) error {
	if job == nil {
		return nil
	}
	probeCtx, cancelProbe := context.WithTimeout(parent, sharedPoolProbeJobTimeout)
	defer cancelProbe()
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(sharedPoolProbeJobHeartbeatEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				heartbeatCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				_ = repo.HeartbeatSharedPoolProbeJob(heartbeatCtx, job.ID, workerID, sharedPoolProbeJobLease)
				cancel()
			case <-probeCtx.Done():
				return
			}
		}
	}()

	accountID := int64(0)
	if job.AccountID != nil {
		accountID = *job.AccountID
	}
	pool, poolErr := s.findOwnedSharedPool(probeCtx, job.PoolID, job.OwnerID)
	var result *SharedPoolUpstreamProbeResult
	var probeErr error
	if poolErr != nil {
		probeErr = poolErr
	} else if isNativeR1Pool(pool) {
		probeErr = ErrBillingActivationRequired
	} else {
		result, probeErr = s.ProbeSharedPoolUpstream(probeCtx, SharedPoolUpstreamProbeInput{
			PoolID:     job.PoolID,
			AccountID:  accountID,
			OwnerID:    job.OwnerID,
			ProbeModel: sharedPoolProbeJobExecutionModel(job),
			ProbeType:  job.ProbeType,
			SkipRecord: true,
		})
	}
	if result != nil {
		// Keep the public result on the pool's model name; the upstream alias is
		// retained separately on the durable job and history.
		result.Model = job.ModelName
	}
	cancelProbe()
	<-heartbeatDone

	history := sharedPoolProbeHistoryInputFromResult(SharedPoolUpstreamProbeInput{
		PoolID: job.PoolID, AccountID: accountID, OwnerID: job.OwnerID,
		ProbeModel: job.ModelName, ProbeType: job.ProbeType,
	}, result, probeErr)
	history.ModelName = job.ModelName
	history.UpstreamModelName = sharedPoolProbeJobExecutionModel(job)
	completion := SharedPoolProbeJobCompletion{
		JobID:          job.ID,
		LeaseOwner:     workerID,
		TerminalStatus: sharedPoolProbeJobTerminalStatus(result, probeErr),
		History:        history,
		Result:         result,
	}
	writeCtx, cancelWrite := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelWrite()
	if _, err := repo.CompleteSharedPoolProbeJob(writeCtx, completion); err != nil {
		return fmt.Errorf("complete shared pool probe job %s: %w", job.ID, err)
	}
	return nil
}
