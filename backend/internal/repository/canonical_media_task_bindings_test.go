package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/platform/mediatask"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

var canonicalMediaTaskBindingColumns = []string{
	"business_event_id", "idempotency_key", "request_hash", "api_key_id", "user_id",
	"group_id", "account_id", "endpoint", "state", "upstream_task_id",
}

func newCanonicalMediaTaskBindingInput(t *testing.T) mediatask.ClaimBindingInput {
	t.Helper()
	createContext, err := mediatask.NewCreateContext("media:11:client-1", "client-1", []byte(`{"prompt":"clip"}`))
	require.NoError(t, err)
	return mediatask.ClaimBindingInput{
		Context: createContext, APIKeyID: 11, UserID: 22, GroupID: 33, AccountID: 44, Endpoint: "videos_generations",
	}
}

func canonicalMediaTaskBindingRow(input mediatask.ClaimBindingInput, state, upstreamTaskID string) *sqlmock.Rows {
	return sqlmock.NewRows(canonicalMediaTaskBindingColumns).AddRow(
		input.Context.BusinessEventID(), input.Context.IdempotencyKey(), input.Context.RequestHash(),
		input.APIKeyID, input.UserID, input.GroupID, input.AccountID, input.Endpoint, state, upstreamTaskID,
	)
}

func TestClaimCanonicalMediaTaskReplaysMatchingBindingAndRejectsConflict(t *testing.T) {
	tests := []struct {
		name          string
		storedContext mediatask.CreateContext
		wantConflict  bool
	}{
		{name: "matching payload replays without another create"},
		{
			name: "different payload conflicts",
			storedContext: func() mediatask.CreateContext {
				value, err := mediatask.NewCreateContext("media:11:client-1", "client-1", []byte(`{"prompt":"different"}`))
				require.NoError(t, err)
				return value
			}(),
			wantConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			input := newCanonicalMediaTaskBindingInput(t)
			storedContext := tt.storedContext
			if storedContext.BusinessEventID() == "" {
				storedContext = input.Context
			}
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO canonical_media_task_bindings")).
				WithArgs(input.Context.BusinessEventID(), input.Context.IdempotencyKey(), input.Context.RequestHash(), input.APIKeyID, input.UserID, input.GroupID, input.AccountID, input.Endpoint).
				WillReturnRows(sqlmock.NewRows(canonicalMediaTaskBindingColumns))
			mock.ExpectQuery(regexp.QuoteMeta("FROM canonical_media_task_bindings\n\t\tWHERE api_key_id = $1 AND idempotency_key = $2")).
				WithArgs(input.APIKeyID, input.Context.IdempotencyKey()).
				WillReturnRows(sqlmock.NewRows(canonicalMediaTaskBindingColumns).AddRow(
					storedContext.BusinessEventID(), storedContext.IdempotencyKey(), storedContext.RequestHash(),
					input.APIKeyID, input.UserID, input.GroupID, input.AccountID, input.Endpoint, "pending", "",
				))

			// When
			binding, claimed, claimErr := (&usageBillingRepository{db: db}).ClaimCanonicalMediaTask(context.Background(), input)

			// Then
			if tt.wantConflict {
				require.ErrorIs(t, claimErr, service.ErrCanonicalMediaTaskConflict)
				require.False(t, claimed)
				require.Zero(t, binding)
			} else {
				require.NoError(t, claimErr)
				require.False(t, claimed)
				require.Equal(t, input.Context.RequestHash(), binding.Context.RequestHash())
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAcceptCanonicalMediaTaskReplaysSameIDAndRejectsDifferentID(t *testing.T) {
	tests := []struct {
		name         string
		storedTaskID string
		wantConflict bool
	}{
		{name: "same accepted task id replays", storedTaskID: "upstream-task-1"},
		{name: "different accepted task id conflicts", storedTaskID: "upstream-task-2", wantConflict: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			input := newCanonicalMediaTaskBindingInput(t)
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mock.ExpectQuery(regexp.QuoteMeta("UPDATE canonical_media_task_bindings")).
				WithArgs(input.Context.BusinessEventID(), "upstream-task-1").
				WillReturnRows(sqlmock.NewRows(canonicalMediaTaskBindingColumns))
			mock.ExpectQuery(regexp.QuoteMeta("FROM canonical_media_task_bindings WHERE business_event_id = $1")).
				WithArgs(input.Context.BusinessEventID()).
				WillReturnRows(canonicalMediaTaskBindingRow(input, "accepted", tt.storedTaskID))

			// When
			binding, acceptErr := (&usageBillingRepository{db: db}).AcceptCanonicalMediaTask(context.Background(), input.Context, "upstream-task-1")

			// Then
			if tt.wantConflict {
				require.ErrorIs(t, acceptErr, service.ErrCanonicalMediaTaskConflict)
				require.Zero(t, binding)
			} else {
				require.NoError(t, acceptErr)
				require.Equal(t, "upstream-task-1", binding.UpstreamTaskID)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
