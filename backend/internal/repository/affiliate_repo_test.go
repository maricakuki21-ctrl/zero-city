package repository

import (
	"context"
	"os"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateRechargeCapLocksBeforeReadingAndAccruing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := &affiliateRepository{client: client}
	mock.ExpectBegin()
	mock.ExpectQuery("(?s)SELECT user_id FROM user_affiliates.*ORDER BY user_id FOR UPDATE").
		WithArgs(int64(10), int64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(10)))
	mock.ExpectQuery("(?s)SELECT COALESCE\\(SUM\\(amount\\), 0\\).*user_affiliate_ledger").
		WithArgs(int64(10), int64(20)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(9.75))
	mock.ExpectExec("(?s)INSERT INTO user_affiliate_ledger.*ON CONFLICT DO NOTHING").
		WithArgs(int64(10), 0.25, int64(20), int64(30), "recharge_tier1", "order:30").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE user_affiliates SET aff_quota").
		WithArgs(0.25, int64(10)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	result, err := repo.AccrueRechargeInviteRewards(context.Background(), service.RechargeInviteRewardsInput{
		DirectInviterID: 10, InviteeUserID: 20, SourceOrderID: 30,
		BaseRechargeAmount: 100, DirectQuotaRebate: 5, PerInviteeCap: 10,
	})
	require.NoError(t, err)
	require.Equal(t, 0.25, result.DirectQuotaRebate)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAffiliateUserOverviewSQLIncludesMaturedFrozenQuota(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateUserOverviewSQL), " ")

	require.Contains(t, query, "ua.aff_quota + COALESCE(matured.matured_frozen_quota, 0)")
	require.Contains(t, query, "frozen_until <= NOW()")
}

func TestAffiliateRecordQueriesUseLedgerAuditFields(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	content := string(source)

	require.Contains(t, content, "JOIN payment_orders po ON po.id = ual.source_order_id")
	require.Contains(t, content, "ual.amount::double precision")
	require.Contains(t, content, "ual.balance_after::double precision")
	require.NotContains(t, content, "parseAffiliateRebateAmount")
	require.NotContains(t, content, `"current_balance": "u.balance"`)
}

func TestInviteRewardPromoAdminGrantUseCreditBalanceButAffiliateRebateUsesBalance(t *testing.T) {
	inviteRewardSource, err := os.ReadFile("bizdecipher_repo.go")
	require.NoError(t, err)
	inviteRewardContent := string(inviteRewardSource)

	grantStart := strings.Index(inviteRewardContent, "func (r *bizDecipherRepository) GrantInviteRewardTx")
	require.NotEqual(t, -1, grantStart)
	grantEnd := strings.Index(inviteRewardContent[grantStart:], "// ---------------------------------------------------------------------------")
	require.NotEqual(t, -1, grantEnd)
	grantBody := inviteRewardContent[grantStart : grantStart+grantEnd]
	require.Contains(t, grantBody, "UPDATE users SET credit_balance = credit_balance + $1")
	require.Contains(t, grantBody, "RETURNING credit_balance")
	require.NotContains(t, grantBody, "UPDATE users SET balance = balance + $1")

	adminGrantStart := strings.Index(inviteRewardContent, "func (r *bizDecipherRepository) GrantCredit")
	require.NotEqual(t, -1, adminGrantStart)
	adminGrantEnd := strings.Index(inviteRewardContent[adminGrantStart:], "func (r *bizDecipherRepository) EnsureStarterCreditLedger")
	require.NotEqual(t, -1, adminGrantEnd)
	adminGrantBody := inviteRewardContent[adminGrantStart : adminGrantStart+adminGrantEnd]
	require.Contains(t, adminGrantBody, "UPDATE users SET credit_balance = credit_balance + $1")
	require.Contains(t, adminGrantBody, "RETURNING credit_balance")
	require.NotContains(t, adminGrantBody, "UPDATE users SET balance = balance + $1")

	starterStart := strings.Index(inviteRewardContent, "func (r *bizDecipherRepository) EnsureStarterCreditLedger")
	require.NotEqual(t, -1, starterStart)
	starterEnd := strings.Index(inviteRewardContent[starterStart:], "func (r *bizDecipherRepository) GetPoolStatus")
	require.NotEqual(t, -1, starterEnd)
	starterBody := inviteRewardContent[starterStart : starterStart+starterEnd]
	require.Contains(t, starterBody, "SELECT credit_balance FROM users")
	require.NotContains(t, starterBody, "SELECT balance FROM users")

	promoStart := strings.Index(inviteRewardContent, "func (r *bizDecipherRepository) ClaimPromoTx")
	require.NotEqual(t, -1, promoStart)
	promoEnd := strings.Index(inviteRewardContent[promoStart:], "func (r *bizDecipherRepository) ListPromoCampaigns")
	require.NotEqual(t, -1, promoEnd)
	promoBody := inviteRewardContent[promoStart : promoStart+promoEnd]
	require.Contains(t, promoBody, "UPDATE users SET credit_balance = credit_balance + $1")
	require.Contains(t, promoBody, "RETURNING credit_balance")
	require.Contains(t, promoBody, "SELECT credit_balance FROM users")
	require.NotContains(t, promoBody, "UPDATE users SET balance = balance + $1")
	require.NotContains(t, promoBody, "SELECT balance FROM users")

	affiliateSource, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	affiliateContent := string(affiliateSource)
	transferStart := strings.Index(affiliateContent, "func (r *affiliateRepository) TransferQuotaToBalance")
	require.NotEqual(t, -1, transferStart)
	transferEnd := strings.Index(affiliateContent[transferStart:], "func (r *affiliateRepository) ListInvitees")
	require.NotEqual(t, -1, transferEnd)
	transferBody := affiliateContent[transferStart : transferStart+transferEnd]
	require.Contains(t, transferBody, "AddBalance(transferred)")
	require.Contains(t, transferBody, "queryUserBalance")
	require.Contains(t, transferBody, "snapshot.BalanceAfter")
	require.NotContains(t, transferBody, "AddCreditBalance(transferred)")
	require.NotContains(t, transferBody, "AddTotalRecharged(transferred)")
}
