package service

import (
	"context"
	"errors"
	"testing"
)

type yunmaoGatewayUserRepoStub struct {
	UserRepository
	usersByID    map[int64]*User
	usersByEmail map[string]*User
}

func (r *yunmaoGatewayUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	if user, ok := r.usersByID[id]; ok {
		return user, nil
	}
	return nil, errors.New("not found")
}

func (r *yunmaoGatewayUserRepoStub) GetByEmail(ctx context.Context, email string) (*User, error) {
	if user, ok := r.usersByEmail[email]; ok {
		return user, nil
	}
	return nil, errors.New("not found")
}

type yunmaoGatewayRedeemRepoStub struct {
	RedeemCodeRepository
	codes map[string]*RedeemCode
}

func (r *yunmaoGatewayRedeemRepoStub) GetByCode(ctx context.Context, code string) (*RedeemCode, error) {
	if redeemCode, ok := r.codes[code]; ok {
		copyCode := *redeemCode
		return &copyCode, nil
	}
	return nil, ErrRedeemCodeNotFound
}

func TestYunmaoGatewayHelpers(t *testing.T) {
	t.Setenv("YUNMAO_GATEWAY_APIKEY", "secret-key")
	t.Setenv("YUNMAO_DEFAULT_CREDIT_AMOUNT", "9.9")

	if !verifyYunmaoGatewayAPIKey("Bearer secret-key") {
		t.Fatal("expected bearer api key to verify")
	}
	if verifyYunmaoGatewayAPIKey("wrong-key") {
		t.Fatal("expected wrong api key to fail")
	}

	amount, ok := configuredYunmaoDefaultCreditAmount()
	if !ok || amount != 9.9 {
		t.Fatalf("expected configured amount 9.9, got %v ok=%v", amount, ok)
	}

	code1 := yunmaoRedeemCode("order-123")
	code2 := yunmaoRedeemCode("order-123")
	if code1 != code2 || len(code1) != 27 || code1[:3] != yunmaoGatewayCodePrefix {
		t.Fatalf("unexpected deterministic code: %q / %q", code1, code2)
	}
}

func TestYunmaoGatewayResolveUser(t *testing.T) {
	user := &User{ID: 7, Email: "buyer@example.com"}
	svc := &PaymentService{userRepo: &yunmaoGatewayUserRepoStub{
		usersByID:    map[int64]*User{7: user},
		usersByEmail: map[string]*User{"buyer@example.com": user},
	}}

	resolved, err := svc.resolveYunmaoGatewayUser(context.Background(), YunmaoGatewayRequest{UserID: "7"})
	if err != nil || resolved.ID != 7 {
		t.Fatalf("resolve by user_id failed: user=%v err=%v", resolved, err)
	}
	resolved, err = svc.resolveYunmaoGatewayUser(context.Background(), YunmaoGatewayRequest{Email: "BUYER@example.com"})
	if err != nil || resolved.ID != 7 {
		t.Fatalf("resolve by email failed: user=%v err=%v", resolved, err)
	}
}

func TestYunmaoGatewayUsesServerConfiguredAmountForIdempotentResponse(t *testing.T) {
	t.Setenv("YUNMAO_GATEWAY_APIKEY", "secret-key")
	t.Setenv("YUNMAO_DEFAULT_CREDIT_AMOUNT", "9.9")
	userID := int64(7)
	code := yunmaoRedeemCode("order-server-amount")
	redeemRepo := &yunmaoGatewayRedeemRepoStub{codes: map[string]*RedeemCode{
		code: &RedeemCode{ID: 1, Code: code, Type: RedeemTypeBalance, Value: 9.9, Status: StatusUsed, UsedBy: &userID},
	}}
	redeemSvc := &RedeemService{redeemRepo: redeemRepo}
	svc := &PaymentService{
		userRepo:      &yunmaoGatewayUserRepoStub{usersByID: map[int64]*User{userID: &User{ID: userID, Email: "buyer@example.com"}}},
		redeemService: redeemSvc,
	}

	result, err := svc.HandleYunmaoGatewayFulfillment(context.Background(), YunmaoGatewayRequest{
		APIKey:  "secret-key",
		OrderID: "order-server-amount",
		UserID:  "7",
		Amount:  9999,
	})
	if err != nil {
		t.Fatalf("expected fulfillment to ignore client amount, got err=%v", err)
	}
	if result.CreditAmount != 9.9 {
		t.Fatalf("expected server configured amount 9.9, got %v", result.CreditAmount)
	}
}

func TestYunmaoGatewayRequiresServerConfiguredAmount(t *testing.T) {
	t.Setenv("YUNMAO_GATEWAY_APIKEY", "secret-key")
	t.Setenv("YUNMAO_DEFAULT_CREDIT_AMOUNT", "")
	svc := &PaymentService{
		userRepo:      &yunmaoGatewayUserRepoStub{usersByID: map[int64]*User{7: &User{ID: 7, Email: "buyer@example.com"}}},
		redeemService: &RedeemService{redeemRepo: &yunmaoGatewayRedeemRepoStub{codes: map[string]*RedeemCode{}}},
	}

	_, err := svc.HandleYunmaoGatewayFulfillment(context.Background(), YunmaoGatewayRequest{
		APIKey:  "secret-key",
		OrderID: "order-no-server-amount",
		UserID:  "7",
		Amount:  9.9,
	})
	if err == nil {
		t.Fatal("expected missing server amount to fail")
	}
}

func TestYunmaoGatewayAlreadyUsedByDifferentUser(t *testing.T) {
	t.Setenv("YUNMAO_GATEWAY_APIKEY", "secret-key")
	t.Setenv("YUNMAO_DEFAULT_CREDIT_AMOUNT", "9.9")
	ownerID := int64(99)
	code := yunmaoRedeemCode("order-dup")
	redeemRepo := &yunmaoGatewayRedeemRepoStub{codes: map[string]*RedeemCode{
		code: &RedeemCode{ID: 1, Code: code, Type: RedeemTypeBalance, Value: 9.9, Status: StatusUsed, UsedBy: &ownerID},
	}}
	redeemSvc := &RedeemService{redeemRepo: redeemRepo}
	svc := &PaymentService{
		userRepo:      &yunmaoGatewayUserRepoStub{usersByID: map[int64]*User{7: &User{ID: 7, Email: "buyer@example.com"}}},
		redeemService: redeemSvc,
	}

	_, err := svc.HandleYunmaoGatewayFulfillment(context.Background(), YunmaoGatewayRequest{
		APIKey:  "secret-key",
		OrderID: "order-dup",
		UserID:  "7",
		Amount:  9.9,
	})
	if err == nil {
		t.Fatal("expected conflict for order already fulfilled by another user")
	}
}
