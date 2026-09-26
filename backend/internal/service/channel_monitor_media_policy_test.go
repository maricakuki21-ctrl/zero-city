package service

import (
	"errors"
	"testing"
)

func TestChannelMonitorPaidMediaClassification(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{model: "gpt-5.6-sol", want: false},
		{model: "claude-sonnet-4-5", want: false},
		{model: "gpt-image-1", want: true},
		{model: "flux-1.1-pro", want: true},
		{model: "grok-imagine-video", want: true},
		{model: "veo-3.1", want: true},
		{model: "doubao-seedream-4", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			if got := isChannelMonitorPaidMediaModel(tt.model); got != tt.want {
				t.Fatalf("isChannelMonitorPaidMediaModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestProjectMonitorToActiveProbeModelsDropsPaidMedia(t *testing.T) {
	monitor := &ChannelMonitor{
		PrimaryModel: "gpt-image-1",
		ExtraModels:  []string{"gpt-5.6-sol", "veo-3.1", "gpt-5.6-terra"},
	}
	projected := projectMonitorToActiveProbeModels(monitor)
	if projected == nil {
		t.Fatal("expected text models to remain active")
	}
	if projected.PrimaryModel != "gpt-5.6-sol" {
		t.Fatalf("primary model = %q, want gpt-5.6-sol", projected.PrimaryModel)
	}
	if len(projected.ExtraModels) != 1 || projected.ExtraModels[0] != "gpt-5.6-terra" {
		t.Fatalf("unexpected extra models: %#v", projected.ExtraModels)
	}
}

func TestProjectMonitorToActiveProbeModelsRejectsMediaOnly(t *testing.T) {
	monitor := &ChannelMonitor{PrimaryModel: "gpt-image-1", ExtraModels: []string{"sora-2"}}
	if projected := projectMonitorToActiveProbeModels(monitor); projected != nil {
		t.Fatalf("expected media-only monitor to have no active probe models, got %#v", projected)
	}
}

func TestValidateCreateParamsRejectsPaidMediaProbe(t *testing.T) {
	err := validateCreateParams(ChannelMonitorCreateParams{
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "https://1.1.1.1",
		APIKey:          "test-key",
		PrimaryModel:    "gpt-image-1",
		IntervalSeconds: 60,
	})
	if !errors.Is(err, ErrChannelMonitorPaidMediaProbeDisabled) {
		t.Fatalf("error = %v, want ErrChannelMonitorPaidMediaProbeDisabled", err)
	}
}

func TestApplyMonitorUpdateCannotEnablePersistedPaidMediaProbe(t *testing.T) {
	existing := &ChannelMonitor{
		Provider:        MonitorProviderOpenAI,
		APIMode:         MonitorAPIModeChatCompletions,
		Endpoint:        "https://api.openai.com",
		PrimaryModel:    "gpt-image-1",
		IntervalSeconds: 60,
	}
	enabled := true
	err := applyMonitorUpdate(existing, ChannelMonitorUpdateParams{Enabled: &enabled})
	if !errors.Is(err, ErrChannelMonitorPaidMediaProbeDisabled) {
		t.Fatalf("error = %v, want ErrChannelMonitorPaidMediaProbeDisabled", err)
	}
}
