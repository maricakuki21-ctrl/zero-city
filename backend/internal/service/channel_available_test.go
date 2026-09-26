//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// stubGroupRepoForAvailable 是 ListAvailable 测试用的 GroupRepository stub，
// 仅实现 ListActive；其他方法对本测试无关，返回零值即可。
// listActiveErr 非 nil 时，ListActive 返回该错误用于错误传播测试。
// listActiveCalls 记录调用次数，用于断言「失败短路时不再访问 groupRepo」等行为。
type stubGroupRepoForAvailable struct {
	activeGroups    []Group
	listActiveErr   error
	listActiveCalls int
}

func (s *stubGroupRepoForAvailable) ListActive(ctx context.Context) ([]Group, error) {
	s.listActiveCalls++
	if s.listActiveErr != nil {
		return nil, s.listActiveErr
	}
	return s.activeGroups, nil
}

func (s *stubGroupRepoForAvailable) Create(ctx context.Context, group *Group) error { return nil }
func (s *stubGroupRepoForAvailable) GetByID(ctx context.Context, id int64) (*Group, error) {
	return nil, nil
}
func (s *stubGroupRepoForAvailable) GetByIDLite(ctx context.Context, id int64) (*Group, error) {
	return nil, nil
}
func (s *stubGroupRepoForAvailable) Update(ctx context.Context, group *Group) error { return nil }
func (s *stubGroupRepoForAvailable) UpdateBillingAssetType(ctx context.Context, id int64, assetType string) error {
	return nil
}
func (s *stubGroupRepoForAvailable) Delete(ctx context.Context, id int64) error { return nil }
func (s *stubGroupRepoForAvailable) DeleteCascade(ctx context.Context, id int64) ([]int64, error) {
	return nil, nil
}
func (s *stubGroupRepoForAvailable) List(ctx context.Context, params pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *stubGroupRepoForAvailable) ListWithFilters(ctx context.Context, params pagination.PaginationParams, platform, status, search string, isExclusive *bool, source ...string) ([]Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *stubGroupRepoForAvailable) ListActiveByPlatform(ctx context.Context, platform string) ([]Group, error) {
	return nil, nil
}
func (s *stubGroupRepoForAvailable) ExistsByName(ctx context.Context, name string) (bool, error) {
	return false, nil
}
func (s *stubGroupRepoForAvailable) GetAccountCount(ctx context.Context, groupID int64) (int64, int64, error) {
	return 0, 0, nil
}
func (s *stubGroupRepoForAvailable) DeleteAccountGroupsByGroupID(ctx context.Context, groupID int64) (int64, error) {
	return 0, nil
}
func (s *stubGroupRepoForAvailable) GetAccountIDsByGroupIDs(ctx context.Context, groupIDs []int64) ([]int64, error) {
	return nil, nil
}
func (s *stubGroupRepoForAvailable) BindAccountsToGroup(ctx context.Context, groupID int64, accountIDs []int64) error {
	return nil
}
func (s *stubGroupRepoForAvailable) UpdateSortOrders(ctx context.Context, updates []GroupSortOrderUpdate) error {
	return nil
}

type stubAccountRepoForAvailable struct {
	accountsByGroupPlatform map[string][]Account
}

func accountGroupPlatformKey(groupID int64, platform string) string {
	return platform + ":" + string(rune(groupID))
}

func (s *stubAccountRepoForAvailable) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]Account, error) {
	return s.accountsByGroupPlatform[accountGroupPlatformKey(groupID, platform)], nil
}

// newAvailableChannelService 构造一个 ChannelService，channelRepo.ListAll 返回给定 channels，
// groupRepo 由参数决定。传入空 stub 表示「活跃分组列表为空」。
func newAvailableChannelService(channels []Channel, groupRepo GroupRepository) *ChannelService {
	repo := &mockChannelRepository{
		listAllFn: func(ctx context.Context) ([]Channel, error) { return channels, nil },
	}
	return NewChannelService(repo, groupRepo, nil, nil)
}

