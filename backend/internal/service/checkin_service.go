package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	CheckinTypeFree    = "free"
	CheckinTypePaid    = "paid"
	CheckinTypeCredit  = "credit"
	CheckinTypeBalance = "balance"

	checkinCreditCost  = 20.0
	checkinBalanceCost = 5.0

	collectibleShopSourceType  = "card_shop"
	collectibleShopSourceLabel = "credit_shop"

	jackpotProbabilityScale = int64(1000000)
	freeJackpotThreshold    = int64(20)  // 0.002%
	creditJackpotThreshold  = int64(100) // 0.01%
	balanceJackpotThreshold = int64(300) // 0.03%

	collectibleDropScale        = int64(1000000)
	freeCollectibleThreshold    = int64(1800)
	creditCollectibleThreshold  = int64(4500)
	balanceCollectibleThreshold = int64(9000)
)

type weightedCheckinReward struct {
	Value  float64
	Weight int64
}

type collectibleCardTemplate struct {
	Key    string
	Rarity string
	Weight int64
}

type collectibleShopTier struct {
	Rarity string
	Price  float64
}

var (
	freeCheckinRewards = []weightedCheckinReward{
		{Value: 5, Weight: 40},
		{Value: 7, Weight: 30},
		{Value: 10, Weight: 15},
		{Value: 15, Weight: 15},
	}
	creditCheckinRewards = []weightedCheckinReward{
		{Value: 5, Weight: 44},
		{Value: 10, Weight: 28},
		{Value: 20, Weight: 20},
		{Value: 50, Weight: 6},
		{Value: 100, Weight: 2},
	}
	balanceCheckinRewards = []weightedCheckinReward{
		{Value: 1, Weight: 36},
		{Value: 2, Weight: 32},
		{Value: 5, Weight: 20},
		{Value: 10, Weight: 9},
		{Value: 20, Weight: 3},
	}
	collectibleShopTiers = map[string]collectibleShopTier{
		"common": {Rarity: "common", Price: 260},
		"good":   {Rarity: "good", Price: 780},
		"rare":   {Rarity: "rare", Price: 2400},
	}
	collectibleEditionSupplies = map[string]int64{
		"common":    9999,
		"good":      3333,
		"rare":      999,
		"epic":      300,
		"legendary": 100,
		"mythic":    30,
	}
	collectibleCardTemplates = []collectibleCardTemplate{
		{Key: "low_battery_sprite", Rarity: "common", Weight: 45},
		{Key: "early_failer", Rarity: "common", Weight: 45},
		{Key: "sofa_observer", Rarity: "common", Weight: 45},
		{Key: "micro_disconnect_worker", Rarity: "common", Weight: 45},
		{Key: "monday_escapee", Rarity: "common", Weight: 45},
		{Key: "warm_water_guard", Rarity: "common", Weight: 45},
		{Key: "slow_boot_machine", Rarity: "common", Weight: 45},
		{Key: "tiny_step_champion", Rarity: "common", Weight: 45},
		{Key: "self_rescue_store", Rarity: "common", Weight: 45},
		{Key: "retry_button_keeper", Rarity: "common", Weight: 45},
		{Key: "calendar_slacker", Rarity: "common", Weight: 45},
		{Key: "unread_message_monk", Rarity: "common", Weight: 45},
		{Key: "blanket_fort_guardian", Rarity: "common", Weight: 45},
		{Key: "snack_budget_scholar", Rarity: "common", Weight: 45},
		{Key: "fog_window_wiper", Rarity: "common", Weight: 45},
		{Key: "almost_ok_actor", Rarity: "common", Weight: 45},
		{Key: "battery_anxiety_meter", Rarity: "common", Weight: 45},
		{Key: "side_quest_picker", Rarity: "common", Weight: 45},
		{Key: "tiny_rain_shelter", Rarity: "common", Weight: 45},
		{Key: "lost_focus_fisher", Rarity: "common", Weight: 45},
		{Key: "browser_tab_shepherd", Rarity: "common", Weight: 45},
		{Key: "midnight_cookie_auditor", Rarity: "common", Weight: 45},
		{Key: "instant_noodle_prophet", Rarity: "common", Weight: 45},
		{Key: "pocket_sun_carrier", Rarity: "common", Weight: 45},
		{Key: "humble_loading_bar", Rarity: "common", Weight: 45},
		{Key: "small_cloud_tenant", Rarity: "common", Weight: 45},
		{Key: "chair_rooted_guard", Rarity: "common", Weight: 45},
		{Key: "pending_reply_sprite", Rarity: "common", Weight: 45},
		{Key: "tiny_courage_bean", Rarity: "common", Weight: 45},
		{Key: "desktop_dust_archivist", Rarity: "common", Weight: 45},
		{Key: "cloud_patch_apprentice", Rarity: "good", Weight: 42},
		{Key: "mood_buffer_agent", Rarity: "good", Weight: 42},
		{Key: "hope_coupon_keeper", Rarity: "good", Weight: 42},
		{Key: "rationality_dog_walker", Rarity: "good", Weight: 42},
		{Key: "tiny_luck_runner", Rarity: "good", Weight: 42},
		{Key: "anxiety_gardener", Rarity: "good", Weight: 42},
		{Key: "failure_recycler", Rarity: "good", Weight: 42},
		{Key: "luck_taster", Rarity: "good", Weight: 42},
		{Key: "reverse_koi_assistant", Rarity: "good", Weight: 42},
		{Key: "expectation_admin", Rarity: "good", Weight: 42},
		{Key: "metaphysics_compliance", Rarity: "good", Weight: 42},
		{Key: "digital_scavenger", Rarity: "good", Weight: 42},
		{Key: "little_profit_philosopher", Rarity: "good", Weight: 42},
		{Key: "wallet_listener", Rarity: "good", Weight: 42},
		{Key: "surprise_ticket_checker", Rarity: "good", Weight: 42},
		{Key: "risk_kitten_keeper", Rarity: "good", Weight: 42},
		{Key: "noise_reducer", Rarity: "good", Weight: 42},
		{Key: "hope_night_shift", Rarity: "good", Weight: 42},
		{Key: "almost_miracle_clerk", Rarity: "good", Weight: 42},
		{Key: "soft_deadline_negotiator", Rarity: "good", Weight: 42},
		{Key: "low_battery_saint", Rarity: "rare", Weight: 40},
		{Key: "cloud_patch_worker", Rarity: "rare", Weight: 40},
		{Key: "late_but_arrived", Rarity: "rare", Weight: 40},
		{Key: "tiny_luck_clerk", Rarity: "rare", Weight: 40},
		{Key: "mood_janitor", Rarity: "rare", Weight: 40},
		{Key: "hope_inventory_keeper", Rarity: "rare", Weight: 40},
		{Key: "doomed_plan_rescuer", Rarity: "rare", Weight: 40},
		{Key: "tiny_storm_captain", Rarity: "rare", Weight: 40},
		{Key: "lucky_bug_keeper", Rarity: "rare", Weight: 40},
		{Key: "deadline_exorcist", Rarity: "rare", Weight: 40},
		{Key: "half_awake_oracle", Rarity: "rare", Weight: 40},
		{Key: "almost_win_archivist", Rarity: "rare", Weight: 40},
		{Key: "probability_rebel", Rarity: "epic", Weight: 26},
		{Key: "mirror_lake_admin", Rarity: "epic", Weight: 26},
		{Key: "blue_hour_operator", Rarity: "epic", Weight: 26},
		{Key: "black_sun_intern", Rarity: "epic", Weight: 26},
		{Key: "fate_customer_service", Rarity: "epic", Weight: 26},
		{Key: "hit_rate_tamer", Rarity: "epic", Weight: 26},
		{Key: "miracle_tester", Rarity: "epic", Weight: 26},
		{Key: "prize_pool_diver", Rarity: "epic", Weight: 26},
		{Key: "numbered_crown_holder", Rarity: "legendary", Weight: 23},
		{Key: "afterglow_cartographer", Rarity: "legendary", Weight: 23},
		{Key: "zero_hour_lighthouse_keeper", Rarity: "legendary", Weight: 23},
		{Key: "hidden_plot_curator", Rarity: "legendary", Weight: 23},
		{Key: "zero_point_pilot", Rarity: "mythic", Weight: 15},
		{Key: "terminal_hope_backup", Rarity: "mythic", Weight: 15},
	}
)

