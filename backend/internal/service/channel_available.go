package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// AvailableGroupRef 渠道视图中关联分组的简要信息。
//
// 用户侧「可用渠道」页面据此展示：专属分组 vs 公开分组（IsExclusive）、
// 订阅 vs 标准（SubscriptionType）、默认倍率（RateMultiplier）与高峰倍率规则。
// 用户专属倍率不在这里暴露，前端自己通过 /groups/rates 拉取，和 API 密钥页面保持一致。
type AvailableGroupRef struct {
	ID                 int64
	Name               string
	Platform           string
	SubscriptionType   string
	RateMultiplier     float64
	PeakRateEnabled    bool
	PeakStart          string
	PeakEnd            string
	PeakRateMultiplier float64
	IsExclusive        bool
	ModelsListConfig   GroupModelsListConfig
}

// AvailableChannel 可用渠道视图：用于「可用渠道」页面展示渠道基础信息 +
// 关联的分组 + 推导出的支持模型列表（无通配符）。
type AvailableChannel struct {
	ID                 int64
	Name               string
	Description        string
	Status             string
	BillingModelSource string
	RestrictModels     bool
	Groups             []AvailableGroupRef
	SupportedModels    []SupportedModel
}

// ListAvailable 返回用户侧「可用模型」视图：每个活跃分组独立生成一条展示记录。
//
// 支持模型来自分组 ModelsListConfig 与分组下可调度账号的 model_mapping 并集；不再优先
// 依赖 channels/channel_model_pricing，避免生产只配置了某个渠道时把其他分组的模型隐藏。
// 对于未配置渠道定价的模型，进一步用 PricingService 的全局 LiteLLM 数据合成展示用定价。
//
// 前置条件：s.groupRepo 必须非 nil（由 wire DI 保证）。直接 nil-deref 用于 fail-fast，
// 避免静默掩盖注入缺失。
func (s *ChannelService) ListAvailable(ctx context.Context) ([]AvailableChannel, error) {
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}
	groupByID := make(map[int64]AvailableGroupRef, len(groups))
	for i := range groups {
		g := groups[i]
		groupByID[g.ID] = AvailableGroupRef{
			ID:                 g.ID,
			Name:               g.Name,
			Platform:           g.Platform,
			SubscriptionType:   g.SubscriptionType,
			RateMultiplier:     g.RateMultiplier,
			PeakRateEnabled:    g.PeakRateEnabled,
			PeakStart:          g.PeakStart,
			PeakEnd:            g.PeakEnd,
			PeakRateMultiplier: g.PeakRateMultiplier,
			IsExclusive:        g.IsExclusive,
			ModelsListConfig:   g.ModelsListConfig,
		}
	}

	out, err := s.availableChannelsFromGroupAccounts(ctx, groups, groupByID)
	if err != nil {
		return nil, err
	}
	s.fillGlobalPricingFallback(outSupportedModels(out))
	return out, nil
}

