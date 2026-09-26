package handler

import (
	"encoding/json"
	"testing"
)

func TestUpdateSharedPoolRequestLeavesOmittedFieldsUnset(t *testing.T) {
	var req updateSharedPoolRequest
	if err := json.Unmarshal([]byte(`{}`), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	input := updateSharedPoolInputFromRequest(req)
	if input.NameSet || input.ListedSet || input.AccountModeSet || input.ProxyIDSet || input.ModelsSet || input.ModelConfigsSet || input.SyncModelRates {
		t.Fatalf("omitted PATCH fields must stay unset: %#v", input)
	}
}

func TestUpdateSharedPoolRequestMapsSyncModelRatesOnlyWhenTrue(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    bool
	}{
		{name: "omitted", payload: `{}`, want: false},
		{name: "explicit false", payload: `{"sync_model_rates":false}`, want: false},
		{name: "explicit true", payload: `{"sync_model_rates":true}`, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req updateSharedPoolRequest
			if err := json.Unmarshal([]byte(tt.payload), &req); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			input := updateSharedPoolInputFromRequest(req)
			if input.SyncModelRates != tt.want {
				t.Fatalf("sync_model_rates=%v, want %v", input.SyncModelRates, tt.want)
			}
		})
	}
}

func TestUpdateSharedPoolRequestPreservesExplicitValues(t *testing.T) {
	var req updateSharedPoolRequest
	payload := `{
		"name":"",
		"listed":false,
		"account_mode_enabled":false,
		"proxy_id":null,
		"models":[],
		"model_configs":[]
	}`
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	input := updateSharedPoolInputFromRequest(req)
	if !input.NameSet || input.Name != "" {
		t.Fatalf("explicit empty name was not preserved: %#v", input)
	}
	if !input.ListedSet || input.Listed {
		t.Fatalf("explicit listed=false was not preserved: %#v", input)
	}
	if !input.AccountModeSet || input.AccountModeEnabled {
		t.Fatalf("explicit account_mode_enabled=false was not preserved: %#v", input)
	}
	if !input.ProxyIDSet || input.ProxyID != nil {
		t.Fatalf("explicit proxy_id=null was not preserved: %#v", input)
	}
	if !input.ModelsSet || input.Models == nil || len(input.Models) != 0 {
		t.Fatalf("explicit empty models was not preserved: %#v", input)
	}
	if !input.ModelConfigsSet || len(input.ModelConfigs) != 0 {
		t.Fatalf("explicit empty model_configs was not preserved: %#v", input)
	}
}

func TestUpdateSharedPoolRequestPreservesExplicitTrueValues(t *testing.T) {
	var req updateSharedPoolRequest
	if err := json.Unmarshal([]byte(`{"expected_config_version":17,"listed":true,"account_mode_enabled":true,"proxy_id":91}`), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	input := updateSharedPoolInputFromRequest(req)
	if !input.ListedSet || !input.Listed || !input.AccountModeSet || !input.AccountModeEnabled {
		t.Fatalf("explicit true values were not preserved: %#v", input)
	}
	if !input.ProxyIDSet || input.ProxyID == nil || *input.ProxyID != 91 {
		t.Fatalf("explicit proxy id was not preserved: %#v", input)
	}
	if input.ExpectedConfigVersion != 17 {
		t.Fatalf("expected config version was not preserved: %#v", input)
	}
}
