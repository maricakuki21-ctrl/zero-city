package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

// 渠道监控聚合层：把 latest + availability 拼成 admin/user 视图所需的 summary / detail。
// 所有方法都遵守"失败仅日志，返回零值"的原则，避免 N+1 查询失败拖垮列表渲染。

// BatchMonitorStatusSummary 批量聚合多个监控的 latest + 7d 可用率（admin/user list 用，消除 N+1）。
// 失败时返回空 map，错误仅日志，不影响列表渲染。
//
// 参数：
//   - ids: 要聚合的 monitor ID 列表
//   - primaryByID: monitor ID -> primary model（用于读 7d 可用率与 latest 状态）
//   - extrasByID: monitor ID -> extra models 列表（用于读 latest 状态填充 ExtraModels）
func (s *ChannelMonitorService) BatchMonitorStatusSummary(
	ctx context.Context,
	ids []int64,
	primaryByID map[int64]string,
	extrasByID map[int64][]string,
) map[int64]MonitorStatusSummary {
	out := make(map[int64]MonitorStatusSummary, len(ids))
	if len(ids) == 0 {
		return out
	}
	if err := s.constrainSummaryModelsToActiveGroups(ctx, primaryByID, extrasByID); err != nil {
		slog.Warn("channel_monitor: constrain summary models failed", "error", err)
	}
	latestMap, err := s.repo.ListLatestForMonitorIDs(ctx, ids)
	if err != nil {
		slog.Warn("channel_monitor: batch load latest failed", "error", err)
		latestMap = map[int64][]*ChannelMonitorLatest{}
	}
	availMap, err := s.repo.ComputeAvailabilityForMonitors(ctx, ids, monitorAvailability7Days)
	if err != nil {
		slog.Warn("channel_monitor: batch compute availability failed", "error", err)
		availMap = map[int64][]*ChannelMonitorAvailability{}
	}

	for _, id := range ids {
		out[id] = buildStatusSummary(
			indexLatestByModel(latestMap[id]),
			indexAvailabilityByModel(availMap[id]),
			primaryByID[id],
			extrasByID[id],
		)
	}
	return out
}

// ListUserView 用户只读视图：列出所有 enabled 监控的概览。
// 使用批量聚合接口避免 N+1：
//
//	1 次查 monitors；
//	1 次批量 latest（含 ping_latency_ms）；
//	1 次批量 7d availability；
//	1 次批量 timeline（主模型最近 N 条）。
func (s *ChannelMonitorService) ListUserView(ctx context.Context) ([]*UserMonitorView, error) {
	monitors, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled monitors: %w", err)
	}

	monitors, err = s.filterMonitorsByActiveGroups(ctx, monitors)
	if err != nil {
		return nil, err
	}

	views := make([]*UserMonitorView, 0, len(monitors))
	if len(monitors) > 0 {
		ids, primaryByID, extrasByID := collectMonitorIndexes(monitors)
		summaries := s.BatchMonitorStatusSummary(ctx, ids, primaryByID, extrasByID)
		latestMap := s.batchLatest(ctx, ids)
		timelineMap := s.batchTimeline(ctx, ids, primaryByID)

		for _, m := range monitors {
			primaryLatest := pickLatest(latestMap[m.ID], m.PrimaryModel)
			views = append(views, buildUserViewFromSummary(m, summaries[m.ID], primaryLatest, timelineMap[m.ID]))
		}
	}

	synthetic, err := s.listSyntheticOfficialGroupViews(ctx, monitors)
	if err != nil {
		return nil, err
	}
	return append(views, synthetic...), nil
}

