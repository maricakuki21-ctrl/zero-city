package handler

import (
	"bytes"
	"os"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/platform/corecontracts"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestSharedKeyChatUsesCanonicalGateway(t *testing.T) {
	source, err := os.ReadFile("openai_shared_pool_chat.go")
	if err != nil {
		t.Fatalf("non-canonical path: read shared chat handler: %v", err)
	}
	for _, forbidden := range [][]byte{
		[]byte("acquireSharedPoolSlots("),
		[]byte("ForwardSharedPoolResponses("),
		[]byte("ForwardSharedPoolRawChatCompletions("),
		[]byte("shouldFailoverSharedPoolRoute("),
	} {
		if bytes.Contains(source, forbidden) {
			t.Fatalf("non-canonical path: shared chat still owns %q", forbidden)
		}
	}
}

func TestCanonicalSharedKeyContextUsesBizDecipherLedgerPolicy(t *testing.T) {
	accountID := service.CanonicalAccountID(29)
	resolved := &service.SharedPoolCanonicalGatewayContext{
		AccessKey: &service.SharedPoolAccessKey{PoolID: 11, UserID: 19},
		Identity: &service.SharedPoolCanonicalIdentity{
			PoolID: 11, GroupID: service.CanonicalGroupID(23), AccountID: &accountID,
		},
		BillingPolicy: corecontracts.BillingPolicyNativeSub2,
	}

	context := newCanonicalSharedKeyContext(resolved)

	if context.BillingPolicy != corecontracts.BillingPolicyBizDecipherLedger {
		t.Fatalf("shared key policy = %q, want %q", context.BillingPolicy, corecontracts.BillingPolicyBizDecipherLedger)
	}
	if context.CanonicalAccount == nil || *context.CanonicalAccount != int64(accountID) {
		t.Fatalf("canonical account = %v, want %d", context.CanonicalAccount, accountID)
	}
}
