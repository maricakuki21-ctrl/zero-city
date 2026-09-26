package repository

import (
	"context"
	"encoding/json"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAssetWorkflowCheckpointCommitsUnderRowLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	state := service.AssetWorkflowState{RequestID: "workflow-operation", Total: 1, State: "running"}
	body, _ := json.Marshal(state)
	mock.ExpectExec("INSERT INTO asset_workflow_checkpoints").WithArgs(int64(7), "workflow-operation", "digest", string(body)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT input_digest,state .* FOR UPDATE").WithArgs(int64(7), "workflow-operation").
		WillReturnRows(sqlmock.NewRows([]string{"input_digest", "state"}).AddRow("digest", body))
	mock.ExpectExec("UPDATE asset_workflow_checkpoints").WithArgs(int64(7), "workflow-operation", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	repo := &bizDecipherRepository{db: db}
	result, err := repo.AdvanceAssetWorkflow(context.Background(), 7, "workflow-operation", "digest", state, func(s *service.AssetWorkflowState) error {
		s.State, s.Output = "succeeded", "plugin output"
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, "plugin output", result.Output)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAssetWorkflowCheckpointRejectsChangedInputBeforeExecution(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("INSERT INTO asset_workflow_checkpoints").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT input_digest,state .* FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"input_digest", "state"}).AddRow("original", []byte(`{}`)))
	mock.ExpectRollback()
	repo := &bizDecipherRepository{db: db}
	_, err = repo.AdvanceAssetWorkflow(context.Background(), 7, "workflow-operation", "changed", service.AssetWorkflowState{}, func(*service.AssetWorkflowState) error {
		t.Fatal("changed input dispatched")
		return nil
	})
	require.ErrorIs(t, err, service.ErrAssetCommerceConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
