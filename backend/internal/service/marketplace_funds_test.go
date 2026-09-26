package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMarketplaceFundsMoneyValidation(t *testing.T) {
	for _, amount := range []string{"0", "-1", "1e2", "1.000000001", "NaN", " 2", "1,000", "1000000000000"} {
		_, err := NormalizeMarketplacePayment(MarketplacePaymentInput{ExpectedAmount: amount, Currency: "USD", OperationID: "payment_operation_0001"})
		require.Error(t, err, amount)
	}
	in, err := NormalizeMarketplacePayment(MarketplacePaymentInput{ExpectedAmount: "1.23000001", Currency: "USD", OperationID: "payment_operation_0001"})
	require.NoError(t, err)
	require.Equal(t, "1.23000001", in.ExpectedAmount)
	in.Currency = "CNY"
	_, err = NormalizeMarketplacePayment(in)
	require.Error(t, err)
}
func TestMarketplaceFundsCacheRequired(t *testing.T) {
	s := NewMarketplaceService(&marketplaceRepoStub{})
	require.ErrorIs(t, s.invalidateMarketplaceFunds(context.Background(), 1), ErrMarketplaceFundsClosed)
}

func TestMarketplaceFundsNewOrdersCannotUseLegacyConfirm(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		order := &MarketplaceOrder{ID: 1, BuyerUserID: 7, SellerUserID: 9, Status: "quoted", LegacyCooperation: legacy}
		actions := MarketplaceOrderActions(order, 7)
		if legacy {
			require.Contains(t, actions, "confirm")
		} else {
			require.NotContains(t, actions, "confirm")
		}
		if !legacy {
			s := NewMarketplaceService(&marketplaceRepoStub{order: order})
			_, err := s.ApplyOrderAction(context.Background(), 1, 7, MarketplaceOrderActionInput{Action: "confirm"})
			require.ErrorIs(t, err, ErrMarketplaceFundsInvalid)
		}
	}
}
