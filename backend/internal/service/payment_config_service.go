package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"math/big"
	"net/url"
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SettingPaymentEnabled      = "payment_enabled"
	SettingMinRechargeAmount   = "MIN_RECHARGE_AMOUNT"
	SettingMaxRechargeAmount   = "MAX_RECHARGE_AMOUNT"
	SettingDailyRechargeLimit  = "DAILY_RECHARGE_LIMIT"
	SettingOrderTimeoutMinutes = "ORDER_TIMEOUT_MINUTES"
	SettingMaxPendingOrders    = "MAX_PENDING_ORDERS"
	SettingEnabledPaymentTypes = "ENABLED_PAYMENT_TYPES"
	SettingLoadBalanceStrategy = "LOAD_BALANCE_STRATEGY"
	SettingBalancePayDisabled  = "BALANCE_PAYMENT_DISABLED"
	SettingBalanceRechargeMult = "BALANCE_RECHARGE_MULTIPLIER"
	// SettingSubscriptionUSDToCNYRate 是订阅 CNY 换算汇率（1 USD = X CNY）。
	// 0/未配置 = 关闭换算（订阅按 price 数值直付），显式配置后 CNY 通道订阅按 price × rate 收款。
	SettingSubscriptionUSDToCNYRate = "SUBSCRIPTION_USD_TO_CNY_RATE"
	SettingRechargeFeeRate          = "RECHARGE_FEE_RATE"
	SettingProductNamePrefix        = "PRODUCT_NAME_PREFIX"
	SettingProductNameSuffix        = "PRODUCT_NAME_SUFFIX"
	SettingHelpImageURL             = "PAYMENT_HELP_IMAGE_URL"
	SettingHelpText                 = "PAYMENT_HELP_TEXT"
	SettingCancelRateLimitOn        = "CANCEL_RATE_LIMIT_ENABLED"
	SettingCancelRateLimitMax       = "CANCEL_RATE_LIMIT_MAX"
	SettingCancelWindowSize         = "CANCEL_RATE_LIMIT_WINDOW"
	SettingCancelWindowUnit         = "CANCEL_RATE_LIMIT_UNIT"
	SettingCancelWindowMode         = "CANCEL_RATE_LIMIT_WINDOW_MODE"
	SettingAlipayForceQRCode        = "ALIPAY_FORCE_QRCODE"
	SettingExternalCardShopEnabled  = "PAYMENT_EXTERNAL_CARD_SHOP_ENABLED"
	SettingExternalCardShopName     = "PAYMENT_EXTERNAL_CARD_SHOP_NAME"
	SettingExternalCardShopURL      = "PAYMENT_EXTERNAL_CARD_SHOP_URL"
	SettingExternalCardShopEmbed    = "PAYMENT_EXTERNAL_CARD_SHOP_EMBED"
	SettingExternalCardShopProducts = "PAYMENT_EXTERNAL_CARD_SHOP_PRODUCTS"
	SettingCryptoPaymentEnabled     = "PAYMENT_CRYPTO_ENABLED"
	SettingCryptoWalletAddress      = "PAYMENT_CRYPTO_WALLET_ADDRESS"
	SettingCryptoWalletConfirmed    = "PAYMENT_CRYPTO_WALLET_CONFIRMED_ADDRESS"
	SettingCryptoNetwork            = "PAYMENT_CRYPTO_NETWORK"
	SettingCryptoToken              = "PAYMENT_CRYPTO_TOKEN"
	SettingCryptoChain              = "PAYMENT_CRYPTO_CHAIN"
	SettingCryptoContractAddress    = "PAYMENT_CRYPTO_CONTRACT_ADDRESS"
	SettingCryptoMinAmount          = "PAYMENT_CRYPTO_MIN_AMOUNT"
	SettingCryptoUSDTBalanceRate    = "PAYMENT_CRYPTO_USDT_BALANCE_RATE"
	SettingCryptoExplorerBaseURL    = "PAYMENT_CRYPTO_EXPLORER_BASE_URL"
	SettingCryptoTronGridBaseURL    = "PAYMENT_CRYPTO_TRONGRID_BASE_URL"
	SettingCryptoTronGridAPIKey     = "PAYMENT_CRYPTO_TRONGRID_API_KEY"
	SettingCryptoSweepLookbackMin   = "PAYMENT_CRYPTO_SWEEP_LOOKBACK_MINUTES"
)

// Default values for payment configuration settings.
const (
	defaultOrderTimeoutMin  = 30
	defaultMaxPendingOrders = 3
	// Migration 176 seeded this recipient. It is not an operator-owned default.
	legacySeededCryptoWalletAddress = "TJC3ZHLWuXqGcqJSxXZt2uVWAUtXniwmBr"
	defaultCryptoNetwork            = "TRC20"
	defaultCryptoToken              = "USDT"
	defaultCryptoChain              = "tron"
	// Tether TRON mainnet USDt; never infer a token's identity from its symbol.
	defaultCryptoUSDTContractAddress = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
	defaultCryptoMinAmount           = 0.01
	defaultCryptoUSDTBalanceRate     = 7.0
	defaultCryptoExplorerBaseURL     = "https://tronscan.org/#/transaction/"
	defaultCryptoTronGridBaseURL     = "https://api.trongrid.io"
	defaultCryptoSweepLookbackMin    = 180
)

