package handler

import (
	"net/http"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/stretchr/testify/require"
)

func TestCreatorColumnQueryRejectsAnonymousMineAndBadPages(t *testing.T) {
	for _, tc := range []struct {
		query  string
		status int
	}{
		{"?mine=true", http.StatusUnauthorized},
		{"?mine=nope", http.StatusBadRequest},
		{"?limit=51", http.StatusBadRequest},
		{"?limit=0", http.StatusBadRequest},
		{"?cursor=-1", http.StatusBadRequest},
	} {
		t.Run(tc.query, func(t *testing.T) {
			w, c := newPollHandlerContext(http.MethodGet, "/columns"+tc.query, "", "", 0)
			_, ok := creatorColumnQuery(c, false)
			require.False(t, ok)
			require.Equal(t, tc.status, w.Code)
		})
	}
	_, c := newPollHandlerContext(http.MethodGet, "/columns?mine=true&cursor=80&limit=5", "", "", 42)
	q, ok := creatorColumnQuery(c, false)
	require.True(t, ok)
	require.Equal(t, int64(42), q.ViewerID)
	require.Equal(t, int64(80), q.Cursor)
	require.Equal(t, 5, q.Limit)
	require.False(t, q.Admin)
	_, c = newPollHandlerContext(http.MethodGet, "/columns?admin=true", "", "", 42)
	q, ok = creatorColumnQuery(c, false)
	require.True(t, ok)
	require.False(t, q.Admin)
	c.Set(string(middleware2.ContextKeyUserRole), "admin")
	q, ok = creatorColumnQuery(c, false)
	require.True(t, ok)
	require.True(t, q.Admin)
}