func TestListAvailable_EmptyActiveGroups_ReturnsEmpty(t *testing.T) {
	// 活跃分组列表为空时，用户侧可用模型页应为空，不再回退展示渠道空壳。
	channels := []Channel{{
		ID:       1,
		Name:     "chA",
		Status:   StatusActive,
		GroupIDs: []int64{10, 20},
	}}
	svc := newAvailableChannelService(channels, &stubGroupRepoForAvailable{})
	out, err := svc.ListAvailable(context.Background())
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestListAvailable_IgnoresChannelsAndBuildsFromAllActiveGroups(t *testing.T) {
	// 即使生产只配置了某个渠道，用户侧可用模型页也应按所有活跃分组的模型集合生成。
	channels := []Channel{{
		ID:       2,
		Name:     "Xiaomi Mimo",
		Status:   StatusActive,
		GroupIDs: []int64{10},
	}}
	groupRepo := &stubGroupRepoForAvailable{
		activeGroups: []Group{
			{
				ID:       6,
				Name:     "GPT Plus",
				Platform: PlatformOpenAI,
				ModelsListConfig: GroupModelsListConfig{
					Enabled: true,
					Models:  []string{"gpt-5.5", "gpt-5.4"},
				},
			},
			{
				ID:       10,
				Name:     "积分池",
				Platform: PlatformOpenAI,
				ModelsListConfig: GroupModelsListConfig{
					Enabled: true,
					Models:  []string{"mimo-v2.5", "mimo-v2.5-pro"},
				},
			},
		},
	}
	svc := newAvailableChannelService(channels, groupRepo)
	out, err := svc.ListAvailable(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 2)
	byName := make(map[string][]string, len(out))
	for _, ch := range out {
		models := make([]string, 0, len(ch.SupportedModels))
		for _, model := range ch.SupportedModels {
			models = append(models, model.Name)
		}
		byName[ch.Name] = models
	}
	require.ElementsMatch(t, []string{"gpt-5.4", "gpt-5.5"}, byName["GPT Plus"])
	require.ElementsMatch(t, []string{"mimo-v2.5", "mimo-v2.5-pro"}, byName["积分池"])
}

func TestListAvailable_FallsBackToAccountModelMapping(t *testing.T) {
	channels := []Channel{{
		ID:       1,
		Name:     "auto-models",
		Status:   StatusActive,
		GroupIDs: []int64{4},
	}}
	groupRepo := &stubGroupRepoForAvailable{
		activeGroups: []Group{{ID: 4, Name: "GPT5.5号池", Platform: PlatformOpenAI}},
	}
	svc := newAvailableChannelService(channels, groupRepo)
	svc.SetAccountRepository(&stubAccountRepoForAvailable{accountsByGroupPlatform: map[string][]Account{
		accountGroupPlatformKey(4, PlatformOpenAI): {{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-5.5": "gpt-5.5",
					"gpt-5.4": "gpt-5.4",
				},
			},
		}},
	}})

	out, err := svc.ListAvailable(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.ElementsMatch(t, []string{"gpt-5.4", "gpt-5.5"}, []string{out[0].SupportedModels[0].Name, out[0].SupportedModels[1].Name})
}

func TestListAvailable_GroupModelsListConfigFeedsSupportedModelsWithoutAccountRepo(t *testing.T) {
	channels := []Channel{{
		ID:       1,
		Name:     "auto-models-from-group",
		Status:   StatusActive,
		GroupIDs: []int64{4},
	}}
	groupRepo := &stubGroupRepoForAvailable{
		activeGroups: []Group{{
			ID:       4,
			Name:     "GPT5.5号池",
			Platform: PlatformOpenAI,
			ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.5", "gpt-5.4", "gpt-5.5"},
			},
		}},
	}
	svc := newAvailableChannelService(channels, groupRepo)

	out, err := svc.ListAvailable(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.ElementsMatch(t, []string{"gpt-5.4", "gpt-5.5"}, []string{out[0].SupportedModels[0].Name, out[0].SupportedModels[1].Name})
}

func TestListAvailable_NoChannelsBuildsFromAccountModelMapping(t *testing.T) {
	groupRepo := &stubGroupRepoForAvailable{
		activeGroups: []Group{{ID: 4, Name: "GPT5.5号池", Platform: PlatformOpenAI}},
	}
	svc := newAvailableChannelService(nil, groupRepo)
	svc.SetAccountRepository(&stubAccountRepoForAvailable{accountsByGroupPlatform: map[string][]Account{
		accountGroupPlatformKey(4, PlatformOpenAI): {{
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"gpt-5.5": "gpt-5.5",
					"gpt-5.4": "gpt-5.4",
				},
			},
		}},
	}})

	out, err := svc.ListAvailable(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "GPT5.5号池", out[0].Name)
	require.Equal(t, StatusActive, out[0].Status)
	require.Len(t, out[0].Groups, 1)
	require.ElementsMatch(t, []string{"gpt-5.4", "gpt-5.5"}, []string{out[0].SupportedModels[0].Name, out[0].SupportedModels[1].Name})
}

