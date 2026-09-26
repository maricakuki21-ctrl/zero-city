package handler

import (
	"encoding/json"
	"testing"
)

func TestCheckoutResponseAlwaysIncludesPaymentEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		body, err := json.Marshal(checkoutInfoResponse{PaymentEnabled: enabled})
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatal(err)
		}
		actual, present := decoded["payment_enabled"]
		if !present || actual != enabled {
			t.Fatalf("payment_enabled must explicitly represent %v, got %s", enabled, body)
		}
	}
}