var (
	ErrCheckinInvalidType                 = infraerrors.BadRequest("CHECKIN_INVALID_TYPE", "invalid checkin type")
	ErrCheckinAlreadyClaimed              = infraerrors.Conflict("CHECKIN_ALREADY_CLAIMED", "today's checkin has already been claimed")
	ErrCheckinInsufficientPoint           = infraerrors.BadRequest("CHECKIN_INSUFFICIENT_POINTS", "insufficient points for checkin")
	ErrCheckinInsufficientBalance         = infraerrors.BadRequest("CHECKIN_INSUFFICIENT_BALANCE", "insufficient balance for checkin")
	ErrCheckinMilestoneInvalid            = infraerrors.BadRequest("CHECKIN_MILESTONE_INVALID", "invalid checkin milestone")
	ErrCheckinMilestoneNotEligible        = infraerrors.BadRequest("CHECKIN_MILESTONE_NOT_ELIGIBLE", "checkin milestone is not eligible yet")
	ErrCheckinMilestoneAlreadyClaimed     = infraerrors.Conflict("CHECKIN_MILESTONE_ALREADY_CLAIMED", "checkin milestone has already been claimed")
	ErrCheckinMilestoneActivationRequired = infraerrors.BadRequest("CHECKIN_MILESTONE_ACTIVATION_REQUIRED", "email verification and real API usage are required for this milestone")
	ErrCheckinCardShopInvalidTier         = infraerrors.BadRequest("CHECKIN_CARD_SHOP_INVALID_TIER", "this card tier is not available in the point shop")
	ErrCheckinCardSoldOut                 = infraerrors.Conflict("CHECKIN_CARD_SOLD_OUT", "this collectible card edition is sold out")
	ErrCheckinCreditLotteryMoved          = infraerrors.BadRequest("CHECKIN_CREDIT_LOTTERY_MOVED", "credit lottery must use the credit lottery endpoint")
	ErrCheckinOperationIDInvalid          = infraerrors.BadRequest("CHECKIN_OPERATION_ID_INVALID", "invalid checkin operation id")
	ErrCheckinOperationConflict           = infraerrors.Conflict("CHECKIN_OPERATION_CONFLICT", "checkin operation id was already used for another draw")
	ErrCheckinOperationNotFound           = infraerrors.NotFound("CHECKIN_OPERATION_NOT_FOUND", "checkin operation was not found")
)

