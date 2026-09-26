package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreatorColumnsPostgresPrivacyAndModeration(t *testing.T) {
	dsn := os.Getenv("MARKETPLACE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MARKETPLACE_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if os.Getenv("CREATOR_COLUMNS_TEST_ISOLATED") == "1" {
		var database string
		require.NoError(t, db.QueryRowContext(ctx, "SELECT current_database()").Scan(&database))
		require.True(t, strings.HasPrefix(database, "bizdecipher_columns_acceptance_test_"), "isolated migration requires a disposable database")
		require.NoError(t, ApplyMigrations(ctx, db))
	}
	owner := insertMarketplaceTestUser(t, db, fmt.Sprintf("column-owner-%d@example.test", time.Now().UnixNano()))
	other := insertMarketplaceTestUser(t, db, fmt.Sprintf("column-other-%d@example.test", time.Now().UnixNano()))
	r := &bizDecipherRepository{db: db}
	title, body := "Column", "Private draft body"
	column, err := r.CreateCreatorColumn(ctx, owner, service.CreatorColumnInput{Title: &title})
	require.NoError(t, err)
	_, err = r.CreateCreatorColumn(ctx, owner, service.CreatorColumnInput{Title: &title})
	require.ErrorIs(t, err, service.ErrCreatorColumnExists)
	article, err := r.SaveCreatorColumnArticle(ctx, column.ID, 0, owner, service.CreatorColumnArticleInput{Title: &title, Body: &body})
	require.NoError(t, err)
	_, err = r.GetCreatorColumnArticle(ctx, column.ID, article.ID, service.CreatorColumnQuery{ViewerID: other})
	require.ErrorIs(t, err, service.ErrCreatorColumnNotFound)
	_, err = r.SaveCreatorColumnArticle(ctx, column.ID, article.ID, other, service.CreatorColumnArticleInput{Title: &title})
	require.ErrorIs(t, err, service.ErrCreatorColumnForbidden)
	_, err = r.UpdateCreatorColumn(ctx, column.ID, other, service.CreatorColumnInput{Title: &title})
	require.ErrorIs(t, err, service.ErrCreatorColumnForbidden)
	status := "published"
	_, err = r.SaveCreatorColumnArticle(ctx, column.ID, article.ID, owner, service.CreatorColumnArticleInput{Status: &status})
	require.NoError(t, err)
	items, err := r.ListCreatorColumnArticles(ctx, column.ID, service.CreatorColumnQuery{Limit: 20})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Empty(t, items[0].Body)
	_, err = r.ModerateCreatorColumn(ctx, column.ID, "suspended", "Review")
	require.NoError(t, err)
	_, err = r.GetCreatorColumn(ctx, column.ID, service.CreatorColumnQuery{ViewerID: other})
	require.ErrorIs(t, err, service.ErrCreatorColumnNotFound)
	_, err = r.GetCreatorColumn(ctx, column.ID, service.CreatorColumnQuery{ViewerID: other, Admin: true})
	require.NoError(t, err)
	_, err = r.GetCreatorColumnArticle(ctx, column.ID, article.ID, service.CreatorColumnQuery{ViewerID: other, Admin: true})
	require.NoError(t, err)
	_, err = r.GetCreatorColumnArticle(ctx, column.ID, article.ID, service.CreatorColumnQuery{ViewerID: other})
	require.ErrorIs(t, err, service.ErrCreatorColumnNotFound)
	_, err = r.SaveCreatorColumnArticle(ctx, column.ID, article.ID, owner, service.CreatorColumnArticleInput{Title: &title})
	require.ErrorIs(t, err, service.ErrCreatorColumnForbidden)
	own, err := r.ListCreatorColumnArticles(ctx, column.ID, service.CreatorColumnQuery{ViewerID: owner, Mine: true, Limit: 20})
	require.NoError(t, err)
	require.Len(t, own, 1)
	_, err = r.ModerateCreatorColumn(ctx, column.ID, "active", "")
	require.NoError(t, err)
	read, err := r.GetCreatorColumnArticle(ctx, column.ID, article.ID, service.CreatorColumnQuery{})
	require.NoError(t, err)
	require.Equal(t, body, read.Body)
	status = "archived"
	_, err = r.SaveCreatorColumnArticle(ctx, column.ID, article.ID, owner, service.CreatorColumnArticleInput{Status: &status})
	require.NoError(t, err)
	_, err = r.GetCreatorColumnArticle(ctx, column.ID, article.ID, service.CreatorColumnQuery{})
	require.ErrorIs(t, err, service.ErrCreatorColumnNotFound)
}
