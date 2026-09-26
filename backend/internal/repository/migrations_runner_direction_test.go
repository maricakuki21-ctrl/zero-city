package repository

import (
	"context"
	"database/sql"
	"io/fs"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMigrationUpSQL(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "ordinary SQL stays byte for byte unchanged",
			content: " \nCREATE TABLE keep_me(id BIGINT);\n-- regular comment\n ",
			want:    " \nCREATE TABLE keep_me(id BIGINT);\n-- regular comment\n ",
		},
		{
			name:    "Up only",
			content: "-- +goose Up\nCREATE TABLE keep_me(id BIGINT);",
			want:    "-- +goose Up\nCREATE TABLE keep_me(id BIGINT);",
		},
		{
			name:    "Up Down retains complete forward SQL and preamble",
			content: "-- migration notes\n-- +goose Up\nCREATE TABLE keep_me(id BIGINT);\nCREATE INDEX keep_idx ON keep_me(id);\n\n-- +goose Down\nDROP TABLE keep_me;",
			want:    "-- migration notes\n-- +goose Up\nCREATE TABLE keep_me(id BIGINT);\nCREATE INDEX keep_idx ON keep_me(id);",
		},
		{
			name:    "statement annotations and Windows line endings",
			content: "-- +goose Up\r\n-- +goose StatementBegin\r\nSELECT 1;\r\n-- +goose StatementEnd\r\n-- +goose Down\r\n-- +goose StatementBegin\r\nSELECT 2;\r\n-- +goose StatementEnd",
			want:    "-- +goose Up\r\n-- +goose StatementBegin\r\nSELECT 1;\r\n-- +goose StatementEnd",
		},
		{
			name:    "single quoted marker is SQL data",
			content: "-- +goose Up\nSELECT 'escaped '' quote\n-- +goose Down\nstill data';\n-- +goose Down\nDROP TABLE keep_me;",
			want:    "-- +goose Up\nSELECT 'escaped '' quote\n-- +goose Down\nstill data';",
		},
		{
			name:    "double quoted identifier is not a marker",
			content: "-- +goose Up\nSELECT \"identifier\n-- +goose Down\nname\";\n-- +goose Down\nDROP TABLE keep_me;",
			want:    "-- +goose Up\nSELECT \"identifier\n-- +goose Down\nname\";",
		},
		{
			name:    "dollar quoted body is not parsed as annotation",
			content: "-- +goose Up\nDO $body$\nBEGIN\n-- +goose Down\nPERFORM 1;\nEND;\n$body$;\n-- +goose Down\nDROP TABLE keep_me;",
			want:    "-- +goose Up\nDO $body$\nBEGIN\n-- +goose Down\nPERFORM 1;\nEND;\n$body$;",
		},
		{
			name:    "nested block comments preserve annotation-like text",
			content: "-- +goose Up\n/* outer /* nested */\n-- +goose Down\n*/\nSELECT 1;\n-- +goose Down\nSELECT 2;",
			want:    "-- +goose Up\n/* outer /* nested */\n-- +goose Down\n*/\nSELECT 1;",
		},
		{
			name:    "PostgreSQL escape string",
			content: "-- +goose Up\nSELECT E'escaped \\' quote\n-- +goose Down\nstill data';\n-- +goose Down\nSELECT 2;",
			want:    "-- +goose Up\nSELECT E'escaped \\' quote\n-- +goose Down\nstill data';",
		},
		{
			name:    "unannotated SQL containing marker text stays unchanged",
			content: "SELECT $$\n-- +goose Down\n$$;",
			want:    "SELECT $$\n-- +goose Down\n$$;",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := migrationUpSQL(tt.content)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMigrationUpSQL_RejectsMalformedDirectionBeforeExecution(t *testing.T) {
	for name, content := range map[string]string{
		"Down before Up":          "-- +goose Down\nDROP TABLE keep_me;",
		"duplicate Up":            "-- +goose Up\nSELECT 1;\n-- +goose Up\nSELECT 2;",
		"duplicate Down":          "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 2;\n-- +goose Down",
		"SQL before Up":           "SELECT 1;\n-- +goose Up\nSELECT 2;",
		"direction has arguments": "-- +goose Up trailing\nSELECT 1;",
		"empty annotation":        "-- +goose\nSELECT 1;",
		"unknown annotation":      "-- +goose Up\n-- +goose Ups\nSELECT 1;",
		"unsupported mode":        "-- +goose NO TRANSACTION\n-- +goose Up\nSELECT 1;",
		"inline annotation":       "-- +goose Up\nSELECT 1; -- +goose Down\nDROP TABLE keep_me;",
		"block before direction":  "-- +goose StatementBegin\nSELECT 1;",
		"unmatched block end":     "-- +goose Up\n-- +goose StatementEnd",
		"nested statement block":  "-- +goose Up\n-- +goose StatementBegin\n-- +goose StatementBegin",
		"unclosed statement":      "-- +goose Up\n-- +goose StatementBegin\nSELECT 1;",
		"Down inside statement":   "-- +goose Up\n-- +goose StatementBegin\nSELECT 1;\n-- +goose Down",
		"unterminated quote":      "-- +goose Up\nSELECT 'open\n-- +goose Down",
		"unterminated dollar":     "-- +goose Up\nDO $body$\n-- +goose Down",
		"unterminated comment":    "-- +goose Up\n/*\n-- +goose Down",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := migrationUpSQL(content)
			require.Error(t, err)
		})
	}
}

