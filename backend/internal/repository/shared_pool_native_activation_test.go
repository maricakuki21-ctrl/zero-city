package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNativePoolActivation_rejectsWrongOwner(t *testing.T) {
	repo, mock, closeDB := newNativeActivationMock(t)
	defer closeDB()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT native_onboarding_state").WithArgs(int64(1), int64(99)).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err := repo.ActivateNativePoolBilling(context.Background(), nativeActivationInput(1, 99, "op", 7))

	require.ErrorIs(t, err, service.ErrPoolForbidden)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNativePoolActivation_rejectsStaleConfig(t *testing.T) {
	repo, mock, closeDB := newNativeActivationMock(t)
	defer closeDB()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT native_onboarding_state").WithArgs(int64(1), int64(2)).WillReturnRows(nativePoolActivationRow("supply_ready_billing_blocked", 8, "", "", nil, nil))
	mock.ExpectRollback()

	_, err := repo.ActivateNativePoolBilling(context.Background(), nativeActivationInput(1, 2, "op", 7))

	require.ErrorIs(t, err, service.ErrNativeActivationStale)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNativePoolActivation_replaysSameOperation(t *testing.T) {
	repo, mock, closeDB := newNativeActivationMock(t)
	defer closeDB()
	input := nativeActivationInput(1, 2, "op", 7)
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT native_onboarding_state").WithArgs(int64(1), int64(2)).WillReturnRows(nativePoolActivationRow("billing_active", 9, "op", input.RequestFingerprint, int64(9), now))
	mock.ExpectCommit()

	result, err := repo.ActivateNativePoolBilling(context.Background(), input)

	require.NoError(t, err)
	require.True(t, result.AlreadyActive)
	require.Equal(t, int64(9), result.ActivatedConfigVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

func newNativeActivationMock(t *testing.T) (*bizDecipherRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return &bizDecipherRepository{db: db}, mock, func() { _ = db.Close() }
}

func nativeActivationInput(poolID, ownerID int64, operation string, version int64) service.SharedPoolNativeActivationInput {
	input, _ := service.NewSharedPoolNativeActivationInput(poolID, ownerID, operation, version)
	return input
}

func nativePoolActivationRow(state string, config int64, operation, fingerprint string, activatedVersion any, activatedAt any) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"state", "config", "operation", "fingerprint", "activated_version", "activated_at"}).
		AddRow(state, config, operation, fingerprint, activatedVersion, activatedAt)
}
