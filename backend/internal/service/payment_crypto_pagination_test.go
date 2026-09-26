package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCryptoDepositsPaginateOnConfiguredEndpoint(t *testing.T) {
	calls := 0
	var bound string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "secret", r.Header.Get("TRON-PRO-API-KEY"))
		require.Equal(t, "true", r.URL.Query().Get("only_confirmed"))
		require.Equal(t, "200", r.URL.Query().Get("limit"))
		require.Equal(t, defaultCryptoUSDTContractAddress, r.URL.Query().Get("contract_address"))
		if calls == 1 {
			bound = r.URL.Query().Get("max_timestamp")
		}
		require.NotEmpty(t, bound)
		require.Equal(t, bound, r.URL.Query().Get("max_timestamp"))
		meta := `{}`
		if calls == 1 {
			meta = `{"fingerprint":"page two/+","links":{"next":"https://untrusted.invalid/steal"}}`
		} else {
			require.Equal(t, "page two/+", r.URL.Query().Get("fingerprint"))
		}
		fmt.Fprintf(w, `{"success":true,"data":[{"transaction_id":"tx%d","to":"wallet","value":"3500154","token_info":{"symbol":"USDT","address":"%s","decimals":6}}],"meta":%s}`, calls, defaultCryptoUSDTContractAddress, meta)
	}))
	defer server.Close()
	result, err := fetchCryptoDeposits(context.Background(), CryptoPaymentConfig{
		WalletAddress: "wallet", ContractAddress: defaultCryptoUSDTContractAddress,
		Token: "USDT", TronGridBaseURL: server.URL, TronGridAPIKey: "secret",
	}, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, 2, calls)
	require.Equal(t, "3.500154", result[1].AmountText)
}

func TestCryptoDepositsRejectIncompleteAndFailedScans(t *testing.T) {
	for name, body := range map[string]string{
		"repeated cursor":  `{"data":[],"meta":{"fingerprint":"same"}}`,
		"missing cursor":   `{"data":[],"meta":{"links":{"next":"https://example.test/next"}}}`,
		"provider failure": `{"success":false,"data":[]}`,
		"missing data":     `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			result, err := fetchCryptoDeposits(context.Background(), CryptoPaymentConfig{WalletAddress: "wallet", TronGridBaseURL: server.URL}, time.Time{})
			require.Error(t, err)
			require.Nil(t, result)
		})
	}
}

func TestCryptoDepositsCancelRateLimitWait(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	result, err := fetchCryptoDeposits(ctx, CryptoPaymentConfig{WalletAddress: "wallet", TronGridBaseURL: server.URL}, time.Time{})
	require.Error(t, err)
	require.Nil(t, result)
}

func TestCryptoContractGuardPreservesRecipientConfirmation(t *testing.T) {
	require.Equal(t, "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", defaultCryptoUSDTContractAddress)
	values := map[string]string{SettingCryptoPaymentEnabled: "true", SettingCryptoWalletAddress: legacySeededCryptoWalletAddress}
	require.False(t, parseCryptoPaymentConfig(values).Enabled)
	values[SettingCryptoWalletConfirmed] = legacySeededCryptoWalletAddress
	require.True(t, parseCryptoPaymentConfig(values).Enabled)
	values[SettingCryptoContractAddress] = "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj"
	require.False(t, parseCryptoPaymentConfig(values).Enabled)
}
