//go:build unit

package service

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestResolveRechargeQuotaRebateRatePercent verifies that the actual
// withdrawable recharge rebate uses a per-inviter override when configured and
// otherwise preserves the existing 5% product default. It must not fall back to
// the separate 20% global/display setting.
func TestResolveRechargeQuotaRebateRatePercent(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}

	require.InDelta(t, 5.0,
		svc.resolveRechargeQuotaRebateRatePercent(&AffiliateSummary{}), 1e-9)

	rate := 10.0
	require.InDelta(t, 10.0,
		svc.resolveRechargeQuotaRebateRatePercent(&AffiliateSummary{AffRebateRatePercent: &rate}), 1e-9)

	zero := 0.0
	require.InDelta(t, 0.0,
		svc.resolveRechargeQuotaRebateRatePercent(&AffiliateSummary{AffRebateRatePercent: &zero}), 1e-9)

	tooHigh := 250.0
	require.InDelta(t, affiliateGrowthMaxDirectPercent,
		svc.resolveRechargeQuotaRebateRatePercent(&AffiliateSummary{AffRebateRatePercent: &tooHigh}), 1e-9)

	tooLow := -5.0
	require.InDelta(t, AffiliateRebateRateMin,
		svc.resolveRechargeQuotaRebateRatePercent(&AffiliateSummary{AffRebateRatePercent: &tooLow}), 1e-9)

	nan := math.NaN()
	require.InDelta(t, 5.0,
		svc.resolveRechargeQuotaRebateRatePercent(&AffiliateSummary{AffRebateRatePercent: &nan}), 1e-9)
}

// TestIsEnabled_NilSettingServiceReturnsDefault verifies that IsEnabled safely
// handles a nil settingService dependency by returning the configured default.
func TestIsEnabled_NilSettingServiceReturnsDefault(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	require.Equal(t, AffiliateEnabledDefault, svc.IsEnabled(context.Background()))
}

// TestValidateExclusiveRate_BoundaryAndInvalid covers the validator used by
// admin-facing rate setters: nil is always valid (clear), in-range values
// are accepted, NaN/Inf and out-of-range values produce a typed BadRequest.
func TestValidateExclusiveRate_BoundaryAndInvalid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateExclusiveRate(nil))

	for _, v := range []float64{0, 0.01, 5, 8, 12} {
		v := v
		require.NoError(t, validateExclusiveRate(&v), "value %v should be valid", v)
	}

	for _, v := range []float64{-0.01, 12.01, -100, 200} {
		v := v
		require.Error(t, validateExclusiveRate(&v), "value %v should be rejected", v)
	}

	nan := math.NaN()
	require.Error(t, validateExclusiveRate(&nan))
	posInf := math.Inf(1)
	require.Error(t, validateExclusiveRate(&posInf))
	negInf := math.Inf(-1)
	require.Error(t, validateExclusiveRate(&negInf))
}

func TestAccrueRechargeInviteRebateUsesDefaultTierRules(t *testing.T) {
	t.Parallel()
	tier2InviterID := int64(7)
	directInviterID := int64(10)
	inviteeID := int64(20)
	orderID := int64(300)
	repo := &redeemCodeAffiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{
			tier2InviterID:  {UserID: tier2InviterID, AffCode: "TIER2"},
			directInviterID: {UserID: directInviterID, AffCode: "DIRECT", InviterID: &tier2InviterID},
			inviteeID:       {UserID: inviteeID, AffCode: "INVITEE", InviterID: &directInviterID},
		},
		applied: true,
	}
	svc := &AffiliateService{repo: repo}

	result, err := svc.AccrueInviteRebateForOrder(context.Background(), inviteeID, 100, &orderID)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Applied)
	require.InDelta(t, 5.0, result.DirectQuotaRebate, 1e-9)
	require.Zero(t, result.DirectCreditReward)
	require.InDelta(t, 1.0, result.Tier2QuotaRebate, 1e-9)
	require.InDelta(t, 6.0, result.TotalQuotaRebate, 1e-9)
	require.Len(t, repo.rechargeCalls, 1)
	call := repo.rechargeCalls[0].input
	require.Equal(t, directInviterID, call.DirectInviterID)
	require.Equal(t, tier2InviterID, call.Tier2InviterID)
	require.Equal(t, inviteeID, call.InviteeUserID)
	require.Equal(t, orderID, call.SourceOrderID)
	require.InDelta(t, 5.0, call.DirectQuotaRebate, 1e-9)
	require.Zero(t, call.DirectCreditReward)
	require.InDelta(t, 1.0, call.Tier2QuotaRebate, 1e-9)
}