type CheckinRepository interface {
	GetDailyStatus(ctx context.Context, userID int64, day time.Time) (*CheckinStatus, error)
	ClaimDaily(ctx context.Context, input CheckinClaimInput) (*CheckinResult, error)
	GetOperationResult(ctx context.Context, userID int64, operationID string) (*CheckinResult, error)
	ClaimMilestone(ctx context.Context, userID int64, milestoneDays int) (*CheckinMilestoneClaimResult, error)
	BuyCollectibleCard(ctx context.Context, input CheckinCardPurchaseInput) (*CheckinCardPurchaseResult, error)
	ListDailyRecords(ctx context.Context, userID int64, limit int) ([]CheckinRecord, error)
	ListCollectibleCards(ctx context.Context, userID int64, limit int) ([]CheckinCollectibleCard, error)
}

// CheckinCollectibleCardPageRepository is optional so older repository stubs
// remain compatible while the production repository exposes the complete album.
type CheckinCollectibleCardPageRepository interface {
	ListCollectibleCardsPage(ctx context.Context, userID int64, page, pageSize int) (*CheckinCollectibleCardsPage, error)
}

type CheckinService struct {
	repo CheckinRepository
	now  func() time.Time
}

type CheckinStatus struct {
	Date             string                   `json:"date"`
	FreeClaimed      bool                     `json:"free_claimed"`
	CreditCount      int                      `json:"credit_count"`
	BalanceCount     int                      `json:"balance_count"`
	CreditLimit      int                      `json:"credit_limit"`
	BalanceLimit     int                      `json:"balance_limit"`
	PaidClaimed      bool                     `json:"paid_claimed"`
	PaidCost         float64                  `json:"paid_cost"`
	CreditCost       float64                  `json:"credit_cost"`
	BalanceCost      float64                  `json:"balance_cost"`
	FreeRewardMin    float64                  `json:"free_reward_min"`
	FreeRewardMax    float64                  `json:"free_reward_max"`
	PaidRewardMin    float64                  `json:"paid_reward_min"`
	PaidRewardMax    float64                  `json:"paid_reward_max"`
	CreditRewardMin  float64                  `json:"credit_reward_min"`
	CreditRewardMax  float64                  `json:"credit_reward_max"`
	BalanceRewardMin float64                  `json:"balance_reward_min"`
	BalanceRewardMax float64                  `json:"balance_reward_max"`
	Balance          float64                  `json:"balance"`
	CreditBalance    float64                  `json:"credit_balance"`
	StreakDays       int                      `json:"streak_days"`
	CycleNo          int                      `json:"cycle_no"`
	CycleDay         int                      `json:"cycle_day"`
	Milestones       []CheckinMilestoneStatus `json:"milestones"`
	CreditJackpot    float64                  `json:"credit_jackpot"`
	BalanceJackpot   float64                  `json:"balance_jackpot"`
}

