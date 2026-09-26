//go:build unit

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type payhipOrderRepoStub struct {
	orders map[string]*PayhipOrder
}

func newPayhipOrderRepoStub() *payhipOrderRepoStub {
	return &payhipOrderRepoStub{orders: map[string]*PayhipOrder{}}
}

func (r *payhipOrderRepoStub) CreatePaidOrder(ctx context.Context, order *PayhipOrder) (*PayhipOrder, bool, error) {
	if existing, ok := r.orders[order.TransactionID]; ok {
		return existing, false, nil
	}
	copy := *order
	copy.ID = int64(len(r.orders) + 1)
	r.orders[copy.TransactionID] = &copy
	return &copy, true, nil
}

func (r *payhipOrderRepoStub) MarkRefunded(ctx context.Context, transactionID string, amountRefunded int64, rawPayload string, refundedAt time.Time) (*PayhipOrder, error) {
	order, ok := r.orders[transactionID]
	if !ok {
		return nil, ErrPayhipOrderNotFound
	}
	order.Status = PayhipOrderStatusRefunded
	order.AmountRefunded = amountRefunded
	order.RefundedAt = &refundedAt
	order.RawPayload = rawPayload
	return order, nil
}

func (r *payhipOrderRepoStub) GetByTransactionID(ctx context.Context, transactionID string) (*PayhipOrder, error) {
	order, ok := r.orders[transactionID]
	if !ok {
		return nil, ErrPayhipOrderNotFound
	}
	return order, nil
}

func (r *payhipOrderRepoStub) MarkClaimed(ctx context.Context, transactionID string, userID int64) (*PayhipOrder, error) {
	order, ok := r.orders[transactionID]
	if !ok {
		return nil, ErrPayhipOrderNotFound
	}
	if order.Status != PayhipOrderStatusRefunded {
		order.Status = PayhipOrderStatusClaimed
	}
	if order.ClaimedBy == nil {
		order.ClaimedBy = &userID
		now := time.Now().UTC()
		order.ClaimedAt = &now
	}
	return order, nil
}

type payhipRedeemerStub struct {
	codes map[string]*RedeemCode
}

func newPayhipRedeemerStub() *payhipRedeemerStub {
	return &payhipRedeemerStub{codes: map[string]*RedeemCode{}}
}

func (r *payhipRedeemerStub) CreateCode(ctx context.Context, code *RedeemCode) error {
	if code == nil || code.Code == "" {
		return errors.New("code is required")
	}
	if _, ok := r.codes[code.Code]; ok {
		return errors.New("duplicate code")
	}
	copy := *code
	copy.ID = int64(len(r.codes) + 1)
	r.codes[copy.Code] = &copy
	return nil
}

func (r *payhipRedeemerStub) GetByCode(ctx context.Context, code string) (*RedeemCode, error) {
	if c, ok := r.codes[code]; ok {
		copy := *c
		return &copy, nil
	}
	return nil, ErrRedeemCodeNotFound
}

func (r *payhipRedeemerStub) Redeem(ctx context.Context, userID int64, code string) (*RedeemCode, error) {
	c, ok := r.codes[code]
	if !ok {
		return nil, ErrRedeemCodeNotFound
	}
	if c.Status == StatusUsed {
		return nil, ErrRedeemCodeUsed
	}
	c.Status = StatusUsed
	c.UsedBy = &userID
	now := time.Now().UTC()
	c.UsedAt = &now
	copy := *c
	return &copy, nil
}

func signedPayhipPayload(t *testing.T, apiKey string, payload map[string]any) []byte {
	t.Helper()
	sum := sha256.Sum256([]byte(apiKey))
	payload["signature"] = hex.EncodeToString(sum[:])
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}

func newPayhipTestService() (*PayhipService, *payhipOrderRepoStub, *payhipRedeemerStub) {
	orderRepo := newPayhipOrderRepoStub()
	redeemer := newPayhipRedeemerStub()
	svc := NewPayhipServiceWithConfig(orderRepo, redeemer, PayhipConfig{APIKey: "test-api-key", ProductKey: "38YbA", PurchaseURL: "https://payhip.com/b/38YbA", Currency: "USD", CreditCents: 990, CreditAmount: 9.90})
	return svc, orderRepo, redeemer
}

func validPaidPayload(t *testing.T, apiKey string) []byte {
	return signedPayhipPayload(t, apiKey, map[string]any{
		"id":           "txn_123",
		"email":        "buyer@example.com",
		"currency":     "USD",
		"price":        990,
		"items":        []map[string]any{{"product_key": "38YbA", "product_name": "BizDecipher AI Gateway Credits", "product_permalink": "https://payhip.com/b/38YbA", "quantity": "1"}},
		"payment_type": "paypal",
		"date":         int64(1703693218),
		"type":         "paid",
	})
}

