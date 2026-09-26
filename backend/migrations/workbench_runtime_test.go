package migrations

import (
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestWorkbenchRuntimeMigration(t *testing.T) {
	// Given
	content, err := FS.ReadFile("242_workbench_runtime.sql")
	require.NoError(t, err)
	migrationSQL := string(content)
	lowerSQL := strings.ToLower(migrationSQL)
	for _, fragment := range []string{
		"create table if not exists workbench_runs",
		"create table if not exists workbench_operations",
		"create table if not exists workbench_run_events",
		"create table if not exists workbench_artifacts",
		"create table if not exists workbench_saved_snapshots",
		"create table if not exists workbench_run_derivations",
		"create table if not exists canonical_runner_jobs",
		"unique (run_id, owner_user_id)",
		"foreign key (run_id, owner_user_id)",
		"foreign key (source_snapshot_id, source_run_id, owner_user_id)",
		"workbench_runs_replay_owner_fk",
		"workbench_runs_fork_owner_fk",
		"workbench_operations_result_snapshot_fk",
		"primary key (run_id, seq)",
		"numeric(24,12)",
		"^[0-9a-f]{64}$",
		"workbench terminal lineage is immutable",
	} {
		require.Contains(t, lowerSQL, fragment)
	}
	for _, forbidden := range []string{
		"create table if not exists workbench_usage",
		"create table if not exists workbench_ledger",
		"create table if not exists workbench_balance",
		"create table if not exists workbench_scheduler",
		"create table if not exists workbench_health",
		"create table if not exists workbench_provider_accounts",
		"create table if not exists workbench_media_tasks",
		"usage_tokens",
		"provider_api_key",
		"retry_count",
	} {
		require.NotContains(t, lowerSQL, forbidden)
	}

	db := startWorkbenchPostgres(t)
	createWorkbenchPrerequisites(t, db)
	t.Log("prerequisite schema ready")

	// When
	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(migrationSQL)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	t.Log("transactional rollback verified")

	// Then
	var rolledBack bool
	require.NoError(t, db.QueryRow(`SELECT to_regclass('public.workbench_runs') IS NULL`).Scan(&rolledBack))
	require.True(t, rolledBack)
	_, err = db.Exec(migrationSQL)
	require.NoError(t, err)
	t.Log("migration apply and repeat-apply verified")
	_, err = db.Exec(migrationSQL)
	require.NoError(t, err)

	insertRun := `INSERT INTO workbench_runs
		(run_id, owner_user_id, workspace_id, capability, input_snapshot, input_sha256, state, terminal_at)
		VALUES ($1, $2, 'wbw_main', 'text', '{}', $3, $4, $5)`
	validDigest := strings.Repeat("a", 64)
	requireExecError(t, db, insertRun, "wbr_badstate", 1, validDigest, "unknown", nil)
	requireExecError(t, db, insertRun, "wbr_baddigest", 1, "ABC", "queued", nil)
	_, err = db.Exec(insertRun, "wbr_source", 1, validDigest, "queued", nil)
	require.NoError(t, err)
	_, err = db.Exec(insertRun, "wbr_snapshot", 1, validDigest, "queued", nil)
	require.NoError(t, err)
	_, err = db.Exec(insertRun, "wbr_derived", 1, validDigest, "queued", nil)
	require.NoError(t, err)
	_, err = db.Exec(insertRun, "wbr_otherowner", 2, validDigest, "queued", nil)
	require.NoError(t, err)
	requireExecError(t, db, `INSERT INTO workbench_runs
		(run_id, owner_user_id, workspace_id, capability, input_snapshot, input_sha256, replay_of_run_id)
		VALUES ('wbr_crossowner', 1, 'wbw_main', 'text', '{}', $1, 'wbr_otherowner')`, validDigest)
	_, err = db.Exec(insertRun, "wbr_terminal", 1, validDigest, "succeeded", "2026-09-03T00:00:00Z")
	require.NoError(t, err)

	requireExecError(t, db, `UPDATE workbench_runs SET updated_at = NOW() WHERE run_id = 'wbr_source'`)
	requireExecError(t, db, `UPDATE workbench_runs SET canonical_request_id = 'req_changed', version = version + 1 WHERE run_id = 'wbr_terminal'`)

	insertOperation := `INSERT INTO workbench_operations
		(operation_id, run_id, owner_user_id, operation_key, operation_kind, request_fingerprint_sha256)
		VALUES ($1, 'wbr_source', $2, 'op-key', 'create', $3)`
	requireExecError(t, db, insertOperation, "wbo_wrongowner", 2, validDigest)
	_, err = db.Exec(insertOperation, "wbo_first", 1, validDigest)
	require.NoError(t, err)
	requireExecError(t, db, insertOperation, "wbo_duplicate", 1, validDigest)
	requireExecError(t, db, `INSERT INTO workbench_operations
		(operation_id, run_id, owner_user_id, operation_key, operation_kind, request_fingerprint_sha256, state, result_run_id, result_snapshot_id, result_state)
		VALUES ('wbo_bad_snapshot', 'wbr_source', 1, 'bad-snapshot', 'replay', $1, 'committed', 'wbr_source', 'wbs_missing', 'succeeded')`, validDigest)

	insertEvent := `INSERT INTO workbench_run_events
		(run_id, seq, event_id, event_kind, event_payload, payload_sha256)
		VALUES ('wbr_source', 1, $1, 'created', '{}', $2)`
	_, err = db.Exec(insertEvent, "wbe_first", validDigest)
	require.NoError(t, err)
	requireExecError(t, db, insertEvent, "wbe_reused", validDigest)
	requireExecError(t, db, `UPDATE workbench_run_events SET event_kind = 'changed' WHERE run_id = 'wbr_source' AND seq = 1`)

	insertSnapshot := `INSERT INTO workbench_saved_snapshots
		(snapshot_id, run_id, owner_user_id, snapshot, snapshot_sha256) VALUES ($1, $2, $3, '{}', $4)`
	_, err = db.Exec(insertSnapshot, "wbs_source", "wbr_source", 1, validDigest)
	require.NoError(t, err)
	_, err = db.Exec(insertSnapshot, "wbs_other", "wbr_snapshot", 1, strings.Repeat("b", 64))
	require.NoError(t, err)
	requireExecError(t, db, insertSnapshot, "wbs_wrongowner", "wbr_source", 2, strings.Repeat("c", 64))

	insertArtifact := `INSERT INTO workbench_artifacts
		(artifact_id, run_id, owner_user_id, artifact_kind, media_type, storage_uri, byte_size, digest_sha256)
		VALUES ($1, 'wbr_source', $2, 'text', 'text/plain', 'memory://artifact', 1, $3)`
	requireExecError(t, db, insertArtifact, "wba_wrongowner", 2, validDigest)
	_, err = db.Exec(insertArtifact, "wba_output", 1, validDigest)
	require.NoError(t, err)

	insertDerivation := `INSERT INTO workbench_run_derivations
		(derivation_id, owner_user_id, source_run_id, source_snapshot_id, derived_run_id, derivation_kind)
		VALUES ($1, $2, $3, $4, $5, 'replay')`
	requireExecError(t, db, insertDerivation, "wbd_badsnapshot", 1, "wbr_source", "wbs_other", "wbr_derived")
	requireExecError(t, db, insertDerivation, "wbd_badowner", 1, "wbr_source", "wbs_source", "wbr_otherowner")
	_, err = db.Exec(insertDerivation, "wbd_valid", 1, "wbr_source", "wbs_source", "wbr_derived")
	require.NoError(t, err)

	insertJob := `INSERT INTO canonical_runner_jobs
		(job_id, run_id, owner_user_id, operation_id, idempotency_key, request_fingerprint_sha256, state)
		VALUES ($1, 'wbr_source', $2, 'wbo_first', 'job-key', $3, 'queued')`
	requireExecError(t, db, insertJob, "wbj_wrongowner", 2, validDigest)
	_, err = db.Exec(insertJob, "wbj_first", 1, validDigest)
	require.NoError(t, err)
	t.Log("constraint rejection scenarios verified")
}

func startWorkbenchPostgres(t *testing.T) *sql.DB {
	t.Helper()
	binDir := strings.TrimSpace(os.Getenv("BIZDECIPHER_TEST_POSTGRES_BIN"))
	if binDir == "" && runtime.GOOS == "windows" {
		binDir = `D:\BizDecipher-NewSystem-20260819\work\toolchains\postgres\pg16\bin`
	}
	initdb := filepath.Join(binDir, executableName("initdb"))
	pgctl := filepath.Join(binDir, executableName("pg_ctl"))
	if binDir == "" || !regularFile(initdb) || !regularFile(pgctl) {
		t.Fatalf("PostgreSQL 16 test binaries unavailable; set BIZDECIPHER_TEST_POSTGRES_BIN")
	}
	t.Logf("using isolated PostgreSQL binaries from %s", binDir)
	dataRoot := strings.TrimSpace(os.Getenv("BIZDECIPHER_TEST_POSTGRES_DATA_ROOT"))
	if dataRoot == "" && runtime.GOOS == "windows" {
		dataRoot = `C:\ProgramData\CodexTemp`
	}
	tempDir, err := os.MkdirTemp(dataRoot, "workbench-pg-")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(tempDir)) })
	dataDir := filepath.Join(tempDir, "data")
	output, err := exec.Command(initdb, "-D", dataDir, "-A", "trust", "-U", "postgres", "--no-locale", "--encoding=UTF8").CombinedOutput()
	if err != nil {
		t.Fatalf("PostgreSQL initdb unavailable: %v: %s", err, output)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	logPath := filepath.Join(filepath.Dir(dataDir), "postgres.log")
	err = exec.Command(pgctl, "-D", dataDir, "-l", logPath, "-o", fmt.Sprintf("-h 127.0.0.1 -p %d", port), "-w", "start").Run()
	if err != nil {
		logOutput, readErr := os.ReadFile(logPath)
		require.NoError(t, readErr)
		t.Fatalf("PostgreSQL server unavailable: %v: %s", err, logOutput)
	}
	t.Cleanup(func() {
		stopOutput, stopErr := exec.Command(pgctl, "-D", dataDir, "-m", "immediate", "-w", "stop").CombinedOutput()
		require.NoError(t, stopErr, string(stopOutput))
		t.Log("isolated PostgreSQL cleanup complete")
	})
	db, err := sql.Open("postgres", fmt.Sprintf("postgres://postgres@127.0.0.1:%d/postgres?sslmode=disable&connect_timeout=5", port))
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	t.Log("isolated PostgreSQL connection ready")
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func createWorkbenchPrerequisites(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		CREATE TABLE users (id BIGINT PRIMARY KEY);
		CREATE TABLE bizdecipher_accepted_price_quotes (quote_id TEXT PRIMARY KEY);
		CREATE TABLE canonical_usage_outbox (event_id TEXT PRIMARY KEY);
		CREATE TABLE bizdecipher_ledger_journals (id TEXT PRIMARY KEY);
		CREATE TABLE canonical_media_task_bindings (business_event_id TEXT PRIMARY KEY);
		INSERT INTO users (id) VALUES (1), (2);`)
	require.NoError(t, err)
}

func requireExecError(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	_, err := db.Exec(query, args...)
	require.Error(t, err)
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
