package migrate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeneratedPaymentOrderPayAmountPreservesCryptoFingerprintPrecision(t *testing.T) {
	for _, column := range PaymentOrdersColumns {
		if column.Name != "pay_amount" {
			continue
		}
		require.Equal(t, "decimal(20,6)", column.SchemaType["postgres"])
		return
	}
	t.Fatal("generated payment order pay_amount column not found")
}