type CheckinMilestoneStatus struct {
	Days               int     `json:"days"`
	CreditReward       float64 `json:"credit_reward"`
	BalanceReward      float64 `json:"balance_reward"`
	Eligible           bool    `json:"eligible"`
	Claimed            bool    `json:"claimed"`
	Claimable          bool    `json:"claimable"`
	RequiresVerified   bool    `json:"requires_verified"`
	RequiresAPIUsage   bool    `json:"requires_api_usage"`
	ActivationRequired bool    `json:"activation_required"`
}

type CheckinClaimInput struct {
	UserID      int64
	OperationID string
	Type        string
	Date        time.Time
	Cost        float64
	Reward      float64
	CostAsset   string
	RewardAsset string
	DailyLimit  int
	Jackpot     JackpotPlan
	Collectible *CheckinCollectibleCard
}

type JackpotPlan struct {
	Hit              bool
	WinnerRatio      float64
	CelebrationRatio float64
}

type JackpotPayout struct {
	PoolType                string  `json:"pool_type"`
	RewardAsset             string  `json:"reward_asset"`
	WinnerAmount            float64 `json:"winner_amount"`
	CelebrationAmount       float64 `json:"celebration_amount"`
	CelebrationActualAmount float64 `json:"celebration_actual_amount"`
	CelebrationUserCount    int     `json:"celebration_user_count"`
}

type CheckinMilestoneClaimResult struct {
	MilestoneDays      int     `json:"milestone_days"`
	CreditReward       float64 `json:"credit_reward"`
	BalanceReward      float64 `json:"balance_reward"`
	BalanceAfter       float64 `json:"balance_after"`
	CreditBalanceAfter float64 `json:"credit_balance_after"`
}

type CheckinCardPurchaseInput struct {
	UserID        int64
	Rarity        string
	Price         float64
	CardKey       string
	SourceType    string
	SourceLabel   string
	EditionSupply int64
}

type CheckinCardPurchaseResult struct {
	Rarity             string                 `json:"rarity"`
	Price              float64                `json:"price"`
	BalanceAfter       float64                `json:"balance_after"`
	CreditBalanceAfter float64                `json:"credit_balance_after"`
	CollectibleCard    CheckinCollectibleCard `json:"collectible_card"`
}

type CheckinCollectibleCard struct {
	ID            int64     `json:"id"`
	CheckinID     int64     `json:"checkin_id,omitempty"`
	CardKey       string    `json:"card_key"`
	Rarity        string    `json:"rarity"`
	SourceType    string    `json:"source_type"`
	SourceLabel   string    `json:"source_label"`
	SerialNo      int64     `json:"serial_no"`
	EditionNo     int64     `json:"edition_no,omitempty"`
	EditionSupply int64     `json:"edition_supply,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type CheckinCollectibleCardsPage struct {
	Items    []CheckinCollectibleCard `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	HasMore  bool                     `json:"has_more"`
}

type CheckinRecord struct {
	ID                  int64                   `json:"id"`
	Date                string                  `json:"date"`
	Type                string                  `json:"type"`
	Cost                float64                 `json:"cost"`
	Reward              float64                 `json:"reward"`
	RewardAsset         string                  `json:"reward_asset"`
	JackpotHit          bool                    `json:"jackpot_hit"`
	JackpotPool         string                  `json:"jackpot_pool,omitempty"`
	JackpotWinnerAmount float64                 `json:"jackpot_winner_amount,omitempty"`
	JackpotPayouts      []JackpotPayout         `json:"jackpot_payouts,omitempty"`
	CreatedAt           time.Time               `json:"created_at"`
	CollectibleCard     *CheckinCollectibleCard `json:"collectible_card,omitempty"`
}

