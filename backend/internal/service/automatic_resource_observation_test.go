package service

import "testing"

func TestAutomaticProbeModel(t *testing.T) {
	tests := []struct {
		name   string
		models []string
		want   string
	}{
		{"prefers light text", []string{"gpt-5-pro", "gemini-2.5-flash", "gpt-image-1"}, "gemini-2.5-flash"},
		{"no paid media", []string{"grok-imagine", "sora-2", "gpt-image-1", "text-embedding-3-small", "whisper-1"}, ""},
		{"trim", []string{" gpt-4o-mini "}, "gpt-4o-mini"},
		{"no wildcard probing", []string{"unsupported", "gpt-4o"}, "gpt-4o"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := automaticProbeModel(tt.models, func(model string) bool { return model != "unsupported" })
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAutomaticProbeIntersectsLiveCatalogAndRespectsMapping(t *testing.T) {
	declared:=[]string{"unavailable-flash","alias","gpt-image-1"}
	live:=map[string]bool{"upstream-text":true,"gpt-image-1":true}
	mapped:=func(model string) string { if model=="alias" { return "upstream-text" }; return model }
	got:=automaticProbeModel(intersectProbeModels(declared,live,mapped),func(string)bool{return true})
	if got!="alias" { t.Fatalf("got %q",got) }
	if len(intersectProbeModels(declared,map[string]bool{},mapped))!=0 { t.Fatal("empty live catalog must not fabricate availability") }
	if len(intersectProbeModels(declared,nil,mapped))!=len(declared) { t.Fatal("unavailable models endpoint should retain declared models for actual probing") }
}
