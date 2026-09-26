package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	pendingCryptoReconcileLimit       = 50
	cryptoReconcileCandidateLimit     = 500
	cryptoTRC20QueryLimit             = 200
	cryptoTRC20MaxPages               = 50
	cryptoOrderLookbackPadding        = 10 * time.Minute
	maxCryptoLateReconcileLookback    = 7 * 24 * time.Hour
	cryptoRecoveryDeadlineMetadataKey = "crypto_recovery_deadline"
)

type cryptoDeposit struct {
	TxHash          string
	From            string
	To              string
	Token           string
	ContractAddress string
	Amount          float64
	AmountText      string
	BlockTime       time.Time
}

type tronGridTRC20Response struct {
	Data    []tronGridTRC20Transfer `json:"data"`
	Success *bool                   `json:"success"`
	Meta    struct {
		Fingerprint string `json:"fingerprint"`
		Links       struct {
			Next string `json:"next"`
		} `json:"links"`
	} `json:"meta"`
}

type tronGridTRC20Transfer struct {
	TransactionID  string             `json:"transaction_id"`
	From           string             `json:"from"`
	To             string             `json:"to"`
	Value          string             `json:"value"`
	BlockTimestamp int64              `json:"block_timestamp"`
	TokenInfo      tronGridTRC20Token `json:"token_info"`
}

type tronGridTRC20Token struct {
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Decimals any    `json:"decimals"`
}

func (s *PaymentService) createCryptoOrder(ctx context.Context, req CreateOrderRequest, user *User, plan *dbent.SubscriptionPlan, cfg *PaymentConfig, orderAmount, limitAmount, feeRate float64) (*CreateOrderResponse, error) {
	cryptoCfg := cfg.CryptoPayment
	if err := validateCryptoPaymentConfig(cryptoCfg); err != nil {
		return nil, err
	}
	basePayAmountStr, err := cryptoBasePayAmountText(limitAmount, feeRate, cryptoCfg)
	if err != nil {
		return nil, err
	}
	order, details, err := s.createCryptoOrderInTx(ctx, req, user, plan, cfg, cryptoCfg, orderAmount, limitAmount, feeRate, basePayAmountStr)
	if err != nil {
		return nil, err
	}
	s.writeAuditLog(ctx, order.ID, "ORDER_CREATED", fmt.Sprintf("user:%d", req.UserID), map[string]any{
		"paymentAmount":  req.Amount,
		"creditedAmount": order.Amount,
		"payAmount":      order.PayAmount,
		"paymentType":    req.PaymentType,
		"orderType":      req.OrderType,
		"paymentSource":  NormalizePaymentSource(req.PaymentSource),
		"network":        details.Network,
		"token":          details.Token,
	})
	return &CreateOrderResponse{
		OrderID:     order.ID,
		Amount:      order.Amount,
		PayAmount:   order.PayAmount,
		FeeRate:     order.FeeRate,
		Status:      OrderStatusPending,
		ResultType:  payment.CreatePaymentResultOrderCreated,
		PaymentType: payment.TypeCrypto,
		OutTradeNo:  order.OutTradeNo,
		Currency:    cryptoCfg.Token,
		ExpiresAt:   order.ExpiresAt,
		PaymentMode: payment.TypeCrypto,
		Crypto:      details,
	}, nil
}

func validateCryptoPaymentConfig(cfg CryptoPaymentConfig) error {
	if !cfg.Enabled {
		return infraerrors.Forbidden("CRYPTO_PAYMENT_DISABLED", "crypto payment is disabled")
	}
	if strings.TrimSpace(cfg.WalletAddress) == "" {
		return infraerrors.ServiceUnavailable("CRYPTO_PAYMENT_NOT_CONFIGURED", "crypto wallet address is not configured")
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Chain), "tron") || !strings.EqualFold(strings.TrimSpace(cfg.Network), "TRC20") {
		return infraerrors.ServiceUnavailable("CRYPTO_PAYMENT_UNSUPPORTED_NETWORK", "only USDT-TRC20 crypto payment is currently supported")
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Token), "USDT") {
		return infraerrors.ServiceUnavailable("CRYPTO_PAYMENT_UNSUPPORTED_TOKEN", "only USDT crypto payment is currently supported")
	}
	if strings.TrimSpace(cfg.ContractAddress) != defaultCryptoUSDTContractAddress {
		return infraerrors.ServiceUnavailable("CRYPTO_PAYMENT_NOT_CONFIGURED", "USDT-TRC20 contract configuration requires correction")
	}
	if strings.TrimSpace(cfg.TronGridBaseURL) == "" {
		return infraerrors.ServiceUnavailable("CRYPTO_PAYMENT_NOT_CONFIGURED", "TRONGrid API endpoint is not configured")
	}
	if cfg.USDTBalanceRate <= 0 || math.IsNaN(cfg.USDTBalanceRate) || math.IsInf(cfg.USDTBalanceRate, 0) {
		return infraerrors.ServiceUnavailable("CRYPTO_PAYMENT_NOT_CONFIGURED", "USDT balance exchange rate is not configured")
	}
	return nil
}

