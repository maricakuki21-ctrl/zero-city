package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWithdrawalDecimalValidation(t *testing.T) {
	for _, v := range []string{"0", "-1", "1e3", "1.0000000000001", "1000000000000", "NaN", " 1", "01"} {
		_, err := NormalizeWithdrawalInput(WithdrawalInput{Amount: v, OperationID: "valid_token_123456", Channel: "bank", Recipient: "person"})
		require.Error(t, err, v)
	}
	for _, v := range []string{"0.000000000001", "999999999999.999999999999", "1"} {
		in, err := NormalizeWithdrawalInput(WithdrawalInput{Amount: v, OperationID: "valid_token_123456", Channel: "bank", Recipient: "person"})
		require.NoError(t, err)
		require.NotEmpty(t, in.Amount)
	}
}
