package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestTavernGamePackageRepositoryPostgres_pinsPublishedVersion(t *testing.T) {
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
	ownerID := insertCapabilityAssetActivityUser(t, db, fmt.Sprintf("tavern-package-%d@example.test", suffix))
	var scriptID int64
	err = db.QueryRowContext(ctx, `
		INSERT INTO tavern_scripts (
			user_id, title, slug, summary, description, genre, status, visibility,
			player_min, player_max, estimated_minutes, difficulty
		) VALUES (
			$1, 'Package integration', $2, 'summary', 'description',
			'mystery', 'listed', 'public', 1, 6, 60, 'normal'
		)
		RETURNING id`, ownerID, fmt.Sprintf("package-integration-%d", suffix)).Scan(&scriptID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM tavern_rooms WHERE script_id = $1`, scriptID)
		_, _ = db.ExecContext(ctx, `DELETE FROM tavern_game_packages WHERE script_id = $1`, scriptID)
		_, _ = db.ExecContext(ctx, `DELETE FROM tavern_scripts WHERE id = $1`, scriptID)
		_, _ = db.ExecContext(ctx, `DELETE FROM biz_profiles WHERE user_id = $1`, ownerID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, ownerID)
	})

	repo := &bizDecipherRepository{db: db}
	svc := service.NewBizDecipherService(repo, nil, nil)
	manifest := service.TavernGamePackageManifest{
		SchemaVersion:   service.TavernGamePackageSchemaV1,
		RuntimeKind:     service.TavernGamePackageRuntime,
		ProtocolVersion: service.TavernGamePackageProtocolV1,
		Entry:           service.TavernGamePackageEntry{Kind: "prompt_flow", Ref: "main"},
		Permissions: service.TavernGamePackagePermissions{
			AIGateway: true,
			Save:      true,
		},
		Content: json.RawMessage(`{"scenes":[]}`),
		Limits:  service.TavernGamePackageLimits{MaxTurns: 64, MaxScenes: 8},
	}

	created, err := svc.CreateTavernGamePackage(ctx, scriptID, ownerID, service.TavernGamePackageInput{
		Version: "v2",
		Manifest: mustJSON(t, manifest),
	})
	require.NoError(t, err)
	require.Equal(t, service.TavernGamePackageStatusDraft, created.Status)

	_, err = svc.CreateTavernRoom(ctx, ownerID, service.TavernRoomInput{ScriptID: scriptID, Title: "Draft package room"})
	require.ErrorIs(t, err, service.ErrTavernGamePackageUnavailable)

	published, err := svc.PublishTavernGamePackage(ctx, scriptID, ownerID, "v2")
	require.NoError(t, err)
	require.Equal(t, service.TavernGamePackageStatusPublished, published.Status)

	room, err := svc.CreateTavernRoom(ctx, ownerID, service.TavernRoomInput{ScriptID: scriptID, Title: "Pinned room"})
	require.NoError(t, err)
	require.NotNil(t, room.PackageID)
	require.Equal(t, published.ID, *room.PackageID)
	require.Equal(t, "v2", room.PackageVersion)

	runtimeConfig, err := svc.GetTavernRoomRuntime(ctx, ownerID, room.ID)
	require.NoError(t, err)
	require.NotNil(t, runtimeConfig.Package)
	require.Equal(t, "v2", runtimeConfig.Package.Version)
	require.Equal(t, service.TavernGamePackageProtocolV1, runtimeConfig.Bridge.ProtocolVersion)
	require.Equal(t, 64, runtimeConfig.Budget.TurnBudget)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}