// CryptoPaymentConfig holds the user-facing and service-side crypto payment configuration.
type CryptoPaymentConfig struct {
	Enabled                  bool    `json:"enabled"`
	WalletAddress            string  `json:"wallet_address"`
	Network                  string  `json:"network"`
	Token                    string  `json:"token"`
	Chain                    string  `json:"chain"`
	ContractAddress          string  `json:"contract_address"`
	MinAmount                float64 `json:"min_amount"`
	USDTBalanceRate          float64 `json:"usdt_balance_rate"`
	ExplorerBaseURL          string  `json:"explorer_base_url"`
	TronGridBaseURL          string  `json:"trongrid_base_url"`
	TronGridAPIKey           string  `json:"-"`
	TronGridAPIKeyConfigured bool    `json:"trongrid_api_key_configured"`
	SweepLookbackMinutes     int     `json:"sweep_lookback_minutes"`
}

// UpdateCryptoPaymentConfigRequest is the administrator-facing crypto patch.
// An empty TronGridAPIKey means "keep the existing secret".
type UpdateCryptoPaymentConfigRequest struct {
	Enabled              *bool    `json:"enabled"`
	WalletAddress        *string  `json:"wallet_address"`
	Network              *string  `json:"network"`
	Token                *string  `json:"token"`
	Chain                *string  `json:"chain"`
	ContractAddress      *string  `json:"contract_address"`
	MinAmount            *float64 `json:"min_amount"`
	USDTBalanceRate      *float64 `json:"usdt_balance_rate"`
	ExplorerBaseURL      *string  `json:"explorer_base_url"`
	TronGridBaseURL      *string  `json:"trongrid_base_url"`
	TronGridAPIKey       *string  `json:"trongrid_api_key"`
	SweepLookbackMinutes *int     `json:"sweep_lookback_minutes"`
}

// PaymentConfig holds the payment system configuration.
type PaymentConfig struct {
	Enabled                   bool                `json:"enabled"`
	MinAmount                 float64             `json:"min_amount"`
	MaxAmount                 float64             `json:"max_amount"`
	DailyLimit                float64             `json:"daily_limit"`
	OrderTimeoutMin           int                 `json:"order_timeout_minutes"`
	MaxPendingOrders          int                 `json:"max_pending_orders"`
	EnabledTypes              []string            `json:"enabled_payment_types"`
	BalanceDisabled           bool                `json:"balance_disabled"`
	BalanceRechargeMultiplier float64             `json:"balance_recharge_multiplier"`
	RechargeFeeRate           float64             `json:"recharge_fee_rate"`
	LoadBalanceStrategy       string              `json:"load_balance_strategy"`
	ProductNamePrefix         string              `json:"product_name_prefix"`
	ProductNameSuffix         string              `json:"product_name_suffix"`
	HelpImageURL              string              `json:"help_image_url"`
	HelpText                  string              `json:"help_text"`
	StripePublishableKey      string              `json:"stripe_publishable_key,omitempty"`
	ExternalCardShopEnabled   bool                `json:"external_card_shop_enabled"`
	ExternalCardShopName      string              `json:"external_card_shop_name"`
	ExternalCardShopURL       string              `json:"external_card_shop_url"`
	ExternalCardShopEmbed     bool                `json:"external_card_shop_embed"`
	ExternalCardShopProducts  string              `json:"external_card_shop_products"`
	CryptoPayment             CryptoPaymentConfig `json:"crypto_payment"`
	// SubscriptionUSDToCNYRate 为 0 时订阅换算关闭（兼容存量行为）。
	SubscriptionUSDToCNYRate float64 `json:"subscription_usd_to_cny_rate"`

	// Cancel rate limit settings
	CancelRateLimitEnabled bool   `json:"cancel_rate_limit_enabled"`
	CancelRateLimitMax     int    `json:"cancel_rate_limit_max"`
	CancelRateLimitWindow  int    `json:"cancel_rate_limit_window"`
	CancelRateLimitUnit    string `json:"cancel_rate_limit_unit"`
	CancelRateLimitMode    string `json:"cancel_rate_limit_window_mode"`

	// Force Alipay mobile users to use QR code instead of mobile redirect
	AlipayForceQRCode bool `json:"alipay_force_qrcode"`
}

