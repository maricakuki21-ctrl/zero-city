package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestCapabilityAssetActivityRepositoryPostgres_countsAndViewerState(t *testing.T) {
	dsn := os.Getenv("BIZDECIPHER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("BIZDECIPHER_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	require.NoError(t, db.PingContext(ctx))

	suffix := time.Now().UnixNano()
	ownerID := insertCapabilityAssetActivityUser(t, db, fmt.Sprintf("asset-owner-%d@example.test", suffix))
	viewerID := insertCapabilityAssetActivityUser(t, db, fmt.Sprintf("asset-viewer-%d@example.test", suffix))
	otherID := insertCapabilityAssetActivityUser(t, db, fmt.Sprintf("asset-other-%d@example.test", suffix))

	var assetID int64
	err = db.QueryRowContext(ctx, `
		INSERT INTO capability_assets (user_id, title, slug, summary, description, asset_type, status, published_at)
		VALUES ($1, 'Activity asset', $2, 'summary', 'description', 'workflow', 'listed', NOW())
		RETURNING id`, ownerID, fmt.Sprintf("activity-asset-%d", suffix)).Scan(&assetID)
	require.NoError(t, err)

	repo := &bizDecipherRepository{db: db}
	count, err := repo.RecordCapabilityAssetView(ctx, assetID, viewerID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	count, err = repo.RecordCapabilityAssetView(ctx, assetID, viewerID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count, "same viewer and UTC day must not inflate views")
	count, err = repo.RecordCapabilityAssetView(ctx, assetID, otherID)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	likeCount, err := repo.SetCapabilityAssetLike(ctx, assetID, viewerID, true)
	require.NoError(t, err)
	require.Equal(t, int64(1), likeCount)
	likeCount, err = repo.SetCapabilityAssetLike(ctx, assetID, viewerID, true)
	require.NoError(t, err)
	require.Equal(t, int64(1), likeCount)
	favoriteCount, err := repo.SetCapabilityAssetFavorite(ctx, assetID, viewerID, true)
	require.NoError(t, err)
	require.Equal(t, int64(1), favoriteCount)

	var versionID int64
	err = db.QueryRowContext(ctx, `
		INSERT INTO capability_asset_versions (
			asset_id, owner_user_id, version, runtime_kind, status, file_count, total_bytes, published_at
		) VALUES ($1, $2, 'v1.0.0', 'workflow', 'published', 0, 0, NOW())
		RETURNING id`, assetID, ownerID).Scan(&versionID)
	require.NoError(t, err)
	require.NoError(t, repo.RecordCapabilityAssetDownload(ctx, versionID, viewerID))
	require.NoError(t, repo.RecordCapabilityAssetUse(ctx, assetID, viewerID, "run", "harness", "run-1"))
	require.NoError(t, repo.RecordCapabilityAssetUse(ctx, assetID, viewerID, "run", "harness", "run-1"))

	state, err := repo.GetCapabilityAssetViewerState(ctx, assetID, viewerID)
	require.NoError(t, err)
	require.True(t, state.Liked)
	require.True(t, state.Favorited)
	require.True(t, state.Downloaded)
	require.True(t, state.LatestView)

	stats, err := repo.GetCapabilityAssetStats(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, int64(2), stats[0].ViewCount)
	require.Equal(t, int64(1), stats[0].LikeCount)
	require.Equal(t, int64(1), stats[0].FavoriteCount)
	require.Equal(t, int64(1), stats[0].DownloadCount)
	require.Equal(t, int64(1), stats[0].UseCount, "source_id must make repeated run callbacks idempotent")
	require.Equal(t, int64(2), stats[0].UniqueUsers)
	require.False(t, stats[0].RevenueConnected)

	_, err = repo.SetCapabilityAssetLike(ctx, assetID, viewerID, false)
	require.NoError(t, err)
	_, err = repo.SetCapabilityAssetFavorite(ctx, assetID, viewerID, false)
	require.NoError(t, err)
	state, err = repo.GetCapabilityAssetViewerState(ctx, assetID, viewerID)
	require.NoError(t, err)
	require.False(t, state.Liked)
	require.False(t, state.Favorited)
}

func insertCapabilityAssetActivityUser(t *testing.T, db *sql.DB, email string) int64 {
	t.Helper()
	var userID int64
	err := db.QueryRow(`
		INSERT INTO users (email, password_hash, role, status)
		VALUES ($1, 'test', 'user', 'active')
		RETURNING id`, email).Scan(&userID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO biz_profiles (user_id, display_name) VALUES ($1, $2)`, userID, email)
	require.NoError(t, err)
	return userID
}
