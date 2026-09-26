package service

import (
	"context"
	"testing"
	"time"
)

type sharedPoolProbeJobRepoStub struct {
	BizDecipherRepository
	pools        []SharedPool
	accounts     []SharedPoolAccount
	enqueueInput SharedPoolProbeJobEnqueueInput
	created      bool
}

func (r *sharedPoolProbeJobRepoStub) ListMySharedPools(context.Context, int64) ([]SharedPool, error) {
	return r.pools, nil
}

func (r *sharedPoolProbeJobRepoStub) ListSharedPoolAccounts(context.Context, int64, int64) ([]SharedPoolAccount, error) {
	return r.accounts, nil
}

func (r *sharedPoolProbeJobRepoStub) EnqueueSharedPoolProbeJob(_ context.Context, input SharedPoolProbeJobEnqueueInput) (*SharedPoolProbeJob, bool, error) {
	r.enqueueInput = input
	accountID := input.AccountID
	return &SharedPoolProbeJob{
		ID:            "00000000-0000-4000-8000-000000000001",
		OperationID:   input.OperationID,
		PoolID:        input.PoolID,
		AccountID:     &accountID,
		OwnerID:       input.OwnerID,
		ModelName:     input.ModelName,
		ProbeType:     input.ProbeType,
		CheckLevel:    input.CheckLevel,
		ConfigVersion: 1,
		Status:        SharedPoolProbeJobQueued,
	}, r.created, nil
}

func (r *sharedPoolProbeJobRepoStub) GetSharedPoolProbeJob(context.Context, string, int64) (*SharedPoolProbeJob, error) {
	return nil, nil
}

func (r *sharedPoolProbeJobRepoStub) ClaimSharedPoolProbeJobs(context.Context, string, int, time.Duration) ([]SharedPoolProbeJob, error) {
	return nil, nil
}

func (r *sharedPoolProbeJobRepoStub) HeartbeatSharedPoolProbeJob(context.Context, string, string, time.Duration) error {
	return nil
}

func (r *sharedPoolProbeJobRepoStub) CompleteSharedPoolProbeJob(context.Context, SharedPoolProbeJobCompletion) (*SharedPoolProbeJob, error) {
	return nil, nil
}

func TestEnqueueSharedPoolProbeJobAllowsInitialUngatedAccount(t *testing.T) {
	ownerID := int64(19)
	repo := &sharedPoolProbeJobRepoStub{
		created: true,
		pools: []SharedPool{{
			ID:                 7,
			OwnerID:            &ownerID,
			AccountModeEnabled: true,
			Models:             []string{"custom-model"},
		}},
		accounts: []SharedPoolAccount{{
			ID:              31,
			PoolID:          7,
			OwnerID:         ownerID,
			AuthType:        "api_key",
			UpstreamBaseURL: "https://upstream.example",
			HasUpstreamKey:  true,
			Schedulable:     true,
			Status:          "active",
			GateRequired:    true,
			GatePassed:      false,
			ModelConfigs: []SharedPoolModelConfig{{
				ModelName:         "custom-model",
				UpstreamModelName: "vendor/custom-model",
				ModelOpen:         true,
			}},
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	job, err := svc.EnqueueSharedPoolProbeJob(context.Background(), SharedPoolUpstreamProbeInput{
		PoolID:    7,
		OwnerID:   ownerID,
		ProbeType: "manual",
	}, "operation-initial-gate")
	if err != nil {
		t.Fatalf("enqueue initial gate probe: %v", err)
	}
	if job == nil || job.Status != SharedPoolProbeJobQueued {
		t.Fatalf("unexpected job: %#v", job)
	}
	if repo.enqueueInput.AccountID != 31 {
		t.Fatalf("selected account = %d, want 31", repo.enqueueInput.AccountID)
	}
	if repo.enqueueInput.ModelName != "custom-model" || repo.enqueueInput.UpstreamModelName != "vendor/custom-model" {
		t.Fatalf("unexpected model mapping: %#v", repo.enqueueInput)
	}
	if repo.enqueueInput.CheckLevel != "full" {
		t.Fatalf("check level = %q, want full", repo.enqueueInput.CheckLevel)
	}
}

func TestSharedPoolProbeCheckLevelScheduledFullIsFull(t *testing.T) {
	if got := sharedPoolProbeCheckLevel("scheduled_full"); got != "full" {
		t.Fatalf("scheduled_full check level = %q, want full", got)
	}
	if got := sharedPoolProbeCheckLevel("scheduled"); got != "basic" {
		t.Fatalf("scheduled check level = %q, want basic", got)
	}
}

func TestSharedPoolProbeJobTerminalStatus(t *testing.T) {
	if got := sharedPoolProbeJobTerminalStatus(&SharedPoolUpstreamProbeResult{OK: true}, nil); got != SharedPoolProbeJobSucceeded {
		t.Fatalf("success status = %q", got)
	}
	if got := sharedPoolProbeJobTerminalStatus(nil, context.DeadlineExceeded); got != SharedPoolProbeJobTimedOut {
		t.Fatalf("timeout status = %q", got)
	}
	if got := sharedPoolProbeJobTerminalStatus(nil, context.Canceled); got != SharedPoolProbeJobCancelled {
		t.Fatalf("cancelled status = %q", got)
	}
	if got := sharedPoolProbeJobTerminalStatus(&SharedPoolUpstreamProbeResult{OK: false}, nil); got != SharedPoolProbeJobFailed {
		t.Fatalf("failed status = %q", got)
	}
}

func TestSharedPoolProbeBudgetsCoverBoundedExecutionPlan(t *testing.T) {
	if sharedPoolFullProbeConcurrency < 2 || sharedPoolFullProbeConcurrency > 3 {
		t.Fatalf("full probe concurrency = %d, want 2..3", sharedPoolFullProbeConcurrency)
	}
	planned := sharedPoolFullProbePlannedWorstCase(len(sharedPoolFullCheckSpecs("gpt-test")))
	if planned > sharedPoolFullProbeTimeout {
		t.Fatalf("planned full probe worst case %s exceeds execution budget %s", planned, sharedPoolFullProbeTimeout)
	}
	if sharedPoolProbeJobTimeout <= sharedPoolFullProbeTimeout {
		t.Fatalf("job budget %s must leave completion margin after execution budget %s", sharedPoolProbeJobTimeout, sharedPoolFullProbeTimeout)
	}
	if sharedPoolProbeJobTimeout >= sharedPoolProbeJobLease {
		t.Fatalf("job budget %s must fit within lease %s", sharedPoolProbeJobTimeout, sharedPoolProbeJobLease)
	}
}

func TestSharedPoolProbeJobExecutionModelUsesUpstreamAlias(t *testing.T) {
	job := &SharedPoolProbeJob{
		ModelName:         "owner/custom-model",
		UpstreamModelName: "vendor/model-v2",
	}
	if got := sharedPoolProbeJobExecutionModel(job); got != "vendor/model-v2" {
		t.Fatalf("execution model = %q, want upstream alias", got)
	}
	job.UpstreamModelName = ""
	if got := sharedPoolProbeJobExecutionModel(job); got != "owner/custom-model" {
		t.Fatalf("fallback execution model = %q, want owner model", got)
	}
}
