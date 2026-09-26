package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsReservedEmail_DingTalkDomain(t *testing.T) {
	require.True(t, isReservedEmail("dingtalk-123@dingtalk-connect.invalid"))
	require.True(t, isReservedEmail("DINGTALK-456@DINGTALK-CONNECT.INVALID")) // case-insensitive
	require.False(t, isReservedEmail("real@dingtalk.com"))
}

func TestEnsureStarterCreditLedgerUsesCreditBalance(t *testing.T) {
	grantor := &starterCreditGrantorStub{}
	svc := &AuthService{starterCreditGrantor: grantor}

	svc.ensureStarterCreditLedger(context.Background(), &User{
		ID:            123,
		Balance:       0,
		CreditBalance: 30,
	}, "")

	require.True(t, grantor.called)
	require.Equal(t, int64(123), grantor.input.UserID)
	require.Equal(t, 30.0, grantor.input.Amount)
	require.Equal(t, "email", grantor.input.SignupSource)
}

type starterCreditGrantorStub struct {
	called bool
	input  BizStarterCreditInput
}

func (s *starterCreditGrantorStub) GrantStarterCredit(ctx context.Context, input BizStarterCreditInput) (*BizCreditLedgerEntry, bool, error) {
	s.called = true
	s.input = input
	return nil, true, nil
}
