package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type sharedPoolUsageReviewRepoStub struct {
	BizDecipherRepository
	prepareInput  *ResolveSharedPoolUsageReviewInput
	prepareInputs []ResolveSharedPoolUsageReviewInput
	recorded      *SharedPoolUsageInput
	listState     string
	listBeforeID  int64
	listLimit     int
}

func (r *sharedPoolUsageReviewRepoStub) ListSharedPoolUsageReviews(_ context.Context, state string, beforeID int64, limit int) (*SharedPoolUsageReviewPage, error) {
	r.listState = state
	r.listBeforeID = beforeID
	r.listLimit = limit
	return &SharedPoolUsageReviewPage{Items: []SharedPoolUsageReview{}}, nil
}

func (r *sharedPoolUsageReviewRepoStub) PrepareSharedPoolUsageReviewResolutionTx(_ context.Context, input ResolveSharedPoolUsageReviewInput) (*PreparedSharedPoolUsageReviewResolution, error) {
	copyInput := input
	r.prepareInput = &copyInput
	r.prepareInputs = append(r.prepareInputs, copyInput)
	review := &SharedPoolUsageReview{ReservationID: input.ReservationID, ReservationStatus: "released"}
	if input.Action == SharedPoolUsageReviewActionRelease {
		return &PreparedSharedPoolUsageReviewResolution{Review: review}, nil
	}
	amount := 1.5
	if input.Amount != nil {
		amount = *input.Amount
	}
	review.ReservationStatus = "settlement_pending"
	return &PreparedSharedPoolUsageReviewResolution{
		Review: review,
		Usage: &SharedPoolUsageInput{
			AccessKeyID: 11, PoolID: 22, UserID: 33, Cost: amount,
			RequestID: "review-request", Success: true, PriceVersionID: 44,
			PricingSource: SharedPoolPricingSourceOfficial,
		},
	}, nil
}

func (r *sharedPoolUsageReviewRepoStub) RecordSharedPoolUsageTx(_ context.Context, input SharedPoolUsageInput) error {
	copyInput := input
	r.recorded = &copyInput
	return nil
}

func TestAdminListSharedPoolUsageReviewsDefaultsAndClamps(t *testing.T) {
	repo := &sharedPoolUsageReviewRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	page, err := svc.AdminListSharedPoolUsageReviews(context.Background(), "", 0, 500)
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Equal(t, "pending", repo.listState)
	require.Equal(t, 100, repo.listLimit)
}

func TestAdminResolveSharedPoolUsageReviewRejectsUnsafeInputBeforeRepository(t *testing.T) {
	repo := &sharedPoolUsageReviewRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	_, err := svc.AdminResolveSharedPoolUsageReview(context.Background(), ResolveSharedPoolUsageReviewInput{
		ReservationID: 7, AdminUserID: 9, Action: SharedPoolUsageReviewActionSettleAmount,
		Note: "已核对上游账单", OperationID: "review-op-1",
	})
	require.Error(t, err)
	require.Nil(t, repo.prepareInput)

	amount := 1.0
	_, err = svc.AdminResolveSharedPoolUsageReview(context.Background(), ResolveSharedPoolUsageReviewInput{
		ReservationID: 7, AdminUserID: 9, Action: SharedPoolUsageReviewActionRelease,
		Amount: &amount, Note: "确认没有产生上游费用", OperationID: "review-op-2",
	})
	require.Error(t, err)
	require.Nil(t, repo.prepareInput)
}

func TestAdminResolveSharedPoolUsageReviewRejectsLegacyChargeActionsBeforeRepository(t *testing.T) {
	amount := 0.75
	for _, testCase := range []struct {
		name   string
		action string
		amount *float64
	}{
		{name: "capture hold", action: SharedPoolUsageReviewActionCaptureHold},
		{name: "manual settlement", action: SharedPoolUsageReviewActionSettleAmount, amount: &amount},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &sharedPoolUsageReviewRepoStub{}
			svc := NewBizDecipherService(repo, nil, nil)

			_, err := svc.AdminResolveSharedPoolUsageReview(context.Background(), ResolveSharedPoolUsageReviewInput{
				ReservationID: 7, AdminUserID: 9, Action: testCase.action, Amount: testCase.amount,
				Note: "旧冻结记录不能直接产生消费", OperationID: "review-op-charge-disabled",
			})

			require.ErrorContains(t, err, testCase.action)
			require.Nil(t, repo.prepareInput)
			require.Nil(t, repo.recorded)
		})
	}
}