func TestAccrueRechargeInviteRebateUsesCustomDirectRateWithoutAmplifyingTier2(t *testing.T) {
	t.Parallel()
	tier2InviterID := int64(7)
	directInviterID := int64(10)
	inviteeID := int64(20)
	orderID := int64(301)
	customRate := 10.0
	repo := &redeemCodeAffiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{
			tier2InviterID:  {UserID: tier2InviterID, AffCode: "TIER2"},
			directInviterID: {UserID: directInviterID, AffCode: "DIRECT", InviterID: &tier2InviterID, AffRebateRatePercent: &customRate},
			inviteeID:       {UserID: inviteeID, AffCode: "INVITEE", InviterID: &directInviterID},
		},
		applied: true,
	}
	svc := &AffiliateService{repo: repo}

	result, err := svc.AccrueInviteRebateForOrder(context.Background(), inviteeID, 100, &orderID)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Applied)
	require.InDelta(t, 10.0, result.DirectQuotaRebate, 1e-9)
	require.Zero(t, result.DirectCreditReward)
	// Direct overrides do not amplify the ancestor's independently earned tier.
	require.InDelta(t, 1.0, result.Tier2QuotaRebate, 1e-9)
	require.InDelta(t, 11.0, result.TotalQuotaRebate, 1e-9)
	require.Len(t, repo.rechargeCalls, 1)
	call := repo.rechargeCalls[0].input
	require.InDelta(t, 10.0, call.DirectQuotaRebate, 1e-9)
	require.Zero(t, call.DirectCreditReward)
	require.InDelta(t, 1.0, call.Tier2QuotaRebate, 1e-9)
}

func TestAdminGetUserOverviewUsesCashSettlementDefault(t *testing.T) {
	t.Parallel()
	repo := &redeemCodeAffiliateRepoStub{
		overview: &AffiliateUserOverview{UserID: 10, RebateRatePercent: 20, RebateRateCustom: false},
	}
	svc := &AffiliateService{repo: repo}

	overview, err := svc.AdminGetUserOverview(context.Background(), 10)

	require.NoError(t, err)
	require.NotNil(t, overview)
	require.InDelta(t, 5.0, overview.RebateRatePercent, 1e-9)
}

func TestAccrueRechargeInviteRebateIdempotentRepositoryResult(t *testing.T) {
	t.Parallel()
	directInviterID := int64(10)
	inviteeID := int64(20)
	orderID := int64(302)
	customRate := 10.0
	repo := &redeemCodeAffiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{
			directInviterID: {UserID: directInviterID, AffCode: "DIRECT", AffRebateRatePercent: &customRate},
			inviteeID:       {UserID: inviteeID, AffCode: "INVITEE", InviterID: &directInviterID},
		},
		applied: false,
	}
	svc := &AffiliateService{repo: repo}

	result, err := svc.AccrueInviteRebateForOrder(context.Background(), inviteeID, 100, &orderID)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Applied)
	require.Zero(t, result.DirectQuotaRebate)
	require.Zero(t, result.DirectCreditReward)
	require.Zero(t, result.TotalQuotaRebate)
	require.Len(t, repo.rechargeCalls, 1)
	require.InDelta(t, 10.0, repo.rechargeCalls[0].input.DirectQuotaRebate, 1e-9)
}

