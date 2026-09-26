package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsRequestDetailsFirstTokenLatency(t *testing.T) {
	for _, tc := range []struct{ sort, order string }{
		{"ttft_desc", "first_token_ms DESC NULLS LAST, created_at DESC"},
		{"duration_desc", "duration_ms DESC NULLS LAST, created_at DESC"},
		{"", "created_at DESC"},
	} {
		t.Run(tc.order, func(t *testing.T) {
			db, mock := newSQLMock(t)
			repo := &opsRepository{db: db}
			start := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
			end := start.Add(time.Hour)
			filter := &service.OpsRequestDetailFilter{StartTime: &start, EndTime: &end, Sort: tc.sort, Page: 2, PageSize: 10}
			mock.ExpectQuery(`SELECT COUNT\(1\) FROM combined`).WithArgs(start, end).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(13))
			rows := sqlmock.NewRows([]string{
				"kind", "created_at", "request_id", "platform", "model", "duration_ms", "first_token_ms",
				"status_code", "error_id", "phase", "severity", "message", "user_id", "api_key_id", "account_id", "group_id", "stream",
			}).
				AddRow("success", start, "slow", "openai", "test", 12000, 800, nil, nil, nil, nil, nil, 1, 2, 3, 4, true).
				AddRow("error", start, "zero", "openai", "test", 9000, 0, 502, 5, "upstream", "error", "failed", 1, 2, 3, 4, true).
				AddRow("success", start, "missing", "openai", "test", 5000, nil, nil, nil, nil, nil, nil, 1, 2, 3, 4, false)
			mock.ExpectQuery(`(?s)ul\.first_token_ms AS first_token_ms.*o\.time_to_first_token_ms AS first_token_ms.*ORDER BY `+tc.order+`\s+LIMIT \$3 OFFSET \$4`).
				WithArgs(start, end, 10, 10).WillReturnRows(rows)
			items, total, err := repo.ListRequestDetails(context.Background(), filter)
			require.NoError(t, err)
			require.EqualValues(t, 13, total)
			require.Len(t, items, 3)
			require.Equal(t, 800, *items[0].FirstTokenMs)
			require.Equal(t, 0, *items[1].FirstTokenMs)
			require.Nil(t, items[2].FirstTokenMs)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestOpsRequestDetailsRejectInvalidSort(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	mock.ExpectQuery(`SELECT COUNT\(1\) FROM combined`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	_, _, err := repo.ListRequestDetails(context.Background(), &service.OpsRequestDetailFilter{Sort: "first_token_ms; DROP TABLE usage_logs"})
	require.ErrorContains(t, err, "invalid sort")
	require.NoError(t, mock.ExpectationsWereMet())
}
