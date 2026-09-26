package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type captureSharedPoolMediaTaskQueryMatcher struct {
	actual *string
}

func TestRecoverExpiredSharedPoolMediaTasksMovesUnknownResultsToReviewWithoutRefund(t *testing.T) {
	var query string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(captureSharedPoolMediaTaskQueryMatcher{actual: &query}))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, time.July, 19, 18, 0, 0, 0, time.UTC)

	mock.ExpectQuery("expired media tasks review").
		WithArgs(now, 100).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)).AddRow(int64(2)))

	count, err := repo.RecoverExpiredSharedPoolMediaTasksTx(context.Background(), now, 100)
	if err != nil {
		t.Fatalf("recover expired media tasks: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 recovered tasks, got %d", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
	for _, fragment := range []string{
		"t.status IN ('submitted', 'processing')",
		"reservation.status IN ('forwarding', 'review_required')",
		"FOR UPDATE OF t, reservation SKIP LOCKED",
		"SET status = 'review_required'",
		"video_task_expired_without_terminal_result",
		"expired_without_terminal_result",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("recovery query missing %q\n%s", fragment, query)
		}
	}
	for _, forbidden := range []string{"available_balance", "held_balance = held_balance -", "status = 'released'"} {
		if strings.Contains(query, forbidden) {
			t.Errorf("unknown video result recovery must not refund via %q\n%s", forbidden, query)
		}
	}
}

func (m captureSharedPoolMediaTaskQueryMatcher) Match(_ string, actual string) error {
	*m.actual = actual
	return nil
}

func TestCreateSharedPoolMediaTaskIdempotentReplayReturnsExistingOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}
	now := time.Date(2026, time.July, 19, 18, 0, 0, 0, time.UTC)
	input := service.CreateSharedPoolMediaTaskInput{
		ReservationID:            1,
		AccessKeyID:              2,
		PoolID:                   3,
		AccountID:                0,
		UserID:                   4,
		PriceVersionID:           5,
		Provider:                 service.PlatformGrok,
		ModelSnapshot:            "published-video",
		UpstreamModelSnapshot:    "grok-imagine-video",
		UpstreamRequestID:        "upstream-task-1",
		ReservationRequestID:     "reservation-request-1",
		RequestedResolution:      service.VideoBillingResolution480P,
		RequestedDurationSeconds: 5,
		ExpiresAt:                now.Add(24 * time.Hour),
	}

	for attempt := 0; attempt < 2; attempt++ {
		mock.ExpectQuery(`(?s)INSERT INTO shared_pool_media_tasks`).
			WithArgs(
				input.ReservationID, input.AccessKeyID, input.PoolID, input.AccountID,
				input.UserID, input.PriceVersionID, input.Provider, input.ModelSnapshot,
				input.UpstreamModelSnapshot, input.UpstreamRequestID, input.RequestedResolution,
				input.RequestedDurationSeconds, input.ExpiresAt, input.ReservationRequestID,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))
		mock.ExpectQuery(`(?s)SELECT.*FROM shared_pool_media_tasks t`).
			WithArgs(input.ReservationID).
			WillReturnRows(sharedPoolMediaTaskReplayRows(input, now))

		task, createErr := repo.CreateSharedPoolMediaTask(context.Background(), input)
		if createErr != nil {
			t.Fatalf("replay attempt %d: %v", attempt+1, createErr)
		}
		if task == nil || task.ID != 99 || task.UpstreamModelSnapshot != input.UpstreamModelSnapshot {
			t.Fatalf("unexpected replay task on attempt %d: %#v", attempt+1, task)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func sharedPoolMediaTaskReplayRows(input service.CreateSharedPoolMediaTaskInput, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "reservation_id", "access_key_id", "pool_id", "account_id",
		"user_id", "price_version_id", "endpoint_type", "provider",
		"model_snapshot", "upstream_model_snapshot", "upstream_request_id", "reservation_request_id",
		"requested_resolution", "requested_duration_seconds", "status",
		"last_upstream_status", "last_http_status", "pricing_source_snapshot",
		"price_snapshot", "reservation_status", "expires_at", "completed_at", "created_at", "updated_at",
	}).AddRow(
		int64(99), input.ReservationID, input.AccessKeyID, input.PoolID, nil,
		input.UserID, input.PriceVersionID, service.SharedPoolEndpointVideo, input.Provider,
		input.ModelSnapshot, input.UpstreamModelSnapshot, input.UpstreamRequestID, input.ReservationRequestID,
		input.RequestedResolution, input.RequestedDurationSeconds, service.SharedPoolMediaTaskSubmitted,
		"", nil, service.SharedPoolPricingSourceOwner,
		[]byte(`{}`), "forwarding", input.ExpiresAt, nil, now, now,
	)
}

func TestGetSharedPoolMediaTaskByAPIKeyIDScopesStatusLookupToAuthenticatedKey(t *testing.T) {
	var query string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(captureSharedPoolMediaTaskQueryMatcher{actual: &query}))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectQuery("media task owner scope").
		WithArgs(int64(17), "upstream-task-91").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err = repo.GetSharedPoolMediaTaskByAPIKeyID(context.Background(), 17, "upstream-task-91")
	if !errors.Is(err, service.ErrSharedPoolMediaTaskNotFound) {
		t.Fatalf("cross-key or unknown task must be hidden as not found, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
	for _, fragment := range []string{
		"JOIN shared_pool_access_keys sak",
		"sak.id = t.access_key_id",
		"sak.pool_id = t.pool_id",
		"sak.user_id = t.user_id",
		"sak.api_key_id = $1",
		"t.upstream_request_id = $2",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("media task status lookup missing owner fence %q\n%s", fragment, query)
		}
	}
}

func TestGetSharedPoolAccessKeyForMediaTaskUsesFrozenPriceVersionRoute(t *testing.T) {
	var query string
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(captureSharedPoolMediaTaskQueryMatcher{actual: &query}))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer func() { _ = db.Close() }()
	repo := &bizDecipherRepository{db: db}

	mock.ExpectQuery("media task frozen route").
		WithArgs(int64(91), int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err = repo.GetSharedPoolAccessKeyForMediaTask(context.Background(), 17, 91)
	if !errors.Is(err, service.ErrSharedPoolMediaTaskNotFound) {
		t.Fatalf("expected not found from empty fixture, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}

	for _, fragment := range []string{
		"JOIN shared_pool_price_versions pv",
		"pv.id = t.price_version_id",
		"JOIN shared_pool_model_endpoints spe",
		"spe.id = pv.endpoint_id",
		"JOIN shared_pool_models spm",
		"spm.id = pv.pool_model_id",
		"t.upstream_model_snapshot",
		"LOWER(mc.provider) = LOWER(t.provider)",
		"), ''), t.provider",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("frozen media task query missing %q\n%s", fragment, query)
		}
	}
	for _, forbidden := range []string{
		"spm.model_name = t.model_snapshot",
		"jsonb_to_recordset",
		"cfg.upstream_model_name",
	} {
		if strings.Contains(query, forbidden) {
			t.Errorf("frozen media task query still depends on mutable mapping %q\n%s", forbidden, query)
		}
	}
}