func (s *ChannelMonitorService) listSyntheticOfficialGroupViews(ctx context.Context, monitors []*ChannelMonitor) ([]*UserMonitorView, error) {
	if s.groupRepo == nil || s.accountRepo == nil {
		return []*UserMonitorView{}, nil
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups for synthetic monitor views: %w", err)
	}
	monitoredGroups := make(map[string]struct{}, len(monitors))
	for _, monitor := range monitors {
		if monitor == nil {
			continue
		}
		if groupName := strings.TrimSpace(monitor.GroupName); groupName != "" {
			monitoredGroups[groupName] = struct{}{}
		}
	}

	views := make([]*UserMonitorView, 0)
	for _, group := range groups {
		groupName := strings.TrimSpace(group.Name)
		if groupName == "" {
			continue
		}
		if _, exists := monitoredGroups[groupName]; exists {
			continue
		}
		provider, ok := monitorProviderForGroup(group)
		if !ok {
			continue
		}
		accounts, err := s.schedulableGroupAccounts(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		unavailable := len(accounts) == 0
		if unavailable {
			if reader, ok := s.accountRepo.(interface {
				ListGroupAccountsForObservation(context.Context, int64) ([]Account, error)
			}); ok {
				accounts, err = reader.ListGroupAccountsForObservation(ctx, group.ID)
				if err != nil {
					return nil, fmt.Errorf("read paused group models: %w", err)
				}
			}
		}
		models := monitorableGroupModels(group, accounts)
		observationMode := "pending"
		if len(models) == 0 {
			models = passiveMediaGroupModels(group, accounts)
			observationMode = "passive_media"
		}
		if len(models) == 0 {
			continue
		}
		if unavailable {
			observationMode = "unavailable"
		}
		view := &UserMonitorView{
			ID:              -group.ID,
			Name:            groupName + " 服务状态",
			Provider:        provider,
			GroupName:       groupName,
			PrimaryModel:    models[0],
			ExtraModels:     syntheticExtraModels(models[1:]),
			Timeline:        []UserMonitorTimelinePoint{},
			Synthetic:       true,
			ObservationMode: observationMode,
		}
		if reader, ok := s.repo.(interface {
			ReadGroupAccountObservation(context.Context, int64, []string) (*UserMonitorView, error)
		}); ok && observationMode == "pending" {
			observation, err := reader.ReadGroupAccountObservation(ctx, group.ID, models)
			if err != nil {
				return nil, fmt.Errorf("read native group observation: %w", err)
			}
			if observation != nil && len(observation.Timeline) > 0 {
				view.PrimaryModel = observation.PrimaryModel
				view.PrimaryStatus = observation.PrimaryStatus
				view.PrimaryLatencyMs = observation.PrimaryLatencyMs
				view.Availability7d = observation.Availability7d
				view.Timeline = observation.Timeline
				view.ObservationMode = "active"
			}
		}
		views = append(views, view)
	}
	return views, nil
}

func monitorProviderForGroup(group Group) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(group.Platform)) {
	case PlatformOpenAI, "openai_compatible":
		return MonitorProviderOpenAI, true
	case PlatformAnthropic:
		return MonitorProviderAnthropic, true
	case PlatformGemini:
		return MonitorProviderGemini, true
	case PlatformGrok:
		return MonitorProviderGrok, true
	default:
		return "", false
	}
}

