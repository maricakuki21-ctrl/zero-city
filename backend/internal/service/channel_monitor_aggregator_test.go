package service

import (
	"context"
	"testing"
)

type observationGroups struct{ GroupRepository }

func (*observationGroups) ListActive(context.Context) ([]Group, error) {
	return []Group{{ID: 9, Name: "paused", Platform: PlatformOpenAI}}, nil
}

type observationAccounts struct{}

func (*observationAccounts) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return nil, nil
}

func (*observationAccounts) ListGroupAccountsForObservation(context.Context, int64) ([]Account, error) {
	return []Account{{Status: StatusError, Credentials: map[string]any{
		"model_mapping": map[string]any{"text-mini": "text-mini"},
	}}}, nil
}

func TestObservationGroupModelsKeepsPausedResourcesVisible(t *testing.T) {
	svc := &ChannelMonitorService{groupRepo: &observationGroups{}, accountRepo: &observationAccounts{}}
	views, err := svc.listSyntheticOfficialGroupViews(context.Background(), nil)
	if err != nil || len(views) != 1 {
		t.Fatalf("expected paused resource metadata, got %#v %v", views, err)
	}
	if views[0].ObservationMode != "unavailable" || len(views[0].Timeline) != 0 {
		t.Fatalf("paused resource must not fabricate samples: %#v", views[0])
	}
}

func TestObservationGroupModelsFallsBackToAccountAliases(t *testing.T) {
	accounts := []Account{{Credentials: map[string]any{
		"model_mapping": map[string]any{"text-mini": "upstream-text", "image-model": "image-model", "*": "*"},
	}}}
	models := observationGroupModels(Group{}, accounts)
	if len(models) != 2 || models[0] != "image-model" || models[1] != "text-mini" {
		t.Fatalf("expected stable concrete aliases, got %#v", models)
	}
	restricted := observationGroupModels(Group{ModelsListConfig: GroupModelsListConfig{
		Enabled: true, Models: []string{"text-mini"},
	}}, accounts)
	if len(restricted) != 1 || restricted[0] != "text-mini" {
		t.Fatalf("explicit group model configuration must win: %#v", restricted)
	}
}

func TestObservationGroupModelsDoesNotInventModels(t *testing.T) {
	if models := observationGroupModels(Group{}, []Account{{}}); len(models) != 0 {
		t.Fatalf("empty account must remain unknown: %#v", models)
	}
}

func TestProjectMonitorToGroupModelsDropsRemovedModels(t *testing.T) {
	monitor := &ChannelMonitor{
		ID:           1,
		Name:         "积分池 自动服务状态",
		GroupName:    "积分池",
		PrimaryModel: "gpt-5.5",
		ExtraModels:  []string{"mimo-v2.5", "gpt-5.4", "gpt-5.5"},
	}
	group := Group{
		Name: "积分池",
		ModelsListConfig: GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"mimo-v2.5", "gpt-5.4"},
		},
	}

	projected := projectMonitorToGroupModels(monitor, group)
	if projected == nil {
		t.Fatal("expected monitor to keep valid group models")
	}
	if projected.PrimaryModel != "mimo-v2.5" {
		t.Fatalf("expected primary model to move to first valid model, got %q", projected.PrimaryModel)
	}
	if len(projected.ExtraModels) != 1 || projected.ExtraModels[0] != "gpt-5.4" {
		t.Fatalf("unexpected extra models: %#v", projected.ExtraModels)
	}
}

func TestProjectMonitorToGroupModelsDropsMonitorWhenNoGroupModelsMatch(t *testing.T) {
	monitor := &ChannelMonitor{
		ID:           1,
		Name:         "旧分组 自动服务状态",
		GroupName:    "旧分组",
		PrimaryModel: "gpt-5.5",
		ExtraModels:  []string{"gpt-5.4"},
	}
	group := Group{
		Name: "旧分组",
		ModelsListConfig: GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"mimo-v2.5"},
		},
	}

	if projected := projectMonitorToGroupModels(monitor, group); projected != nil {
		t.Fatalf("expected monitor to be dropped, got %#v", projected)
	}
}

func TestProjectMonitorToGroupModelsDropsMonitorWhenCustomModelsDisabled(t *testing.T) {
	monitor := &ChannelMonitor{
		ID:           1,
		Name:         "空分组 自动服务状态",
		GroupName:    "空分组",
		PrimaryModel: "gpt-5.5",
	}
	group := Group{
		Name: "空分组",
		ModelsListConfig: GroupModelsListConfig{
			Enabled: false,
			Models:  []string{"gpt-5.5"},
		},
	}

	if projected := projectMonitorToGroupModels(monitor, group); projected != nil {
		t.Fatalf("expected monitor to be dropped, got %#v", projected)
	}
}
