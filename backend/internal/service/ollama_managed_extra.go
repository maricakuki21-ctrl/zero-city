package service

var ollamaManagedExtraKeys = []string{
	OllamaCloudUsageSessionExtraKey, OllamaCloudUsageAutoRefreshExtraKey, OllamaCloudUsageSnapshotExtraKey,
}

func removeOllamaManagedExtra(extra map[string]any) {
	for _, key := range ollamaManagedExtraKeys {
		delete(extra, key)
	}
}
