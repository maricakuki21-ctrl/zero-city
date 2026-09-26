//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Uses a disposable database because accepted prices and supply mappings are
// append-only audit records. Never run these fixture writes on the site's DB.
func TestCanonicalSettlementClosedLoop(t *testing.T) {
	runCanonicalSettlementClosedLoop(t, false)
}

func TestCanonicalSettlementBuyerSurchargeClosedLoop(t *testing.T) {
	runCanonicalSettlementClosedLoop(t, true)
}

func runCanonicalSettlementClosedLoop(t *testing.T, surcharge bool) {
	t.Helper()
	buyerCost, ownerIncome, feeMode := 0.0002, 0.00018, ""
	if surcharge {
		buyerCost, ownerIncome, feeMode = 0.00022, 0.0002, service.SharedPoolFeeModeBuyerSurcharge
	}
	ctx := context.Background()
	var database string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT current_database()`).Scan(&database))
	if !strings.HasPrefix(database, "bizdecipher_settlement_acceptance_test_") {
		t.Skip("requires disposable settlement acceptance database")
	}
	id := func(query string, args ...any) int64 {
		t.Helper()
		var value int64
		require.NoError(t, integrationDB.QueryRowContext(ctx, query, args...).Scan(&value))
		return value
	}
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := integrationDB.ExecContext(ctx, query, args...)
		require.NoError(t, err)
	}
	owner := id(`INSERT INTO users(email,password_hash,balance) VALUES('settlement-owner@example.test','test-only',0) RETURNING id`)
	member := id(`INSERT INTO users(email,password_hash,balance) VALUES('settlement-member@example.test','test-only',5) RETURNING id`)
	group := id(`INSERT INTO groups(name) VALUES('settlement-acceptance') RETURNING id`)
	canonicalAccount := id(`INSERT INTO accounts(name,platform,type) VALUES('settlement-upstream','openai','apikey') RETURNING id`)
	// Pin the fixture fee rather than inheriting the operator's schema default.
	// A schema-only test database can legitimately default to a different rate.
	pool := id(`INSERT INTO shared_pools(owner_id,name,listed,status,native_onboarding_state,platform_fee_percent)
		VALUES($1,'settlement-acceptance',FALSE,'offline','legacy_existing',10) RETURNING id`, owner)
	poolAccount := id(`INSERT INTO shared_pool_accounts(id,pool_id,owner_id,name)
		VALUES($1,$2,$3,'settlement-local') RETURNING id`, canonicalAccount+100000, pool, owner)
	exec(`INSERT INTO shared_pool_sub2_bindings(pool_id,owner_id,canonical_group_id,lifecycle)
		VALUES($1,$2,$3,'archived')`, pool, owner, group)
	exec(`INSERT INTO shared_pool_supply_dispositions(source_kind,source_id,pool_id,owner_id,
		canonical_account_id,disposition,reason_code,credential_hash,source_checksum)
		VALUES('pool_account',$1,$2,$3,$4,'mapped','acceptance',repeat('a',64),repeat('b',64))`,
		poolAccount, pool, owner, canonicalAccount)
	key := id(`INSERT INTO api_keys(user_id,key,name,status) VALUES($1,'acceptance-only','acceptance','disabled') RETURNING id`, member)
	access := id(`INSERT INTO shared_pool_access_keys(pool_id,user_id,api_key_id,status)
		VALUES($1,$2,$3,'disabled') RETURNING id`, pool, member, key)
	model := id(`INSERT INTO shared_pool_models(pool_id,model_name,pricing_source,pricing_status)
		VALUES($1,'acceptance-model','owner_custom','ready') RETURNING id`, pool)
	endpoint := id(`SELECT id FROM shared_pool_model_endpoints WHERE pool_model_id=$1 AND endpoint_type='responses'`, model)
	price := id(`INSERT INTO shared_pool_price_versions(pool_id,pool_model_id,endpoint_id,version_no,
		config_version,operation_id,source,billing_mode,input_price,output_price,multiplier,effective_from,price_hash)
		VALUES($1,$2,$3,1,1,'acceptance-old','owner_custom','token',0.000001,0.000002,1,NOW()-interval '1 hour',repeat('c',64)) RETURNING id`, pool, model, endpoint)
	inPrice, outPrice := 1e-6, 2e-6
	snapshot, err := service.NewCanonicalUsageSettlementSnapshot(&service.SharedPoolPriceQuote{
		FeeMode: feeMode, PlatformFeePercent: 10,
		PriceVersionID: price, PoolID: pool, PoolModelID: model, EndpointID: endpoint,
		ModelName: "acceptance-model", EndpointType: "responses", PricingSource: "owner_custom",
		Multiplier: 1, BasePrice: service.SharedPoolPriceComponents{
			BillingMode: "token", Currency: "USD", InputPrice: &inPrice, OutputPrice: &outPrice,
		},
	})
	require.NoError(t, err)
	// A newer price must not affect this already accepted request.
	exec(`INSERT INTO shared_pool_price_versions(pool_id,pool_model_id,endpoint_id,version_no,
		config_version,operation_id,source,billing_mode,input_price,output_price,multiplier,effective_from,price_hash)
		VALUES($1,$2,$3,2,1,'acceptance-new','owner_custom','token',1,2,1,NOW(),repeat('d',64))`, pool, model, endpoint)
	requestID := fmt.Sprintf("acceptance-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	event, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy:    corecontracts.BillingPolicyBizDecipherLedger,
		RequestID: requestID, UsageReference: fmt.Sprintf("sub2-usage:%d:%s", key, requestID),
		UserID: member, AccountID: canonicalAccount, GroupID: group, Model: "acceptance-model",
		Protocol: "/v1/responses", Units: corecontracts.ExactUsageUnits{InputTokens: 100, OutputTokens: 50},
		CommitState: corecontracts.CanonicalUsageCommitStateUsageCommitted, CommittedAt: now, FinalizedAt: now,
	})
	require.NoError(t, err)
	producer := &usageBillingRepository{db: integrationDB}
	cmd := &service.UsageBillingCommand{RequestID: requestID, APIKeyID: key, UserID: member, AccountID: canonicalAccount}
	result, err := producer.ApplyWithCanonicalUsage(ctx, cmd, event, snapshot)
	require.NoError(t, err)
	require.True(t, result.Applied)
	_, err = producer.ApplyWithCanonicalUsage(ctx, cmd, event, nil)
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict, "dropping the accepted quote on replay must fail")
	repo := &bizDecipherRepository{db: integrationDB}
	svc := service.NewBizDecipherService(repo, nil, nil)
	// Two drainers must not double debit, even when both observe the pending row.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			summary, err := svc.SettlePendingCanonicalSharedPoolUsage(ctx, 50)
			if err == nil && (summary.Failed != 0 || summary.Deferred != 0) {
				err = fmt.Errorf("unexpected settlement summary: %+v", summary)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.InDelta(t, 5-buyerCost, sharedPoolBillingFloat(t, `SELECT balance::float8 FROM users WHERE id=$1`, member), 1e-9)
	require.InDelta(t, ownerIncome, sharedPoolBillingFloat(t, `SELECT available_amount::float8 FROM shared_pool_owner_wallets WHERE owner_id=$1`, owner), 1e-9)
	require.Equal(t, 1, sharedPoolBillingInt(t, `SELECT count(*) FROM shared_pool_balance_ledger WHERE user_id=$1 AND source_id=$2 AND status='posted'`, member, requestID))
	require.Equal(t, "published", sharedPoolBillingString(t, `SELECT status FROM canonical_usage_outbox WHERE event_id=$1`, event.EventID()))
	require.Equal(t, int(poolAccount), sharedPoolBillingInt(t, `SELECT account_id FROM shared_pool_balance_ledger WHERE user_id=$1 AND source_id=$2`, member, requestID))
	require.InDelta(t, buyerCost, sharedPoolBillingFloat(t, `SELECT total_used::float8 FROM shared_pool_access_keys WHERE id=$1`, access), 1e-9)
	// Simulate a crash after money commit but before outbox publication.
	exec(`UPDATE canonical_usage_outbox SET status='pending',published_at=NULL WHERE event_id=$1`, event.EventID())
	summary, err := svc.SettlePendingCanonicalSharedPoolUsage(ctx, 50)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Settled)
	require.InDelta(t, 5-buyerCost, sharedPoolBillingFloat(t, `SELECT balance::float8 FROM users WHERE id=$1`, member), 1e-9)
	// An old event without a quote must be delayed, allowing the next event
	// to progress even with a batch limit of one.
	oldRequest := requestID + "-old"
	oldEvent, err := corecontracts.NewCanonicalUsageFinalized(corecontracts.CanonicalUsageFinalizedInput{
		Policy:    corecontracts.BillingPolicyBizDecipherLedger,
		RequestID: oldRequest, UsageReference: fmt.Sprintf("sub2-usage:%d:%s", key, oldRequest),
		UserID: member, AccountID: canonicalAccount, GroupID: group, Model: "acceptance-model",
		Protocol: "/v1/responses", Units: corecontracts.ExactUsageUnits{InputTokens: 100},
		CommitState: corecontracts.CanonicalUsageCommitStateUsageCommitted, CommittedAt: now, FinalizedAt: now,
	})
	require.NoError(t, err)
	oldCmd := &service.UsageBillingCommand{RequestID: oldRequest, APIKeyID: key, UserID: member, AccountID: canonicalAccount}
	_, err = producer.ApplyWithCanonicalUsage(ctx, oldCmd, oldEvent, nil)
	require.NoError(t, err)
	_, err = producer.ApplyWithCanonicalUsage(ctx, oldCmd, oldEvent, snapshot)
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict, "adding a quote to old history is not a duplicate")
	exec(`UPDATE canonical_usage_outbox SET created_at=NOW()-interval '1 day' WHERE event_id=$1`, oldEvent.EventID())
	exec(`UPDATE canonical_usage_outbox SET status='pending',published_at=NULL WHERE event_id=$1`, event.EventID())
	summary, err = svc.SettlePendingCanonicalSharedPoolUsage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Deferred)
	summary, err = svc.SettlePendingCanonicalSharedPoolUsage(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, summary.Settled)
	require.Equal(t, "pending", sharedPoolBillingString(t, `SELECT status FROM canonical_usage_outbox WHERE event_id=$1`, oldEvent.EventID()))
}
