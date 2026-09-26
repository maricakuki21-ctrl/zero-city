package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type capabilityAssetPackageRepoStub struct {
	BizDecipherRepository
	asset           *CapabilityAsset
	assetErr        error
	versions        []CapabilityAssetVersion
	pkg             *CapabilityAssetPackage
	createCalls     int
	downloadRecords int
	putPaths        []string
	finalizeCalls   int
	importErr       error
}

func (r *capabilityAssetPackageRepoStub) ImportCapabilityAssetPackageTx(
	ctx context.Context, assetID, userID int64, input CreateCapabilityAssetVersionInput,
	files []PreparedCapabilityAssetFile, finalize bool,
) (*CapabilityAssetVersion, error) {
	if r.importErr != nil {
		return nil, r.importErr
	}
	if len(r.versions) == 0 {
		r.createCalls++
	}
	r.putPaths = nil
	for _, file := range files {
		r.putPaths = append(r.putPaths, file.Path)
	}
	if finalize {
		return r.FinalizeCapabilityAssetVersion(ctx, assetID, userID, input.Version)
	}
	return &CapabilityAssetVersion{Version: input.Version, Status: CapabilityAssetVersionStatusDraft}, nil
}

func (r *capabilityAssetPackageRepoStub) GetCapabilityAsset(context.Context, int64, bool) (*CapabilityAsset, error) {
	return r.asset, r.assetErr
}

func (r *capabilityAssetPackageRepoStub) ListCapabilityAssetVersions(context.Context, int64) ([]CapabilityAssetVersion, error) {
	return r.versions, nil
}

func (r *capabilityAssetPackageRepoStub) CreateCapabilityAssetVersion(
	context.Context,
	int64,
	int64,
	CreateCapabilityAssetVersionInput,
) (*CapabilityAssetVersion, error) {
	r.createCalls++
	return &CapabilityAssetVersion{Version: "v1.0.0", Status: CapabilityAssetVersionStatusDraft}, nil
}

func (r *capabilityAssetPackageRepoStub) GetCapabilityAssetPackage(context.Context, int64, string) (*CapabilityAssetPackage, error) {
	return r.pkg, nil
}

func (r *capabilityAssetPackageRepoStub) PutCapabilityAssetFile(
	_ context.Context,
	_, _ int64,
	_ string,
	input PreparedCapabilityAssetFile,
) (*CapabilityAssetVersion, error) {
	r.putPaths = append(r.putPaths, input.Path)
	return &CapabilityAssetVersion{
		Version:   "v1.0.0",
		Status:    CapabilityAssetVersionStatusDraft,
		FileCount: len(r.putPaths),
	}, nil
}

func (r *capabilityAssetPackageRepoStub) FinalizeCapabilityAssetVersion(
	context.Context,
	int64,
	int64,
	string,
) (*CapabilityAssetVersion, error) {
	r.finalizeCalls++
	return &CapabilityAssetVersion{
		Version:   "v1.0.0",
		Status:    CapabilityAssetVersionStatusReady,
		FileCount: len(r.putPaths),
	}, nil
}

func (r *capabilityAssetPackageRepoStub) RecordCapabilityAssetDownload(context.Context, int64, int64) error {
	r.downloadRecords++
	return nil
}

func TestCapabilityAssetPackageDigestIsDeterministicAndSensitive(t *testing.T) {
	files := []CapabilityAssetFile{
		{Path: "src/main.go", SHA256: strings.Repeat("a", 64), ByteSize: 12},
		{Path: "manifest.json", SHA256: strings.Repeat("b", 64), ByteSize: 34},
	}
	reordered := []CapabilityAssetFile{files[1], files[0]}
	require.Equal(t, CapabilityAssetPackageDigest(files), CapabilityAssetPackageDigest(reordered))

	changed := append([]CapabilityAssetFile(nil), files...)
	changed[0].SHA256 = strings.Repeat("c", 64)
	require.NotEqual(t, CapabilityAssetPackageDigest(files), CapabilityAssetPackageDigest(changed))
}