func TestMigrationUpSQL_EmbeddedMigration240CannotExecuteDown(t *testing.T) {
	content, err := migrations.FS.ReadFile("240_canonical_usage_outbox.sql")
	require.NoError(t, err)
	up, err := migrationUpSQL(strings.TrimSpace(string(content)))
	require.NoError(t, err)
	require.Contains(t, up, "CREATE TABLE IF NOT EXISTS canonical_usage_outbox")
	require.Contains(t, up, "CREATE INDEX IF NOT EXISTS idx_canonical_usage_outbox_pending")
	require.NotContains(t, up, "DROP TABLE")
	require.NotContains(t, up, "-- +goose Down")
}

func TestMigrationUpSQL_AllEmbeddedFilesRetainUnmarkedContent(t *testing.T) {
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			content, err := migrations.FS.ReadFile(name)
			require.NoError(t, err)
			original := strings.TrimSpace(string(content))
			up, err := migrationUpSQL(original)
			require.NoError(t, err)
			if !strings.Contains(original, "-- +goose Up") {
				require.Equal(t, original, up)
			}
		})
	}
}

func TestApplyMigrationsFS_GooseExecutesOnlyUpAndRecordsCompleteChecksum(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	const name = "001_direction.sql"
	const up = "-- +goose Up\nCREATE TABLE keep_me(id BIGINT);"
	const content = up + "\n\n-- +goose Down\nDROP TABLE keep_me;"
	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(name).WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec("^" + regexp.QuoteMeta(strings.Join(strings.Fields(up), " ")) + "$").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations").
		WithArgs(name, migrationChecksum(content)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	expectMigrationsSessionCleanup(mock)
	err = applyMigrationsFS(context.Background(), db, fstest.MapFS{name: {Data: []byte(content)}})
	require.NoError(t, err)
	require.NotEqual(t, migrationChecksum(up), migrationChecksum(content))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_GooseInvalidDirectionDoesNotBeginOrRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	const name = "001_bad_direction.sql"
	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(name).WillReturnError(sql.ErrNoRows)
	expectMigrationsSessionCleanup(mock)
	err = applyMigrationsFS(context.Background(), db, fstest.MapFS{
		name: {Data: []byte("-- +goose Down\nDROP TABLE keep_me;")},
	})
	require.ErrorContains(t, err, "parse migration "+name)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_GooseNotxValidatesAndExecutesOnlyUp(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	const name = "001_direction_notx.sql"
	const up = "-- +goose Up\nCREATE INDEX CONCURRENTLY IF NOT EXISTS keep_idx ON keep_me(id);"
	const content = up + "\n-- +goose Down\nBEGIN;\nDROP TABLE keep_me;\nCOMMIT;"
	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(name).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("^" + regexp.QuoteMeta(strings.TrimSuffix(strings.Join(strings.Fields(up), " "), ";")) + "$").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations").
		WithArgs(name, migrationChecksum(content)).WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsSessionCleanup(mock)
	err = applyMigrationsFS(context.Background(), db, fstest.MapFS{name: {Data: []byte(content)}})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMigrationUpChecksumCompatibility_ExactHistorical019Prefix(t *testing.T) {
	content, err := migrations.FS.ReadFile("019_migrate_wechat_to_attributes.sql")
	require.NoError(t, err)
	original := strings.TrimSpace(string(content))
	const historical = "142811c782d3ff3a6662a2e200bcd357c4ff4bcea59572111eb436f19d834da9"
	require.Equal(t, "d45e05b4bb722b287377790583c2677b8666dbf7e02b626c93468491d4ce8cf8", migrationChecksum(original))
	require.True(t, isMigrationUpChecksumCompatible(original, historical))
	require.False(t, isMigrationUpChecksumCompatible(strings.Replace(original, "SET display_order = -1", "SET display_order = -2", 1), historical))
	require.False(t, isMigrationUpChecksumCompatible(original, strings.Repeat("0", 64)))
	require.False(t, isMigrationUpChecksumCompatible("SELECT 1;", historical))
	require.False(t, isMigrationUpChecksumCompatible("-- +goose Down\nSELECT 1;", historical))
}

func TestMigrationUpChecksumCompatibility_AllObservedUpOnlyDeployments(t *testing.T) {
	for name, checksum := range map[string]string{
		"019_migrate_wechat_to_attributes.sql": "142811c782d3ff3a6662a2e200bcd357c4ff4bcea59572111eb436f19d834da9",
		"024_add_gemini_tier_id.sql":           "a5f4d870a0225af2e5b38188b2ea20705a2a222839cf78a85e2657738b38ff8d",
		"037_ops_alert_silences.sql":           "38ed6841314a12e31d394bef5658f7b47e0b1a1cfe6583fb1a224b7627a126bd",
		"240_canonical_usage_outbox.sql":       "0906007b02bf551753489973757f736aaf569f32110a9e0b2fae876406442b83",
	} {
		t.Run(name, func(t *testing.T) {
			content, err := migrations.FS.ReadFile(name)
			require.NoError(t, err)
			require.True(t, isMigrationUpChecksumCompatible(strings.TrimSpace(string(content)), checksum))
		})
	}
}

func TestApplyMigrationsFS_GooseHistoricalUpChecksumIsReadOnly(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	const name = "001_direction.sql"
	const up = "-- +goose Up\nCREATE TABLE keep_me(id BIGINT);"
	const content = up + "\n-- +goose Down\nDROP TABLE keep_me;"
	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(name).WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(migrationChecksum(up)))
	expectMigrationsSessionCleanup(mock)
	err = applyMigrationsFS(context.Background(), db, fstest.MapFS{name: {Data: []byte(content)}})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
