package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const capabilityAssetVersionColumns = `
	v.id, v.asset_id, v.version, v.runtime_kind, v.manifest,
	COALESCE(v.package_digest, ''), v.status, v.file_count, v.total_bytes,
	v.published_at, v.created_at, v.updated_at`

const capabilityAssetVersionColumnsUnqualified = `
	id, asset_id, version, runtime_kind, manifest,
	COALESCE(package_digest, ''), status, file_count, total_bytes,
	published_at, created_at, updated_at`

func scanCapabilityAssetVersion(row interface{ Scan(...any) error }) (*service.CapabilityAssetVersion, error) {
	var item service.CapabilityAssetVersion
	var manifest []byte
	var publishedAt sql.NullTime
	if err := row.Scan(
		&item.ID, &item.AssetID, &item.Version, &item.RuntimeKind, &manifest,
		&item.PackageDigest, &item.Status, &item.FileCount, &item.TotalBytes,
		&publishedAt, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Manifest = json.RawMessage(manifest)
	if publishedAt.Valid {
		value := publishedAt.Time
		item.PublishedAt = &value
	}
	return &item, nil
}

func (r *bizDecipherRepository) ListCapabilityAssetVersions(ctx context.Context, assetID int64) ([]service.CapabilityAssetVersion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+capabilityAssetVersionColumns+`
		FROM capability_asset_versions v
		WHERE v.asset_id = $1
		ORDER BY v.id DESC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]service.CapabilityAssetVersion, 0)
	for rows.Next() {
		item, err := scanCapabilityAssetVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) CreateCapabilityAssetVersion(
	ctx context.Context,
	assetID, ownerUserID int64,
	input service.CreateCapabilityAssetVersionInput,
) (*service.CapabilityAssetVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO capability_asset_versions (
			asset_id, owner_user_id, version, runtime_kind, manifest
		)
		SELECT a.id, $2, $3, $4, $5::jsonb
		FROM capability_assets a
		WHERE a.id = $1
		  AND a.user_id = $2
		  AND a.deleted_at IS NULL
		ON CONFLICT (asset_id, version) DO NOTHING
		RETURNING `+capabilityAssetVersionColumnsUnqualified,
		assetID, ownerUserID, input.Version, input.RuntimeKind, string(input.Manifest))
	item, err := scanCapabilityAssetVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCapabilityAssetPackageConflict
	}
	return item, err
}

