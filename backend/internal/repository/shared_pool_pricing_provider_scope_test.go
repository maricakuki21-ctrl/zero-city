package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResolveSharedPoolOfficialAliasKeepsProviderBoundaryAndIgnoresCase(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &bizDecipherRepository{db: db}

	mock.ExpectQuery(`(?s)LOWER\(mc\.provider\) = CASE LOWER\(BTRIM\(spm\.provider\)\).*WHEN 'openai 兼容中转' THEN 'openai'.*jsonb_array_elements_text.*jsonb_typeof\(mc\.aliases\) = 'array'.*LOWER\(official_alias\.value\) = LOWER\(spm\.model_name\).*LIMIT 1$`).
		WithArgs(int64(9), int64(0), "GPT-OFFICIAL-ALIAS", "chat").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "pool_id", "provider", "model_name", "display_name", "upstream_model_name", "model_aliases",
			"pricing_source", "pricing_status", "pricing_config_version", "rate_multiplier", "endpoint_id",
			"endpoint_type", "enabled", "gate_status", "endpoint_pricing_status", "endpoint_config_version", "official",
		}).AddRow(12, 9, "openai", "GPT-OFFICIAL-ALIAS", "Official Alias", "gpt-canonical", []byte(`[]`),
			"official_catalog", "ready", 1, 1.0, 42, "chat", true, "passed", "ready", 1, true))
	quote, err := repo.ResolveSharedPoolPriceQuote(context.Background(), 9, "GPT-OFFICIAL-ALIAS", "chat")
	require.ErrorIs(t, err, service.ErrSharedPoolOfficialPriceRequired)
	require.Nil(t, quote)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNormalizeSharedPoolProviderName(t *testing.T) {
	tests := map[string]string{
		"OpenAI 兼容中转":       "openai",
		"openai-compatible": "openai",
		"GROK":              "xai",
		"Gemini":            "google",
		"Claude":            "anthropic",
		"DeepSeek":          "deepseek",
	}
	for input, want := range tests {
		require.Equal(t, want, normalizeSharedPoolProviderName(input), input)
	}
}
