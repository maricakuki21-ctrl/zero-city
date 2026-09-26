package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreatorColumnValidation(t *testing.T) {
	s := &BizDecipherService{}
	text := strings.Repeat("x", 121)
	_, err := s.SaveCreatorColumn(context.Background(), 0, 1, CreatorColumnInput{Title: &text})
	require.ErrorIs(t, err, ErrCreatorColumnInvalid)
	empty := " "
	_, err = s.SaveCreatorColumn(context.Background(), 1, 1, CreatorColumnInput{Title: &empty})
	require.ErrorIs(t, err, ErrCreatorColumnInvalid)
	title, body, status := "Article", "Content", "paid"
	_, err = s.SaveCreatorColumnArticle(context.Background(), 1, 0, 1, CreatorColumnArticleInput{Title: &title, Body: &body, Status: &status})
	require.ErrorIs(t, err, ErrCreatorColumnInvalid)
	status = "archived"
	_, err = s.SaveCreatorColumnArticle(context.Background(), 1, 0, 1, CreatorColumnArticleInput{Title: &title, Body: &body, Status: &status})
	require.ErrorIs(t, err, ErrCreatorColumnInvalid)
	_, err = s.ModerateCreatorColumn(context.Background(), 1, "published", "")
	require.ErrorIs(t, err, ErrCreatorColumnInvalid)
	_, err = s.ModerateCreatorColumn(context.Background(), 1, "suspended", "  ")
	require.ErrorIs(t, err, ErrCreatorColumnInvalid)
	_, err = s.ModerateCreatorColumn(context.Background(), 1, "active", "")
	require.ErrorIs(t, err, ErrCreatorColumnInvalid)
}

func TestCreatorColumnPaginationValidation(t *testing.T) {
	for _, q := range []CreatorColumnQuery{{Cursor: -1}, {Limit: 51}, {Mine: true}} {
		_, err := validateColumnQuery(q)
		require.ErrorIs(t, err, ErrCreatorColumnInvalid)
	}
	q, err := validateColumnQuery(CreatorColumnQuery{Mine: true, ViewerID: 1})
	require.NoError(t, err)
	require.Equal(t, 20, q.Limit)
}
