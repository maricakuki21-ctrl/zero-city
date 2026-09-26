package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	PayhipEventPaid     = "paid"
	PayhipEventRefunded = "refunded"

	PayhipOrderStatusPaid     = "paid"
	PayhipOrderStatusClaimed  = "claimed"
	PayhipOrderStatusRefunded = "refunded"

	payhipDefaultProductKey   = "38YbA"
	payhipDefaultCreditCents  = 990
	payhipDefaultCreditAmount = 9.90
	payhipRedeemCodePrefix    = "PH-"
)

var (
	ErrPayhipDisabled           = infraerrors.BadRequest("PAYHIP_DISABLED", "Payhip webhook is not configured")
	ErrPayhipInvalidEvent       = infraerrors.BadRequest("PAYHIP_INVALID_EVENT", "invalid Payhip event")
	ErrPayhipInvalidSig         = infraerrors.Unauthorized("PAYHIP_INVALID_SIGNATURE", "invalid Payhip signature")
	ErrPayhipUnsupportedProduct = infraerrors.BadRequest("PAYHIP_UNSUPPORTED_PRODUCT", "unsupported Payhip product")
	ErrPayhipOrderNotFound      = infraerrors.NotFound("PAYHIP_ORDER_NOT_FOUND", "Payhip order not found")
	ErrPayhipOrderRefunded      = infraerrors.Conflict("PAYHIP_ORDER_REFUNDED", "Payhip order has been refunded")
	ErrPayhipEmailMismatch      = infraerrors.Forbidden("PAYHIP_EMAIL_MISMATCH", "Payhip buyer email does not match")
)

type PayhipProductConfig struct {
	ProductKey   string  `json:"product_key"`
	PurchaseURL  string  `json:"purchase_url"`
	Currency     string  `json:"currency"`
	CreditCents  int64   `json:"credit_cents"`
	CreditAmount float64 `json:"credit_amount"`
	Title        string  `json:"title,omitempty"`
}

type PayhipConfig struct {
	APIKey       string
	ProductKey   string
	PurchaseURL  string
	Currency     string
	CreditCents  int64
	CreditAmount float64
	Products     []PayhipProductConfig
}

func LoadPayhipConfigFromEnv() PayhipConfig {
	cfg := PayhipConfig{
		APIKey:       strings.TrimSpace(os.Getenv("PAYHIP_API_KEY")),
		ProductKey:   strings.TrimSpace(os.Getenv("PAYHIP_PRODUCT_KEY")),
		PurchaseURL:  strings.TrimSpace(os.Getenv("PAYHIP_PURCHASE_URL")),
		Currency:     strings.ToUpper(strings.TrimSpace(os.Getenv("PAYHIP_CURRENCY"))),
		CreditCents:  payhipDefaultCreditCents,
		CreditAmount: payhipDefaultCreditAmount,
	}
	if raw := strings.TrimSpace(os.Getenv("PAYHIP_CREDIT_CENTS")); raw != "" {
		if cents, err := strconv.ParseInt(raw, 10, 64); err == nil && cents > 0 {
			cfg.CreditCents = cents
		}
	}
	if raw := strings.TrimSpace(os.Getenv("PAYHIP_CREDIT_AMOUNT")); raw != "" {
		if amount, err := strconv.ParseFloat(raw, 64); err == nil && amount > 0 {
			cfg.CreditAmount = math.Round(amount*100) / 100
		}
	}
	if raw := strings.TrimSpace(os.Getenv("PAYHIP_PRODUCTS")); raw != "" {
		var products []PayhipProductConfig
		if err := json.Unmarshal([]byte(raw), &products); err == nil {
			cfg.Products = products
		} else {
			slog.Warn("invalid PAYHIP_PRODUCTS config, using legacy Payhip product", "error", err)
		}
	}
	return normalizePayhipConfig(cfg)
}

func normalizePayhipConfig(cfg PayhipConfig) PayhipConfig {
	if len(cfg.Products) == 0 {
		cfg.Products = []PayhipProductConfig{{
			ProductKey:   cfg.ProductKey,
			PurchaseURL:  cfg.PurchaseURL,
			Currency:     cfg.Currency,
			CreditCents:  cfg.CreditCents,
			CreditAmount: cfg.CreditAmount,
		}}
	}
	cfg.Products = normalizePayhipProducts(cfg.Products)
	if len(cfg.Products) > 0 {
		first := cfg.Products[0]
		cfg.ProductKey = first.ProductKey
		cfg.PurchaseURL = first.PurchaseURL
		cfg.Currency = first.Currency
		cfg.CreditCents = first.CreditCents
		cfg.CreditAmount = first.CreditAmount
	}
	return cfg
}