func (s *ChannelService) availableChannelsFromGroupAccounts(ctx context.Context, groups []Group, groupByID map[int64]AvailableGroupRef) ([]AvailableChannel, error) {
	if s == nil || len(groups) == 0 {
		return []AvailableChannel{}, nil
	}

	out := make([]AvailableChannel, 0, len(groups))
	for _, group := range groups {
		ref, ok := groupByID[group.ID]
		if !ok {
			continue
		}
		supported, err := s.supportedModelsFromGroupAccounts(ctx, 0, []AvailableGroupRef{ref})
		if err != nil {
			return nil, err
		}
		if len(supported) == 0 {
			continue
		}
		out = append(out, AvailableChannel{
			ID:                 -group.ID,
			Name:               group.Name,
			Description:        "Automatically generated from schedulable account model mappings.",
			Status:             StatusActive,
			BillingModelSource: BillingModelSourceChannelMapped,
			Groups:             []AvailableGroupRef{ref},
			SupportedModels:    supported,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (s *ChannelService) supportedModelsFromGroupAccounts(ctx context.Context, channelID int64, groups []AvailableGroupRef) ([]SupportedModel, error) {
	if s == nil || len(groups) == 0 {
		return nil, nil
	}

	type dedupKey struct {
		platform string
		name     string
	}
	seen := make(map[dedupKey]struct{})
	result := make([]SupportedModel, 0)
	addModel := func(platform, model string) {
		model = strings.TrimSpace(model)
		if model == "" {
			return
		}
		key := dedupKey{platform: platform, name: strings.ToLower(model)}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		result = append(result, SupportedModel{Name: model, Platform: platform})
	}

	for _, group := range groups {
		if group.ModelsListConfig.Enabled {
			for _, model := range group.ModelsListConfig.Models {
				addModel(group.Platform, model)
			}
		}
		if s.accountRepo == nil {
			continue
		}
		accounts, err := s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, group.ID, group.Platform)
		if err != nil {
			return nil, fmt.Errorf("list schedulable accounts for channel %d group %d: %w", channelID, group.ID, err)
		}
		for i := range accounts {
			for model := range accounts[i].GetModelMapping() {
				addModel(group.Platform, model)
			}
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Platform != result[j].Platform {
			return result[i].Platform < result[j].Platform
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func outSupportedModels(channels []AvailableChannel) []SupportedModel {
	var out []SupportedModel
	for i := range channels {
		out = append(out, channels[i].SupportedModels...)
	}
	return out
}

// fillGlobalPricingFallback 对未命中渠道定价的支持模型，从全局 LiteLLM 数据合成一份
// 展示用定价。仅用于「可用渠道」展示，不影响真实计费链路。
//
// 触发条件：
//  1. Pricing == nil（渠道完全没声明该模型的定价条目）
//  2. Pricing 非 nil 但所有价格字段为空（admin UI 建了条目但没填价格）
//
// 当 s.pricingService 为 nil（测试场景），跳过回落。
func (s *ChannelService) fillGlobalPricingFallback(models []SupportedModel) {
	if s.pricingService == nil {
		return
	}
	for i := range models {
		if !pricingNeedsFallback(models[i].Pricing) {
			continue
		}
		lp := s.pricingService.GetModelPricing(models[i].Name)
		if lp == nil {
			continue
		}
		models[i].Pricing = synthesizePricingFromLiteLLM(lp, models[i].Pricing)
	}
}

// pricingNeedsFallback 判定一个 ChannelModelPricing 是否需要走全局回落。
// 价格全部缺失（无 flat 字段且无任何带价 interval）即视为未配置。
func pricingNeedsFallback(p *ChannelModelPricing) bool {
	if p == nil {
		return true
	}
	if p.InputPrice != nil || p.OutputPrice != nil ||
		p.CacheWritePrice != nil || p.CacheReadPrice != nil ||
		p.ImageOutputPrice != nil || p.PerRequestPrice != nil {
		return false
	}
	for _, iv := range p.Intervals {
		if iv.InputPrice != nil || iv.OutputPrice != nil ||
			iv.CacheWritePrice != nil || iv.CacheReadPrice != nil ||
			iv.PerRequestPrice != nil {
			return false
		}
	}
	return true
}

// synthesizePricingFromLiteLLM 把 LiteLLM 的定价数据转成 ChannelModelPricing 形态，
// 仅用于展示。
//
// 计费模式优先级：
//  1. 渠道已选 BillingMode（admin 在 UI 里选了 image / per_request 但没填价的场景，
//     按选定模式合成对应字段）
//  2. LiteLLM mode="image_generation" → image
//  3. 默认 token
//
// LiteLLM 中字段 0 视为未配置，不带入展示。
func synthesizePricingFromLiteLLM(lp *LiteLLMModelPricing, existing *ChannelModelPricing) *ChannelModelPricing {
	if lp == nil {
		return existing
	}

	mode := BillingModeToken
	switch {
	case existing != nil && existing.BillingMode != "":
		mode = existing.BillingMode
	case lp.Mode == "image_generation":
		mode = BillingModeImage
	}

	if mode == BillingModeImage || mode == BillingModePerRequest {
		return &ChannelModelPricing{
			BillingMode:      mode,
			PerRequestPrice:  nonZeroPtr(lp.OutputCostPerImage),
			ImageOutputPrice: nonZeroPtr(lp.OutputCostPerImageToken),
			InputPrice:       nonZeroPtr(lp.InputCostPerToken),
			OutputPrice:      nonZeroPtr(lp.OutputCostPerToken),
		}
	}
	return &ChannelModelPricing{
		BillingMode:      mode,
		InputPrice:       nonZeroPtr(lp.InputCostPerToken),
		OutputPrice:      nonZeroPtr(lp.OutputCostPerToken),
		CacheWritePrice:  nonZeroPtr(lp.CacheCreationInputTokenCost),
		CacheReadPrice:   nonZeroPtr(lp.CacheReadInputTokenCost),
		ImageOutputPrice: nonZeroPtr(lp.OutputCostPerImageToken),
	}
}

func nonZeroPtr(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}