func cryptoBasePayAmountText(balanceAmount, feeRate float64, cfg CryptoPaymentConfig) (string, error) {
	if cfg.USDTBalanceRate <= 0 || math.IsNaN(cfg.USDTBalanceRate) || math.IsInf(cfg.USDTBalanceRate, 0) {
		return "", infraerrors.ServiceUnavailable("CRYPTO_PAYMENT_NOT_CONFIGURED", "USDT balance exchange rate is not configured")
	}
	base := decimal.NewFromFloat(balanceAmount).Div(decimal.NewFromFloat(cfg.USDTBalanceRate))
	if feeRate > 0 {
		fee := base.Mul(decimal.NewFromFloat(feeRate)).Div(decimal.NewFromInt(100)).RoundUp(int32(payment.CurrencyMaxFractionDigits(cfg.Token)))
		base = base.Add(fee)
	}
	amountText := base.StringFixed(int32(payment.CurrencyMaxFractionDigits(cfg.Token)))
	if _, err := payment.AmountToMinorUnit(amountText, cfg.Token); err != nil {
		return "", infraerrors.BadRequest("INVALID_AMOUNT", err.Error()).WithMetadata(map[string]string{"currency": cfg.Token})
	}
	amount, err := strconv.ParseFloat(amountText, 64)
	if err != nil {
		return "", infraerrors.BadRequest("INVALID_AMOUNT", "invalid crypto payment amount")
	}
	if cfg.MinAmount > 0 && amount < cfg.MinAmount {
		return "", infraerrors.BadRequest("CRYPTO_AMOUNT_TOO_SMALL", "crypto payment amount is below the minimum chain transfer amount").WithMetadata(map[string]string{
			"min":      payment.FormatAmountForCurrency(cfg.MinAmount, cfg.Token),
			"currency": cfg.Token,
		})
	}
	return amountText, nil
}

func (s *PaymentService) createCryptoOrderInTx(ctx context.Context, req CreateOrderRequest, user *User, plan *dbent.SubscriptionPlan, cfg *PaymentConfig, cryptoCfg CryptoPaymentConfig, orderAmount, limitAmount, feeRate float64, basePayAmountStr string) (*dbent.PaymentOrder, *CryptoPaymentDetails, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := s.checkPendingLimit(ctx, tx, req.UserID, cfg.MaxPendingOrders); err != nil {
		return nil, nil, err
	}
	if err := s.checkDailyLimit(ctx, tx, req.UserID, limitAmount, cfg.DailyLimit); err != nil {
		return nil, nil, err
	}
	tm := cfg.OrderTimeoutMin
	if tm <= 0 {
		tm = defaultOrderTimeoutMin
	}
	expiresAt := time.Now().Add(time.Duration(tm) * time.Minute)
	outTradeNo, err := s.allocateOutTradeNo(ctx, tx)
	if err != nil {
		return nil, nil, err
	}
	usedAmounts, err := pendingCryptoPayAmountTexts(ctx, tx, cryptoCfg.Token)
	if err != nil {
		return nil, nil, err
	}
	payAmount, payAmountText, err := cryptoPayAmountWithFingerprint(basePayAmountStr, outTradeNo, req.UserID, usedAmounts, cryptoCfg.Token)
	if err != nil {
		return nil, nil, err
	}
	details := &CryptoPaymentDetails{
		WalletAddress:   cryptoCfg.WalletAddress,
		Network:         cryptoCfg.Network,
		Token:           cryptoCfg.Token,
		Chain:           cryptoCfg.Chain,
		ContractAddress: cryptoCfg.ContractAddress,
		PayAmount:       payAmount,
		PayAmountText:   payAmountText,
		BalanceAmount:   orderAmount,
		BalanceCurrency: payment.DefaultPaymentCurrency,
		USDTBalanceRate: cryptoCfg.USDTBalanceRate,
		MinAmount:       cryptoCfg.MinAmount,
		OutTradeNo:      outTradeNo,
		ExplorerBaseURL: cryptoCfg.ExplorerBaseURL,
	}
	providerSnapshot := buildCryptoPaymentSnapshot(details)
	providerSnapshot["crypto_sweep_lookback_minutes"] = cryptoCfg.SweepLookbackMinutes
	builder := tx.PaymentOrder.Create().
		SetUserID(req.UserID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetNillableUserNotes(psNilIfEmpty(user.Notes)).
		SetAmount(orderAmount).
		SetPayAmount(payAmount).
		SetFeeRate(feeRate).
		SetRechargeCode("").
		SetOutTradeNo(outTradeNo).
		SetPaymentType(payment.TypeCrypto).
		SetPaymentTradeNo("").
		SetOrderType(req.OrderType).
		SetStatus(OrderStatusPending).
		SetExpiresAt(expiresAt).
		SetClientIP(req.ClientIP).
		SetSrcHost(req.SrcHost).
		SetProviderKey(payment.TypeCrypto).
		SetProviderSnapshot(providerSnapshot)
	if req.SrcURL != "" {
		builder.SetSrcURL(req.SrcURL)
	}
	if plan != nil {
		builder.SetPlanID(plan.ID).SetSubscriptionGroupID(plan.GroupID).SetSubscriptionDays(psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit))
	}
	order, err := builder.Save(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("create crypto order: %w", err)
	}
	code := fmt.Sprintf("PAY-%d-%d", order.ID, time.Now().UnixNano()%100000)
	order, err = tx.PaymentOrder.UpdateOneID(order.ID).SetRechargeCode(code).Save(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("set recharge code: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit crypto order transaction: %w", err)
	}
	return order, details, nil
}