func normalizePayhipProducts(products []PayhipProductConfig) []PayhipProductConfig {
	seen := map[string]struct{}{}
	out := make([]PayhipProductConfig, 0, len(products))
	for _, product := range products {
		product.ProductKey = strings.TrimSpace(product.ProductKey)
		if product.ProductKey == "" {
			product.ProductKey = payhipDefaultProductKey
		}
		product.PurchaseURL = strings.TrimSpace(product.PurchaseURL)
		if product.PurchaseURL == "" {
			product.PurchaseURL = "https://payhip.com/b/" + product.ProductKey
		}
		product.Currency = strings.ToUpper(strings.TrimSpace(product.Currency))
		if product.Currency == "" {
			product.Currency = "USD"
		}
		if product.CreditCents <= 0 {
			product.CreditCents = payhipDefaultCreditCents
		}
		if product.CreditAmount <= 0 {
			product.CreditAmount = math.Round(float64(product.CreditCents)) / 100
		} else {
			product.CreditAmount = math.Round(product.CreditAmount*100) / 100
		}
		product.Title = strings.TrimSpace(product.Title)
		fingerprint := fmt.Sprintf("%s:%d:%s", product.ProductKey, product.CreditCents, product.Currency)
		if _, ok := seen[fingerprint]; ok {
			continue
		}
		seen[fingerprint] = struct{}{}
		out = append(out, product)
	}
	return out
}

func (c PayhipConfig) Enabled() bool {
	return strings.TrimSpace(c.APIKey) != ""
}

type PayhipItem struct {
	ProductID        string `json:"product_id"`
	ProductName      string `json:"product_name"`
	ProductKey       string `json:"product_key"`
	ProductPermalink string `json:"product_permalink"`
	Quantity         string `json:"quantity"`
}

type PayhipWebhookPayload struct {
	ID             string       `json:"id"`
	Email          string       `json:"email"`
	Currency       string       `json:"currency"`
	Price          int64        `json:"price"`
	Items          []PayhipItem `json:"items"`
	PaymentType    string       `json:"payment_type"`
	AmountRefunded int64        `json:"amount_refunded"`
	Date           int64        `json:"date"`
	DateCreated    int64        `json:"date_created"`
	DateRefunded   int64        `json:"date_refunded"`
	Type           string       `json:"type"`
	Signature      string       `json:"signature"`
}

type PayhipOrder struct {
	ID             int64
	TransactionID  string
	BuyerEmail     string
	Currency       string
	PriceCents     int64
	CreditAmount   float64
	ProductKey     string
	ProductName    string
	ProductLink    string
	PaymentType    string
	Status         string
	RedeemCode     string
	ClaimedBy      *int64
	ClaimedAt      *time.Time
	RefundedAt     *time.Time
	AmountRefunded int64
	RawPayload     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PayhipOrderRepository interface {
	CreatePaidOrder(ctx context.Context, order *PayhipOrder) (*PayhipOrder, bool, error)
	MarkRefunded(ctx context.Context, transactionID string, amountRefunded int64, rawPayload string, refundedAt time.Time) (*PayhipOrder, error)
	GetByTransactionID(ctx context.Context, transactionID string) (*PayhipOrder, error)
	MarkClaimed(ctx context.Context, transactionID string, userID int64) (*PayhipOrder, error)
}

type PayhipRedeemer interface {
	CreateCode(ctx context.Context, code *RedeemCode) error
	GetByCode(ctx context.Context, code string) (*RedeemCode, error)
	Redeem(ctx context.Context, userID int64, code string) (*RedeemCode, error)
}

type PayhipService struct {
	repo          PayhipOrderRepository
	redeemService PayhipRedeemer
	config        PayhipConfig
}

func NewPayhipService(repo PayhipOrderRepository, redeemService *RedeemService) *PayhipService {
	return NewPayhipServiceWithConfig(repo, redeemService, LoadPayhipConfigFromEnv())
}

func NewPayhipServiceWithConfig(repo PayhipOrderRepository, redeemService PayhipRedeemer, cfg PayhipConfig) *PayhipService {
	return &PayhipService{repo: repo, redeemService: redeemService, config: normalizePayhipConfig(cfg)}
}

func (s *PayhipService) Config() PayhipConfig {
	if s == nil {
		return PayhipConfig{}
	}
	return s.config
}

func (s *PayhipService) HandleWebhook(ctx context.Context, rawBody []byte) (*PayhipOrder, error) {
	if s == nil || s.repo == nil || s.redeemService == nil || !s.config.Enabled() {
		return nil, ErrPayhipDisabled
	}
	var payload PayhipWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("parse Payhip webhook: %w", err)
	}
	payload.ID = strings.TrimSpace(payload.ID)
	payload.Type = strings.TrimSpace(payload.Type)
	payload.Email = strings.ToLower(strings.TrimSpace(payload.Email))
	if payload.ID == "" || payload.Type == "" {
		return nil, ErrPayhipInvalidEvent
	}
	if !s.verifySignature(payload.Signature) {
		return nil, ErrPayhipInvalidSig
	}

	switch payload.Type {
	case PayhipEventPaid:
		return s.handlePaid(ctx, payload, string(rawBody))
	case PayhipEventRefunded:
		return s.handleRefunded(ctx, payload, string(rawBody))
	default:
		return nil, ErrPayhipInvalidEvent
	}
}