func TestAccrueRechargeRejectsLegacySelfInvite(t *testing.T) {
	userID := int64(20)
	orderID := int64(303)
	repo := &redeemCodeAffiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{
			userID: {UserID: userID, InviterID: &userID},
		},
		applied: true,
	}
	result, err := (&AffiliateService{repo: repo}).AccrueInviteRebateForOrder(context.Background(), userID, 100, &orderID)
	require.NoError(t, err)
	require.False(t, result.HasReward())
	require.Empty(t, repo.rechargeCalls)
}
func TestAccrueRedeemCodeRebateUsesFixedQuotaAndCreditRates(t *testing.T) {
	t.Parallel()
	inviterID := int64(10)
	inviteeID := int64(20)
	redeemCodeID := int64(300)
	repo := &redeemCodeAffiliateRepoStub{
		summaries: map[int64]*AffiliateSummary{
			inviterID: {UserID: inviterID, AffCode: "INVITER"},
			inviteeID: {UserID: inviteeID, AffCode: "INVITEE", InviterID: &inviterID},
		},
		applied: true,
	}
	svc := &AffiliateService{repo: repo}

	quotaRebate, creditReward, err := svc.AccrueRedeemCodeRebate(context.Background(), inviteeID, redeemCodeID, 100)

	require.NoError(t, err)
	require.InDelta(t, 5.0, quotaRebate, 1e-9)
	require.InDelta(t, 20.0, creditReward, 1e-9)
	require.Len(t, repo.redeemCalls, 1)
	require.Equal(t, redeemCodeAffiliateCall{
		inviterID:     inviterID,
		inviteeUserID: inviteeID,
		redeemCodeID:  redeemCodeID,
		quotaAmount:   5.0,
		creditAmount:  20.0,
	}, repo.redeemCalls[0])
}

type redeemCodeAffiliateCall struct {
	inviterID     int64
	inviteeUserID int64
	redeemCodeID  int64
	quotaAmount   float64
	creditAmount  float64
}

type rechargeInviteRewardsCall struct {
	input RechargeInviteRewardsInput
}

type redeemCodeAffiliateRepoStub struct {
	summaries     map[int64]*AffiliateSummary
	overview      *AffiliateUserOverview
	applied       bool
	redeemCalls   []redeemCodeAffiliateCall
	rechargeCalls []rechargeInviteRewardsCall
}

func (r *redeemCodeAffiliateRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	if summary, ok := r.summaries[userID]; ok {
		return summary, nil
	}
	return nil, ErrAffiliateProfileNotFound
}

func (r *redeemCodeAffiliateRepoStub) GetAffiliateByCode(context.Context, string) (*AffiliateSummary, error) {
	panic("unexpected GetAffiliateByCode call")
}

func (r *redeemCodeAffiliateRepoStub) BindInviter(context.Context, int64, int64) (bool, error) {
	panic("unexpected BindInviter call")
}

func (r *redeemCodeAffiliateRepoStub) AccrueQuota(context.Context, int64, int64, float64, int, *int64) (bool, error) {
	panic("unexpected AccrueQuota call")
}

func (r *redeemCodeAffiliateRepoStub) AccrueRechargeInviteRewards(_ context.Context, input RechargeInviteRewardsInput) (*RechargeInviteRewardsResult, error) {
	r.rechargeCalls = append(r.rechargeCalls, rechargeInviteRewardsCall{input: input})
	if !r.applied {
		return &RechargeInviteRewardsResult{BaseRechargeAmount: input.BaseRechargeAmount}, nil
	}
	return &RechargeInviteRewardsResult{
		Applied:            true,
		DirectInviterID:    input.DirectInviterID,
		Tier2InviterID:     input.Tier2InviterID,
		DirectQuotaRebate:  input.DirectQuotaRebate,
		DirectCreditReward: input.DirectCreditReward,
		Tier2QuotaRebate:   input.Tier2QuotaRebate,
		TotalQuotaRebate:   input.DirectQuotaRebate + input.Tier2QuotaRebate,
		BaseRechargeAmount: input.BaseRechargeAmount,
	}, nil
}

