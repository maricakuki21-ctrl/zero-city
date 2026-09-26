package service

import (
	"context"
	"strings"
	"time"
)

// Automatic tests reuse the account adapter (credentials, model mappings and proxy).
// One inexpensive text model per account is sampled, never a paid media generation.
type AutomaticTestCandidate struct {
	AccountID int64
	Models    []string
}

type automaticModelCatalog struct {
	models map[string]bool
	expires time.Time
}

func intersectProbeModels(declared []string, available map[string]bool, mapped func(string) string) []string {
	if available == nil { return declared }
	out:=[]string{}
	for _,model:=range declared {
		if available[mapped(model)] { out=append(out,model) }
	}
	return out
}

type automaticTestRepository interface {
	ListAutomaticTestCandidates(context.Context) ([]AutomaticTestCandidate, error)
	EnsureAutomaticTest(context.Context, int64, string) error
}

func automaticProbeModel(models []string, supports func(string) bool) string {
	best, score := "", 100
	for _, raw := range models {
		model := strings.TrimSpace(raw)
		lower := strings.ToLower(model)
		if model == "" || isChannelMonitorPaidMediaModel(model) ||
			strings.Contains(lower, "embed") || strings.Contains(lower, "audio") ||
			strings.Contains(lower, "tts") || strings.Contains(lower, "whisper") ||
			strings.Contains(lower, "rerank") || strings.Contains(lower,"review") || !supports(model) {
			continue
		}
		rank := 10
		for _, marker := range []string{"flash-lite", "mini", "nano", "haiku", "flash"} {
			if strings.Contains(lower, marker) {
				rank = 1
				break
			}
		}
		if rank < score {
			best, score = model, rank
		}
	}
	return best
}

func (s *ScheduledTestRunnerService) reconcileAutomaticTests(ctx context.Context) error {
	repo, ok := s.planRepo.(automaticTestRepository)
	if !ok || s.accountTestSvc == nil {
		return nil
	}
	candidates, err := repo.ListAutomaticTestCandidates(ctx)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		account, err := s.accountTestSvc.accountRepo.GetByID(ctx, candidate.AccountID)
		if err != nil {
			return err
		}
		if s.modelCatalogs==nil { s.modelCatalogs=make(map[int64]automaticModelCatalog) }
		catalog,found:=s.modelCatalogs[candidate.AccountID]
		if !found || time.Now().After(catalog.expires) {
			catalog=automaticModelCatalog{expires:time.Now().Add(15*time.Minute)}
			modelCtx,cancel:=context.WithTimeout(ctx,10*time.Second)
			models,fetchErr:=s.accountTestSvc.FetchUpstreamSupportedModels(modelCtx,account)
			cancel()
			if fetchErr==nil {
				catalog.models=make(map[string]bool,len(models))
				for _,model:=range models { catalog.models[model]=true }
			}
			s.modelCatalogs[candidate.AccountID]=catalog
		}
		model := automaticProbeModel(intersectProbeModels(candidate.Models,catalog.models,account.GetMappedModel), func(model string) bool {
			mapped := account.GetMappedModel(model)
			return !isChannelMonitorPaidMediaModel(mapped) && anyAccountSupportsModel([]Account{*account}, model)
		})
		if err := repo.EnsureAutomaticTest(ctx, candidate.AccountID, model); err != nil {
			return err
		}
	}
	return nil
}
