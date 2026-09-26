// Test-only rollback probe; never connects to an arbitrary production database.
package main

import (
	"context"
	"fmt"
	"math"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
)

func main() {
	if err := verify(); err != nil {
		panic(err)
	}
}

func verify() error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client, err := dbent.Open(dialect.Postgres, "host=127.0.0.1 port=25436 user=postgres dbname=bizdecipher_test sslmode=disable")
	if err != nil {
		return err
	}
	defer client.Close()
	tx, err := client.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ctx = dbent.NewTxContext(ctx, tx)
	repo := repository.NewAffiliateRepository(tx.Client(), nil)
	stamp := time.Now().UnixNano()
	var ids []int64
	for i := 0; i < 3; i++ {
		u, err := tx.Client().User.Create().SetEmail(fmt.Sprintf("growth-rollback-%d-%d@example.test", stamp, i)).
			SetPasswordHash("unusable-test-password").SetUsername("Rollback probe").Save(ctx)
		if err != nil {
			return err
		}
		ids = append(ids, u.ID)
		if _, err := repo.EnsureUserAffiliate(ctx, u.ID); err != nil {
			return err
		}
	}
	for i := 1; i < 3; i++ {
		if _, err := repo.BindInviter(ctx, ids[i], ids[i-1]); err != nil {
			return err
		}
	}
	order, err := tx.Client().PaymentOrder.Create().SetUserID(ids[2]).
		SetUserEmail("rollback@example.test").SetUserName("Rollback").SetAmount(100).SetPayAmount(100).
		SetRechargeCode(fmt.Sprintf("probe-%d", stamp)).SetOutTradeNo(fmt.Sprintf("probe-%d", stamp)).
		SetPaymentType("test").SetPaymentTradeNo("test").SetOrderType("balance").
		SetClientIP("127.0.0.1").SetSrcHost("rollback-test").
		SetStatus("COMPLETED").SetExpiresAt(time.Now().Add(time.Hour)).SetCompletedAt(time.Now()).Save(ctx)
	if err != nil {
		return err
	}
	overview, err := repo.GetAffiliateUserOverview(ctx, ids[1])
	if err != nil || overview == nil {
		return fmt.Errorf("overview unavailable: %w", err)
	}
	if overview.QualifiedPaidInvitees != 1 {
		return fmt.Errorf("qualified count: %d", overview.QualifiedPaidInvitees)
	}
	svc := service.NewAffiliateService(repo, nil, nil, nil)
	reward, err := svc.AccrueInviteRebateForOrder(ctx, ids[2], 100, &order.ID)
	if err != nil {
		return err
	}
	if reward == nil || math.Abs(reward.DirectQuotaRebate-5) > 1e-8 ||
		math.Abs(reward.Tier2QuotaRebate-1) > 1e-8 || reward.DirectCreditReward != 0 {
		return fmt.Errorf("unexpected reward: %+v", reward)
	}
	repeated, err := svc.AccrueInviteRebateForOrder(ctx, ids[2], 100, &order.ID)
	if err != nil || repeated == nil || repeated.HasReward() {
		return fmt.Errorf("repeat not idempotent: %+v %w", repeated, err)
	}
	transferred, _, err := repo.TransferQuotaToBalance(ctx, ids[1])
	if err != nil || math.Abs(transferred-5) > 1e-8 {
		return fmt.Errorf("transfer: %v %w", transferred, err)
	}
	if err := tx.Rollback(); err != nil {
		return err
	}
	fmt.Println("PASS real PostgreSQL: qualified count, direct 5 + indirect 1, repeat no reward, transfer 5; transaction rolled back")
	return nil
}
