package config

// BackgroundRuntime controls process-wide workers that are independent of an
// individual HTTP request. The zero value is active for deployment compatibility.
type BackgroundRuntime struct {
	role string
}

func ProvideBackgroundRuntime(cfg *Config) BackgroundRuntime {
	if cfg == nil {
		return BackgroundRuntime{role: BackgroundRuntimeRoleActive}
	}
	return BackgroundRuntime{role: cfg.BackgroundRuntimeRole}
}

func (r BackgroundRuntime) Role() string {
	if r.role == BackgroundRuntimeRoleStandby {
		return BackgroundRuntimeRoleStandby
	}
	return BackgroundRuntimeRoleActive
}

func (r BackgroundRuntime) Enabled() bool {
	return r.Role() == BackgroundRuntimeRoleActive
}

// SchedulerSnapshotEnabled allows a standby HTTP instance to keep scheduler
// snapshots current without enabling billing, expiry, monitoring, or other
// autonomous workers owned by the active runtime.
func (r BackgroundRuntime) SchedulerSnapshotEnabled(cfg *Config) bool {
	return r.Enabled() || (cfg != nil && cfg.SchedulerSnapshotWorkerEnabled)
}

// Start invokes a process-wide worker starter only for the active runtime.
func (r BackgroundRuntime) Start(start func()) bool {
	if !r.Enabled() || start == nil {
		return false
	}
	start()
	return true
}
