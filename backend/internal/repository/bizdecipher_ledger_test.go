package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
	platformledger "github.com/Wei-Shaw/sub2api/internal/platform/ledger"
	"github.com/stretchr/testify/require"
)

func TestBizDecipherLedgerPostRejectsUserBalanceOverdraftAndRollsBack(t *testing.T) {
	// Given
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	debit, err := billingcontract.ParseDecimal("-8")
	require.NoError(t, err)
	credit, err := billingcontract.ParseDecimal("8")
	require.NoError(t, err)
	postedAt := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	journal := platformledger.Journal{
		ID:             "journal-overdraft",
		EventID:        "event-overdraft",
		IdempotencyKey: "idempotency-overdraft",
		PayloadSHA256:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Kind:           "usage",
		PostedAt:       postedAt,
		Entries: []platformledger.Entry{
			{AccountID: "user-balance", Amount: debit},
			{AccountID: "clearing", Amount: credit},
		},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO bizdecipher_ledger_journals`).
		WithArgs(journal.ID, journal.EventID, journal.IdempotencyKey, journal.PayloadSHA256, journal.Kind, journal.ReversalOf, postedAt).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(journal.ID))
	mock.ExpectExec(`INSERT INTO bizdecipher_ledger_entries`).
		WithArgs(journal.ID, 1, "user-balance", "-8").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE bizdecipher_ledger_projections AS projection[\s\S]+FROM bizdecipher_ledger_accounts AS account[\s\S]+account\.purpose <> 'user_balance'[\s\S]+projection\.balance \+ \$2::numeric >= projection\.reserved_amount`).
		WithArgs("user-balance", "-8", postedAt).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT account\.purpose[\s\S]+FROM bizdecipher_ledger_accounts AS account[\s\S]+JOIN bizdecipher_ledger_projections AS projection`).
		WithArgs("user-balance").
		WillReturnRows(sqlmock.NewRows([]string{"purpose"}).AddRow("user_balance"))
	mock.ExpectRollback()

	// When
	_, err = NewBizDecipherLedgerRepository(db).Post(context.Background(), journal)

	// Then
	require.ErrorIs(t, err, platformledger.ErrInsufficientFunds)
	require.NoError(t, mock.ExpectationsWereMet())
}
