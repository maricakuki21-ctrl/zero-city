package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type BizProfile struct {
	UserID                      int64     `json:"user_id"`
	Handle                      string    `json:"handle"`
	DisplayName                 string    `json:"display_name"`
	AvatarURL                   string    `json:"avatar_url"`
	Bio                         string    `json:"bio"`
	ProfileVisibility           string    `json:"profile_visibility"`
	ProfileTheme                string    `json:"profile_theme"`
	BackgroundCardKey           string    `json:"background_card_key"`
	BackgroundCardRarity        string    `json:"background_card_rarity"`
	BackgroundCardSerialNo      *int64    `json:"background_card_serial_no,omitempty"`
	BackgroundCardEditionNo     *int64    `json:"background_card_edition_no,omitempty"`
	BackgroundCardEditionSupply *int64    `json:"background_card_edition_supply,omitempty"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

type ZeroCityProfileIdentityInput struct {
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

type ZeroCityProfileBackgroundInput struct {
	CardKey  string `json:"card_key"`
	SerialNo *int64 `json:"serial_no,omitempty"`
}

type ZeroCityFollowState struct {
	IsFollowing bool  `json:"is_following"`
	Followers   int64 `json:"followers"`
	Following   int64 `json:"following"`
}

type ZeroCityProfileStats struct {
	Followers        int64 `json:"followers"`
	Following        int64 `json:"following"`
	CommunityPosts   int64 `json:"community_posts"`
	SharedPools      int64 `json:"shared_pools"`
	CollectibleCards int64 `json:"collectible_cards"`
}

type ZeroCityPublicProfile struct {
	Profile     BizProfile           `json:"profile"`
	Stats       ZeroCityProfileStats `json:"stats"`
	FollowState ZeroCityFollowState  `json:"follow_state"`
	SharedPools []SharedPool         `json:"shared_pools"`
	Posts       []CommunityPost      `json:"posts"`
}

type BizContributorApplication struct {
	CapacityTypes    []string `json:"capacity_types"`
	SettlementMethod string   `json:"settlement_method"`
	CapacityHint     string   `json:"capacity_hint"`
	Label            string   `json:"label"`
}

type BizOperatorApplication struct {
	Skills []string `json:"skills"`
}

type BizCustomRequestInput struct {
	Goal       string   `json:"goal"`
	Context    string   `json:"context"`
	Budget     string   `json:"budget_range"`
	Deadline   string   `json:"deadline"`
	References []string `json:"references"`
}

type BizCreditGrantInput struct {
	UserID     int64   `json:"user_id"`
	SourceType string  `json:"source_type"`
	SourceID   string  `json:"source_id"`
	Amount     float64 `json:"amount"`
	Note       string  `json:"note"`
	CreatedBy  int64   `json:"created_by"`
}

type BizStarterCreditInput struct {
	UserID       int64   `json:"user_id"`
	Amount       float64 `json:"amount"`
	SignupSource string  `json:"signup_source"`
	Note         string  `json:"note"`
}

// InviteRewardInput describes a one-time invite reward grant.
// InviterID receives Amount credits when InviteeUserID registers and the
// inviter makes their first real API call.
type InviteRewardInput struct {
	InviterID       int64   `json:"inviter_id"`
	InviteeUserID   int64   `json:"invitee_user_id"`
	DirectInviterID int64   `json:"direct_inviter_id,omitempty"`
	Amount          float64 `json:"amount"`
	RewardKind      string  `json:"reward_kind"`
	InviteeAffCode  string  `json:"invitee_aff_code"`
}

type BizContributorProfile struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	Status           string    `json:"status"`
	CapacityTypes    []string  `json:"capacity_types"`
	SettlementMethod string    `json:"settlement_method"`
	RiskNotes        string    `json:"risk_notes"`
	AdminNotes       string    `json:"admin_notes"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type BizOperatorProfile struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Status         string    `json:"status"`
	Skills         []string  `json:"skills"`
	Rank           string    `json:"rank"`
	CompletedTasks int       `json:"completed_tasks"`
	QualityScore   float64   `json:"quality_score"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BizCustomRequest struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Goal       string     `json:"goal"`
	Context    string     `json:"context"`
	Budget     string     `json:"budget_range"`
	Deadline   *time.Time `json:"deadline,omitempty"`
	References []string   `json:"references"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
}

type BizCreditLedgerEntry struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	SourceType   string     `json:"source_type"`
	SourceID     string     `json:"source_id"`
	AssetType    string     `json:"asset_type"`
	Amount       float64    `json:"amount"`
	BalanceAfter float64    `json:"balance_after"`
	Status       string     `json:"status"`
	Note         string     `json:"note"`
	CreatedBy    *int64     `json:"created_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	PostedAt     *time.Time `json:"posted_at,omitempty"`
}

type SharedPoolBalanceLedgerEntry struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	PoolID       *int64     `json:"pool_id,omitempty"`
	SourceType   string     `json:"source_type"`
	SourceID     string     `json:"source_id"`
	AssetType    string     `json:"asset_type"`
	Amount       float64    `json:"amount"`
	BalanceAfter float64    `json:"balance_after"`
	Status       string     `json:"status"`
	Note         string     `json:"note"`
	CreatedAt    time.Time  `json:"created_at"`
	PostedAt     *time.Time `json:"posted_at,omitempty"`
}

type SharedPoolOwnerWallet struct {
	OwnerID           int64     `json:"owner_id"`
	AvailableAmount   float64   `json:"available_amount"`
	PendingAmount     float64   `json:"pending_amount"`
	FrozenAmount      float64   `json:"frozen_amount"`
	TransferredAmount float64   `json:"transferred_amount"`
	TotalEarned       float64   `json:"total_earned"`
	Version           int64     `json:"version"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type SharedPoolOwnerEarningsEntry struct {
	ID                    int64           `json:"id"`
	OwnerID               int64           `json:"owner_id"`
	PoolID                *int64          `json:"pool_id,omitempty"`
	AccountID             *int64          `json:"account_id,omitempty"`
	PriceVersionID        *int64          `json:"price_version_id,omitempty"`
	EventType             string          `json:"event_type"`
	OperationID           string          `json:"operation_id"`
	RequestID             string          `json:"request_id"`
	PoolNameSnapshot      string          `json:"pool_name_snapshot"`
	OwnerLabelSnapshot    string          `json:"owner_label_snapshot"`
	ModelSnapshot         string          `json:"model_snapshot"`
	PricingSourceSnapshot string          `json:"pricing_source_snapshot"`
	GrossAmount           float64         `json:"gross_amount"`
	PlatformFeeAmount     float64         `json:"platform_fee_amount"`
	NetAmount             float64         `json:"net_amount"`
	WalletDelta           float64         `json:"wallet_delta"`
	AvailableAfter        float64         `json:"available_after"`
	Status                string          `json:"status"`
	AvailableAt           *time.Time      `json:"available_at,omitempty"`
	Metadata              json.RawMessage `json:"metadata"`
	CreatedAt             time.Time       `json:"created_at"`
	PostedAt              *time.Time      `json:"posted_at,omitempty"`
}

type SharedPoolLedgerView struct {
	Wallet             SharedPoolOwnerWallet          `json:"wallet"`
	Earnings           []SharedPoolOwnerEarningsEntry `json:"earnings"`
	Activity           []SharedPoolBalanceLedgerEntry `json:"activity"`
	Withdrawable       []SharedPoolBalanceLedgerEntry `json:"withdrawable"`
	LegacyWithdrawable []SharedPoolBalanceLedgerEntry `json:"legacy_withdrawable"`
	Incentives         []BizCreditLedgerEntry         `json:"incentives"`
}

type SharedPoolOwnerEarningsPage struct {
	Items        []SharedPoolOwnerEarningsEntry `json:"items"`
	NextBeforeID int64                          `json:"next_before_id,omitempty"`
	HasMore      bool                           `json:"has_more"`
}

type AdminSharedPoolOwnerEarningsEntry struct {
	SharedPoolOwnerEarningsEntry
	OwnerEmail    string `json:"owner_email"`
	OwnerUsername string `json:"owner_username"`
}

type AdminSharedPoolOwnerEarningsFilter struct {
	OwnerID  int64
	PoolID   int64
	BeforeID int64
	Kind     string
	Status   string
	Limit    int
}

type AdminSharedPoolOwnerEarningsPage struct {
	Items               []AdminSharedPoolOwnerEarningsEntry `json:"items"`
	NextBeforeID        int64                               `json:"next_before_id,omitempty"`
	HasMore             bool                                `json:"has_more"`
	MatchingEntries     int64                               `json:"matching_entries"`
	MatchingOwners      int64                               `json:"matching_owners"`
	MatchingGrossAmount float64                             `json:"matching_gross_amount"`
	MatchingPlatformFee float64                             `json:"matching_platform_fee"`
	MatchingNetAmount   float64                             `json:"matching_net_amount"`
}

type SharedPoolOwnerWalletTransferResult struct {
	OperationID  string  `json:"operation_id"`
	Amount       float64 `json:"amount"`
	WalletAfter  float64 `json:"wallet_after"`
	BalanceAfter float64 `json:"balance_after"`
	AlreadyDone  bool    `json:"already_done"`
}

type BizPoolStatus struct {
	Health              string `json:"health"`
	ActiveResources     int64  `json:"active_resources"`
	TestingResources    int64  `json:"testing_resources"`
	PendingContributors int64  `json:"pending_contributors"`
	OpenRequests        int64  `json:"open_requests"`
}

// PlatformStatus reports the health of the underlying API gateway itself
// (channels & upstream accounts), deliberately kept SEPARATE from the
// marketplace pool status (BizPoolStatus). This backs the platform status
// bar required by the product design ("平台状态栏与共享池状态必须分离").
type PlatformStatus struct {
	// Health is an aggregate verdict: "operational" | "degraded" | "down".
	Health string `json:"health"`
	// ActiveChannels is the number of enabled (status='active') channels.
	ActiveChannels int64 `json:"active_channels"`
	// TotalChannels is the total number of configured channels.
	TotalChannels int64 `json:"total_channels"`
	// ActiveAccounts is the number of usable upstream accounts (status='active').
	ActiveAccounts int64 `json:"active_accounts"`
	// ErrorAccounts is the number of upstream accounts currently in error state.
	ErrorAccounts int64 `json:"error_accounts"`
	// TotalAccounts is the total number of non-deleted upstream accounts.
	TotalAccounts int64 `json:"total_accounts"`
}

// PromoCampaign is an admin-configurable, time-boxed reward campaign.
type PromoCampaign struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Enabled      bool      `json:"enabled"`
	CreditAmount float64   `json:"credit_amount"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	Target       string    `json:"target"`
	AutoHide     bool      `json:"auto_hide"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PromoCampaignInput is the payload for creating/updating a campaign.
