package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const tavernGamePackageColumns = `
	p.id, p.script_id, p.owner_user_id, p.version,
	p.schema_version, p.runtime_kind, p.protocol_version, p.manifest,
	COALESCE(p.package_digest, ''), p.status, p.published_at,
	p.created_at, p.updated_at`

const tavernGamePackageColumnsUnqualified = `
	id, script_id, owner_user_id, version,
	schema_version, runtime_kind, protocol_version, manifest,
	COALESCE(package_digest, ''), status, published_at,
	created_at, updated_at`

func scanTavernGamePackage(row interface{ Scan(...any) error }) (*service.TavernGamePackage, error) {
	var pkg service.TavernGamePackage
	var manifestRaw []byte
	var publishedAt sql.NullTime
	if err := row.Scan(
		&pkg.ID, &pkg.ScriptID, &pkg.OwnerUserID, &pkg.Version,
		&pkg.SchemaVersion, &pkg.RuntimeKind, &pkg.ProtocolVersion, &manifestRaw,
		&pkg.PackageDigest, &pkg.Status, &publishedAt,
		&pkg.CreatedAt, &pkg.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(manifestRaw, &pkg.Manifest); err != nil {
		return nil, err
	}
	if publishedAt.Valid {
		value := publishedAt.Time
		pkg.PublishedAt = &value
	}
	return &pkg, nil
}

func (r *bizDecipherRepository) ListTavernGamePackages(ctx context.Context, scriptID int64, includeUnpublished bool) ([]service.TavernGamePackage, error) {
	statusClause := " AND p.status = 'published'"
	if includeUnpublished {
		statusClause = ""
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+tavernGamePackageColumns+`
		FROM tavern_game_packages p
		WHERE p.script_id = $1`+statusClause+`
		ORDER BY p.id DESC`, scriptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.TavernGamePackage, 0)
	for rows.Next() {
		pkg, err := scanTavernGamePackage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *pkg)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) GetTavernGamePackage(ctx context.Context, packageID int64) (*service.TavernGamePackage, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+tavernGamePackageColumns+`
		FROM tavern_game_packages p
		WHERE p.id = $1`, packageID)
	pkg, err := scanTavernGamePackage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

func (r *bizDecipherRepository) GetLatestPublishedTavernGamePackage(ctx context.Context, scriptID int64) (*service.TavernGamePackage, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+tavernGamePackageColumns+`
		FROM tavern_game_packages p
		WHERE p.script_id = $1 AND p.status = 'published'
		ORDER BY p.id DESC
		LIMIT 1`, scriptID)
	pkg, err := scanTavernGamePackage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

func (r *bizDecipherRepository) CreateTavernGamePackage(
	ctx context.Context,
	scriptID, ownerUserID int64,
	version string,
	manifest service.TavernGamePackageManifest,
) (*service.TavernGamePackage, error) {
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	digest, err := service.TavernGamePackageDigest(manifest)
	if err != nil {
		return nil, err
	}
	var packageID int64
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO tavern_game_packages (
			script_id, owner_user_id, version, schema_version, runtime_kind,
			protocol_version, manifest, package_digest, status
		)
		SELECT s.id, $2, $3, $4, $5, $6, $7::jsonb, $8, 'draft'
		FROM tavern_scripts s
		WHERE s.id = $1
		  AND s.user_id = $2
		  AND s.deleted_at IS NULL
		ON CONFLICT (script_id, version) DO NOTHING
		RETURNING id`,
		scriptID, ownerUserID, version,
		manifest.SchemaVersion, manifest.RuntimeKind, manifest.ProtocolVersion,
		string(raw), digest,
	).Scan(&packageID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrTavernGamePackageConflict
	}
	if err != nil {
		return nil, err
	}
	return r.GetTavernGamePackage(ctx, packageID)
}

func (r *bizDecipherRepository) SetTavernGamePackageStatus(
	ctx context.Context,
	scriptID, ownerUserID int64,
	version, status string,
) (*service.TavernGamePackage, error) {
	if status != service.TavernGamePackageStatusPublished && status != service.TavernGamePackageStatusRevoked {
		return nil, service.ErrTavernGamePackageInvalid
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE tavern_game_packages p
		SET status = $4::varchar,
		    published_at = CASE
		        WHEN $4::varchar = 'published' THEN COALESCE(p.published_at, NOW())
		        ELSE p.published_at
		    END,
		    updated_at = NOW()
		WHERE p.script_id = $1
		  AND p.owner_user_id = $2
		  AND p.version = $3
		  AND (
		      p.status = $4::varchar
		      OR (p.status = 'draft' AND $4::varchar = 'published')
		      OR (p.status = 'published' AND $4::varchar = 'revoked')
		  )
		RETURNING `+tavernGamePackageColumnsUnqualified,
		scriptID, ownerUserID, version, status)
	pkg, err := scanTavernGamePackage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrTavernGamePackageConflict
	}
	if err != nil {
		return nil, err
	}
	return pkg, nil
}
