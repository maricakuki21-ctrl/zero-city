package schema

import (
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"
)

func TestPaymentOrderPayAmountPreservesCryptoFingerprintPrecision(t *testing.T) {
	for _, orderField := range (PaymentOrder{}).Fields() {
		descriptor := orderField.Descriptor()
		if descriptor.Name != "pay_amount" {
			continue
		}
		require.Equal(t, "decimal(20,6)", descriptor.SchemaType[dialect.Postgres])
		return
	}
	t.Fatal("payment order pay_amount field not found")
}