func observationGroupModels(group Group, accounts []Account) []string {
	if group.ModelsListConfig.Enabled {
		return group.ModelsListConfig.Models
	}
	// A missing display override must not hide the account's actual aliases.
	seen := make(map[string]struct{})
	for i := range accounts {
		for model := range accounts[i].GetModelMapping() {
			model = strings.TrimSpace(model)
			if model != "" && !strings.Contains(model, "*") {
				seen[model] = struct{}{}
			}
		}
	}
	if model := strings.TrimSpace(group.DefaultMappedModel); model != "" && !strings.Contains(model, "*") {
		seen[model] = struct{}{}
	}
	models := make([]string, 0, len(seen))
	for model := range seen {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}

func monitorableGroupModels(group Group, accounts []Account) []string {
	if len(accounts) == 0 {
		return nil
	}
	models := make([]string, 0, len(group.ModelsListConfig.Models))
	seen := make(map[string]struct{}, len(group.ModelsListConfig.Models))
	for _, model := range observationGroupModels(group, accounts) {
		model = strings.TrimSpace(model)
		if model == "" || isChannelMonitorPaidMediaModel(model) || !anyAccountSupportsModel(accounts, model) {
			continue
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}
	return models
}

func passiveMediaGroupModels(group Group, accounts []Account) []string {
	if len(accounts) == 0 {
		return nil
	}
	models := make([]string, 0, len(group.ModelsListConfig.Models))
	seen := make(map[string]struct{}, len(group.ModelsListConfig.Models))
	for _, model := range observationGroupModels(group, accounts) {
		model = strings.TrimSpace(model)
		if model == "" || !isChannelMonitorPaidMediaModel(model) || !anyAccountSupportsModel(accounts, model) {
			continue
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		models = append(models, model)
	}
	return models
}

func isChannelMonitorPaidMediaModel(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	for _, marker := range []string{
		"image", "dall", "midjourney", "stable-diffusion", "stable_diffusion", "sdxl", "flux",
		"imagen", "ideogram", "recraft", "seedream", "qwen-image", "wanx", "kolors", "cogview", "hidream",
		"video", "sora", "veo", "kling", "runway", "luma", "hailuo", "seedance", "vidu", "pika", "grok-imagine",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func syntheticExtraModels(models []string) []ExtraModelStatus {
	out := make([]ExtraModelStatus, 0, len(models))
	for _, model := range models {
		out = append(out, ExtraModelStatus{Model: model})
	}
	return out
}

func (s *ChannelMonitorService) constrainSummaryModelsToActiveGroups(ctx context.Context, primaryByID map[int64]string, extrasByID map[int64][]string) error {
	if len(primaryByID) == 0 {
		return nil
	}
	monitors, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("list enabled monitors for summary constraints: %w", err)
	}
	groupByName, err := s.activeGroupByName(ctx)
	if err != nil {
		return err
	}
	for _, monitor := range monitors {
		if monitor == nil {
			continue
		}
		if _, tracked := primaryByID[monitor.ID]; !tracked {
			continue
		}
		groupName := strings.TrimSpace(monitor.GroupName)
		if groupName == "" {
			continue
		}
		group, ok := groupByName[groupName]
		if !ok {
			primaryByID[monitor.ID] = ""
			extrasByID[monitor.ID] = nil
			continue
		}
		projected, err := s.projectMonitorToCurrentGroupModels(ctx, monitor, group)
		if err != nil {
			return err
		}
		if projected == nil {
			primaryByID[monitor.ID] = ""
			extrasByID[monitor.ID] = nil
			continue
		}
		primaryByID[monitor.ID] = projected.PrimaryModel
		extrasByID[monitor.ID] = projected.ExtraModels
	}
	return nil
}

func (s *ChannelMonitorService) filterMonitorsByActiveGroups(ctx context.Context, monitors []*ChannelMonitor) ([]*ChannelMonitor, error) {
	groupByName, err := s.activeGroupByName(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]*ChannelMonitor, 0, len(monitors))
	for _, monitor := range monitors {
		if monitor == nil {
			continue
		}
		groupName := strings.TrimSpace(monitor.GroupName)
		if groupName == "" {
			projected := projectMonitorToActiveProbeModels(monitor)
			if projected != nil {
				filtered = append(filtered, projected)
			}
			continue
		}
		group, ok := groupByName[groupName]
		if !ok {
			continue
		}
		projected, err := s.projectMonitorToCurrentGroupModels(ctx, monitor, group)
		if err != nil {
			return nil, err
		}
		if projected == nil {
			continue
		}
		filtered = append(filtered, projected)
	}
	return filtered, nil
}

func (s *ChannelMonitorService) activeGroupByName(ctx context.Context) (map[string]Group, error) {
	if s.groupRepo == nil {
		return map[string]Group{}, nil
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups for channel monitors: %w", err)
	}
	out := make(map[string]Group, len(groups))
	for _, group := range groups {
		name := strings.TrimSpace(group.Name)
		if name == "" {
			continue
		}
		out[name] = group
	}
	return out, nil
}

func projectMonitorToGroupModels(monitor *ChannelMonitor, group Group) *ChannelMonitor {
	allowedModels := groupMonitorModelSet(group)
	if len(allowedModels) == 0 {
		return nil
	}
	models := append([]string{monitor.PrimaryModel}, monitor.ExtraModels...)
	kept := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || isChannelMonitorPaidMediaModel(model) {
			continue
		}
		if _, ok := allowedModels[model]; !ok {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		kept = append(kept, model)
	}
	if len(kept) == 0 {
		return nil
	}
	projected := *monitor
	projected.PrimaryModel = kept[0]
	projected.ExtraModels = append([]string(nil), kept[1:]...)
	return &projected
}

func groupMonitorModelSet(group Group) map[string]struct{} {
	out := make(map[string]struct{}, len(group.ModelsListConfig.Models))
	if !group.CustomModelsListEnabled() {
		return out
	}
	for _, model := range group.ModelsListConfig.Models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		out[model] = struct{}{}
	}
	return out
}

func (s *ChannelMonitorService) projectMonitorToCurrentGroupModels(ctx context.Context, monitor *ChannelMonitor, group Group) (*ChannelMonitor, error) {
	projected := projectMonitorToGroupModels(monitor, group)
	if projected == nil {
		return nil, nil
	}
	if s.accountRepo == nil {
		return projected, nil
	}
	accounts, err := s.schedulableGroupAccounts(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, nil
	}
	models := append([]string{projected.PrimaryModel}, projected.ExtraModels...)
	kept := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || isChannelMonitorPaidMediaModel(model) {
			continue
		}
		if !anyAccountSupportsModel(accounts, model) {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		kept = append(kept, model)
	}
	if len(kept) == 0 {
		return nil, nil
	}
	projected.PrimaryModel = kept[0]
	projected.ExtraModels = append([]string(nil), kept[1:]...)
	return projected, nil
}

func (s *ChannelMonitorService) schedulableGroupAccounts(ctx context.Context, groupID int64) ([]Account, error) {
	if s.accountRepo == nil {
		return nil, nil
	}
	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("list schedulable accounts for channel monitor group %d: %w", groupID, err)
	}
	return accounts, nil
}

func anyAccountSupportsModel(accounts []Account, model string) bool {
	for i := range accounts {
		if accounts[i].IsModelSupported(model) {
			return true
		}
	}
	return false
}

func (s *ChannelMonitorService) projectMonitorByCurrentGroup(ctx context.Context, monitor *ChannelMonitor) (*ChannelMonitor, error) {
	if monitor == nil {
		return nil, nil
	}
	if strings.TrimSpace(monitor.GroupName) == "" {
		return projectMonitorToActiveProbeModels(monitor), nil
	}
	groupByName, err := s.activeGroupByName(ctx)
	if err != nil {
		return nil, err
	}
	group, ok := groupByName[strings.TrimSpace(monitor.GroupName)]
	if !ok {
		return nil, nil
	}
	return s.projectMonitorToCurrentGroupModels(ctx, monitor, group)
}

func projectMonitorToActiveProbeModels(monitor *ChannelMonitor) *ChannelMonitor {
	if monitor == nil {
		return nil
	}
	models := append([]string{monitor.PrimaryModel}, monitor.ExtraModels...)
	kept := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || isChannelMonitorPaidMediaModel(model) {
			continue
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		kept = append(kept, model)
	}
	if len(kept) == 0 {
		return nil
	}
	projected := *monitor
	projected.PrimaryModel = kept[0]
	projected.ExtraModels = append([]string(nil), kept[1:]...)
	return &projected
}

// collectMonitorIndexes 把 monitors 列表按 ID 展开为聚合查询所需的三个索引结构。
func collectMonitorIndexes(monitors []*ChannelMonitor) ([]int64, map[int64]string, map[int64][]string) {
	ids := make([]int64, 0, len(monitors))
	primaryByID := make(map[int64]string, len(monitors))
	extrasByID := make(map[int64][]string, len(monitors))
	for _, m := range monitors {
		ids = append(ids, m.ID)
		primaryByID[m.ID] = m.PrimaryModel
		extrasByID[m.ID] = m.ExtraModels
	}
	return ids, primaryByID, extrasByID
}

// batchLatest 批量取 latest per model，失败仅日志（与现有 BatchMonitorStatusSummary 一致，不阻断列表渲染）。
func (s *ChannelMonitorService) batchLatest(ctx context.Context, ids []int64) map[int64][]*ChannelMonitorLatest {
	latestMap, err := s.repo.ListLatestForMonitorIDs(ctx, ids)
	if err != nil {
		slog.Warn("channel_monitor: user view batch latest failed", "error", err)
		return map[int64][]*ChannelMonitorLatest{}
	}
	return latestMap
}

// batchTimeline 批量取每个 monitor 主模型最近 monitorTimelineMaxPoints 条历史。
func (s *ChannelMonitorService) batchTimeline(
	ctx context.Context,
	ids []int64,
	primaryByID map[int64]string,
) map[int64][]*ChannelMonitorHistoryEntry {
	timelineMap, err := s.repo.ListRecentHistoryForMonitors(ctx, ids, primaryByID, monitorTimelineMaxPoints)
	if err != nil {
		slog.Warn("channel_monitor: user view batch timeline failed", "error", err)
		return map[int64][]*ChannelMonitorHistoryEntry{}
	}
	return timelineMap
}

// pickLatest 从 latest 切片中挑出指定 model 对应项，未命中返回 nil。
func pickLatest(rows []*ChannelMonitorLatest, model string) *ChannelMonitorLatest {
	if model == "" {
		return nil
	}
	for _, r := range rows {
		if r.Model == model {
			return r
		}
	}
	return nil
}

// GetUserDetail 用户只读视图：单个监控详情（每个模型 7d/15d/30d 可用率与平均延迟）。
// 不暴露 api_key。
func (s *ChannelMonitorService) GetUserDetail(ctx context.Context, id int64) (*UserMonitorDetail, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !m.Enabled {
		return nil, ErrChannelMonitorNotFound
	}
	m, err = s.projectMonitorByCurrentGroup(ctx, m)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrChannelMonitorNotFound
	}

	latest, err := s.repo.ListLatestPerModel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list latest per model: %w", err)
	}
	availMap, err := s.collectAvailabilityWindows(ctx, id)
	if err != nil {
		return nil, err
	}

	models := mergeModelDetails(m, latest, availMap)
	return &UserMonitorDetail{
		ID:        m.ID,
		Name:      m.Name,
		Provider:  m.Provider,
		GroupName: m.GroupName,
		Models:    models,
	}, nil
}

