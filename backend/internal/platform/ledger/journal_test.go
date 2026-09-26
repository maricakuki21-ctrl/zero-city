package ledger

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain/billingcontract"
	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	"github.com/stretchr/testify/require"
)

func TestJournalBalancesExactDecimalAndRebuildsProjection(t *testing.T) {
	debit := mustDecimal(t, "-0.0001")
	credit := mustDecimal(t, "0.0001")

	journal, err := NewJournal(Journal{
		ID: "journal-1", EventID: "usage-1", IdempotencyKey: "usage-1", PayloadSHA256: fmt.Sprintf("%064d", 1),
		Kind: "canonical_usage", PostedAt: time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC),
		Entries: []Entry{{AccountID: "user-1", Amount: debit}, {AccountID: "clearing-1", Amount: credit}},
	})
	require.NoError(t, err)

	projection, err := Rebuild(journal.Entries)
	require.NoError(t, err)
	require.Equal(t, "-0.0001", projection["user-1"].String())
	require.Equal(t, "0.0001", projection["clearing-1"].String())
}

func TestJournalRejectsUnbalancedSplitWithoutEpsilon(t *testing.T) {
	gross := mustDecimal(t, "1")

	_, err := NewGrossSplit(GrossSplit{
		Gross: gross, OwnerNet: mustDecimal(t, "0.8"), PlatformFee: mustDecimal(t, "0.1"),
		ProcessorFee: mustDecimal(t, "0.0999"), RoundingResidue: mustDecimal(t, "0"),
	})

	require.ErrorIs(t, err, ErrUnbalancedJournal)
}

func TestGrossSplitPostsExplicitRoundingResidue(t *testing.T) {
	split, err := NewGrossSplit(GrossSplit{
		Gross: mustDecimal(t, "1"), OwnerNet: mustDecimal(t, "0.8"), PlatformFee: mustDecimal(t, "0.1"),
		ProcessorFee: mustDecimal(t, "0.0999"), RoundingResidue: mustDecimal(t, "0.0001"),
	})
	require.NoError(t, err)

	entries, err := split.PostingEntries(GrossSplitAccounts{
		UserDebit: "user-1", OwnerEarnings: "owner-1", PlatformFee: "platform-1",
		ProcessorFee: "processor-1", RoundingResidue: "rounding-1",
	})
	require.NoError(t, err)
	require.Len(t, entries, 5)
	require.Equal(t, "rounding-1", entries[4].AccountID)
	require.Equal(t, "0.0001", entries[4].Amount.String())
	projection, err := Rebuild(entries)
	require.NoError(t, err)
	require.Equal(t, "0.8", projection["owner-1"].String())
	require.Equal(t, "0.0001", projection["rounding-1"].String())
}

func TestCanonicalUsagePayloadSHA256IsStableAndPayloadSensitive(t *testing.T) {
	input := corecontracts.CanonicalUsageFinalizedInput{
		Policy: corecontracts.BillingPolicyBizDecipherLedger, RequestID: "request-1",
		UsageReference: "sub2-usage:1:request-1", UserID: 1, AccountID: 2, GroupID: 3,
		Model: "gpt-test", Protocol: "responses", Units: corecontracts.ExactUsageUnits{InputTokens: 1},
		CommitState: corecontracts.CanonicalUsageCommitStateUsageCommitted,
		CommittedAt: time.Date(2026, time.September, 2, 1, 0, 0, 0, time.UTC),
		FinalizedAt: time.Date(2026, time.September, 2, 1, 0, 1, 0, time.UTC),
	}
	first, err := corecontracts.NewCanonicalUsageFinalized(input)
	require.NoError(t, err)
	firstSHA, err := CanonicalUsagePayloadSHA256(first)
	require.NoError(t, err)
	replayedSHA, err := CanonicalUsagePayloadSHA256(first)
	require.NoError(t, err)
	require.Equal(t, firstSHA, replayedSHA)

	input.Units.InputTokens = 2
	changed, err := corecontracts.NewCanonicalUsageFinalized(input)
	require.NoError(t, err)
	changedSHA, err := CanonicalUsagePayloadSHA256(changed)
	require.NoError(t, err)
	require.NotEqual(t, firstSHA, changedSHA)
}

func TestReverseProducesBalancedImmutableCompensation(t *testing.T) {
	original, err := NewJournal(Journal{
		ID: "journal-1", EventID: "usage-1", IdempotencyKey: "usage-1", PayloadSHA256: fmt.Sprintf("%064d", 1),
		Kind: "canonical_usage", PostedAt: time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC),
		Entries: []Entry{{AccountID: "user-1", Amount: mustDecimal(t, "-2")}, {AccountID: "owner-1", Amount: mustDecimal(t, "2")}},
	})
	require.NoError(t, err)

	reversal, err := Reverse(original, ReversalRequest{
		JournalID: "journal-2", EventID: "refund-1", IdempotencyKey: "refund-1",
		PayloadSHA256: fmt.Sprintf("%064d", 2), PostedAt: original.PostedAt.Add(time.Second),
	})
	require.NoError(t, err)
	require.Equal(t, "journal-1", reversal.ReversalOf)
	require.Equal(t, "2", reversal.Entries[0].Amount.String())
	require.Equal(t, "-2", reversal.Entries[1].Amount.String())
	require.Equal(t, "-2", original.Entries[0].Amount.String())
}

func TestConcurrentHoldDecisionNeverOverspends(t *testing.T) {
	balance := mustDecimal(t, "1")
	amount := mustDecimal(t, "0.02")
	reserved := mustDecimal(t, "0")
	var accepted atomic.Int64
	var unexpected atomic.Int64
	var mu sync.Mutex
	var wg sync.WaitGroup

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			defer mu.Unlock()
			next, err := Reserve(balance, reserved, amount)
			if errors.Is(err, ErrInsufficientFunds) {
				return
			}
			if err != nil {
				unexpected.Add(1)
				return
			}
			reserved = next
			accepted.Add(1)
		}()
	}
	wg.Wait()

	require.Equal(t, int64(50), accepted.Load())
	require.Zero(t, unexpected.Load())
	require.Equal(t, "1", reserved.String())
}

func mustDecimal(t *testing.T, raw string) billingcontract.Decimal {
	t.Helper()
	value, err := billingcontract.ParseDecimal(raw)
	require.NoError(t, err)
	return value
}
