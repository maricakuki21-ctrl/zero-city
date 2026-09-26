package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	TavernGamePackageStatusDraft     = "draft"
	TavernGamePackageStatusPublished = "published"
	TavernGamePackageStatusRevoked   = "revoked"

	TavernGamePackageSchemaV1     = "tavern.package.v1"
	TavernGamePackageProtocolV1   = "2026-09-13.package.v1"
	TavernGamePackageRuntime      = "declarative"
	TavernGamePackageManifestMax  = 256 << 10
	TavernGamePackageContentMax   = 200 << 10
	TavernGamePackageMaxMaxTurns  = 500
	TavernGamePackageMaxScenesMax = 100
)

var (
	ErrTavernGamePackageNotFound  = errors.New("tavern game package not found")
	ErrTavernGamePackageForbidden = errors.New("tavern game package operation forbidden")
	ErrTavernGamePackageConflict  = errors.New("tavern game package state conflict")
	ErrTavernGamePackageInvalid   = errors.New("tavern game package is invalid")
	ErrTavernGamePackageUnavailable = errors.New("tavern game package is unavailable")
)

type TavernGamePackageEntry struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type TavernGamePackagePermissions struct {
	AIGateway bool `json:"ai_gateway"`
	Save      bool `json:"save"`
	Score     bool `json:"score"`
	Purchases bool `json:"purchases"`
	Presence  bool `json:"presence"`
}

type TavernGamePackageLimits struct {
	MaxTurns  int `json:"max_turns"`
	MaxScenes int `json:"max_scenes"`
}

type TavernGamePackageManifest struct {
	SchemaVersion   string                       `json:"schema_version"`
	RuntimeKind     string                       `json:"runtime_kind"`
	ProtocolVersion string                       `json:"protocol_version"`
	Entry           TavernGamePackageEntry       `json:"entry"`
	Permissions     TavernGamePackagePermissions `json:"permissions"`
	Content         json.RawMessage              `json:"content"`
	Limits          TavernGamePackageLimits      `json:"limits"`
}

type TavernGamePackageInput struct {
	Version  string          `json:"version"`
	Manifest json.RawMessage `json:"manifest"`
}

type TavernGamePackage struct {
	ID              int64                    `json:"id"`
	ScriptID        int64                    `json:"script_id"`
	OwnerUserID     int64                    `json:"owner_user_id"`
	Version         string                   `json:"version"`
	SchemaVersion   string                   `json:"schema_version"`
	RuntimeKind     string                   `json:"runtime_kind"`
	ProtocolVersion string                   `json:"protocol_version"`
	Manifest        TavernGamePackageManifest `json:"manifest"`
	PackageDigest   string                   `json:"package_digest,omitempty"`
	Status          string                   `json:"status"`
	PublishedAt     *time.Time               `json:"published_at,omitempty"`
	CreatedAt       time.Time                `json:"created_at"`
	UpdatedAt       time.Time                `json:"updated_at"`
}

func (s *BizDecipherService) ListTavernGamePackages(ctx context.Context, scriptID, viewerUserID int64) ([]TavernGamePackage, error) {
	if scriptID <= 0 {
		return nil, ErrTavernGamePackageInvalid
	}
	script, err := s.repo.GetTavernScript(ctx, scriptID, true)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, ErrTavernGamePackageNotFound
	}
	includeUnpublished := viewerUserID > 0 && script.UserID == viewerUserID
	if script.Status != TavernScriptStatusListed && !includeUnpublished {
		return nil, ErrTavernGamePackageNotFound
	}
	return s.repo.ListTavernGamePackages(ctx, scriptID, includeUnpublished)
}

func (s *BizDecipherService) CreateTavernGamePackage(
	ctx context.Context,
	scriptID, ownerUserID int64,
	input TavernGamePackageInput,
) (*TavernGamePackage, error) {
	if ownerUserID <= 0 {
		return nil, ErrTavernGamePackageForbidden
	}
	script, err := s.repo.GetTavernScript(ctx, scriptID, true)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, ErrTavernGamePackageNotFound
	}
	if script.UserID != ownerUserID {
		return nil, ErrTavernGamePackageForbidden
	}
	version, manifest, err := normalizeTavernGamePackageInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateTavernGamePackage(ctx, scriptID, ownerUserID, version, manifest)
}

func (s *BizDecipherService) SetTavernGamePackageStatus(
	ctx context.Context,
	scriptID, ownerUserID int64,
	version, status string,
) (*TavernGamePackage, error) {
	if ownerUserID <= 0 {
		return nil, ErrTavernGamePackageForbidden
	}
	version = strings.TrimSpace(version)
	if !capabilityAssetVersionPattern.MatchString(version) {
		return nil, fmt.Errorf("%w: invalid version", ErrTavernGamePackageInvalid)
	}
	switch status {
	case TavernGamePackageStatusPublished, TavernGamePackageStatusRevoked:
	default:
		return nil, fmt.Errorf("%w: invalid status", ErrTavernGamePackageInvalid)
	}
	script, err := s.repo.GetTavernScript(ctx, scriptID, true)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, ErrTavernGamePackageNotFound
	}
	if script.UserID != ownerUserID {
		return nil, ErrTavernGamePackageForbidden
	}
	return s.repo.SetTavernGamePackageStatus(ctx, scriptID, ownerUserID, version, status)
}

func (s *BizDecipherService) PublishTavernGamePackage(ctx context.Context, scriptID, ownerUserID int64, version string) (*TavernGamePackage, error) {
	return s.SetTavernGamePackageStatus(ctx, scriptID, ownerUserID, version, TavernGamePackageStatusPublished)
}