func TestAdminResolveSharedPoolUsageReviewReleaseNeverCreatesUsageCharge(t *testing.T) {
	repo := &sharedPoolUsageReviewRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	result, err := svc.AdminResolveSharedPoolUsageReview(context.Background(), ResolveSharedPoolUsageReviewInput{
		ReservationID: 7, AdminUserID: 9, Action: SharedPoolUsageReviewActionRelease,
		Note: "确认上游没有接受请求，释放全部冻结", OperationID: "review-op-4",
	})
	require.NoError(t, err)
	require.Equal(t, "released", result.Review.ReservationStatus)
	require.Nil(t, repo.recorded)
}

func TestPreparedSharedPoolUsageReviewResolutionNeverSerializesInternalUsagePayload(t *testing.T) {
	encoded, err := json.Marshal(PreparedSharedPoolUsageReviewResolution{
		Review: &SharedPoolUsageReview{ReservationID: 7, AccessKeyID: 11, AccountID: 33},
		Usage: &SharedPoolUsageInput{
			AccessKeyID: 11, AccountID: 33, PriceSnapshot: json.RawMessage(`{"internal":true}`),
		},
		Replay: true,
	})
	require.NoError(t, err)
	text := string(encoded)
	require.Contains(t, text, `"review"`)
	require.Contains(t, text, `"replay":true`)
	require.NotContains(t, strings.ToLower(text), "usage")
	require.NotContains(t, text, "AccessKeyID")
	require.NotContains(t, text, "internal")
}

func TestAdminBatchResolveSharedPoolUsageReviewsDeduplicatesAndAuditsEachRelease(t *testing.T) {
	repo := &sharedPoolUsageReviewRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)

	result, err := svc.AdminBatchResolveSharedPoolUsageReviews(context.Background(), BatchResolveSharedPoolUsageReviewsInput{
		ReservationIDs: []int64{7, 8, 7},
		AdminUserID:    9,
		Action:         SharedPoolUsageReviewActionRelease,
		Note:           "已核对同批上游故障，均未返回成功响应",
		OperationID:    "review-batch-1",
	})
	require.NoError(t, err)
	require.Equal(t, 2, result.Succeeded)
	require.Zero(t, result.Failed)
	require.Len(t, repo.prepareInputs, 2)
	require.Equal(t, "review-batch-1:7", repo.prepareInputs[0].OperationID)
	require.Equal(t, "review-batch-1:8", repo.prepareInputs[1].OperationID)
	require.Nil(t, repo.recorded, "batch release must never create a usage charge")
}

func TestAdminBatchResolveSharedPoolUsageReviewsRejectsLegacyChargeActionsBeforeRepository(t *testing.T) {
	for _, action := range []string{SharedPoolUsageReviewActionCaptureHold, SharedPoolUsageReviewActionSettleAmount} {
		t.Run(action, func(t *testing.T) {
			repo := &sharedPoolUsageReviewRepoStub{}
			svc := NewBizDecipherService(repo, nil, nil)

			_, err := svc.AdminBatchResolveSharedPoolUsageReviews(context.Background(), BatchResolveSharedPoolUsageReviewsInput{
				ReservationIDs: []int64{7, 8}, AdminUserID: 9, Action: action,
				Note: "旧冻结记录不能批量产生消费", OperationID: "review-batch-charge-disabled",
			})

			require.ErrorContains(t, err, action)
			require.Empty(t, repo.prepareInputs)
			require.Nil(t, repo.recorded)
		})
	}
}
