package service

import (
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestCryptoConfigForOrderPrefersOrderSnapshot(t *testing.T) {
	current := CryptoPaymentConfig{
		Enabled:              true,
		WalletAddress:        "current-wallet",
		Network:              "TRC20",
		Token:                "USDT",
		Chain:                "tron",
		ContractAddress:      "current-contract",
		MinAmount:            2,
		USDTBalanceRate:      8,
		ExplorerBaseURL:      "https://current-explorer/",
		TronGridBaseURL:      "https://current-grid.example",
		TronGridAPIKey:       "current-secret",
		SweepLookbackMinutes: 30,
	}
	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeCrypto,
		PayAmount:   12.345678,
		ProviderSnapshot: map[string]any{
			"crypto_wallet_address":         "snapshot-wallet",
			"crypto_network":                "TRC20",
			"crypto_token":                  "USDT",
			"crypto_chain":                  "tron",
			"crypto_contract":               "snapshot-contract",
			"crypto_pay_amount_text":        "12.345678",
			"crypto_usdt_balance_rate":      6.8,
			"crypto_min_amount":             0.1,
			"crypto_explorer_base_url":      "https://snapshot-explorer/",
			"crypto_sweep_lookback_minutes": 240,
		},
	}

	got := cryptoConfigForOrder(current, order)
	require.Equal(t, "snapshot-wallet", got.WalletAddress)
	require.Equal(t, "snapshot-contract", got.ContractAddress)
	require.Equal(t, 6.8, got.USDTBalanceRate)
	require.Equal(t, 0.1, got.MinAmount)
	require.Equal(t, 240, got.SweepLookbackMinutes)
	require.Equal(t, "https://current-grid.example", got.TronGridBaseURL)
	require.Equal(t, "current-secret", got.TronGridAPIKey)
	require.Equal(t, "12.345678", cryptoExpectedPayAmountText(order, got))
}

func TestCryptoConfigForLegacyOrderWithoutSnapshotFallsBackToCurrentConfig(t *testing.T) {
	current := CryptoPaymentConfig{
		Enabled:              true,
		WalletAddress:        "current-wallet",
		Network:              "TRC20",
		Token:                "USDT",
		Chain:                "tron",
		ContractAddress:      "current-contract",
		USDTBalanceRate:      7.2,
		TronGridBaseURL:      "https://grid.example",
		SweepLookbackMinutes: 90,
	}
	order := &dbent.PaymentOrder{PaymentType: payment.TypeCrypto, PayAmount: 1.000217}

	require.Equal(t, current, cryptoConfigForOrder(current, order))
	require.Equal(t, "1.000217", cryptoExpectedPayAmountText(order, current))
}

func TestValidateCryptoDepositForOrderRequiresSnapshotWalletContractAndExactAmount(t *testing.T) {
	now := time.Now().UTC()
	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeCrypto,
		PayAmount:   30.000217,
		CreatedAt:   now.Add(-20 * time.Minute),
		ExpiresAt:   now.Add(-5 * time.Minute),
		ProviderSnapshot: map[string]any{
			"crypto_pay_amount_text": "30.000217",
		},
	}
	cfg := CryptoPaymentConfig{
		WalletAddress:        "snapshot-wallet",
		ContractAddress:      "snapshot-contract",
		Token:                "USDT",
		SweepLookbackMinutes: 180,
	}
	deposit := &cryptoDeposit{
		To:              "snapshot-wallet",
		ContractAddress: "snapshot-contract",
		Token:           "USDT",
		Amount:          30.000217,
		AmountText:      "30.000217",
		BlockTime:       now.Add(-2 * time.Minute),
	}

	require.NoError(t, validateCryptoDepositForOrder(cfg, order, deposit))

	wrongAmount := *deposit
	wrongAmount.AmountText = "30.000218"
	require.Error(t, validateCryptoDepositForOrder(cfg, order, &wrongAmount))

	wrongWallet := *deposit
	wrongWallet.To = "current-wallet"
	require.Error(t, validateCryptoDepositForOrder(cfg, order, &wrongWallet))

	tooLate := *deposit
	tooLate.BlockTime = order.ExpiresAt.Add(181 * time.Minute)
	require.Error(t, validateCryptoDepositForOrder(cfg, order, &tooLate))
}

func TestCryptoRecoveryWindowIsFiniteAndNotificationBounded(t *testing.T) {
	now := time.Now().UTC()
	order := &dbent.PaymentOrder{
		PaymentType: payment.TypeCrypto,
		Status:      OrderStatusExpired,
		ExpiresAt:   now.Add(-2 * time.Hour),
	}
	cfg := CryptoPaymentConfig{SweepLookbackMinutes: 180}
	require.True(t, cryptoOrderWithinRecoveryWindow(order, cfg, now))

	order.ExpiresAt = now.Add(-4 * time.Hour)
	require.False(t, cryptoOrderWithinRecoveryWindow(order, cfg, now))

	order.ExpiresAt = now.Add(-time.Hour)
	deadline := order.ExpiresAt.Add(2 * time.Hour)
	metadata := map[string]string{
		"source":                          "auto_sweep",
		cryptoRecoveryDeadlineMetadataKey: deadline.Format(time.RFC3339Nano),
	}
	require.True(t, cryptoNotificationCanRecoverOrder(order, metadata, now))
	order.ProviderSnapshot = map[string]any{"crypto_sweep_lookback_minutes": 30}
	require.False(t, cryptoNotificationCanRecoverOrder(order, metadata, now))
	order.ProviderSnapshot = nil

	metadata[cryptoRecoveryDeadlineMetadataKey] = order.ExpiresAt.Add(maxCryptoLateReconcileLookback + time.Minute).Format(time.RFC3339Nano)
	require.False(t, cryptoNotificationCanRecoverOrder(order, metadata, now))

	require.Equal(t, maxCryptoLateReconcileLookback, effectiveCryptoSweepLookback(CryptoPaymentConfig{SweepLookbackMinutes: 60 * 24 * 30}))
}
