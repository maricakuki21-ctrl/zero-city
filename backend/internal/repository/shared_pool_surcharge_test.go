package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestSharedPoolBuyerSurchargeWalletSettlement(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		snapshot               string
		cost, hold, owner, fee float64
	}{
		{"surcharge", `{"price_version_id":55,"fee_mode":"buyer_surcharge_v1","platform_fee_percent":10}`, 11, 11, 10, 1},
		{"refund_unused_hold", `{"price_version_id":55,"fee_mode":"buyer_surcharge_v1","platform_fee_percent":10}`, 11, 22, 10, 1},
		{"capped", `{"price_version_id":55,"fee_mode":"buyer_surcharge_v1","platform_fee_percent":10}`, 22, 11, 10, 1},
		{"legacy", `{"price_version_id":55}`, 10, 10, 2.5, 7.5},
		{"zero_fee", `{"price_version_id":55,"fee_mode":"buyer_surcharge_v1","platform_fee_percent":0}`, 10, 10, 10, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &bizDecipherRepository{db: db}
			input := sharedPoolPendingUsageInput("surcharge:" + tc.name)
			input.Cost, input.PriceSnapshot = tc.cost, json.RawMessage(tc.snapshot)
			buyer := tc.owner + tc.fee
			note := "共享池 API 调用 · 池 #22"
			failure := ""
			if tc.cost > tc.hold {
				note += "（上游用量超过预授权，已按预授权上限结算）"
				failure = "reported_cost_exceeded_hold"
			}
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT authority_epoch FROM shared_pool_cutover_control`).
				WillReturnRows(sqlmock.NewRows([]string{"authority_epoch"}).AddRow(1))
			mock.ExpectExec(`SELECT set_config`).
				WithArgs("1").WillReturnResult(sqlmock.NewResult(0, 1))
			expectSharedPoolReservationLookup(mock, input, "forwarding", tc.hold)
			mock.ExpectQuery(`SELECT owner_id, platform_fee_percent FROM shared_pools`).
				WithArgs(input.PoolID).
				WillReturnRows(sqlmock.NewRows([]string{"owner_id", "platform_fee_percent"}).AddRow(int64(99), 75.0))
			mock.ExpectQuery(`SELECT hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, effective_from`).
				WithArgs(input.PoolID, sharedPoolReservationReservedAtForTest).WillReturnError(sql.ErrNoRows)
			mock.ExpectQuery(`INSERT INTO shared_pool_balance_ledger`).
				WithArgs(input.UserID, input.PoolID, input.AccountID, input.RequestID, "共享池 API 调用 · 池 #22", input.PriceVersionID, input.PricingSource, jsonTextArg(input.PriceSnapshot)).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(700)))
			mock.ExpectQuery(`UPDATE users(?s).*balance = balance \+ \(\$1 - \$2\)`).
				WithArgs(tc.hold, buyer, input.UserID).
				WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(100.0))
			mock.ExpectExec(`UPDATE shared_pool_usage_reservations`).
				WithArgs(int64(7), tc.cost, buyer, failure).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(`UPDATE shared_pool_balance_ledger SET amount`).
				WithArgs(int64(700), -buyer, 100.0, note, tc.fee, tc.owner, input.PriceVersionID, input.PricingSource, jsonTextArg(input.PriceSnapshot)).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(`SELECT id FROM users`).
				WithArgs(int64(99)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
			mock.ExpectQuery(`SELECT name, owner_label FROM shared_pools`).
				WithArgs(input.PoolID).WillReturnRows(sqlmock.NewRows([]string{"name", "owner_label"}).AddRow("pool", "owner"))
			mock.ExpectExec(`INSERT INTO shared_pool_owner_wallets`).
				WithArgs(int64(99)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(`(?s)INSERT INTO shared_pool_owner_earnings_ledger .*RETURNING id, available_after`).
				WithArgs(int64(99), input.PoolID, input.AccountID, input.PriceVersionID, input.RequestID, input.RequestID,
					"pool", "owner", input.Model, input.PricingSource, buyer, tc.fee, tc.owner, "share_pool_payout").
				WillReturnRows(sqlmock.NewRows([]string{"id", "available_after"}).AddRow(int64(91), 0.0))
			mock.ExpectQuery(`(?s)UPDATE shared_pool_owner_wallets.*RETURNING available_amount`).
				WithArgs(int64(99), tc.owner).
				WillReturnRows(sqlmock.NewRows([]string{"available_amount"}).AddRow(tc.owner))
			mock.ExpectExec(`UPDATE shared_pool_owner_earnings_ledger`).
				WithArgs(int64(91), tc.owner, tc.owner).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(`INSERT INTO shared_pool_balance_ledger`).
				WithArgs(int64(99), input.PoolID, input.AccountID, "share_pool_payout", input.RequestID, tc.owner, tc.owner,
					"共享池 API 分润 · 池 #22", "pool", "owner", input.Model, input.PriceVersionID, int64(91)).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(701)))
			mock.ExpectExec(`UPDATE shared_pool_access_keys`).WithArgs(buyer, input.AccessKeyID).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(`UPDATE pool_seat_bindings`).WithArgs(input.PoolID, input.UserID).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(`UPDATE shared_pools SET total_calls`).WithArgs(input.PoolID).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			require.NoError(t, repo.RecordSharedPoolUsageTx(context.Background(), input))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
