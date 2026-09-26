package dto

import "github.com/Wei-Shaw/sub2api/internal/service"

func redactAccountManagedExtra(extra map[string]any) map[string]any {
	if extra == nil {
		return nil
	}
	out := make(map[string]any, len(extra))
	for key, value := range extra {
		switch key {
		case service.OllamaCloudUsageSessionExtraKey, service.OllamaCloudUsageAutoRefreshExtraKey, service.OllamaCloudUsageSnapshotExtraKey:
			continue
		default:
			out[key] = value
		}
	}
	return out
}