// collectAvailabilityWindows 一次性查询 7/15/30 天三个窗口，按模型组织。
func (s *ChannelMonitorService) collectAvailabilityWindows(ctx context.Context, monitorID int64) (map[int]map[string]*ChannelMonitorAvailability, error) {
	out := make(map[int]map[string]*ChannelMonitorAvailability, 3)
	windows := []int{monitorAvailability7Days, monitorAvailability15Days, monitorAvailability30Days}
	for _, w := range windows {
		rows, err := s.repo.ComputeAvailability(ctx, monitorID, w)
		if err != nil {
			return nil, fmt.Errorf("compute availability %dd: %w", w, err)
		}
		out[w] = indexAvailabilityByModel(rows)
	}
	return out, nil
}

// ---------- 纯函数 helper（无 IO，可在 batch / 单 monitor / detail 路径复用）----------

// indexLatestByModel 把 latest 切片按 model 索引（小工具，避免在 hot path 重复写）。
func indexLatestByModel(rows []*ChannelMonitorLatest) map[string]*ChannelMonitorLatest {
	m := make(map[string]*ChannelMonitorLatest, len(rows))
	for _, r := range rows {
		m[r.Model] = r
	}
	return m
}

// indexAvailabilityByModel 把 availability 切片按 model 索引。
func indexAvailabilityByModel(rows []*ChannelMonitorAvailability) map[string]*ChannelMonitorAvailability {
	m := make(map[string]*ChannelMonitorAvailability, len(rows))
	for _, r := range rows {
		m[r.Model] = r
	}
	return m
}