func TestPrepareCapabilityAssetFileRejectsUnsafeOrOversizedInput(t *testing.T) {
	tests := []struct {
		name  string
		input CapabilityAssetFileInput
	}{
		{
			name: "absolute path",
			input: CapabilityAssetFileInput{
				Path:          "/etc/passwd",
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("x")),
			},
		},
		{
			name: "path traversal",
			input: CapabilityAssetFileInput{
				Path:          "src/../../secret.txt",
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("x")),
			},
		},
		{
			name: "windows path",
			input: CapabilityAssetFileInput{
				Path:          `src\main.go`,
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("x")),
			},
		},
		{
			name: "oversized file",
			input: CapabilityAssetFileInput{
				Path:          "large.bin",
				ContentBase64: base64.StdEncoding.EncodeToString(make([]byte, CapabilityAssetMaxFileBytes+1)),
			},
		},
		{
			name: "empty content",
			input: CapabilityAssetFileInput{
				Path: "empty.txt",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := prepareCapabilityAssetFile(test.input)
			require.ErrorIs(t, err, ErrCapabilityAssetPackageInvalid)
		})
	}
}

func TestPrepareCapabilityAssetFileNormalizesMetadataAndDigest(t *testing.T) {
	prepared, err := prepareCapabilityAssetFile(CapabilityAssetFileInput{
		Path:          " manifest.json ",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte(`{"name":"demo"}`)),
	})

	require.NoError(t, err)
	require.Equal(t, "manifest.json", prepared.Path)
	require.Equal(t, "application/octet-stream", prepared.ContentType)
	require.NotEmpty(t, prepared.SHA256)
	require.Equal(t, []byte(`{"name":"demo"}`), prepared.Content)
}

func TestCreateCapabilityAssetVersionRejectsNonOwnerBeforeRepositoryWrite(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.CreateCapabilityAssetVersion(context.Background(), 41, 7, CreateCapabilityAssetVersionInput{
		Version:     "v1.0.0",
		RuntimeKind: "workflow",
		Manifest:    []byte(`{}`),
	})

	require.ErrorIs(t, err, ErrCapabilityAssetPackageForbidden)
	require.Zero(t, repo.createCalls)
}

func TestListCapabilityAssetVersionsHidesUnpublishedVersionsFromPublicViewer(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
		versions: []CapabilityAssetVersion{
			{Version: "draft", Status: CapabilityAssetVersionStatusDraft},
			{Version: "ready", Status: CapabilityAssetVersionStatusReady},
			{Version: "published", Status: CapabilityAssetVersionStatusPublished},
			{Version: "revoked", Status: CapabilityAssetVersionStatusRevoked},
		},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	versions, err := svc.ListCapabilityAssetVersions(context.Background(), 41, 7)

	require.NoError(t, err)
	require.Len(t, versions, 1)
	require.Equal(t, "published", versions[0].Version)
}

