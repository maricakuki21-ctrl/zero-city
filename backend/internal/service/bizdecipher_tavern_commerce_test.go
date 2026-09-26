package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTavernTicketMoneyValidation(t *testing.T) {
	for _, p := range []string{"0", "-1", "1e2", "1.123456789", "1000000000000"} {
		_, err := NormalizeTavernTicket(TavernTicketInput{ExpectedPrice: p, Currency: "USD", OperationID: "tavern_ticket_operation_01"})
		require.Error(t, err)
	}
	in, err := NormalizeTavernTicket(TavernTicketInput{ExpectedPrice: "10.12345678", Currency: "USD", OperationID: "tavern_ticket_operation_01"})
	require.NoError(t, err)
	require.Equal(t, "10.12345678", in.ExpectedPrice)
	in.Currency = "CNY"
	_, err = NormalizeTavernTicket(in)
	require.Error(t, err)
	p, err := NormalizeTavernPricing(TavernPricingInput{Price: "0", Currency: "USD"})
	require.NoError(t, err)
	require.Equal(t, "0.00000000", p.Price)
}