// buildStatusSummary 由 latest + availability 字典构造 MonitorStatusSummary。
// 不做任何 IO，纯组装，便于在 batch 与单 monitor 路径复用。
func buildStatusSummary(
	latestByModel map[string]*ChannelMonitorLatest,
	availByModel map[string]*ChannelMonitorAvailability,
	primary string,
	extras []string,
) MonitorStatusSummary {
	summary := MonitorStatusSummary{ExtraModels: make([]ExtraModelStatus, 0, len(extras))}
	if primary != "" {
		if l, ok := latestByModel[primary]; ok {
			summary.PrimaryStatus = l.Status
			summary.PrimaryLatencyMs = l.LatencyMs
		}
		if a, ok := availByModel[primary]; ok {
			summary.Availability7d = a.AvailabilityPct
		}
	}
	for _, model := range extras {
		entry := ExtraModelStatus{Model: model}
		if l, ok := latestByModel[model]; ok {
			entry.Status = l.Status
			entry.LatencyMs = l.LatencyMs
		}
		summary.ExtraModels = append(summary.ExtraModels, entry)
	}
	return summary
}

// buildUserViewFromSummary 用预聚合好的 MonitorStatusSummary + 主模型 latest + timeline 装填 UserMonitorView（无 IO）。
// primaryLatest 可能为 nil（该监控尚无历史）；timelineEntries 可能为空。
func buildUserViewFromSummary(
	m *ChannelMonitor,
	summary MonitorStatusSummary,
	primaryLatest *ChannelMonitorLatest,
	timelineEntries []*ChannelMonitorHistoryEntry,
) *UserMonitorView {
	view := &UserMonitorView{
		ID:               m.ID,
		Name:             m.Name,
		Provider:         m.Provider,
		GroupName:        m.GroupName,
		PrimaryModel:     m.PrimaryModel,
		PrimaryStatus:    summary.PrimaryStatus,
		PrimaryLatencyMs: summary.PrimaryLatencyMs,
		Availability7d:   summary.Availability7d,
		ExtraModels:      summary.ExtraModels,
		Timeline:         buildTimelinePoints(timelineEntries),
	}
	if primaryLatest != nil {
		view.PrimaryPingLatencyMs = primaryLatest.PingLatencyMs
	}
	return view
}

