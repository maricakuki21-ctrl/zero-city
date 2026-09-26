package service

import "context"

// BeginSharedPoolGatewayUsageTrace is observability-only. It must never gate or
// mutate forwarding, balance reservation, settlement, or owner payout.
func (s *OpenAIGatewayService) BeginSharedPoolGatewayUsageTrace(ctx context.Context, trace SharedPoolUsageTrace) error {
	if s == nil || s.bizDecipherService == nil {
		return nil
	}
	return s.bizDecipherService.BeginSharedPoolUsageTrace(ctx, trace)
}

// FinalizeSharedPoolGatewayUsageTrace writes the terminal request metadata in a
// separate transaction after the existing settlement path has decided its
// outcome. The accounting transaction remains the sole source of money truth.
func (s *OpenAIGatewayService) FinalizeSharedPoolGatewayUsageTrace(ctx context.Context, trace SharedPoolUsageTrace) error {
	if s == nil || s.bizDecipherService == nil {
		return nil
	}
	return s.bizDecipherService.FinalizeSharedPoolUsageTrace(ctx, trace)
}
