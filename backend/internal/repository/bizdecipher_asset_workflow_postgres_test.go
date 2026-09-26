package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAssetWorkflowCheckpointPostgres(t *testing.T) {
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var database string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT current_database()").Scan(&database))
	require.True(t, strings.HasPrefix(database, "bizdecipher_columns_acceptance_test_"))
	require.NoError(t, ApplyMigrations(ctx, db))
	owner := insertMarketplaceTestUser(t, db, fmt.Sprintf("workflow-%d@example.test", time.Now().UnixNano()))
	other := insertMarketplaceTestUser(t, db, fmt.Sprintf("workflow-other-%d@example.test", time.Now().UnixNano()))
	repo := &bizDecipherRepository{db: db}
	initial := service.AssetWorkflowState{RequestID: "checkpoint-test", AssetID: 41, Version: "v1",
		PlanDigest: strings.Repeat("a", 64), State: "running", Total: 2}
	digest := strings.Repeat("b", 64)
	_, err = repo.AdvanceAssetWorkflow(ctx, owner, initial.RequestID, digest, initial, func(s *service.AssetWorkflowState) error {
		s.Steps = append(s.Steps, service.AssetWorkflowStepResult{Index: 0, Action: "plugin.wasm", State: "succeeded", Output: "first"})
		s.Output = "first"
		return nil
	})
	require.NoError(t, err)
	// Recreate the repository to ensure results are not process-local memory.
	repo = &bizDecipherRepository{db: db}
	saved, err := repo.GetAssetWorkflow(ctx, owner, initial.RequestID)
	require.NoError(t, err)
	require.Equal(t, "first", saved.Output)
	_, err = repo.GetAssetWorkflow(ctx, other, initial.RequestID)
	require.ErrorIs(t, err, service.ErrCapabilityAssetPackageNotFound)
	_, err = repo.AdvanceAssetWorkflow(ctx, owner, initial.RequestID, strings.Repeat("c", 64), initial,
		func(*service.AssetWorkflowState) error { t.Fatal("changed input executed"); return nil })
	require.ErrorIs(t, err, service.ErrAssetCommerceConflict)
	injected := errors.New("step interrupted")
	_, err = repo.AdvanceAssetWorkflow(ctx, owner, initial.RequestID, digest, initial, func(s *service.AssetWorkflowState) error {
		s.Output = "uncommitted"
		return injected
	})
	require.ErrorIs(t, err, injected)
	saved, err = repo.GetAssetWorkflow(ctx, owner, initial.RequestID)
	require.NoError(t, err)
	require.Equal(t, "first", saved.Output)
	var executions atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := repo.AdvanceAssetWorkflow(ctx, owner, initial.RequestID, digest, initial, func(s *service.AssetWorkflowState) error {
				if s.State == "succeeded" {
					return nil
				}
				executions.Add(1)
				s.Steps = append(s.Steps, service.AssetWorkflowStepResult{Index: 1, Action: "plugin.wasm", State: "succeeded", Output: "second"})
				s.State, s.Output = "succeeded", "second"
				return nil
			})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	require.Equal(t, int32(1), executions.Load())
	saved, err = repo.GetAssetWorkflow(ctx, owner, initial.RequestID)
	require.NoError(t, err)
	require.Len(t, saved.Steps, 2)
	require.Equal(t, "succeeded", saved.State)
	history, err := repo.ListAssetWorkflows(ctx, owner, 41, "v1")
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, initial.RequestID, history[0].RequestID)
	require.Empty(t, history[0].Output)
	require.Empty(t, history[0].Steps[0].Output)
	for _, scope := range []struct {
		actor   int64
		asset   int64
		version string
	}{{other, 41, "v1"}, {owner, 42, "v1"}, {owner, 41, "v2"}} {
		history, err = repo.ListAssetWorkflows(ctx, scope.actor, scope.asset, scope.version)
		require.NoError(t, err)
		require.Empty(t, history)
	}
}