// UpdatePaymentConfigRequest contains fields to update payment configuration.
type UpdatePaymentConfigRequest struct {
	Enabled                   *bool                             `json:"enabled"`
	MinAmount                 *float64                          `json:"min_amount"`
	MaxAmount                 *float64                          `json:"max_amount"`
	DailyLimit                *float64                          `json:"daily_limit"`
	OrderTimeoutMin           *int                              `json:"order_timeout_minutes"`
	MaxPendingOrders          *int                              `json:"max_pending_orders"`
	EnabledTypes              []string                          `json:"enabled_payment_types"`
	BalanceDisabled           *bool                             `json:"balance_disabled"`
	BalanceRechargeMultiplier *float64                          `json:"balance_recharge_multiplier"`
	SubscriptionUSDToCNYRate  *float64                          `json:"subscription_usd_to_cny_rate"`
	RechargeFeeRate           *float64                          `json:"recharge_fee_rate"`
	LoadBalanceStrategy       *string                           `json:"load_balance_strategy"`
	ProductNamePrefix         *string                           `json:"product_name_prefix"`
	ProductNameSuffix         *string                           `json:"product_name_suffix"`
	HelpImageURL              *string                           `json:"help_image_url"`
	HelpText                  *string                           `json:"help_text"`
	ExternalCardShopEnabled   *bool                             `json:"external_card_shop_enabled"`
	ExternalCardShopName      *string                           `json:"external_card_shop_name"`
	ExternalCardShopURL       *string                           `json:"external_card_shop_url"`
	ExternalCardShopEmbed     *bool                             `json:"external_card_shop_embed"`
	CryptoPayment             *UpdateCryptoPaymentConfigRequest `json:"crypto_payment"`

	// Cancel rate limit settings
	CancelRateLimitEnabled *bool   `json:"cancel_rate_limit_enabled"`
	CancelRateLimitMax     *int    `json:"cancel_rate_limit_max"`
	CancelRateLimitWindow  *int    `json:"cancel_rate_limit_window"`
	CancelRateLimitUnit    *string `json:"cancel_rate_limit_unit"`
	CancelRateLimitMode    *string `json:"cancel_rate_limit_window_mode"`

	// Force Alipay mobile users to use QR code instead of mobile redirect
	AlipayForceQRCode *bool `json:"alipay_force_qrcode"`

	VisibleMethodAlipaySource  *string `json:"payment_visible_method_alipay_source"`
	VisibleMethodWxpaySource   *string `json:"payment_visible_method_wxpay_source"`
	VisibleMethodAlipayEnabled *bool   `json:"payment_visible_method_alipay_enabled"`
	VisibleMethodWxpayEnabled  *bool   `json:"payment_visible_method_wxpay_enabled"`
}

// MethodLimits holds per-payment-type limits.
type MethodLimits struct {
	PaymentType string  `json:"payment_type"`
	DisplayName string  `json:"display_name,omitempty"`
	Currency    string  `json:"currency"`
	FeeRate     float64 `json:"fee_rate"`
	DailyLimit  float64 `json:"daily_limit"`
	SingleMin   float64 `json:"single_min"`
	SingleMax   float64 `json:"single_max"`
}

// MethodLimitsResponse is the full response for the user-facing /limits API.
// It includes per-method limits and the global widest range (union of all methods).
type MethodLimitsResponse struct {
	Methods   map[string]MethodLimits `json:"methods"`
	GlobalMin float64                 `json:"global_min"` // 0 = no minimum
	GlobalMax float64                 `json:"global_max"` // 0 = no maximum
}

type CreateProviderInstanceRequest struct {
	ProviderKey     string            `json:"provider_key"`
	Name            string            `json:"name"`
	Config          map[string]string `json:"config"`
	SupportedTypes  []string          `json:"supported_types"`
	Enabled         bool              `json:"enabled"`
	PaymentMode     string            `json:"payment_mode"`
	SortOrder       int               `json:"sort_order"`
	Limits          string            `json:"limits"`
	RefundEnabled   bool              `json:"refund_enabled"`
	AllowUserRefund bool              `json:"allow_user_refund"`
}