func TestListAvailable_NoChannelsBuildsFromGroupModelsListConfigWithoutAccountRepo(t *testing.T) {
	groupRepo := &stubGroupRepoForAvailable{
		activeGroups: []Group{{
			ID:       4,
			Name:     "GPT5.5号池",
			Platform: PlatformOpenAI,
			ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.5", "gpt-5.4"},
			},
		}},
	}
	svc := newAvailableChannelService(nil, groupRepo)

	out, err := svc.ListAvailable(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "GPT5.5号池", out[0].Name)
	require.Equal(t, StatusActive, out[0].Status)
	require.Len(t, out[0].Groups, 1)
	require.ElementsMatch(t, []string{"gpt-5.4", "gpt-5.5"}, []string{out[0].SupportedModels[0].Name, out[0].SupportedModels[1].Name})
}

func TestListAvailable_SortedByGroupName(t *testing.T) {
	groupRepo := &stubGroupRepoForAvailable{
		activeGroups: []Group{
			{ID: 2, Name: "beta", Platform: PlatformOpenAI, ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"b"}}},
			{ID: 1, Name: "Alpha", Platform: PlatformOpenAI, ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"a"}}},
			{ID: 3, Name: "charlie", Platform: PlatformOpenAI, ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"c"}}},
		},
	}
	svc := newAvailableChannelService([]Channel{{ID: 1, Name: "ignored"}}, groupRepo)
	out, err := svc.ListAvailable(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 3)
	require.Equal(t, "Alpha", out[0].Name)
	require.Equal(t, "beta", out[1].Name)
	require.Equal(t, "charlie", out[2].Name)
}

func TestListAvailable_DoesNotReadChannelRepo(t *testing.T) {
	// 用户侧可用模型页只按活跃分组生成；渠道仓库故障不应影响分组模型展示。
	sentinel := errors.New("list-all-boom")
	repo := &mockChannelRepository{
		listAllFn: func(ctx context.Context) ([]Channel, error) { return nil, sentinel },
	}
	groupRepo := &stubGroupRepoForAvailable{
		activeGroups: []Group{{
			ID:       4,
			Name:     "GPT Plus",
			Platform: PlatformOpenAI,
			ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.5"},
			},
		}},
	}
	svc := NewChannelService(repo, groupRepo, nil, nil)
	out, err := svc.ListAvailable(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "GPT Plus", out[0].Name)
	require.Equal(t, 1, groupRepo.listActiveCalls)
}

func TestListAvailable_ListActiveErrorPropagates(t *testing.T) {
	// groupRepo.ListActive 返回错误时 ListAvailable 应直接返回包装后的错误。
	sentinel := errors.New("list-active-boom")
	svc := newAvailableChannelService(
		[]Channel{{ID: 1, Name: "chA"}},
		&stubGroupRepoForAvailable{listActiveErr: sentinel},
	)
	out, err := svc.ListAvailable(context.Background())
	require.Nil(t, out)
	require.ErrorIs(t, err, sentinel)
	require.Contains(t, err.Error(), "list active groups", "wrap 前缀缺失，可能 %w 被改为 %v")
}

func TestListAvailable_GeneratedGroupRowsUseChannelMappedSource(t *testing.T) {
	groupRepo := &stubGroupRepoForAvailable{
		activeGroups: []Group{{
			ID:       4,
			Name:     "GPT Plus",
			Platform: PlatformOpenAI,
			ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.5"},
			},
		}},
	}
	svc := newAvailableChannelService([]Channel{{ID: 1, Name: "ignored"}}, groupRepo)
	out, err := svc.ListAvailable(context.Background())
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, BillingModelSourceChannelMapped, out[0].BillingModelSource)
}

func TestPricingNeedsFallback(t *testing.T) {
	tests := []struct {
		name string
		in   *ChannelModelPricing
		want bool
	}{
		{"nil", nil, true},
		{"empty struct", &ChannelModelPricing{BillingMode: BillingModeToken}, true},
		{"all-empty intervals", &ChannelModelPricing{
			BillingMode: BillingModeImage,
			Intervals:   []PricingInterval{{TierLabel: "1K"}, {TierLabel: "2K"}},
		}, true},
		{"flat input set", &ChannelModelPricing{InputPrice: testPtrFloat64(3e-6)}, false},
		{"flat per_request set", &ChannelModelPricing{PerRequestPrice: testPtrFloat64(0.04)}, false},
		{"interval with price", &ChannelModelPricing{
			Intervals: []PricingInterval{{TierLabel: "1K", PerRequestPrice: testPtrFloat64(0.04)}},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, pricingNeedsFallback(tt.in))
		})
	}
}