func (s *BizDecipherService) RevokeTavernGamePackage(ctx context.Context, scriptID, ownerUserID int64, version string) (*TavernGamePackage, error) {
	return s.SetTavernGamePackageStatus(ctx, scriptID, ownerUserID, version, TavernGamePackageStatusRevoked)
}

func normalizeTavernGamePackageInput(input TavernGamePackageInput) (string, TavernGamePackageManifest, error) {
	input.Version = strings.TrimSpace(input.Version)
	if !capabilityAssetVersionPattern.MatchString(input.Version) {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: invalid version", ErrTavernGamePackageInvalid)
	}
	if len(input.Manifest) == 0 || len(input.Manifest) > TavernGamePackageManifestMax {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: invalid manifest size", ErrTavernGamePackageInvalid)
	}
	var manifest TavernGamePackageManifest
	decoder := json.NewDecoder(bytes.NewReader(input.Manifest))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: %v", ErrTavernGamePackageInvalid, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: trailing manifest data", ErrTavernGamePackageInvalid)
	}
	manifest.SchemaVersion = strings.TrimSpace(manifest.SchemaVersion)
	if manifest.SchemaVersion == "" {
		manifest.SchemaVersion = TavernGamePackageSchemaV1
	}
	if manifest.SchemaVersion != TavernGamePackageSchemaV1 {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: unsupported schema version", ErrTavernGamePackageInvalid)
	}
	manifest.RuntimeKind = strings.TrimSpace(manifest.RuntimeKind)
	if manifest.RuntimeKind == "" {
		manifest.RuntimeKind = TavernGamePackageRuntime
	}
	if manifest.RuntimeKind != TavernGamePackageRuntime {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: unsupported runtime kind", ErrTavernGamePackageInvalid)
	}
	manifest.ProtocolVersion = strings.TrimSpace(manifest.ProtocolVersion)
	if manifest.ProtocolVersion == "" {
		manifest.ProtocolVersion = TavernGamePackageProtocolV1
	}
	if manifest.ProtocolVersion != TavernGamePackageProtocolV1 {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: unsupported protocol version", ErrTavernGamePackageInvalid)
	}
	manifest.Entry.Kind = strings.TrimSpace(manifest.Entry.Kind)
	if manifest.Entry.Kind == "" {
		manifest.Entry.Kind = "prompt_flow"
	}
	switch manifest.Entry.Kind {
	case "prompt_flow", "scene_graph":
	default:
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: invalid entry kind", ErrTavernGamePackageInvalid)
	}
	manifest.Entry.Ref = truncateCapabilityString(strings.TrimSpace(manifest.Entry.Ref), 160)
	if manifest.Entry.Ref == "" {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: entry ref is required", ErrTavernGamePackageInvalid)
	}
	if manifest.Permissions.Purchases {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: purchase permission is not available", ErrTavernGamePackageInvalid)
	}
	if len(manifest.Content) == 0 {
		manifest.Content = json.RawMessage(`{}`)
	}
	if len(manifest.Content) > TavernGamePackageContentMax || !json.Valid(manifest.Content) {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: invalid content", ErrTavernGamePackageInvalid)
	}
	var content any
	if err := json.Unmarshal(manifest.Content, &content); err != nil {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: invalid content", ErrTavernGamePackageInvalid)
	}
	if _, ok := content.(map[string]any); !ok {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: content must be an object", ErrTavernGamePackageInvalid)
	}
	if err := validateTavernPackageContent(content, ""); err != nil {
		return "", TavernGamePackageManifest{}, err
	}
	if manifest.Limits.MaxTurns <= 0 {
		manifest.Limits.MaxTurns = 48
	}
	if manifest.Limits.MaxTurns > TavernGamePackageMaxMaxTurns {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: max_turns exceeds limit", ErrTavernGamePackageInvalid)
	}
	if manifest.Limits.MaxScenes <= 0 {
		manifest.Limits.MaxScenes = 12
	}
	if manifest.Limits.MaxScenes > TavernGamePackageMaxScenesMax {
		return "", TavernGamePackageManifest{}, fmt.Errorf("%w: max_scenes exceeds limit", ErrTavernGamePackageInvalid)
	}
	return input.Version, manifest, nil
}

func validateTavernPackageContent(value any, path string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			normalized := strings.ToLower(strings.TrimSpace(key))
			switch normalized {
			case "password", "secret", "token", "api_key", "apikey", "authorization", "cookie", "private_key":
				return fmt.Errorf("%w: forbidden content key %q", ErrTavernGamePackageInvalid, key)
			}
			nextPath := key
			if path != "" {
				nextPath = path + "." + key
			}
			if err := validateTavernPackageContent(child, nextPath); err != nil {
				return err
			}
		}
	case []any:
		for index, child := range typed {
			if err := validateTavernPackageContent(child, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	case string:
		if len(typed) > 4000 {
			return fmt.Errorf("%w: content string at %s is too long", ErrTavernGamePackageInvalid, path)
		}
	}
	return nil
}

func TavernGamePackageDigest(manifest TavernGamePackageManifest) (string, error) {
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func (s *BizDecipherService) GetTavernGamePackageForRoom(ctx context.Context, packageID int64) (*TavernGamePackage, error) {
	if packageID <= 0 {
		return nil, ErrTavernGamePackageNotFound
	}
	pkg, err := s.repo.GetTavernGamePackage(ctx, packageID)
	if err != nil {
		return nil, err
	}
	if !tavernGamePackagePublishedForRoom(pkg) {
		return nil, ErrTavernGamePackageUnavailable
	}
	return pkg, nil
}

func tavernGamePackagePublishedForRoom(pkg *TavernGamePackage) bool {
	return pkg != nil && pkg.Status == TavernGamePackageStatusPublished
}

func tavernGamePackageNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrTavernGamePackageNotFound)
}