type UpdateProviderInstanceRequest struct {
	Name            *string           `json:"name"`
	Config          map[string]string `json:"config"`
	SupportedTypes  []string          `json:"supported_types"`
	Enabled         *bool             `json:"enabled"`
	PaymentMode     *string           `json:"payment_mode"`
	SortOrder       *int              `json:"sort_order"`
	Limits          *string           `json:"limits"`
	RefundEnabled   *bool             `json:"refund_enabled"`
	AllowUserRefund *bool             `json:"allow_user_refund"`
}
type CreatePlanRequest struct {
	GroupID       int64    `json:"group_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Price         float64  `json:"price"`
	OriginalPrice *float64 `json:"original_price"`
	Currency      string   `json:"currency"`
	ValidityDays  int      `json:"validity_days"`
	ValidityUnit  string   `json:"validity_unit"`
	Features      string   `json:"features"`
	ProductName   string   `json:"product_name"`
	ForSale       bool     `json:"for_sale"`
	SortOrder     int      `json:"sort_order"`
}

type UpdatePlanRequest struct {
	GroupID       *int64   `json:"group_id"`
	Name          *string  `json:"name"`
	Description   *string  `json:"description"`
	Price         *float64 `json:"price"`
	OriginalPrice *float64 `json:"original_price"`
	Currency      *string  `json:"currency"`
	ValidityDays  *int     `json:"validity_days"`
	ValidityUnit  *string  `json:"validity_unit"`
	Features      *string  `json:"features"`
	ProductName   *string  `json:"product_name"`
	ForSale       *bool    `json:"for_sale"`
	SortOrder     *int     `json:"sort_order"`
}

// PaymentConfigService manages payment configuration and CRUD for
// provider instances, channels, and subscription plans.
type PaymentConfigService struct {
	entClient     *dbent.Client
	settingRepo   SettingRepository
	encryptionKey []byte
}

// NewPaymentConfigService creates a new PaymentConfigService.
func NewPaymentConfigService(entClient *dbent.Client, settingRepo SettingRepository, encryptionKey []byte) *PaymentConfigService {
	return &PaymentConfigService{entClient: entClient, settingRepo: settingRepo, encryptionKey: encryptionKey}
}

// IsPaymentEnabled returns whether the payment system is enabled.
func (s *PaymentConfigService) IsPaymentEnabled(ctx context.Context) bool {
	val, err := s.settingRepo.GetValue(ctx, SettingPaymentEnabled)
	if err != nil {
		return false
	}
	return val == "true"
}

// GetPaymentConfig returns the full payment configuration.
func (s *PaymentConfigService) GetPaymentConfig(ctx context.Context) (*PaymentConfig, error) {
	keys := []string{
		SettingPaymentEnabled, SettingMinRechargeAmount, SettingMaxRechargeAmount,
		SettingDailyRechargeLimit, SettingOrderTimeoutMinutes, SettingMaxPendingOrders,
		SettingEnabledPaymentTypes, SettingBalancePayDisabled, SettingBalanceRechargeMult, SettingSubscriptionUSDToCNYRate, SettingRechargeFeeRate, SettingLoadBalanceStrategy,
		SettingProductNamePrefix, SettingProductNameSuffix,
		SettingHelpImageURL, SettingHelpText,
		SettingCancelRateLimitOn, SettingCancelRateLimitMax,
		SettingCancelWindowSize, SettingCancelWindowUnit, SettingCancelWindowMode,
		SettingAlipayForceQRCode,
		SettingExternalCardShopEnabled, SettingExternalCardShopName, SettingExternalCardShopURL, SettingExternalCardShopEmbed, SettingExternalCardShopProducts,
		SettingCryptoPaymentEnabled, SettingCryptoWalletAddress, SettingCryptoWalletConfirmed, SettingCryptoNetwork, SettingCryptoToken, SettingCryptoChain,
		SettingCryptoContractAddress, SettingCryptoMinAmount, SettingCryptoUSDTBalanceRate, SettingCryptoExplorerBaseURL, SettingCryptoTronGridBaseURL,
		SettingCryptoTronGridAPIKey, SettingCryptoSweepLookbackMin,
		SettingPaymentVisibleMethodAlipayEnabled, SettingPaymentVisibleMethodAlipaySource,
		SettingPaymentVisibleMethodWxpayEnabled, SettingPaymentVisibleMethodWxpaySource,
	}
	vals, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get payment config settings: %w", err)
	}
	cfg := s.parsePaymentConfig(vals)
	// Load Stripe publishable key from the first enabled Stripe provider instance
	cfg.StripePublishableKey = s.getStripePublishableKey(ctx)
	return cfg, nil
}

func (s *PaymentConfigService) parsePaymentConfig(vals map[string]string) *PaymentConfig {
	cfg := &PaymentConfig{
		Enabled:                   vals[SettingPaymentEnabled] == "true",
		MinAmount:                 pcParseFloat(vals[SettingMinRechargeAmount], 1),
		MaxAmount:                 pcParseFloat(vals[SettingMaxRechargeAmount], 0),
		DailyLimit:                pcParseFloat(vals[SettingDailyRechargeLimit], 0),
		OrderTimeoutMin:           pcParseInt(vals[SettingOrderTimeoutMinutes], defaultOrderTimeoutMin),
		MaxPendingOrders:          pcParseInt(vals[SettingMaxPendingOrders], defaultMaxPendingOrders),
		BalanceDisabled:           vals[SettingBalancePayDisabled] == "true",
		BalanceRechargeMultiplier: normalizeBalanceRechargeMultiplier(pcParseFloat(vals[SettingBalanceRechargeMult], defaultBalanceRechargeMultiplier)),
		SubscriptionUSDToCNYRate:  normalizeSubscriptionUSDToCNYRate(pcParseFloat(vals[SettingSubscriptionUSDToCNYRate], 0)),
		RechargeFeeRate:           pcParseFloat(vals[SettingRechargeFeeRate], 0),
		LoadBalanceStrategy:       vals[SettingLoadBalanceStrategy],
		ProductNamePrefix:         vals[SettingProductNamePrefix],
		ProductNameSuffix:         vals[SettingProductNameSuffix],
		HelpImageURL:              vals[SettingHelpImageURL],
		HelpText:                  vals[SettingHelpText],
		ExternalCardShopEnabled:   vals[SettingExternalCardShopEnabled] == "true",
		ExternalCardShopName:      strings.TrimSpace(vals[SettingExternalCardShopName]),
		ExternalCardShopURL:       strings.TrimSpace(vals[SettingExternalCardShopURL]),
		ExternalCardShopEmbed:     vals[SettingExternalCardShopEmbed] == "true",
		ExternalCardShopProducts:  strings.TrimSpace(vals[SettingExternalCardShopProducts]),
		CryptoPayment:             parseCryptoPaymentConfig(vals),

		CancelRateLimitEnabled: vals[SettingCancelRateLimitOn] == "true",
		CancelRateLimitMax:     pcParseInt(vals[SettingCancelRateLimitMax], 10),
		CancelRateLimitWindow:  pcParseInt(vals[SettingCancelWindowSize], 1),
		CancelRateLimitUnit:    vals[SettingCancelWindowUnit],
		CancelRateLimitMode:    vals[SettingCancelWindowMode],

		AlipayForceQRCode: vals[SettingAlipayForceQRCode] == "true",
	}
	if cfg.LoadBalanceStrategy == "" {
		cfg.LoadBalanceStrategy = payment.DefaultLoadBalanceStrategy
	}
	if raw := vals[SettingEnabledPaymentTypes]; raw != "" {
		types := make([]string, 0, len(strings.Split(raw, ",")))
		for _, t := range strings.Split(raw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				types = append(types, t)
			}
		}
		cfg.EnabledTypes = NormalizeVisibleMethods(types)
	}
	return cfg
}

func parseCryptoPaymentConfig(vals map[string]string) CryptoPaymentConfig {
	walletAddress := strings.TrimSpace(vals[SettingCryptoWalletAddress])
	if walletAddress == legacySeededCryptoWalletAddress && strings.TrimSpace(vals[SettingCryptoWalletConfirmed]) != walletAddress {
		walletAddress = ""
	}
	network := strings.ToUpper(strings.TrimSpace(vals[SettingCryptoNetwork]))
	if network == "" {
		network = defaultCryptoNetwork
	}
	token := strings.ToUpper(strings.TrimSpace(vals[SettingCryptoToken]))
	if token == "" {
		token = defaultCryptoToken
	}
	chain := strings.ToLower(strings.TrimSpace(vals[SettingCryptoChain]))
	if chain == "" {
		chain = defaultCryptoChain
	}
	contractAddress := strings.TrimSpace(vals[SettingCryptoContractAddress])
	if contractAddress == "" {
		contractAddress = defaultCryptoUSDTContractAddress
	}
	explorerBaseURL := strings.TrimSpace(vals[SettingCryptoExplorerBaseURL])
	if explorerBaseURL == "" {
		explorerBaseURL = defaultCryptoExplorerBaseURL
	}
	tronGridBaseURL := strings.TrimSpace(vals[SettingCryptoTronGridBaseURL])
	if tronGridBaseURL == "" {
		tronGridBaseURL = defaultCryptoTronGridBaseURL
	}
	lookback := pcParseInt(vals[SettingCryptoSweepLookbackMin], defaultCryptoSweepLookbackMin)
	if lookback <= 0 {
		lookback = defaultCryptoSweepLookbackMin
	}
	usdtBalanceRate := pcParseFloat(vals[SettingCryptoUSDTBalanceRate], defaultCryptoUSDTBalanceRate)
	if usdtBalanceRate <= 0 {
		usdtBalanceRate = defaultCryptoUSDTBalanceRate
	}
	return CryptoPaymentConfig{
		Enabled:                  vals[SettingCryptoPaymentEnabled] == "true" && isTRONAddress(walletAddress) && chain == defaultCryptoChain && network == defaultCryptoNetwork && token == defaultCryptoToken && contractAddress == defaultCryptoUSDTContractAddress,
		WalletAddress:            walletAddress,
		Network:                  network,
		Token:                    token,
		Chain:                    chain,
		ContractAddress:          contractAddress,
		MinAmount:                pcParseFloat(vals[SettingCryptoMinAmount], defaultCryptoMinAmount),
		USDTBalanceRate:          usdtBalanceRate,
		ExplorerBaseURL:          explorerBaseURL,
		TronGridBaseURL:          strings.TrimRight(tronGridBaseURL, "/"),
		TronGridAPIKey:           strings.TrimSpace(vals[SettingCryptoTronGridAPIKey]),
		TronGridAPIKeyConfigured: strings.TrimSpace(vals[SettingCryptoTronGridAPIKey]) != "",
		SweepLookbackMinutes:     lookback,
	}
}

// getStripePublishableKey finds the publishable key from the first enabled Stripe provider instance.
func (s *PaymentConfigService) getStripePublishableKey(ctx context.Context) string {
	if s.entClient == nil {
		return ""
	}
	instances, err := s.entClient.PaymentProviderInstance.Query().
		Where(
			paymentproviderinstance.EnabledEQ(true),
			paymentproviderinstance.ProviderKeyEQ(payment.TypeStripe),
		).Limit(1).All(ctx)
	if err != nil || len(instances) == 0 {
		return ""
	}
	cfg, err := s.decryptConfig(instances[0].Config)
	if err != nil || cfg == nil {
		return ""
	}
	return cfg[payment.ConfigKeyPublishableKey]
}

// UpdatePaymentConfig updates the payment configuration settings.
// NOTE: This function exceeds 30 lines because each field requires an independent
// nil-check before serialisation — this is inherent to patch-style update patterns
// and cannot be meaningfully decomposed without introducing unnecessary abstraction.
func (s *PaymentConfigService) UpdatePaymentConfig(ctx context.Context, req UpdatePaymentConfigRequest) error {
	if req.BalanceRechargeMultiplier != nil {
		if math.IsNaN(*req.BalanceRechargeMultiplier) || math.IsInf(*req.BalanceRechargeMultiplier, 0) || *req.BalanceRechargeMultiplier <= 0 {
			return infraerrors.BadRequest("INVALID_BALANCE_RECHARGE_MULTIPLIER", "balance recharge multiplier must be greater than 0")
		}
	}
	if req.SubscriptionUSDToCNYRate != nil {
		v := *req.SubscriptionUSDToCNYRate
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return infraerrors.BadRequest("INVALID_SUBSCRIPTION_USD_TO_CNY_RATE", "subscription USD to CNY rate must be 0 (disabled) or a positive number")
		}
	}
	if req.RechargeFeeRate != nil {
		v := *req.RechargeFeeRate
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
			return infraerrors.BadRequest("INVALID_RECHARGE_FEE_RATE", "recharge fee rate must be between 0 and 100")
		}
		// Enforce max 2 decimal places
		if math.Round(v*100) != v*100 {
			return infraerrors.BadRequest("INVALID_RECHARGE_FEE_RATE", "recharge fee rate allows at most 2 decimal places")
		}
	}
	m := make(map[string]string)
	setBoolSetting(m, SettingPaymentEnabled, req.Enabled)
	setFloatSetting(m, SettingMinRechargeAmount, req.MinAmount, formatPositiveFloat)
	setFloatSetting(m, SettingMaxRechargeAmount, req.MaxAmount, formatPositiveFloat)
	setFloatSetting(m, SettingDailyRechargeLimit, req.DailyLimit, formatPositiveFloat)
	setIntSetting(m, SettingOrderTimeoutMinutes, req.OrderTimeoutMin)
	setIntSetting(m, SettingMaxPendingOrders, req.MaxPendingOrders)
	setBoolSetting(m, SettingBalancePayDisabled, req.BalanceDisabled)
	setFloatSetting(m, SettingBalanceRechargeMult, req.BalanceRechargeMultiplier, formatPositiveFloat)
	setFloatSetting(m, SettingSubscriptionUSDToCNYRate, req.SubscriptionUSDToCNYRate, formatPositiveFloatExact)
	setFloatSetting(m, SettingRechargeFeeRate, req.RechargeFeeRate, formatNonNegativeFloat)
	setStringSetting(m, SettingLoadBalanceStrategy, req.LoadBalanceStrategy)
	setStringSetting(m, SettingProductNamePrefix, req.ProductNamePrefix)
	setStringSetting(m, SettingProductNameSuffix, req.ProductNameSuffix)
	setStringSetting(m, SettingHelpImageURL, req.HelpImageURL)
	setStringSetting(m, SettingHelpText, req.HelpText)
	setBoolSetting(m, SettingExternalCardShopEnabled, req.ExternalCardShopEnabled)
	setStringSetting(m, SettingExternalCardShopName, req.ExternalCardShopName)
	setStringSetting(m, SettingExternalCardShopURL, req.ExternalCardShopURL)
	setBoolSetting(m, SettingExternalCardShopEmbed, req.ExternalCardShopEmbed)
	setBoolSetting(m, SettingCancelRateLimitOn, req.CancelRateLimitEnabled)
	setIntSetting(m, SettingCancelRateLimitMax, req.CancelRateLimitMax)
	setIntSetting(m, SettingCancelWindowSize, req.CancelRateLimitWindow)
	setStringSetting(m, SettingCancelWindowUnit, req.CancelRateLimitUnit)
	setStringSetting(m, SettingCancelWindowMode, req.CancelRateLimitMode)
	setBoolSetting(m, SettingAlipayForceQRCode, req.AlipayForceQRCode)
	setStringSetting(m, SettingPaymentVisibleMethodAlipaySource, req.VisibleMethodAlipaySource)
	setStringSetting(m, SettingPaymentVisibleMethodWxpaySource, req.VisibleMethodWxpaySource)
	setBoolSetting(m, SettingPaymentVisibleMethodAlipayEnabled, req.VisibleMethodAlipayEnabled)
	setBoolSetting(m, SettingPaymentVisibleMethodWxpayEnabled, req.VisibleMethodWxpayEnabled)
	if req.EnabledTypes != nil {
		m[SettingEnabledPaymentTypes] = strings.Join(req.EnabledTypes, ",")
	}
	if req.CryptoPayment != nil {
		if err := s.applyCryptoPaymentPatch(ctx, m, req.CryptoPayment); err != nil {
			return err
		}
	}
	return s.settingRepo.SetMultiple(ctx, m)
}

func (s *PaymentConfigService) applyCryptoPaymentPatch(ctx context.Context, updates map[string]string, req *UpdateCryptoPaymentConfigRequest) error {
	keys := []string{
		SettingCryptoPaymentEnabled, SettingCryptoWalletAddress, SettingCryptoWalletConfirmed, SettingCryptoNetwork, SettingCryptoToken,
		SettingCryptoChain, SettingCryptoContractAddress, SettingCryptoMinAmount, SettingCryptoUSDTBalanceRate,
		SettingCryptoExplorerBaseURL, SettingCryptoTronGridBaseURL, SettingCryptoTronGridAPIKey, SettingCryptoSweepLookbackMin,
	}
	vals, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return fmt.Errorf("get existing crypto payment config: %w", err)
	}
	effective := parseCryptoPaymentConfig(vals)
	// Validate the requested persisted switch, not the runtime readiness projection.
	effective.Enabled = vals[SettingCryptoPaymentEnabled] == "true"

	if req.Enabled != nil {
		effective.Enabled = *req.Enabled
	}
	if req.WalletAddress != nil {
		effective.WalletAddress = strings.TrimSpace(*req.WalletAddress)
	}
	if req.Network != nil {
		effective.Network = strings.ToUpper(strings.TrimSpace(*req.Network))
	}
	if req.Token != nil {
		effective.Token = strings.ToUpper(strings.TrimSpace(*req.Token))
	}
	if req.Chain != nil {
		effective.Chain = strings.ToLower(strings.TrimSpace(*req.Chain))
	}
	if req.ContractAddress != nil {
		effective.ContractAddress = strings.TrimSpace(*req.ContractAddress)
	}
	if req.MinAmount != nil {
		effective.MinAmount = *req.MinAmount
	}
	if req.USDTBalanceRate != nil {
		effective.USDTBalanceRate = *req.USDTBalanceRate
	}
	if req.ExplorerBaseURL != nil {
		effective.ExplorerBaseURL = strings.TrimSpace(*req.ExplorerBaseURL)
	}
	if req.TronGridBaseURL != nil {
		effective.TronGridBaseURL = strings.TrimRight(strings.TrimSpace(*req.TronGridBaseURL), "/")
	}
	if req.SweepLookbackMinutes != nil {
		effective.SweepLookbackMinutes = *req.SweepLookbackMinutes
	}
	if key := strings.TrimSpace(derefStr(req.TronGridAPIKey)); key != "" {
		effective.TronGridAPIKey = key
	}

	if err := validateAdminCryptoPaymentConfig(effective); err != nil {
		return err
	}

	setBoolSetting(updates, SettingCryptoPaymentEnabled, req.Enabled)
	setStringSettingNormalized(updates, SettingCryptoWalletAddress, req.WalletAddress, strings.TrimSpace)
	if req.WalletAddress != nil {
		updates[SettingCryptoWalletConfirmed] = effective.WalletAddress
	}
	setStringSettingNormalized(updates, SettingCryptoNetwork, req.Network, func(v string) string { return strings.ToUpper(strings.TrimSpace(v)) })
	setStringSettingNormalized(updates, SettingCryptoToken, req.Token, func(v string) string { return strings.ToUpper(strings.TrimSpace(v)) })
	setStringSettingNormalized(updates, SettingCryptoChain, req.Chain, func(v string) string { return strings.ToLower(strings.TrimSpace(v)) })
	setStringSettingNormalized(updates, SettingCryptoContractAddress, req.ContractAddress, strings.TrimSpace)
	setFloatSetting(updates, SettingCryptoMinAmount, req.MinAmount, formatPositiveFloatExact)
	setFloatSetting(updates, SettingCryptoUSDTBalanceRate, req.USDTBalanceRate, formatPositiveFloatExact)
	setStringSettingNormalized(updates, SettingCryptoExplorerBaseURL, req.ExplorerBaseURL, strings.TrimSpace)
	setStringSettingNormalized(updates, SettingCryptoTronGridBaseURL, req.TronGridBaseURL, func(v string) string { return strings.TrimRight(strings.TrimSpace(v), "/") })
	setIntSetting(updates, SettingCryptoSweepLookbackMin, req.SweepLookbackMinutes)
	if key := strings.TrimSpace(derefStr(req.TronGridAPIKey)); key != "" {
		updates[SettingCryptoTronGridAPIKey] = key
	}
	return nil
}

func validateAdminCryptoPaymentConfig(cfg CryptoPaymentConfig) error {
	if cfg.Network != defaultCryptoNetwork || cfg.Token != defaultCryptoToken || cfg.Chain != defaultCryptoChain {
		return infraerrors.BadRequest("INVALID_CRYPTO_NETWORK", "only USDT on TRON TRC20 is supported")
	}
	if (cfg.Enabled || cfg.WalletAddress != "") && !isTRONAddress(cfg.WalletAddress) {
		return infraerrors.BadRequest("INVALID_CRYPTO_WALLET", "crypto wallet must be a valid TRON address")
	}
	if cfg.ContractAddress != defaultCryptoUSDTContractAddress {
		return infraerrors.BadRequest("INVALID_CRYPTO_CONTRACT", "USDT-TRC20 must use the verified Tether mainnet contract")
	}
	if !isFinitePositive(cfg.MinAmount) {
		return infraerrors.BadRequest("INVALID_CRYPTO_MIN_AMOUNT", "crypto minimum amount must be greater than 0")
	}
	if !isFinitePositive(cfg.USDTBalanceRate) {
		return infraerrors.BadRequest("INVALID_CRYPTO_RATE", "USDT balance rate must be greater than 0")
	}
	if cfg.SweepLookbackMinutes < 1 || cfg.SweepLookbackMinutes > 10080 {
		return infraerrors.BadRequest("INVALID_CRYPTO_LOOKBACK", "crypto sweep lookback must be between 1 and 10080 minutes")
	}
	if !isHTTPSURL(cfg.ExplorerBaseURL) || !isHTTPSURL(cfg.TronGridBaseURL) {
		return infraerrors.BadRequest("INVALID_CRYPTO_URL", "crypto explorer and TRONGrid URLs must use HTTPS")
	}
	return nil
}

func isTRONAddress(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 34 || value[0] != 'T' {
		return false
	}
	const base58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	decoded := new(big.Int)
	base := big.NewInt(58)
	for _, ch := range value {
		index := strings.IndexRune(base58, ch)
		if index < 0 {
			return false
		}
		decoded.Mul(decoded, base)
		decoded.Add(decoded, big.NewInt(int64(index)))
	}
	raw := decoded.Bytes()
	for i := 0; i < len(value) && value[i] == '1'; i++ {
		raw = append([]byte{0}, raw...)
	}
	if len(raw) != 25 || raw[0] != 0x41 {
		return false
	}
	first := sha256.Sum256(raw[:21])
	second := sha256.Sum256(first[:])
	return bytes.Equal(raw[21:], second[:4])
}

func isFinitePositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func isHTTPSURL(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func setBoolSetting(m map[string]string, key string, value *bool) {
	if value != nil {
		m[key] = strconv.FormatBool(*value)
	}
}

func setFloatSetting(m map[string]string, key string, value *float64, format func(*float64) string) {
	if value != nil {
		m[key] = format(value)
	}
}

func setIntSetting(m map[string]string, key string, value *int) {
	if value != nil {
		m[key] = formatPositiveInt(value)
	}
}

func setStringSetting(m map[string]string, key string, value *string) {
	if value != nil {
		m[key] = *value
	}
}

func setStringSettingNormalized(m map[string]string, key string, value *string, normalize func(string) string) {
	if value != nil {
		m[key] = normalize(*value)
	}
}

func formatBoolOrEmpty(v *bool) string {
	if v == nil {
		return ""
	}
	return strconv.FormatBool(*v)
}

func formatPositiveFloat(v *float64) string {
	if v == nil || *v <= 0 {
		return "" // empty → parsePaymentConfig uses default
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

// formatPositiveFloatExact 保留完整精度，用于汇率等对小数位敏感的配置。
func formatPositiveFloatExact(v *float64) string {
	if v == nil || *v <= 0 {
		return "" // empty → parsePaymentConfig 视为未配置（换算关闭）
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func formatNonNegativeFloat(v *float64) string {
	if v == nil || *v < 0 {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}

func formatPositiveInt(v *int) string {
	if v == nil || *v <= 0 {
		return ""
	}
	return strconv.Itoa(*v)
}

func derefStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func splitTypes(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func joinTypes(types []string) string {
	return strings.Join(types, ",")
}

func pcParseFloat(s string, defaultVal float64) float64 {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

func pcParseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func buildVisibleMethodSourceAvailability(instances []*dbent.PaymentProviderInstance) map[string]bool {
	available := make(map[string]bool, 4)
	for _, inst := range instances {
		switch inst.ProviderKey {
		case payment.TypeAlipay:
			if inst.SupportedTypes == "" || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeAlipay) || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeAlipayDirect) {
				available[VisibleMethodSourceOfficialAlipay] = true
			}
		case payment.TypeWxpay:
			if inst.SupportedTypes == "" || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeWxpay) || payment.InstanceSupportsType(inst.SupportedTypes, payment.TypeWxpayDirect) {
				available[VisibleMethodSourceOfficialWechat] = true
			}
		case payment.TypeEasyPay:
			for _, supportedType := range splitTypes(inst.SupportedTypes) {
				switch NormalizeVisibleMethod(supportedType) {
				case payment.TypeAlipay:
					available[VisibleMethodSourceEasyPayAlipay] = true
				case payment.TypeWxpay:
					available[VisibleMethodSourceEasyPayWechat] = true
				}
			}
		}
	}
	return available
}

func applyVisibleMethodRoutingToEnabledTypes(base []string, vals map[string]string, available map[string]bool) []string {
	shouldExpose := map[string]bool{
		payment.TypeAlipay: visibleMethodShouldBeExposed(payment.TypeAlipay, vals, available),
		payment.TypeWxpay:  visibleMethodShouldBeExposed(payment.TypeWxpay, vals, available),
	}

	seen := make(map[string]struct{}, len(base)+2)
	out := make([]string, 0, len(base)+2)
	appendType := func(paymentType string) {
		paymentType = NormalizeVisibleMethod(paymentType)
		if paymentType == "" {
			return
		}
		if _, ok := seen[paymentType]; ok {
			return
		}
		seen[paymentType] = struct{}{}
		out = append(out, paymentType)
	}

	for _, paymentType := range base {
		visibleMethod := NormalizeVisibleMethod(paymentType)
		switch visibleMethod {
		case payment.TypeAlipay, payment.TypeWxpay:
			if shouldExpose[visibleMethod] {
				appendType(visibleMethod)
			}
		default:
			appendType(visibleMethod)
		}
	}

	for _, visibleMethod := range []string{payment.TypeAlipay, payment.TypeWxpay} {
		if shouldExpose[visibleMethod] {
			appendType(visibleMethod)
		}
	}
	return out
}

func visibleMethodShouldBeExposed(method string, vals map[string]string, available map[string]bool) bool {
	enabledKey := visibleMethodEnabledSettingKey(method)
	sourceKey := visibleMethodSourceSettingKey(method)
	if enabledKey == "" || sourceKey == "" || vals[enabledKey] != "true" {
		return false
	}
	source := NormalizeVisibleMethodSource(method, vals[sourceKey])
	return source != "" && available[source]
}
