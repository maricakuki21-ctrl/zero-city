package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
)

type result struct {
	DatabaseVersion   string         `json:"database_version"`
	MigrationSHA256   string         `json:"migration_sha256"`
	DispositionCounts map[string]int `json:"disposition_counts"`
	CanonicalAccounts int            `json:"canonical_accounts"`
	CanonicalGroups   int            `json:"canonical_groups"`
	RepeatApplyStable bool           `json:"repeat_apply_stable"`
	SourceRowsStable  bool           `json:"source_rows_stable"`
	SecretsAbsent     bool           `json:"secrets_absent_from_audit_schema"`
	MutationRejected  bool           `json:"audit_mutation_rejected"`
}

func main() {
	dsn := flag.String("dsn", "", "")
	migrationPath := flag.String("migration", "", "")
	schemaPath := flag.String("schema", "", "")
	fixturesPath := flag.String("fixtures", "", "")
	outputPath := flag.String("output", "", "")
	flag.Parse()
	if *dsn == "" || *migrationPath == "" || *schemaPath == "" || *fixturesPath == "" || *outputPath == "" {
		panic("all task6 QA arguments are required")
	}
	if err := run(*dsn, *migrationPath, *schemaPath, *fixturesPath, *outputPath); err != nil {
		panic(err)
	}
}

func run(dsn, migrationPath, schemaPath, fixturesPath, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("open task6 database: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping task6 database: %w", err)
	}
	for _, path := range []string{schemaPath, fixturesPath} {
		if err := execFile(ctx, db, path); err != nil {
			return err
		}
	}
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		return fmt.Errorf("apply migration first pass: %w", err)
	}
	firstCounts, err := dispositionCounts(ctx, db)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, string(migration)); err != nil {
		return fmt.Errorf("apply migration second pass: %w", err)
	}
	secondCounts, err := dispositionCounts(ctx, db)
	if err != nil {
		return err
	}
	want := map[string]int{
		"invalid/invalid_credential":          2,
		"manual-review/cross_pool_reuse":      2,
		"manual-review/model_alias_collision": 1,
		"mapped/canonicalized":                2,
		"quarantined/duplicate_account":       1,
		"quarantined/orphan_owner":            1,
		"quarantined/orphan_pool":             1,
	}
	if !mapsEqual(secondCounts, want) {
		return fmt.Errorf("unexpected dispositions: got=%v want=%v", secondCounts, want)
	}
	var version string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version); err != nil {
		return fmt.Errorf("read postgres version: %w", err)
	}
	canonicalAccounts, err := scalar(ctx, db, "SELECT COUNT(*) FROM shared_pool_supply_dispositions WHERE disposition = 'mapped' AND canonical_account_id IS NOT NULL")
	if err != nil {
		return err
	}
	canonicalGroups, err := scalar(ctx, db, "SELECT COUNT(*) FROM shared_pool_sub2_bindings WHERE lifecycle = 'active'")
	if err != nil {
		return err
	}
	stableSources, err := scalar(ctx, db, "SELECT COUNT(*) FROM shared_pool_accounts WHERE upstream_api_key <> '' OR credentials_encrypted <> ''")
	if err != nil {
		return err
	}
	secretColumns, err := scalar(ctx, db, `SELECT COUNT(*) FROM information_schema.columns WHERE table_name IN ('shared_pool_sub2_bindings','shared_pool_supply_dispositions') AND column_name ~ '(api_key|credential|secret|concurrency|health|calls)' AND column_name NOT IN ('credential_hash')`)
	if err != nil {
		return err
	}
	_, mutationErr := db.ExecContext(ctx, "UPDATE shared_pool_supply_dispositions SET reason_code = 'changed' WHERE source_id = 101")
	if mutationErr == nil {
		return errors.New("append-only audit accepted mutation")
	}
	sum := sha256.Sum256(migration)
	receipt := result{
		DatabaseVersion: version, MigrationSHA256: hex.EncodeToString(sum[:]),
		DispositionCounts: secondCounts, CanonicalAccounts: canonicalAccounts,
		CanonicalGroups: canonicalGroups, RepeatApplyStable: mapsEqual(firstCounts, secondCounts),
		SourceRowsStable: stableSources == 9, SecretsAbsent: secretColumns == 0,
		MutationRejected: true,
	}
	if !receipt.RepeatApplyStable || !receipt.SourceRowsStable || !receipt.SecretsAbsent || canonicalAccounts != 2 || canonicalGroups != 2 {
		return fmt.Errorf("task6 invariant failed: %+v", receipt)
	}
	payload, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("encode receipt: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create evidence directory: %w", err)
	}
	if err := os.WriteFile(outputPath, append(payload, '\n'), 0o600); err != nil {
		return fmt.Errorf("write receipt: %w", err)
	}
	return nil
}

func execFile(ctx context.Context, db *sql.DB, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if _, err := db.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("execute %s: %w", path, err)
	}
	return nil
}

func dispositionCounts(ctx context.Context, db *sql.DB) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, "SELECT disposition || '/' || reason_code, COUNT(*) FROM shared_pool_supply_dispositions GROUP BY disposition, reason_code")
	if err != nil {
		return nil, fmt.Errorf("read dispositions: %w", err)
	}
	defer rows.Close()
	counts := make(map[string]int)
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return nil, fmt.Errorf("scan disposition: %w", err)
		}
		counts[key] = count
	}
	return counts, rows.Err()
}

func scalar(ctx context.Context, db *sql.DB, query string) (int, error) {
	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("scalar query: %w", err)
	}
	return count, nil
}

func mapsEqual(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