func (r *redeemCodeAffiliateRepoStub) AccrueRedeemCodeRebate(_ context.Context, inviterID, inviteeUserID, redeemCodeID int64, quotaAmount, creditAmount float64) (bool, error) {
	r.redeemCalls = append(r.redeemCalls, redeemCodeAffiliateCall{
		inviterID:     inviterID,
		inviteeUserID: inviteeUserID,
		redeemCodeID:  redeemCodeID,
		quotaAmount:   quotaAmount,
		creditAmount:  creditAmount,
	})
	return r.applied, nil
}

func (r *redeemCodeAffiliateRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	panic("unexpected GetAccruedRebateFromInvitee call")
}

func (r *redeemCodeAffiliateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	panic("unexpected ThawFrozenQuota call")
}

func (r *redeemCodeAffiliateRepoStub) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	panic("unexpected TransferQuotaToBalance call")
}

func (r *redeemCodeAffiliateRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	panic("unexpected ListInvitees call")
}

func (r *redeemCodeAffiliateRepoStub) UpdateUserAffCode(context.Context, int64, string) error {
	panic("unexpected UpdateUserAffCode call")
}

func (r *redeemCodeAffiliateRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	panic("unexpected ResetUserAffCode call")
}

func (r *redeemCodeAffiliateRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	panic("unexpected SetUserRebateRate call")
}

func (r *redeemCodeAffiliateRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	panic("unexpected BatchSetUserRebateRate call")
}

func (r *redeemCodeAffiliateRepoStub) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	panic("unexpected ListUsersWithCustomSettings call")
}

func (r *redeemCodeAffiliateRepoStub) ListAffiliateInviteRecords(context.Context, AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	panic("unexpected ListAffiliateInviteRecords call")
}

func (r *redeemCodeAffiliateRepoStub) ListAffiliateRebateRecords(context.Context, AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	panic("unexpected ListAffiliateRebateRecords call")
}

func (r *redeemCodeAffiliateRepoStub) ListAffiliateTransferRecords(context.Context, AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	panic("unexpected ListAffiliateTransferRecords call")
}

func (r *redeemCodeAffiliateRepoStub) GetAffiliateUserOverview(context.Context, int64) (*AffiliateUserOverview, error) {
	if r.overview != nil {
		copy := *r.overview
		return &copy, nil
	}
	return &AffiliateUserOverview{}, nil
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	// 邀请码格式校验同时服务于：
	// 1) 系统自动生成的 12 位随机码（A-Z 去 I/O，2-9 去 0/1）
	// 2) 管理员设置的自定义专属码（如 "VIP2026"、"NEW_USER-1"）
	// 因此校验放宽到 [A-Z0-9_-]{4,32}（要求调用方先 ToUpper）。
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical 12-char", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"valid admin custom short", "VIP1", true},
		{"valid admin custom with hyphen", "NEW-USER", true},
		{"valid admin custom with underscore", "VIP_2026", true},
		{"valid 32-char max", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", true},
		// Previously-excluded chars (I/O/0/1) are now allowed since admins may use them.
		{"letter I now allowed", "IBCDEFGHJKLM", true},
		{"letter O now allowed", "OBCDEFGHJKLM", true},
		{"digit 0 now allowed", "0BCDEFGHJKLM", true},
		{"digit 1 now allowed", "1BCDEFGHJKLM", true},
		{"too short (3 chars)", "ABC", false},
		{"too long (33 chars)", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"utf8 non-ascii", "ÄÄÄÄÄÄ", false}, // bytes out of charset
		{"ascii punctuation .", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}