type CheckinResult struct {
	OperationID                 string                  `json:"operation_id,omitempty"`
	Type                        string                  `json:"type"`
	Date                        string                  `json:"date"`
	Cost                        float64                 `json:"cost"`
	Reward                      float64                 `json:"reward"`
	RewardAsset                 string                  `json:"reward_asset"`
	BalanceAfter                float64                 `json:"balance_after"`
	CreditBalanceAfter          float64                 `json:"credit_balance_after"`
	AlreadyClaimed              bool                    `json:"already_claimed"`
	JackpotHit                  bool                    `json:"jackpot_hit"`
	JackpotPool                 string                  `json:"jackpot_pool,omitempty"`
	JackpotAmount               float64                 `json:"jackpot_amount,omitempty"`
	JackpotWinnerAmount         float64                 `json:"jackpot_winner_amount,omitempty"`
	JackpotCelebrationAmount    float64                 `json:"jackpot_celebration_amount,omitempty"`
	JackpotCelebrationUserCount int                     `json:"jackpot_celebration_user_count,omitempty"`
	JackpotPayouts              []JackpotPayout         `json:"jackpot_payouts,omitempty"`
	CollectibleCard             *CheckinCollectibleCard `json:"collectible_card,omitempty"`
}

func NewCheckinService(repo CheckinRepository) *CheckinService {
	return &CheckinService{repo: repo, now: time.Now}
}

func (s *CheckinService) GetStatus(ctx context.Context, userID int64) (*CheckinStatus, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("CHECKIN_UNAVAILABLE", "checkin service is not configured")
	}
	return s.repo.GetDailyStatus(ctx, userID, s.today())
}

func (s *CheckinService) ListRecords(ctx context.Context, userID int64, limit int) ([]CheckinRecord, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("CHECKIN_UNAVAILABLE", "checkin service is not configured")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.ListDailyRecords(ctx, userID, limit)
}

func (s *CheckinService) ListCollectibleCards(ctx context.Context, userID int64, limit int) ([]CheckinCollectibleCard, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("CHECKIN_UNAVAILABLE", "checkin service is not configured")
	}
	if limit <= 0 || limit > 100 {
		limit = 60
	}
	return s.repo.ListCollectibleCards(ctx, userID, limit)
}

func (s *CheckinService) ListCollectibleCardsPage(ctx context.Context, userID int64, page, pageSize int) (*CheckinCollectibleCardsPage, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("CHECKIN_UNAVAILABLE", "checkin service is not configured")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 100
	}
	pagedRepo, ok := s.repo.(CheckinCollectibleCardPageRepository)
	if !ok {
		if page > 1 {
			return &CheckinCollectibleCardsPage{
				Items:    []CheckinCollectibleCard{},
				Page:     page,
				PageSize: pageSize,
			}, nil
		}
		items, err := s.repo.ListCollectibleCards(ctx, userID, pageSize)
		if err != nil {
			return nil, err
		}
		return &CheckinCollectibleCardsPage{
			Items:    items,
			Total:    int64(len(items)),
			Page:     page,
			PageSize: pageSize,
			HasMore:  len(items) == pageSize,
		}, nil
	}
	return pagedRepo.ListCollectibleCardsPage(ctx, userID, page, pageSize)
}

func (s *CheckinService) ClaimMilestone(ctx context.Context, userID int64, milestoneDays int) (*CheckinMilestoneClaimResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("CHECKIN_UNAVAILABLE", "checkin service is not configured")
	}
	if !isValidCheckinMilestone(milestoneDays) {
		return nil, ErrCheckinMilestoneInvalid
	}
	return s.repo.ClaimMilestone(ctx, userID, milestoneDays)
}

func (s *CheckinService) BuyCollectibleCard(ctx context.Context, userID int64, rarity string) (*CheckinCardPurchaseResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("CHECKIN_UNAVAILABLE", "checkin service is not configured")
	}
	tier, ok := collectibleShopTierByRarity(rarity)
	if !ok {
		return nil, ErrCheckinCardShopInvalidTier
	}
	triedKeys := map[string]struct{}{}
	for attempts := 0; attempts < len(collectibleCardTemplates); attempts++ {
		template, err := randomCollectibleTemplateByRarityExcluding(tier.Rarity, triedKeys)
		if err != nil {
			if errors.Is(err, ErrCheckinInvalidType) && len(triedKeys) > 0 {
				return nil, ErrCheckinCardSoldOut
			}
			return nil, err
		}
		triedKeys[template.Key] = struct{}{}
		result, err := s.repo.BuyCollectibleCard(ctx, CheckinCardPurchaseInput{
			UserID:        userID,
			Rarity:        tier.Rarity,
			Price:         tier.Price,
			CardKey:       template.Key,
			SourceType:    collectibleShopSourceType,
			SourceLabel:   collectibleShopSourceLabel,
			EditionSupply: collectibleEditionSupply(template.Rarity),
		})
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, ErrCheckinCardSoldOut) {
			return nil, err
		}
	}
	return nil, ErrCheckinCardSoldOut
}

