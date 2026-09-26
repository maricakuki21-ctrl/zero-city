package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	yunmaoGatewayCodePrefix = "YM-"
	yunmaoGatewayProvider   = "yunmao"
)

type YunmaoGatewayRequest struct {
	APIKey   string
	OrderID  string
	TradeNo  string
	UserID   string
	Email    string
	Account  string
	Amount   float64
	Currency string
	Raw      map[string]string
}

type YunmaoGatewayResponse struct {
	Status       string  `json:"status"`
	OrderID      string  `json:"order_id"`
	TradeNo      string  `json:"trade_no,omitempty"`
	UserID       int64   `json:"user_id"`
	Email        string  `json:"email,omitempty"`
	CreditAmount float64 `json:"credit_amount"`
	RedeemCode   string  `json:"redeem_code"`
	AlreadyDone  bool    `json:"already_done"`
}

func (s *PaymentService) HandleYunmaoGatewayFulfillment(ctx context.Context, req YunmaoGatewayRequest) (*YunmaoGatewayResponse, error) {
	if s == nil || s.userRepo == nil || s.redeemService == nil {
		return nil, infraerrors.New(500, "YUNMAO_GATEWAY_UNAVAILABLE", "yunmao gateway is unavailable")
	}
	if !verifyYunmaoGatewayAPIKey(req.APIKey) {
		return nil, infraerrors.Forbidden("YUNMAO_GATEWAY_FORBIDDEN", "invalid yunmao gateway api key")
	}

	orderID := firstNonEmpty(req.OrderID, req.TradeNo)
	if orderID == "" {
		return nil, infraerrors.BadRequest("YUNMAO_ORDER_ID_REQUIRED", "order_id or trade_no is required")
	}

	amount, ok := configuredYunmaoDefaultCreditAmount()
	if !ok || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, infraerrors.BadRequest("YUNMAO_AMOUNT_INVALID", "YUNMAO_DEFAULT_CREDIT_AMOUNT must be configured and greater than 0")
	}

	user, err := s.resolveYunmaoGatewayUser(ctx, req)
	if err != nil {
		return nil, err
	}

	code := yunmaoRedeemCode(orderID)
	existing, lookupErr := s.redeemService.GetByCode(ctx, code)
	if lookupErr != nil && !errors.Is(lookupErr, ErrRedeemCodeNotFound) {
		return nil, fmt.Errorf("lookup yunmao redeem code: %w", lookupErr)
	}
	if existing == nil || errors.Is(lookupErr, ErrRedeemCodeNotFound) {
		notes := buildYunmaoGatewayNotes(orderID, req.TradeNo, req.Currency)
		createErr := s.redeemService.CreateCode(ctx, &RedeemCode{
			Code:   code,
			Type:   RedeemTypeBalance,
			Value:  amount,
			Status: StatusUnused,
			Notes:  notes,
		})
		if createErr != nil {
			// If two callbacks arrive at the same time, another request may have
			// created the deterministic code first. Reload and continue idempotently.
			existing, lookupErr = s.redeemService.GetByCode(ctx, code)
			if lookupErr != nil || existing == nil {
				return nil, fmt.Errorf("create yunmao redeem code: %w", createErr)
			}
		}
	}

	if existing != nil && existing.IsUsed() {
		if existing.UsedBy != nil && *existing.UsedBy == user.ID {
			return &YunmaoGatewayResponse{
				Status:       "success",
				OrderID:      orderID,
				TradeNo:      strings.TrimSpace(req.TradeNo),
				UserID:       user.ID,
				Email:        user.Email,
				CreditAmount: existing.Value,
				RedeemCode:   code,
				AlreadyDone:  true,
			}, nil
		}
		return nil, infraerrors.Conflict("YUNMAO_ORDER_ALREADY_USED", "yunmao order has already been fulfilled for another user")
	}

	if _, err := s.redeemService.Redeem(ctx, user.ID, code); err != nil {
		if errors.Is(err, ErrRedeemCodeUsed) {
			redeemed, lookupErr := s.redeemService.GetByCode(ctx, code)
			if lookupErr != nil {
				return nil, fmt.Errorf("reload used yunmao redeem code: %w", lookupErr)
			}
			if redeemed != nil && redeemed.UsedBy != nil && *redeemed.UsedBy == user.ID {
				return &YunmaoGatewayResponse{
					Status:       "success",
					OrderID:      orderID,
					TradeNo:      strings.TrimSpace(req.TradeNo),
					UserID:       user.ID,
					Email:        user.Email,
					CreditAmount: redeemed.Value,
					RedeemCode:   code,
					AlreadyDone:  true,
				}, nil
			}
			return nil, infraerrors.Conflict("YUNMAO_ORDER_ALREADY_USED", "yunmao order has already been fulfilled for another user")
		}
		return nil, err
	}

	return &YunmaoGatewayResponse{
		Status:       "success",
		OrderID:      orderID,
		TradeNo:      strings.TrimSpace(req.TradeNo),
		UserID:       user.ID,
		Email:        user.Email,
		CreditAmount: amount,
		RedeemCode:   code,
		AlreadyDone:  false,
	}, nil
}

