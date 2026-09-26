//go:build unit

package service

import (
	"context"
	"testing"
	"time"
)

type stubChannelMonitorRepoForFiltering struct {
	enabled       []*ChannelMonitor
	historyRows   []*ChannelMonitorHistoryRow
	markedChecked []int64
}

func (s *stubChannelMonitorRepoForFiltering) Create(ctx context.Context, m *ChannelMonitor) error {
	return nil
}
func (s *stubChannelMonitorRepoForFiltering) GetByID(ctx context.Context, id int64) (*ChannelMonitor, error) {
	for _, monitor := range s.enabled {
		if monitor != nil && monitor.ID == id {
			copy := *monitor
			copy.ExtraModels = append([]string(nil), monitor.ExtraModels...)
			return &copy, nil
		}
	}
	return nil, ErrChannelMonitorNotFound
}
func (s *stubChannelMonitorRepoForFiltering) FindByDuplicateOperationID(_ context.Context, _ string) (*ChannelMonitor, error) {
	return nil, nil
}
func (s *stubChannelMonitorRepoForFiltering) Update(ctx context.Context, m *ChannelMonitor) error {
	return nil
}
func (s *stubChannelMonitorRepoForFiltering) Delete(ctx context.Context, id int64) error { return nil }
func (s *stubChannelMonitorRepoForFiltering) List(ctx context.Context, params ChannelMonitorListParams) ([]*ChannelMonitor, int64, error) {
	return nil, 0, nil
}
func (s *stubChannelMonitorRepoForFiltering) ListEnabled(ctx context.Context) ([]*ChannelMonitor, error) {
	out := make([]*ChannelMonitor, 0, len(s.enabled))
	for _, monitor := range s.enabled {
		if monitor == nil {
			continue
		}
		copy := *monitor
		copy.ExtraModels = append([]string(nil), monitor.ExtraModels...)
		out = append(out, &copy)
	}
	return out, nil
}
func (s *stubChannelMonitorRepoForFiltering) MarkChecked(ctx context.Context, id int64, checkedAt time.Time) error {
	s.markedChecked = append(s.markedChecked, id)
	return nil
}
func (s *stubChannelMonitorRepoForFiltering) InsertHistoryBatch(ctx context.Context, rows []*ChannelMonitorHistoryRow) error {
	s.historyRows = append(s.historyRows, rows...)
	return nil
}
func (s *stubChannelMonitorRepoForFiltering) DeleteHistoryBefore(ctx context.Context, before time.Time) (int64, error) {
	return 0, nil
}
func (s *stubChannelMonitorRepoForFiltering) ListHistory(ctx context.Context, monitorID int64, model string, limit int) ([]*ChannelMonitorHistoryEntry, error) {
	return nil, nil
}
func (s *stubChannelMonitorRepoForFiltering) ListLatestPerModel(ctx context.Context, monitorID int64) ([]*ChannelMonitorLatest, error) {
	return nil, nil
}
func (s *stubChannelMonitorRepoForFiltering) ComputeAvailability(ctx context.Context, monitorID int64, windowDays int) ([]*ChannelMonitorAvailability, error) {
	return nil, nil
}
func (s *stubChannelMonitorRepoForFiltering) ListLatestForMonitorIDs(ctx context.Context, ids []int64) (map[int64][]*ChannelMonitorLatest, error) {
	return nil, nil
}
func (s *stubChannelMonitorRepoForFiltering) ComputeAvailabilityForMonitors(ctx context.Context, ids []int64, windowDays int) (map[int64][]*ChannelMonitorAvailability, error) {
	return nil, nil
}
func (s *stubChannelMonitorRepoForFiltering) ListRecentHistoryForMonitors(ctx context.Context, ids []int64, primaryModels map[int64]string, perMonitorLimit int) (map[int64][]*ChannelMonitorHistoryEntry, error) {
	return nil, nil
}
func (s *stubChannelMonitorRepoForFiltering) UpsertDailyRollupsFor(ctx context.Context, targetDate time.Time) (int64, error) {
	return 0, nil
}
func (s *stubChannelMonitorRepoForFiltering) DeleteRollupsBefore(ctx context.Context, beforeDate time.Time) (int64, error) {
	return 0, nil
}
func (s *stubChannelMonitorRepoForFiltering) LoadAggregationWatermark(ctx context.Context) (*time.Time, error) {
	return nil, nil
}
func (s *stubChannelMonitorRepoForFiltering) UpdateAggregationWatermark(ctx context.Context, date time.Time) error {
	return nil
}

type stubChannelMonitorAccountRepoForFiltering struct {
	byGroup map[int64][]Account
}

func (s *stubChannelMonitorAccountRepoForFiltering) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error) {
	if s == nil || s.byGroup == nil {
		return nil, nil
	}
	accounts := s.byGroup[groupID]
	out := make([]Account, len(accounts))
	copy(out, accounts)
	return out, nil
}

type plainChannelMonitorEncryptor struct{}

