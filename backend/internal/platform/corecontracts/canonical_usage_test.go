package corecontracts

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_BillingPolicy_resolves_native_and_ledger_variants(t *testing.T) {
	tests := []struct {
		name string
		in   BillingPolicy
		want BillingPolicy
	}{
		{name: "empty preserves native compatibility", in: "", want: BillingPolicyNativeSub2},
		{name: "native remains native", in: BillingPolicyNativeSub2, want: BillingPolicyNativeSub2},
		{name: "shared market selects ledger", in: BillingPolicyBizDecipherLedger, want: BillingPolicyBizDecipherLedger},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := ResolveBillingPolicy(tt.in)

			// Then
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_CanonicalUsageFinalized_rejects_missing_commit_state(t *testing.T) {
	// Given
	input := validCanonicalUsageInput()
	input.CommitState = ""

	// When
	_, err := NewCanonicalUsageFinalized(input)

	// Then
	require.ErrorIs(t, err, ErrCanonicalUsageCommitStateRequired)
}

func Test_CanonicalUsageFinalized_preserves_exact_units_without_float_fields(t *testing.T) {
	// Given
	input := validCanonicalUsageInput()

	// When
	event, err := NewCanonicalUsageFinalized(input)

	// Then
	require.NoError(t, err)
	require.Equal(t, input.Units, event.Units())
	require.False(t, containsFloatField(reflect.TypeOf(event)))

	payload, err := json.Marshal(event)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "cost")
	require.NotContains(t, string(payload), "amount")

	var roundTripped CanonicalUsageFinalized
	require.NoError(t, json.Unmarshal(payload, &roundTripped))
	require.Equal(t, event.EventID(), roundTripped.EventID())
	require.Equal(t, event.Units(), roundTripped.Units())
}

func validCanonicalUsageInput() CanonicalUsageFinalizedInput {
	committedAt := time.Date(2026, time.September, 1, 8, 30, 0, 0, time.UTC)
	return CanonicalUsageFinalizedInput{
		Policy:         BillingPolicyBizDecipherLedger,
		RequestID:      "req-7",
		UsageReference: "sub2-usage:17:req-7",
		UserID:         11,
		AccountID:      13,
		GroupID:        19,
		Model:          "gpt-5.6",
		Protocol:       "/v1/responses:stream",
		Units: ExactUsageUnits{
			InputTokens:          101,
			OutputTokens:         37,
			CacheCreationTokens:  23,
			CacheReadTokens:      29,
			ImageInputTokens:     31,
			ImageOutputTokens:    41,
			ImageCount:           2,
			VideoCount:           1,
			VideoDurationSeconds: 9,
		},
		CommitState: CanonicalUsageCommitStateUsageCommitted,
		CommittedAt: committedAt,
		FinalizedAt: committedAt.Add(time.Second),
	}
}

func containsFloatField(typ reflect.Type) bool {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Float32, reflect.Float64:
		return true
	case reflect.Struct:
		for i := range typ.NumField() {
			if containsFloatField(typ.Field(i).Type) {
				return true
			}
		}
	case reflect.Array, reflect.Slice:
		return containsFloatField(typ.Elem())
	}
	return false
}
