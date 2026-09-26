package service

import (
	"context"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type CreditSystemRepository interface {
	GetCreditSystemOverview(ctx context.Context) (*CreditSystemOverview, error)
	ListCreditLedger(ctx context.Context, filter CreditLedgerFilter) ([]BizCreditLedgerEntry, int64, error)
	GetCreditUserSummary(ctx context.Context, userID int64) (*CreditUserSummary, error)
	GrantCredit(ctx context.Context, input BizCreditGrantInput) (*BizCreditLedgerEntry, error)
}

type CreditLedgerFilter struct {
	UserID     int64
	Search     string
	SourceType string
	Status     string
	StartAt    *time.Time
	EndAt      *time.Time
	Page       int
	PageSize   int
}

type CreditSystemOverview struct {
	TotalCreditBalance     float64                    `json:"total_credit_balance"`
	PositiveCreditUsers    int64                      `json:"positive_credit_users"`
	LedgerEntryCount       int64                      `json:"ledger_entry_count"`
	TodayGrantedCredits    float64                    `json:"today_granted_credits"`
	TodayConsumedCredits   float64                    `json:"today_consumed_credits"`
	SourceTypeDistribution []CreditSourceDistribution `json:"source_type_distribution"`
	Rules                  []CreditSystemRule         `json:"rules"`
}

type CreditSourceDistribution struct {
	SourceType string  `json:"source_type"`
	Count      int64   `json:"count"`
	Amount     float64 `json:"amount"`
}

type CreditSystemRule struct {
	Key         string  `json:"key"`
	Title       string  `json:"title"`
	AssetType   string  `json:"asset_type"`
	Amount      float64 `json:"amount,omitempty"`
	RatePercent float64 `json:"rate_percent,omitempty"`
	Description string  `json:"description"`
}

type CreditUserSummary struct {
	UserID        int64                  `json:"user_id"`
	Email         string                 `json:"email"`
	Username      string                 `json:"username"`
	CreditBalance float64                `json:"credit_balance"`
	Ledger        []BizCreditLedgerEntry `json:"ledger"`
}

type CreditSystemService struct {
	repo CreditSystemRepository
}

func NewCreditSystemService(repo CreditSystemRepository) *CreditSystemService {
	return &CreditSystemService{repo: repo}
}

func (s *CreditSystemService) GetOverview(ctx context.Context) (*CreditSystemOverview, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "credit system service unavailable")
	}
	overview, err := s.repo.GetCreditSystemOverview(ctx)
	if err != nil {
		return nil, err
	}
	overview.Rules = defaultCreditSystemRules()
	return overview, nil
}

func (s *CreditSystemService) ListLedger(ctx context.Context, filter CreditLedgerFilter) ([]BizCreditLedgerEntry, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "credit system service unavailable")
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.SourceType = strings.TrimSpace(filter.SourceType)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Page, filter.PageSize = normalizeCreditPagination(filter.Page, filter.PageSize)
	return s.repo.ListCreditLedger(ctx, filter)
}

func (s *CreditSystemService) GetUserSummary(ctx context.Context, userID int64) (*CreditUserSummary, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "credit system service unavailable")
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	return s.repo.GetCreditUserSummary(ctx, userID)
}

func (s *CreditSystemService) AdminGrantCredit(ctx context.Context, input BizCreditGrantInput) (*BizCreditLedgerEntry, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "credit system service unavailable")
	}
	if input.UserID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if input.Amount == 0 || math.IsNaN(input.Amount) || math.IsInf(input.Amount, 0) {
		return nil, infraerrors.BadRequest("INVALID_AMOUNT", "amount must not be zero")
	}
	input.SourceType = strings.TrimSpace(input.SourceType)
	if input.SourceType == "" {
		input.SourceType = "admin_adjustment"
	}
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.Note = strings.TrimSpace(input.Note)
	return s.repo.GrantCredit(ctx, input)
}

func normalizeCreditPagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func defaultCreditSystemRules() []CreditSystemRule {
	return []CreditSystemRule{
		{Key: "invite_key_reward", Title: "拉人用 Key 奖励", AssetType: "credit", Amount: 30, Description: "被邀请人完成 Key 使用后，直接邀请人获得 30 不可提现积分。"},
		{Key: "recharge_direct_rebate", Title: "充值一级返利", AssetType: "withdrawable_balance_quota", RatePercent: 5, Description: "被邀请人充值成功后，直接邀请人获得充值面值 5% 可提现返利额度，后续可转入 users.balance。"},
		{Key: "recharge_credit_reward", Title: "充值积分奖励", AssetType: "credit", RatePercent: 20, Description: "被邀请人充值成功后，直接邀请人获得充值面值 20% 不可提现积分，写入 users.credit_balance。"},
		{Key: "recharge_tier2_rebate", Title: "二级返利", AssetType: "withdrawable_balance_quota", RatePercent: 0.25, Description: "直接邀请人的上级获得充值面值 0.25% 可提现返利额度，即一级 5% 返利中的 5%。"},
		{Key: "shared_pool_stability_reward", Title: "共享池稳定奖励", AssetType: "credit", Amount: 120, Description: "池主稳定供给按完整 UTC 日结算不可提现积分：基础第1池120/第2池72/第3池36/第4池起0；质量加成可用率≥95%+30、低探针失败+20、稳定延迟+20；真实使用加成真实调用+30、付费/席位用户+30；优秀池主日结 quality 80 / certified 150。池子收益仍进入可提现余额。"},
		{Key: "daily_checkin_milestone", Title: "签到七天积分", AssetType: "credit", Amount: 100, Description: "每轮累计签到 3/5/7 天分别领取 30/50/100 不可提现积分，不再赠送余额。历史已发余额与账单保留；每日签到、收藏卡和大奖池规则不变。"},
	}
}