func TestPayhipWebhookPaidCreatesRedeemCode(t *testing.T) {
	svc, orderRepo, redeemer := newPayhipTestService()
	order, err := svc.HandleWebhook(context.Background(), validPaidPayload(t, "test-api-key"))
	require.NoError(t, err)
	require.Equal(t, "txn_123", order.TransactionID)
	require.Equal(t, PayhipOrderStatusPaid, order.Status)
	require.Equal(t, 9.90, order.CreditAmount)
	require.Len(t, orderRepo.orders, 1)
	require.Len(t, redeemer.codes, 1)
	require.Contains(t, redeemer.codes, order.RedeemCode)
}

func TestPayhipWebhookPaidUsesMatchedProductTier(t *testing.T) {
	orderRepo := newPayhipOrderRepoStub()
	redeemer := newPayhipRedeemerStub()
	svc := NewPayhipServiceWithConfig(orderRepo, redeemer, PayhipConfig{
		APIKey: "test-api-key",
		Products: []PayhipProductConfig{
			{ProductKey: "SMALL", PurchaseURL: "https://payhip.com/b/SMALL", Currency: "USD", CreditCents: 500, CreditAmount: 5},
			{ProductKey: "LARGE", PurchaseURL: "https://payhip.com/b/LARGE", Currency: "USD", CreditCents: 1990, CreditAmount: 19.90},
		},
	})
	payload := signedPayhipPayload(t, "test-api-key", map[string]any{
		"id":       "txn_large",
		"email":    "buyer@example.com",
		"currency": "USD",
		"price":    1990,
		"items":    []map[string]any{{"product_key": "LARGE", "product_name": "Large credits", "product_permalink": "https://payhip.com/b/LARGE"}},
		"type":     "paid",
	})
	order, err := svc.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	require.Equal(t, "LARGE", order.ProductKey)
	require.Equal(t, 19.90, order.CreditAmount)
	require.Len(t, redeemer.codes, 1)
}

func TestPayhipWebhookPaidAllowsSameProductKeyDifferentPriceTiers(t *testing.T) {
	orderRepo := newPayhipOrderRepoStub()
	redeemer := newPayhipRedeemerStub()
	svc := NewPayhipServiceWithConfig(orderRepo, redeemer, PayhipConfig{
		APIKey: "test-api-key",
		Products: []PayhipProductConfig{
			{ProductKey: "38YbA", Currency: "USD", CreditCents: 990, CreditAmount: 9.90},
			{ProductKey: "38YbA", Currency: "USD", CreditCents: 1990, CreditAmount: 19.90},
		},
	})
	payload := signedPayhipPayload(t, "test-api-key", map[string]any{
		"id":       "txn_same_key_1990",
		"email":    "buyer@example.com",
		"currency": "USD",
		"price":    1990,
		"items":    []map[string]any{{"product_key": "38YbA", "product_permalink": "https://payhip.com/b/38YbA"}},
		"type":     "paid",
	})
	order, err := svc.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	require.Equal(t, "38YbA", order.ProductKey)
	require.Equal(t, 19.90, order.CreditAmount)
}

func TestPayhipWebhookRejectsTierPriceMismatch(t *testing.T) {
	orderRepo := newPayhipOrderRepoStub()
	redeemer := newPayhipRedeemerStub()
	svc := NewPayhipServiceWithConfig(orderRepo, redeemer, PayhipConfig{
		APIKey:   "test-api-key",
		Products: []PayhipProductConfig{{ProductKey: "LARGE", Currency: "USD", CreditCents: 1990, CreditAmount: 19.90}},
	})
	payload := signedPayhipPayload(t, "test-api-key", map[string]any{
		"id":       "txn_bad_price",
		"email":    "buyer@example.com",
		"currency": "USD",
		"price":    990,
		"items":    []map[string]any{{"product_key": "LARGE", "product_permalink": "https://payhip.com/b/LARGE"}},
		"type":     "paid",
	})
	_, err := svc.HandleWebhook(context.Background(), payload)
	require.ErrorIs(t, err, ErrPayhipUnsupportedProduct)
}

func TestPayhipWebhookRejectsInvalidSignature(t *testing.T) {
	svc, _, _ := newPayhipTestService()
	payload := validPaidPayload(t, "wrong-key")
	_, err := svc.HandleWebhook(context.Background(), payload)
	require.ErrorIs(t, err, ErrPayhipInvalidSig)
}

func TestPayhipWebhookRejectsUnsupportedProduct(t *testing.T) {
	svc, _, _ := newPayhipTestService()
	payload := signedPayhipPayload(t, "test-api-key", map[string]any{
		"id":       "txn_other",
		"email":    "buyer@example.com",
		"currency": "USD",
		"price":    990,
		"items":    []map[string]any{{"product_key": "OTHER", "product_permalink": "https://payhip.com/b/OTHER"}},
		"type":     "paid",
	})
	_, err := svc.HandleWebhook(context.Background(), payload)
	require.ErrorIs(t, err, ErrPayhipUnsupportedProduct)
}

