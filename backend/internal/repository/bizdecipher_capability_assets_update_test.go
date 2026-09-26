package repository

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func validCapabilityAssetUpdateInput() service.CapabilityAssetInput {
	return service.CapabilityAssetInput{
		Title:             "Draft title",
		Summary:           "Draft summary",
		Description:       "Draft description",
		AssetType:         "workflow",
		Status:            service.CapabilityAssetStatusDraft,
		Tags:              []string{"ai"},
		ScenarioTags:      []string{"support"},
		IntegrationTags:   []string{"api"},
		ScreenshotURLs:    []string{"https://example.test/shot.png"},
		PrimaryActionType: "view_workflow",
		PricingType:       "free",
		ContactEnabled:    true,
	}
}

func TestUpdateCapabilityAssetRejectsRowsOutsideOwnedDraftPredicate(t *testing.T) {
	for _, scenario := range []string{"cross owner", "not found", "listed public state", "review won concurrency race"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			repo := &bizDecipherRepository{db: db}
			input := validCapabilityAssetUpdateInput()

			mock.ExpectQuery(`(?s)UPDATE capability_assets.*WHERE id = \$1.*AND user_id = \$2.*AND status = 'draft'.*AND deleted_at IS NULL.*RETURNING id`).
				WithArgs(
					int64(41), int64(17), input.Title, input.Summary, input.Description, input.AssetType,
					`["ai"]`, `["support"]`, `["api"]`, input.CoverURL, `["https://example.test/shot.png"]`,
					input.VideoURL, input.DemoURL, input.DocURL, input.SourceURL, input.TemplateURL,
					input.PrimaryActionType, input.PricingType, input.ContactEnabled,
				).
				WillReturnRows(sqlmock.NewRows([]string{"id"}))

			asset, err := repo.UpdateCapabilityAsset(context.Background(), 41, 17, input)

			require.Nil(t, asset)
			require.ErrorIs(t, err, service.ErrCapabilityAssetDraftNotFound)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUpdateCapabilityAssetUsesSingleAtomicOwnerAndDraftPredicate(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}
	input := validCapabilityAssetUpdateInput()
	query := `(?s)` + regexp.QuoteMeta("UPDATE capability_assets") + `.*` +
		regexp.QuoteMeta("WHERE id = $1") + `.*` + regexp.QuoteMeta("AND user_id = $2") + `.*` +
		regexp.QuoteMeta("AND status = 'draft'") + `.*` + regexp.QuoteMeta("RETURNING id")

	mock.ExpectQuery(query).
		WithArgs(
			int64(41), int64(17), input.Title, input.Summary, input.Description, input.AssetType,
			`["ai"]`, `["support"]`, `["api"]`, input.CoverURL, `["https://example.test/shot.png"]`,
			input.VideoURL, input.DemoURL, input.DocURL, input.SourceURL, input.TemplateURL,
			input.PrimaryActionType, input.PricingType, input.ContactEnabled,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err = repo.UpdateCapabilityAsset(context.Background(), 41, 17, input)

	require.ErrorIs(t, err, service.ErrCapabilityAssetDraftNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