func (s *PayhipService) handlePaid(ctx context.Context, payload PayhipWebhookPayload, rawPayload string) (*PayhipOrder, error) {
	payloadCurrency := strings.ToUpper(strings.TrimSpace(payload.Currency))
	item, product, ok := s.matchConfiguredItem(payload.Items, payload.Price, payloadCurrency)
	if !ok {
		return nil, ErrPayhipUnsupportedProduct
	}
	if existing, err := s.repo.GetByTransactionID(ctx, payload.ID); err == nil && existing != nil {
		if err := s.ensureRedeemCode(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	} else if err != nil && !errors.Is(err, ErrPayhipOrderNotFound) {
		return nil, err
	}

	code, err := s.generateUniqueRedeemCode(ctx)
	if err != nil {
		return nil, err
	}
	createdAt := unixTimeOrNow(payload.Date)
	order := &PayhipOrder{
		TransactionID: payload.ID,
		BuyerEmail:    payload.Email,
		Currency:      strings.ToUpper(strings.TrimSpace(payload.Currency)),
		PriceCents:    payload.Price,
		CreditAmount:  product.CreditAmount,
		ProductKey:    item.ProductKey,
		ProductName:   item.ProductName,
		ProductLink:   item.ProductPermalink,
		PaymentType:   payload.PaymentType,
		Status:        PayhipOrderStatusPaid,
		RedeemCode:    code,
		RawPayload:    rawPayload,
		CreatedAt:     createdAt,
	}
	created, _, err := s.repo.CreatePaidOrder(ctx, order)
	if err != nil {
		return nil, err
	}
	if err := s.ensureRedeemCode(ctx, created); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *PayhipService) ensureRedeemCode(ctx context.Context, order *PayhipOrder) error {
	if order == nil || strings.TrimSpace(order.RedeemCode) == "" {
		return errors.New("payhip redeem code is required")
	}
	existing, err := s.redeemService.GetByCode(ctx, order.RedeemCode)
	if err == nil && existing != nil {
		return nil
	}
	if err != nil && !errors.Is(err, ErrRedeemCodeNotFound) {
		return fmt.Errorf("check Payhip redeem code: %w", err)
	}
	if err := s.redeemService.CreateCode(ctx, &RedeemCode{
		Code:   order.RedeemCode,
		Type:   RedeemTypeBalance,
		Value:  order.CreditAmount,
		Status: StatusUnused,
		Notes:  "Payhip transaction " + order.TransactionID,
	}); err != nil {
		return fmt.Errorf("create Payhip redeem code: %w", err)
	}
	return nil
}

func (s *PayhipService) handleRefunded(ctx context.Context, payload PayhipWebhookPayload, rawPayload string) (*PayhipOrder, error) {
	refundedAt := unixTimeOrNow(payload.DateRefunded)
	order, err := s.repo.MarkRefunded(ctx, payload.ID, payload.AmountRefunded, rawPayload, refundedAt)
	if err != nil {
		if errors.Is(err, ErrPayhipOrderNotFound) {
			slog.Warn("Payhip refund webhook references unknown transaction", "transaction_id", payload.ID)
			return nil, nil
		}
		return nil, err
	}
	return order, nil
}

func (s *PayhipService) Claim(ctx context.Context, userID int64, transactionID, email string) (*PayhipOrder, *RedeemCode, error) {
	if s == nil || s.repo == nil || s.redeemService == nil {
		return nil, nil, ErrPayhipDisabled
	}
	transactionID = strings.TrimSpace(transactionID)
	email = strings.ToLower(strings.TrimSpace(email))
	if transactionID == "" || email == "" || userID <= 0 {
		return nil, nil, infraerrors.BadRequest("PAYHIP_CLAIM_INVALID", "transaction id and buyer email are required")
	}
	order, err := s.repo.GetByTransactionID(ctx, transactionID)
	if err != nil {
		return nil, nil, err
	}
	if order.Status == PayhipOrderStatusRefunded {
		return nil, nil, ErrPayhipOrderRefunded
	}
	if strings.ToLower(strings.TrimSpace(order.BuyerEmail)) != email {
		return nil, nil, ErrPayhipEmailMismatch
	}
	redeemed, err := s.redeemService.Redeem(ctx, userID, order.RedeemCode)
	if err != nil {
		if errors.Is(err, ErrRedeemCodeUsed) && s.redeemCodeBelongsToUser(ctx, order.RedeemCode, userID, order) {
			updated, updateErr := s.repo.MarkClaimed(ctx, transactionID, userID)
			if updateErr != nil {
				return nil, nil, updateErr
			}
			return updated, nil, nil
		}
		return nil, nil, err
	}
	updated, err := s.repo.MarkClaimed(ctx, transactionID, userID)
	if err != nil {
		return nil, nil, err
	}
	return updated, redeemed, nil
}

func (s *PayhipService) redeemCodeBelongsToUser(ctx context.Context, code string, userID int64, order *PayhipOrder) bool {
	if order != nil && order.ClaimedBy != nil && *order.ClaimedBy == userID {
		return true
	}
	redeemCode, err := s.redeemService.GetByCode(ctx, code)
	return err == nil && redeemCode != nil && redeemCode.UsedBy != nil && *redeemCode.UsedBy == userID
}

func (s *PayhipService) verifySignature(signature string) bool {
	signature = strings.ToLower(strings.TrimSpace(signature))
	if signature == "" || strings.TrimSpace(s.config.APIKey) == "" {
		return false
	}
	sum := sha256.Sum256([]byte(s.config.APIKey))
	expected := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) == 1
}

