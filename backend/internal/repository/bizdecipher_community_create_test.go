package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreateCommunityPostPersistsLocationAndScansInsertedRow(t *testing.T) {
	for _, location := range []struct{ district, channel string }{
		{"tavern", "chat-hall"},
		{"", ""},
	} {
		t.Run(location.district+"/"+location.channel, func(t *testing.T) {
			// Match the returned SQL projection too: a mock with extra columns
			// would otherwise hide an INSERT/Scan column-count mismatch.
			matcher := sqlmock.QueryMatcherFunc(func(expected, actual string) error {
				return sqlmock.QueryMatcherRegexp.Match(expected, strings.Join(strings.Fields(actual), " "))
			})
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
			require.NoError(t, err)
			defer db.Close()

			input := service.CommunityPostInput{
				Kind: "support", Title: "Need help", Body: "Body",
				Tags: []string{"qa"}, District: location.district, Channel: location.channel,
				Scenario: "incident_support", ActionType: "ask_help",
				Evidence: json.RawMessage(`[]`), TrustSignals: json.RawMessage(`{}`),
			}
			projection := "RETURNING id, user_id, '' AS author, kind, title, body, tags, district, channel, private, status, pinned, catches, replies, views, source_type, source_id, scenario, subject_type, subject_id, subject_title, action_type, evidence, trust_signals, created_at, updated_at"
			mock.ExpectQuery(`^INSERT INTO community_posts .*trust_signals, district, channel\) VALUES .*`+regexp.QuoteMeta(projection)+`$`).
				WithArgs(int64(303), input.Kind, input.Title, input.Body, `["qa"]`, false,
					"", "", input.Scenario, "", "", "", input.ActionType, `[]`, `{}`,
					location.district, location.channel).
				WillReturnRows(newCommunityPostRows().AddRow(
					int64(101), int64(303), "", input.Kind, input.Title, input.Body, []byte(`["qa"]`),
					location.district, location.channel, false, "open", false, 0, 0, 0,
					"", "", input.Scenario, "", "", "", input.ActionType,
					[]byte(`[]`), []byte(`{}`), time.Now(), time.Now(),
				))

			post, err := (&bizDecipherRepository{db: db}).CreateCommunityPost(context.Background(), 303, input)
			require.NoError(t, err)
			require.Equal(t, int64(101), post.ID)
			require.Equal(t, location.district, post.District)
			require.Equal(t, location.channel, post.Channel)
			require.Equal(t, input.Tags, post.Tags)
			require.Equal(t, "open", post.Status)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