func (r *bizDecipherRepository) PutCapabilityAssetFile(
	ctx context.Context,
	assetID, ownerUserID int64,
	version string,
	input service.PreparedCapabilityAssetFile,
) (_ *service.CapabilityAssetVersion, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	item, err := lockCapabilityAssetVersion(ctx, tx, assetID, ownerUserID, version)
	if err != nil {
		return nil, err
	}
	if item.Status != service.CapabilityAssetVersionStatusDraft {
		return nil, service.ErrCapabilityAssetPackageConflict
	}

	var existingBytes int64
	existingErr := tx.QueryRowContext(ctx, `
		SELECT byte_size
		FROM capability_asset_files
		WHERE version_id = $1 AND relative_path = $2`,
		item.ID, input.Path,
	).Scan(&existingBytes)
	if existingErr != nil && !errors.Is(existingErr, sql.ErrNoRows) {
		return nil, existingErr
	}
	if existingErr == nil {
		item.FileCount--
		item.TotalBytes -= existingBytes
	}
	if item.FileCount >= service.CapabilityAssetMaxFiles {
		return nil, fmt.Errorf("%w: too many files", service.ErrCapabilityAssetPackageInvalid)
	}
	if item.TotalBytes+int64(len(input.Content)) > service.CapabilityAssetMaxPackageSize {
		return nil, fmt.Errorf("%w: package exceeds size limit", service.ErrCapabilityAssetPackageInvalid)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO capability_asset_files (
			version_id, relative_path, content_type, byte_size, sha256, content
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (version_id, relative_path) DO UPDATE SET
			content_type = EXCLUDED.content_type,
			byte_size = EXCLUDED.byte_size,
			sha256 = EXCLUDED.sha256,
			content = EXCLUDED.content,
			updated_at = NOW()`,
		item.ID, input.Path, input.ContentType, len(input.Content), input.SHA256, input.Content,
	); err != nil {
		return nil, err
	}

	row := tx.QueryRowContext(ctx, `
		UPDATE capability_asset_versions v
		SET file_count = counts.file_count,
		    total_bytes = counts.total_bytes,
		    updated_at = NOW()
		FROM (
			SELECT COUNT(*)::integer AS file_count, COALESCE(SUM(byte_size), 0)::bigint AS total_bytes
			FROM capability_asset_files
			WHERE version_id = $1
		) counts
		WHERE v.id = $1
		RETURNING `+capabilityAssetVersionColumns, item.ID)
	updated, err := scanCapabilityAssetVersion(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *bizDecipherRepository) ImportCapabilityAssetPackageTx(
	ctx context.Context, assetID, ownerUserID int64,
	input service.CreateCapabilityAssetVersionInput,
	files []service.PreparedCapabilityAssetFile, finalize bool,
) (*service.CapabilityAssetVersion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	// The unique version key serializes concurrent creates; the row lock also
	// serializes import with the existing file/finalize/publish endpoints.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO capability_asset_versions (asset_id, owner_user_id, version, runtime_kind, manifest)
		SELECT id, $2, $3, $4, $5::jsonb FROM capability_assets
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		ON CONFLICT (asset_id, version) DO NOTHING`,
		assetID, ownerUserID, input.Version, input.RuntimeKind, string(input.Manifest))
	if err != nil {
		return nil, err
	}
	item, err := lockCapabilityAssetVersion(ctx, tx, assetID, ownerUserID, input.Version)
	if err != nil {
		return nil, err
	}
	if item.Status != service.CapabilityAssetVersionStatusDraft {
		return nil, service.ErrCapabilityAssetPackageConflict
	}
	var matches bool
	// PostgreSQL JSONB equality preserves numeric precision and ignores object
	// key order, unlike decoding into float64-based Go interface values.
	err = tx.QueryRowContext(ctx, `SELECT runtime_kind = $2::varchar AND manifest = $3::jsonb
		FROM capability_asset_versions WHERE id = $1`,
		item.ID, input.RuntimeKind, string(input.Manifest)).Scan(&matches)
	if err != nil {
		return nil, err
	}
	if !matches {
		return nil, fmt.Errorf("%w: version metadata differs", service.ErrCapabilityAssetPackageConflict)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM capability_asset_files WHERE version_id = $1`, item.ID); err != nil {
		return nil, err
	}
	var total int64
	digestFiles := make([]service.CapabilityAssetFile, 0, len(files))
	for _, file := range files {
		_, err = tx.ExecContext(ctx, `INSERT INTO capability_asset_files
			(version_id, relative_path, content_type, byte_size, sha256, content)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			item.ID, file.Path, file.ContentType, len(file.Content), file.SHA256, file.Content)
		if err != nil {
			return nil, err
		}
		total += int64(len(file.Content))
		digestFiles = append(digestFiles, service.CapabilityAssetFile{
			Path: file.Path, SHA256: file.SHA256, ByteSize: int64(len(file.Content)),
		})
	}
	status, digest := service.CapabilityAssetVersionStatusDraft, ""
	if finalize {
		status = service.CapabilityAssetVersionStatusReady
		digest = service.CapabilityAssetPackageDigest(digestFiles)
	}
	updated, err := scanCapabilityAssetVersion(tx.QueryRowContext(ctx, `
		UPDATE capability_asset_versions SET status = $2::varchar, package_digest = NULLIF($3, ''),
			file_count = $4, total_bytes = $5, updated_at = NOW()
		WHERE id = $1 RETURNING `+capabilityAssetVersionColumnsUnqualified,
		item.ID, status, digest, len(files), total))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *bizDecipherRepository) FinalizeCapabilityAssetVersion(
	ctx context.Context,
	assetID, ownerUserID int64,
	version string,
) (_ *service.CapabilityAssetVersion, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	item, err := lockCapabilityAssetVersion(ctx, tx, assetID, ownerUserID, version)
	if err != nil {
		return nil, err
	}
	if item.Status == service.CapabilityAssetVersionStatusReady || item.Status == service.CapabilityAssetVersionStatusPublished {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return item, nil
	}
	if item.Status != service.CapabilityAssetVersionStatusDraft {
		return nil, service.ErrCapabilityAssetPackageConflict
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT relative_path, content_type, byte_size, sha256
		FROM capability_asset_files
		WHERE version_id = $1
		ORDER BY relative_path`, item.ID)
	if err != nil {
		return nil, err
	}
	files := make([]service.CapabilityAssetFile, 0, item.FileCount)
	for rows.Next() {
		var file service.CapabilityAssetFile
		if err := rows.Scan(&file.Path, &file.ContentType, &file.ByteSize, &file.SHA256); err != nil {
			_ = rows.Close()
			return nil, err
		}
		files = append(files, file)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: package has no files", service.ErrCapabilityAssetPackageInvalid)
	}
	digest := service.CapabilityAssetPackageDigest(files)
	row := tx.QueryRowContext(ctx, `
		UPDATE capability_asset_versions
		SET status = 'ready',
		    package_digest = $2,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING `+capabilityAssetVersionColumnsUnqualified, item.ID, digest)
	updated, err := scanCapabilityAssetVersion(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *bizDecipherRepository) SetCapabilityAssetVersionStatus(
	ctx context.Context,
	assetID, ownerUserID int64,
	version, status string,
) (_ *service.CapabilityAssetVersion, err error) {
	if status != service.CapabilityAssetVersionStatusPublished && status != service.CapabilityAssetVersionStatusRevoked {
		return nil, fmt.Errorf("%w: unsupported status", service.ErrCapabilityAssetPackageInvalid)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	item, err := lockCapabilityAssetVersion(ctx, tx, assetID, ownerUserID, version)
	if err != nil {
		return nil, err
	}
	if item.Status == status {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return item, nil
	}
	if status == service.CapabilityAssetVersionStatusPublished && item.Status != service.CapabilityAssetVersionStatusReady {
		return nil, service.ErrCapabilityAssetPackageConflict
	}
	if status == service.CapabilityAssetVersionStatusRevoked &&
		item.Status != service.CapabilityAssetVersionStatusReady &&
		item.Status != service.CapabilityAssetVersionStatusPublished {
		return nil, service.ErrCapabilityAssetPackageConflict
	}
	row := tx.QueryRowContext(ctx, `
		UPDATE capability_asset_versions
		SET status = $2::varchar,
		    published_at = CASE WHEN $2::varchar = 'published'::varchar THEN COALESCE(published_at, NOW()) ELSE published_at END,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING `+capabilityAssetVersionColumnsUnqualified, item.ID, status)
	updated, err := scanCapabilityAssetVersion(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func (r *bizDecipherRepository) GetCapabilityAssetPackage(ctx context.Context, assetID int64, version string) (*service.CapabilityAssetPackage, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+capabilityAssetVersionColumns+`
		FROM capability_asset_versions v
		WHERE v.asset_id = $1 AND v.version = $2`, assetID, version)
	item, err := scanCapabilityAssetVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT relative_path, content_type, byte_size, sha256, content
		FROM capability_asset_files
		WHERE version_id = $1
		ORDER BY relative_path`, item.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := make([]service.CapabilityAssetFile, 0, item.FileCount)
	for rows.Next() {
		var file service.CapabilityAssetFile
		if err := rows.Scan(&file.Path, &file.ContentType, &file.ByteSize, &file.SHA256, &file.Content); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &service.CapabilityAssetPackage{Version: *item, Files: files}, nil
}

func (r *bizDecipherRepository) RecordCapabilityAssetDownload(ctx context.Context, versionID, userID int64) error {
	if versionID <= 0 || userID <= 0 {
		return service.ErrCapabilityAssetPackageForbidden
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO capability_asset_download_events (version_id, user_id)
		VALUES ($1, $2)`, versionID, userID)
	return err
}

func lockCapabilityAssetVersion(
	ctx context.Context,
	tx *sql.Tx,
	assetID, ownerUserID int64,
	version string,
) (*service.CapabilityAssetVersion, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT `+capabilityAssetVersionColumns+`
		FROM capability_asset_versions v
		JOIN capability_assets a ON a.id = v.asset_id
		WHERE v.asset_id = $1
		  AND v.owner_user_id = $2
		  AND v.version = $3
		  AND a.user_id = $2
		  AND a.deleted_at IS NULL
		FOR UPDATE OF v`, assetID, ownerUserID, version)
	item, err := scanCapabilityAssetVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCapabilityAssetPackageNotFound
	}
	return item, err
}
