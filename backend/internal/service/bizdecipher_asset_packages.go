package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	CapabilityAssetVersionStatusDraft     = "draft"
	CapabilityAssetVersionStatusReady     = "ready"
	CapabilityAssetVersionStatusPublished = "published"
	CapabilityAssetVersionStatusRevoked   = "revoked"

	CapabilityAssetMaxFiles       = 64
	CapabilityAssetMaxFileBytes   = int64(8 << 20)
	CapabilityAssetMaxPackageSize = int64(32 << 20)
	CapabilityAssetMaxManifest    = 256 << 10
)

var (
	ErrCapabilityAssetPackageNotFound  = errors.New("capability asset package not found")
	ErrCapabilityAssetPackageForbidden = errors.New("capability asset package operation forbidden")
	ErrCapabilityAssetPackageConflict  = errors.New("capability asset package state conflict")
	ErrCapabilityAssetPackageInvalid   = errors.New("capability asset package is invalid")
)

var capabilityAssetVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type CapabilityAssetVersion struct {
	ID            int64           `json:"id"`
	AssetID       int64           `json:"asset_id"`
	Version       string          `json:"version"`
	RuntimeKind   string          `json:"runtime_kind"`
	Manifest      json.RawMessage `json:"manifest"`
	PackageDigest string          `json:"package_digest,omitempty"`
	Status        string          `json:"status"`
	FileCount     int             `json:"file_count"`
	TotalBytes    int64           `json:"total_bytes"`
	PublishedAt   *time.Time      `json:"published_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type CapabilityAssetFile struct {
	Path        string `json:"path"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
	SHA256      string `json:"sha256"`
	Content     []byte `json:"-"`
}

type CapabilityAssetPackage struct {
	Version CapabilityAssetVersion `json:"version"`
	Files   []CapabilityAssetFile  `json:"files"`
	Body    []byte                 `json:"-"`
}

type CreateCapabilityAssetVersionInput struct {
	Version     string          `json:"version"`
	RuntimeKind string          `json:"runtime_kind"`
	Manifest    json.RawMessage `json:"manifest"`
}

type CapabilityAssetFileInput struct {
	Path          string `json:"path"`
	ContentType   string `json:"content_type"`
	ContentBase64 string `json:"content_base64"`
}

// ImportCapabilityAssetPackageInput is the one-shot local/CLI ingestion
// contract: a version, its runtime kind, a manifest and the package files.
type ImportCapabilityAssetPackageInput struct {
	Version     string
	RuntimeKind string
	Manifest    json.RawMessage
	Files       []CapabilityAssetFileInput
	Finalize    bool
}

type PreparedCapabilityAssetFile struct {
	Path        string
	ContentType string
	Content     []byte
	SHA256      string
}

func normalizeCreateCapabilityAssetVersionInput(input CreateCapabilityAssetVersionInput) (CreateCapabilityAssetVersionInput, error) {
	input.Version = strings.TrimSpace(input.Version)
	input.RuntimeKind = strings.TrimSpace(input.RuntimeKind)
	if !capabilityAssetVersionPattern.MatchString(input.Version) || len(input.Version) > 64 {
		return input, fmt.Errorf("%w: invalid version", ErrCapabilityAssetPackageInvalid)
	}
	if input.RuntimeKind == "" || len(input.RuntimeKind) > 32 {
		return input, fmt.Errorf("%w: invalid runtime kind", ErrCapabilityAssetPackageInvalid)
	}
	if len(input.Manifest) == 0 {
		input.Manifest = json.RawMessage(`{}`)
	}
	if len(input.Manifest) > CapabilityAssetMaxManifest || !json.Valid(input.Manifest) {
		return input, fmt.Errorf("%w: invalid manifest", ErrCapabilityAssetPackageInvalid)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(input.Manifest, &object); err != nil || object == nil {
		return input, fmt.Errorf("%w: manifest must be an object", ErrCapabilityAssetPackageInvalid)
	}
	return input, nil
}

func prepareCapabilityAssetFile(input CapabilityAssetFileInput) (PreparedCapabilityAssetFile, error) {
	relative, err := normalizeCapabilityAssetPath(input.Path)
	if err != nil {
		return PreparedCapabilityAssetFile{}, err
	}
	input.ContentType = strings.TrimSpace(input.ContentType)
	if input.ContentType == "" {
		input.ContentType = "application/octet-stream"
	}
	if len(input.ContentType) > 160 {
		return PreparedCapabilityAssetFile{}, fmt.Errorf("%w: content type is too long", ErrCapabilityAssetPackageInvalid)
	}
	encoded := strings.TrimSpace(input.ContentBase64)
	if encoded == "" || int64(base64.StdEncoding.DecodedLen(len(encoded))) > CapabilityAssetMaxFileBytes+2 {
		return PreparedCapabilityAssetFile{}, fmt.Errorf("%w: invalid file size", ErrCapabilityAssetPackageInvalid)
	}
	content, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return PreparedCapabilityAssetFile{}, fmt.Errorf("%w: invalid base64 content", ErrCapabilityAssetPackageInvalid)
	}
	if int64(len(content)) > CapabilityAssetMaxFileBytes {
		return PreparedCapabilityAssetFile{}, fmt.Errorf("%w: file exceeds limit", ErrCapabilityAssetPackageInvalid)
	}
	digest := sha256.Sum256(content)
	return PreparedCapabilityAssetFile{
		Path:        relative,
		ContentType: input.ContentType,
		Content:     content,
		SHA256:      hex.EncodeToString(digest[:]),
	}, nil
}

func normalizeCapabilityAssetPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("%w: invalid file path", ErrCapabilityAssetPackageInvalid)
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != value || len(clean) > 512 {
		return "", fmt.Errorf("%w: invalid file path", ErrCapabilityAssetPackageInvalid)
	}
	for _, segment := range strings.Split(clean, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("%w: invalid file path", ErrCapabilityAssetPackageInvalid)
		}
	}
	return clean, nil
}

func CapabilityAssetPackageDigest(files []CapabilityAssetFile) string {
	sorted := append([]CapabilityAssetFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	hash := sha256.New()
	for _, file := range sorted {
		_, _ = hash.Write([]byte(file.Path))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(file.SHA256))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(fmt.Sprintf("%d", file.ByteSize)))
		_, _ = hash.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (s *BizDecipherService) ListCapabilityAssetVersions(ctx context.Context, assetID, viewerUserID int64) ([]CapabilityAssetVersion, error) {
	asset, err := s.requireCapabilityAssetPackageAccess(ctx, assetID, viewerUserID)
	if err != nil {
		return nil, err
	}
	versions, err := s.repo.ListCapabilityAssetVersions(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if asset.UserID == viewerUserID {
		return versions, nil
	}
	out := make([]CapabilityAssetVersion, 0, len(versions))
	for _, version := range versions {
		if version.Status == CapabilityAssetVersionStatusPublished {
			out = append(out, version)
		}
	}
	entitled, err := s.assetPackageEntitled(ctx, asset, viewerUserID)
	if err != nil {
		return nil, err
	}
	if !entitled {
		hidePaidAssetManifest(out)
	}
	return out, nil
}

func (s *BizDecipherService) CreateCapabilityAssetVersion(ctx context.Context, assetID, ownerUserID int64, input CreateCapabilityAssetVersionInput) (*CapabilityAssetVersion, error) {
	asset, err := s.requireCapabilityAssetPackageAccess(ctx, assetID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if asset.UserID != ownerUserID {
		return nil, ErrCapabilityAssetPackageForbidden
	}
	input, err = normalizeCreateCapabilityAssetVersionInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateCapabilityAssetVersion(ctx, assetID, ownerUserID, input)
}

func (s *BizDecipherService) PutCapabilityAssetFile(ctx context.Context, assetID, ownerUserID int64, version string, input CapabilityAssetFileInput) (*CapabilityAssetVersion, error) {
	if _, err := s.requireCapabilityAssetOwner(ctx, assetID, ownerUserID); err != nil {
		return nil, err
	}
	version = strings.TrimSpace(version)
	if !capabilityAssetVersionPattern.MatchString(version) {
		return nil, fmt.Errorf("%w: invalid version", ErrCapabilityAssetPackageInvalid)
	}
	prepared, err := prepareCapabilityAssetFile(input)
	if err != nil {
		return nil, err
	}
	return s.repo.PutCapabilityAssetFile(ctx, assetID, ownerUserID, version, prepared)
}

func (s *BizDecipherService) FinalizeCapabilityAssetVersion(ctx context.Context, assetID, ownerUserID int64, version string) (*CapabilityAssetVersion, error) {
	if _, err := s.requireCapabilityAssetOwner(ctx, assetID, ownerUserID); err != nil {
		return nil, err
	}
	return s.repo.FinalizeCapabilityAssetVersion(ctx, assetID, ownerUserID, strings.TrimSpace(version))
}

// ImportCapabilityAssetPackage validates all files before an atomic package
// replacement. The repository serializes writers to the same version.
func (s *BizDecipherService) ImportCapabilityAssetPackage(
	ctx context.Context,
	assetID, ownerUserID int64,
	input ImportCapabilityAssetPackageInput,
) (*CapabilityAssetVersion, error) {
	if _, err := s.requireCapabilityAssetOwner(ctx, assetID, ownerUserID); err != nil {
		return nil, err
	}
	versionInput, err := normalizeCreateCapabilityAssetVersionInput(CreateCapabilityAssetVersionInput{
		Version:     input.Version,
		RuntimeKind: input.RuntimeKind,
		Manifest:    input.Manifest,
	})
	if err != nil {
		return nil, err
	}
	if len(input.Files) == 0 {
		return nil, fmt.Errorf("%w: package has no files", ErrCapabilityAssetPackageInvalid)
	}
	if len(input.Files) > CapabilityAssetMaxFiles {
		return nil, fmt.Errorf("%w: package has too many files", ErrCapabilityAssetPackageInvalid)
	}
	prepared := make([]PreparedCapabilityAssetFile, 0, len(input.Files))
	seen := make(map[string]struct{}, len(input.Files))
	var totalBytes int64
	for _, file := range input.Files {
		item, err := prepareCapabilityAssetFile(file)
		if err != nil {
			label := strings.TrimSpace(file.Path)
			if label == "" {
				label = "unnamed"
			}
			return nil, fmt.Errorf("%w: file %q: %v", ErrCapabilityAssetPackageInvalid, label, err)
		}
		if _, exists := seen[item.Path]; exists {
			return nil, fmt.Errorf("%w: duplicate file path %q", ErrCapabilityAssetPackageInvalid, item.Path)
		}
		seen[item.Path] = struct{}{}
		totalBytes += int64(len(item.Content))
		prepared = append(prepared, item)
	}
	if totalBytes > CapabilityAssetMaxPackageSize {
		return nil, fmt.Errorf("%w: package exceeds size limit", ErrCapabilityAssetPackageInvalid)
	}

	return s.repo.ImportCapabilityAssetPackageTx(ctx, assetID, ownerUserID, versionInput, prepared, input.Finalize)
}

func (s *BizDecipherService) PublishCapabilityAssetVersion(ctx context.Context, assetID, ownerUserID int64, version string) (*CapabilityAssetVersion, error) {
	if _, err := s.requireCapabilityAssetOwner(ctx, assetID, ownerUserID); err != nil {
		return nil, err
	}
	return s.repo.SetCapabilityAssetVersionStatus(ctx, assetID, ownerUserID, strings.TrimSpace(version), CapabilityAssetVersionStatusPublished)
}

func (s *BizDecipherService) RevokeCapabilityAssetVersion(ctx context.Context, assetID, ownerUserID int64, version string) (*CapabilityAssetVersion, error) {
	if _, err := s.requireCapabilityAssetOwner(ctx, assetID, ownerUserID); err != nil {
		return nil, err
	}
	return s.repo.SetCapabilityAssetVersionStatus(ctx, assetID, ownerUserID, strings.TrimSpace(version), CapabilityAssetVersionStatusRevoked)
}

func (s *BizDecipherService) DownloadCapabilityAssetPackage(ctx context.Context, assetID, viewerUserID int64, version string) (*CapabilityAssetPackage, error) {
	asset, err := s.requireCapabilityAssetPackageAccess(ctx, assetID, viewerUserID)
	if err != nil {
		return nil, err
	}
	entitled, err := s.assetPackageEntitled(ctx, asset, viewerUserID)
	if err != nil {
		return nil, err
	}
	if !entitled {
		return nil, ErrCapabilityAssetPackageForbidden
	}
	pkg, err := s.repo.GetCapabilityAssetPackage(ctx, assetID, strings.TrimSpace(version))
	if err != nil {
		return nil, err
	}
	if pkg == nil || pkg.Version.Status == CapabilityAssetVersionStatusRevoked {
		return nil, ErrCapabilityAssetPackageNotFound
	}
	if asset.UserID != viewerUserID && pkg.Version.Status != CapabilityAssetVersionStatusPublished {
		return nil, ErrCapabilityAssetPackageForbidden
	}
	if pkg.Version.Status == CapabilityAssetVersionStatusDraft || pkg.Version.Status == CapabilityAssetVersionStatusReady && asset.UserID != viewerUserID {
		return nil, ErrCapabilityAssetPackageForbidden
	}
	if err := s.repo.RecordCapabilityAssetDownload(ctx, pkg.Version.ID, viewerUserID); err != nil {
		return nil, err
	}
	return pkg, nil
}

func capabilityAssetHasPublicPackageDelivery(pricingType string) bool {
	switch strings.TrimSpace(pricingType) {
	case "free", "open_source":
		return true
	default:
		return false
	}
}

func (s *BizDecipherService) requireCapabilityAssetOwner(ctx context.Context, assetID, ownerUserID int64) (*CapabilityAsset, error) {
	asset, err := s.requireCapabilityAssetPackageAccess(ctx, assetID, ownerUserID)
	if err != nil {
		return nil, err
	}
	if asset.UserID != ownerUserID {
		return nil, ErrCapabilityAssetPackageForbidden
	}
	return asset, nil
}

func (s *BizDecipherService) requireCapabilityAssetPackageAccess(ctx context.Context, assetID, viewerUserID int64) (*CapabilityAsset, error) {
	if s == nil || s.repo == nil || assetID <= 0 {
		return nil, ErrCapabilityAssetPackageNotFound
	}
	asset, err := s.repo.GetCapabilityAsset(ctx, assetID, true)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, ErrCapabilityAssetPackageNotFound
	}
	if asset.Status != CapabilityAssetStatusListed && asset.UserID != viewerUserID {
		return nil, ErrCapabilityAssetPackageNotFound
	}
	return asset, nil
}