func TestDownloadCapabilityAssetPackageRequiresPublishedVersionForNonOwner(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
		pkg: &CapabilityAssetPackage{Version: CapabilityAssetVersion{
			ID: 100, AssetID: 41, Version: "v1.0.0", Status: CapabilityAssetVersionStatusReady,
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.DownloadCapabilityAssetPackage(context.Background(), 41, 7, "v1.0.0")

	require.ErrorIs(t, err, ErrCapabilityAssetPackageForbidden)
	require.Zero(t, repo.downloadRecords)
}

func TestDownloadCapabilityAssetPackageRecordsPublishedDownload(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed, PricingType: "free"},
		pkg: &CapabilityAssetPackage{Version: CapabilityAssetVersion{
			ID: 100, AssetID: 41, Version: "v1.0.0", Status: CapabilityAssetVersionStatusPublished,
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	pkg, err := svc.DownloadCapabilityAssetPackage(context.Background(), 41, 7, "v1.0.0")

	require.NoError(t, err)
	require.Equal(t, int64(100), pkg.Version.ID)
	require.Equal(t, 1, repo.downloadRecords)
}

func TestDownloadCapabilityAssetPackageBlocksPaidDeliveryUntilEntitlementExists(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed, PricingType: "paid"},
		pkg: &CapabilityAssetPackage{Version: CapabilityAssetVersion{
			ID: 100, AssetID: 41, Version: "v1.0.0", Status: CapabilityAssetVersionStatusPublished,
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.DownloadCapabilityAssetPackage(context.Background(), 41, 7, "v1.0.0")

	require.ErrorIs(t, err, ErrCapabilityAssetPackageForbidden)
	require.Zero(t, repo.downloadRecords)
}

func TestDownloadCapabilityAssetPackageRejectsRevokedVersion(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
		pkg: &CapabilityAssetPackage{Version: CapabilityAssetVersion{
			ID: 100, AssetID: 41, Version: "v1.0.0", Status: CapabilityAssetVersionStatusRevoked,
		}},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.DownloadCapabilityAssetPackage(context.Background(), 41, 9, "v1.0.0")

	require.True(t, errors.Is(err, ErrCapabilityAssetPackageNotFound))
	require.Zero(t, repo.downloadRecords)
}

func TestImportCapabilityAssetPackageValidatesEveryFileBeforeAnyWrite(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.ImportCapabilityAssetPackage(context.Background(), 41, 9, ImportCapabilityAssetPackageInput{
		Version:     "v1.0.0",
		RuntimeKind: "workflow",
		Manifest:    []byte(`{"name":"demo"}`),
		Files: []CapabilityAssetFileInput{
			{Path: "ok/run.json", ContentBase64: base64.StdEncoding.EncodeToString([]byte(`{"steps":[]}`))},
			{Path: "../escape.txt", ContentBase64: base64.StdEncoding.EncodeToString([]byte("x"))},
		},
		Finalize: true,
	})

	require.ErrorIs(t, err, ErrCapabilityAssetPackageInvalid)
	require.Contains(t, err.Error(), "../escape.txt")
	require.Zero(t, repo.createCalls)
	require.Empty(t, repo.putPaths)
	require.Zero(t, repo.finalizeCalls)
}

func TestImportCapabilityAssetPackageRejectsDuplicatePaths(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.ImportCapabilityAssetPackage(context.Background(), 41, 9, ImportCapabilityAssetPackageInput{
		Version:     "v1.0.0",
		RuntimeKind: "workflow",
		Manifest:    []byte(`{}`),
		Files: []CapabilityAssetFileInput{
			{Path: "a.txt", ContentBase64: base64.StdEncoding.EncodeToString([]byte("1"))},
			{Path: " a.txt ", ContentBase64: base64.StdEncoding.EncodeToString([]byte("2"))},
		},
		Finalize: true,
	})

	require.ErrorIs(t, err, ErrCapabilityAssetPackageInvalid)
	require.Contains(t, err.Error(), "duplicate")
	require.Zero(t, repo.createCalls)
}

func TestImportCapabilityAssetPackageCreatesWritesAndFinalizes(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	item, err := svc.ImportCapabilityAssetPackage(context.Background(), 41, 9, ImportCapabilityAssetPackageInput{
		Version:     "v1.0.0",
		RuntimeKind: "game",
		Manifest:    []byte(`{"name":"demo"}`),
		Files: []CapabilityAssetFileInput{
			{Path: "manifest.json", ContentBase64: base64.StdEncoding.EncodeToString([]byte(`{"name":"demo"}`))},
			{Path: "scenes/intro.json", ContentBase64: base64.StdEncoding.EncodeToString([]byte(`{"id":"intro"}`))},
		},
		Finalize: true,
	})

	require.NoError(t, err)
	require.Equal(t, CapabilityAssetVersionStatusReady, item.Status)
	require.Equal(t, 1, repo.createCalls)
	require.ElementsMatch(t, []string{"manifest.json", "scenes/intro.json"}, repo.putPaths)
	require.Equal(t, 1, repo.finalizeCalls)
}

func TestImportCapabilityAssetPackageResumesExistingDraftWithoutFinalize(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
		versions: []CapabilityAssetVersion{
			{AssetID: 41, Version: "v1.0.0", Status: CapabilityAssetVersionStatusDraft, RuntimeKind: "workflow", Manifest: []byte(`{}`)},
		},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	item, err := svc.ImportCapabilityAssetPackage(context.Background(), 41, 9, ImportCapabilityAssetPackageInput{
		Version:     "v1.0.0",
		RuntimeKind: "workflow",
		Manifest:    []byte(`{}`),
		Files: []CapabilityAssetFileInput{
			{Path: "run.json", ContentBase64: base64.StdEncoding.EncodeToString([]byte(`{"steps":[]}`))},
		},
		Finalize: false,
	})

	require.NoError(t, err)
	require.Equal(t, CapabilityAssetVersionStatusDraft, item.Status)
	require.Zero(t, repo.createCalls)
	require.Equal(t, []string{"run.json"}, repo.putPaths)
	require.Zero(t, repo.finalizeCalls)
}

func TestImportCapabilityAssetPackagePropagatesRepositoryConflict(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		importErr: ErrCapabilityAssetPackageConflict,
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
		versions: []CapabilityAssetVersion{
			{AssetID: 41, Version: "v1.0.0", Status: CapabilityAssetVersionStatusPublished},
		},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.ImportCapabilityAssetPackage(context.Background(), 41, 9, ImportCapabilityAssetPackageInput{
		Version:     "v1.0.0",
		RuntimeKind: "workflow",
		Manifest:    []byte(`{}`),
		Files: []CapabilityAssetFileInput{
			{Path: "run.json", ContentBase64: base64.StdEncoding.EncodeToString([]byte(`{}`))},
		},
		Finalize: true,
	})

	require.ErrorIs(t, err, ErrCapabilityAssetPackageConflict)
	require.Zero(t, repo.createCalls)
	require.Empty(t, repo.putPaths)
}

func TestImportCapabilityAssetPackageRejectsNonOwner(t *testing.T) {
	repo := &capabilityAssetPackageRepoStub{
		asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
	}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.ImportCapabilityAssetPackage(context.Background(), 41, 7, ImportCapabilityAssetPackageInput{
		Version:     "v1.0.0",
		RuntimeKind: "workflow",
		Manifest:    []byte(`{}`),
		Files: []CapabilityAssetFileInput{
			{Path: "run.json", ContentBase64: base64.StdEncoding.EncodeToString([]byte(`{}`))},
		},
	})

	require.ErrorIs(t, err, ErrCapabilityAssetPackageForbidden)
	require.Zero(t, repo.createCalls)
}

func TestImportCapabilityAssetPackageDoesNotWriteAfterRepositoryConflict(t *testing.T) {
	for _, runtime := range []string{"workflow", "game"} {
		t.Run(runtime, func(t *testing.T) {
			repo := &capabilityAssetPackageRepoStub{
				importErr: ErrCapabilityAssetPackageConflict,
				asset: &CapabilityAsset{ID: 41, UserID: 9, Status: CapabilityAssetStatusListed},
				versions: []CapabilityAssetVersion{{
					Version: "v1", Status: CapabilityAssetVersionStatusDraft,
					RuntimeKind: "workflow", Manifest: []byte(`{"entry":"original.json"}`),
				}},
			}
			svc := NewBizDecipherService(repo, nil, nil)
			_, err := svc.ImportCapabilityAssetPackage(context.Background(), 41, 9, ImportCapabilityAssetPackageInput{
				Version: "v1", RuntimeKind: runtime, Manifest: []byte(`{"entry":"replacement.json"}`),
				Files: []CapabilityAssetFileInput{{Path: "run.json", ContentBase64: "e30="}},
				Finalize: true,
			})
			require.ErrorIs(t, err, ErrCapabilityAssetPackageConflict)
			require.Empty(t, repo.putPaths)
			require.Zero(t, repo.finalizeCalls)
		})
	}
}