func (s *PayhipService) matchConfiguredItem(items []PayhipItem, price int64, currency string) (PayhipItem, PayhipProductConfig, bool) {
	products := s.config.Products
	if len(products) == 0 {
		products = normalizePayhipConfig(s.config).Products
	}
	for _, item := range items {
		itemKey := strings.TrimSpace(item.ProductKey)
		itemPermalink := strings.TrimSpace(item.ProductPermalink)
		for _, product := range products {
			configured := strings.TrimSpace(product.ProductKey)
			productCurrency := strings.ToUpper(strings.TrimSpace(product.Currency))
			keyMatches := itemKey == configured || strings.HasSuffix(itemPermalink, "/b/"+configured)
			if keyMatches && product.CreditCents == price && productCurrency == currency {
				if item.ProductKey == "" {
					item.ProductKey = configured
				}
				return item, product, true
			}
		}
	}
	return PayhipItem{}, PayhipProductConfig{}, false
}

func (s *PayhipService) generateUniqueRedeemCode(ctx context.Context) (string, error) {
	for i := 0; i < 5; i++ {
		random, err := GenerateRedeemCode()
		if err != nil {
			return "", err
		}
		code := payhipRedeemCodePrefix + strings.ToUpper(random[:16])
		if _, err := s.redeemService.GetByCode(ctx, code); errors.Is(err, ErrRedeemCodeNotFound) {
			return code, nil
		}
	}
	return "", fmt.Errorf("generate unique Payhip redeem code failed")
}

func unixTimeOrNow(ts int64) time.Time {
	if ts <= 0 {
		return time.Now().UTC()
	}
	return time.Unix(ts, 0).UTC()
}

func ScanPayhipOrderForRepository(row interface{ Scan(dest ...any) error }) (*PayhipOrder, error) {
	var order PayhipOrder
	var claimedBy sql.NullInt64
	var claimedAt sql.NullTime
	var refundedAt sql.NullTime
	if err := row.Scan(
		&order.ID,
		&order.TransactionID,
		&order.BuyerEmail,
		&order.Currency,
		&order.PriceCents,
		&order.CreditAmount,
		&order.ProductKey,
		&order.ProductName,
		&order.ProductLink,
		&order.PaymentType,
		&order.Status,
		&order.RedeemCode,
		&claimedBy,
		&claimedAt,
		&refundedAt,
		&order.AmountRefunded,
		&order.RawPayload,
		&order.CreatedAt,
		&order.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if claimedBy.Valid {
		order.ClaimedBy = &claimedBy.Int64
	}
	if claimedAt.Valid {
		order.ClaimedAt = &claimedAt.Time
	}
	if refundedAt.Valid {
		order.RefundedAt = &refundedAt.Time
	}
	return &order, nil
}