func pendingCryptoPayAmountTexts(ctx context.Context, tx *dbent.Tx, token string) (map[string]struct{}, error) {
	orders, err := tx.PaymentOrder.Query().
		Where(paymentorder.PaymentTypeEQ(payment.TypeCrypto), paymentorder.StatusEQ(OrderStatusPending), paymentorder.ExpiresAtGT(time.Now())).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query pending crypto orders: %w", err)
	}
	used := make(map[string]struct{}, len(orders))
	for _, order := range orders {
		used[payment.FormatAmountForCurrency(order.PayAmount, token)] = struct{}{}
	}
	return used, nil
}

func cryptoPayAmountWithFingerprint(basePayAmountStr, outTradeNo string, userID int64, used map[string]struct{}, token string) (float64, string, error) {
	base, err := decimal.NewFromString(strings.TrimSpace(basePayAmountStr))
	if err != nil {
		return 0, "", infraerrors.BadRequest("INVALID_AMOUNT", "invalid crypto payment amount")
	}
	seed := cryptoFingerprintSeed(outTradeNo, userID)
	for offset := 0; offset < 899; offset++ {
		fingerprint := ((seed + offset - 1) % 899) + 1
		amountText := base.Add(decimal.NewFromInt(int64(fingerprint)).Shift(-6)).StringFixed(6)
		if _, exists := used[amountText]; exists {
			continue
		}
		if _, err := payment.AmountToMinorUnit(amountText, token); err != nil {
			return 0, "", infraerrors.BadRequest("INVALID_AMOUNT", err.Error()).WithMetadata(map[string]string{"currency": token})
		}
		amount, err := strconv.ParseFloat(amountText, 64)
		if err != nil {
			return 0, "", infraerrors.BadRequest("INVALID_AMOUNT", "invalid crypto payment amount")
		}
		return amount, amountText, nil
	}
	return 0, "", infraerrors.TooManyRequests("CRYPTO_AMOUNT_FINGERPRINT_EXHAUSTED", "too many pending crypto orders with similar amount")
}

func cryptoFingerprintSeed(outTradeNo string, userID int64) int {
	seed := int(userID % 899)
	for _, ch := range outTradeNo {
		seed += int(ch)
	}
	seed %= 899
	if seed == 0 {
		seed = 1
	}
	return seed
}

func buildCryptoPaymentSnapshot(details *CryptoPaymentDetails) map[string]any {
	if details == nil {
		return nil
	}
	return map[string]any{
		"schema_version":                3,
		"provider_key":                  payment.TypeCrypto,
		"payment_mode":                  payment.TypeCrypto,
		"currency":                      details.Token,
		"crypto_wallet_address":         details.WalletAddress,
		"crypto_network":                details.Network,
		"crypto_token":                  details.Token,
		"crypto_chain":                  details.Chain,
		"crypto_contract":               details.ContractAddress,
		"crypto_pay_amount_text":        details.PayAmountText,
		"crypto_balance_amount":         details.BalanceAmount,
		"crypto_balance_currency":       details.BalanceCurrency,
		"crypto_usdt_balance_rate":      details.USDTBalanceRate,
		"crypto_min_amount":             details.MinAmount,
		"crypto_explorer_base_url":      details.ExplorerBaseURL,
		"crypto_sweep_lookback_minutes": defaultCryptoSweepLookbackMin,
	}
}