type PromoCampaignInput struct {
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Enabled      bool      `json:"enabled"`
	CreditAmount float64   `json:"credit_amount"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	Target       string    `json:"target"`
	AutoHide     bool      `json:"auto_hide"`
}

// PromoActiveView is what an authenticated user sees for the active campaign,
// including whether they have already claimed it.
type PromoActiveView struct {
	Campaign  *PromoCampaign `json:"campaign"`
	Claimed   bool           `json:"claimed"`
	Claimable bool           `json:"claimable"`
}

// PromoClaimResult is returned after a successful (or idempotent) claim.
type PromoClaimResult struct {
	PromoID      int64   `json:"promo_id"`
	Amount       float64 `json:"amount"`
	BalanceAfter float64 `json:"balance_after"`
	AlreadyDone  bool    `json:"already_claimed"`
}

type CommunityPostInput struct {
	Kind         string          `json:"kind"`
	Title        string          `json:"title"`
	Body         string          `json:"body"`
	Tags         []string        `json:"tags"`
	District     string          `json:"district"`
	Channel      string          `json:"channel"`
	Private      bool            `json:"private"`
	SourceType   string          `json:"source_type"`
	SourceID     string          `json:"source_id"`
	Scenario     string          `json:"scenario"`
	SubjectType  string          `json:"subject_type"`
	SubjectID    string          `json:"subject_id"`
	SubjectTitle string          `json:"subject_title"`
	ActionType   string          `json:"action_type"`
	Evidence     json.RawMessage `json:"evidence"`
	TrustSignals json.RawMessage `json:"trust_signals"`
}

type CommunityPostQuery struct {
	Kind        string
	District    string
	Channel     string
	SourceType  string
	SourceID    string
	Scenario    string
	SubjectType string
	SubjectID   string
	ActionType  string
	Status      string
	PrivateOnly bool
	Limit       int
}

type SharedPoolCommunitySummary struct {
	PoolID          int64           `json:"pool_id"`
	TotalPosts      int             `json:"total_posts"`
	DiscussionPosts int             `json:"discussion_posts"`
	FeedbackPosts   int             `json:"feedback_posts"`
	IncidentPosts   int             `json:"incident_posts"`
	RiskSignals     int             `json:"risk_signals"`
	LastPostTitle   string          `json:"last_post_title"`
	LastPostAt      *time.Time      `json:"last_post_at,omitempty"`
	LatestPosts     []CommunityPost `json:"latest_posts,omitempty"`
}

type CommunityPost struct {
	ID           int64              `json:"id"`
	UserID       int64              `json:"user_id"`
	Author       string             `json:"author"`
	Kind         string             `json:"kind"`
	Title        string             `json:"title"`
	Body         string             `json:"body"`
	Tags         []string           `json:"tags"`
	District     string             `json:"district"`
	Channel      string             `json:"channel"`
	Private      bool               `json:"private"`
	Status       string             `json:"status"`
	Pinned       bool               `json:"pinned"`
	Catches      int                `json:"catches"`
	Replies      int                `json:"replies"`
	Views        int                `json:"views"`
	SourceType   string             `json:"source_type"`
	SourceID     string             `json:"source_id"`
	Scenario     string             `json:"scenario"`
	SubjectType  string             `json:"subject_type"`
	SubjectID    string             `json:"subject_id"`
	SubjectTitle string             `json:"subject_title"`
	ActionType   string             `json:"action_type"`
	Evidence     json.RawMessage    `json:"evidence"`
	TrustSignals json.RawMessage    `json:"trust_signals"`
	Comments     []CommunityComment `json:"comments,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

type CommunityCommentInput struct {
	Body       string `json:"body"`
	HelperRole string `json:"helper_role"`
}

type CommunityComment struct {
	ID         int64     `json:"id"`
	PostID     int64     `json:"post_id"`
	UserID     int64     `json:"user_id"`
	Author     string    `json:"author"`
	Body       string    `json:"body"`
	HelperRole string    `json:"helper_role"`
	Official   bool      `json:"official"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type TokenPowerLeaderboardRow struct {
	Rank           int64   `json:"rank"`
	UserID         int64   `json:"user_id"`
	DisplayName    string  `json:"display_name"`
	EffectiveSpend float64 `json:"effective_spend"`
	RequestCount   int64   `json:"request_count"`
	TokenCount     int64   `json:"token_count"`
	RewardCap      float64 `json:"reward_cap"`
	RewardAmount   float64 `json:"reward_amount"`
	ReviewStatus   string  `json:"review_status"`
}

type TokenPowerLeaderboard struct {
	WeekStart string                     `json:"week_start"`
	WeekEnd   string                     `json:"week_end"`
	RewardCap float64                    `json:"reward_cap"`
	Rules     []string                   `json:"rules"`
	Rows      []TokenPowerLeaderboardRow `json:"rows"`
}

// SharedPoolPriceSnapshot contains the canonical price shown in Account Square.
// Official models are populated by BillingService. Repository/channel pricing
// tables must never be used as a UI fallback because they may contain stale or
// route-specific overrides that are not the price used by runtime billing.
type SharedPoolPriceSnapshot struct {
	BillingMode      string   `json:"billing_mode"`
	InputPrice       *float64 `json:"input_price,omitempty"`
	OutputPrice      *float64 `json:"output_price,omitempty"`
	CacheWritePrice  *float64 `json:"cache_write_price,omitempty"`
	CacheReadPrice   *float64 `json:"cache_read_price,omitempty"`
	ImageOutputPrice *float64 `json:"image_output_price,omitempty"`
	PerRequestPrice  *float64 `json:"per_request_price,omitempty"`
	PriceSource      string   `json:"price_source"`
	// IntervalCount is internal routing metadata. Shared-pool pricing currently
	// supports one immutable component set per accepted quote, so tiered catalog
	// prices must fail closed until the complete interval schedule is versioned.
	IntervalCount int `json:"-"`
}

type ModelCatalogEntry struct {
	ID                    int64                    `json:"id"`
	Provider              string                   `json:"provider"`
	ModelName             string                   `json:"model_name"`
	DisplayName           string                   `json:"display_name"`
	Family                string                   `json:"family"`
	TierLabel             string                   `json:"tier_label"`
	CapabilityTags        []string                 `json:"capability_tags"`
	Aliases               []string                 `json:"aliases"`
	DefaultRateMultiplier float64                  `json:"default_rate_multiplier"`
	DefaultRankWeight     float64                  `json:"default_rank_weight"`
	Mainstream            bool                     `json:"mainstream"`
	Enabled               bool                     `json:"enabled"`
	SortOrder             int                      `json:"sort_order"`
	ModalitiesIn          []string                 `json:"modalities_in"`
	ModalitiesOut         []string                 `json:"modalities_out"`
	AdapterKind           string                   `json:"adapter_kind"`
	ContextWindow         int                      `json:"context_window"`
	Orchestrator          bool                     `json:"orchestrator"`
	RuntimeRole           string                   `json:"runtime_role"`
	Pricing               *SharedPoolPriceSnapshot `json:"pricing,omitempty"`
}

// ModelCapabilityProfileInput updates the declared capability matrix entry for
// one catalog model. Empty slices and empty strings leave the stored value
// untouched so a partial admin edit cannot silently blank a model's profile.
type ModelCapabilityProfileInput struct {
	ModalitiesIn  []string
	ModalitiesOut []string
	AdapterKind   string
	ContextWindow *int
	Orchestrator  *bool
	RuntimeRole   string
}

// ModelCatalogProfileUpdate is the repository-facing, already-normalized form
// of a capability matrix edit. Empty values mean "leave unchanged".
type ModelCatalogProfileUpdate struct {
	ModalitiesIn  []string
	ModalitiesOut []string
	AdapterKind   string
	ContextWindow *int
	Orchestrator  *bool
	RuntimeRole   string
}

type SharedPoolModelConfig struct {
	Provider                  string                   `json:"provider"`
	ModelName                 string                   `json:"model_name"`
	UpstreamModelName         string                   `json:"upstream_model_name"`
	DisplayName               string                   `json:"display_name"`
	Aliases                   []string                 `json:"aliases"`
	RateMultiplier            float64                  `json:"rate_multiplier"`
	RankWeight                float64                  `json:"rank_weight"`
	FiveHourProtectionPercent float64                  `json:"five_hour_protection_percent"`
	SevenDayProtectionPercent float64                  `json:"seven_day_protection_percent"`
	DailyProtectionPercent    float64                  `json:"daily_protection_percent"`
	MinBalanceAdmission       float64                  `json:"min_balance_admission"`
	HourlySeatFee             float64                  `json:"hourly_seat_fee"`
	HourlyMinUsageWaiver      float64                  `json:"hourly_min_usage_waiver"`
	MaxConcurrency            int                      `json:"max_concurrency"`
	ModelOpen                 bool                     `json:"model_open"`
	Tags                      []string                 `json:"tags"`
	Pricing                   *SharedPoolPriceSnapshot `json:"pricing,omitempty"`
	// CanonicalModelName is the enabled model_catalog identity resolved inside
	// the provider boundary. It is internal-only and lets the service ask the
	// same BillingService used by runtime billing for the displayed price.
	CanonicalModelName string `json:"-"`
}

type SharedPoolAccountSummary struct {
	TotalAccounts           int        `json:"total_accounts"`
	ConfiguredAccounts      int        `json:"configured_accounts"`
	SchedulableAccounts     int        `json:"schedulable_accounts"`
	GateBlockedAccounts     int        `json:"gate_blocked_accounts"`
	DisabledAccounts        int        `json:"disabled_accounts"`
	TotalAccountConcurrency int        `json:"total_account_concurrency"`
	TotalUserConcurrency    int        `json:"total_user_concurrency"`
	TotalRPMLimit           int        `json:"total_rpm_limit"`
	AverageFullCheckScore   float64    `json:"average_full_check_score"`
	FullCheckPassedAccounts int        `json:"full_check_passed_accounts"`
	FullCheckTotalAccounts  int        `json:"full_check_total_accounts"`
	TotalCalls              int64      `json:"total_calls"`
	SuccessfulCalls         int64      `json:"successful_calls"`
	FailedCalls             int64      `json:"failed_calls"`
	SuccessRate             float64    `json:"success_rate"`
	ModelCoverage           []string   `json:"model_coverage"`
	LastProbeAt             *time.Time `json:"last_probe_at,omitempty"`
}

// SharedPool is a read-only catalog entry in the Account Square marketplace.
// NOTE: joining / binding / seat-hour billing are intentionally NOT modeled here
// (separate, money-sensitive workstream). This is the browse-only view.
type SharedPoolOwnerCardAsset struct {
	CollectibleCount       int64  `json:"collectible_count"`
	HighestRarity          string `json:"highest_rarity"`
	FeaturedCardKey        string `json:"featured_card_key"`
	FeaturedCardRarity     string `json:"featured_card_rarity"`
	FeaturedCardSerialNo   *int64 `json:"featured_card_serial_no,omitempty"`
	FeaturedCardEditionNo  *int64 `json:"featured_card_edition_no,omitempty"`
	FeaturedCardSupply     *int64 `json:"featured_card_supply,omitempty"`
	ProfileBackgroundReady bool   `json:"profile_background_ready"`
}

type SharedPool struct {
	ID                          int64                    `json:"id"`
	Name                        string                   `json:"name"`
	Description                 string                   `json:"description"`
	AvatarURL                   string                   `json:"avatar_url"`
	CardSkinKey                 string                   `json:"card_skin_key,omitempty"`
	CardSkinRarity              string                   `json:"card_skin_rarity,omitempty"`
	StatusNote                  string                   `json:"status_note"`
	DisabledReason              string                   `json:"disabled_reason"`
	FeaturedScore               float64                  `json:"featured_score"`
	OwnerID                     *int64                   `json:"owner_id,omitempty"`
	OwnerLabel                  string                   `json:"owner_label"`
	OwnerCardAsset              SharedPoolOwnerCardAsset `json:"owner_card_asset"`
	Tier                        string                   `json:"tier"`
	Status                      string                   `json:"status"`
	Listed                      bool                     `json:"listed"`
	Models                      []string                 `json:"models"`
	ModelConfigs                []SharedPoolModelConfig  `json:"model_configs"`
	AccountSummary              SharedPoolAccountSummary `json:"account_summary"`
	RateMultiplier              float64                  `json:"rate_multiplier"`
	MaxUsers                    int                      `json:"max_users"`
	CurrentUsers                int                      `json:"current_users"`
	MinBalanceAdmission         float64                  `json:"min_balance_admission"`
	HourlySeatFee               float64                  `json:"hourly_seat_fee"`
	HourlyMinUsageWaiver        float64                  `json:"hourly_min_usage_waiver"`
	SettlementRuleEffective     *time.Time               `json:"settlement_rule_effective_from,omitempty"`
	PendingHourlySeatFee        *float64                 `json:"pending_hourly_seat_fee,omitempty"`
	PendingHourlyMinUsageWaiver *float64                 `json:"pending_hourly_min_usage_waiver,omitempty"`
	PendingPlatformFeePercent   *float64                 `json:"pending_platform_fee_percent,omitempty"`
	PendingRuleEffective        *time.Time               `json:"pending_settlement_rule_effective_from,omitempty"`
	TodayAvailability           float64                  `json:"today_availability"`
	SevenDayAvail               float64                  `json:"seven_day_availability"`
	AvgLatencyMs                int                      `json:"avg_latency_ms"`
	LastProbeAt                 *time.Time               `json:"last_probe_at,omitempty"`
	LastProbeSuccess            *bool                    `json:"last_probe_success,omitempty"`
	LastProbeErrorType          string                   `json:"last_probe_error_type"`
	LastProbeErrorMessage       string                   `json:"last_probe_error_message"`
	ConsecutiveProbeFailures    int                      `json:"consecutive_probe_failures"`
	LastSuccessfulProbeAt       *time.Time               `json:"last_successful_probe_at,omitempty"`
	LastProbeCheckLevel         string                   `json:"last_probe_check_level"`
	LastProbeGateRequired       bool                     `json:"last_probe_gate_required"`
	LastProbeGatePassed         bool                     `json:"last_probe_gate_passed"`
	LastProbeFullCheckPassed    int                      `json:"last_probe_full_check_passed"`
	LastProbeFullCheckTotal     int                      `json:"last_probe_full_check_total"`
	LastProbeFullCheckScore     float64                  `json:"last_probe_full_check_score"`
	UpstreamBaseURL             string                   `json:"upstream_base_url,omitempty"`
	HasUpstreamKey              bool                     `json:"has_upstream_key"`
	ProxyID                     *int64                   `json:"proxy_id,omitempty"`
	ProxyURL                    string                   `json:"proxy_url,omitempty"`
	ProxyRegion                 string                   `json:"proxy_region,omitempty"`
	ProxyStatus                 string                   `json:"proxy_status,omitempty"`
	AccountConcurrency          int                      `json:"account_concurrency"`
	UserConcurrency             int                      `json:"user_concurrency"`
	AccountModeEnabled          bool                     `json:"account_mode_enabled"`
	OAuthProvider               string                   `json:"oauth_provider,omitempty"`
	VerificationMode            string                   `json:"verification_mode,omitempty"`
	VerificationExemptionReason string                   `json:"verification_exemption_reason,omitempty"`
	OwnerSharePercent           float64                  `json:"owner_share_percent"`
	PlatformFeePercent          float64                  `json:"platform_fee_percent"`
	QualityScore                float64                  `json:"quality_score"`
	RankWeight                  float64                  `json:"rank_weight"`
	RewardScore                 float64                  `json:"reward_score"`
	PenaltyScore                float64                  `json:"penalty_score"`
	MarketScore                 float64                  `json:"market_score"`
	GovernanceStatus            string                   `json:"governance_status"`
	GovernanceNote              string                   `json:"governance_note"`
	AdminNote                   string                   `json:"admin_note,omitempty"`
	ComplaintCount              int                      `json:"complaint_count"`
	LikeCount                   int                      `json:"like_count"`
	LikedByMe                   bool                     `json:"liked_by_me"`
	TotalCalls                  int64                    `json:"total_calls"`
	SuccessfulCalls             int64                    `json:"successful_calls"`
	FailedCalls                 int64                    `json:"failed_calls"`
	LifecycleState              string                   `json:"lifecycle_state"`
	OwnerPaused                 bool                     `json:"owner_paused"`
	ArchivedAt                  *time.Time               `json:"archived_at,omitempty"`
	ArchivedBy                  *int64                   `json:"archived_by,omitempty"`
	ArchiveReason               string                   `json:"archive_reason"`
	SourceKind                  string                   `json:"source_kind"`
	ConfigVersion               int64                    `json:"config_version,omitempty"`
	NativeOnboardingState       string                   `json:"native_onboarding_state,omitempty"`
	BillingActivationRequired   bool                     `json:"billing_activation_required"`
}

type SharedPoolAccount struct {
	ID                             int64                   `json:"id"`
	PoolID                         int64                   `json:"pool_id"`
	OwnerID                        int64                   `json:"owner_id"`
	Name                           string                  `json:"name"`
	Description                    string                  `json:"description"`
	Provider                       string                  `json:"provider"`
	AuthType                       string                  `json:"auth_type"`
	UpstreamBaseURL                string                  `json:"upstream_base_url"`
	HasUpstreamKey                 bool                    `json:"has_upstream_key"`
	HasOAuthCredentials            bool                    `json:"has_oauth_credentials"`
	KeyPreview                     string                  `json:"key_preview"`
	CredentialFingerprint          string                  `json:"-"`
	ExpiresAt                      *time.Time              `json:"expires_at,omitempty"`
	AutoPauseOnExpired             bool                    `json:"auto_pause_on_expired"`
	Schedulable                    bool                    `json:"schedulable"`
	Status                         string                  `json:"status"`
	StatusNote                     string                  `json:"status_note"`
	DisabledReason                 string                  `json:"disabled_reason"`
	GroupName                      string                  `json:"group_name"`
	ProxyID                        *int64                  `json:"proxy_id,omitempty"`
	ProxyURL                       string                  `json:"proxy_url"`
	ProxyRegion                    string                  `json:"proxy_region"`
	ProxyStatus                    string                  `json:"proxy_status"`
	AccountWeight                  float64                 `json:"account_weight"`
	Priority                       int                     `json:"priority"`
	RPMLimit                       int                     `json:"rpm_limit"`
	AccountConcurrency             int                     `json:"account_concurrency"`
	UserConcurrency                int                     `json:"user_concurrency"`
	TLSProfileID                   *int64                  `json:"tls_profile_id,omitempty"`
	TTLSeconds                     int                     `json:"ttl_seconds"`
	CachePolicy                    json.RawMessage         `json:"cache_policy"`
	RoutingPolicy                  json.RawMessage         `json:"routing_policy"`
	ModelConfigs                   []SharedPoolModelConfig `json:"model_configs"`
	LastProbeAt                    *time.Time              `json:"last_probe_at,omitempty"`
	LastProbeSuccess               *bool                   `json:"last_probe_success,omitempty"`
	LastProbeErrorType             string                  `json:"last_probe_error_type"`
	LastProbeErrorMessage          string                  `json:"last_probe_error_message"`
	LastSuccessfulProbeAt          *time.Time              `json:"last_successful_probe_at,omitempty"`
	FullCheckScore                 float64                 `json:"full_check_score"`
	FullCheckPassed                int                     `json:"full_check_passed"`
	FullCheckTotal                 int                     `json:"full_check_total"`
	GateRequired                   bool                    `json:"gate_required"`
	GatePassed                     bool                    `json:"gate_passed"`
	TotalCalls                     int64                   `json:"total_calls"`
	SuccessfulCalls                int64                   `json:"successful_calls"`
	FailedCalls                    int64                   `json:"failed_calls"`
	LastUsedAt                     *time.Time              `json:"last_used_at,omitempty"`
	CreatedAt                      time.Time               `json:"created_at"`
	UpdatedAt                      time.Time               `json:"updated_at"`
	NativeBindingState             string                  `json:"native_binding_state,omitempty"`
	NativeBindingStep              string                  `json:"native_binding_step,omitempty"`
	NativeOperationID              string                  `json:"native_operation_id,omitempty"`
	NativeErrorCode                string                  `json:"native_error_code,omitempty"`
	NativeErrorMessage             string                  `json:"native_error_message,omitempty"`
	NativeModels                   []string                `json:"native_models,omitempty"`
	NativeModelsVerifiedAt         *time.Time              `json:"native_models_verified_at,omitempty"`
	NativeConnectionStatus         string                  `json:"native_connection_status,omitempty"`
	NativeConnectionVerifiedAt     *time.Time              `json:"native_connection_verified_at,omitempty"`
	NativeEvidenceStale            bool                    `json:"native_evidence_stale"`
	BillingActivationRequired      bool                    `json:"billing_activation_required"`
	NativeAccountObservedUpdatedAt *time.Time              `json:"-"`
}

// SharedPoolListView wraps the marketplace listing plus aggregate stats for the
// header cards (total / online / limited / average availability).
type SharedPoolListView struct {
	Pools             []SharedPool                 `json:"pools"`
	Total             int                          `json:"total"`
	Online            int                          `json:"online"`
	Limited           int                          `json:"limited"`
	AvgAvailability   float64                      `json:"avg_availability"`
	GovernanceSummary *SharedPoolGovernanceSummary `json:"governance_summary,omitempty"`
}

type SharedPoolGovernanceSummary struct {
	SnapshotAt            time.Time `json:"snapshot_at"`
	TodayGrossCharges     float64   `json:"today_gross_charges"`
	TodayOwnerPayout      float64   `json:"today_owner_payout"`
	TodayPlatformFee      float64   `json:"today_platform_fee"`
	TodayAPIUsageCharges  float64   `json:"today_api_usage_charges"`
	TodayAPIOwnerPayout   float64   `json:"today_api_owner_payout"`
	TodayAPIPlatformFee   float64   `json:"today_api_platform_fee"`
	TodaySeatFeeCharges   float64   `json:"today_seat_fee_charges"`
	TodaySeatOwnerPayout  float64   `json:"today_seat_owner_payout"`
	TodaySeatPlatformFee  float64   `json:"today_seat_platform_fee"`
	TodayUsageCount       int64     `json:"today_usage_count"`
	TodayActivePools      int64     `json:"today_active_pools"`
	OpenComplaints        int64     `json:"open_complaints"`
	ListedPools           int64     `json:"listed_pools"`
	HealthyPools          int64     `json:"healthy_pools"`
	LimitedPools          int64     `json:"limited_pools"`
	OfflinePools          int64     `json:"offline_pools"`
	WatchPools            int64     `json:"watch_pools"`
	SuppressedPools       int64     `json:"suppressed_pools"`
	BannedPools           int64     `json:"banned_pools"`
	TotalCalls            int64     `json:"total_calls"`
	SuccessfulCalls       int64     `json:"successful_calls"`
	FailedCalls           int64     `json:"failed_calls"`
	AverageAvailability   float64   `json:"average_availability"`
	PlatformFeeBasis      string    `json:"platform_fee_basis"`
	AutoGovernanceSummary string    `json:"auto_governance_summary"`
}

type SharedPoolUpstreamProbeInput struct {
	PoolID          int64
	AccountID       int64
	OwnerID         int64
	UpstreamBaseURL string
	UpstreamAPIKey  string
	ProbeModel      string
	ProbeType       string
	ProxyURL        string
	// SkipRecord lets the durable probe-job finalizer own history/readiness
	// writes atomically after it verifies the pool config_version fence.
	SkipRecord bool
}

type SharedPoolUpstreamModelsInput struct {
	PoolID          int64
	AccountID       int64
	OwnerID         int64
	UpstreamBaseURL string
	UpstreamAPIKey  string
	ProxyURL        string
}

type SharedPoolUpstreamModelsResult struct {
	Models     []string `json:"models"`
	CheckedAt  string   `json:"checked_at"`
	HTTPStatus int      `json:"http_status"`
}

type SharedPoolFullCheckItem struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	Required     bool   `json:"required"`
	Success      bool   `json:"success"`
	HTTPStatus   int    `json:"http_status"`
	LatencyMs    int    `json:"latency_ms"`
	Evidence     string `json:"evidence,omitempty"`
	ErrorType    string `json:"error_type,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type SharedPoolProbeMetadata struct {
	CheckLevel      string                    `json:"check_level"`
	GateRequired    bool                      `json:"gate_required"`
	GatePassed      bool                      `json:"gate_passed"`
	FullCheckPassed int                       `json:"full_check_passed"`
	FullCheckTotal  int                       `json:"full_check_total"`
	FullCheckScore  float64                   `json:"full_check_score"`
	Checks          []SharedPoolFullCheckItem `json:"checks,omitempty"`
}

type SharedPoolUpstreamProbeResult struct {
	OK              bool                      `json:"ok"`
	Model           string                    `json:"model"`
	Models          []string                  `json:"models"`
	Message         string                    `json:"message"`
	CheckedAt       string                    `json:"checked_at"`
	LatencyMs       int                       `json:"latency_ms"`
	HTTPStatus      int                       `json:"http_status"`
	ErrorType       string                    `json:"error_type,omitempty"`
	ErrorMessage    string                    `json:"error_message,omitempty"`
	CheckLevel      string                    `json:"check_level"`
	GateRequired    bool                      `json:"gate_required"`
	GatePassed      bool                      `json:"gate_passed"`
	FullCheckPassed int                       `json:"full_check_passed"`
	FullCheckTotal  int                       `json:"full_check_total"`
	FullCheckScore  float64                   `json:"full_check_score"`
	Checks          []SharedPoolFullCheckItem `json:"checks,omitempty"`
}

type SharedPoolProbeHistory struct {
	ID                int64                   `json:"id"`
	PoolID            int64                   `json:"pool_id"`
	AccountID         *int64                  `json:"account_id,omitempty"`
	OwnerID           *int64                  `json:"owner_id,omitempty"`
	ModelName         string                  `json:"model_name"`
	UpstreamModelName string                  `json:"upstream_model_name"`
	ProbeType         string                  `json:"probe_type"`
	Success           bool                    `json:"success"`
	HTTPStatus        int                     `json:"http_status"`
	ErrorType         string                  `json:"error_type"`
	ErrorMessage      string                  `json:"error_message"`
	LatencyMs         int                     `json:"latency_ms"`
	CheckedAt         time.Time               `json:"checked_at"`
	CreatedAt         time.Time               `json:"created_at"`
	Metadata          SharedPoolProbeMetadata `json:"metadata"`
}

type SharedPoolProbeHistoryInput struct {
	PoolID            int64
	AccountID         int64
	OwnerID           int64
	ModelName         string
	UpstreamModelName string
	ProbeType         string
	Success           bool
	HTTPStatus        int
	ErrorType         string
	ErrorMessage      string
	LatencyMs         int
	CheckedAt         time.Time
	Metadata          SharedPoolProbeMetadata
}

type SharedPoolProbeCandidate struct {
	PoolID            int64
	AccountID         int64
	OwnerID           int64
	UpstreamBaseURL   string
	UpstreamAPIKey    string
	ProbeModel        string
	UpstreamModelName string
	ProxyURL          string
	FullProbeRequired bool
}

type SharedPoolProbeError struct {
	ErrorType  string
	HTTPStatus int
	Message    string
	Cause      error
}

func (e *SharedPoolProbeError) Error() string {
	if e == nil {
		return "shared pool upstream probe failed"
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "shared pool upstream probe failed"
}

func (e *SharedPoolProbeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type SharedPoolGovernanceLog struct {
	ID          int64           `json:"id"`
	PoolID      int64           `json:"pool_id"`
	AdminUserID *int64          `json:"admin_user_id,omitempty"`
	Action      string          `json:"action"`
	BeforeValue json.RawMessage `json:"before_value"`
	AfterValue  json.RawMessage `json:"after_value"`
	Reason      string          `json:"reason"`
	CreatedAt   time.Time       `json:"created_at"`
}

type SharedPoolUpstreamRuntime struct {
	AuthType             string     `json:"-"`
	UpstreamBaseURL      string     `json:"-"`
	UpstreamAPIKey       string     `json:"-"`
	CredentialsEncrypted string     `json:"-"`
	ProxyURL             string     `json:"-"`
	ProbeModel           string     `json:"-"`
	ExpiresAt            *time.Time `json:"-"`
}

// PoolSeat is an active (or historical) seat a user holds in a shared pool.
type PoolSeat struct {
	ID                      int64     `json:"id"`
	PoolID                  int64     `json:"pool_id"`
	PoolName                string    `json:"pool_name"`
	UserID                  int64     `json:"user_id"`
	Status                  string    `json:"status"`
	HourlySeatFee           float64   `json:"hourly_seat_fee"`
	HourlyMinUsageWaiver    float64   `json:"hourly_min_usage_waiver"`
	CurrentHourUsageAmount  float64   `json:"current_hour_usage_amount"`
	CurrentHourWaiverRemain float64   `json:"current_hour_waiver_remaining"`
	CurrentHourWaiverMet    bool      `json:"current_hour_waiver_met"`
	JoinedAt                time.Time `json:"joined_at"`
	LastChargedAt           time.Time `json:"last_charged_at"`
	// LastActivityAt is join time or the latest shared-pool API activity.
	// Active seats idle for SharedPoolSeatIdleTimeout are auto-released.
	LastActivityAt time.Time `json:"last_activity_at"`
	// IdleReleaseAt is when an active seat will be auto-released if no further activity occurs.
	IdleReleaseAt *time.Time `json:"idle_release_at,omitempty"`
	ReleasedAt    *time.Time `json:"released_at,omitempty"`
	ReleaseReason string     `json:"release_reason,omitempty"`
	TotalCharged  float64    `json:"total_charged"`
}

// SharedPoolSeatIdleTimeout is the product rule for automatic seat release:
// no real shared-pool API activity for this duration releases the seat.
const SharedPoolSeatIdleTimeout = 2 * time.Hour

// JoinPoolResult is returned after a successful (or idempotent) join.
type JoinPoolResult struct {
	Seat        PoolSeat `json:"seat"`
	AlreadyHeld bool     `json:"already_held"`
}

type SharedPoolAccessKey struct {
	ID                        int64                 `json:"id"`
	PoolID                    int64                 `json:"pool_id"`
	PoolName                  string                `json:"pool_name"`
	UserID                    int64                 `json:"user_id"`
	APIKeyID                  int64                 `json:"api_key_id"`
	Name                      string                `json:"name"`
	Key                       string                `json:"key,omitempty"`
	KeyPreview                string                `json:"key_preview"`
	Status                    string                `json:"status"`
	AllowedModels             []string              `json:"allowed_models"`
	TotalUsed                 float64               `json:"total_used"`
	LastUsedAt                *time.Time            `json:"last_used_at,omitempty"`
	CreatedAt                 time.Time             `json:"created_at"`
	OwnerID                   *int64                `json:"owner_id,omitempty"`
	AccountID                 int64                 `json:"shared_pool_account_id,omitempty"`
	AuthType                  string                `json:"-"`
	UpstreamBaseURL           string                `json:"-"`
	UpstreamAPIKey            string                `json:"-"`
	OAuthCredentialsEncrypted string                `json:"-"`
	OAuthCredentials          map[string]any        `json:"-"`
	ExpiresAt                 *time.Time            `json:"-"`
	PublishedModelName        string                `json:"-"`
	UpstreamModelName         string                `json:"-"`
	CanonicalModelName        string                `json:"-"`
	Provider                  string                `json:"-"`
	ProxyURL                  string                `json:"-"`
	RateMultiplier            float64               `json:"rate_multiplier"`
	OwnerSharePercent         float64               `json:"owner_share_percent"`
	AccountConcurrency        int                   `json:"account_concurrency"`
	UserConcurrency           int                   `json:"user_concurrency"`
	ModelConcurrency          int                   `json:"model_concurrency"`
	AccountMode               bool                  `json:"account_mode"`
	PriceQuote                *SharedPoolPriceQuote `json:"-"`
}

type SharedPoolUsageInput struct {
	AccessKeyID    int64
	PoolID         int64
	AccountID      int64
	UserID         int64
	Cost           float64
	Model          string
	RequestID      string
	Success        bool
	PriceVersionID int64
	PricingSource  string
	PriceSnapshot  json.RawMessage
}

type SharedPoolModelInput struct {
	Provider                  string
	ModelName                 string
	UpstreamModelName         string
	RateMultiplier            float64
	FiveHourProtectionPercent float64
	SevenDayProtectionPercent float64
	DailyProtectionPercent    float64
	MaxConcurrency            int
	ModelOpen                 bool
}

type CreateSharedPoolInput struct {
	OwnerID                     int64
	Name                        string
	Description                 string
	AvatarURL                   string
	StatusNote                  string
	DisabledReason              string
	UpstreamBaseURL             string
	UpstreamAPIKey              string
	Models                      []string
	ModelConfigs                []SharedPoolModelInput
	RateMultiplier              float64
	MaxUsers                    int
	MinBalanceAdmission         float64
	HourlySeatFee               float64
	HourlyMinUsageWaiver        float64
	PlatformFeePercent          float64
	ProxyID                     *int64
	ProxyURL                    string
	ProxyRegion                 string
	ProxyStatus                 string
	AccountConcurrency          int
	UserConcurrency             int
	AccountModeEnabled          bool
	OAuthProvider               string
	VerificationMode            string
	VerificationExemptionReason string
	ProbeModel                  string
	Listed                      bool
	Status                      string
	SupplyMode                  string
	OperationID                 string
	RequestFingerprint          string
}

type UpdateSharedPoolInput struct {
	ExpectedConfigVersion       int64
	Name                        string
	NameSet                     bool
	Description                 string
	DescriptionSet              bool
	AvatarURL                   string
	AvatarURLSet                bool
	StatusNote                  string
	StatusNoteSet               bool
	DisabledReason              string
	DisabledReasonSet           bool
	UpstreamBaseURL             string
	UpstreamBaseURLSet          bool
	UpstreamAPIKey              string
	UpstreamAPIKeySet           bool
	Models                      []string
	ModelsSet                   bool
	ModelConfigs                []SharedPoolModelInput
	ModelConfigsSet             bool
	RateMultiplier              float64
	RateMultiplierSet           bool
	SyncModelRates              bool
	MaxUsers                    int
	MaxUsersSet                 bool
	MinBalanceAdmission         float64
	MinBalanceAdmissionSet      bool
	HourlySeatFee               float64
	HourlySeatFeeSet            bool
	HourlyMinUsageWaiver        float64
	HourlyMinUsageWaiverSet     bool
	ProxyID                     *int64
	ProxyIDSet                  bool
	ProxyURL                    string
	ProxyURLSet                 bool
	ProxyRegion                 string
	ProxyRegionSet              bool
	ProxyStatus                 string
	ProxyStatusSet              bool
	AccountConcurrency          int
	AccountConcurrencySet       bool
	UserConcurrency             int
	UserConcurrencySet          bool
	AccountModeEnabled          bool
	AccountModeSet              bool
	OAuthProvider               string
	OAuthProviderSet            bool
	VerificationMode            string
	VerificationModeSet         bool
	VerificationExemptionReason string
	VerificationReasonSet       bool
	ProbeModel                  string
	ProbeModelSet               bool
	Listed                      bool
	ListedSet                   bool
	Status                      string
	StatusSet                   bool
}

type ImportSharedPoolsInput struct {
	OwnerID int64
	Items   []CreateSharedPoolInput
}

type SharedPoolAccountInput struct {
	PoolID                int64
	OwnerID               int64
	Name                  string
	Description           string
	Provider              string
	AuthType              string
	UpstreamBaseURL       string
	UpstreamAPIKey        string
	CredentialsEncrypted  string
	CredentialFingerprint string
	ExpiresAt             *time.Time
	AutoPauseOnExpired    bool
	AutoPauseOnExpiredSet bool
	Schedulable           bool
	SchedulableSet        bool
	Status                string
	StatusNote            string
	DisabledReason        string
	GroupName             string
	ProxyID               *int64
	ProxyURL              string
	ProxyRegion           string
	ProxyStatus           string
	AccountWeight         float64
	Priority              int
	RPMLimit              int
	AccountConcurrency    int
	UserConcurrency       int
	TLSProfileID          *int64
	TTLSeconds            int
	CachePolicy           json.RawMessage
	RoutingPolicy         json.RawMessage
	ModelConfigs          []SharedPoolModelInput
	GateRequired          bool
	GatePassed            bool
	FullCheckScore        float64
	FullCheckPassed       int
	FullCheckTotal        int
	OperationID           string
}

type ImportSharedPoolAccountsInput struct {
	PoolID  int64
	OwnerID int64
	Items   []SharedPoolAccountInput
}

type SharedPoolOAuthDataAccount struct {
	Name               string         `json:"name"`
	Platform           string         `json:"platform"`
	Type               string         `json:"type"`
	Credentials        map[string]any `json:"credentials"`
	Extra              map[string]any `json:"extra,omitempty"`
	Concurrency        int            `json:"concurrency"`
	Priority           int            `json:"priority"`
	ExpiresAt          *int64         `json:"expires_at,omitempty"`
	AutoPauseOnExpired *bool          `json:"auto_pause_on_expired,omitempty"`
}

type SharedPoolOAuthDataPackage struct {
	Type     string                       `json:"type"`
	Version  int                          `json:"version"`
	Accounts []SharedPoolOAuthDataAccount `json:"accounts"`
}

type ImportSharedPoolOAuthPackageInput struct {
	PoolID         int64
	OwnerID        int64
	Data           SharedPoolOAuthDataPackage
	UpdateExisting bool
}

type SharedPoolOAuthImportMessage struct {
	Index   int    `json:"index"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

type SharedPoolOAuthImportItemResult struct {
	Index     int    `json:"index"`
	Name      string `json:"name,omitempty"`
	Action    string `json:"action"`
	AccountID int64  `json:"account_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

type SharedPoolOAuthImportResult struct {
	Total    int                               `json:"total"`
	Created  int                               `json:"created"`
	Updated  int                               `json:"updated"`
	Skipped  int                               `json:"skipped"`
	Failed   int                               `json:"failed"`
	Items    []SharedPoolOAuthImportItemResult `json:"items"`
	Warnings []SharedPoolOAuthImportMessage    `json:"warnings,omitempty"`
	Errors   []SharedPoolOAuthImportMessage    `json:"errors,omitempty"`
}

type SharedPoolAccountImportItemResult struct {
	Index   int                `json:"index"`
	Name    string             `json:"name"`
	Created bool               `json:"created"`
	Account *SharedPoolAccount `json:"account,omitempty"`
	Error   string             `json:"error,omitempty"`
}

type SharedPoolAccountImportResult struct {
	Total   int                                 `json:"total"`
	Created int                                 `json:"created"`
	Failed  int                                 `json:"failed"`
	Items   []SharedPoolAccountImportItemResult `json:"items"`
}

type SharedPoolImportItemResult struct {
	Index   int         `json:"index"`
	Name    string      `json:"name"`
	Created bool        `json:"created"`
	Pool    *SharedPool `json:"pool,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type SharedPoolImportResult struct {
	Total   int                          `json:"total"`
	Created int                          `json:"created"`
	Failed  int                          `json:"failed"`
	Items   []SharedPoolImportItemResult `json:"items"`
}

type SharedPoolAccessKeyResult struct {
	AccessKey   SharedPoolAccessKey `json:"access_key"`
	AlreadyHeld bool                `json:"already_held"`
}

// ChargeSeatsSummary reports the outcome of one billing sweep.
type ChargeSeatsSummary struct {
	SeatsProcessed       int     `json:"seats_processed"`
	HoursCharged         int     `json:"hours_charged"`
	TotalCharged         float64 `json:"total_charged"`
	IdleSeatsReleased    int     `json:"idle_seats_released"`
	StabilityRewards     int     `json:"stability_rewards"`
	TotalStabilityCredit float64 `json:"total_stability_credit"`
}

// ErrPoolNotJoinable is returned when a pool cannot be joined (unlisted, full,
// offline, or the user lacks the minimum admission balance).
var ErrPoolNotJoinable = errors.New("pool is not joinable")

// ErrPoolInsufficientBalance is returned when the user's balance is below the
// pool's minimum admission requirement.
var ErrPoolInsufficientBalance = errors.New("insufficient balance for pool admission")

// ErrPoolFull is returned when the pool is already at max_users capacity.
var ErrPoolFull = errors.New("pool is at capacity")

// ErrPoolSeatRequired is returned when a pool action requires an active seat first.
var ErrPoolSeatRequired = errors.New("active pool seat is required")

// ErrPoolForbidden is returned when the current user is not allowed to manage a pool.
var ErrPoolForbidden = errors.New("no permission for shared pool")

// ErrSharedPoolConcurrentUpdate prevents an owner save from overwriting a
// governance, probe, account, or model change made after the edit snapshot.
var ErrSharedPoolConcurrentUpdate = infraerrors.Conflict(
	"SHARED_POOL_CONCURRENT_UPDATE",
	"池子设置刚被其他操作更新，请刷新页面后再保存",
)

var ErrSharedPoolGovernanceBlocked = infraerrors.Forbidden(
	"SHARED_POOL_GOVERNANCE_BLOCKED",
	"该池当前受平台治理限制，不能由池主自行上架",
)

var ErrSharedPoolProbeRequired = infraerrors.Conflict(
	"SHARED_POOL_PROBE_REQUIRED",
	"请在经营控制台运行满血检测；检测达标后会自动上架",
)

func IsSharedPoolAPIKey(apiKey *APIKey) bool {
	return apiKey != nil && (apiKey.SharedPoolManaged || strings.HasPrefix(strings.TrimSpace(apiKey.Key), "sk-share-"))
}

type BizDecipherSettingRepository interface {
	GetValue(ctx context.Context, key string) (string, error)
}

type BizDecipherRepository interface {
	EnsureProfile(ctx context.Context, userID int64) (*BizProfile, error)
	UpsertContributor(ctx context.Context, userID int64, input BizContributorApplication) (*BizContributorProfile, error)
	UpsertOperator(ctx context.Context, userID int64, input BizOperatorApplication) (*BizOperatorProfile, error)
	CreateCustomRequest(ctx context.Context, userID int64, input BizCustomRequestInput) (*BizCustomRequest, error)
	ListCreditLedger(ctx context.Context, userID int64, limit int) ([]BizCreditLedgerEntry, error)
	ListContributors(ctx context.Context, limit int) ([]BizContributorProfile, error)
	ListOperators(ctx context.Context, limit int) ([]BizOperatorProfile, error)
	ListCustomRequests(ctx context.Context, limit int) ([]BizCustomRequest, error)
	ListAllCreditLedger(ctx context.Context, limit int) ([]BizCreditLedgerEntry, error)
	GrantCredit(ctx context.Context, input BizCreditGrantInput) (*BizCreditLedgerEntry, error)
	EnsureStarterCreditLedger(ctx context.Context, input BizStarterCreditInput) (*BizCreditLedgerEntry, bool, error)
	GetPoolStatus(ctx context.Context) (*BizPoolStatus, error)
	// GetPlatformStatus aggregates gateway-level health (channels & upstream
	// accounts), separate from the marketplace pool status.
	GetPlatformStatus(ctx context.Context) (*PlatformStatus, error)

	// Invite reward: check if inviter has already received reward for this invitee and kind
	HasInviteReward(ctx context.Context, inviterID, inviteeUserID int64, rewardKind string) (bool, error)
	// Invite reward: count direct rewards already granted to choose tier
	CountInviteRewards(ctx context.Context, inviterID int64, rewardKind string) (int, error)
	// Invite reward: grant reward (credit balance + ledger) and bump counter atomically
	GrantInviteRewardTx(ctx context.Context, input InviteRewardInput) (*BizCreditLedgerEntry, error)

	// Promo campaigns
	// GetActivePromoCampaign returns the single enabled campaign whose [start_at, end_at]
	// window contains NOW(), or nil if none.
	GetActivePromoCampaign(ctx context.Context) (*PromoCampaign, error)
	// HasPromoClaim reports whether the user already claimed the given campaign.
	HasPromoClaim(ctx context.Context, promoID, userID int64) (bool, error)
	// ClaimPromoTx atomically: validates the campaign is active, that the user has not
	// already claimed it, credits users.credit_balance, writes a credit_ledger entry
	// (source_type='promo_bonus'), and records the claim. Idempotent per (promo, user).
	ClaimPromoTx(ctx context.Context, promoID, userID int64) (*PromoClaimResult, error)
	ListPromoCampaigns(ctx context.Context, limit int) ([]PromoCampaign, error)
	CreatePromoCampaign(ctx context.Context, input PromoCampaignInput) (*PromoCampaign, error)
	UpdatePromoCampaign(ctx context.Context, id int64, input PromoCampaignInput) (*PromoCampaign, error)

	// Shared pools (Account Square marketplace, read-only phase)
	// ListSharedPools returns listed pools (optionally filtered) with their models.
	ListSharedPools(ctx context.Context, filter SharedPoolFilter) ([]SharedPool, error)
	// GetSharedPool returns a single listed pool by id, or nil if not found.
	GetSharedPool(ctx context.Context, id int64) (*SharedPool, error)
	ListSharedPoolProbeHistories(ctx context.Context, poolID, accountID, ownerID int64, limit int) ([]SharedPoolProbeHistory, error)
	RecordSharedPoolProbeHistory(ctx context.Context, input SharedPoolProbeHistoryInput) error
	ListSharedPoolProbeCandidates(ctx context.Context, limit int) ([]SharedPoolProbeCandidate, error)
	ApplySharedPoolProbeResult(ctx context.Context, input SharedPoolProbeHistoryInput) error
	RunSharedPoolProbeAggregation(ctx context.Context, now time.Time, limit int) (*SharedPoolProbeAggregationSummary, error)
	ListSharedPoolGovernanceLogs(ctx context.Context, poolID int64, limit int) ([]SharedPoolGovernanceLog, error)
	GetSharedPoolGovernanceSummary(ctx context.Context, now time.Time) (*SharedPoolGovernanceSummary, error)
	// UpdateSharedPoolGovernanceTx lets admins control marketplace ranking, fees, and governance state.
	UpdateSharedPoolGovernanceTx(ctx context.Context, poolID int64, adminUserID int64, input SharedPoolGovernanceInput) (*SharedPool, error)

	// Pool seats (money-sensitive: join / leave / hourly billing / owner payout)
	// JoinSharedPoolTx atomically validates admission (listed, online, capacity,
	// min balance), creates an active seat snapshotting the pool's hourly fee, and
	// bumps current_users. Idempotent: re-joining returns the existing active seat.
	JoinSharedPoolTx(ctx context.Context, poolID, userID int64) (*JoinPoolResult, error)
	// LeaveSharedPoolTx settles any whole overdue hours, marks the seat released,
	// and decrements current_users. Idempotent: leaving twice is a no-op.
	LeaveSharedPoolTx(ctx context.Context, poolID, userID int64) error
	// ListMySeats returns the user's seats (active first, newest first).
	ListMySeats(ctx context.Context, userID int64) ([]PoolSeat, error)
	// ListSharedPoolMembers returns active, released, or all seats in an owned
	// pool. The repository must enforce shared_pools.owner_id == ownerID.
	ListSharedPoolMembers(ctx context.Context, poolID, ownerID int64, status string) ([]PoolSeat, error)
	// RemoveSharedPoolMemberTx settles and releases a member seat from an owned pool.
	RemoveSharedPoolMemberTx(ctx context.Context, poolID, seatID, ownerID int64) error
	// ChargeDueSeats charges every active seat for each whole elapsed hour since
	// last_charged_at: debits the holder (pool_seat_fee) and credits the owner
	// (pool_owner_payout), idempotent per (seat, billing hour).
	ChargeDueSeats(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error)
	// ReleaseIdleSharedPoolSeats releases active seats with no real shared-pool
	// API activity for SharedPoolSeatIdleTimeout. It settles whole overdue hours
	// first (same leave path) and is idempotent per seat.
	ReleaseIdleSharedPoolSeats(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error)
	// GrantSharedPoolStabilityRewards credits non-withdrawable daily stability
	// incentives for listed owner pools (previous complete UTC day). It must only touch users.credit_balance.
	GrantSharedPoolStabilityRewards(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error)

	// Shared pool owner/accounting views. New owner earnings are held in the
	// dedicated owner wallet; stability listing rewards remain users.credit_balance.
	ListMySharedPoolLedger(ctx context.Context, ownerID int64, limit int) (*SharedPoolLedgerView, error)
	TransferSharedPoolOwnerEarningsTx(ctx context.Context, ownerID int64, amount float64, operationID string) (*SharedPoolOwnerWalletTransferResult, error)

	// Shared pool access-key phase.
	ListModelCatalog(ctx context.Context) ([]ModelCatalogEntry, error)
	UpdateModelCatalogProfile(ctx context.Context, id int64, input ModelCatalogProfileUpdate) (*ModelCatalogEntry, error)
	GetSharedPoolUpstreamRuntime(ctx context.Context, poolID, ownerID int64) (*SharedPoolUpstreamRuntime, error)
	GetSharedPoolAccountUpstreamRuntime(ctx context.Context, poolID, accountID, ownerID int64) (*SharedPoolUpstreamRuntime, error)
	ApplySharedPoolAccountProbeResult(ctx context.Context, input SharedPoolProbeHistoryInput) error
	CreateSharedPoolTx(ctx context.Context, input CreateSharedPoolInput) (*SharedPool, error)
	GetOwnedSharedPool(ctx context.Context, poolID, ownerID int64) (*SharedPool, error)
	UpdateSharedPoolTx(ctx context.Context, poolID, ownerID int64, input UpdateSharedPoolInput) (*SharedPool, error)
	ListMySharedPools(ctx context.Context, ownerID int64) ([]SharedPool, error)
	DeleteSharedPoolTx(ctx context.Context, poolID, ownerID int64) error
	RestoreSharedPoolTx(ctx context.Context, poolID, actorID int64, reason, operationID string) (*SharedPool, error)
	ListSharedPoolAccounts(ctx context.Context, poolID, ownerID int64) ([]SharedPoolAccount, error)
	CreateSharedPoolAccount(ctx context.Context, input SharedPoolAccountInput) (*SharedPoolAccount, error)
	UpdateSharedPoolAccount(ctx context.Context, accountID int64, input SharedPoolAccountInput) (*SharedPoolAccount, error)
	PersistNativeReadiness(ctx context.Context, input SharedPoolNativeReadinessInput, update NativeReadinessUpdate) error
	FindSharedPoolAccountByFingerprint(ctx context.Context, poolID, ownerID int64, fingerprint string) (*SharedPoolAccount, error)
	DeleteSharedPoolAccount(ctx context.Context, poolID, accountID, ownerID int64) error
	CreateSharedPoolAccessKeyTx(ctx context.Context, poolID, userID int64, name, rawKey string) (*SharedPoolAccessKeyResult, error)
	ListMySharedPoolAccessKeys(ctx context.Context, userID int64) ([]SharedPoolAccessKey, error)
	GetSharedPoolAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel string) (*SharedPoolAccessKey, error)
	GetSharedPoolCompactAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel string) (*SharedPoolAccessKey, error)
	GetSharedPoolMediaAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel, endpointType string) (*SharedPoolAccessKey, error)
	ResolveSharedPoolCanonicalIdentity(ctx context.Context, query SharedPoolIdentityQuery) (*SharedPoolCanonicalIdentity, error)
	RecordSharedPoolUsageTx(ctx context.Context, input SharedPoolUsageInput) error
	ReportSharedPoolTx(ctx context.Context, poolID, userID int64, reason string) (*SharedPool, error)
	LikeSharedPoolTx(ctx context.Context, poolID, userID int64) (*SharedPool, error)
	UnlikeSharedPoolTx(ctx context.Context, poolID, userID int64) (*SharedPool, error)
	ListSharedPoolLikedIDs(ctx context.Context, userID int64, poolIDs []int64) (map[int64]bool, error)
	GetUserCollectibleCardRarity(ctx context.Context, userID int64, cardKey string) (string, error)
	SetPoolCardSkinTx(ctx context.Context, poolID, ownerID int64, cardKey, cardRarity string) error

	ListCommunityPosts(ctx context.Context, query CommunityPostQuery) ([]CommunityPost, error)
	ListSharedPoolCommunitySummaries(ctx context.Context, poolIDs []int64, limit int) ([]SharedPoolCommunitySummary, error)
	ListPublicCommunityPostsByUser(ctx context.Context, userID int64, limit int) ([]CommunityPost, error)
	ListPublicSharedPoolsByOwner(ctx context.Context, ownerID int64, limit int) ([]SharedPool, error)
	GetProfileByHandleOrID(ctx context.Context, target string) (*BizProfile, error)
	UpdateProfileBackgroundCard(ctx context.Context, userID int64, input ZeroCityProfileBackgroundInput) (*BizProfile, error)
	GetZeroCityProfileStats(ctx context.Context, userID int64) (*ZeroCityProfileStats, error)
	GetZeroCityFollowState(ctx context.Context, viewerID, targetID int64) (*ZeroCityFollowState, error)
	FollowZeroCityProfileTx(ctx context.Context, followerID, followingID int64) (*ZeroCityFollowState, error)
	UnfollowZeroCityProfileTx(ctx context.Context, followerID, followingID int64) (*ZeroCityFollowState, error)
	ListMyCommunityPosts(ctx context.Context, userID int64, kind string, limit int) ([]CommunityPost, error)
	AdminListCommunityPosts(ctx context.Context, query CommunityPostQuery) ([]CommunityPost, error)
	CreateCommunityPost(ctx context.Context, userID int64, input CommunityPostInput) (*CommunityPost, error)
	UpdateCommunityPostStatus(ctx context.Context, postID int64, status string) (*CommunityPost, error)
	UpdateCommunityPostModeration(ctx context.Context, postID int64, status string, pinned *bool) (*CommunityPost, error)
	UpdateOwnedCommunityPostStatus(ctx context.Context, postID, userID int64, status string) (*CommunityPost, error)
	ListCommunityComments(ctx context.Context, postID int64, limit int) ([]CommunityComment, error)
	CreateCommunityComment(ctx context.Context, postID, userID int64, input CommunityCommentInput) (*CommunityComment, error)
	AcceptCommunityCommentTx(ctx context.Context, postID, commentID, ownerID int64) (*CommunityPost, error)
	UpdateCommunityCommentStatus(ctx context.Context, commentID int64, status string, official bool) (*CommunityComment, error)
	GetTokenPowerLeaderboard(ctx context.Context, now time.Time, limit int) (*TokenPowerLeaderboard, error)
	ListCapabilityAssets(ctx context.Context, query CapabilityAssetQuery) ([]CapabilityAsset, error)
	GetCapabilityAsset(ctx context.Context, id int64, includeUnlisted bool) (*CapabilityAsset, error)
	RecordCapabilityAssetView(ctx context.Context, assetID, userID int64) (int64, error)
	GetCapabilityAssetViewerState(ctx context.Context, assetID, userID int64) (*CapabilityAssetViewerState, error)
	SetCapabilityAssetLike(ctx context.Context, assetID, userID int64, liked bool) (int64, error)
	SetCapabilityAssetFavorite(ctx context.Context, assetID, userID int64, favorited bool) (int64, error)
	RecordCapabilityAssetUse(ctx context.Context, assetID, userID int64, eventType, sourceKind, sourceID string) error
	GetCapabilityAssetStats(ctx context.Context, ownerUserID int64) ([]CapabilityAssetStats, error)
	ListMyCapabilityAssets(ctx context.Context, userID int64, status string, limit int) ([]CapabilityAsset, error)
	CreateCapabilityAsset(ctx context.Context, userID int64, slugBase string, input CapabilityAssetInput) (*CapabilityAsset, error)
	UpdateCapabilityAsset(ctx context.Context, assetID, userID int64, input CapabilityAssetInput) (*CapabilityAsset, error)
	ListCapabilityAssetVersions(ctx context.Context, assetID int64) ([]CapabilityAssetVersion, error)
	CreateCapabilityAssetVersion(ctx context.Context, assetID, ownerUserID int64, input CreateCapabilityAssetVersionInput) (*CapabilityAssetVersion, error)
	ImportCapabilityAssetPackageTx(ctx context.Context, assetID, ownerUserID int64, input CreateCapabilityAssetVersionInput, files []PreparedCapabilityAssetFile, finalize bool) (*CapabilityAssetVersion, error)
	PutCapabilityAssetFile(ctx context.Context, assetID, ownerUserID int64, version string, input PreparedCapabilityAssetFile) (*CapabilityAssetVersion, error)
	FinalizeCapabilityAssetVersion(ctx context.Context, assetID, ownerUserID int64, version string) (*CapabilityAssetVersion, error)
	SetCapabilityAssetVersionStatus(ctx context.Context, assetID, ownerUserID int64, version, status string) (*CapabilityAssetVersion, error)
	GetCapabilityAssetPackage(ctx context.Context, assetID int64, version string) (*CapabilityAssetPackage, error)
	RecordCapabilityAssetDownload(ctx context.Context, versionID, userID int64) error
	AdminListCapabilityAssets(ctx context.Context, query CapabilityAssetQuery) ([]CapabilityAsset, error)
	AdminReviewCapabilityAsset(ctx context.Context, assetID, reviewerID int64, input CapabilityAssetReviewInput) (*CapabilityAsset, error)
	ListTavernScripts(ctx context.Context, query TavernScriptQuery) ([]TavernScript, error)
	GetTavernScript(ctx context.Context, id int64, includeUnlisted bool) (*TavernScript, error)
	ListTavernGamePackages(ctx context.Context, scriptID int64, includeUnpublished bool) ([]TavernGamePackage, error)
	GetTavernGamePackage(ctx context.Context, packageID int64) (*TavernGamePackage, error)
	GetLatestPublishedTavernGamePackage(ctx context.Context, scriptID int64) (*TavernGamePackage, error)
	CreateTavernGamePackage(ctx context.Context, scriptID, ownerUserID int64, version string, manifest TavernGamePackageManifest) (*TavernGamePackage, error)
	SetTavernGamePackageStatus(ctx context.Context, scriptID, ownerUserID int64, version, status string) (*TavernGamePackage, error)
	ListMyTavernScripts(ctx context.Context, userID int64, status string, limit int) ([]TavernScript, error)
	CreateTavernScript(ctx context.Context, userID int64, slugBase string, input TavernScriptInput) (*TavernScript, error)
	AdminListTavernScripts(ctx context.Context, query TavernScriptQuery) ([]TavernScript, error)
	AdminReviewTavernScript(ctx context.Context, scriptID, reviewerID int64, input TavernScriptReviewInput) (*TavernScript, error)
	CreateTavernRoom(ctx context.Context, ownerID int64, input TavernRoomInput) (*TavernRoom, error)
	GetTavernRoom(ctx context.Context, id int64) (*TavernRoom, error)
	ListMyTavernRooms(ctx context.Context, query TavernRoomQuery) ([]TavernRoom, error)
	IsTavernRoomParticipant(ctx context.Context, roomID, userID int64) (bool, error)
	CreateTavernRuntimeSession(ctx context.Context, input TavernRuntimeSessionInput) (*TavernRuntimeSession, error)
	GetTavernRuntimeSessionByTokenHash(ctx context.Context, tokenHash string) (*TavernRuntimeSession, error)
	TouchTavernRuntimeSession(ctx context.Context, sessionID int64) error
	OpenTavernRoom(ctx context.Context, roomID, ownerID int64) (*TavernRoom, error)
	JoinTavernRoomTx(ctx context.Context, roomID, userID int64) (*TavernRoom, error)
	StartTavernRoom(ctx context.Context, roomID, ownerID int64) (*TavernRoom, error)
	CompleteTavernRoom(ctx context.Context, roomID, ownerID int64) (*TavernRoom, error)
	CancelTavernRoom(ctx context.Context, roomID, ownerID int64) (*TavernRoom, error)
}

const (
	defaultSharedPoolDisplayCacheTTL       = 30 * time.Second
	defaultSharedPoolCacheOperationTimeout = 2 * time.Second
	sharedPoolFullProbeTimeout             = 150 * time.Second
	sharedPoolModelsProbeTimeout           = 8 * time.Second
	sharedPoolChatProbeTimeout             = 20 * time.Second
	sharedPoolFullProbeConcurrency         = 3
	sharedPoolProbeWriteTimeout            = 5 * time.Second
)

var ErrSharedPoolDisplayCacheMiss = errors.New("shared pool display cache miss")

type SharedPoolDisplayCache interface {
	GetSharedPoolList(ctx context.Context, filter SharedPoolFilter) (*SharedPoolListView, error)
	SetSharedPoolList(ctx context.Context, filter SharedPoolFilter, view *SharedPoolListView, ttl time.Duration) error
	GetSharedPool(ctx context.Context, id int64) (*SharedPool, error)
	SetSharedPool(ctx context.Context, pool *SharedPool, ttl time.Duration) error
	FlushSharedPoolDisplay(ctx context.Context) error
}

type BizDecipherService struct {
	repo                       BizDecipherRepository
	apiKeyService              *APIKeyService
	settingRepo                BizDecipherSettingRepository
	secretEncryptor            SecretEncryptor
	sharedPoolCache            SharedPoolDisplayCache
	billingService             *BillingService
	nativeOnboardingRepo       SharedPoolNativeOnboardingRepository
	nativeAccountRepo          SharedPoolNativeAccountRepository
	nativeReadinessRepo        SharedPoolNativeReadinessRepository
	nativeModelDiscoverer      SharedPoolNativeModelDiscoverer
	nativeConnectionDiagnostic SharedPoolNativeConnectionDiagnostic
}

func NewBizDecipherService(repo BizDecipherRepository, apiKeyService *APIKeyService, settingRepo BizDecipherSettingRepository, sharedPoolCache ...SharedPoolDisplayCache) *BizDecipherService {
	var cache SharedPoolDisplayCache
	if len(sharedPoolCache) > 0 {
		cache = sharedPoolCache[0]
	}
	return &BizDecipherService{repo: repo, apiKeyService: apiKeyService, settingRepo: settingRepo, sharedPoolCache: cache}
}

func (s *BizDecipherService) SetSecretEncryptor(encryptor SecretEncryptor) {
	if s != nil {
		s.secretEncryptor = encryptor
	}
}

// SetBillingService attaches the single canonical pricing source shared by the
// gateway runtime and the Account Square UI. Tests and lightweight callers may
// omit it; official prices then fail closed and remain hidden.
func (s *BizDecipherService) SetBillingService(billingService *BillingService) {
	if s != nil {
		s.billingService = billingService
	}
}

func (s *BizDecipherService) GetProfile(ctx context.Context, userID int64) (*BizProfile, error) {
	return s.repo.EnsureProfile(ctx, userID)
}

type zeroCityProfileIdentityRepository interface {
	UpdateProfileIdentity(ctx context.Context, userID int64, displayName, avatarURL string) (*BizProfile, error)
}

func (s *BizDecipherService) UpdateZeroCityProfileIdentity(ctx context.Context, userID int64, input ZeroCityProfileIdentityInput) (*BizProfile, error) {
	if s == nil || s.repo == nil || userID <= 0 {
		return nil, errors.New("profile service unavailable")
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.AvatarURL = strings.TrimSpace(input.AvatarURL)
	if input.DisplayName == "" {
		return nil, errors.New("display name is required")
	}
	if len([]rune(input.DisplayName)) > 120 {
		return nil, errors.New("display name is too long")
	}
	if len(input.AvatarURL) > 2048 {
		return nil, errors.New("avatar url is too long")
	}
	if input.AvatarURL != "" {
		parsed, err := url.Parse(input.AvatarURL)
		if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || strings.TrimSpace(parsed.Host) == "" {
			return nil, errors.New("avatar url must use http or https")
		}
	}
	identityRepo, ok := s.repo.(zeroCityProfileIdentityRepository)
	if !ok {
		return nil, errors.New("profile identity update unavailable")
	}
	if _, err := s.repo.EnsureProfile(ctx, userID); err != nil {
		return nil, err
	}
	return identityRepo.UpdateProfileIdentity(ctx, userID, input.DisplayName, input.AvatarURL)
}

func (s *BizDecipherService) GetZeroCityPublicProfile(ctx context.Context, viewerID int64, target string) (*ZeroCityPublicProfile, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, errors.New("profile target is required")
	}
	profile, err := s.repo.GetProfileByHandleOrID(ctx, target)
	if err != nil {
		return nil, err
	}
	if profile.ProfileVisibility == "private" && viewerID != profile.UserID {
		return nil, sql.ErrNoRows
	}
	stats, err := s.repo.GetZeroCityProfileStats(ctx, profile.UserID)
	if err != nil {
		return nil, err
	}
	followState, err := s.repo.GetZeroCityFollowState(ctx, viewerID, profile.UserID)
	if err != nil {
		return nil, err
	}
	pools, err := s.repo.ListPublicSharedPoolsByOwner(ctx, profile.UserID, 8)
	if err != nil {
		return nil, err
	}
	s.enrichSharedPoolOfficialPricing(pools)
	posts, err := s.repo.ListPublicCommunityPostsByUser(ctx, profile.UserID, 12)
	if err != nil {
		return nil, err
	}
	return &ZeroCityPublicProfile{
		Profile:     *profile,
		Stats:       valueOrZeroProfileStats(stats),
		FollowState: valueOrZeroFollowState(followState),
		SharedPools: pools,
		Posts:       posts,
	}, nil
}

func (s *BizDecipherService) UpdateZeroCityProfileBackground(ctx context.Context, userID int64, input ZeroCityProfileBackgroundInput) (*BizProfile, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	input.CardKey = strings.TrimSpace(input.CardKey)
	if len([]rune(input.CardKey)) > 80 {
		return nil, errors.New("card key is too long")
	}
	if _, err := s.repo.EnsureProfile(ctx, userID); err != nil {
		return nil, err
	}
	return s.repo.UpdateProfileBackgroundCard(ctx, userID, input)
}

func (s *BizDecipherService) FollowZeroCityProfile(ctx context.Context, followerID, followingID int64) (*ZeroCityFollowState, error) {
	if followerID <= 0 || followingID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if followerID == followingID {
		return nil, errors.New("cannot follow yourself")
	}
	return s.repo.FollowZeroCityProfileTx(ctx, followerID, followingID)
}

func (s *BizDecipherService) UnfollowZeroCityProfile(ctx context.Context, followerID, followingID int64) (*ZeroCityFollowState, error) {
	if followerID <= 0 || followingID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if followerID == followingID {
		return nil, errors.New("cannot unfollow yourself")
	}
	return s.repo.UnfollowZeroCityProfileTx(ctx, followerID, followingID)
}

func valueOrZeroProfileStats(stats *ZeroCityProfileStats) ZeroCityProfileStats {
	if stats == nil {
		return ZeroCityProfileStats{}
	}
	return *stats
}

func valueOrZeroFollowState(state *ZeroCityFollowState) ZeroCityFollowState {
	if state == nil {
		return ZeroCityFollowState{}
	}
	return *state
}

func (s *BizDecipherService) ApplyContributor(ctx context.Context, userID int64, input BizContributorApplication) (*BizContributorProfile, error) {
	_, err := s.repo.EnsureProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.UpsertContributor(ctx, userID, input)
}

func (s *BizDecipherService) ApplyOperator(ctx context.Context, userID int64, input BizOperatorApplication) (*BizOperatorProfile, error) {
	_, err := s.repo.EnsureProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.UpsertOperator(ctx, userID, input)
}

func (s *BizDecipherService) CreateCustomRequest(ctx context.Context, userID int64, input BizCustomRequestInput) (*BizCustomRequest, error) {
	_, err := s.repo.EnsureProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateCustomRequest(ctx, userID, input)
}

func (s *BizDecipherService) ListCommunityPosts(ctx context.Context, query CommunityPostQuery) ([]CommunityPost, error) {
	query = normalizeCommunityPostQuery(query, false)
	return s.repo.ListCommunityPosts(ctx, query)
}

func (s *BizDecipherService) ListSharedPoolCommunitySummaries(ctx context.Context, poolIDs []int64, limit int) ([]SharedPoolCommunitySummary, error) {
	cleaned := make([]int64, 0, len(poolIDs))
	seen := map[int64]bool{}
	for _, id := range poolIDs {
		if id <= 0 || seen[id] {
			continue
		}
		cleaned = append(cleaned, id)
		seen[id] = true
		if len(cleaned) >= 100 {
			break
		}
	}
	if len(cleaned) == 0 {
		return []SharedPoolCommunitySummary{}, nil
	}
	return s.repo.ListSharedPoolCommunitySummaries(ctx, cleaned, clampCommunitySummaryLimit(limit))
}

func (s *BizDecipherService) ListMyCommunityPosts(ctx context.Context, userID int64, kind string, limit int) ([]CommunityPost, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	return s.repo.ListMyCommunityPosts(ctx, userID, strings.TrimSpace(kind), limit)
}

func (s *BizDecipherService) AdminListCommunityPosts(ctx context.Context, query CommunityPostQuery) ([]CommunityPost, error) {
	query = normalizeCommunityPostQuery(query, true)
	return s.repo.AdminListCommunityPosts(ctx, query)
}

func (s *BizDecipherService) UpdateCommunityPostStatus(ctx context.Context, postID int64, status string) (*CommunityPost, error) {
	if postID <= 0 {
		return nil, errors.New("invalid post id")
	}
	status = normalizeCommunityStatus(status)
	if status == "" {
		return nil, errors.New("invalid post status")
	}
	return s.repo.UpdateCommunityPostStatus(ctx, postID, status)
}

func (s *BizDecipherService) UpdateCommunityPostModeration(ctx context.Context, postID int64, status string, pinned *bool) (*CommunityPost, error) {
	if postID <= 0 {
		return nil, errors.New("invalid post id")
	}
	status = normalizeCommunityStatus(status)
	if status == "" {
		return nil, errors.New("invalid post status")
	}
	return s.repo.UpdateCommunityPostModeration(ctx, postID, status, pinned)
}

func (s *BizDecipherService) UpdateOwnedCommunityPostStatus(ctx context.Context, postID, userID int64, status string) (*CommunityPost, error) {
	if postID <= 0 || userID <= 0 {
		return nil, errors.New("invalid post or user id")
	}
	status = normalizeCommunityOwnerStatus(status)
	if status == "" {
		return nil, errors.New("invalid owner post status")
	}
	return s.repo.UpdateOwnedCommunityPostStatus(ctx, postID, userID, status)
}

func (s *BizDecipherService) AcceptCommunityComment(ctx context.Context, postID, commentID, ownerID int64) (*CommunityPost, error) {
	if postID <= 0 || commentID <= 0 || ownerID <= 0 {
		return nil, errors.New("invalid post, comment, or owner id")
	}
	return s.repo.AcceptCommunityCommentTx(ctx, postID, commentID, ownerID)
}

func (s *BizDecipherService) UpdateCommunityCommentStatus(ctx context.Context, commentID int64, status string, official bool) (*CommunityComment, error) {
	if commentID <= 0 {
		return nil, errors.New("invalid comment id")
	}
	status = normalizeCommunityCommentStatus(status)
	if status == "" {
		return nil, errors.New("invalid comment status")
	}
	return s.repo.UpdateCommunityCommentStatus(ctx, commentID, status, official)
}

func (s *BizDecipherService) CreateCommunityPost(ctx context.Context, userID int64, input CommunityPostInput) (*CommunityPost, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	district, channel, locationErr := normalizeCommunityLocation(input.District, input.Channel)
	if locationErr != nil {
		return nil, locationErr
	}
	input.District, input.Channel = district, channel
	if err := s.requireCommunityParticipation(ctx, userID, district, channel, strings.TrimSpace(input.Kind) == "announcement" || strings.TrimSpace(input.ActionType) == "announce"); err != nil {
		return nil, err
	}
	input.Kind = normalizeCommunityKind(input.Kind)
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	input.Tags = trimStringList(input.Tags)
	input.SourceType = normalizeCommunitySourceType(input.SourceType)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.SubjectType = normalizeCommunitySubjectType(input.SubjectType)
	input.SubjectID = strings.TrimSpace(input.SubjectID)
	input.SubjectTitle = strings.TrimSpace(input.SubjectTitle)
	input.Scenario = normalizeCommunityScenario(input.Scenario, input.Kind, input.SourceType, input.SubjectType)
	input.ActionType = normalizeCommunityActionType(input.ActionType, input.Kind, input.Scenario)
	if len(input.Evidence) == 0 || !json.Valid(input.Evidence) {
		input.Evidence = json.RawMessage(`[]`)
	}
	// Trust markers are server-owned, never evidence supplied by the author.
	input.TrustSignals = json.RawMessage(`{}`)
	if input.SourceType != "" && input.SubjectType == "" {
		input.SubjectType = input.SourceType
		input.SubjectID = input.SourceID
	}
	if input.Title == "" || input.Body == "" {
		return nil, errors.New("title and body are required")
	}
	if len([]rune(input.Title)) > 180 {
		return nil, errors.New("title is too long")
	}
	if len([]rune(input.Body)) > 8000 {
		return nil, errors.New("body is too long")
	}
	if input.SourceType != "" {
		if input.SourceID == "" {
			return nil, errors.New("source id is required")
		}
		if err := s.validateCommunitySubject(ctx, input.SourceType, input.SourceID); err != nil {
			return nil, err
		}
	}
	if input.SubjectType != "" {
		if input.SubjectID == "" {
			return nil, errors.New("subject id is required")
		}
		if err := s.validateCommunitySubject(ctx, input.SubjectType, input.SubjectID); err != nil {
			return nil, err
		}
	}
	_, err := s.repo.EnsureProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateCommunityPost(ctx, userID, input)
}

func (s *BizDecipherService) ListCommunityComments(ctx context.Context, postID int64, limit int) ([]CommunityComment, error) {
	if postID <= 0 {
		return nil, errors.New("invalid post id")
	}
	return s.repo.ListCommunityComments(ctx, postID, limit)
}

func (s *BizDecipherService) CreateCommunityComment(ctx context.Context, postID, userID int64, input CommunityCommentInput) (*CommunityComment, error) {
	if postID <= 0 || userID <= 0 {
		return nil, errors.New("invalid post or user id")
	}
	input.Body = strings.TrimSpace(input.Body)
	input.HelperRole = "resident"
	if input.Body == "" {
		return nil, errors.New("comment body is required")
	}
	if len([]rune(input.Body)) > 3000 {
		return nil, errors.New("comment is too long")
	}
	participation, err := s.participationRepository()
	if err != nil {
		return nil, err
	}
	district, channel, err := participation.GetCommunityPostLocation(ctx, postID)
	if err != nil {
		return nil, err
	}
	district, channel, err = normalizeCommunityLocation(district, channel)
	if err != nil {
		return nil, err
	}
	// Newcomers can discuss proposals, but formal votes still require L1.
	if district == "governance" && channel == "votes" {
		district, channel = "tavern", "chat-hall"
	}
	// Residents can discuss rules; publishing official rules remains staff-only.
	if channel == "rules" {
		channel = "votes"
	}
	if err := s.requireCommunityParticipation(ctx, userID, district, channel, false); err != nil {
		return nil, err
	}
	_, err = s.repo.EnsureProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateCommunityComment(ctx, postID, userID, input)
}

func (s *BizDecipherService) GetTokenPowerLeaderboard(ctx context.Context, now time.Time, limit int) (*TokenPowerLeaderboard, error) {
	return s.repo.GetTokenPowerLeaderboard(ctx, now, limit)
}

func normalizeCommunityKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "token", "support", "pool", "feedback", "announcement", "card":
		return strings.TrimSpace(kind)
	default:
		return "card"
	}
}

func normalizeCommunityKindFilter(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" || kind == "all" {
		return ""
	}
	return normalizeCommunityKind(kind)
}

func normalizeCommunityPostQuery(query CommunityPostQuery, admin bool) CommunityPostQuery {
	query.Kind = normalizeCommunityKindFilter(query.Kind)
	query.SourceType = normalizeCommunitySourceType(query.SourceType)
	query.SourceID = strings.TrimSpace(query.SourceID)
	query.SubjectType = normalizeCommunitySubjectType(query.SubjectType)
	query.SubjectID = strings.TrimSpace(query.SubjectID)
	query.Scenario = normalizeCommunityScenarioFilter(query.Scenario)
	query.ActionType = normalizeCommunityActionTypeFilter(query.ActionType)
	query.Status = normalizeCommunityStatusFilter(query.Status)
	if query.SourceType == "" && query.SubjectType != "" {
		query.SourceType = query.SubjectType
		query.SourceID = query.SubjectID
	}
	if !admin {
		query.Status = ""
		query.PrivateOnly = false
	}
	return query
}

func normalizeCommunitySourceType(sourceType string) string {
	return normalizeCommunitySubjectType(sourceType)
}

func normalizeCommunitySubjectType(subjectType string) string {
	switch strings.TrimSpace(subjectType) {
	case "shared_pool", "market_demand", "workbench_task", "capability_asset", "model", "user", "organization":
		return strings.TrimSpace(subjectType)
	default:
		return ""
	}
}

func normalizeCommunityScenarioFilter(scenario string) string {
	switch strings.TrimSpace(scenario) {
	case "resource_decision", "incident_support", "feedback_triage", "demand_match", "capability_showcase", "delivery_collaboration", "usage_intel", "announcement", "general":
		return strings.TrimSpace(scenario)
	default:
		return ""
	}
}

func normalizeCommunityScenario(scenario, kind, sourceType, subjectType string) string {
	if normalized := normalizeCommunityScenarioFilter(scenario); normalized != "" {
		return normalized
	}
	switch {
	case sourceType == "shared_pool" || subjectType == "shared_pool" || kind == "pool":
		return "resource_decision"
	case kind == "support":
		return "incident_support"
	case kind == "feedback":
		return "feedback_triage"
	case kind == "token":
		return "usage_intel"
	case kind == "announcement":
		return "announcement"
	default:
		return "general"
	}
}

func normalizeCommunityActionTypeFilter(actionType string) string {
	switch strings.TrimSpace(actionType) {
	case "discuss", "ask_help", "report", "share_signal", "recommend", "offer", "request", "accept", "deliver", "review", "announce":
		return strings.TrimSpace(actionType)
	default:
		return ""
	}
}

func normalizeCommunityActionType(actionType, kind, scenario string) string {
	if normalized := normalizeCommunityActionTypeFilter(actionType); normalized != "" {
		return normalized
	}
	switch {
	case kind == "feedback" || scenario == "feedback_triage":
		return "report"
	case kind == "support" || scenario == "incident_support":
		return "ask_help"
	case kind == "pool" || scenario == "resource_decision":
		return "share_signal"
	case scenario == "capability_showcase":
		return "offer"
	case scenario == "demand_match":
		return "request"
	case kind == "announcement" || scenario == "announcement":
		return "announce"
	default:
		return "discuss"
	}
}

func (s *BizDecipherService) validateCommunitySubject(ctx context.Context, subjectType, subjectID string) error {
	subjectType = normalizeCommunitySubjectType(subjectType)
	if subjectType == "" {
		return errors.New("unsupported subject type")
	}
	if strings.TrimSpace(subjectID) == "" {
		return errors.New("subject id is required")
	}
	switch subjectType {
	case "shared_pool":
		poolID, err := strconv.ParseInt(subjectID, 10, 64)
		if err != nil || poolID <= 0 {
			return errors.New("invalid shared pool subject id")
		}
		pool, err := s.repo.GetSharedPool(ctx, poolID)
		if err != nil {
			return err
		}
		if pool == nil {
			return sql.ErrNoRows
		}
	}
	return nil
}

func normalizeCommunityStatusFilter(status string) string {
	status = strings.TrimSpace(status)
	if status == "" || status == "all" {
		return ""
	}
	return normalizeCommunityStatus(status)
}

func normalizeCommunityStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "open", "reviewing", "answered", "confirmed", "resolved", "hidden", "deleted", "rejected":
		return strings.TrimSpace(status)
	default:
		return ""
	}
}

func normalizeCommunityOwnerStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "open", "answered", "resolved":
		return strings.TrimSpace(status)
	default:
		return ""
	}
}

func clampCommunitySummaryLimit(limit int) int {
	if limit <= 0 {
		return 3
	}
	if limit > 8 {
		return 8
	}
	return limit
}

func normalizeCommunityCommentStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "visible", "accepted", "confirmed", "resolved", "hidden", "deleted":
		return strings.TrimSpace(status)
	default:
		return ""
	}
}

func (s *BizDecipherService) ListCreditLedger(ctx context.Context, userID int64, limit int) ([]BizCreditLedgerEntry, error) {
	return s.repo.ListCreditLedger(ctx, userID, limit)
}

func (s *BizDecipherService) ListMySharedPoolLedger(ctx context.Context, ownerID int64, limit int) (*SharedPoolLedgerView, error) {
	if s == nil || s.repo == nil {
		return &SharedPoolLedgerView{
			Earnings:           []SharedPoolOwnerEarningsEntry{},
			Activity:           []SharedPoolBalanceLedgerEntry{},
			Withdrawable:       []SharedPoolBalanceLedgerEntry{},
			LegacyWithdrawable: []SharedPoolBalanceLedgerEntry{},
			Incentives:         []BizCreditLedgerEntry{},
		}, nil
	}
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	return s.repo.ListMySharedPoolLedger(ctx, ownerID, limit)
}

type sharedPoolOwnerEarningsPageRepository interface {
	ListSharedPoolOwnerEarningsPage(ctx context.Context, ownerID, poolID, beforeID int64, limit int) (*SharedPoolOwnerEarningsPage, error)
}

func (s *BizDecipherService) ListSharedPoolOwnerEarningsPage(ctx context.Context, ownerID, poolID, beforeID int64, limit int) (*SharedPoolOwnerEarningsPage, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if ownerID <= 0 || poolID < 0 || beforeID < 0 {
		return nil, errors.New("invalid earnings query")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	repo, ok := s.repo.(sharedPoolOwnerEarningsPageRepository)
	if !ok {
		return nil, errors.New("owner earnings pagination is unavailable")
	}
	return repo.ListSharedPoolOwnerEarningsPage(ctx, ownerID, poolID, beforeID, limit)
}

func (s *BizDecipherService) TransferSharedPoolOwnerEarnings(ctx context.Context, ownerID int64, amount float64, operationID string) (*SharedPoolOwnerWalletTransferResult, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return nil, errors.New("transfer amount must be greater than zero")
	}
	amount = withdrawableSharedPoolMoney(amount)
	if amount <= 0 {
		return nil, errors.New("transfer amount must be at least 0.00000001")
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return nil, errors.New("operation id is required")
	}
	if len(operationID) > 160 {
		return nil, errors.New("operation id is too long")
	}
	return s.repo.TransferSharedPoolOwnerEarningsTx(ctx, ownerID, amount, operationID)
}

func (s *BizDecipherService) GetPoolStatus(ctx context.Context) (*BizPoolStatus, error) {
	return s.repo.GetPoolStatus(ctx)
}

// GetPlatformStatus reports the health of the underlying API gateway, kept
// separate from the marketplace pool status by design.
func (s *BizDecipherService) GetPlatformStatus(ctx context.Context) (*PlatformStatus, error) {
	return s.repo.GetPlatformStatus(ctx)
}
func (s *BizDecipherService) AdminListContributors(ctx context.Context, limit int) ([]BizContributorProfile, error) {
	return s.repo.ListContributors(ctx, limit)
}
func (s *BizDecipherService) AdminListOperators(ctx context.Context, limit int) ([]BizOperatorProfile, error) {
	return s.repo.ListOperators(ctx, limit)
}
func (s *BizDecipherService) AdminListCustomRequests(ctx context.Context, limit int) ([]BizCustomRequest, error) {
	return s.repo.ListCustomRequests(ctx, limit)
}
func (s *BizDecipherService) AdminListCreditLedger(ctx context.Context, limit int) ([]BizCreditLedgerEntry, error) {
	return s.repo.ListAllCreditLedger(ctx, limit)
}
func (s *BizDecipherService) AdminGrantCredit(ctx context.Context, input BizCreditGrantInput) (*BizCreditLedgerEntry, error) {
	return s.repo.GrantCredit(ctx, input)
}

func (s *BizDecipherService) AdminListSharedPools(ctx context.Context, filter SharedPoolFilter) (*SharedPoolListView, error) {
	filter.IncludeUnlisted = true
	if strings.TrimSpace(filter.Lifecycle) == "" {
		filter.Lifecycle = "all"
	}
	view, err := s.ListSharedPools(ctx, filter)
	if err != nil {
		return nil, err
	}
	if s == nil || s.repo == nil {
		return view, nil
	}
	summary, err := s.repo.GetSharedPoolGovernanceSummary(ctx, time.Now())
	if err != nil {
		return nil, err
	}
	view.GovernanceSummary = summary
	return view, nil
}

type adminSharedPoolOwnerEarningsRepository interface {
	AdminListSharedPoolOwnerEarningsPage(context.Context, AdminSharedPoolOwnerEarningsFilter) (*AdminSharedPoolOwnerEarningsPage, error)
}

func (s *BizDecipherService) AdminListSharedPoolOwnerEarningsPage(ctx context.Context, filter AdminSharedPoolOwnerEarningsFilter) (*AdminSharedPoolOwnerEarningsPage, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	repo, ok := s.repo.(adminSharedPoolOwnerEarningsRepository)
	if !ok || repo == nil {
		return nil, errors.New("shared pool owner earnings ledger is unavailable")
	}
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}
	return repo.AdminListSharedPoolOwnerEarningsPage(ctx, filter)
}

func (s *BizDecipherService) AdminUpdateSharedPoolGovernance(ctx context.Context, poolID int64, adminUserID int64, input SharedPoolGovernanceInput) (*SharedPool, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	if pool, err := s.repo.GetSharedPool(ctx, poolID); err == nil && isNativeR1Pool(pool) && !isNativeBillingActive(pool) {
		if input.Listed != nil && *input.Listed {
			return nil, ErrBillingActivationRequired
		}
		if input.Status != nil && (*input.Status == "healthy" || *input.Status == "limited") {
			return nil, ErrBillingActivationRequired
		}
	}
	if input.PlatformFeePercent != nil {
		v := clampFloat(*input.PlatformFeePercent, 0, 100)
		input.PlatformFeePercent = &v
	}
	if input.FeaturedScore != nil {
		v := clampFloat(*input.FeaturedScore, -1000, 1000)
		input.FeaturedScore = &v
	}
	if input.RewardScore != nil {
		v := clampFloat(*input.RewardScore, 0, 1000)
		input.RewardScore = &v
	}
	if input.PenaltyScore != nil {
		v := clampFloat(*input.PenaltyScore, 0, 1000)
		input.PenaltyScore = &v
	}
	if input.GovernanceStatus != nil {
		status := strings.TrimSpace(*input.GovernanceStatus)
		switch status {
		case "normal", "boosted", "watch", "suppressed", "banned":
			input.GovernanceStatus = &status
		default:
			return nil, errors.New("invalid governance status")
		}
	}
	if input.Status != nil {
		status := strings.TrimSpace(*input.Status)
		switch status {
		case "healthy", "limited", "offline", "maintenance":
			input.Status = &status
		default:
			return nil, errors.New("invalid pool status")
		}
	}
	trimStringPtr(input.GovernanceNote)
	trimStringPtr(input.AdminNote)
	pool, err := s.repo.UpdateSharedPoolGovernanceTx(ctx, poolID, adminUserID, input)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "governance_update")
	return pool, nil
}

func (s *BizDecipherService) GrantStarterCredit(ctx context.Context, input BizStarterCreditInput) (*BizCreditLedgerEntry, bool, error) {
	return s.repo.EnsureStarterCreditLedger(ctx, input)
}

const (
	inviteRewardKindDirect    = "direct"
	inviteRewardKindIndirect  = "indirect"
	inviteRewardKindMilestone = "milestone"
	inviteRewardIndirectRate  = 0.20

	inviteRewardDefaultEnabled            = true
	inviteRewardDefaultAmount             = 30.0
	inviteRewardDefaultMilestoneThreshold = 0
	inviteRewardDefaultMilestoneAmount    = 0.0
)

type InviteRewardConfig struct {
	Enabled            bool    `json:"enabled"`
	Amount             float64 `json:"amount"`
	MilestoneThreshold int     `json:"milestone_threshold"`
	MilestoneAmount    float64 `json:"milestone_amount"`
}

func (s *BizDecipherService) GetInviteRewardConfig(ctx context.Context) InviteRewardConfig {
	cfg := InviteRewardConfig{
		Enabled:            inviteRewardDefaultEnabled,
		Amount:             inviteRewardDefaultAmount,
		MilestoneThreshold: inviteRewardDefaultMilestoneThreshold,
		MilestoneAmount:    inviteRewardDefaultMilestoneAmount,
	}
	if s == nil || s.settingRepo == nil {
		return cfg
	}
	if raw, err := s.settingRepo.GetValue(ctx, SettingKeyInviteRewardEnabled); err == nil {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "false", "0", "off", "disabled", "no":
			cfg.Enabled = false
		case "true", "1", "on", "enabled", "yes":
			cfg.Enabled = true
		}
	}
	if raw, err := s.settingRepo.GetValue(ctx, SettingKeyInviteRewardAmount); err == nil {
		if v, parseErr := strconv.ParseFloat(strings.TrimSpace(raw), 64); parseErr == nil && v >= 0 {
			cfg.Amount = v
		}
	}
	if raw, err := s.settingRepo.GetValue(ctx, SettingKeyInviteRewardMilestoneThreshold); err == nil {
		if v, parseErr := strconv.Atoi(strings.TrimSpace(raw)); parseErr == nil && v >= 0 {
			cfg.MilestoneThreshold = v
		}
	}
	if raw, err := s.settingRepo.GetValue(ctx, SettingKeyInviteRewardMilestoneAmount); err == nil {
		if v, parseErr := strconv.ParseFloat(strings.TrimSpace(raw), 64); parseErr == nil && v >= 0 {
			cfg.MilestoneAmount = v
		}
	}
	return cfg
}

// GrantInviteRewardOnce grants a one-time invite point reward to the direct inviter.
// Returns (entry, true, nil) on success, (nil, false, nil) if reward was already
// granted or conditions are not met, or (nil, false, err) on failure.
func (s *BizDecipherService) GrantInviteRewardOnce(ctx context.Context, inviterID, inviteeUserID int64) (*BizCreditLedgerEntry, bool, error) {
	if s == nil || s.repo == nil {
		return nil, false, nil
	}
	if inviterID <= 0 || inviteeUserID <= 0 || inviterID == inviteeUserID {
		return nil, false, nil
	}

	cfg := s.GetInviteRewardConfig(ctx)
	if !cfg.Enabled || cfg.Amount <= 0 {
		return nil, false, nil
	}

	has, err := s.repo.HasInviteReward(ctx, inviterID, inviteeUserID, inviteRewardKindDirect)
	if err != nil {
		logger.LegacyPrintf("service.bizdecipher", "[InviteReward] HasInviteReward check failed: inviter=%d invitee=%d kind=%s err=%v", inviterID, inviteeUserID, inviteRewardKindDirect, err)
		return nil, false, err
	}
	if has {
		return nil, false, nil
	}

	directCount := 0
	if cfg.MilestoneThreshold > 0 && cfg.MilestoneAmount > 0 {
		directCount, err = s.repo.CountInviteRewards(ctx, inviterID, inviteRewardKindDirect)
		if err != nil {
			logger.LegacyPrintf("service.bizdecipher", "[InviteReward] CountInviteRewards failed: inviter=%d kind=%s err=%v", inviterID, inviteRewardKindDirect, err)
			return nil, false, err
		}
	}
	amount := cfg.Amount

	input := InviteRewardInput{
		InviterID:     inviterID,
		InviteeUserID: inviteeUserID,
		Amount:        amount,
		RewardKind:    inviteRewardKindDirect,
	}

	entry, err := s.repo.GrantInviteRewardTx(ctx, input)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		logger.LegacyPrintf("service.bizdecipher", "[InviteReward] GrantInviteRewardTx failed: inviter=%d invitee=%d kind=%s err=%v", inviterID, inviteeUserID, inviteRewardKindDirect, err)
		return nil, false, err
	}
	if entry == nil {
		return nil, false, nil
	}

	logger.LegacyPrintf("service.bizdecipher", "[InviteReward] Granted %.0f to user %d for inviting user %d", amount, inviterID, inviteeUserID)

	// Optional single milestone bonus. Disabled by default; configured by settings.
	if cfg.MilestoneThreshold > 0 && cfg.MilestoneAmount > 0 && directCount+1 == cfg.MilestoneThreshold {
		milestoneInput := InviteRewardInput{
			InviterID:     inviterID,
			InviteeUserID: int64(cfg.MilestoneThreshold), // used as source_id uniqueness key
			Amount:        cfg.MilestoneAmount,
			RewardKind:    inviteRewardKindMilestone,
		}
		milestoneEntry, milestoneErr := s.repo.GrantInviteRewardTx(ctx, milestoneInput)
		if milestoneErr != nil && milestoneErr != sql.ErrNoRows {
			logger.LegacyPrintf("service.bizdecipher", "[InviteReward] Milestone grant failed: inviter=%d invitee=%d err=%v", inviterID, inviteeUserID, milestoneErr)
		} else if milestoneEntry != nil {
			logger.LegacyPrintf("service.bizdecipher", "[InviteReward] Milestone %.0f granted to user %d at %d invites", cfg.MilestoneAmount, inviterID, cfg.MilestoneThreshold)
		}
	}

	return entry, true, nil
}

// GrantIndirectInviteRewardOnce grants a one-time second-level growth reward.
// Example: A invited B, B invited C, C activates -> B receives direct reward,
// A receives 20% of B's direct reward. Only one upstream level is supported.
func (s *BizDecipherService) GrantIndirectInviteRewardOnce(ctx context.Context, upstreamInviterID, directInviterID, inviteeUserID int64, directAmount float64) (*BizCreditLedgerEntry, bool, error) {
	if s == nil || s.repo == nil {
		return nil, false, nil
	}
	if upstreamInviterID <= 0 || directInviterID <= 0 || inviteeUserID <= 0 || directAmount <= 0 {
		return nil, false, nil
	}
	if upstreamInviterID == directInviterID || upstreamInviterID == inviteeUserID || directInviterID == inviteeUserID {
		return nil, false, nil
	}

	has, err := s.repo.HasInviteReward(ctx, upstreamInviterID, inviteeUserID, inviteRewardKindIndirect)
	if err != nil {
		logger.LegacyPrintf("service.bizdecipher", "[InviteReward] HasInviteReward check failed: upstream=%d direct=%d invitee=%d kind=%s err=%v", upstreamInviterID, directInviterID, inviteeUserID, inviteRewardKindIndirect, err)
		return nil, false, err
	}
	if has {
		return nil, false, nil
	}

	amount := directAmount * inviteRewardIndirectRate
	input := InviteRewardInput{
		InviterID:       upstreamInviterID,
		InviteeUserID:   inviteeUserID,
		DirectInviterID: directInviterID,
		Amount:          amount,
		RewardKind:      inviteRewardKindIndirect,
	}
	entry, err := s.repo.GrantInviteRewardTx(ctx, input)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		logger.LegacyPrintf("service.bizdecipher", "[InviteReward] Grant indirect reward failed: upstream=%d direct=%d invitee=%d err=%v", upstreamInviterID, directInviterID, inviteeUserID, err)
		return nil, false, err
	}
	if entry == nil {
		return nil, false, nil
	}
	logger.LegacyPrintf("service.bizdecipher", "[InviteReward] Granted indirect %.2f to user %d via direct inviter %d and invitee %d", amount, upstreamInviterID, directInviterID, inviteeUserID)
	return entry, true, nil
}

// ---------------------------------------------------------------------------
// Promo campaigns
// ---------------------------------------------------------------------------

// ErrPromoNotActive is returned when a claim targets a campaign that is not
// currently enabled or is outside its [start_at, end_at] window.
var ErrPromoNotActive = errors.New("promo campaign is not active")

// GetActivePromo returns the currently-active campaign for a user, together
// with whether they have already claimed it and whether they may still claim.
// "Active" means: enabled = true AND now within [start_at, end_at].
func (s *BizDecipherService) GetActivePromo(ctx context.Context, userID int64) (*PromoActiveView, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	campaign, err := s.repo.GetActivePromoCampaign(ctx)
	if err != nil {
		return nil, err
	}
	if campaign == nil {
		return &PromoActiveView{Campaign: nil, Claimed: false, Claimable: false}, nil
	}
	claimed := false
	if userID > 0 {
		claimed, err = s.repo.HasPromoClaim(ctx, campaign.ID, userID)
		if err != nil {
			return nil, err
		}
	}
	view := &PromoActiveView{
		Campaign:  campaign,
		Claimed:   claimed,
		Claimable: !claimed,
	}
	return view, nil
}

// ClaimPromo grants the campaign's credit to the user exactly once.
// It is idempotent: a second call returns the existing claim with AlreadyDone=true.
func (s *BizDecipherService) ClaimPromo(ctx context.Context, promoID, userID int64) (*PromoClaimResult, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if promoID <= 0 || userID <= 0 {
		return nil, errors.New("invalid promo or user")
	}
	return s.repo.ClaimPromoTx(ctx, promoID, userID)
}

// AdminListPromos returns all campaigns for the admin console.
func (s *BizDecipherService) AdminListPromos(ctx context.Context, limit int) ([]PromoCampaign, error) {
	return s.repo.ListPromoCampaigns(ctx, limit)
}

// AdminCreatePromo creates a new campaign.
func (s *BizDecipherService) AdminCreatePromo(ctx context.Context, input PromoCampaignInput) (*PromoCampaign, error) {
	if err := validatePromoInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreatePromoCampaign(ctx, input)
}

// AdminUpdatePromo updates an existing campaign (including enabling/disabling it).
func (s *BizDecipherService) AdminUpdatePromo(ctx context.Context, id int64, input PromoCampaignInput) (*PromoCampaign, error) {
	if id <= 0 {
		return nil, errors.New("invalid promo id")
	}
	if err := validatePromoInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdatePromoCampaign(ctx, id, input)
}

func validatePromoInput(input PromoCampaignInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}
	if input.CreditAmount < 0 {
		return errors.New("credit_amount must not be negative")
	}
	if !input.EndAt.IsZero() && !input.StartAt.IsZero() && input.EndAt.Before(input.StartAt) {
		return errors.New("end_at must be after start_at")
	}
	return nil
}

type sharedPoolBrandingFields struct {
	Name           *string
	Description    *string
	AvatarURL      *string
	StatusNote     *string
	DisabledReason *string
}

func clampFloat(v, minValue, maxValue float64) float64 {
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func trimStringPtr(v *string) {
	if v == nil {
		return
	}
	*v = strings.TrimSpace(*v)
}

func trimStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func normalizeSharedPoolBranding(fields sharedPoolBrandingFields) error {
	if fields.Name == nil || fields.Description == nil || fields.AvatarURL == nil || fields.StatusNote == nil || fields.DisabledReason == nil {
		return errors.New("invalid shared pool input")
	}
	*fields.Name = strings.TrimSpace(*fields.Name)
	*fields.Description = strings.TrimSpace(*fields.Description)
	*fields.AvatarURL = strings.TrimSpace(*fields.AvatarURL)
	*fields.StatusNote = strings.TrimSpace(*fields.StatusNote)
	*fields.DisabledReason = strings.TrimSpace(*fields.DisabledReason)
	if len([]rune(*fields.Name)) > 120 {
		return errors.New("pool name is too long")
	}
	if len([]rune(*fields.Description)) > 1000 {
		return errors.New("pool description is too long")
	}
	if len([]rune(*fields.StatusNote)) > 500 {
		return errors.New("status note is too long")
	}
	if len([]rune(*fields.DisabledReason)) > 500 {
		return errors.New("disabled reason is too long")
	}
	if *fields.AvatarURL != "" {
		u, err := url.ParseRequestURI(*fields.AvatarURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("avatar_url must be a valid http(s) URL")
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Shared pools (Account Square marketplace, read-only phase)
// ---------------------------------------------------------------------------

// SharedPoolFilter narrows the marketplace listing. Empty fields mean "no filter".
// Filtering/sorting is applied in the repository (SQL) where possible; the frontend
// may further refine client-side.
type SharedPoolFilter struct {
	Keyword         string  // matches name / owner_label / model name
	Model           string  // exact model name / alias preferred by users
	Status          string  // healthy | limited | offline | maintenance
	View            string  // public | observation
	Lifecycle       string  // current | attention | archived | all (admin only)
	MinAvailability float64 // today_availability >= this
	SortBy          string  // recommended | availability | rate | latency | users | newest | weight
	IncludeUnlisted bool    // admin-only: include hidden pools
	Limit           int
}

type SharedPoolGovernanceInput struct {
	PlatformFeePercent *float64
	FeaturedScore      *float64
	RewardScore        *float64
	PenaltyScore       *float64
	GovernanceStatus   *string
	GovernanceNote     *string
	AdminNote          *string
	Listed             *bool
	Status             *string
}

type SharedPoolProbeAggregationSummary struct {
	PoolsChecked     int `json:"pools_checked"`
	PoolLevelChecked int `json:"pool_level_checked"`
	AccountsChecked  int `json:"accounts_checked"`
	Succeeded        int `json:"succeeded"`
	Failed           int `json:"failed"`
	Limited          int `json:"limited"`
	Offlined         int `json:"offlined"`
	// CandidatesFound is how many probe candidates were selected before running.
	// Zero usually means every eligible target is inside the 5-minute cooldown.
	CandidatesFound int `json:"candidates_found"`
	// SkippedCooldown is true when no candidates were runnable (cooldown / empty).
	SkippedCooldown bool   `json:"skipped_cooldown"`
	Message         string `json:"message,omitempty"`
}

func (s *BizDecipherService) ListModelCatalog(ctx context.Context) ([]ModelCatalogEntry, error) {
	if s == nil || s.repo == nil {
		return []ModelCatalogEntry{}, nil
	}
	catalog, err := s.repo.ListModelCatalog(ctx)
	if err != nil {
		return nil, err
	}
	for i := range catalog {
		// Always discard repository-provided pricing. Official catalog display
		// prices are allowed to come from BillingService only.
		catalog[i].Pricing = s.canonicalSharedPoolPriceSnapshot(catalog[i].ModelName)
	}
	return catalog, nil
}

var (
	modelCapabilityModalities = map[string]struct{}{
		"text": {}, "image": {}, "audio": {}, "video": {}, "embedding": {}, "file": {},
	}
	modelCapabilityAdapterKinds = map[string]struct{}{
		"openai_chat": {}, "openai_responses": {}, "anthropic_messages": {},
		"google_generate_content": {}, "custom_http": {}, "external_service": {},
	}
	modelCapabilityRuntimeRoles = map[string]struct{}{
		"text": {}, "image": {}, "video": {}, "audio": {}, "embedding": {}, "orchestrator": {},
	}
)

func normalizeModelCapabilityModalities(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := modelCapabilityModalities[value]; !ok {
			return nil, fmt.Errorf("unsupported modality %q", value)
		}
		if _, dup := seen[value]; dup {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

// UpdateModelCatalogProfile writes the capability matrix entry for one catalog
// model. Unknown modalities, adapter protocols or runtime roles are rejected
// rather than stored, so downstream package/runtime matching can trust the row.
func (s *BizDecipherService) UpdateModelCatalogProfile(ctx context.Context, id int64, input ModelCapabilityProfileInput) (*ModelCatalogEntry, error) {
	if s == nil || s.repo == nil || id <= 0 {
		return nil, errors.New("model catalog entry not found")
	}
	modalitiesIn, err := normalizeModelCapabilityModalities(input.ModalitiesIn)
	if err != nil {
		return nil, err
	}
	modalitiesOut, err := normalizeModelCapabilityModalities(input.ModalitiesOut)
	if err != nil {
		return nil, err
	}
	adapterKind := strings.ToLower(strings.TrimSpace(input.AdapterKind))
	if adapterKind != "" {
		if _, ok := modelCapabilityAdapterKinds[adapterKind]; !ok {
			return nil, fmt.Errorf("unsupported adapter kind %q", adapterKind)
		}
	}
	runtimeRole := strings.ToLower(strings.TrimSpace(input.RuntimeRole))
	if runtimeRole != "" {
		if _, ok := modelCapabilityRuntimeRoles[runtimeRole]; !ok {
			return nil, fmt.Errorf("unsupported runtime role %q", runtimeRole)
		}
	}
	if input.ContextWindow != nil {
		if *input.ContextWindow < 0 || *input.ContextWindow > 10_000_000 {
			return nil, fmt.Errorf("context window out of range")
		}
	}
	return s.repo.UpdateModelCatalogProfile(ctx, id, ModelCatalogProfileUpdate{
		ModalitiesIn:  modalitiesIn,
		ModalitiesOut: modalitiesOut,
		AdapterKind:   adapterKind,
		ContextWindow: input.ContextWindow,
		Orchestrator:  input.Orchestrator,
		RuntimeRole:   runtimeRole,
	})
}

func (s *BizDecipherService) FetchSharedPoolUpstreamModels(ctx context.Context, input SharedPoolUpstreamModelsInput) (*SharedPoolUpstreamModelsResult, error) {
	if s == nil {
		return nil, errors.New("service unavailable")
	}
	var runtime *SharedPoolUpstreamRuntime
	if input.PoolID > 0 {
		if s.repo == nil || input.OwnerID <= 0 {
			return nil, errors.New("service unavailable")
		}
		var err error
		if input.AccountID > 0 {
			runtime, err = s.repo.GetSharedPoolAccountUpstreamRuntime(ctx, input.PoolID, input.AccountID, input.OwnerID)
		} else {
			runtime, err = s.repo.GetSharedPoolUpstreamRuntime(ctx, input.PoolID, input.OwnerID)
		}
		if err != nil {
			return nil, err
		}
		if runtime != nil {
			input.UpstreamBaseURL = firstNonEmpty(input.UpstreamBaseURL, runtime.UpstreamBaseURL)
			input.UpstreamAPIKey = firstNonEmpty(input.UpstreamAPIKey, runtime.UpstreamAPIKey)
			input.ProxyURL = firstNonEmpty(input.ProxyURL, runtime.ProxyURL)
		}
	}
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	if runtime != nil && strings.EqualFold(strings.TrimSpace(runtime.AuthType), AccountTypeOAuth) {
		if _, err := s.sharedPoolOAuthCredentials(runtime); err != nil {
			return nil, err
		}
		model := strings.TrimSpace(runtime.ProbeModel)
		if model == "" {
			return nil, errors.New("oauth account has no configured model; choose a detection model first")
		}
		return &SharedPoolUpstreamModelsResult{Models: []string{model}, CheckedAt: checkedAt, HTTPStatus: http.StatusOK}, nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(input.UpstreamBaseURL), "/")
	apiKey := strings.TrimSpace(input.UpstreamAPIKey)
	if baseURL == "" {
		return nil, errors.New("upstream base url is required")
	}
	if apiKey == "" {
		return nil, errors.New("upstream api key is required")
	}
	validatedURL, err := validateSharedPoolUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	models, status, err := fetchSharedPoolUpstreamModelsWithStatus(ctx, validatedURL, apiKey, strings.TrimSpace(input.ProxyURL))
	if err != nil {
		probeErr := classifySharedPoolProbeError(err, status)
		if probeErr == nil {
			return nil, err
		}
		probeErr.Message = redactSharedPoolSecret(sanitizeSharedPoolProbeMessage(probeErr.Message), apiKey)
		return nil, probeErr
	}
	return &SharedPoolUpstreamModelsResult{Models: models, CheckedAt: checkedAt, HTTPStatus: status}, nil
}

func sharedPoolAccountHasRunnableCredentials(account SharedPoolAccount) bool {
	if strings.EqualFold(strings.TrimSpace(account.AuthType), AccountTypeOAuth) {
		return account.HasOAuthCredentials
	}
	return strings.TrimSpace(account.UpstreamBaseURL) != "" && account.HasUpstreamKey
}

func sharedPoolAccountExpiredForProbe(account SharedPoolAccount, now time.Time) bool {
	if account.ExpiresAt == nil || account.ExpiresAt.IsZero() {
		return false
	}
	if account.ExpiresAt.After(now) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(account.AuthType), AccountTypeOAuth) && !account.AutoPauseOnExpired {
		return false
	}
	return true
}

func sharedPoolAccountSupportsProbeModel(account SharedPoolAccount, probeModel string) bool {
	probeModel = strings.TrimSpace(probeModel)
	if probeModel == "" {
		return true
	}
	configs := account.ModelConfigs
	if len(configs) == 0 {
		return false
	}
	for _, cfg := range configs {
		if !cfg.ModelOpen {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(cfg.ModelName), probeModel) || strings.EqualFold(strings.TrimSpace(cfg.UpstreamModelName), probeModel) {
			return true
		}
	}
	return false
}

func sharedPoolAccountEligibleForProbe(account SharedPoolAccount, probeModel string, now time.Time) bool {
	if !account.Schedulable {
		return false
	}
	switch strings.TrimSpace(account.Status) {
	case "active", "limited", "testing":
	default:
		return false
	}
	if !sharedPoolAccountHasRunnableCredentials(account) {
		return false
	}
	if sharedPoolAccountExpiredForProbe(account, now) {
		return false
	}
	if !sharedPoolAccountSupportsProbeModel(account, probeModel) {
		return false
	}
	return true
}

func (s *BizDecipherService) findOwnedSharedPool(ctx context.Context, poolID, ownerID int64) (*SharedPool, error) {
	if s == nil || s.repo == nil || poolID <= 0 || ownerID <= 0 {
		return nil, nil
	}
	pools, err := s.ListMySharedPools(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	for i := range pools {
		if pools[i].ID == poolID {
			pool := pools[i]
			return &pool, nil
		}
	}
	return nil, nil
}

func (s *BizDecipherService) selectSharedPoolProbeAccount(ctx context.Context, poolID, ownerID int64, probeModel string) (*SharedPoolAccount, error) {
	accounts, err := s.ListSharedPoolAccounts(ctx, poolID, ownerID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var fallback *SharedPoolAccount
	for i := range accounts {
		account := accounts[i]
		if !sharedPoolAccountEligibleForProbe(account, probeModel, now) {
			continue
		}
		if account.LastProbeSuccess != nil && *account.LastProbeSuccess {
			picked := account
			return &picked, nil
		}
		if fallback == nil {
			picked := account
			fallback = &picked
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	if strings.TrimSpace(probeModel) != "" {
		return nil, fmt.Errorf("当前没有可检测账号：请先确保至少一个账号已保存有效凭证、未过期、可调度，并开放探针模型 %s", probeModel)
	}
	return nil, errors.New("当前没有可检测账号：请先确保至少一个账号已保存有效凭证、未过期并可调度")
}

func (s *BizDecipherService) ProbeSharedPoolUpstream(ctx context.Context, input SharedPoolUpstreamProbeInput) (*SharedPoolUpstreamProbeResult, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if strings.TrimSpace(input.ProbeType) == "" {
		input.ProbeType = "manual"
	}
	ownedPool, _ := s.findOwnedSharedPool(ctx, input.PoolID, input.OwnerID)
	if input.PoolID > 0 && input.AccountID == 0 && strings.TrimSpace(input.UpstreamAPIKey) == "" && ownedPool != nil && ownedPool.AccountModeEnabled {
		selectedAccount, err := s.selectSharedPoolProbeAccount(ctx, input.PoolID, input.OwnerID, input.ProbeModel)
		if err != nil {
			return nil, err
		}
		if selectedAccount != nil {
			input.AccountID = selectedAccount.ID
		}
	}
	var runtime *SharedPoolUpstreamRuntime
	if input.PoolID > 0 && input.AccountID > 0 {
		var err error
		runtime, err = s.repo.GetSharedPoolAccountUpstreamRuntime(ctx, input.PoolID, input.AccountID, input.OwnerID)
		if err != nil {
			return nil, err
		}
		if runtime != nil {
			input.UpstreamBaseURL = firstNonEmpty(input.UpstreamBaseURL, runtime.UpstreamBaseURL)
			input.UpstreamAPIKey = firstNonEmpty(input.UpstreamAPIKey, runtime.UpstreamAPIKey)
			input.ProxyURL = firstNonEmpty(input.ProxyURL, runtime.ProxyURL)
			input.ProbeModel = firstNonEmpty(input.ProbeModel, runtime.ProbeModel)
		}
	} else if input.PoolID > 0 && strings.TrimSpace(input.UpstreamAPIKey) == "" {
		runtime, err := s.repo.GetSharedPoolUpstreamRuntime(ctx, input.PoolID, input.OwnerID)
		if err != nil {
			return nil, err
		}
		if runtime != nil {
			input.UpstreamAPIKey = runtime.UpstreamAPIKey
			input.UpstreamBaseURL = firstNonEmpty(input.UpstreamBaseURL, runtime.UpstreamBaseURL)
			input.ProxyURL = firstNonEmpty(input.ProxyURL, runtime.ProxyURL)
			input.ProbeModel = firstNonEmpty(input.ProbeModel, runtime.ProbeModel)
		}
	}
	if strings.TrimSpace(input.ProbeModel) == "" && ownedPool != nil {
		if len(ownedPool.Models) > 0 {
			input.ProbeModel = strings.TrimSpace(ownedPool.Models[0])
		}
	}
	if strings.TrimSpace(input.ProbeModel) == "" && input.PoolID > 0 {
		if pool, err := s.repo.GetSharedPool(ctx, input.PoolID); err == nil && pool != nil {
			if len(pool.Models) > 0 {
				input.ProbeModel = strings.TrimSpace(pool.Models[0])
			}
		}
	}
	if strings.TrimSpace(input.ProbeModel) == "" {
		return nil, errors.New("probe model is required; open the model list and choose a detection model first")
	}
	var (
		result *SharedPoolUpstreamProbeResult
		err    error
	)
	if runtime != nil && strings.EqualFold(strings.TrimSpace(runtime.AuthType), AccountTypeOAuth) {
		credentials, credentialErr := s.sharedPoolOAuthCredentials(runtime)
		if credentialErr != nil {
			err = credentialErr
		} else {
			result, err = probeSharedPoolOAuthUpstream(ctx, input, credentials)
		}
	} else {
		result, err = probeSharedPoolUpstream(ctx, input)
	}
	if shouldSkipSharedPoolProbeOutcomeAfterCancellation(ctx, result, err) {
		return result, ctx.Err()
	}
	if !input.SkipRecord {
		s.recordSharedPoolProbeOutcome(ctx, input, result, err)
	}
	return result, err
}

func shouldSkipSharedPoolProbeOutcomeAfterCancellation(ctx context.Context, result *SharedPoolUpstreamProbeResult, probeErr error) bool {
	return errors.Is(ctx.Err(), context.Canceled) && (probeErr != nil || result == nil || !result.OK)
}

func sharedPoolProbeWriteContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), sharedPoolProbeWriteTimeout)
}

func (s *BizDecipherService) recordSharedPoolProbeOutcome(ctx context.Context, input SharedPoolUpstreamProbeInput, result *SharedPoolUpstreamProbeResult, probeErr error) {
	if s == nil || s.repo == nil || input.PoolID <= 0 {
		return
	}
	writeCtx, cancel := sharedPoolProbeWriteContext(ctx)
	defer cancel()
	ctx = writeCtx
	history := sharedPoolProbeHistoryInputFromResult(input, result, probeErr)
	if strings.TrimSpace(history.ModelName) == "" {
		history.ModelName = strings.TrimSpace(input.ProbeModel)
	}
	if strings.TrimSpace(history.UpstreamModelName) == "" {
		history.UpstreamModelName = strings.TrimSpace(input.ProbeModel)
	}
	if recordErr := s.repo.RecordSharedPoolProbeHistory(ctx, history); recordErr != nil {
		logger.LegacyPrintf("service.bizdecipher", "[SharedPoolProbe] record history failed pool=%d account=%d err=%v", input.PoolID, input.AccountID, recordErr)
	}
	if input.AccountID > 0 {
		if applyErr := s.repo.ApplySharedPoolAccountProbeResult(ctx, history); applyErr != nil {
			logger.LegacyPrintf("service.bizdecipher", "[SharedPoolProbe] apply account result failed pool=%d account=%d err=%v", input.PoolID, input.AccountID, applyErr)
			return
		}
		s.flushSharedPoolDisplayCache(ctx, "account_probe_result")
		return
	}
	if applyErr := s.repo.ApplySharedPoolProbeResult(ctx, history); applyErr != nil {
		logger.LegacyPrintf("service.bizdecipher", "[SharedPoolProbe] apply result failed pool=%d err=%v", input.PoolID, applyErr)
		return
	}
	s.flushSharedPoolDisplayCache(ctx, "probe_result")
}

func probeSharedPoolUpstream(ctx context.Context, input SharedPoolUpstreamProbeInput) (*SharedPoolUpstreamProbeResult, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(input.UpstreamBaseURL), "/")
	apiKey := strings.TrimSpace(input.UpstreamAPIKey)
	model := strings.TrimSpace(input.ProbeModel)
	if baseURL == "" {
		return nil, errors.New("upstream base url is required")
	}
	if apiKey == "" {
		return nil, errors.New("upstream api key is required")
	}
	if model == "" {
		return nil, errors.New("probe model is required")
	}
	validatedURL, err := validateSharedPoolUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	proxyURL := strings.TrimSpace(input.ProxyURL)
	checkLevel := sharedPoolProbeCheckLevel(input.ProbeType)
	probeCtx, cancel := sharedPoolProbeExecutionContext(ctx, checkLevel)
	defer cancel()
	modelsStarted := time.Now()
	models, modelsStatus, modelsErr := fetchSharedPoolUpstreamModelsWithStatus(probeCtx, validatedURL, apiKey, proxyURL)
	modelsCheck := sharedPoolModelsCheckFromResult(models, modelsStatus, modelsErr, model, int(time.Since(modelsStarted).Milliseconds()))
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	checks := runSharedPoolFullCheck(probeCtx, validatedURL, apiKey, model, proxyURL, checkLevel, modelsCheck)
	passed, total, score := summarizeSharedPoolFullCheck(checks)
	gateRequired := checkLevel == "full"
	gatePassed := sharedPoolFullCheckGatePassed(checks, score)
	latencyMs := maxSharedPoolCheckLatency(checks)
	httpStatus := firstSharedPoolCheckHTTPStatus(checks)
	result := &SharedPoolUpstreamProbeResult{
		OK:              gatePassed,
		Model:           model,
		Models:          models,
		Message:         "full capability check passed",
		CheckedAt:       checkedAt,
		LatencyMs:       latencyMs,
		HTTPStatus:      httpStatus,
		CheckLevel:      checkLevel,
		GateRequired:    gateRequired,
		GatePassed:      gatePassed,
		FullCheckPassed: passed,
		FullCheckTotal:  total,
		FullCheckScore:  score,
		Checks:          checks,
	}
	if executionErr := probeCtx.Err(); executionErr != nil {
		probeErr := classifySharedPoolProbeError(executionErr, 0)
		result.OK = false
		result.GatePassed = false
		result.Message = "full capability check did not finish"
		result.ErrorType = probeErr.ErrorType
		result.ErrorMessage = probeErr.Error()
		return result, executionErr
	}
	if gatePassed {
		return result, nil
	}
	failed := firstFailedSharedPoolCheck(checks)
	probeErr := classifySharedPoolProbeError(errors.New(firstNonEmpty(failed.ErrorMessage, "shared pool full capability check failed")), failed.HTTPStatus)
	if strings.TrimSpace(failed.ErrorType) != "" {
		probeErr.ErrorType = failed.ErrorType
	}
	if failed.HTTPStatus > 0 {
		probeErr.HTTPStatus = failed.HTTPStatus
	}
	if failed.ErrorType == "timeout" {
		probeErr.Cause = context.DeadlineExceeded
	} else if failed.ErrorType == "cancelled" {
		probeErr.Cause = context.Canceled
	}
	result.Message = fmt.Sprintf("full capability check failed: %d/%d passed", passed, total)
	result.ErrorType = probeErr.ErrorType
	result.ErrorMessage = sanitizeSharedPoolProbeMessage(firstNonEmpty(failed.ErrorMessage, result.Message))
	result.HTTPStatus = failed.HTTPStatus
	result.LatencyMs = failed.LatencyMs
	return result, probeErr
}

func (s *BizDecipherService) sharedPoolOAuthCredentials(runtime *SharedPoolUpstreamRuntime) (map[string]any, error) {
	if runtime == nil || !strings.EqualFold(strings.TrimSpace(runtime.AuthType), AccountTypeOAuth) {
		return nil, errors.New("oauth runtime is required")
	}
	if s == nil || s.secretEncryptor == nil {
		return nil, errors.New("oauth credential decryption is unavailable")
	}
	if runtime.ExpiresAt != nil && !runtime.ExpiresAt.After(time.Now()) {
		return nil, errors.New("oauth access token has expired; re-import the account credentials")
	}
	ciphertext := strings.TrimSpace(runtime.CredentialsEncrypted)
	if ciphertext == "" {
		return nil, errors.New("oauth credentials are missing")
	}
	plaintext, err := s.secretEncryptor.Decrypt(ciphertext)
	if err != nil {
		return nil, errors.New("oauth credentials could not be decrypted")
	}
	var credentials map[string]any
	if err := json.Unmarshal([]byte(plaintext), &credentials); err != nil {
		return nil, errors.New("oauth credentials are invalid")
	}
	if strings.TrimSpace(stringValue(credentials["access_token"])) == "" {
		return nil, errors.New("oauth access_token is missing")
	}
	return credentials, nil
}

func probeSharedPoolOAuthUpstream(ctx context.Context, input SharedPoolUpstreamProbeInput, credentials map[string]any) (*SharedPoolUpstreamProbeResult, error) {
	model := strings.TrimSpace(input.ProbeModel)
	if model == "" {
		return nil, errors.New("probe model is required")
	}
	accessToken := strings.TrimSpace(stringValue(credentials["access_token"]))
	accountID := strings.TrimSpace(firstNonEmpty(stringValue(credentials["chatgpt_account_id"]), stringValue(credentials["account_id"])))
	payload := createOpenAITestPayload(model, true)
	payloadBytes, _ := json.Marshal(payload)
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	started := time.Now()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, chatgptCodexAPIURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req = applySharedPoolOAuthProbeRequestShape(req, input, accessToken, accountID)
	resp, err := sharedPoolProbeHTTPClient(strings.TrimSpace(input.ProxyURL)).Do(req)
	latencyMs := int(time.Since(started).Milliseconds())
	if err != nil {
		probeErr := classifySharedPoolProbeError(err, 0)
		probeErr.Message = redactSharedPoolSecret(sanitizeSharedPoolProbeMessage(probeErr.Message), accessToken)
		return nil, probeErr
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		probeErr := classifySharedPoolProbeError(fmt.Errorf("oauth probe failed with %d: %s", resp.StatusCode, strings.TrimSpace(string(body))), resp.StatusCode)
		probeErr.Message = redactSharedPoolSecret(sanitizeSharedPoolProbeMessage(probeErr.Message), accessToken)
		return &SharedPoolUpstreamProbeResult{Model: model, Models: []string{model}, Message: "oauth capability check failed", CheckedAt: time.Now().UTC().Format(time.RFC3339), LatencyMs: latencyMs, HTTPStatus: resp.StatusCode, ErrorType: probeErr.ErrorType, ErrorMessage: probeErr.Message, CheckLevel: "oauth", GateRequired: true}, probeErr
	}
	content, streamErr := readSharedPoolOAuthProbeStream(resp.Body)
	check := SharedPoolFullCheckItem{ID: "oauth_responses", Title: "OpenAI OAuth Responses", Category: "compat", Required: true, LatencyMs: latencyMs, HTTPStatus: resp.StatusCode}
	result := &SharedPoolUpstreamProbeResult{Model: model, Models: []string{model}, CheckedAt: time.Now().UTC().Format(time.RFC3339), LatencyMs: latencyMs, HTTPStatus: resp.StatusCode, CheckLevel: "oauth", GateRequired: true, FullCheckTotal: 1, Checks: []SharedPoolFullCheckItem{check}}
	if streamErr != nil {
		probeErr := classifySharedPoolProbeError(streamErr, resp.StatusCode)
		probeErr.Message = redactSharedPoolSecret(sanitizeSharedPoolProbeMessage(probeErr.Message), accessToken)
		result.Message = "oauth capability check failed"
		result.ErrorType = probeErr.ErrorType
		result.ErrorMessage = probeErr.Message
		result.Checks[0].ErrorType = probeErr.ErrorType
		result.Checks[0].ErrorMessage = probeErr.Message
		return result, probeErr
	}
	check.Success = true
	check.Evidence = truncateSharedPoolEvidence(content)
	compactCheck := probeSharedPoolOAuthCompact(ctx, input, credentials)
	result.Checks = []SharedPoolFullCheckItem{check, compactCheck}
	result.FullCheckPassed, result.FullCheckTotal, result.FullCheckScore = summarizeSharedPoolFullCheck(result.Checks)
	// These two OAuth checks run sequentially, so expose their real wall-clock
	// cost instead of the max used by the parallel full-check path.
	result.LatencyMs = check.LatencyMs + compactCheck.LatencyMs
	result.OK = true
	result.GatePassed = true
	if compactCheck.Success {
		result.Message = "oauth Responses and Codex compact capability checks passed"
	} else {
		result.Message = "oauth Responses capability check passed; Codex compact is not verified"
	}
	return result, nil
}

func applySharedPoolOAuthProbeRequestShape(req *http.Request, input SharedPoolUpstreamProbeInput, accessToken, accountID string) *http.Request {
	if req == nil {
		return nil
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	probeSessionID := sharedPoolOAuthProbeSessionID(input)
	req.Host = "chatgpt.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("Originator", "codex_cli_rs")
	req.Header.Set("User-Agent", codexCLIUserAgent)
	req.Header.Set("Version", codexCLIVersion)
	req.Header.Set("Session_ID", probeSessionID)
	req.Header.Set("Conversation_ID", probeSessionID)
	if accountID != "" {
		req.Header.Set("chatgpt-account-id", accountID)
	}
	return req
}

func sharedPoolOAuthProbeSessionID(input SharedPoolUpstreamProbeInput) string {
	if input.AccountID > 0 {
		return compactProbeSessionID(input.AccountID)
	}
	if input.PoolID > 0 {
		return "probe_shared_pool_" + strconv.FormatInt(input.PoolID, 10)
	}
	return "probe_shared_pool"
}

func readSharedPoolOAuthProbeStream(body io.Reader) (string, error) {
	scanner := bufio.NewScanner(io.LimitReader(body, 2<<20))
	scanner.Buffer(make([]byte, 64*1024), 2<<20)
	var content strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event map[string]any
		if json.Unmarshal([]byte(payload), &event) != nil {
			continue
		}
		switch strings.TrimSpace(stringValue(event["type"])) {
		case "response.output_text.delta":
			content.WriteString(stringValue(event["delta"]))
		case "response.completed", "response.done":
			return content.String(), nil
		case "response.failed", "error":
			message := "OpenAI OAuth response failed"
			if errorData, ok := event["error"].(map[string]any); ok {
				message = firstNonEmpty(stringValue(errorData["message"]), message)
			}
			if responseData, ok := event["response"].(map[string]any); ok {
				if errorData, ok := responseData["error"].(map[string]any); ok {
					message = firstNonEmpty(stringValue(errorData["message"]), message)
				}
			}
			return "", errors.New(message)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("oauth stream ended before response.completed")
}

func validateSharedPoolUpstreamBaseURL(baseURL string) (string, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("upstream base url must be a valid http(s) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("upstream base url must use http or https")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func fetchSharedPoolUpstreamModels(ctx context.Context, baseURL, apiKey, proxyURL string) []string {
	models, _, err := fetchSharedPoolUpstreamModelsWithStatus(ctx, baseURL, apiKey, proxyURL)
	if err != nil {
		return []string{}
	}
	return models
}

func fetchSharedPoolUpstreamModelsWithStatus(ctx context.Context, baseURL, apiKey, proxyURL string) ([]string, int, error) {
	endpoint := buildOpenAIEndpointURL(baseURL, "/v1/models")
	probeCtx, cancel := context.WithTimeout(ctx, sharedPoolModelsProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return []string{}, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := sharedPoolProbeHTTPClient(proxyURL).Do(req)
	if err != nil {
		return []string{}, 0, fmt.Errorf("upstream models request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail := strings.TrimSpace(string(bodyBytes))
		if detail == "" {
			detail = resp.Status
		}
		return []string{}, resp.StatusCode, fmt.Errorf("upstream models endpoint failed with %d: %s", resp.StatusCode, detail)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return []string{}, resp.StatusCode, fmt.Errorf("upstream models endpoint returned invalid JSON: %w", err)
	}
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		models = append(models, item.ID)
	}
	return normalizeSharedPoolModels(models), resp.StatusCode, nil
}

func sharedPoolProbeHTTPClient(proxyURL string) *http.Client {
	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{Timeout: 20 * time.Second}
	}
	transport := baseTransport.Clone()
	_, parsed, err := parseSharedPoolProxyURL(proxyURL)
	if err != nil {
		transport.Proxy = func(*http.Request) (*url.URL, error) {
			return nil, errInvalidSharedPoolProxy
		}
	} else if parsed != nil {
		transport.Proxy = http.ProxyURL(parsed)
	}
	return &http.Client{Transport: transport, Timeout: 20 * time.Second}
}

func sharedPoolProbeCheckLevel(probeType string) string {
	switch strings.TrimSpace(probeType) {
	case "publish_gate", "manual", "scheduled_full":
		return "full"
	default:
		return "basic"
	}
}

func sharedPoolProbeExecutionContext(ctx context.Context, checkLevel string) (context.Context, context.CancelFunc) {
	if checkLevel == "full" {
		return context.WithTimeout(ctx, sharedPoolFullProbeTimeout)
	}
	return context.WithCancel(ctx)
}

type sharedPoolFullCheckSpec struct {
	item     SharedPoolFullCheckItem
	body     map[string]any
	keywords []string
}

func runSharedPoolFullCheck(ctx context.Context, baseURL, apiKey, model, proxyURL, checkLevel string, modelsCheck SharedPoolFullCheckItem) []SharedPoolFullCheckItem {
	if checkLevel != "full" {
		return []SharedPoolFullCheckItem{
			probeSharedPoolChatCompletion(ctx, baseURL, apiKey, model, proxyURL, SharedPoolFullCheckItem{ID: "chat_basic", Title: "Chat Completions basic response", Category: "compat", Required: true}, map[string]any{
				"model": model,
				"messages": []map[string]string{
					{"role": "system", "content": "You are a concise API compatibility checker."},
					{"role": "user", "content": "Reply with exactly: pong"},
				},
				"max_tokens": 8,
				"stream":     false,
			}, []string{"pong"}),
		}
	}
	coreChat := probeSharedPoolChatCompletion(ctx, baseURL, apiKey, model, proxyURL, SharedPoolFullCheckItem{ID: "chat_basic", Title: "Chat Completions basic response", Category: "compat", Required: true}, map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a concise API compatibility checker."},
			{"role": "user", "content": "Reply with exactly: pong"},
		},
		"max_tokens": 8,
		"stream":     false,
	}, []string{"pong"})
	checks := []SharedPoolFullCheckItem{modelsCheck, coreChat}
	fullChecks := sharedPoolFullCheckSpecs(model)
	if ctxErr := ctx.Err(); ctxErr != nil {
		for _, spec := range fullChecks {
			checks = append(checks, interruptedSharedPoolFullCheck(spec.item, ctxErr))
		}
		return checks
	}
	if !modelsCheck.Success || !coreChat.Success {
		for _, spec := range fullChecks {
			item := spec.item
			item.ErrorType = "skipped_core_failure"
			item.ErrorMessage = "skipped because a core compatibility check failed"
			checks = append(checks, item)
		}
		return checks
	}

	results := make([]SharedPoolFullCheckItem, len(fullChecks))
	semaphore := make(chan struct{}, sharedPoolFullProbeConcurrency)
	var wg sync.WaitGroup
	for index, spec := range fullChecks {
		index, spec := index, spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results[index] = interruptedSharedPoolFullCheck(spec.item, ctx.Err())
				return
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				results[index] = interruptedSharedPoolFullCheck(spec.item, ctxErr)
				return
			}
			results[index] = probeSharedPoolChatCompletion(ctx, baseURL, apiKey, model, proxyURL, spec.item, spec.body, spec.keywords)
		}()
	}
	wg.Wait()
	return append(checks, results...)
}

func sharedPoolFullCheckSpecs(model string) []sharedPoolFullCheckSpec {
	return []sharedPoolFullCheckSpec{
		{SharedPoolFullCheckItem{ID: "reasoning", Title: "Reasoning instruction following", Category: "capability", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "system", "content": "Follow the user instructions exactly."}, {"role": "user", "content": "Calculate 17 + 25 and include the token CHECK_REASONING in the answer."}}, "temperature": 0.1, "max_tokens": 80, "stream": false}, []string{"42", "check_reasoning"}},
		{SharedPoolFullCheckItem{ID: "code", Title: "Code generation", Category: "capability", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Write a JavaScript function named add that returns a+b. Include CHECK_CODE."}}, "temperature": 0.1, "max_tokens": 120, "stream": false}, []string{"function", "add", "check_code"}},
		{SharedPoolFullCheckItem{ID: "json_mode", Title: "Structured JSON output", Category: "compat", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Return only JSON: {\"ok\":true,\"marker\":\"CHECK_JSON\"}."}}, "temperature": 0, "max_tokens": 80, "stream": false}, []string{"check_json", "ok"}},
		{SharedPoolFullCheckItem{ID: "multilingual", Title: "Multilingual response", Category: "capability", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "用中文回答：共享池满血检测通过。并包含 CHECK_ZH。"}}, "temperature": 0.1, "max_tokens": 80, "stream": false}, []string{"check_zh", "共享池"}},
		{SharedPoolFullCheckItem{ID: "long_context", Title: "Long-context stability", Category: "capability", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "system", "content": strings.Repeat("context-line ", 512)}, {"role": "user", "content": "Summarize the previous context in one sentence and include CHECK_CONTEXT."}}, "temperature": 0.1, "max_tokens": 120, "stream": false}, []string{"check_context"}},
		{SharedPoolFullCheckItem{ID: "low_temperature", Title: "Low-temperature determinism", Category: "compat", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Return CHECK_TEMP and the number 7."}}, "temperature": 0, "max_tokens": 40, "stream": false}, []string{"check_temp", "7"}},
		{SharedPoolFullCheckItem{ID: "stop_sequence", Title: "Stop sequence handling", Category: "compat", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Say CHECK_STOP then STOP_HERE then extra words."}}, "temperature": 0, "max_tokens": 80, "stop": []string{"STOP_HERE"}, "stream": false}, []string{"check_stop"}},
		{SharedPoolFullCheckItem{ID: "system_priority", Title: "System message priority", Category: "compat", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "system", "content": "Always include CHECK_SYSTEM in your answer."}, {"role": "user", "content": "Say hello."}}, "temperature": 0.1, "max_tokens": 60, "stream": false}, []string{"check_system"}},
		{SharedPoolFullCheckItem{ID: "unicode", Title: "Unicode handling", Category: "compat", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Echo this marker exactly once: CHECK_UNICODE_零点城"}}, "temperature": 0, "max_tokens": 60, "stream": false}, []string{"check_unicode", "零点城"}},
		{SharedPoolFullCheckItem{ID: "usage_shape", Title: "Usage payload compatibility", Category: "billing", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Reply CHECK_USAGE."}}, "temperature": 0, "max_tokens": 32, "stream": false}, []string{"check_usage"}},
		{SharedPoolFullCheckItem{ID: "stream", Title: "Streaming response", Category: "compat", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Reply CHECK_STREAM."}}, "temperature": 0, "max_tokens": 32, "stream": true}, []string{"check_stream"}},
		{SharedPoolFullCheckItem{ID: "error_shape", Title: "Error shape guard", Category: "trust", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Reply CHECK_ERROR_SHAPE."}}, "temperature": 0, "max_tokens": 32, "stream": false}, []string{"check_error_shape"}},
		{SharedPoolFullCheckItem{ID: "latency_guard", Title: "Latency guard", Category: "trust", Required: true}, map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Reply CHECK_LATENCY."}}, "temperature": 0, "max_tokens": 32, "stream": false}, []string{"check_latency"}},
	}
}

func interruptedSharedPoolFullCheck(item SharedPoolFullCheckItem, err error) SharedPoolFullCheckItem {
	probeErr := classifySharedPoolProbeError(err, 0)
	item.ErrorType = probeErr.ErrorType
	item.ErrorMessage = probeErr.Error()
	return item
}

func sharedPoolFullProbePlannedWorstCase(extraChecks int) time.Duration {
	if extraChecks < 0 {
		extraChecks = 0
	}
	batches := (extraChecks + sharedPoolFullProbeConcurrency - 1) / sharedPoolFullProbeConcurrency
	return sharedPoolModelsProbeTimeout + sharedPoolChatProbeTimeout + time.Duration(batches)*sharedPoolChatProbeTimeout
}

func probeSharedPoolModelsEndpoint(ctx context.Context, baseURL, apiKey, model, proxyURL string) SharedPoolFullCheckItem {
	started := time.Now()
	models, status, err := fetchSharedPoolUpstreamModelsWithStatus(ctx, baseURL, apiKey, proxyURL)
	return sharedPoolModelsCheckFromResult(models, status, err, model, int(time.Since(started).Milliseconds()))
}

func sharedPoolModelsCheckFromResult(models []string, status int, err error, model string, latencyMs int) SharedPoolFullCheckItem {
	item := SharedPoolFullCheckItem{ID: "models", Title: "GET /v1/models", Category: "compat", Required: true, LatencyMs: latencyMs}
	item.HTTPStatus = status
	if err != nil {
		probeErr := classifySharedPoolProbeError(err, status)
		item.ErrorType = probeErr.ErrorType
		item.ErrorMessage = sanitizeSharedPoolProbeMessage(probeErr.Error())
		return item
	}
	if len(models) == 0 {
		item.ErrorType = "model_error"
		item.ErrorMessage = "models endpoint returned no models"
		return item
	}
	item.Success = true
	if item.HTTPStatus == 0 {
		item.HTTPStatus = http.StatusOK
	}
	item.Evidence = firstNonEmpty(matchSharedPoolModelName(models, model), models[0])
	return item
}

func probeSharedPoolChatCompletion(ctx context.Context, baseURL, apiKey, model, proxyURL string, item SharedPoolFullCheckItem, body map[string]any, keywords []string) SharedPoolFullCheckItem {
	probeCtx, cancel := context.WithTimeout(ctx, sharedPoolChatProbeTimeout)
	defer cancel()
	started := time.Now()
	content, status, err := doSharedPoolChatCompletion(probeCtx, baseURL, apiKey, proxyURL, body)
	item.LatencyMs = int(time.Since(started).Milliseconds())
	item.HTTPStatus = status
	if err != nil {
		probeErr := classifySharedPoolProbeError(err, status)
		item.ErrorType = probeErr.ErrorType
		item.ErrorMessage = sanitizeSharedPoolProbeMessage(probeErr.Error())
		return item
	}
	missing := []string{}
	lowerContent := strings.ToLower(content)
	for _, keyword := range keywords {
		if strings.TrimSpace(keyword) == "" {
			continue
		}
		if !strings.Contains(lowerContent, strings.ToLower(keyword)) {
			missing = append(missing, keyword)
		}
	}
	if len(missing) > 0 {
		item.ErrorType = "invalid_response"
		item.ErrorMessage = "response missing marker: " + strings.Join(missing, ", ")
		item.Evidence = truncateSharedPoolEvidence(content)
		return item
	}
	item.Success = true
	item.Evidence = truncateSharedPoolEvidence(content)
	return item
}

func doSharedPoolChatCompletion(ctx context.Context, baseURL, apiKey, proxyURL string, body map[string]any) (string, int, error) {
	bodyBytes, _ := json.Marshal(body)
	chatURL := buildOpenAIChatCompletionsURL(baseURL)
	if strings.TrimSpace(chatURL) == "" {
		return "", 0, errors.New("upstream chat completions endpoint is invalid")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if body["stream"] == true {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := sharedPoolProbeHTTPClient(proxyURL).Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("upstream probe request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	bodyLimit := int64(1 << 20)
	if body["stream"] == true {
		bodyLimit = 1 << 18
	}
	bodyBytes, _ = io.ReadAll(io.LimitReader(resp.Body, bodyLimit))
	detail := strings.TrimSpace(string(bodyBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if detail == "" {
			detail = resp.Status
		}
		return "", resp.StatusCode, fmt.Errorf("upstream probe failed with %d: %s", resp.StatusCode, detail)
	}
	if body["stream"] == true {
		content, streamErr := extractSharedPoolProbeStreamContent(detail)
		if streamErr != nil {
			return "", resp.StatusCode, streamErr
		}
		return content, resp.StatusCode, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return "", resp.StatusCode, fmt.Errorf("upstream probe returned invalid JSON: %w", err)
	}
	if _, ok := payload["error"]; ok {
		return "", resp.StatusCode, errors.New("upstream probe returned an error")
	}
	return extractSharedPoolProbeContent(payload), resp.StatusCode, nil
}

func extractSharedPoolProbeStreamContent(detail string) (string, error) {
	var content strings.Builder
	for _, line := range strings.Split(detail, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}
		if _, ok := payload["error"]; ok {
			return "", errors.New("upstream probe stream returned an error")
		}
		choices, _ := payload["choices"].([]any)
		if len(choices) == 0 {
			continue
		}
		choice, _ := choices[0].(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		if text := extractSharedPoolProbeMessageContent(delta["content"]); text != "" {
			_, _ = content.WriteString(text)
			continue
		}
		if text, ok := choice["text"].(string); ok {
			_, _ = content.WriteString(text)
		}
	}
	if content.Len() > 0 {
		return content.String(), nil
	}
	if strings.Contains(detail, "\"error\"") {
		return "", errors.New("upstream probe stream returned an error")
	}
	return detail, nil
}

func extractSharedPoolProbeContent(payload map[string]any) string {
	choices, _ := payload["choices"].([]any)
	if len(choices) == 0 {
		return ""
	}
	choice, _ := choices[0].(map[string]any)
	if text, ok := choice["text"].(string); ok && strings.TrimSpace(text) != "" {
		return text
	}
	message, _ := choice["message"].(map[string]any)
	return extractSharedPoolProbeMessageContent(message["content"])
}

func extractSharedPoolProbeMessageContent(value any) string {
	if content, ok := value.(string); ok {
		return content
	}
	if contentItems, ok := value.([]any); ok {
		parts := []string{}
		for _, item := range contentItems {
			if itemMap, ok := item.(map[string]any); ok {
				if text, ok := itemMap["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func matchSharedPoolModelName(models []string, target string) string {
	target = strings.TrimSpace(strings.ToLower(target))
	if target == "" {
		return ""
	}
	for _, model := range models {
		if strings.ToLower(strings.TrimSpace(model)) == target {
			return model
		}
	}
	return ""
}

const sharedPoolFullCheckPassingScore = 70.0

// sharedPoolFullCheckGatePassed intentionally keeps the transitional gate
// model-aware and tolerant. The core chat request must work, while the wider
// capability matrix remains evidence and needs a 70+ score. A single optional
// capability mismatch must not reject an otherwise usable mainstream model.
func sharedPoolFullCheckGatePassed(checks []SharedPoolFullCheckItem, score float64) bool {
	if len(checks) == 0 || score < sharedPoolFullCheckPassingScore {
		return false
	}
	for _, check := range checks {
		if check.ID == "chat_basic" {
			return check.Success
		}
	}
	return false
}

func summarizeSharedPoolFullCheck(checks []SharedPoolFullCheckItem) (int, int, float64) {
	passed := 0
	total := 0
	for _, check := range checks {
		if !check.Required {
			continue
		}
		total++
		if check.Success {
			passed++
		}
	}
	if total == 0 {
		return 0, 0, 0
	}
	return passed, total, float64(passed) * 100 / float64(total)
}

func maxSharedPoolCheckLatency(checks []SharedPoolFullCheckItem) int {
	maxLatency := 0
	for _, check := range checks {
		if check.LatencyMs > maxLatency {
			maxLatency = check.LatencyMs
		}
	}
	return maxLatency
}

func firstSharedPoolCheckHTTPStatus(checks []SharedPoolFullCheckItem) int {
	for _, check := range checks {
		if check.HTTPStatus > 0 {
			return check.HTTPStatus
		}
	}
	return 0
}

func firstFailedSharedPoolCheck(checks []SharedPoolFullCheckItem) SharedPoolFullCheckItem {
	for _, check := range checks {
		if check.Required && !check.Success {
			return check
		}
	}
	return SharedPoolFullCheckItem{ErrorMessage: "shared pool full capability check failed"}
}

func truncateSharedPoolEvidence(content string) string {
	content = sanitizeSharedPoolProbeMessage(content)
	const maxRunes = 180
	runes := []rune(content)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return content
}

func classifySharedPoolProbeError(err error, httpStatus int) *SharedPoolProbeError {
	if err == nil {
		return nil
	}
	message := sanitizeSharedPoolProbeMessage(err.Error())
	errorType := "upstream_error"
	lower := strings.ToLower(message)
	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded"):
		errorType = "timeout"
	case errors.Is(err, context.Canceled):
		errorType = "cancelled"
	case httpStatus == http.StatusUnauthorized || httpStatus == http.StatusForbidden || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "invalid api key"):
		errorType = "auth_error"
	case httpStatus == http.StatusTooManyRequests || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests"):
		errorType = "rate_limited"
	case httpStatus == http.StatusBadRequest || httpStatus == http.StatusNotFound || strings.Contains(lower, "model") && (strings.Contains(lower, "not found") || strings.Contains(lower, "does not exist")):
		errorType = "model_error"
	case httpStatus >= 500:
		errorType = "upstream_5xx"
	case httpStatus >= 400:
		errorType = "upstream_4xx"
	case strings.Contains(lower, "invalid json"):
		errorType = "invalid_response"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") || strings.Contains(lower, "proxyconnect"):
		errorType = "network_error"
	}
	return &SharedPoolProbeError{ErrorType: errorType, HTTPStatus: httpStatus, Message: message, Cause: err}
}

func sanitizeSharedPoolProbeMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "shared pool upstream probe failed"
	}
	message = strings.ReplaceAll(message, "\n", " ")
	message = strings.ReplaceAll(message, "\r", " ")
	for strings.Contains(message, "  ") {
		message = strings.ReplaceAll(message, "  ", " ")
	}
	const maxRunes = 500
	runes := []rune(message)
	if len(runes) > maxRunes {
		message = string(runes[:maxRunes])
	}
	return message
}

func redactSharedPoolSecret(message, secret string) string {
	message = strings.TrimSpace(message)
	secret = strings.TrimSpace(secret)
	if message == "" {
		return "shared pool upstream probe failed"
	}
	if secret == "" {
		return message
	}
	message = strings.ReplaceAll(message, secret, "[redacted]")
	if len(secret) >= 8 {
		message = strings.ReplaceAll(message, secret[:4], "[redacted]")
		message = strings.ReplaceAll(message, secret[len(secret)-4:], "[redacted]")
	}
	return message
}

func sharedPoolProbeHistoryInputFromResult(input SharedPoolUpstreamProbeInput, result *SharedPoolUpstreamProbeResult, err error) SharedPoolProbeHistoryInput {
	probeType := strings.TrimSpace(input.ProbeType)
	if probeType == "" {
		probeType = "manual"
	}
	switch probeType {
	case "manual", "publish_gate", "scheduled", "scheduled_full":
	default:
		probeType = "manual"
	}
	checkLevel := sharedPoolProbeCheckLevel(probeType)
	history := SharedPoolProbeHistoryInput{
		PoolID:            input.PoolID,
		AccountID:         input.AccountID,
		OwnerID:           input.OwnerID,
		ModelName:         strings.TrimSpace(input.ProbeModel),
		UpstreamModelName: strings.TrimSpace(input.ProbeModel),
		ProbeType:         probeType,
		CheckedAt:         time.Now().UTC(),
		Metadata: SharedPoolProbeMetadata{
			CheckLevel:   checkLevel,
			GateRequired: checkLevel == "full",
			GatePassed:   false,
		},
	}
	if result != nil {
		history.ModelName = strings.TrimSpace(result.Model)
		if history.UpstreamModelName == "" {
			history.UpstreamModelName = strings.TrimSpace(result.Model)
		}
		history.Success = result.OK
		history.HTTPStatus = result.HTTPStatus
		history.ErrorType = strings.TrimSpace(result.ErrorType)
		history.ErrorMessage = sanitizeSharedPoolProbeMessage(result.ErrorMessage)
		history.LatencyMs = result.LatencyMs
		history.Metadata = SharedPoolProbeMetadata{
			CheckLevel:      result.CheckLevel,
			GateRequired:    result.GateRequired,
			GatePassed:      result.GatePassed,
			FullCheckPassed: result.FullCheckPassed,
			FullCheckTotal:  result.FullCheckTotal,
			FullCheckScore:  result.FullCheckScore,
			Checks:          result.Checks,
		}
		if checkedAt, parseErr := time.Parse(time.RFC3339, result.CheckedAt); parseErr == nil {
			history.CheckedAt = checkedAt
		}
	}
	if err != nil {
		probeErr := classifySharedPoolProbeError(err, history.HTTPStatus)
		history.Success = false
		if probeErr != nil {
			history.HTTPStatus = probeErr.HTTPStatus
			history.ErrorType = probeErr.ErrorType
			history.ErrorMessage = sanitizeSharedPoolProbeMessage(probeErr.Error())
		}
	}
	if history.Success {
		history.ErrorType = ""
		history.ErrorMessage = ""
	}
	return history
}

func shouldProbeSharedPoolBeforeListing(listed bool, status string) bool {
	return listed && (status == "healthy" || status == "limited")
}

// ensureAccountModePoolListable requires at least one account-mode account that
// has passed its capability gate before the pool can be listed in the market.
func (s *BizDecipherService) ensureAccountModePoolListable(ctx context.Context, poolID, ownerID int64, verificationMode string) error {
	if s == nil || s.repo == nil {
		return errors.New("service unavailable")
	}
	if poolID <= 0 || ownerID <= 0 {
		return errors.New("invalid pool or owner id")
	}
	verificationMode = strings.TrimSpace(strings.ToLower(verificationMode))
	accounts, err := s.ListSharedPoolAccounts(ctx, poolID, ownerID)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, acc := range accounts {
		// Listing must use the same current routing predicate as request
		// dispatch. A historical gate_passed snapshot is not sufficient when
		// the account has since been disabled, expired, lost credentials, or
		// marked non-schedulable. Keep professional-review pools subject to
		// this predicate as well; the route query still enforces gate_required.
		if !sharedPoolAccountEligibleForProbe(acc, "", now) {
			continue
		}
		if acc.GateRequired && !acc.GatePassed {
			continue
		}
		return nil
	}
	if verificationMode == "professional_review" {
		return errors.New("at least one schedulable account is required before listing this professionally reviewed pool")
	}
	return errors.New("at least one account must pass full capability check before listing this pool")
}

func requiresSharedPoolPublishProbe(verificationMode string) bool {
	return !strings.EqualFold(strings.TrimSpace(verificationMode), "professional_review")
}

func SharedPoolGovernanceAllowsOwnerListing(governanceStatus string) bool {
	switch strings.ToLower(strings.TrimSpace(governanceStatus)) {
	case "", "normal", "boosted":
		return true
	default:
		return false
	}
}

func (s *BizDecipherService) resolveSharedPoolVerificationPolicy(ctx context.Context, configs []SharedPoolModelInput, requestedMode string, exemptionReason string) (string, string, error) {
	openModels := make([]string, 0, len(configs)*2)
	for _, cfg := range configs {
		if !cfg.ModelOpen {
			continue
		}
		for _, modelName := range []string{cfg.ModelName, cfg.UpstreamModelName} {
			modelName = strings.TrimSpace(modelName)
			if modelName != "" {
				openModels = append(openModels, modelName)
			}
		}
	}
	requestedMode = strings.TrimSpace(strings.ToLower(requestedMode))
	exemptionReason = strings.TrimSpace(exemptionReason)
	if len(openModels) == 0 {
		if requestedMode == "professional_review" {
			return "", "", errors.New("professional review requires at least one open model")
		}
		return "full_check", "", nil
	}
	catalog, err := s.repo.ListModelCatalog(ctx)
	if err != nil {
		if requestedMode == "professional_review" {
			return "", "", errors.New("model catalog is unavailable; professional review cannot be verified")
		}
		return "full_check", "", nil
	}
	catalogByName := map[string]ModelCatalogEntry{}
	for _, entry := range catalog {
		catalogByName[strings.ToLower(strings.TrimSpace(entry.ModelName))] = entry
		for _, alias := range entry.Aliases {
			catalogByName[strings.ToLower(strings.TrimSpace(alias))] = entry
		}
	}
	requiresFullCheck := false
	for _, modelName := range openModels {
		entry, ok := catalogByName[strings.ToLower(strings.TrimSpace(modelName))]
		if ok && entry.Mainstream {
			requiresFullCheck = true
			break
		}
	}
	if requiresFullCheck {
		if requestedMode == "professional_review" {
			return "", "", errors.New("mainstream models must pass full verification before listing")
		}
		return "full_check", "", nil
	}
	if requestedMode == "" {
		return "full_check", "", nil
	}
	if requestedMode != "professional_review" && requestedMode != "full_check" {
		return "", "", errors.New("invalid verification mode")
	}
	if requestedMode == "professional_review" {
		if exemptionReason == "" {
			return "", "", errors.New("verification exemption reason is required when using professional review")
		}
		return "professional_review", exemptionReason, nil
	}
	return "full_check", "", nil
}

// ListSharedPools returns the marketplace catalog plus aggregate header stats.

func sharedPoolModelNames(configs []SharedPoolModelInput, models []string) []string {
	out := make([]string, 0, len(configs)+len(models))
	seen := map[string]struct{}{}
	for _, cfg := range configs {
		name := strings.TrimSpace(cfg.ModelName)
		if name == "" {
			name = strings.TrimSpace(cfg.UpstreamModelName)
		}
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, model := range models {
		name := strings.TrimSpace(model)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func resolveSharedPoolProbeModel(probeModel string, configs []SharedPoolModelInput, models []string) (string, error) {
	probeModel = strings.TrimSpace(probeModel)
	available := sharedPoolModelNames(configs, models)
	if len(available) == 0 {
		return "", errors.New("at least one model is required")
	}
	if probeModel == "" {
		return "", errors.New("probe model is required; select a detection model from the open model list")
	}
	for _, name := range available {
		if name == probeModel {
			return probeModel, nil
		}
	}
	for _, cfg := range configs {
		if strings.TrimSpace(cfg.UpstreamModelName) == probeModel || strings.TrimSpace(cfg.ModelName) == probeModel {
			if strings.TrimSpace(cfg.ModelName) != "" {
				return strings.TrimSpace(cfg.ModelName), nil
			}
			return probeModel, nil
		}
	}
	return "", fmt.Errorf("probe model %q must be one of the open models", probeModel)
}

func preferSharedPoolProbeModel(configs []SharedPoolModelInput, probeModel string) []SharedPoolModelInput {
	probeModel = strings.TrimSpace(probeModel)
	if probeModel == "" || len(configs) == 0 {
		return configs
	}
	out := make([]SharedPoolModelInput, 0, len(configs))
	var preferred *SharedPoolModelInput
	for _, cfg := range configs {
		name := strings.TrimSpace(cfg.ModelName)
		upstream := strings.TrimSpace(cfg.UpstreamModelName)
		if preferred == nil && (name == probeModel || upstream == probeModel) {
			copyCfg := cfg
			preferred = &copyCfg
			continue
		}
		out = append(out, cfg)
	}
	if preferred == nil {
		return configs
	}
	return append([]SharedPoolModelInput{*preferred}, out...)
}

func (s *BizDecipherService) ListSharedPools(ctx context.Context, filter SharedPoolFilter) (*SharedPoolListView, error) {
	if s == nil || s.repo == nil {
		return &SharedPoolListView{Pools: []SharedPool{}}, nil
	}
	if view, ok := s.getSharedPoolListCache(ctx, filter); ok {
		return view, nil
	}
	pools, err := s.repo.ListSharedPools(ctx, filter)
	if err != nil {
		return nil, err
	}
	s.enrichSharedPoolOfficialPricing(pools)
	view := buildSharedPoolListView(pools)
	s.saveSharedPoolListCache(ctx, filter, view)
	return view, nil
}

// GetSharedPool returns a single pool by id.
func (s *BizDecipherService) GetSharedPool(ctx context.Context, id int64) (*SharedPool, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if id <= 0 {
		return nil, errors.New("invalid pool id")
	}
	if pool, ok := s.getSharedPoolCache(ctx, id); ok {
		return pool, nil
	}
	pool, err := s.repo.GetSharedPool(ctx, id)
	if err != nil {
		return nil, err
	}
	if pool != nil {
		pools := []SharedPool{*pool}
		s.enrichSharedPoolOfficialPricing(pools)
		*pool = pools[0]
	}
	s.saveSharedPoolCache(ctx, pool)
	return pool, nil
}

func buildSharedPoolListView(pools []SharedPool) *SharedPoolListView {
	view := &SharedPoolListView{Pools: pools, Total: len(pools)}
	var availSum float64
	for _, p := range pools {
		switch p.Status {
		case "healthy":
			view.Online++
		case "limited":
			view.Limited++
		}
		availSum += p.TodayAvailability
	}
	if len(pools) > 0 {
		view.AvgAvailability = availSum / float64(len(pools))
	}
	return view
}

func (s *BizDecipherService) sharedPoolCacheContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultSharedPoolCacheOperationTimeout)
}

func (s *BizDecipherService) getSharedPoolListCache(ctx context.Context, filter SharedPoolFilter) (*SharedPoolListView, bool) {
	if s == nil || s.sharedPoolCache == nil {
		return nil, false
	}
	view, err := s.sharedPoolCache.GetSharedPoolList(ctx, filter)
	if err == nil && view != nil {
		return view, true
	}
	if err != nil && !errors.Is(err, ErrSharedPoolDisplayCacheMiss) {
		logger.LegacyPrintf("service.bizdecipher", "[SharedPoolCache] list read failed: %v", err)
	}
	return nil, false
}

func (s *BizDecipherService) saveSharedPoolListCache(ctx context.Context, filter SharedPoolFilter, view *SharedPoolListView) {
	if s == nil || s.sharedPoolCache == nil || view == nil {
		return
	}
	cacheCtx, cancel := s.sharedPoolCacheContext()
	defer cancel()
	if err := s.sharedPoolCache.SetSharedPoolList(cacheCtx, filter, view, defaultSharedPoolDisplayCacheTTL); err != nil {
		logger.LegacyPrintf("service.bizdecipher", "[SharedPoolCache] list write failed: %v", err)
	}
}

func (s *BizDecipherService) getSharedPoolCache(ctx context.Context, id int64) (*SharedPool, bool) {
	if s == nil || s.sharedPoolCache == nil || id <= 0 {
		return nil, false
	}
	pool, err := s.sharedPoolCache.GetSharedPool(ctx, id)
	if err == nil && pool != nil {
		return pool, true
	}
	if err != nil && !errors.Is(err, ErrSharedPoolDisplayCacheMiss) {
		logger.LegacyPrintf("service.bizdecipher", "[SharedPoolCache] detail read failed id=%d err=%v", id, err)
	}
	return nil, false
}

func (s *BizDecipherService) saveSharedPoolCache(ctx context.Context, pool *SharedPool) {
	if s == nil || s.sharedPoolCache == nil || pool == nil || pool.ID <= 0 {
		return
	}
	cacheCtx, cancel := s.sharedPoolCacheContext()
	defer cancel()
	if err := s.sharedPoolCache.SetSharedPool(cacheCtx, pool, defaultSharedPoolDisplayCacheTTL); err != nil {
		logger.LegacyPrintf("service.bizdecipher", "[SharedPoolCache] detail write failed id=%d err=%v", pool.ID, err)
	}
}

func (s *BizDecipherService) flushSharedPoolDisplayCache(ctx context.Context, reason string) {
	if s == nil || s.sharedPoolCache == nil {
		return
	}
	cacheCtx, cancel := s.sharedPoolCacheContext()
	defer cancel()
	if err := s.sharedPoolCache.FlushSharedPoolDisplay(cacheCtx); err != nil {
		logger.LegacyPrintf("service.bizdecipher", "[SharedPoolCache] flush failed reason=%s err=%v", reason, err)
	}
}

func (s *BizDecipherService) ListSharedPoolProbeHistories(ctx context.Context, poolID, accountID, ownerID int64, limit int) ([]SharedPoolProbeHistory, error) {
	if s == nil || s.repo == nil {
		return []SharedPoolProbeHistory{}, nil
	}
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	if accountID < 0 {
		return nil, errors.New("invalid account id")
	}
	if ownerID < 0 {
		return nil, errors.New("invalid owner id")
	}
	return s.repo.ListSharedPoolProbeHistories(ctx, poolID, accountID, ownerID, limit)
}

type sharedPoolLatestFullCheckRepository interface {
	GetLatestSharedPoolFullCheckHistory(ctx context.Context, poolID int64) (*SharedPoolProbeHistory, error)
}

func (s *BizDecipherService) GetLatestSharedPoolFullCheckHistory(ctx context.Context, poolID int64) (*SharedPoolProbeHistory, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	repo, ok := s.repo.(sharedPoolLatestFullCheckRepository)
	if !ok || repo == nil {
		return nil, errors.New("shared pool full-check history repository is unavailable")
	}
	return repo.GetLatestSharedPoolFullCheckHistory(ctx, poolID)
}

func (s *BizDecipherService) AdminListSharedPoolGovernanceLogs(ctx context.Context, poolID int64, limit int) ([]SharedPoolGovernanceLog, error) {
	if s == nil || s.repo == nil {
		return []SharedPoolGovernanceLog{}, nil
	}
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	return s.repo.ListSharedPoolGovernanceLogs(ctx, poolID, limit)
}

func (s *BizDecipherService) RunSharedPoolProbeAggregation(ctx context.Context, now time.Time, limit int) (*SharedPoolProbeAggregationSummary, error) {
	if s == nil || s.repo == nil {
		return &SharedPoolProbeAggregationSummary{}, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	if repoSummary, err := s.repo.RunSharedPoolProbeAggregation(ctx, now, limit); err == nil && repoSummary != nil && repoSummary.PoolsChecked > 0 {
		return repoSummary, nil
	} else if err != nil {
		return nil, err
	}
	candidates, err := s.repo.ListSharedPoolProbeCandidates(ctx, limit)
	if err != nil {
		return nil, err
	}
	summary := &SharedPoolProbeAggregationSummary{
		CandidatesFound: len(candidates),
	}
	if len(candidates) == 0 {
		summary.SkippedCooldown = true
		summary.Message = "当前没有可探测目标：候选池/账号均在 5 分钟冷却内，或尚无上架可调度账号"
		return summary, nil
	}
	for _, candidate := range candidates {
		probeType := "scheduled"
		if candidate.FullProbeRequired {
			probeType = "scheduled_full"
		}
		probeInput := SharedPoolUpstreamProbeInput{
			PoolID:          candidate.PoolID,
			AccountID:       candidate.AccountID,
			OwnerID:         candidate.OwnerID,
			UpstreamBaseURL: candidate.UpstreamBaseURL,
			UpstreamAPIKey:  candidate.UpstreamAPIKey,
			ProbeModel:      firstNonEmpty(candidate.UpstreamModelName, candidate.ProbeModel),
			ProbeType:       probeType,
			ProxyURL:        candidate.ProxyURL,
		}
		result, probeErr := s.ProbeSharedPoolUpstream(ctx, probeInput)
		if candidate.AccountID > 0 {
			summary.AccountsChecked++
		} else {
			summary.PoolLevelChecked++
		}
		summary.PoolsChecked++
		if probeErr == nil && result != nil && result.OK {
			summary.Succeeded++
		} else {
			summary.Failed++
			httpStatus := 0
			errorType := ""
			if result != nil {
				httpStatus = result.HTTPStatus
				errorType = result.ErrorType
			}
			if typedErr := new(SharedPoolProbeError); errors.As(probeErr, &typedErr) {
				httpStatus = typedErr.HTTPStatus
				errorType = typedErr.ErrorType
			}
			if httpStatus == http.StatusTooManyRequests || errorType == "rate_limited" {
				summary.Limited++
			}
			if httpStatus == 0 || httpStatus >= 500 || errorType == "timeout" || errorType == "network_error" {
				summary.Offlined++
			}
		}
	}
	if summary.PoolsChecked > 0 {
		s.flushSharedPoolDisplayCache(ctx, "scheduled_probe_aggregation")
		summary.Message = fmt.Sprintf("已检查 %d 个目标（账号 %d，池级 %d）", summary.PoolsChecked, summary.AccountsChecked, summary.PoolLevelChecked)
	}
	return summary, nil
}

func (s *BizDecipherService) ImportSharedPools(ctx context.Context, input ImportSharedPoolsInput) (*SharedPoolImportResult, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if input.OwnerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	result := &SharedPoolImportResult{
		Total: len(input.Items),
		Items: make([]SharedPoolImportItemResult, 0, len(input.Items)),
	}
	if len(input.Items) == 0 {
		return result, nil
	}
	if len(input.Items) > 50 {
		return nil, errors.New("shared pool import supports at most 50 items per batch")
	}
	for index, item := range input.Items {
		item.OwnerID = input.OwnerID
		pool, err := s.CreateSharedPool(ctx, item)
		itemResult := SharedPoolImportItemResult{Index: index, Name: strings.TrimSpace(item.Name)}
		if err != nil {
			itemResult.Error = err.Error()
			result.Failed++
		} else {
			itemResult.Created = true
			itemResult.Pool = pool
			result.Created++
		}
		result.Items = append(result.Items, itemResult)
	}
	return result, nil
}

func (s *BizDecipherService) CreateSharedPool(ctx context.Context, input CreateSharedPoolInput) (*SharedPool, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if err := normalizeSharedPoolBranding(sharedPoolBrandingFields{
		Name:           &input.Name,
		Description:    &input.Description,
		AvatarURL:      &input.AvatarURL,
		StatusNote:     &input.StatusNote,
		DisabledReason: &input.DisabledReason,
	}); err != nil {
		return nil, err
	}
	input.UpstreamBaseURL = strings.TrimRight(strings.TrimSpace(input.UpstreamBaseURL), "/")
	input.UpstreamAPIKey = strings.TrimSpace(input.UpstreamAPIKey)
	input.ProxyURL = strings.TrimSpace(input.ProxyURL)
	input.ProxyRegion = strings.TrimSpace(input.ProxyRegion)
	input.ProxyStatus = strings.TrimSpace(input.ProxyStatus)
	input.OAuthProvider = strings.TrimSpace(input.OAuthProvider)
	input.ModelConfigs = normalizeSharedPoolModelInputs(input.ModelConfigs, input.Models, input.RateMultiplier)
	input.Models = openSharedPoolModelInputNames(input.ModelConfigs)
	if input.OwnerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	if input.Name == "" {
		return nil, errors.New("pool name is required")
	}
	if input.AccountModeEnabled {
		input.UpstreamBaseURL = ""
		input.UpstreamAPIKey = ""
		input.Listed = false
	} else {
		if input.UpstreamBaseURL == "" {
			return nil, errors.New("upstream base url is required")
		}
		if input.UpstreamAPIKey == "" {
			return nil, errors.New("upstream api key is required")
		}
	}
	if len(input.Models) == 0 {
		if !input.AccountModeEnabled {
			return nil, errors.New("at least one model is required")
		}
		input.ProbeModel = ""
		input.ModelConfigs = nil
	} else {
		probeModel, err := resolveSharedPoolProbeModel(input.ProbeModel, input.ModelConfigs, input.Models)
		if err != nil {
			return nil, err
		}
		input.ProbeModel = probeModel
		input.ModelConfigs = preferSharedPoolProbeModel(input.ModelConfigs, probeModel)
	}
	if input.RateMultiplier <= 0 {
		input.RateMultiplier = 1
	}
	if input.MaxUsers <= 0 {
		input.MaxUsers = 20
	}
	if input.MinBalanceAdmission < 0 {
		input.MinBalanceAdmission = 0
	}
	if input.HourlySeatFee < 0 {
		input.HourlySeatFee = 0
	}
	if input.HourlyMinUsageWaiver < 0 {
		input.HourlyMinUsageWaiver = 0
	}
	if input.AccountConcurrency <= 0 {
		input.AccountConcurrency = 1
	}
	if input.UserConcurrency <= 0 {
		input.UserConcurrency = 1
	}
	if input.OAuthProvider == "" {
		input.OAuthProvider = "openai"
	}
	verificationMode, verificationReason, err := s.resolveSharedPoolVerificationPolicy(ctx, input.ModelConfigs, input.VerificationMode, input.VerificationExemptionReason)
	if err != nil {
		return nil, err
	}
	input.VerificationMode = verificationMode
	input.VerificationExemptionReason = verificationReason
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = "healthy"
	}
	switch input.Status {
	case "healthy", "limited", "offline", "maintenance":
	default:
		return nil, errors.New("invalid pool status")
	}
	platformFeePercent, err := s.loadSharedPoolDefaultPlatformFeePercent(ctx)
	if err != nil {
		return nil, err
	}
	input.PlatformFeePercent = platformFeePercent
	if shouldProbeSharedPoolBeforeListing(input.Listed, input.Status) && requiresSharedPoolPublishProbe(input.VerificationMode) {
		// Creation stays responsive and produces a draft. The durable background
		// probe job owns evidence persistence and the eventual listing transition.
		input.Listed = false
	}
	pool, err := s.repo.CreateSharedPoolTx(ctx, input)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_create")
	return pool, nil
}

func sharedPoolModelInputsFromCurrent(configs []SharedPoolModelConfig) []SharedPoolModelInput {
	out := make([]SharedPoolModelInput, 0, len(configs))
	for _, cfg := range configs {
		out = append(out, SharedPoolModelInput{
			Provider:                  cfg.Provider,
			ModelName:                 cfg.ModelName,
			UpstreamModelName:         cfg.UpstreamModelName,
			RateMultiplier:            cfg.RateMultiplier,
			FiveHourProtectionPercent: cfg.FiveHourProtectionPercent,
			SevenDayProtectionPercent: cfg.SevenDayProtectionPercent,
			DailyProtectionPercent:    cfg.DailyProtectionPercent,
			MaxConcurrency:            cfg.MaxConcurrency,
			ModelOpen:                 cfg.ModelOpen,
		})
	}
	return out
}

func cloneOptionalInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func mergeSharedPoolUpdateInput(current *SharedPool, patch UpdateSharedPoolInput) UpdateSharedPoolInput {
	input := patch
	if !patch.NameSet {
		input.Name = current.Name
	}
	if !patch.DescriptionSet {
		input.Description = current.Description
	}
	if !patch.AvatarURLSet {
		input.AvatarURL = current.AvatarURL
	}
	if !patch.StatusNoteSet {
		input.StatusNote = current.StatusNote
	}
	if !patch.DisabledReasonSet {
		input.DisabledReason = current.DisabledReason
	}
	if !patch.UpstreamBaseURLSet {
		input.UpstreamBaseURL = current.UpstreamBaseURL
	}
	if !patch.UpstreamAPIKeySet {
		// The owner view never exposes the stored secret. An empty value tells the
		// repository to retain the existing encrypted credential.
		input.UpstreamAPIKey = ""
	}
	if !patch.ModelsSet && !patch.ModelConfigsSet {
		input.Models = append([]string(nil), current.Models...)
		input.ModelConfigs = sharedPoolModelInputsFromCurrent(current.ModelConfigs)
	} else if patch.ModelsSet && !patch.ModelConfigsSet {
		input.ModelConfigs = nil
	} else if !patch.ModelsSet && patch.ModelConfigsSet {
		input.Models = nil
	}
	if !patch.RateMultiplierSet {
		input.RateMultiplier = current.RateMultiplier
	}
	if !patch.MaxUsersSet {
		input.MaxUsers = current.MaxUsers
	}
	if !patch.MinBalanceAdmissionSet {
		input.MinBalanceAdmission = current.MinBalanceAdmission
	}
	if !patch.HourlySeatFeeSet {
		input.HourlySeatFee = current.HourlySeatFee
	}
	if !patch.HourlyMinUsageWaiverSet {
		input.HourlyMinUsageWaiver = current.HourlyMinUsageWaiver
	}
	if !patch.ProxyIDSet {
		input.ProxyID = cloneOptionalInt64(current.ProxyID)
	}
	if !patch.ProxyURLSet {
		input.ProxyURL = current.ProxyURL
	}
	if !patch.ProxyRegionSet {
		input.ProxyRegion = current.ProxyRegion
	}
	if !patch.ProxyStatusSet {
		input.ProxyStatus = current.ProxyStatus
	}
	if !patch.AccountConcurrencySet {
		input.AccountConcurrency = current.AccountConcurrency
	}
	if !patch.UserConcurrencySet {
		input.UserConcurrency = current.UserConcurrency
	}
	if !patch.AccountModeSet {
		input.AccountModeEnabled = current.AccountModeEnabled
	}
	if !patch.OAuthProviderSet {
		input.OAuthProvider = current.OAuthProvider
	}
	if !patch.VerificationModeSet {
		input.VerificationMode = current.VerificationMode
	}
	if !patch.VerificationReasonSet {
		input.VerificationExemptionReason = current.VerificationExemptionReason
	}
	if !patch.ProbeModelSet {
		if !patch.ModelsSet && !patch.ModelConfigsSet && len(current.Models) > 0 {
			input.ProbeModel = current.Models[0]
		} else {
			// A model-set PATCH must not carry an obsolete probe model into the
			// replacement list. The normalized replacement picks it below.
			input.ProbeModel = ""
		}
	}
	if !patch.ListedSet {
		input.Listed = current.Listed
	}
	if !patch.StatusSet {
		input.Status = current.Status
	}
	return input
}

func sameOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sameSharedPoolRoutingModels(current []SharedPoolModelConfig, next []SharedPoolModelInput) bool {
	if len(current) != len(next) {
		return false
	}
	normalize := func(value string) string {
		return strings.ToLower(strings.TrimSpace(value))
	}
	normalizeProvider := func(value string) string {
		provider := normalize(value)
		if provider == "" {
			return "openai"
		}
		return provider
	}
	for index := range current {
		if normalizeProvider(current[index].Provider) != normalizeProvider(next[index].Provider) ||
			normalize(current[index].ModelName) != normalize(next[index].ModelName) ||
			normalize(current[index].UpstreamModelName) != normalize(next[index].UpstreamModelName) ||
			current[index].ModelOpen != next[index].ModelOpen {
			return false
		}
	}
	return true
}

func effectiveSharedPoolOAuthProvider(value string) string {
	provider := strings.ToLower(strings.TrimSpace(value))
	if provider == "" {
		return "openai"
	}
	return provider
}

func effectiveSharedPoolVerificationMode(value string) string {
	mode := strings.ToLower(strings.TrimSpace(value))
	if mode == "" {
		return "full_check"
	}
	return mode
}

func sharedPoolGateConfigurationChanged(current *SharedPool, input UpdateSharedPoolInput) bool {
	if current == nil {
		return true
	}
	if current.AccountModeEnabled != input.AccountModeEnabled ||
		strings.TrimRight(strings.TrimSpace(current.UpstreamBaseURL), "/") != input.UpstreamBaseURL ||
		!sameOptionalInt64(current.ProxyID, input.ProxyID) ||
		strings.TrimSpace(current.ProxyURL) != input.ProxyURL ||
		effectiveSharedPoolOAuthProvider(current.OAuthProvider) != effectiveSharedPoolOAuthProvider(input.OAuthProvider) ||
		effectiveSharedPoolVerificationMode(current.VerificationMode) != effectiveSharedPoolVerificationMode(input.VerificationMode) ||
		strings.TrimSpace(current.VerificationExemptionReason) != input.VerificationExemptionReason ||
		!sameSharedPoolRoutingModels(current.ModelConfigs, input.ModelConfigs) {
		return true
	}
	return input.UpstreamAPIKeySet && strings.TrimSpace(input.UpstreamAPIKey) != ""
}

func (s *BizDecipherService) UpdateSharedPool(ctx context.Context, poolID, ownerID int64, patch UpdateSharedPoolInput) (*SharedPool, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	current, err := s.repo.GetOwnedSharedPool(ctx, poolID, ownerID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, sql.ErrNoRows
	}
	input := mergeSharedPoolUpdateInput(current, patch)
	if err := guardNativeR1PoolUpdate(current, input); err != nil {
		return nil, err
	}
	if patch.ExpectedConfigVersion > 0 {
		input.ExpectedConfigVersion = patch.ExpectedConfigVersion
	} else {
		// Keep the transaction race fence for pre-versioned internal callers.
		// The owner HTTP endpoint requires the version loaded by the browser.
		input.ExpectedConfigVersion = current.ConfigVersion
	}
	if err := normalizeSharedPoolBranding(sharedPoolBrandingFields{
		Name:           &input.Name,
		Description:    &input.Description,
		AvatarURL:      &input.AvatarURL,
		StatusNote:     &input.StatusNote,
		DisabledReason: &input.DisabledReason,
	}); err != nil {
		return nil, err
	}
	input.UpstreamBaseURL = strings.TrimRight(strings.TrimSpace(input.UpstreamBaseURL), "/")
	input.UpstreamAPIKey = strings.TrimSpace(input.UpstreamAPIKey)
	input.ProxyURL = strings.TrimSpace(input.ProxyURL)
	input.ProxyRegion = strings.TrimSpace(input.ProxyRegion)
	input.ProxyStatus = strings.TrimSpace(input.ProxyStatus)
	input.OAuthProvider = strings.TrimSpace(input.OAuthProvider)
	input.ModelConfigs = normalizeSharedPoolModelInputs(input.ModelConfigs, input.Models, input.RateMultiplier)
	input.Models = openSharedPoolModelInputNames(input.ModelConfigs)
	if !patch.ProbeModelSet && (patch.ModelsSet || patch.ModelConfigsSet) && len(input.Models) > 0 {
		input.ProbeModel = input.Models[0]
	}
	input.Status = strings.TrimSpace(input.Status)
	if input.Name == "" {
		return nil, errors.New("pool name is required")
	}
	if input.AccountModeEnabled {
		input.UpstreamBaseURL = ""
		input.UpstreamAPIKey = ""
		// Account-mode pools may list after at least one account passes the
		// capability gate. Do not force listed=false here; validate below.
	} else if input.UpstreamBaseURL == "" {
		return nil, errors.New("upstream base url is required")
	}
	if len(input.Models) == 0 {
		if !input.AccountModeEnabled {
			return nil, errors.New("at least one model is required")
		}
		input.ProbeModel = ""
		input.ModelConfigs = nil
	} else {
		probeModel, err := resolveSharedPoolProbeModel(input.ProbeModel, input.ModelConfigs, input.Models)
		if err != nil {
			return nil, err
		}
		input.ProbeModel = probeModel
		input.ModelConfigs = preferSharedPoolProbeModel(input.ModelConfigs, probeModel)
	}
	if input.RateMultiplier <= 0 {
		input.RateMultiplier = 1
	}
	if input.MaxUsers <= 0 {
		input.MaxUsers = 20
	}
	if input.MinBalanceAdmission < 0 {
		input.MinBalanceAdmission = 0
	}
	if input.HourlySeatFee < 0 {
		input.HourlySeatFee = 0
	}
	if input.HourlyMinUsageWaiver < 0 {
		input.HourlyMinUsageWaiver = 0
	}
	if input.AccountConcurrency <= 0 {
		input.AccountConcurrency = 1
	}
	if input.UserConcurrency <= 0 {
		input.UserConcurrency = 1
	}
	if input.OAuthProvider == "" {
		input.OAuthProvider = "openai"
	}
	verificationMode, verificationReason, err := s.resolveSharedPoolVerificationPolicy(ctx, input.ModelConfigs, input.VerificationMode, input.VerificationExemptionReason)
	if err != nil {
		return nil, err
	}
	input.VerificationMode = verificationMode
	input.VerificationExemptionReason = verificationReason
	if input.Status == "" {
		input.Status = "healthy"
	}
	switch input.Status {
	case "healthy", "limited", "offline", "maintenance":
	default:
		return nil, errors.New("invalid pool status")
	}
	gateConfigurationChanged := sharedPoolGateConfigurationChanged(current, input)
	if current.Listed && gateConfigurationChanged {
		// Saving a changed upstream/routing contract must succeed without keeping
		// stale marketplace readiness. The existing async full-check flow can
		// relist it after the new configuration passes.
		input.Listed = false
	}
	if input.Listed && !SharedPoolGovernanceAllowsOwnerListing(current.GovernanceStatus) {
		if !current.Listed {
			return nil, ErrSharedPoolGovernanceBlocked
		}
		// Repair legacy/inconsistent visibility on the next owner save.
		input.Listed = false
	}
	listingTransition := !current.Listed && input.Listed
	if listingTransition && !shouldProbeSharedPoolBeforeListing(true, input.Status) {
		return nil, errors.New("pool status must be healthy or limited before listing")
	}
	if input.AccountModeEnabled && listingTransition {
		if err := s.ensureAccountModePoolListable(ctx, poolID, ownerID, input.VerificationMode); err != nil {
			return nil, err
		}
	}
	// Account-mode listing is gated by account probes, not pool-level API keys.
	probeRequired := !input.AccountModeEnabled && listingTransition && requiresSharedPoolPublishProbe(input.VerificationMode)
	if probeRequired {
		return nil, ErrSharedPoolProbeRequired
	}
	pool, err := s.repo.UpdateSharedPoolTx(ctx, poolID, ownerID, input)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_update")
	return pool, nil
}

func (s *BizDecipherService) ListMySharedPools(ctx context.Context, ownerID int64) ([]SharedPool, error) {
	if s == nil || s.repo == nil {
		return []SharedPool{}, nil
	}
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	pools, err := s.repo.ListMySharedPools(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	s.enrichSharedPoolOfficialPricing(pools)
	return pools, nil
}

func (s *BizDecipherService) enrichSharedPoolOfficialPricing(pools []SharedPool) {
	for poolIndex := range pools {
		for modelIndex := range pools[poolIndex].ModelConfigs {
			model := &pools[poolIndex].ModelConfigs[modelIndex]
			// Repository/channel snapshots are never trusted here. A model that
			// cannot be resolved inside model_catalog's provider boundary remains
			// unpriced, and a canonical BillingService failure remains hidden.
			model.Pricing = nil
			if canonicalModel := strings.TrimSpace(model.CanonicalModelName); canonicalModel != "" {
				model.Pricing = s.canonicalSharedPoolPriceSnapshot(canonicalModel)
			}
		}
	}
}

func (s *BizDecipherService) DeleteSharedPool(ctx context.Context, poolID, ownerID int64) error {
	if s == nil || s.repo == nil {
		return errors.New("service unavailable")
	}
	if poolID <= 0 {
		return errors.New("invalid pool id")
	}
	if ownerID <= 0 {
		return errors.New("invalid owner id")
	}
	if err := s.repo.DeleteSharedPoolTx(ctx, poolID, ownerID); err != nil {
		return err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_archive")
	return nil
}

func (s *BizDecipherService) AdminRestoreSharedPool(ctx context.Context, poolID, adminUserID int64, reason, operationID string) (*SharedPool, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	if adminUserID <= 0 {
		return nil, errors.New("invalid admin user id")
	}
	pool, err := s.repo.RestoreSharedPoolTx(ctx, poolID, adminUserID, reason, operationID)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_restore")
	return pool, nil
}

func (s *BizDecipherService) ListSharedPoolAccounts(ctx context.Context, poolID, ownerID int64) ([]SharedPoolAccount, error) {
	if s == nil || s.repo == nil {
		return []SharedPoolAccount{}, nil
	}
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	return s.repo.ListSharedPoolAccounts(ctx, poolID, ownerID)
}

func (s *BizDecipherService) CreateSharedPoolAccount(ctx context.Context, input SharedPoolAccountInput) (*SharedPoolAccount, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if !input.SchedulableSet {
		input.Schedulable = true
		input.SchedulableSet = true
	}
	if !input.AutoPauseOnExpiredSet {
		input.AutoPauseOnExpired = true
		input.AutoPauseOnExpiredSet = true
	}
	if err := normalizeSharedPoolAccountInput(&input); err != nil {
		return nil, err
	}
	if input.AuthType == AccountTypeOAuth {
		if strings.TrimSpace(input.CredentialsEncrypted) == "" {
			return nil, errors.New("oauth credentials are required")
		}
	} else if input.UpstreamAPIKey == "" {
		return nil, errors.New("upstream api key is required")
	}
	pool, err := s.repo.GetOwnedSharedPool(ctx, input.PoolID, input.OwnerID)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, sql.ErrNoRows
	}
	if isNativeR1Pool(pool) {
		return nil, ErrNativePoolImmutable
	}
	account, err := s.repo.CreateSharedPoolAccount(ctx, input)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_account_create")
	return account, nil
}

func (s *BizDecipherService) UpdateSharedPoolAccount(ctx context.Context, accountID int64, input SharedPoolAccountInput) (*SharedPoolAccount, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if accountID <= 0 {
		return nil, errors.New("invalid account id")
	}
	accounts, err := s.repo.ListSharedPoolAccounts(ctx, input.PoolID, input.OwnerID)
	if err != nil {
		return nil, err
	}
	for index := range accounts {
		if accounts[index].ID == accountID && accounts[index].NativeBindingState != "" {
			return nil, ErrNativePoolImmutable
		}
	}
	if err := normalizeSharedPoolAccountInput(&input); err != nil {
		return nil, err
	}
	account, err := s.repo.UpdateSharedPoolAccount(ctx, accountID, input)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_account_update")
	return account, nil
}

func (s *BizDecipherService) DeleteSharedPoolAccount(ctx context.Context, poolID, accountID, ownerID int64) error {
	if s == nil || s.repo == nil {
		return errors.New("service unavailable")
	}
	if poolID <= 0 {
		return errors.New("invalid pool id")
	}
	if accountID <= 0 {
		return errors.New("invalid account id")
	}
	if ownerID <= 0 {
		return errors.New("invalid owner id")
	}
	accounts, err := s.repo.ListSharedPoolAccounts(ctx, poolID, ownerID)
	if err != nil {
		return err
	}
	for index := range accounts {
		if accounts[index].ID == accountID && accounts[index].NativeBindingState != "" {
			if s.nativeOnboardingRepo == nil {
				return ErrNativeOnboardingUnavailable
			}
			return s.nativeOnboardingRepo.DetachNativeBinding(ctx, poolID, ownerID, accountID)
		}
	}
	if err := s.repo.DeleteSharedPoolAccount(ctx, poolID, accountID, ownerID); err != nil {
		return err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_account_delete")
	return nil
}

func (s *BizDecipherService) ImportSharedPoolAccounts(ctx context.Context, input ImportSharedPoolAccountsInput) (*SharedPoolAccountImportResult, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if input.PoolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	if input.OwnerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	result := &SharedPoolAccountImportResult{Total: len(input.Items), Items: make([]SharedPoolAccountImportItemResult, 0, len(input.Items))}
	if len(input.Items) == 0 {
		return result, nil
	}
	if len(input.Items) > 100 {
		return nil, errors.New("shared pool account import supports at most 100 items per batch")
	}
	for index, item := range input.Items {
		item.PoolID = input.PoolID
		item.OwnerID = input.OwnerID
		itemResult := SharedPoolAccountImportItemResult{Index: index, Name: strings.TrimSpace(item.Name)}
		account, err := s.CreateSharedPoolAccount(ctx, item)
		if err != nil {
			itemResult.Error = err.Error()
			result.Failed++
		} else {
			itemResult.Created = true
			itemResult.Account = account
			result.Created++
		}
		result.Items = append(result.Items, itemResult)
	}
	return result, nil
}

func (s *BizDecipherService) ImportSharedPoolOAuthPackage(ctx context.Context, input ImportSharedPoolOAuthPackageInput) (*SharedPoolOAuthImportResult, error) {
	if s == nil || s.repo == nil || s.secretEncryptor == nil {
		return nil, errors.New("shared pool oauth import is unavailable")
	}
	if input.PoolID <= 0 || input.OwnerID <= 0 {
		return nil, errors.New("invalid pool or owner id")
	}
	if input.Data.Type != "" && input.Data.Type != "sub2api-data" {
		return nil, errors.New("unsupported account data package type")
	}
	if len(input.Data.Accounts) > 100 {
		return nil, errors.New("shared pool oauth import supports at most 100 accounts per batch")
	}
	result := &SharedPoolOAuthImportResult{Total: len(input.Data.Accounts), Items: make([]SharedPoolOAuthImportItemResult, 0, len(input.Data.Accounts))}
	seen := map[string]int{}
	for index, item := range input.Data.Accounts {
		name := strings.TrimSpace(item.Name)
		itemResult := SharedPoolOAuthImportItemResult{Index: index, Name: name}
		accountInput, fingerprint, warnings, err := s.sharedPoolOAuthAccountInput(input.PoolID, input.OwnerID, item)
		for _, warning := range warnings {
			result.Warnings = append(result.Warnings, SharedPoolOAuthImportMessage{Index: index, Name: name, Message: warning})
		}
		if err != nil {
			itemResult.Action = "failed"
			itemResult.Message = err.Error()
			result.Failed++
			result.Errors = append(result.Errors, SharedPoolOAuthImportMessage{Index: index, Name: name, Message: err.Error()})
			result.Items = append(result.Items, itemResult)
			continue
		}
		if previous, ok := seen[fingerprint]; ok {
			message := fmt.Sprintf("duplicate of import item %d", previous)
			itemResult.Action = "skipped"
			itemResult.Message = message
			result.Skipped++
			result.Warnings = append(result.Warnings, SharedPoolOAuthImportMessage{Index: index, Name: name, Message: message})
			result.Items = append(result.Items, itemResult)
			continue
		}
		seen[fingerprint] = index
		existing, findErr := s.repo.FindSharedPoolAccountByFingerprint(ctx, input.PoolID, input.OwnerID, fingerprint)
		if findErr != nil && !errors.Is(findErr, sql.ErrNoRows) {
			return nil, findErr
		}
		if existing != nil {
			if !input.UpdateExisting {
				itemResult.Action = "skipped"
				itemResult.AccountID = existing.ID
				itemResult.Message = "matching account already exists"
				result.Skipped++
				result.Items = append(result.Items, itemResult)
				continue
			}
			accountInput.Schedulable = existing.Schedulable
			accountInput.SchedulableSet = true
			accountInput.Status = existing.Status
			accountInput.StatusNote = existing.StatusNote
			accountInput.DisabledReason = existing.DisabledReason
			accountInput.AutoPauseOnExpiredSet = true
			updated, updateErr := s.UpdateSharedPoolAccount(ctx, existing.ID, accountInput)
			if updateErr != nil {
				itemResult.Action = "failed"
				itemResult.Message = updateErr.Error()
				result.Failed++
				result.Errors = append(result.Errors, SharedPoolOAuthImportMessage{Index: index, Name: name, Message: updateErr.Error()})
			} else {
				itemResult.Action = "updated"
				itemResult.AccountID = updated.ID
				result.Updated++
			}
			result.Items = append(result.Items, itemResult)
			continue
		}
		created, createErr := s.CreateSharedPoolAccount(ctx, accountInput)
		if createErr != nil {
			itemResult.Action = "failed"
			itemResult.Message = createErr.Error()
			result.Failed++
			result.Errors = append(result.Errors, SharedPoolOAuthImportMessage{Index: index, Name: name, Message: createErr.Error()})
		} else {
			itemResult.Action = "created"
			itemResult.AccountID = created.ID
			result.Created++
		}
		result.Items = append(result.Items, itemResult)
	}
	return result, nil
}

func (s *BizDecipherService) sharedPoolOAuthAccountInput(poolID, ownerID int64, item SharedPoolOAuthDataAccount) (SharedPoolAccountInput, string, []string, error) {
	platform := strings.ToLower(strings.TrimSpace(item.Platform))
	accountType := strings.ToLower(strings.TrimSpace(item.Type))
	if platform != PlatformOpenAI || accountType != AccountTypeOAuth {
		return SharedPoolAccountInput{}, "", nil, errors.New("only OpenAI OAuth accounts are supported")
	}
	credentials := copyStringAnyMap(item.Credentials)
	accessToken := strings.TrimSpace(stringValue(credentials["access_token"]))
	if accessToken == "" {
		return SharedPoolAccountInput{}, "", nil, errors.New("oauth access_token is required")
	}
	accountID := strings.TrimSpace(firstNonEmpty(stringValue(credentials["chatgpt_account_id"]), stringValue(credentials["account_id"])))
	userID := strings.TrimSpace(firstNonEmpty(stringValue(credentials["chatgpt_user_id"]), stringValue(credentials["user_id"])))
	email := strings.ToLower(strings.TrimSpace(stringValue(credentials["email"])))
	// Prefer nested OpenAI JWT claims from access_token / id_token so multi-seat
	// packages that share chatgpt_account_id still get distinct identities.
	if claims := parseUnverifiedJWTClaims(accessToken); claims != nil {
		claimAccountID, claimUserID, claimEmail := extractOpenAIOAuthIdentityClaims(claims)
		accountID = firstNonEmpty(accountID, claimAccountID)
		userID = firstNonEmpty(userID, claimUserID)
		email = firstNonEmpty(email, claimEmail)
	}
	if accountID == "" || userID == "" || email == "" {
		if claims := parseUnverifiedJWTClaims(stringValue(credentials["id_token"])); claims != nil {
			claimAccountID, claimUserID, claimEmail := extractOpenAIOAuthIdentityClaims(claims)
			accountID = firstNonEmpty(accountID, claimAccountID)
			userID = firstNonEmpty(userID, claimUserID)
			email = firstNonEmpty(email, claimEmail)
		}
	}
	// Persist normalized identity fields so later probe/runtime paths can read them
	// without re-parsing the JWT.
	if accountID != "" {
		credentials["account_id"] = accountID
		credentials["chatgpt_account_id"] = accountID
	}
	if userID != "" {
		credentials["user_id"] = userID
		credentials["chatgpt_user_id"] = userID
	}
	if email != "" {
		credentials["email"] = email
	}
	// Stable identity prioritizes the unique login subject (user/email) over the
	// shared chatgpt_account_id. Packages that only provide account_id still work.
	identity := sharedPoolOAuthStableIdentity(accountID, userID, email)
	if identity == "" {
		return SharedPoolAccountInput{}, "", nil, errors.New("oauth account has no stable identity; include account_id, user_id, email, or a JWT with OpenAI identity claims before importing")
	}
	fingerprintSum := sha256.Sum256([]byte("openai|oauth|" + identity))
	fingerprint := hex.EncodeToString(fingerprintSum[:])
	encoded, err := json.Marshal(credentials)
	if err != nil {
		return SharedPoolAccountInput{}, "", nil, errors.New("oauth credentials are invalid")
	}
	encrypted, err := s.secretEncryptor.Encrypt(string(encoded))
	if err != nil {
		return SharedPoolAccountInput{}, "", nil, fmt.Errorf("encrypt oauth credentials: %w", err)
	}
	var expiresAt *time.Time
	if item.ExpiresAt != nil && *item.ExpiresAt > 0 {
		t := time.Unix(*item.ExpiresAt, 0).UTC()
		expiresAt = &t
	} else if raw := strings.TrimSpace(firstNonEmpty(stringValue(credentials["expires_at"]), stringValue(credentials["expired"]))); raw != "" {
		if parsed, parseErr := parseSharedPoolOAuthExpiry(raw); parseErr == nil {
			expiresAt = &parsed
		}
	}
	if expiresAt == nil {
		if claims := parseUnverifiedJWTClaims(accessToken); claims != nil {
			if exp, ok := claimUnixTime(claims["exp"]); ok {
				expiresAt = &exp
			}
		}
	}
	autoPause := true
	if item.AutoPauseOnExpired != nil {
		autoPause = *item.AutoPauseOnExpired
	}
	warnings := []string{}
	if strings.TrimSpace(stringValue(credentials["refresh_token"])) == "" {
		warnings = append(warnings, "no refresh_token; the account will stop scheduling after access token expiry")
		autoPause = true
	}
	if expiresAt == nil && autoPause {
		warnings = append(warnings, "token expiry is unknown; re-import before the access token expires")
	}
	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = firstNonEmpty(email, userID, accountID, "OpenAI OAuth account")
	}
	concurrency := item.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	priority := item.Priority
	if priority <= 0 {
		priority = 100
	}
	modelName := strings.TrimSpace(firstNonEmpty(stringValue(item.Extra["model"]), "gpt-5.4"))
	return SharedPoolAccountInput{
		PoolID: poolID, OwnerID: ownerID, Name: name, Provider: PlatformOpenAI, AuthType: AccountTypeOAuth,
		UpstreamBaseURL: "https://chatgpt.com/backend-api/codex", CredentialsEncrypted: encrypted,
		CredentialFingerprint: fingerprint, ExpiresAt: expiresAt, AutoPauseOnExpired: autoPause, AutoPauseOnExpiredSet: true, Schedulable: true, SchedulableSet: true,
		Status: "testing", AccountWeight: 1, Priority: priority, AccountConcurrency: concurrency, UserConcurrency: 1,
		ModelConfigs: []SharedPoolModelInput{{Provider: PlatformOpenAI, ModelName: modelName, RateMultiplier: 1, MaxConcurrency: concurrency, ModelOpen: true}},
		GateRequired: true, GatePassed: false, CachePolicy: json.RawMessage(`{}`), RoutingPolicy: json.RawMessage(`{}`),
	}, fingerprint, warnings, nil
}

func sharedPoolOAuthStableIdentity(accountID, userID, email string) string {
	accountID = strings.TrimSpace(accountID)
	userID = strings.TrimSpace(userID)
	email = strings.ToLower(strings.TrimSpace(email))
	// Prefer unique login subject. Shared chatgpt_account_id alone is too coarse
	// for multi-seat / k12 style packages where many users share one account_id.
	if userID != "" {
		if accountID != "" {
			return accountID + "|user:" + userID
		}
		return "user:" + userID
	}
	if email != "" {
		if accountID != "" {
			return accountID + "|email:" + email
		}
		return "email:" + email
	}
	return accountID
}

func extractOpenAIOAuthIdentityClaims(claims map[string]any) (accountID, userID, email string) {
	if claims == nil {
		return "", "", ""
	}
	accountID = strings.TrimSpace(firstNonEmpty(stringValue(claims["chatgpt_account_id"]), stringValue(claims["account_id"])))
	userID = strings.TrimSpace(firstNonEmpty(stringValue(claims["chatgpt_user_id"]), stringValue(claims["user_id"]), stringValue(claims["sub"])))
	email = strings.ToLower(strings.TrimSpace(stringValue(claims["email"])))
	if auth, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
		accountID = firstNonEmpty(accountID, strings.TrimSpace(firstNonEmpty(stringValue(auth["chatgpt_account_id"]), stringValue(auth["account_id"]))))
		userID = firstNonEmpty(userID, strings.TrimSpace(firstNonEmpty(stringValue(auth["chatgpt_user_id"]), stringValue(auth["user_id"]))))
	}
	if profile, ok := claims["https://api.openai.com/profile"].(map[string]any); ok {
		email = firstNonEmpty(email, strings.ToLower(strings.TrimSpace(stringValue(profile["email"]))))
	}
	return accountID, userID, email
}

func parseSharedPoolOAuthExpiry(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("empty expiry")
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC(), nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed.UTC(), nil
	}
	// Some exporters use "2006-01-02T15:04:05.000Z" without timezone offset variants.
	if parsed, err := time.Parse("2006-01-02T15:04:05.000Z", raw); err == nil {
		return parsed.UTC(), nil
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil && unix > 0 {
		return time.Unix(unix, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unsupported oauth expiry format: %s", raw)
}

func claimUnixTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case float64:
		if typed <= 0 {
			return time.Time{}, false
		}
		return time.Unix(int64(typed), 0).UTC(), true
	case json.Number:
		unix, err := typed.Int64()
		if err != nil || unix <= 0 {
			return time.Time{}, false
		}
		return time.Unix(unix, 0).UTC(), true
	case string:
		unix, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil || unix <= 0 {
			return time.Time{}, false
		}
		return time.Unix(unix, 0).UTC(), true
	default:
		return time.Time{}, false
	}
}

func parseUnverifiedJWTClaims(token string) map[string]any {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some exporters pad standard base64.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil
		}
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return nil
	}
	return claims
}

func copyStringAnyMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func (s *BizDecipherService) CreateSharedPoolAccessKey(ctx context.Context, poolID, userID int64, name string) (*SharedPoolAccessKeyResult, error) {
	if s == nil || s.repo == nil || s.apiKeyService == nil {
		return nil, errors.New("service unavailable")
	}
	if poolID <= 0 || userID <= 0 {
		return nil, errors.New("invalid pool or user id")
	}
	if pool, err := s.repo.GetSharedPool(ctx, poolID); err == nil && isNativeR1Pool(pool) && !isNativeBillingActive(pool) {
		return nil, ErrBillingActivationRequired
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Shared Pool Key"
	}
	rawKey, err := s.apiKeyService.GenerateKeyWithPrefix("sk-share-")
	if err != nil {
		return nil, err
	}
	return s.repo.CreateSharedPoolAccessKeyTx(ctx, poolID, userID, name, rawKey)
}

func (s *BizDecipherService) ListMySharedPoolAccessKeys(ctx context.Context, userID int64) ([]SharedPoolAccessKey, error) {
	if s == nil || s.repo == nil {
		return []SharedPoolAccessKey{}, nil
	}
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	return s.repo.ListMySharedPoolAccessKeys(ctx, userID)
}

func (s *BizDecipherService) DeleteSharedPoolAccessKey(ctx context.Context, apiKeyID, userID int64) error {
	if s == nil || s.apiKeyService == nil {
		return errors.New("service unavailable")
	}
	if apiKeyID <= 0 || userID <= 0 {
		return errors.New("invalid shared pool key or user id")
	}
	return s.apiKeyService.DeleteSharedPoolManaged(ctx, apiKeyID, userID)
}

func (s *BizDecipherService) GetSharedPoolAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel string) (*SharedPoolAccessKey, error) {
	if s == nil || s.repo == nil || apiKeyID <= 0 {
		return nil, nil
	}
	accessKey, err := s.repo.GetSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel)
	if err != nil || accessKey == nil {
		return accessKey, err
	}
	return s.hydrateSharedPoolAccessKeyCredentials(accessKey)
}

func (s *BizDecipherService) GetSharedPoolCompactAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel string) (*SharedPoolAccessKey, error) {
	if s == nil || s.repo == nil || apiKeyID <= 0 || strings.TrimSpace(reqModel) == "" {
		return nil, nil
	}
	accessKey, err := s.repo.GetSharedPoolCompactAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel)
	if err != nil || accessKey == nil {
		return accessKey, err
	}
	return s.hydrateSharedPoolAccessKeyCredentials(accessKey)
}

func (s *BizDecipherService) GetSharedPoolMediaAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel, endpointType string) (*SharedPoolAccessKey, error) {
	if s == nil || s.repo == nil || apiKeyID <= 0 {
		return nil, nil
	}
	accessKey, err := s.repo.GetSharedPoolMediaAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel, endpointType)
	if err != nil || accessKey == nil {
		return accessKey, err
	}
	return s.hydrateSharedPoolAccessKeyCredentials(accessKey)
}

func (s *BizDecipherService) hydrateSharedPoolAccessKeyCredentials(accessKey *SharedPoolAccessKey) (*SharedPoolAccessKey, error) {
	if accessKey == nil {
		return accessKey, nil
	}
	proxyURL, _, err := parseSharedPoolProxyURL(accessKey.ProxyURL)
	if err != nil {
		return nil, err
	}
	accessKey.ProxyURL = proxyURL
	if strings.TrimSpace(accessKey.OAuthCredentialsEncrypted) == "" {
		return accessKey, nil
	}
	if s.secretEncryptor == nil {
		return nil, errors.New("shared pool oauth credential decryptor is unavailable")
	}
	plaintext, err := s.secretEncryptor.Decrypt(accessKey.OAuthCredentialsEncrypted)
	if err != nil {
		return nil, errors.New("shared pool oauth credentials could not be decrypted")
	}
	credentials := map[string]any{}
	if err := json.Unmarshal([]byte(plaintext), &credentials); err != nil {
		return nil, errors.New("shared pool oauth credentials are invalid")
	}
	accessKey.OAuthCredentials = credentials
	return accessKey, nil
}

func (s *BizDecipherService) RecordSharedPoolUsage(ctx context.Context, input SharedPoolUsageInput) error {
	if s == nil || s.repo == nil {
		return nil
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.Success && input.RequestID == "" {
		return errors.New("successful shared pool usage requires a request id")
	}
	if input.AccessKeyID <= 0 || input.PoolID <= 0 || input.UserID <= 0 {
		return nil
	}
	if input.Cost < 0 {
		input.Cost = 0
	}
	input.Cost = normalizedSharedPoolMoney(input.Cost)
	if input.Success {
		if input.PriceVersionID <= 0 {
			return errors.New("successful shared pool usage requires an accepted price version")
		}
		if input.PricingSource != SharedPoolPricingSourceOfficial && input.PricingSource != SharedPoolPricingSourceOwner {
			return errors.New("successful shared pool usage requires a valid pricing source")
		}
	}
	if len(input.PriceSnapshot) == 0 {
		input.PriceSnapshot = json.RawMessage(`{}`)
	}
	return s.repo.RecordSharedPoolUsageTx(ctx, input)
}

func (s *BizDecipherService) ReportSharedPool(ctx context.Context, poolID, userID int64, reason string) (*SharedPool, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if poolID <= 0 || userID <= 0 {
		return nil, errors.New("invalid pool or user id")
	}
	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > 500 {
		reason = string([]rune(reason)[:500])
	}
	pool, err := s.repo.ReportSharedPoolTx(ctx, poolID, userID, reason)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_report")
	return pool, nil
}

func (s *BizDecipherService) LikeSharedPool(ctx context.Context, poolID, userID int64) (*SharedPool, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if poolID <= 0 || userID <= 0 {
		return nil, errors.New("invalid pool or user id")
	}
	pool, err := s.repo.LikeSharedPoolTx(ctx, poolID, userID)
	if err != nil {
		return nil, err
	}
	if pool != nil {
		pool.LikedByMe = true
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_like")
	return pool, nil
}

func (s *BizDecipherService) UnlikeSharedPool(ctx context.Context, poolID, userID int64) (*SharedPool, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if poolID <= 0 || userID <= 0 {
		return nil, errors.New("invalid pool or user id")
	}
	pool, err := s.repo.UnlikeSharedPoolTx(ctx, poolID, userID)
	if err != nil {
		return nil, err
	}
	if pool != nil {
		pool.LikedByMe = false
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_unlike")
	return pool, nil
}

func (s *BizDecipherService) AttachSharedPoolLikeState(ctx context.Context, userID int64, pools []SharedPool) []SharedPool {
	if s == nil || s.repo == nil || userID <= 0 || len(pools) == 0 {
		return pools
	}
	ids := make([]int64, 0, len(pools))
	for i := range pools {
		ids = append(ids, pools[i].ID)
	}
	liked, err := s.repo.ListSharedPoolLikedIDs(ctx, userID, ids)
	if err != nil || liked == nil {
		return pools
	}
	for i := range pools {
		pools[i].LikedByMe = liked[pools[i].ID]
	}
	return pools
}

func (s *BizDecipherService) AttachSingleSharedPoolLikeState(ctx context.Context, userID int64, pool *SharedPool) *SharedPool {
	if pool == nil || userID <= 0 {
		return pool
	}
	out := s.AttachSharedPoolLikeState(ctx, userID, []SharedPool{*pool})
	if len(out) == 0 {
		return pool
	}
	return &out[0]
}

func normalizeSharedPoolModels(models []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(models))
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		key := strings.ToLower(m)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, m)
		if len(out) >= 100 {
			break
		}
	}
	return out
}

func openSharedPoolModelInputNames(configs []SharedPoolModelInput) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(configs))
	for _, cfg := range configs {
		if !cfg.ModelOpen {
			continue
		}
		model := strings.TrimSpace(cfg.ModelName)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, model)
		if len(out) >= 100 {
			break
		}
	}
	return out
}

func normalizeSharedPoolAccountInput(input *SharedPoolAccountInput) error {
	if input == nil {
		return errors.New("account input is required")
	}
	if input.PoolID <= 0 {
		return errors.New("invalid pool id")
	}
	if input.OwnerID <= 0 {
		return errors.New("invalid owner id")
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		input.Name = "Shared pool account"
	}
	input.Description = strings.TrimSpace(input.Description)
	input.Provider = strings.TrimSpace(input.Provider)
	if input.Provider == "" {
		input.Provider = "openai"
	}
	input.AuthType = strings.ToLower(strings.TrimSpace(input.AuthType))
	if input.AuthType == "" || input.AuthType == "api_key" {
		input.AuthType = AccountTypeAPIKey
	}
	if input.AuthType != AccountTypeAPIKey && input.AuthType != AccountTypeOAuth {
		return errors.New("unsupported shared pool account auth type")
	}
	input.UpstreamBaseURL = strings.TrimRight(strings.TrimSpace(input.UpstreamBaseURL), "/")
	if input.AuthType == AccountTypeAPIKey && input.UpstreamBaseURL == "" {
		return errors.New("upstream base url is required")
	}
	if input.AuthType == AccountTypeOAuth {
		if input.UpstreamBaseURL == "" {
			input.UpstreamBaseURL = "https://chatgpt.com/backend-api/codex"
		}
		// OAuth access tokens are never routable after expiry. Refresh is not yet
		// performed inside the shared-pool scheduler, so owner settings must not
		// keep an expired OAuth account eligible.
		input.AutoPauseOnExpired = true
		input.AutoPauseOnExpiredSet = true
	}
	input.UpstreamAPIKey = strings.TrimSpace(input.UpstreamAPIKey)
	input.CredentialsEncrypted = strings.TrimSpace(input.CredentialsEncrypted)
	input.CredentialFingerprint = strings.TrimSpace(input.CredentialFingerprint)
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = "active"
	}
	switch input.Status {
	case "active", "testing", "limited", "disabled", "offline", "expired":
	default:
		return errors.New("invalid shared pool account status")
	}
	input.StatusNote = strings.TrimSpace(input.StatusNote)
	input.DisabledReason = strings.TrimSpace(input.DisabledReason)
	input.GroupName = strings.TrimSpace(input.GroupName)
	input.ProxyURL = strings.TrimSpace(input.ProxyURL)
	input.ProxyRegion = strings.TrimSpace(input.ProxyRegion)
	input.ProxyStatus = strings.TrimSpace(input.ProxyStatus)
	if input.AccountWeight <= 0 {
		input.AccountWeight = 1
	}
	if input.Priority <= 0 {
		input.Priority = 100
	}
	if input.RPMLimit < 0 {
		input.RPMLimit = 0
	}
	if input.AccountConcurrency <= 0 {
		input.AccountConcurrency = 1
	}
	if input.UserConcurrency <= 0 {
		input.UserConcurrency = 1
	}
	if input.TTLSeconds < 0 {
		input.TTLSeconds = 0
	}
	input.ModelConfigs = normalizeSharedPoolModelInputs(input.ModelConfigs, nil, 1)
	if len(input.ModelConfigs) == 0 {
		return errors.New("at least one model is required")
	}
	if len(input.CachePolicy) == 0 {
		input.CachePolicy = json.RawMessage(`{}`)
	}
	if !json.Valid(input.CachePolicy) {
		return errors.New("cache_policy must be valid JSON")
	}
	if len(input.RoutingPolicy) == 0 {
		input.RoutingPolicy = json.RawMessage(`{}`)
	}
	if !json.Valid(input.RoutingPolicy) {
		return errors.New("routing_policy must be valid JSON")
	}
	if input.FullCheckScore < 0 {
		input.FullCheckScore = 0
	}
	if input.FullCheckScore > 100 {
		input.FullCheckScore = 100
	}
	if input.FullCheckPassed < 0 {
		input.FullCheckPassed = 0
	}
	if input.FullCheckTotal < 0 {
		input.FullCheckTotal = 0
	}
	return nil
}

func normalizeSharedPoolModelInputs(configs []SharedPoolModelInput, models []string, poolRate float64) []SharedPoolModelInput {
	seen := map[string]struct{}{}
	out := make([]SharedPoolModelInput, 0, len(configs)+len(models))
	appendConfig := func(cfg SharedPoolModelInput) {
		cfg.ModelName = strings.TrimSpace(cfg.ModelName)
		if cfg.ModelName == "" {
			return
		}
		key := strings.ToLower(cfg.ModelName)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		cfg.UpstreamModelName = strings.TrimSpace(cfg.UpstreamModelName)
		cfg.Provider = strings.TrimSpace(cfg.Provider)
		if cfg.Provider == "" {
			cfg.Provider = "openai"
		}
		if cfg.RateMultiplier <= 0 {
			cfg.RateMultiplier = poolRate
		}
		if cfg.RateMultiplier <= 0 {
			cfg.RateMultiplier = 1
		}
		if cfg.FiveHourProtectionPercent <= 0 || cfg.FiveHourProtectionPercent > 100 {
			cfg.FiveHourProtectionPercent = 100
		}
		if cfg.SevenDayProtectionPercent <= 0 || cfg.SevenDayProtectionPercent > 100 {
			cfg.SevenDayProtectionPercent = 100
		}
		if cfg.DailyProtectionPercent <= 0 || cfg.DailyProtectionPercent > 100 {
			cfg.DailyProtectionPercent = 100
		}
		out = append(out, cfg)
	}
	for _, cfg := range configs {
		appendConfig(cfg)
	}
	for _, model := range normalizeSharedPoolModels(models) {
		appendConfig(SharedPoolModelInput{ModelName: model, RateMultiplier: poolRate, FiveHourProtectionPercent: 100, SevenDayProtectionPercent: 100, DailyProtectionPercent: 100, ModelOpen: true})
	}
	if len(out) > 100 {
		return out[:100]
	}
	return out
}

// ---------------------------------------------------------------------------
// Pool seats (money-sensitive: join / leave / hourly billing / owner payout)
// ---------------------------------------------------------------------------

// JoinSharedPool takes (or re-uses) a seat in a shared pool. The repository
// transaction enforces all admission rules (listed, online, capacity, minimum
// balance) atomically; this method only validates the arguments.
func (s *BizDecipherService) JoinSharedPool(ctx context.Context, poolID, userID int64) (*JoinPoolResult, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("service unavailable")
	}
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	result, err := s.repo.JoinSharedPoolTx(ctx, poolID, userID)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_join")
	return result, nil
}

// LeaveSharedPool releases the user's active seat in a pool, settling any
// outstanding whole hours first.
func (s *BizDecipherService) LeaveSharedPool(ctx context.Context, poolID, userID int64) error {
	if s == nil || s.repo == nil {
		return errors.New("service unavailable")
	}
	if poolID <= 0 {
		return errors.New("invalid pool id")
	}
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	if err := s.repo.LeaveSharedPoolTx(ctx, poolID, userID); err != nil {
		return err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_leave")
	return nil
}

// ListMySeats returns the seats held (or previously held) by the user.
func (s *BizDecipherService) ListMySeats(ctx context.Context, userID int64) ([]PoolSeat, error) {
	if s == nil || s.repo == nil {
		return []PoolSeat{}, nil
	}
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	return s.repo.ListMySeats(ctx, userID)
}

// ListSharedPoolMembers returns member seats for a pool owned by ownerID.
func (s *BizDecipherService) ListSharedPoolMembers(ctx context.Context, poolID, ownerID int64, status string) ([]PoolSeat, error) {
	if s == nil || s.repo == nil {
		return []PoolSeat{}, nil
	}
	if poolID <= 0 {
		return nil, errors.New("invalid pool id")
	}
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "released" && status != "all" {
		return nil, errors.New("invalid seat status")
	}
	return s.repo.ListSharedPoolMembers(ctx, poolID, ownerID, status)
}

// RemoveSharedPoolMember releases one member seat from a pool owned by ownerID.
func (s *BizDecipherService) RemoveSharedPoolMember(ctx context.Context, poolID, seatID, ownerID int64) error {
	if s == nil || s.repo == nil {
		return errors.New("service unavailable")
	}
	if poolID <= 0 {
		return errors.New("invalid pool id")
	}
	if seatID <= 0 {
		return errors.New("invalid seat id")
	}
	if ownerID <= 0 {
		return errors.New("invalid owner id")
	}
	if err := s.repo.RemoveSharedPoolMemberTx(ctx, poolID, seatID, ownerID); err != nil {
		return err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_member_remove")
	return nil
}

// ChargeDueSeats runs one billing sweep. Called by the background billing worker.
func (s *BizDecipherService) ChargeDueSeats(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error) {
	if s == nil || s.repo == nil {
		return &ChargeSeatsSummary{}, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	summary, err := s.repo.ChargeDueSeats(ctx, now)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_billing")
	return summary, nil
}

// ReleaseIdleSharedPoolSeats auto-releases seats idle for SharedPoolSeatIdleTimeout.
func (s *BizDecipherService) ReleaseIdleSharedPoolSeats(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error) {
	if s == nil || s.repo == nil {
		return &ChargeSeatsSummary{}, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	summary, err := s.repo.ReleaseIdleSharedPoolSeats(ctx, now)
	if err != nil {
		return nil, err
	}
	if summary != nil && summary.IdleSeatsReleased > 0 {
		s.flushSharedPoolDisplayCache(ctx, "pool_idle_release")
	}
	return summary, nil
}

func (s *BizDecipherService) GrantSharedPoolStabilityRewards(ctx context.Context, now time.Time) (*ChargeSeatsSummary, error) {
	if s == nil || s.repo == nil {
		return &ChargeSeatsSummary{}, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	summary, err := s.repo.GrantSharedPoolStabilityRewards(ctx, now)
	if err != nil {
		return nil, err
	}
	s.flushSharedPoolDisplayCache(ctx, "pool_reward")
	return summary, nil
}

// ── Card Skin (收藏卡底色) ──────────────────────────────────────────────────

// SetPoolCardSkin 为共享池设置收藏卡底色皮肤。
// 要求：1) 调用者必须是池主；2) 卡必须存在于调用者的 checkin_collectible_cards。
func (s *BizDecipherService) SetPoolCardSkin(ctx context.Context, poolID, ownerID int64, cardKey, cardRarity string) error {
	if s == nil || s.repo == nil {
		return errors.New("service unavailable")
	}
	// 1. 验证池主身份
	pool, err := s.repo.GetOwnedSharedPool(ctx, poolID, ownerID)
	if err != nil {
		return err
	}
	if pool == nil || pool.OwnerID == nil || *pool.OwnerID != ownerID {
		return errors.New("forbidden: not pool owner")
	}
	cardKey = strings.TrimSpace(cardKey)
	if cardKey == "" {
		return errors.New("card key is required")
	}
	// 2. 从本人收藏库读取权威稀有度，不能信任客户端拼接图片路径。
	ownedRarity, err := s.repo.GetUserCollectibleCardRarity(ctx, ownerID, cardKey)
	if err != nil {
		return err
	}
	ownedRarity = strings.ToLower(strings.TrimSpace(ownedRarity))
	if ownedRarity == "" {
		return errors.New("card not found in your collection")
	}
	if submitted := strings.ToLower(strings.TrimSpace(cardRarity)); submitted != "" && submitted != ownedRarity {
		return errors.New("card rarity does not match your collection")
	}
	// 3. 写入
	if err := s.repo.SetPoolCardSkinTx(ctx, poolID, ownerID, cardKey, ownedRarity); err != nil {
		return err
	}
	s.flushSharedPoolDisplayCache(ctx, "card_skin")
	return nil
}

// ClearPoolCardSkin 清除共享池的收藏卡底色皮肤。
func (s *BizDecipherService) ClearPoolCardSkin(ctx context.Context, poolID, ownerID int64) error {
	if s == nil || s.repo == nil {
		return errors.New("service unavailable")
	}
	pool, err := s.repo.GetOwnedSharedPool(ctx, poolID, ownerID)
	if err != nil {
		return err
	}
	if pool == nil || pool.OwnerID == nil || *pool.OwnerID != ownerID {
		return errors.New("forbidden: not pool owner")
	}
	if err := s.repo.SetPoolCardSkinTx(ctx, poolID, ownerID, "", ""); err != nil {
		return err
	}
	s.flushSharedPoolDisplayCache(ctx, "card_skin_clear")
	return nil
}
