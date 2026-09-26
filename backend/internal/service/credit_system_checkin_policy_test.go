//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreditSystemRules_CheckinMilestoneIsPointsOnly(t *testing.T) {
	for _, rule := range defaultCreditSystemRules() {
		if rule.Key != "daily_checkin_milestone" {
			continue
		}
		require.Equal(t, "credit", rule.AssetType)
		require.Equal(t, 100.0, rule.Amount)
		require.NotContains(t, rule.Description, "2 元")
		return
	}
	t.Fatal("checkin milestone policy is missing from the administrative overview")
}