func (s *CheckinService) Claim(ctx context.Context, userID int64, checkinType string) (*CheckinResult, error) {
	return s.ClaimWithOperation(ctx, userID, checkinType, "")
}

func (s *CheckinService) ClaimWithOperation(ctx context.Context, userID int64, checkinType, operationID string) (*CheckinResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("CHECKIN_UNAVAILABLE", "checkin service is not configured")
	}
	operationID, err := normalizeCheckinOperationID(operationID)
	if err != nil {
		return nil, err
	}
	rule, err := checkinRule(checkinType)
	if err != nil {
		return nil, err
	}
	reward, err := randomCheckinReward(rule.rewards)
	if err != nil {
		return nil, err
	}
	jackpot, err := randomJackpotPlan(rule.checkinType)
	if err != nil {
		return nil, err
	}
	collectible, err := randomCollectibleCard(rule.checkinType)
	if err != nil {
		return nil, err
	}
	return s.repo.ClaimDaily(ctx, CheckinClaimInput{
		UserID:      userID,
		OperationID: operationID,
		Type:        rule.checkinType,
		Date:        s.today(),
		Cost:        rule.cost,
		Reward:      reward,
		CostAsset:   rule.costAsset,
		RewardAsset: rule.rewardAsset,
		DailyLimit:  rule.dailyLimit,
		Jackpot:     jackpot,
		Collectible: collectible,
	})
}

func (s *CheckinService) GetOperationResult(ctx context.Context, userID int64, operationID string) (*CheckinResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("CHECKIN_UNAVAILABLE", "checkin service is not configured")
	}
	operationID, err := normalizeCheckinOperationID(operationID)
	if err != nil || operationID == "" {
		return nil, ErrCheckinOperationIDInvalid
	}
	return s.repo.GetOperationResult(ctx, userID, operationID)
}

func normalizeCheckinOperationID(raw string) (string, error) {
	operationID, err := NormalizeIdempotencyKey(raw)
	if err != nil {
		return "", ErrCheckinOperationIDInvalid
	}
	return operationID, nil
}