// cryptoConfigForOrder overlays immutable order-time payment coordinates on
// the current service configuration. TRONGrid endpoint and API credentials are
// intentionally kept current and are never persisted in the order snapshot.
func cryptoConfigForOrder(current CryptoPaymentConfig, order *dbent.PaymentOrder) CryptoPaymentConfig {
	details := CryptoPaymentDetailsForOrder(order)
	if details == nil {
		return current
	}
	if value := strings.TrimSpace(details.WalletAddress); value != "" {
		current.WalletAddress = value
	}
	if value := strings.TrimSpace(details.Network); value != "" {
		current.Network = value
	}
	if value := strings.TrimSpace(details.Token); value != "" {
		current.Token = value
	}
	if value := strings.TrimSpace(details.Chain); value != "" {
		current.Chain = value
	}
	if value := strings.TrimSpace(details.ContractAddress); value != "" {
		current.ContractAddress = value
	}
	if value := cryptoSnapshotFloat(order.ProviderSnapshot, "crypto_usdt_balance_rate", 0); value > 0 {
		current.USDTBalanceRate = value
	}
	if _, exists := order.ProviderSnapshot["crypto_min_amount"]; exists {
		if value := cryptoSnapshotFloat(order.ProviderSnapshot, "crypto_min_amount", current.MinAmount); value >= 0 {
			current.MinAmount = value
		}
	}
	if value := strings.TrimSpace(details.ExplorerBaseURL); value != "" {
		current.ExplorerBaseURL = value
	}
	if order != nil {
		lookback := int(cryptoSnapshotFloat(order.ProviderSnapshot, "crypto_sweep_lookback_minutes", float64(current.SweepLookbackMinutes)))
		if lookback > 0 {
			current.SweepLookbackMinutes = lookback
		}
	}
	return current
}

func effectiveCryptoSweepLookback(cfg CryptoPaymentConfig) time.Duration {
	minutes := cfg.SweepLookbackMinutes
	if minutes <= 0 {
		minutes = defaultCryptoSweepLookbackMin
	}
	lookback := time.Duration(minutes) * time.Minute
	if lookback > maxCryptoLateReconcileLookback {
		return maxCryptoLateReconcileLookback
	}
	return lookback
}

func cryptoExpectedPayAmountText(order *dbent.PaymentOrder, cfg CryptoPaymentConfig) string {
	if order == nil {
		return ""
	}
	if text := cryptoSnapshotString(order.ProviderSnapshot, "crypto_pay_amount_text"); text != "" {
		return text
	}
	return payment.FormatAmountForCurrency(order.PayAmount, cfg.Token)
}

func cryptoAmountTextsEqual(left, right, token string) bool {
	leftMinor, leftErr := payment.AmountToMinorUnit(left, token)
	rightMinor, rightErr := payment.AmountToMinorUnit(right, token)
	return leftErr == nil && rightErr == nil && leftMinor == rightMinor
}

func cryptoOrderRecoveryDeadline(order *dbent.PaymentOrder, cfg CryptoPaymentConfig) time.Time {
	if order == nil || order.ExpiresAt.IsZero() {
		return time.Time{}
	}
	return order.ExpiresAt.Add(effectiveCryptoSweepLookback(cfg))
}

func cryptoOrderWithinRecoveryWindow(order *dbent.PaymentOrder, cfg CryptoPaymentConfig, now time.Time) bool {
	if order == nil {
		return false
	}
	if order.Status != OrderStatusPending && order.Status != OrderStatusExpired && order.Status != OrderStatusCancelled {
		return false
	}
	if order.ExpiresAt.After(now) {
		return true
	}
	deadline := cryptoOrderRecoveryDeadline(order, cfg)
	return !deadline.IsZero() && !now.After(deadline)
}

func cryptoNotificationCanRecoverOrder(order *dbent.PaymentOrder, metadata map[string]string, now time.Time) bool {
	if order == nil || order.PaymentType != payment.TypeCrypto || order.ExpiresAt.IsZero() {
		return false
	}
	source := strings.TrimSpace(metadata["source"])
	if source != "auto_sweep" && source != "user_submitted_tx" {
		return false
	}
	deadline, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(metadata[cryptoRecoveryDeadlineMetadataKey]))
	lookbackMinutes := int(cryptoSnapshotFloat(order.ProviderSnapshot, "crypto_sweep_lookback_minutes", defaultCryptoSweepLookbackMin))
	allowedDeadline := order.ExpiresAt.Add(effectiveCryptoSweepLookback(CryptoPaymentConfig{SweepLookbackMinutes: lookbackMinutes}))
	if err != nil || deadline.After(allowedDeadline) {
		return false
	}
	return !now.After(deadline)
}

func cryptoDepositWithinOrderWindow(order *dbent.PaymentOrder, cfg CryptoPaymentConfig, deposit *cryptoDeposit) bool {
	if order == nil || deposit == nil || deposit.BlockTime.IsZero() {
		return false
	}
	if !order.CreatedAt.IsZero() && deposit.BlockTime.Before(order.CreatedAt.Add(-cryptoOrderLookbackPadding)) {
		return false
	}
	deadline := cryptoOrderRecoveryDeadline(order, cfg)
	return deadline.IsZero() || !deposit.BlockTime.After(deadline)
}

