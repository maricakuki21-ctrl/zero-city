package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func disputeOrderRows(status string) *sqlmock.Rows {
	columns := make([]string, 26)
	for i := range columns {
		columns[i] = string(rune('a' + i))
	}
	now := time.Now()
	values := []driver.Value{int64(1), int64(2), int64(3), "Listing", int64(4), "Buyer", int64(5), "Seller",
		status, "scope", "not money", "tomorrow", 1, "", "", "dispute", "",
		now, now, nil, nil, nil, nil, now, now, true}
	return sqlmock.NewRows(columns).AddRow(values...)
}

func TestMarketplaceDisputeRepositoryRejectsNonAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &marketplaceRepository{db: db}
	for _, operation := range []string{"list", "detail", "resolve"} {
		if operation == "resolve" {
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT buyer_user_id,seller_user_id").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"buyer", "seller"}).AddRow(4, 5))
			mock.ExpectQuery("SELECT id,role,").WillReturnRows(sqlmock.NewRows([]string{"id", "role", "active"}).AddRow(4, "user", true).AddRow(5, "user", true).AddRow(7, "user", true))
		} else {
			mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(false))
		}
		if operation == "resolve" {
			mock.ExpectRollback()
		}
		switch operation {
		case "list":
			_, err = repo.ListMarketplaceDisputes(context.Background(), 7, service.MarketplacePageQuery{Limit: 20})
		case "detail":
			_, err = repo.GetMarketplaceDispute(context.Background(), 1, 7)
		case "resolve":
			_, err = repo.ResolveMarketplaceDispute(context.Background(), 1, 7, service.MarketplaceDisputeResolutionInput{Outcome: "confirmed", Reason: "reason"})
		}
		require.ErrorIs(t, err, service.ErrMarketplaceOrderRoleDenied)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarketplaceDisputeRepositoryResolutionAtomicity(t *testing.T) {
	for _, outcome := range []string{"confirmed", "canceled"} {
		for _, auditFails := range []bool{false, true} {
			t.Run(outcome+map[bool]string{false: "-success", true: "-audit-failure"}[auditFails], func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				defer db.Close()
				repo := &marketplaceRepository{db: db}
				mock.ExpectBegin()
				expectMarketplaceAdminLocks(mock)
				mock.ExpectQuery("SELECT status FROM marketplace_orders WHERE id = \\$1 FOR UPDATE").WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("disputed"))
				mock.ExpectQuery("SELECT .* FROM marketplace_order_funds").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"order_id", "amount", "currency", "status", "operation_id", "paid_at", "released_at", "refunded_at"}))
				if outcome == "confirmed" {
					mock.ExpectQuery("SELECT legacy_cooperation").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"legacy_cooperation"}).AddRow(true))
				}
				mock.ExpectExec(regexp.QuoteMeta(`UPDATE marketplace_orders SET status = $2::varchar, updated_at = NOW(),
					canceled_at = CASE WHEN $2::varchar = 'canceled'::varchar THEN NOW() ELSE NULL END,
					delivered_at = CASE WHEN $2::varchar = 'confirmed'::varchar THEN NULL ELSE delivered_at END,
					accepted_at = CASE WHEN $2::varchar = 'confirmed'::varchar THEN NULL ELSE accepted_at END
					WHERE id = $1 AND status = 'disputed'`)).
					WithArgs(int64(1), outcome).WillReturnResult(sqlmock.NewResult(0, 1))
				audit := mock.ExpectExec("INSERT INTO marketplace_order_events").
					WithArgs(int64(1), int64(7), outcome, "reason")
				if auditFails {
					audit.WillReturnError(errors.New("audit failed"))
					mock.ExpectRollback()
				} else {
					audit.WillReturnResult(sqlmock.NewResult(8, 1))
					mock.ExpectCommit()
					mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
					mock.ExpectQuery("SELECT .*").WithArgs(int64(1)).WillReturnRows(disputeOrderRows(outcome))
					mock.ExpectQuery("SELECT e.id").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "order_id", "actor_user_id", "actor_name", "event", "from", "to", "note", "created_at"}).
						AddRow(8, 1, 7, "Admin", "admin_resolve_dispute", "disputed", outcome, "reason", time.Now()))
				}
				order, err := repo.ResolveMarketplaceDispute(context.Background(), 1, 7, service.MarketplaceDisputeResolutionInput{Outcome: outcome, Reason: "reason"})
				if auditFails {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, outcome, order.Status)
					require.Equal(t, int64(7), order.Events[0].ActorUserID)
					require.Equal(t, "reason", order.Events[0].Note)
				}
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

func TestMarketplaceDisputeRepositoryRejectsAlreadyResolvedUnderLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &marketplaceRepository{db: db}
	mock.ExpectBegin()
	expectMarketplaceAdminLocks(mock)
	mock.ExpectQuery("SELECT status FROM marketplace_orders WHERE id = \\$1 FOR UPDATE").WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("confirmed"))
	mock.ExpectRollback()
	_, err = repo.ResolveMarketplaceDispute(context.Background(), 1, 7, service.MarketplaceDisputeResolutionInput{Outcome: "canceled", Reason: "duplicate"})
	require.ErrorIs(t, err, service.ErrMarketplaceOrderInvalidState)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectMarketplaceAdminLocks(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT buyer_user_id,seller_user_id").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"buyer", "seller"}).AddRow(4, 5))
	mock.ExpectQuery("SELECT id,role,").WillReturnRows(sqlmock.NewRows([]string{"id", "role", "active"}).AddRow(4, "user", true).AddRow(5, "user", true).AddRow(7, "admin", true))
}
