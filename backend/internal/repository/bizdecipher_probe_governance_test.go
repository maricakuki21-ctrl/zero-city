package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestIsSharedPoolFullProbeType verifies which probe types are treated as
// full checks for auto-governance (Behavior 1 — auto-list).
func TestIsSharedPoolFullProbeType(t *testing.T) {
	cases := []struct {
		probeType string
		want      bool
	}{
		{"manual", true},
		{"publish_gate", true},
		{"scheduled_full", true},
		{"scheduled", false},
		{"", false},
		{"unknown", false},
		// case-insensitive
		{"MANUAL", true},
		{"Publish_Gate", true},
		{"Scheduled_Full", true},
		{"Scheduled", false},
		// whitespace trimming
		{" manual ", true},
		{" scheduled ", false},
	}
	for _, tc := range cases {
		got := isSharedPoolFullProbeType(tc.probeType)
		if got != tc.want {
			t.Errorf("isSharedPoolFullProbeType(%q) = %v, want %v", tc.probeType, got, tc.want)
		}
	}
}

// TestSharedPoolProbeAllowsAutoListing verifies the complete auto-listing
// contract shared by pool-level and account-level probe aggregation.
func TestSharedPoolProbeAllowsAutoListing(t *testing.T) {
	cases := []struct {
		name       string
		success    bool
		probeType  string
		gatePassed bool
		score      float64
		wantList   bool
	}{
		{"manual/score=100", true, "manual", true, 100.0, true},
		{"publish_gate/score=70", true, "publish_gate", true, 70.0, true},
		{"scheduled_full/score=95", true, "scheduled_full", true, 95.0, true},
		{"manual/score=69.9", true, "manual", true, 69.9, false},
		{"manual/gate_failed", true, "manual", false, 100.0, false},
		{"manual/history_failed", false, "manual", true, 100.0, false},
		{"scheduled/score=100", true, "scheduled", true, 100.0, false},
		{"empty/score=100", true, "", true, 100.0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := service.SharedPoolProbeHistoryInput{
				Success:   tc.success,
				ProbeType: tc.probeType,
				Metadata: service.SharedPoolProbeMetadata{
					GatePassed:     tc.gatePassed,
					FullCheckScore: tc.score,
				},
			}
			if got := sharedPoolProbeAllowsAutoListing(input); got != tc.wantList {
				t.Errorf("auto-list condition for success=%v probeType=%q gatePassed=%v score=%.1f: got %v, want %v",
					tc.success, tc.probeType, tc.gatePassed, tc.score, got, tc.wantList)
			}
		})
	}
}

// TestObservationAndDelistThresholds verifies the two-step failure policy:
// 3 failures enter observation; 9 failures delist the pool.
func TestApplySharedPoolProbeResultPassingFullCheckUsesResolvedAutoListFlag(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	checkedAt := time.Date(2026, 7, 15, 5, 30, 0, 0, time.UTC)
	input := service.SharedPoolProbeHistoryInput{
		PoolID:    125,
		Success:   true,
		ProbeType: "manual",
		LatencyMs: 42,
		CheckedAt: checkedAt,
		Metadata: service.SharedPoolProbeMetadata{
			GatePassed:     true,
			FullCheckScore: 100,
		},
	}

	mock.ExpectQuery("SELECT status, listed").
		WithArgs(int64(125)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
			AddRow("offline", false, "normal", "", 9))
	mock.ExpectExec("UPDATE shared_pools SET").
		WithArgs(int64(125), checkedAt, 42, true, sharedPoolObservationNote, sharedPoolDelistNote).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.ApplySharedPoolProbeResult(context.Background(), input))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplySharedPoolProbeResultScheduledSuccessDoesNotAutoList(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &bizDecipherRepository{db: db}
	checkedAt := time.Date(2026, 7, 15, 5, 31, 0, 0, time.UTC)
	input := service.SharedPoolProbeHistoryInput{
		PoolID:    126,
		Success:   true,
		ProbeType: "scheduled",
		CheckedAt: checkedAt,
		Metadata: service.SharedPoolProbeMetadata{
			GatePassed:     true,
			FullCheckScore: 100,
		},
	}

	mock.ExpectQuery("SELECT status, listed").
		WithArgs(int64(126)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "listed", "governance_status", "status_note", "consecutive_probe_failures"}).
			AddRow("offline", false, "normal", "", 9))
	mock.ExpectExec("UPDATE shared_pools SET").
		WithArgs(int64(126), checkedAt, 0, false, sharedPoolObservationNote, sharedPoolDelistNote).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.ApplySharedPoolProbeResult(context.Background(), input))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestObservationAndDelistThresholds(t *testing.T) {
	cases := []struct {
		name                    string
		prevConsecutiveFailures int
		wantObservation         bool
		wantDelist              bool
	}{
		{"0_failures_before→1_after", 0, false, false},
		{"1_failure_before→2_after", 1, false, false},
		{"2_failures_before→3_after", 2, true, false},
		{"7_failures_before→8_after", 7, true, false},
		{"8_failures_before→9_after", 8, true, true},
		{"9_failures_before→10_after", 9, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newFailures := tc.prevConsecutiveFailures + 1
			gotObservation := newFailures >= sharedPoolObservationFailureThreshold
			gotDelist := newFailures >= sharedPoolDelistFailureThreshold
			if gotObservation != tc.wantObservation || gotDelist != tc.wantDelist {
				t.Errorf("thresholds for prev=%d new=%d: observation=%v/%v delist=%v/%v",
					tc.prevConsecutiveFailures, newFailures,
					gotObservation, tc.wantObservation, gotDelist, tc.wantDelist)
			}
		})
	}
}

func TestSharedPoolAutoObservationManaged(t *testing.T) {
	cases := []struct {
		name   string
		status string
		note   string
		want   bool
	}{
		{"normal", "normal", "", true},
		{"boosted", "boosted", "manual boost note", true},
		{"system observation", "watch", sharedPoolObservationNote, true},
		{"system delist", "watch", sharedPoolDelistNote, true},
		{"manual watch", "watch", "管理员人工观察：等待补充证据", false},
		{"empty manual watch", "watch", "", false},
		{"suppressed", "suppressed", "manual", false},
		{"banned", "banned", "manual", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sharedPoolAutoObservationManaged(tc.status, tc.note); got != tc.want {
				t.Fatalf("sharedPoolAutoObservationManaged(%q, %q) = %v, want %v", tc.status, tc.note, got, tc.want)
			}
		})
	}
}