func CryptoPaymentDetailsForOrder(order *dbent.PaymentOrder) *CryptoPaymentDetails {
	if order == nil || order.PaymentType != payment.TypeCrypto {
		return nil
	}
	snapshot := order.ProviderSnapshot
	if len(snapshot) == 0 {
		return nil
	}
	token := cryptoSnapshotString(snapshot, "crypto_token")
	if token == "" {
		token = cryptoSnapshotString(snapshot, "currency")
	}
	if token == "" {
		token = defaultCryptoToken
	}
	payAmountText := cryptoSnapshotString(snapshot, "crypto_pay_amount_text")
	if payAmountText == "" {
		payAmountText = payment.FormatAmountForCurrency(order.PayAmount, token)
	}
	return &CryptoPaymentDetails{
		WalletAddress:   cryptoSnapshotString(snapshot, "crypto_wallet_address"),
		Network:         cryptoSnapshotString(snapshot, "crypto_network"),
		Token:           token,
		Chain:           cryptoSnapshotString(snapshot, "crypto_chain"),
		ContractAddress: cryptoSnapshotString(snapshot, "crypto_contract"),
		PayAmount:       order.PayAmount,
		PayAmountText:   payAmountText,
		BalanceAmount:   cryptoSnapshotFloat(snapshot, "crypto_balance_amount", order.Amount),
		BalanceCurrency: cryptoSnapshotStringDefault(snapshot, "crypto_balance_currency", payment.DefaultPaymentCurrency),
		USDTBalanceRate: cryptoSnapshotFloat(snapshot, "crypto_usdt_balance_rate", defaultCryptoUSDTBalanceRate),
		MinAmount:       cryptoSnapshotFloat(snapshot, "crypto_min_amount", defaultCryptoMinAmount),
		OutTradeNo:      order.OutTradeNo,
		ExplorerBaseURL: cryptoSnapshotString(snapshot, "crypto_explorer_base_url"),
		PaymentTradeNo:  strings.TrimSpace(order.PaymentTradeNo),
	}
}

func cryptoSnapshotString(snapshot map[string]any, key string) string {
	if len(snapshot) == 0 {
		return ""
	}
	return psSnapshotStringValue(snapshot[key])
}

func cryptoSnapshotStringDefault(snapshot map[string]any, key, fallback string) string {
	if value := cryptoSnapshotString(snapshot, key); value != "" {
		return value
	}
	return fallback
}

func cryptoSnapshotFloat(snapshot map[string]any, key string, fallback float64) float64 {
	if len(snapshot) == 0 {
		return fallback
	}
	switch value := snapshot[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case json.Number:
		if parsed, err := value.Float64(); err == nil {
			return parsed
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func (s *PaymentService) SubmitCryptoTx(ctx context.Context, orderID, userID int64, txHash string) (*dbent.PaymentOrder, error) {
	txHash, err := normalizeCryptoTxHash(txHash)
	if err != nil {
		return nil, err
	}
	order, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if order.UserID != userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission for this order")
	}
	if order.PaymentType != payment.TypeCrypto {
		return nil, infraerrors.BadRequest("INVALID_PAYMENT_TYPE", "order is not a crypto payment order")
	}
	if duplicate, err := s.findOrderByCryptoTx(ctx, txHash); err != nil {
		return nil, err
	} else if duplicate != nil && duplicate.ID != order.ID {
		return nil, infraerrors.Conflict("CRYPTO_TX_ALREADY_USED", "this transaction has already been used by another order")
	} else if duplicate != nil && duplicate.ID == order.ID && duplicate.Status == OrderStatusCompleted {
		return duplicate, nil
	}
	if order.Status == OrderStatusCompleted {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "completed crypto order cannot accept another transaction")
	}
	if !cryptoOrderCanAcceptTx(order) {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "order cannot accept crypto transaction in current status")
	}
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment config: %w", err)
	}
	orderCryptoCfg := cryptoConfigForOrder(cfg.CryptoPayment, order)
	if err := validateCryptoPaymentConfig(orderCryptoCfg); err != nil {
		return nil, err
	}
	if (order.Status == OrderStatusPending || order.Status == OrderStatusExpired || order.Status == OrderStatusCancelled) && !cryptoOrderWithinRecoveryWindow(order, orderCryptoCfg, time.Now()) {
		return nil, infraerrors.BadRequest("CRYPTO_RECOVERY_WINDOW_EXPIRED", "crypto payment recovery window has expired")
	}
	deposit, err := s.findCryptoDepositByTx(ctx, orderCryptoCfg, order, txHash)
	if err != nil {
		return nil, err
	}
	if err := validateCryptoDepositForOrder(orderCryptoCfg, order, deposit); err != nil {
		return nil, err
	}
	if err := s.confirmCryptoOrderWithDeposit(ctx, orderCryptoCfg, order, deposit, "user_submitted_tx"); err != nil {
		return nil, err
	}
	return s.entClient.PaymentOrder.Get(ctx, order.ID)
}

