package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestApplySharedPoolMediaEndpointProbeRejectsStalePoolConfig(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}
	input := mediaProbeResultForRepositoryTest()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT spe.id, spe.probe_plan_version, spe.config_version,")+`.*`+regexp.QuoteMeta("sp.config_version")+`.*`+regexp.QuoteMeta("FOR SHARE OF sp")+`.*`+regexp.QuoteMeta("FOR UPDATE OF spe")).
		WithArgs(input.PoolID, input.ModelName, input.EndpointType).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "probe_plan_version", "endpoint_config_version", "pool_config_version", "last_media_probe_at",
		}).AddRow(int64(31), input.ProbePlanVersion, input.EndpointConfigVersion, input.PoolConfigVersion+1, nil))
	mock.ExpectRollback()

	err = repo.ApplySharedPoolMediaEndpointProbeResultTx(context.Background(), input)
	if !errors.Is(err, service.ErrSharedPoolMediaProbeConflict) {
		t.Fatalf("expected stale pool config conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestApplySharedPoolMediaEndpointProbeRejectsAccountFromAnotherPool(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}
	input := mediaProbeResultForRepositoryTest()
	input.AccountID = 55
	input.AccountConfigVersion = 4

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT spa.config_version")+`.*`+regexp.QuoteMeta("spa.id = $1 AND spa.pool_id = $2")+`.*`+regexp.QuoteMeta("FOR SHARE")).
		WithArgs(input.AccountID, input.PoolID).
		WillReturnRows(sqlmock.NewRows([]string{"config_version"}))
	mock.ExpectRollback()

	err = repo.ApplySharedPoolMediaEndpointProbeResultTx(context.Background(), input)
	if !errors.Is(err, service.ErrSharedPoolMediaProbeConflict) {
		t.Fatalf("expected cross-pool account conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestApplySharedPoolMediaEndpointProbeRejectsStaleAccountConfig(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}
	input := mediaProbeResultForRepositoryTest()
	input.AccountID = 55
	input.AccountConfigVersion = 4

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT spa.config_version")+`.*`+regexp.QuoteMeta("spa.id = $1 AND spa.pool_id = $2")+`.*`+regexp.QuoteMeta("FOR SHARE")).
		WithArgs(input.AccountID, input.PoolID).
		WillReturnRows(sqlmock.NewRows([]string{"config_version"}).AddRow(input.AccountConfigVersion + 1))
	mock.ExpectRollback()

	err = repo.ApplySharedPoolMediaEndpointProbeResultTx(context.Background(), input)
	if !errors.Is(err, service.ErrSharedPoolMediaProbeConflict) {
		t.Fatalf("expected stale account config conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func mediaProbeResultForRepositoryTest() service.SharedPoolMediaEndpointProbeResult {
	checkedAt := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	return service.SharedPoolMediaEndpointProbeResult{
		PoolID:                7,
		ModelName:             "grok-imagine-video",
		EndpointType:          service.SharedPoolEndpointVideo,
		OperationID:           "probe:test:config-fence",
		PoolConfigVersion:     12,
		ProbePlanVersion:      3,
		EndpointConfigVersion: 9,
		Success:               true,
		OutputObserved:        true,
		AsyncTerminalObserved: true,
		CheckedAt:             checkedAt,
		ExpiresAt:             checkedAt.Add(time.Hour),
		ResultStatus:          "passed",
	}
}
