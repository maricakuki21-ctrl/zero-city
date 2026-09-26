package sharedmarket

import (
	"errors"
	"regexp"
	"time"
)

type Source string

const (
	SourceSub2ModelCatalog Source = "sub2.model_catalog"
	SourceSub2Group        Source = "sub2.group"
	SourceSub2Account      Source = "sub2.account"
	SourceSub2Health       Source = "sub2.health"
	SourceSub2UsageLog     Source = "sub2.usage_log"
	SourceBizPool          Source = "bizdecipher.pool"
	SourceBizMembership    Source = "bizdecipher.membership"
	SourceBizPriceSnapshot Source = "bizdecipher.price_snapshot"
	SourceBizLedger        Source = "bizdecipher.canonical_ledger"
	SourceBizCommunity     Source = "bizdecipher.community"
	SourceOfficialStatus   Source = "official.service_status"
)

type Freshness string

const (
	FreshnessLive         Freshness = "live"
	FreshnessRecent       Freshness = "recent"
	FreshnessLastVerified Freshness = "last_verified"
)

type Confidence string

const (
	ConfidenceHigh    Confidence = "high"
	ConfidenceMedium  Confidence = "medium"
	ConfidenceUnknown Confidence = "unknown"
)

type Evidence struct {
	Source     Source     `json:"source"`
	ObservedAt time.Time  `json:"observed_at"`
	Freshness  Freshness  `json:"freshness"`
	Confidence Confidence `json:"confidence"`
}

type Fact[T any] struct {
	Value    T        `json:"value"`
	Evidence Evidence `json:"evidence"`
}

type DecimalString string

var decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

var ErrInvalidDecimalString = errors.New("decimal must be a plain base-10 string")

func NewDecimalString(value string) (DecimalString, error) {
	if !decimalPattern.MatchString(value) {
		return "", ErrInvalidDecimalString
	}
	return DecimalString(value), nil
}

type Model struct {
	Name       string `json:"name"`
	Executable bool   `json:"executable"`
}

type Availability struct {
	State       string `json:"state"`
	Available   int    `json:"available"`
	Total       int    `json:"total"`
	Explanation string `json:"explanation,omitempty"`
}

type Health struct {
	State      string     `json:"state"`
	Confidence Confidence `json:"confidence"`
	Reason     string     `json:"reason,omitempty"`
}

type CanonicalUsage struct {
	RequestsSucceeded string `json:"requests_succeeded"`
	RequestsFailed    string `json:"requests_failed"`
	InputTokens       string `json:"input_tokens"`
	OutputTokens      string `json:"output_tokens"`
}

type RuntimeFacts struct {
	GroupID              Fact[string]         `json:"group_id"`
	AccountIDs           Fact[[]string]       `json:"account_ids"`
	Models               Fact[[]Model]        `json:"models"`
	GroupAvailability    Fact[Availability]   `json:"group_availability"`
	AccountAvailability  Fact[Availability]   `json:"account_availability"`
	Health               Fact[Health]         `json:"health"`
	TodayAvailability    Fact[DecimalString]  `json:"today_availability_percent"`
	SevenDayAvailability Fact[DecimalString]  `json:"seven_day_availability_percent"`
	LatencyMS            Fact[DecimalString]  `json:"latency_ms"`
	ThroughputRPM        Fact[DecimalString]  `json:"throughput_rpm"`
	SuccessRatePercent   Fact[DecimalString]  `json:"success_rate_percent"`
	CanonicalUsage       Fact[CanonicalUsage] `json:"canonical_usage"`
}

type PricingFacts struct {
	RateMultiplier    Fact[DecimalString] `json:"rate_multiplier"`
	MinimumBalance    Fact[DecimalString] `json:"minimum_balance"`
	HourlySeatFee     Fact[DecimalString] `json:"hourly_seat_fee"`
	HourlyUsageWaiver Fact[DecimalString] `json:"hourly_usage_waiver"`
}

type MembershipFacts struct {
	CurrentUsers Fact[int] `json:"current_users"`
	MaximumUsers Fact[int] `json:"maximum_users"`
}

type FinancialFacts struct {
	AvailableBalance Fact[DecimalString] `json:"available_balance"`
	OwnerGross       Fact[DecimalString] `json:"owner_gross"`
	OwnerNet         Fact[DecimalString] `json:"owner_net"`
	PlatformFee      Fact[DecimalString] `json:"platform_fee"`
	ProcessorFee     Fact[DecimalString] `json:"processor_fee"`
	Residue          Fact[DecimalString] `json:"residue"`
}

type CommunityFacts struct {
	Likes       Fact[int] `json:"likes"`
	Complaints  Fact[int] `json:"complaints"`
	Discussions Fact[int] `json:"discussions"`
}

type ProductFacts struct {
	Name        Fact[string]    `json:"name"`
	Description Fact[string]    `json:"description"`
	OwnerLabel  Fact[string]    `json:"owner_label"`
	Membership  MembershipFacts `json:"membership"`
	Pricing     PricingFacts    `json:"pricing"`
	Financial   *FinancialFacts `json:"financial,omitempty"`
	Community   CommunityFacts  `json:"community"`
}

type DependencyError struct {
	Dependency string `json:"dependency"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable"`
	Action     string `json:"action"`
}

type PoolProjection struct {
	PoolID                int64             `json:"pool_id"`
	Product               ProductFacts      `json:"product"`
	Runtime               *RuntimeFacts     `json:"runtime,omitempty"`
	OfficialServiceStatus Fact[string]      `json:"official_service_status"`
	Stale                 bool              `json:"stale"`
	Errors                []DependencyError `json:"errors"`
}
