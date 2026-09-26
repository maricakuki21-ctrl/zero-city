package repository

import (
	"context"
	"math"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Mirrors rollupSharedPoolMetricsFromAccountProbes availability formula.
func computeProbeAvailability(success, total int) float64 {
	if total <= 0 {
		return 0
	}
	raw := float64(success) * 100 / float64(total)
	if raw < 0 {
		return 0
	}
	if raw > 100 {
		return 100
	}
	return math.Round(raw*100) / 100
}

// Mirrors trailing consecutive failures: count failures from newest until a success.
func computeTrailingProbeFailures(newestFirstSuccess []bool) int {
	n := 0
	for _, ok := range newestFirstSuccess {
		if ok {
			break
		}
		n++
	}
	return n
}

func TestComputeProbeAvailabilityFromAccountHistories(t *testing.T) {
	cases := []struct {
		name    string
		success int
		total   int
		want    float64
	}{
		{"empty", 0, 0, 0},
		{"all_ok", 8, 8, 100},
		{"half", 1, 2, 50},
		{"pool16_shape", 12, 12, 100},
		{"almost", 19, 20, 95},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeProbeAvailability(tc.success, tc.total)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestComputeTrailingProbeFailures(t *testing.T) {
	cases := []struct {
		name string
		seq  []bool // newest first
		want int
	}{
		{"all_success", []bool{true, true}, 0},
		{"one_fail", []bool{false, true}, 1},
		{"three_fail", []bool{false, false, false, true}, 3},
		{"empty", nil, 0},
		{"all_fail", []bool{false, false}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeTrailingProbeFailures(tc.seq)
			if got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestQualityScoreNudgeThresholds(t *testing.T) {
	// quality_score bump rules used by rollup SQL
	nudge := func(avail float64, current float64) float64 {
		target := current
		if avail >= 95 {
			if current < 70 {
				target = 70
			}
		} else if avail >= 80 {
			if current < 60 {
				target = 60
			}
		}
		if target > 100 {
			return 100
		}
		if target < current {
			return current
		}
		return target
	}
	if got := nudge(100, 60); got != 70 {
		t.Fatalf("95+ should lift to 70, got %v", got)
	}
	if got := nudge(90, 50); got != 60 {
		t.Fatalf("80+ should lift to 60, got %v", got)
	}
	if got := nudge(70, 55); got != 55 {
		t.Fatalf("below 80 should keep current, got %v", got)
	}
}

func TestApplySharedPoolAccountProbeResultRollupAggregatesEligibleAccounts(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	checkedAt := time.Date(2026, 7, 16, 6, 55, 0, 0, time.UTC)
	input := service.SharedPoolProbeHistoryInput{
		PoolID:       31,
		AccountID:    302,
		OwnerID:      7,
		Success:      false,
		HTTPStatus:   402,
		ErrorType:    "upstream_error",
		ErrorMessage: "deactivated_workspace",
		CheckedAt:    checkedAt,
	}

	mock.ExpectExec("UPDATE shared_pool_accounts SET[\\s\\S]*status = 'offline'").
		WithArgs(int64(31), int64(302), int64(7), checkedAt, false, "upstream_error", "deactivated_workspace", false, false, false, float64(0), 0, 0).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT status, listed").
		WithArgs(int64(31)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
			AddRow("healthy", true, "normal", "", 0))
	mock.ExpectExec("WITH eligible_accounts AS").
		WithArgs(int64(31), sharedPoolObservationFailureThreshold, sharedPoolDelistFailureThreshold, sharedPoolObservationNote, sharedPoolDelistNote, sharedPoolFullCheckPassingScore).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT status, listed").
		WithArgs(int64(31)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
			AddRow("healthy", true, "normal", "", 0))

	require.NoError(t, repo.ApplySharedPoolAccountProbeResult(context.Background(), input))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSharedPoolProbeCandidateStatusesKeepOfflineAccountsRecoverable(t *testing.T) {
	const want = "'active', 'limited', 'testing', 'offline'"
	if sharedPoolProbeCandidateAccountStatuses != want {
		t.Fatalf("candidate statuses = %q, want %q", sharedPoolProbeCandidateAccountStatuses, want)
	}
}

func TestAccountProbeRollupSQLUsesPerAccountHealthQuorum(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	mock.ExpectQuery("SELECT status, listed").
		WithArgs(int64(32)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
			AddRow("healthy", true, "normal", "", 2))
	mock.ExpectExec("WITH eligible_accounts AS[\\s\\S]*latest_per_account AS[\\s\\S]*PARTITION BY account_id[\\s\\S]*MIN\\(atf.consecutive_failures\\)[\\s\\S]*WHEN ph.healthy_accounts > 0 AND sp.status IN[\\s\\S]*WHEN ph.consecutive_failures >= \\$3 THEN FALSE").
		WithArgs(int64(32), sharedPoolObservationFailureThreshold, sharedPoolDelistFailureThreshold, sharedPoolObservationNote, sharedPoolDelistNote, sharedPoolFullCheckPassingScore).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT status, listed").
		WithArgs(int64(32)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
			AddRow("offline", false, "watch", sharedPoolDelistNote, 9))
	mock.ExpectExec("INSERT INTO shared_pool_governance_logs").
		WithArgs(int64(32), sharedPoolDelistGov, sqlmock.AnyArg(), sqlmock.AnyArg(), sharedPoolDelistNote).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, repo.rollupSharedPoolMetricsFromAccountProbes(context.Background(), 32))
	require.NoError(t, mock.ExpectationsWereMet())
}