func normalizeCryptoTxHash(raw string) (string, error) {
	txHash := strings.ToLower(strings.TrimSpace(raw))
	if len(txHash) != 64 {
		return "", infraerrors.BadRequest("INVALID_CRYPTO_TX", "transaction hash must be 64 hex characters")
	}
	if _, err := hex.DecodeString(txHash); err != nil {
		return "", infraerrors.BadRequest("INVALID_CRYPTO_TX", "transaction hash must be hexadecimal")
	}
	return txHash, nil
}

func cryptoOrderCanAcceptTx(order *dbent.PaymentOrder) bool {
	if order == nil {
		return false
	}
	switch order.Status {
	case OrderStatusPending, OrderStatusExpired, OrderStatusCancelled, OrderStatusPaid, OrderStatusRecharging:
		return true
	case OrderStatusCompleted:
		return strings.TrimSpace(order.PaymentTradeNo) != ""
	default:
		return false
	}
}

func (s *PaymentService) findOrderByCryptoTx(ctx context.Context, txHash string) (*dbent.PaymentOrder, error) {
	order, err := s.entClient.PaymentOrder.Query().Where(paymentorder.PaymentTradeNoEQ(txHash)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("query crypto tx duplicate: %w", err)
	}
	return order, nil
}

func (s *PaymentService) findCryptoDepositByTx(ctx context.Context, cfg CryptoPaymentConfig, order *dbent.PaymentOrder, txHash string) (*cryptoDeposit, error) {
	since := time.Now().Add(-effectiveCryptoSweepLookback(cfg))
	if order != nil && !order.CreatedAt.IsZero() {
		orderSince := order.CreatedAt.Add(-cryptoOrderLookbackPadding)
		if orderSince.After(since) {
			since = orderSince
		}
	}
	deposits, err := fetchCryptoDeposits(ctx, cfg, since)
	if err != nil {
		return nil, err
	}
	for _, deposit := range deposits {
		if strings.EqualFold(deposit.TxHash, txHash) {
			return deposit, nil
		}
	}
	return nil, infraerrors.NotFound("CRYPTO_TX_NOT_FOUND", "confirmed USDT-TRC20 transfer was not found for this wallet")
}

func validateCryptoDepositForOrder(cfg CryptoPaymentConfig, order *dbent.PaymentOrder, deposit *cryptoDeposit) error {
	if deposit == nil {
		return infraerrors.NotFound("CRYPTO_TX_NOT_FOUND", "confirmed USDT-TRC20 transfer was not found for this wallet")
	}
	if !cryptoDepositMatchesConfig(cfg, deposit) {
		return infraerrors.BadRequest("CRYPTO_TX_MISMATCH", "transaction does not match the configured USDT-TRC20 receiving wallet")
	}
	if !cryptoAmountTextsEqual(deposit.AmountText, cryptoExpectedPayAmountText(order, cfg), cfg.Token) {
		return infraerrors.BadRequest("CRYPTO_AMOUNT_MISMATCH", "transaction amount does not match this order").WithMetadata(map[string]string{
			"expected": cryptoExpectedPayAmountText(order, cfg),
			"paid":     deposit.AmountText,
		})
	}
	if !cryptoDepositWithinOrderWindow(order, cfg, deposit) {
		return infraerrors.BadRequest("CRYPTO_TX_OUTSIDE_ORDER_WINDOW", "transaction timestamp is outside this order's payment recovery window")
	}
	return nil
}

func cryptoDepositMatchesConfig(cfg CryptoPaymentConfig, deposit *cryptoDeposit) bool {
	if deposit == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(deposit.To), strings.TrimSpace(cfg.WalletAddress)) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(deposit.ContractAddress), strings.TrimSpace(cfg.ContractAddress)) {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(deposit.Token), strings.TrimSpace(cfg.Token)) {
		return false
	}
	return true
}

func (s *PaymentService) confirmCryptoOrderWithDeposit(ctx context.Context, cfg CryptoPaymentConfig, order *dbent.PaymentOrder, deposit *cryptoDeposit, source string) error {
	metadata := map[string]string{
		"currency": deposit.Token,
		"network":  cfg.Network,
		"source":   source,
	}
	if deadline := cryptoOrderRecoveryDeadline(order, cfg); !deadline.IsZero() {
		metadata[cryptoRecoveryDeadlineMetadataKey] = deadline.UTC().Format(time.RFC3339Nano)
	}
	return s.HandlePaymentNotification(ctx, &payment.PaymentNotification{
		TradeNo:  deposit.TxHash,
		OrderID:  order.OutTradeNo,
		Amount:   deposit.Amount,
		Status:   payment.NotificationStatusSuccess,
		Metadata: metadata,
	}, payment.TypeCrypto)
}