func (s *PaymentService) resolveYunmaoGatewayUser(ctx context.Context, req YunmaoGatewayRequest) (*User, error) {
	if idText := strings.TrimSpace(req.UserID); idText != "" {
		id, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || id <= 0 {
			return nil, infraerrors.BadRequest("YUNMAO_USER_ID_INVALID", "user_id must be a positive integer")
		}
		user, err := s.userRepo.GetByID(ctx, id)
		if err != nil {
			return nil, infraerrors.NotFound("YUNMAO_USER_NOT_FOUND", "user not found")
		}
		return user, nil
	}

	identifier := strings.TrimSpace(firstNonEmpty(req.Email, req.Account))
	if identifier == "" {
		return nil, infraerrors.BadRequest("YUNMAO_USER_REQUIRED", "user_id or email is required")
	}
	if id, err := strconv.ParseInt(identifier, 10, 64); err == nil && id > 0 {
		user, err := s.userRepo.GetByID(ctx, id)
		if err != nil {
			return nil, infraerrors.NotFound("YUNMAO_USER_NOT_FOUND", "user not found")
		}
		return user, nil
	}
	if !strings.Contains(identifier, "@") {
		return nil, infraerrors.BadRequest("YUNMAO_EMAIL_INVALID", "email must be a valid BizDecipher login email")
	}
	user, err := s.userRepo.GetByEmail(ctx, strings.ToLower(identifier))
	if err != nil {
		return nil, infraerrors.NotFound("YUNMAO_USER_NOT_FOUND", "user not found")
	}
	return user, nil
}

func verifyYunmaoGatewayAPIKey(input string) bool {
	expected := strings.TrimSpace(firstNonEmpty(os.Getenv("YUNMAO_GATEWAY_APIKEY"), os.Getenv("CATFK_GATEWAY_APIKEY")))
	if expected == "" {
		return false
	}
	input = strings.TrimSpace(strings.TrimPrefix(input, "Bearer "))
	if len(input) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(input), []byte(expected)) == 1
}

func configuredYunmaoDefaultCreditAmount() (float64, bool) {
	raw := strings.TrimSpace(os.Getenv("YUNMAO_DEFAULT_CREDIT_AMOUNT"))
	if raw == "" {
		return 0, false
	}
	amount, err := strconv.ParseFloat(raw, 64)
	if err != nil || amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, false
	}
	return amount, true
}

func yunmaoRedeemCode(orderID string) string {
	sum := sha256.Sum256([]byte(yunmaoGatewayProvider + ":" + strings.TrimSpace(orderID)))
	return yunmaoGatewayCodePrefix + strings.ToUpper(hex.EncodeToString(sum[:])[:24])
}

func buildYunmaoGatewayNotes(orderID, tradeNo, currency string) string {
	parts := []string{"source=yunmao", "order_id=" + strings.TrimSpace(orderID)}
	if tradeNo = strings.TrimSpace(tradeNo); tradeNo != "" && tradeNo != strings.TrimSpace(orderID) {
		parts = append(parts, "trade_no="+tradeNo)
	}
	if currency = strings.TrimSpace(currency); currency != "" {
		parts = append(parts, "currency="+currency)
	}
	return strings.Join(parts, " ")
}
