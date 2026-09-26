package repository

import (
	"context"
	"time"

	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	dbpredicate "github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Delayed usage probes may only update the generation they actually observed.
func (r *accountRepository) SetRateLimitedIfUnchanged(ctx context.Context, id int64, expectedUpdatedAt time.Time, expectedLimitedAt, expectedResetAt *time.Time, newResetAt time.Time) (bool, error) {
	preds := []dbpredicate.Account{dbaccount.IDEQ(id), dbaccount.UpdatedAtEQ(expectedUpdatedAt)}
	if expectedLimitedAt == nil {
		preds = append(preds, dbaccount.RateLimitedAtIsNil())
	} else {
		preds = append(preds, dbaccount.RateLimitedAtEQ(*expectedLimitedAt))
	}
	if expectedResetAt == nil {
		preds = append(preds, dbaccount.RateLimitResetAtIsNil())
	} else {
		preds = append(preds, dbaccount.RateLimitResetAtEQ(*expectedResetAt))
	}
	updated, err := r.client.Account.Update().Where(preds...).SetRateLimitedAt(time.Now()).SetRateLimitResetAt(newResetAt).Save(ctx)
	if err != nil {
		return false, err
	}
	if updated == 0 {
		r.syncSchedulerAccountSnapshot(ctx, id)
		return false, nil
	}
	if err := enqueueSchedulerOutbox(ctx, r.sql, service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
		logger.LegacyPrintf("repository.account", "[SchedulerOutbox] enqueue rate limit failed: account=%d err=%v", id, err)
	}
	r.syncSchedulerAccountSnapshot(ctx, id)
	return true, nil
}
