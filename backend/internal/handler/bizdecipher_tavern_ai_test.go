package handler

import (
	"database/sql"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTavernAIIntentFreezesContextAtCursor(t *testing.T) {
	config := &service.TavernRuntimeConfig{
		Script:  service.TavernScript{Title: "Mystery"},
		Prompts: service.TavernRuntimePrompts{HostBrief: "Release one clue", SafetyNotes: "No spoilers"},
	}
	turns := []service.TavernRoomTurn{
		{TurnIndex: 1, AuthorName: "Player", Body: "Inspect the room"},
		{TurnIndex: 2, AuthorName: "Player", Body: "Later message"},
	}
	prompt := tavernAIIntent(config, turns, 1, " continue ")
	require.Contains(t, prompt, "Inspect the room")
	require.Contains(t, prompt, "No spoilers")
	require.NotContains(t, prompt, "Later message")
	require.Equal(t, prompt, tavernAIIntent(config, turns[:1], 1, " continue "))
	require.NotContains(t, tavernAIIntent(config, turns, 0, "start"), "Inspect the room")
}

func TestTavernAIErrorStatus(t *testing.T) {
	for _, item := range []struct {
		err    error
		status int
	}{
		{service.ErrTavernRoomForbidden, 403},
		{sql.ErrNoRows, 404},
		{service.ErrTavernRoomState, 400},
		{service.ErrTavernGamePackageUnavailable, 409},
	} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		respondTavernAIError(ctx, item.err)
		require.Equal(t, item.status, recorder.Code)
	}
}

func TestWorkbenchMissingCatalogIsActionable(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	require.True(t, writeWorkbenchError(ctx, service.ErrWorkbenchCatalogUnavailable))
	require.Equal(t, 503, recorder.Code)
	require.Contains(t, recorder.Body.String(), "WORKBENCH_CATALOG_UNAVAILABLE")
	require.NotContains(t, recorder.Body.String(), "internal error")
}