func TestSynthesizePricingFromLiteLLM_TokenMode(t *testing.T) {
	lp := &LiteLLMModelPricing{
		Mode:                        "chat",
		InputCostPerToken:           3e-6,
		OutputCostPerToken:          1.5e-5,
		CacheCreationInputTokenCost: 3.75e-6,
		CacheReadInputTokenCost:     3e-7,
	}
	got := synthesizePricingFromLiteLLM(lp, nil)
	require.NotNil(t, got)
	require.Equal(t, BillingModeToken, got.BillingMode)
	require.NotNil(t, got.InputPrice)
	require.InDelta(t, 3e-6, *got.InputPrice, 1e-12)
	require.NotNil(t, got.CacheReadPrice)
}

func TestSynthesizePricingFromLiteLLM_ImageGenerationMode(t *testing.T) {
	// LiteLLM mode=image_generation 且渠道未声明模式时，按 image 合成。
	lp := &LiteLLMModelPricing{
		Mode:                    "image_generation",
		OutputCostPerImageToken: 4e-5,
	}
	got := synthesizePricingFromLiteLLM(lp, nil)
	require.NotNil(t, got)
	require.Equal(t, BillingModeImage, got.BillingMode)
	require.Nil(t, got.PerRequestPrice)
	require.NotNil(t, got.ImageOutputPrice)
}

func TestSynthesizePricingFromLiteLLM_RespectsExistingChannelMode(t *testing.T) {
	// admin UI 选了 per_request 但没填价：LiteLLM 数据按 per_request 合成,
	// 即便 LiteLLM 标的是 chat 模式也尊重渠道选择。
	lp := &LiteLLMModelPricing{
		Mode:               "chat",
		InputCostPerToken:  5e-6,
		OutputCostPerImage: 0.04,
	}
	existing := &ChannelModelPricing{BillingMode: BillingModePerRequest}
	got := synthesizePricingFromLiteLLM(lp, existing)
	require.NotNil(t, got)
	require.Equal(t, BillingModePerRequest, got.BillingMode)
	require.NotNil(t, got.PerRequestPrice)
	require.InDelta(t, 0.04, *got.PerRequestPrice, 1e-12)
}

func TestFillGlobalPricingFallback_NilPricing(t *testing.T) {
	pricingSvc := newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"claude-opus-4-5": {Mode: "chat", InputCostPerToken: 5e-6},
	})
	svc := &ChannelService{pricingService: pricingSvc}

	models := []SupportedModel{
		{Name: "claude-opus-4-5", Platform: "anthropic"},
	}
	svc.fillGlobalPricingFallback(models)
	require.NotNil(t, models[0].Pricing)
	require.NotNil(t, models[0].Pricing.InputPrice)
	require.InDelta(t, 5e-6, *models[0].Pricing.InputPrice, 1e-12)
}

func TestFillGlobalPricingFallback_EmptyPricingFillsFromLiteLLM(t *testing.T) {
	// 核心场景：admin UI 建了 pricing 条目（image 模式）但没填价，应走 LiteLLM 兜底。
	pricingSvc := newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"gpt-image-1": {
			Mode:                    "image_generation",
			OutputCostPerImageToken: 4e-5,
		},
	})
	svc := &ChannelService{pricingService: pricingSvc}

	models := []SupportedModel{
		{
			Name:     "gpt-image-1",
			Platform: "openai",
			Pricing: &ChannelModelPricing{
				BillingMode: BillingModeImage,
				Intervals:   []PricingInterval{{TierLabel: "1K"}, {TierLabel: "2K"}},
			},
		},
	}
	svc.fillGlobalPricingFallback(models)
	require.NotNil(t, models[0].Pricing)
	require.Equal(t, BillingModeImage, models[0].Pricing.BillingMode)
	require.NotNil(t, models[0].Pricing.ImageOutputPrice)
	require.InDelta(t, 4e-5, *models[0].Pricing.ImageOutputPrice, 1e-12)
}

func TestFillGlobalPricingFallback_KeepsExistingPrice(t *testing.T) {
	// 渠道已经填了价格的条目不应被回落覆盖。
	pricingSvc := newStubPricingServiceFromMap(map[string]*LiteLLMModelPricing{
		"served-model": {Mode: "chat", InputCostPerToken: 1e-6},
	})
	svc := &ChannelService{pricingService: pricingSvc}

	existing := &ChannelModelPricing{
		BillingMode: BillingModeToken,
		InputPrice:  testPtrFloat64(9e-9),
	}
	models := []SupportedModel{
		{Name: "served-model", Platform: "anthropic", Pricing: existing},
	}
	svc.fillGlobalPricingFallback(models)
	require.Same(t, existing, models[0].Pricing)
}

func newStubPricingServiceFromMap(data map[string]*LiteLLMModelPricing) *PricingService {
	return &PricingService{pricingData: data}
}