func (s *CheckinService) today() time.Time {
	now := time.Now
	if s != nil && s.now != nil {
		now = s.now
	}
	y, m, d := now().In(time.Local).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

type checkinRuleConfig struct {
	checkinType string
	cost        float64
	costAsset   string
	rewardAsset string
	dailyLimit  int
	rewards     []weightedCheckinReward
}

func checkinRule(checkinType string) (checkinRuleConfig, error) {
	switch checkinType {
	case CheckinTypeFree:
		return checkinRuleConfig{checkinType: CheckinTypeFree, rewardAsset: "credit", dailyLimit: 1, rewards: freeCheckinRewards}, nil
	case CheckinTypePaid, CheckinTypeCredit:
		return checkinRuleConfig{}, ErrCheckinCreditLotteryMoved
	case CheckinTypeBalance:
		return checkinRuleConfig{checkinType: CheckinTypeBalance, cost: checkinBalanceCost, costAsset: "balance", rewardAsset: "balance", dailyLimit: 10, rewards: balanceCheckinRewards}, nil
	default:
		return checkinRuleConfig{}, ErrCheckinInvalidType
	}
}

func isValidCheckinMilestone(days int) bool {
	return days == 3 || days == 5 || days == 7
}

func randomCollectibleCard(checkinType string) (*CheckinCollectibleCard, error) {
	threshold := int64(0)
	sourceLabel := checkinType
	switch checkinType {
	case CheckinTypeFree:
		threshold = freeCollectibleThreshold
	case CheckinTypePaid, CheckinTypeCredit:
		threshold = creditCollectibleThreshold
		sourceLabel = CheckinTypeCredit
	case CheckinTypeBalance:
		threshold = balanceCollectibleThreshold
	default:
		return nil, ErrCheckinInvalidType
	}
	if threshold <= 0 || len(collectibleCardTemplates) == 0 {
		return nil, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(collectibleDropScale))
	if err != nil {
		return nil, err
	}
	if n.Int64() >= threshold {
		return nil, nil
	}
	template, err := randomCollectibleTemplate()
	if err != nil {
		return nil, err
	}
	return &CheckinCollectibleCard{CardKey: template.Key, Rarity: template.Rarity, SourceType: "daily_fortune", SourceLabel: sourceLabel, EditionSupply: collectibleEditionSupply(template.Rarity)}, nil
}

func collectibleEditionSupply(rarity string) int64 {
	return collectibleEditionSupplies[strings.ToLower(strings.TrimSpace(rarity))]
}

func collectibleShopTierByRarity(rarity string) (collectibleShopTier, bool) {
	tier, ok := collectibleShopTiers[strings.ToLower(strings.TrimSpace(rarity))]
	return tier, ok
}

func randomCollectibleTemplate() (collectibleCardTemplate, error) {
	return randomCollectibleTemplateByRarity("")
}

func randomCollectibleTemplateByRarity(rarity string) (collectibleCardTemplate, error) {
	return randomCollectibleTemplateByRarityExcluding(rarity, nil)
}

func randomCollectibleTemplateByRarityExcluding(rarity string, exclude map[string]struct{}) (collectibleCardTemplate, error) {
	rarity = strings.ToLower(strings.TrimSpace(rarity))
	var totalWeight int64
	for _, template := range collectibleCardTemplates {
		if template.Key == "" || template.Rarity == "" || template.Weight <= 0 || (rarity != "" && template.Rarity != rarity) {
			continue
		}
		if _, skipped := exclude[template.Key]; skipped {
			continue
		}
		totalWeight += template.Weight
	}
	if totalWeight <= 0 {
		return collectibleCardTemplate{}, ErrCheckinInvalidType
	}
	n, err := rand.Int(rand.Reader, big.NewInt(totalWeight))
	if err != nil {
		return collectibleCardTemplate{}, err
	}
	threshold := n.Int64()
	var cumulative int64
	for _, template := range collectibleCardTemplates {
		if template.Key == "" || template.Rarity == "" || template.Weight <= 0 || (rarity != "" && template.Rarity != rarity) {
			continue
		}
		if _, skipped := exclude[template.Key]; skipped {
			continue
		}
		cumulative += template.Weight
		if threshold < cumulative {
			return template, nil
		}
	}
	return collectibleCardTemplate{}, ErrCheckinInvalidType
}

func randomCheckinReward(rewards []weightedCheckinReward) (float64, error) {
	var totalWeight int64
	for _, reward := range rewards {
		if reward.Value <= 0 || reward.Weight <= 0 {
			continue
		}
		totalWeight += reward.Weight
	}
	if totalWeight <= 0 {
		return 0, ErrCheckinInvalidType
	}
	n, err := rand.Int(rand.Reader, big.NewInt(totalWeight))
	if err != nil {
		return 0, err
	}
	threshold := n.Int64()
	var cumulative int64
	for _, reward := range rewards {
		if reward.Value <= 0 || reward.Weight <= 0 {
			continue
		}
		cumulative += reward.Weight
		if threshold < cumulative {
			return reward.Value, nil
		}
	}
	return 0, ErrCheckinInvalidType
}

func randomJackpotPlan(checkinType string) (JackpotPlan, error) {
	threshold := int64(0)
	plan := JackpotPlan{CelebrationRatio: 0.10}
	switch checkinType {
	case CheckinTypeFree:
		threshold = freeJackpotThreshold
		plan.WinnerRatio = 0.50
	case CheckinTypePaid, CheckinTypeCredit:
		threshold = creditJackpotThreshold
		plan.WinnerRatio = 0.60
	case CheckinTypeBalance:
		threshold = balanceJackpotThreshold
		plan.WinnerRatio = 0.80
	default:
		return JackpotPlan{}, ErrCheckinInvalidType
	}
	if threshold <= 0 {
		return plan, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(jackpotProbabilityScale))
	if err != nil {
		return JackpotPlan{}, err
	}
	plan.Hit = n.Int64() < threshold
	return plan, nil
}