func (plainChannelMonitorEncryptor) Encrypt(plaintext string) (string, error) { return plaintext, nil }
func (plainChannelMonitorEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

func TestListEnabledMonitorsFiltersInactiveGroupsRemovedModelsAndUnschedulableAccountModels(t *testing.T) {
	repo := &stubChannelMonitorRepoForFiltering{enabled: []*ChannelMonitor{
		{ID: 1, Name: "积分池 自动服务状态", APIKey: "key", GroupName: "积分池", PrimaryModel: "gpt-5.5", ExtraModels: []string{"mimo-v2.5", "gpt-5.4"}, Enabled: true, IntervalSeconds: 60},
		{ID: 2, Name: "GPT Plus 自动服务状态", APIKey: "key", GroupName: "GPT Plus", PrimaryModel: "gpt-5.5", Enabled: true, IntervalSeconds: 60},
	}}
	groupRepo := &stubGroupRepoForAvailable{activeGroups: []Group{{
		ID:   10,
		Name: "积分池",
		ModelsListConfig: GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"mimo-v2.5", "gpt-5.5", "gpt-5.4"},
		},
	}}}
	accountRepo := &stubChannelMonitorAccountRepoForFiltering{byGroup: map[int64][]Account{
		10: {{ID: 18, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"mimo-v2.5": "mimo-v2.5"}}}},
	}}
	svc := NewChannelMonitorService(repo, groupRepo, accountRepo, plainChannelMonitorEncryptor{})

	monitors, err := svc.ListEnabledMonitors(context.Background())
	if err != nil {
		t.Fatalf("ListEnabledMonitors returned error: %v", err)
	}
	if len(monitors) != 1 {
		t.Fatalf("expected only one valid monitor, got %d", len(monitors))
	}
	if monitors[0].ID != 1 {
		t.Fatalf("expected monitor 1 to remain, got %d", monitors[0].ID)
	}
	if monitors[0].PrimaryModel != "mimo-v2.5" {
		t.Fatalf("expected primary model projected to mimo-v2.5, got %q", monitors[0].PrimaryModel)
	}
	if len(monitors[0].ExtraModels) != 0 {
		t.Fatalf("expected removed extra models to be dropped, got %#v", monitors[0].ExtraModels)
	}
}

func TestRunCheckReturnsNotFoundForDeletedGroupMonitor(t *testing.T) {
	repo := &stubChannelMonitorRepoForFiltering{enabled: []*ChannelMonitor{
		{ID: 2, Name: "GPT Plus 自动服务状态", APIKey: "key", GroupName: "GPT Plus", PrimaryModel: "gpt-5.5", Enabled: true, IntervalSeconds: 60},
	}}
	svc := NewChannelMonitorService(repo, &stubGroupRepoForAvailable{}, &stubChannelMonitorAccountRepoForFiltering{}, plainChannelMonitorEncryptor{})

	_, err := svc.RunCheck(context.Background(), 2)
	if err != ErrChannelMonitorNotFound {
		t.Fatalf("expected ErrChannelMonitorNotFound, got %v", err)
	}
	if len(repo.historyRows) != 0 {
		t.Fatalf("expected no history rows for deleted group monitor, got %d", len(repo.historyRows))
	}
}

func TestListUserViewAddsSyntheticViewsForActiveOfficialGroups(t *testing.T) {
	repo := &stubChannelMonitorRepoForFiltering{}
	groupRepo := &stubGroupRepoForAvailable{activeGroups: []Group{
		{ID: 10, Name: "GPT Plus", Platform: PlatformOpenAI, ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5.5", "gpt-image-2"}}},
		{ID: 11, Name: "绘图组", Platform: PlatformOpenAI, ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"gpt-image-2"}}},
		{ID: 12, Name: "Antigravity", Platform: PlatformAntigravity, ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"claude-sonnet-4"}}},
	}}
	accountRepo := &stubChannelMonitorAccountRepoForFiltering{byGroup: map[int64][]Account{
		10: {{ID: 100, Status: StatusActive, Schedulable: true, Platform: PlatformOpenAI}},
		11: {{ID: 101, Status: StatusActive, Schedulable: true, Platform: PlatformOpenAI}},
		12: {{ID: 102, Status: StatusActive, Schedulable: true, Platform: PlatformAntigravity}},
	}}
	svc := NewChannelMonitorService(repo, groupRepo, accountRepo, plainChannelMonitorEncryptor{})

	views, err := svc.ListUserView(context.Background())
	if err != nil {
		t.Fatalf("ListUserView returned error: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("expected one text synthetic view, got %#v", views)
	}
	view := views[0]
	if !view.Synthetic || view.ID != -10 || view.GroupName != "GPT Plus" || view.PrimaryModel != "gpt-5.5" {
		t.Fatalf("unexpected synthetic view: %#v", view)
	}
}

func TestListUserViewDoesNotDuplicateGroupWithRealMonitor(t *testing.T) {
	repo := &stubChannelMonitorRepoForFiltering{enabled: []*ChannelMonitor{{
		ID: 1, Name: "GPT Plus 服务状态", GroupName: "GPT Plus", Provider: MonitorProviderOpenAI,
		PrimaryModel: "gpt-5.5", Enabled: true,
	}}}
	groupRepo := &stubGroupRepoForAvailable{activeGroups: []Group{{
		ID: 10, Name: "GPT Plus", Platform: PlatformOpenAI,
		ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5.5"}},
	}}}
	accountRepo := &stubChannelMonitorAccountRepoForFiltering{byGroup: map[int64][]Account{
		10: {{ID: 100, Status: StatusActive, Schedulable: true, Platform: PlatformOpenAI}},
	}}
	svc := NewChannelMonitorService(repo, groupRepo, accountRepo, plainChannelMonitorEncryptor{})

	views, err := svc.ListUserView(context.Background())
	if err != nil {
		t.Fatalf("ListUserView returned error: %v", err)
	}
	if len(views) != 1 || views[0].Synthetic {
		t.Fatalf("expected only the persisted monitor, got %#v", views)
	}
}

var _ GroupRepository = (*stubGroupRepoForAvailable)(nil)
var _ ChannelMonitorRepository = (*stubChannelMonitorRepoForFiltering)(nil)
var _ SecretEncryptor = plainChannelMonitorEncryptor{}