func (s *PaymentService) ReconcilePendingCryptoOrders(ctx context.Context) (int, error) {
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return 0, fmt.Errorf("get payment config: %w", err)
	}
	if err := validateCryptoPaymentConfig(cfg.CryptoPayment); err != nil {
		if appErr := new(infraerrors.ApplicationError); errors.As(err, &appErr) {
			return 0, nil
		}
		return 0, err
	}
	now := time.Now()
	hardCutoff := now.Add(-maxCryptoLateReconcileLookback)
	orders, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.PaymentTypeEQ(payment.TypeCrypto),
			paymentorder.StatusIn(OrderStatusPending, OrderStatusExpired, OrderStatusCancelled),
			paymentorder.ExpiresAtGTE(hardCutoff),
		).
		Order(dbent.Asc(paymentorder.FieldCreatedAt)).
		Limit(cryptoReconcileCandidateLimit).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("query pending crypto orders: %w", err)
	}
	if len(orders) == 0 {
		return 0, nil
	}
	type reconcileGroup struct {
		cfg    CryptoPaymentConfig
		since  time.Time
		orders []*dbent.PaymentOrder
	}
	groups := map[string]*reconcileGroup{}
	for _, order := range orders {
		orderCfg := cryptoConfigForOrder(cfg.CryptoPayment, order)
		if err := validateCryptoPaymentConfig(orderCfg); err != nil || !cryptoOrderWithinRecoveryWindow(order, orderCfg, now) {
			continue
		}
		since := now.Add(-effectiveCryptoSweepLookback(orderCfg))
		if !order.CreatedAt.IsZero() {
			orderSince := order.CreatedAt.Add(-cryptoOrderLookbackPadding)
			if orderSince.After(since) {
				since = orderSince
			}
		}
		key := strings.ToLower(strings.Join([]string{
			strings.TrimSpace(orderCfg.WalletAddress),
			strings.TrimSpace(orderCfg.ContractAddress),
			strings.TrimSpace(orderCfg.Token),
			strings.TrimSpace(orderCfg.TronGridBaseURL),
		}, "|"))
		group := groups[key]
		if group == nil {
			group = &reconcileGroup{cfg: orderCfg, since: since}
			groups[key] = group
		} else if since.Before(group.since) {
			group.since = since
		}
		group.orders = append(group.orders, order)
	}
	recovered := 0
	consumed := map[string]struct{}{}
	for _, group := range groups {
		deposits, fetchErr := fetchCryptoDeposits(ctx, group.cfg, group.since)
		if fetchErr != nil {
			return recovered, fetchErr
		}
		for _, order := range group.orders {
			if recovered >= pendingCryptoReconcileLimit {
				return recovered, nil
			}
			orderCfg := cryptoConfigForOrder(group.cfg, order)
			for _, deposit := range deposits {
				if _, ok := consumed[deposit.TxHash]; ok {
					continue
				}
				if err := validateCryptoDepositForOrder(orderCfg, order, deposit); err != nil {
					continue
				}
				if duplicate, dupErr := s.findOrderByCryptoTx(ctx, deposit.TxHash); dupErr != nil {
					slog.Warn("query crypto tx duplicate failed", "tx", deposit.TxHash, "error", dupErr)
					continue
				} else if duplicate != nil && duplicate.ID != order.ID {
					consumed[deposit.TxHash] = struct{}{}
					continue
				}
				if err := s.confirmCryptoOrderWithDeposit(ctx, orderCfg, order, deposit, "auto_sweep"); err != nil {
					slog.Warn("confirm crypto order failed", "orderID", order.ID, "tx", deposit.TxHash, "error", err)
					continue
				}
				consumed[deposit.TxHash] = struct{}{}
				recovered++
				break
			}
		}
	}
	return recovered, nil
}

