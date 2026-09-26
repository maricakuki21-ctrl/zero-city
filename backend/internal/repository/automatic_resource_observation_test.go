package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAutomaticResourcePlanDoesNotOverrideManualControls(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE scheduled_test_plans").WithArgs(int64(1), "mini").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("(?s)INSERT INTO scheduled_test_plans.*WHERE NOT EXISTS.*ON CONFLICT DO NOTHING").WithArgs(int64(1), "mini").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	repo := &scheduledTestPlanRepository{db: db}
	if err := repo.EnsureAutomaticTest(context.Background(), 1, "mini"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticGroupObservationUsesRealSuccessCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now()
	mock.ExpectQuery("(?s)WITH evidence AS.*COUNT.*primary_model").
		WithArgs(int64(2), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"model", "status", "latency", "started", "total", "passed"}).
			AddRow("mini", "failed", 900, now, 4, 3).AddRow("mini", "success", 200, now.Add(-time.Minute), 4, 3))
	repo := &channelMonitorRepository{db: db}
	view, err := repo.ReadGroupAccountObservation(context.Background(), 2, []string{"mini"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Availability7d != 75 || len(view.Timeline) != 2 || view.PrimaryStatus != "failed" {
		t.Fatalf("unexpected view: %#v", view)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
