package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHasSharedPoolOAuthCompactCapabilityUsesLatestCurrentVersionEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT COALESCE((")+`.*`+
		regexp.QuoteMeta("item.success = TRUE")+`.*`+
		regexp.QuoteMeta("item.http_status BETWEEN 200 AND 299")+`.*`+
		regexp.QuoteMeta("job.account_id = $2")+`.*`+
		regexp.QuoteMeta("job.config_version = pool.config_version")+`.*`+
		regexp.QuoteMeta("LOWER(BTRIM(job.model_name)) = LOWER(BTRIM($3))")+`.*`+
		regexp.QuoteMeta("LOWER(BTRIM(job.upstream_model_name)) = LOWER(BTRIM($4))")+`.*`+
		regexp.QuoteMeta("item.check_id = 'oauth_responses_compact'")+`.*`+
		regexp.QuoteMeta("ORDER BY job.finished_at DESC NULLS LAST")).
		WithArgs(int64(9), int64(27), "gpt-5.6", "gpt-5.6-codex").
		WillReturnRows(sqlmock.NewRows([]string{"ready"}).AddRow(true))

	ready, err := repo.HasSharedPoolOAuthCompactCapability(context.Background(), 9, 27, "gpt-5.6", "gpt-5.6-codex")
	if err != nil {
		t.Fatalf("HasSharedPoolOAuthCompactCapability: %v", err)
	}
	if !ready {
		t.Fatal("expected current compact capability")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestHasSharedPoolOAuthCompactCapabilityDefaultsToFalse(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT COALESCE((")).
		WithArgs(int64(9), int64(27), "gpt-5.6", "gpt-5.6-codex").
		WillReturnRows(sqlmock.NewRows([]string{"ready"}).AddRow(false))

	ready, err := repo.HasSharedPoolOAuthCompactCapability(context.Background(), 9, 27, "gpt-5.6", "gpt-5.6-codex")
	if err != nil {
		t.Fatalf("HasSharedPoolOAuthCompactCapability: %v", err)
	}
	if ready {
		t.Fatal("unexpected compact capability")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGetSharedPoolCompactAccessKeyRequiresModelBoundCurrentCapability(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectQuery(`(?s)`+regexp.QuoteMeta("SELECT sak.id")+`.*`+
		regexp.QuoteMeta("COALESCE(NULLIF(spa_req.upstream_model_name, ''), spm_req.upstream_model_name, spm_req.model_name, '') AS upstream_model_name")+`.*`+
		regexp.QuoteMeta("LOWER(TRIM(spa.auth_type)) = 'oauth'")+`.*`+
		regexp.QuoteMeta("spm_req.upstream_model_name,")+`.*`+
		regexp.QuoteMeta("spm_req.model_name,")+`.*`+
		regexp.QuoteMeta("item.success = TRUE")+`.*`+
		regexp.QuoteMeta("item.http_status BETWEEN 200 AND 299")+`.*`+
		regexp.QuoteMeta("job.config_version = sp.config_version")+`.*`+
		regexp.QuoteMeta("LOWER(BTRIM(job.model_name)) = LOWER(BTRIM(spm_req.model_name))")+`.*`+
		regexp.QuoteMeta("LOWER(BTRIM(job.upstream_model_name)) = LOWER(BTRIM(COALESCE(")+`.*`+
		regexp.QuoteMeta("item.check_id = 'oauth_responses_compact'")+`.*`+
		regexp.QuoteMeta("AND sp.account_mode_enabled = TRUE")+`.*`+
		regexp.QuoteMeta("AND spa_req.id IS NOT NULL")).
		WithArgs(int64(41), "published-gpt-5.6").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	accessKey, err := repo.GetSharedPoolCompactAccessKeyByAPIKeyID(context.Background(), 41, "published-gpt-5.6")
	if err != nil {
		t.Fatalf("GetSharedPoolCompactAccessKeyByAPIKeyID: %v", err)
	}
	if accessKey != nil {
		t.Fatalf("unexpected compact route: %#v", accessKey)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