func fetchCryptoDeposits(ctx context.Context, cfg CryptoPaymentConfig, since time.Time) ([]*cryptoDeposit, error) {
	endpoint, err := buildTronGridTRC20URL(cfg, since)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Set("max_timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	client := &http.Client{
		Timeout: 15 * time.Second,
		// Do not forward a merchant's API key through provider redirects.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	seen := map[string]bool{}
	deposits := make([]*cryptoDeposit, 0)
	for page := 0; page < cryptoTRC20MaxPages; page++ {
		u.RawQuery = query.Encode()
		payload, err := fetchCryptoDepositPage(ctx, client, u.String(), cfg.TronGridAPIKey)
		if err != nil {
			return nil, err
		} // Never return a silently truncated successful scan.
		for _, transfer := range payload.Data {
			deposit, err := parseTronGridTRC20Transfer(cfg, transfer)
			if err != nil {
				slog.Warn("skip invalid TRONGrid transfer", "tx", transfer.TransactionID, "error", err)
				continue
			}
			deposits = append(deposits, deposit)
		}
		fingerprint := strings.TrimSpace(payload.Meta.Fingerprint)
		if fingerprint == "" {
			if payload.Meta.Links.Next != "" {
				return nil, infraerrors.ServiceUnavailable("CRYPTO_CHAIN_PAGINATION_INVALID", "TRONGrid continuation is missing its fingerprint")
			}
			return deposits, nil
		}
		if seen[fingerprint] {
			return nil, infraerrors.ServiceUnavailable("CRYPTO_CHAIN_PAGINATION_INVALID", "TRONGrid returned a repeated page cursor")
		}
		seen[fingerprint] = true
		// Reconstruct on the configured endpoint. Never follow an untrusted next URL.
		query.Set("fingerprint", fingerprint)
	}
	return nil, infraerrors.ServiceUnavailable("CRYPTO_CHAIN_SCAN_LIMIT", "TRONGrid scan exceeded its page limit; reconcile a smaller time range")
}

func fetchCryptoDepositPage(ctx context.Context, client *http.Client, endpoint, apiKey string) (*tronGridTRC20Response, error) {
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(apiKey) != "" {
			req.Header.Set("TRON-PRO-API-KEY", strings.TrimSpace(apiKey))
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, infraerrors.ServiceUnavailable("CRYPTO_CHAIN_QUERY_FAILED", "failed to query TRONGrid")
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			delay := time.Duration(attempt+1) * time.Second
			if seconds, parseErr := strconv.Atoi(resp.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
				if seconds > 5 {
					seconds = 5
				}
				delay = time.Duration(seconds) * time.Second
			}
			resp.Body.Close()
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, infraerrors.ServiceUnavailable("CRYPTO_CHAIN_QUERY_FAILED", fmt.Sprintf("TRONGrid returned HTTP %d", resp.StatusCode))
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if len(body) > 4*1024*1024 {
			return nil, fmt.Errorf("TRONGrid response exceeds size limit")
		}
		var payload tronGridTRC20Response
		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			return nil, fmt.Errorf("decode TRONGrid response: %w", err)
		}
		if (payload.Success != nil && !*payload.Success) || payload.Data == nil {
			return nil, infraerrors.ServiceUnavailable("CRYPTO_CHAIN_QUERY_FAILED", "TRONGrid did not return a successful transfer list")
		}
		return &payload, nil
	}
	return nil, infraerrors.ServiceUnavailable("CRYPTO_CHAIN_QUERY_FAILED", "TRONGrid retry limit reached")
}

func buildTronGridTRC20URL(cfg CryptoPaymentConfig, since time.Time) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.TronGridBaseURL), "/")
	if base == "" {
		return "", infraerrors.ServiceUnavailable("CRYPTO_PAYMENT_NOT_CONFIGURED", "TRONGrid API endpoint is not configured")
	}
	u, err := url.Parse(base + "/v1/accounts/" + url.PathEscape(strings.TrimSpace(cfg.WalletAddress)) + "/transactions/trc20")
	if err != nil {
		return "", fmt.Errorf("build TRONGrid URL: %w", err)
	}
	q := u.Query()
	q.Set("only_confirmed", "true")
	q.Set("limit", strconv.Itoa(cryptoTRC20QueryLimit))
	q.Set("order_by", "block_timestamp,desc")
	q.Set("contract_address", strings.TrimSpace(cfg.ContractAddress))
	if !since.IsZero() {
		q.Set("min_timestamp", strconv.FormatInt(since.UnixMilli(), 10))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func parseTronGridTRC20Transfer(cfg CryptoPaymentConfig, transfer tronGridTRC20Transfer) (*cryptoDeposit, error) {
	decimals := parseCryptoTokenDecimals(transfer.TokenInfo.Decimals, payment.CurrencyMinorUnit(cfg.Token))
	amount, err := decimal.NewFromString(strings.TrimSpace(transfer.Value))
	if err != nil {
		return nil, fmt.Errorf("invalid transfer amount: %w", err)
	}
	if decimals > 0 {
		amount = amount.Div(decimal.New(1, int32(decimals)))
	}
	amountText := amount.StringFixed(int32(payment.CurrencyMaxFractionDigits(cfg.Token)))
	amountFloat, err := strconv.ParseFloat(amountText, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid parsed transfer amount: %w", err)
	}
	blockTime := time.Time{}
	if transfer.BlockTimestamp > 0 {
		blockTime = time.UnixMilli(transfer.BlockTimestamp)
	}
	return &cryptoDeposit{
		TxHash:          strings.ToLower(strings.TrimSpace(transfer.TransactionID)),
		From:            strings.TrimSpace(transfer.From),
		To:              strings.TrimSpace(transfer.To),
		Token:           strings.ToUpper(strings.TrimSpace(transfer.TokenInfo.Symbol)),
		ContractAddress: strings.TrimSpace(transfer.TokenInfo.Address),
		Amount:          amountFloat,
		AmountText:      amountText,
		BlockTime:       blockTime,
	}, nil
}

func parseCryptoTokenDecimals(raw any, fallback int) int {
	switch v := raw.(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return fallback
}