func TestPayhipWebhookRejectsCurrencyMismatch(t *testing.T) {
	svc, _, _ := newPayhipTestService()
	payload := signedPayhipPayload(t, "test-api-key", map[string]any{
		"id":           "txn_eur",
		"email":        "buyer@example.com",
		"currency":     "EUR",
		"price":        990,
		"items":        []map[string]any{{"product_key": "38YbA", "product_permalink": "https://payhip.com/b/38YbA"}},
		"payment_type": "card",
		"date":         int64(1703693218),
		"type":         "paid",
	})
	_, err := svc.HandleWebhook(context.Background(), payload)
	require.ErrorIs(t, err, ErrPayhipUnsupportedProduct)
}

func TestPayhipWebhookPaidIsIdempotent(t *testing.T) {
	svc, orderRepo, redeemer := newPayhipTestService()
	payload := validPaidPayload(t, "test-api-key")
	_, err := svc.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	_, err = svc.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	require.Len(t, orderRepo.orders, 1)
	require.Len(t, redeemer.codes, 1)
}

func TestPayhipWebhookPaidRepairsMissingRedeemCode(t *testing.T) {
	svc, orderRepo, redeemer := newPayhipTestService()
	payload := validPaidPayload(t, "test-api-key")
	order, err := svc.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	delete(redeemer.codes, order.RedeemCode)
	_, err = svc.HandleWebhook(context.Background(), payload)
	require.NoError(t, err)
	require.Len(t, orderRepo.orders, 1)
	require.Contains(t, redeemer.codes, order.RedeemCode)
}

func TestPayhipClaimRedeemsForCurrentUser(t *testing.T) {
	svc, _, _ := newPayhipTestService()
	_, err := svc.HandleWebhook(context.Background(), validPaidPayload(t, "test-api-key"))
	require.NoError(t, err)
	order, redeemed, err := svc.Claim(context.Background(), 42, "txn_123", "buyer@example.com")
	require.NoError(t, err)
	require.Equal(t, PayhipOrderStatusClaimed, order.Status)
	require.NotNil(t, redeemed)
	require.Equal(t, int64(42), *redeemed.UsedBy)
}

func TestPayhipClaimIsIdempotentForSameUser(t *testing.T) {
	svc, _, _ := newPayhipTestService()
	_, err := svc.HandleWebhook(context.Background(), validPaidPayload(t, "test-api-key"))
	require.NoError(t, err)
	_, redeemed, err := svc.Claim(context.Background(), 42, "txn_123", "buyer@example.com")
	require.NoError(t, err)
	require.NotNil(t, redeemed)
	order, redeemed, err := svc.Claim(context.Background(), 42, "txn_123", "buyer@example.com")
	require.NoError(t, err)
	require.Equal(t, PayhipOrderStatusClaimed, order.Status)
	require.Nil(t, redeemed)
}

func TestPayhipClaimRepairsOrderWhenRedeemAlreadyUsedByCurrentUser(t *testing.T) {
	svc, orderRepo, redeemer := newPayhipTestService()
	order, err := svc.HandleWebhook(context.Background(), validPaidPayload(t, "test-api-key"))
	require.NoError(t, err)
	userID := int64(42)
	code := redeemer.codes[order.RedeemCode]
	code.Status = StatusUsed
	code.UsedBy = &userID
	orderRepo.orders[order.TransactionID].Status = PayhipOrderStatusPaid
	orderRepo.orders[order.TransactionID].ClaimedBy = nil
	updated, redeemed, err := svc.Claim(context.Background(), userID, "txn_123", "buyer@example.com")
	require.NoError(t, err)
	require.Nil(t, redeemed)
	require.Equal(t, PayhipOrderStatusClaimed, updated.Status)
	require.Equal(t, userID, *updated.ClaimedBy)
}

func TestPayhipClaimRejectsEmailMismatch(t *testing.T) {
	svc, _, _ := newPayhipTestService()
	_, err := svc.HandleWebhook(context.Background(), validPaidPayload(t, "test-api-key"))
	require.NoError(t, err)
	_, _, err = svc.Claim(context.Background(), 42, "txn_123", "other@example.com")
	require.ErrorIs(t, err, ErrPayhipEmailMismatch)
}

func TestPayhipRefundPreventsClaim(t *testing.T) {
	svc, _, _ := newPayhipTestService()
	_, err := svc.HandleWebhook(context.Background(), validPaidPayload(t, "test-api-key"))
	require.NoError(t, err)
	refund := signedPayhipPayload(t, "test-api-key", map[string]any{
		"id":              "txn_123",
		"email":           "buyer@example.com",
		"currency":        "USD",
		"price":           990,
		"amount_refunded": 990,
		"items":           []map[string]any{{"product_key": "38YbA", "product_permalink": "https://payhip.com/b/38YbA"}},
		"type":            "refunded",
		"date_refunded":   int64(1703693410),
	})
	_, err = svc.HandleWebhook(context.Background(), refund)
	require.NoError(t, err)
	_, _, err = svc.Claim(context.Background(), 42, "txn_123", "buyer@example.com")
	require.ErrorIs(t, err, ErrPayhipOrderRefunded)
}