// buildTimelinePoints 把 history entry 裁剪为 timeline 点（去除 message/ID/Model，减小响应体）。
func buildTimelinePoints(entries []*ChannelMonitorHistoryEntry) []UserMonitorTimelinePoint {
	out := make([]UserMonitorTimelinePoint, 0, len(entries))
	for _, e := range entries {
		out = append(out, UserMonitorTimelinePoint{
			Status:        e.Status,
			LatencyMs:     e.LatencyMs,
			PingLatencyMs: e.PingLatencyMs,
			CheckedAt:     e.CheckedAt,
		})
	}
	return out
}

// mergeModelDetails 合并 latest + availability 三个窗口为 ModelDetail 列表。
// 复用 indexLatestByModel，避免在多处重复写 build map 逻辑。
func mergeModelDetails(
	m *ChannelMonitor,
	latest []*ChannelMonitorLatest,
	availMap map[int]map[string]*ChannelMonitorAvailability,
) []ModelDetail {
	all := append([]string{m.PrimaryModel}, m.ExtraModels...)
	latestByModel := indexLatestByModel(latest)
	out := make([]ModelDetail, 0, len(all))
	for _, model := range all {
		d := ModelDetail{Model: model}
		if l, ok := latestByModel[model]; ok {
			d.LatestStatus = l.Status
			d.LatestLatencyMs = l.LatencyMs
		}
		if a, ok := availMap[monitorAvailability7Days][model]; ok {
			d.Availability7d = a.AvailabilityPct
			d.AvgLatency7dMs = a.AvgLatencyMs
		}
		if a, ok := availMap[monitorAvailability15Days][model]; ok {
			d.Availability15d = a.AvailabilityPct
		}
		if a, ok := availMap[monitorAvailability30Days][model]; ok {
			d.Availability30d = a.AvailabilityPct
		}
		out = append(out, d)
	}
	return out
}
