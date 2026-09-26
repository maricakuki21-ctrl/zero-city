package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type tavernRepoStub struct {
	BizDecipherRepository
	script           *TavernScript
	room             *TavernRoom
	input            TavernRoomInput
	joined           bool
	actionRoomID     int64
	actionUserID     int64
	actionName       string
	session          *TavernRuntimeSession
	sessionInput     TavernRuntimeSessionInput
	touchedSessionID int64
	turns            []TavernRoomTurn
	turnInput        TavernRoomTurnInput
	pkg              *TavernGamePackage
	packages         []TavernGamePackage
}

func (r *tavernRepoStub) CreateTavernScript(ctx context.Context, userID int64, slugBase string, input TavernScriptInput) (*TavernScript, error) {
	return &TavernScript{ID: 1, UserID: userID, Title: input.Title, Status: input.Status, Visibility: input.Visibility}, nil
}

func (r *tavernRepoStub) GetTavernScript(ctx context.Context, id int64, includeUnlisted bool) (*TavernScript, error) {
	return r.script, nil
}

func (r *tavernRepoStub) ListTavernGamePackages(ctx context.Context, scriptID int64, includeUnpublished bool) ([]TavernGamePackage, error) {
	return r.packages, nil
}

func (r *tavernRepoStub) GetTavernGamePackage(ctx context.Context, packageID int64) (*TavernGamePackage, error) {
	if r.pkg != nil && r.pkg.ID == packageID {
		return r.pkg, nil
	}
	for i := range r.packages {
		if r.packages[i].ID == packageID {
			return &r.packages[i], nil
		}
	}
	return nil, nil
}

func (r *tavernRepoStub) GetLatestPublishedTavernGamePackage(ctx context.Context, scriptID int64) (*TavernGamePackage, error) {
	if r.pkg != nil {
		return r.pkg, nil
	}
	return &TavernGamePackage{
		ID:              11,
		ScriptID:        scriptID,
		Version:         "v1",
		RuntimeKind:     TavernGamePackageRuntime,
		ProtocolVersion: TavernGamePackageProtocolV1,
		Status:          TavernGamePackageStatusPublished,
		Manifest: TavernGamePackageManifest{
			RuntimeKind:     TavernGamePackageRuntime,
			ProtocolVersion: TavernGamePackageProtocolV1,
			Limits:          TavernGamePackageLimits{MaxTurns: 48, MaxScenes: 12},
		},
	}, nil
}

func (r *tavernRepoStub) CreateTavernGamePackage(ctx context.Context, scriptID, ownerUserID int64, version string, manifest TavernGamePackageManifest) (*TavernGamePackage, error) {
	return &TavernGamePackage{ID: 12, ScriptID: scriptID, OwnerUserID: ownerUserID, Version: version, Manifest: manifest, Status: TavernGamePackageStatusDraft}, nil
}

func (r *tavernRepoStub) SetTavernGamePackageStatus(ctx context.Context, scriptID, ownerUserID int64, version, status string) (*TavernGamePackage, error) {
	return &TavernGamePackage{ID: 12, ScriptID: scriptID, OwnerUserID: ownerUserID, Version: version, Status: status}, nil
}

func (r *tavernRepoStub) CreateTavernRoom(ctx context.Context, ownerID int64, input TavernRoomInput) (*TavernRoom, error) {
	r.input = input
	if r.room != nil {
		return r.room, nil
	}
	return &TavernRoom{ID: 1, OwnerID: ownerID, ScriptID: input.ScriptID, Title: input.Title, MaxPlayers: input.MaxPlayers}, nil
}

func (r *tavernRepoStub) GetTavernRoom(ctx context.Context, id int64) (*TavernRoom, error) {
	return r.room, nil
}

func (r *tavernRepoStub) IsTavernRoomParticipant(ctx context.Context, roomID, userID int64) (bool, error) {
	r.actionRoomID = roomID
	r.actionUserID = userID
	return r.joined, nil
}

func (r *tavernRepoStub) CreateTavernRuntimeSession(ctx context.Context, input TavernRuntimeSessionInput) (*TavernRuntimeSession, error) {
	r.sessionInput = input
	return &TavernRuntimeSession{
		ID:        31,
		RoomID:    input.RoomID,
		UserID:    input.UserID,
		RuntimeID: input.RuntimeID,
		Status:    TavernRuntimeSessionStatusActive,
		ExpiresAt: input.ExpiresAt,
	}, nil
}

func (r *tavernRepoStub) GetTavernRuntimeSessionByTokenHash(ctx context.Context, tokenHash string) (*TavernRuntimeSession, error) {
	if r.session != nil && tokenHash == "" {
		return nil, errors.New("empty token hash")
	}
	return r.session, nil
}

func (r *tavernRepoStub) TouchTavernRuntimeSession(ctx context.Context, sessionID int64) error {
	r.touchedSessionID = sessionID
	return nil
}

func (r *tavernRepoStub) OpenTavernRoom(ctx context.Context, roomID, ownerID int64) (*TavernRoom, error) {
	r.actionName = "open"
	r.actionRoomID = roomID
	r.actionUserID = ownerID
	return &TavernRoom{ID: roomID, OwnerID: ownerID, Status: TavernRoomStatusLobby}, nil
}

func (r *tavernRepoStub) JoinTavernRoomTx(ctx context.Context, roomID, userID int64) (*TavernRoom, error) {
	r.actionName = "join"
	r.actionRoomID = roomID
	r.actionUserID = userID
	return &TavernRoom{ID: roomID, Status: TavernRoomStatusLobby, CurrentPlayers: 1}, nil
}

func (r *tavernRepoStub) StartTavernRoom(ctx context.Context, roomID, ownerID int64) (*TavernRoom, error) {
	r.actionName = "start"
	r.actionRoomID = roomID
	r.actionUserID = ownerID
	return &TavernRoom{ID: roomID, OwnerID: ownerID, Status: TavernRoomStatusRunning}, nil
}

func (r *tavernRepoStub) CompleteTavernRoom(ctx context.Context, roomID, ownerID int64) (*TavernRoom, error) {
	r.actionName = "complete"
	r.actionRoomID = roomID
	r.actionUserID = ownerID
	return &TavernRoom{ID: roomID, OwnerID: ownerID, Status: TavernRoomStatusCompleted}, nil
}

func (r *tavernRepoStub) CancelTavernRoom(ctx context.Context, roomID, ownerID int64) (*TavernRoom, error) {
	r.actionName = "cancel"
	r.actionRoomID = roomID
	r.actionUserID = ownerID
	return &TavernRoom{ID: roomID, OwnerID: ownerID, Status: TavernRoomStatusCancelled}, nil
}

func (r *tavernRepoStub) ListTavernRoomTurns(ctx context.Context, roomID int64, afterIndex, limit int) ([]TavernRoomTurn, error) {
	var out []TavernRoomTurn
	for _, turn := range r.turns {
		if turn.TurnIndex > afterIndex {
			out = append(out, turn)
		}
	}
	return out, nil
}

func (r *tavernRepoStub) AppendTavernRoomTurnTx(ctx context.Context, roomID, userID int64, input TavernRoomTurnInput) (*TavernRoomTurn, error) {
	r.turnInput = input
	turn := TavernRoomTurn{
		ID:              41,
		RoomID:          roomID,
		AuthorUserID:    userID,
		AuthorName:      "Player",
		AuthorRole:      "owner",
		TurnIndex:       len(r.turns) + 1,
		Kind:            "player",
		ClientMessageID: input.ClientMessageID,
		Body:            input.Body,
		CreatedAt:       time.Now().UTC(),
	}
	r.turns = append(r.turns, turn)
	return &turn, nil
}

func TestCreateTavernScriptOnlyAllowsDraftOrPending(t *testing.T) {
	svc := NewBizDecipherService(&tavernRepoStub{}, nil, nil)
	_, err := svc.CreateTavernScript(context.Background(), 7, TavernScriptInput{
		Title:       "City Case",
		Summary:     "A playable mystery",
		Description: "Players investigate a broken model market.",
		Status:      TavernScriptStatusListed,
	})
	if err == nil {
		t.Fatal("expected listed script creation to be rejected")
	}
}

func TestCreateTavernScriptNormalizesPending(t *testing.T) {
	svc := NewBizDecipherService(&tavernRepoStub{}, nil, nil)
	script, err := svc.CreateTavernScript(context.Background(), 7, TavernScriptInput{
		Title:       "City Case",
		Summary:     "A playable mystery",
		Description: "Players investigate a broken model market.",
		Status:      TavernScriptStatusPending,
		Visibility:  "public",
		PlayerMin:   2,
		PlayerMax:   6,
	})
	if err != nil {
		t.Fatalf("CreateTavernScript returned error: %v", err)
	}
	if script.Status != TavernScriptStatusPending {
		t.Fatalf("status = %q, want %q", script.Status, TavernScriptStatusPending)
	}
}

func TestCreateTavernRoomRequiresListedPublicScript(t *testing.T) {
	svc := NewBizDecipherService(&tavernRepoStub{script: &TavernScript{ID: 1, Status: TavernScriptStatusPending, Visibility: "private"}}, nil, nil)
	_, err := svc.CreateTavernRoom(context.Background(), 7, TavernRoomInput{ScriptID: 1, Title: "Night Shift"})
	if err != sql.ErrNoRows {
		t.Fatalf("err = %v, want sql.ErrNoRows", err)
	}
}

func TestCreateTavernRoomClampsMaxPlayersToScriptRange(t *testing.T) {
	repo := &tavernRepoStub{script: &TavernScript{ID: 1, Status: TavernScriptStatusListed, Visibility: "public", PlayerMin: 2, PlayerMax: 5}}
	svc := NewBizDecipherService(repo, nil, nil)
	room, err := svc.CreateTavernRoom(context.Background(), 7, TavernRoomInput{ScriptID: 1, Title: "Night Shift", MaxPlayers: 9})
	if err != nil {
		t.Fatalf("CreateTavernRoom returned error: %v", err)
	}
	if repo.input.MaxPlayers != 5 || room.MaxPlayers != 5 {
		t.Fatalf("max players = input %d room %d, want 5", repo.input.MaxPlayers, room.MaxPlayers)
	}
	if repo.input.PackageID != 11 {
		t.Fatalf("package id = %d, want 11", repo.input.PackageID)
	}
}

func TestTavernRoomLifecycleActionPassesRoomAndOwner(t *testing.T) {
	repo := &tavernRepoStub{}
	svc := NewBizDecipherService(repo, nil, nil)
	room, err := svc.StartTavernRoom(context.Background(), 7, 21)
	if err != nil {
		t.Fatalf("StartTavernRoom returned error: %v", err)
	}
	if room.Status != TavernRoomStatusRunning {
		t.Fatalf("status = %q, want %q", room.Status, TavernRoomStatusRunning)
	}
	if repo.actionName != "start" || repo.actionUserID != 7 || repo.actionRoomID != 21 {
		t.Fatalf("action = %s user %d room %d, want start user 7 room 21", repo.actionName, repo.actionUserID, repo.actionRoomID)
	}
}

func TestJoinTavernRoomRejectsInvalidUser(t *testing.T) {
	svc := NewBizDecipherService(&tavernRepoStub{}, nil, nil)
	_, err := svc.JoinTavernRoom(context.Background(), 0, 21)
	if err == nil {
		t.Fatal("expected invalid user id to be rejected")
	}
}

func TestGetTavernRoomRuntimeAllowsOwner(t *testing.T) {
	repo := &tavernRepoStub{
		script: &TavernScript{ID: 3, Title: "Runtime Script", HostBrief: "Host", OpeningPrompt: "Open", NPCCards: []string{"Keeper"}},
		room:   &TavernRoom{ID: 21, ScriptID: 3, OwnerID: 7, Title: "Runtime Room", Status: TavernRoomStatusLobby, BillingMode: "credit", EntryCreditCost: 2, RoomConfig: map[string]any{"turn_budget": float64(12)}},
	}
	svc := NewBizDecipherService(repo, nil, nil)
	config, err := svc.GetTavernRoomRuntime(context.Background(), 7, 21)
	if err != nil {
		t.Fatalf("GetTavernRoomRuntime returned error: %v", err)
	}
	if config.Participant.Role != "owner" || config.Bridge.Provider != "sillytavern" || config.Bridge.AGPLIsolated {
		t.Fatalf("unexpected runtime bridge: %+v participant %+v", config.Bridge, config.Participant)
	}
	if !config.Bridge.ExternalRuntimeIsolated || config.Bridge.SandboxMode != "no_remote_code" {
		t.Fatalf("unexpected isolation contract: %+v", config.Bridge)
	}
	if config.Budget.TurnBudget != 12 {
		t.Fatalf("turn budget = %d, want 12", config.Budget.TurnBudget)
	}
}

func TestCreateTavernRoomRequiresPublishedPackage(t *testing.T) {
	repo := &tavernRepoStub{
		script: &TavernScript{ID: 1, Status: TavernScriptStatusListed, Visibility: "public", PlayerMax: 6},
		pkg:    &TavernGamePackage{ID: 11, ScriptID: 1, Version: "v1", Status: TavernGamePackageStatusDraft},
	}
	svc := NewBizDecipherService(repo, nil, nil)
	_, err := svc.CreateTavernRoom(context.Background(), 7, TavernRoomInput{ScriptID: 1, Title: "Night Shift"})
	if !errors.Is(err, ErrTavernGamePackageUnavailable) {
		t.Fatalf("err = %v, want ErrTavernGamePackageUnavailable", err)
	}
}

func TestTavernRuntimeUsesRoomPinnedPackage(t *testing.T) {
	packageID := int64(31)
	repo := &tavernRepoStub{
		script: &TavernScript{ID: 3, Title: "Runtime Script"},
		room: &TavernRoom{
			ID:         21,
			ScriptID:   3,
			PackageID:  &packageID,
			OwnerID:    7,
			Title:      "Runtime Room",
			Status:     TavernRoomStatusLobby,
			RoomConfig: map[string]any{"turn_budget": float64(12)},
		},
		pkg: &TavernGamePackage{
			ID:              packageID,
			ScriptID:        3,
			Version:         "v4",
			RuntimeKind:     TavernGamePackageRuntime,
			ProtocolVersion: TavernGamePackageProtocolV1,
			Status:          TavernGamePackageStatusPublished,
			Manifest: TavernGamePackageManifest{
				RuntimeKind:     TavernGamePackageRuntime,
				ProtocolVersion: TavernGamePackageProtocolV1,
				Limits:          TavernGamePackageLimits{MaxTurns: 77, MaxScenes: 9},
			},
		},
	}
	svc := NewBizDecipherService(repo, nil, nil)
	config, err := svc.GetTavernRoomRuntime(context.Background(), 7, 21)
	if err != nil {
		t.Fatalf("GetTavernRoomRuntime returned error: %v", err)
	}
	if config.Package == nil || config.Package.ID != packageID || config.Bridge.ConfigVersion != TavernGamePackageProtocolV1 {
		t.Fatalf("unexpected pinned package: %+v bridge %+v", config.Package, config.Bridge)
	}
	if config.Budget.TurnBudget != 77 {
		t.Fatalf("turn budget = %d, want package limit 77", config.Budget.TurnBudget)
	}
}

func TestNormalizeTavernGamePackageRejectsUnsafeManifests(t *testing.T) {
	base := `{
		"schema_version":"tavern.package.v1",
		"runtime_kind":"declarative",
		"protocol_version":"2026-09-13.package.v1",
		"entry":{"kind":"prompt_flow","ref":"main"},
		"permissions":{"ai_gateway":true,"save":true,"score":false,"purchases":false,"presence":false},
		"content":{"scenes":[]},
		"limits":{"max_turns":48,"max_scenes":12}
	}`
	tests := []struct {
		name     string
		manifest string
		want     string
	}{
		{name: "unsupported schema", manifest: strings.Replace(base, "tavern.package.v1", "tavern.package.v9", 1), want: "unsupported schema"},
		{name: "purchases", manifest: strings.Replace(base, `"purchases":false`, `"purchases":true`, 1), want: "purchase permission"},
		{name: "secret", manifest: strings.Replace(base, `"scenes":[]`, `"token":"secret"`, 1), want: "forbidden content key"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := normalizeTavernGamePackageInput(TavernGamePackageInput{Version: "v2", Manifest: json.RawMessage(test.manifest)})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestGetTavernRoomRuntimeAllowsJoinedPlayer(t *testing.T) {
	repo := &tavernRepoStub{
		script: &TavernScript{ID: 3, Title: "Runtime Script"},
		room:   &TavernRoom{ID: 21, ScriptID: 3, OwnerID: 7, Title: "Runtime Room", Status: TavernRoomStatusRunning, BillingMode: "free"},
		joined: true,
	}
	svc := NewBizDecipherService(repo, nil, nil)
	config, err := svc.GetTavernRoomRuntime(context.Background(), 8, 21)
	if err != nil {
		t.Fatalf("GetTavernRoomRuntime returned error: %v", err)
	}
	if config.Participant.Role != "player" || !config.Room.CurrentUserJoined {
		t.Fatalf("participant = %+v joined = %v, want joined player", config.Participant, config.Room.CurrentUserJoined)
	}
}

func TestGetTavernRoomRuntimeRejectsNonParticipant(t *testing.T) {
	repo := &tavernRepoStub{
		script: &TavernScript{ID: 3, Title: "Runtime Script"},
		room:   &TavernRoom{ID: 21, ScriptID: 3, OwnerID: 7, Title: "Runtime Room", Status: TavernRoomStatusLobby},
	}
	svc := NewBizDecipherService(repo, nil, nil)
	_, err := svc.GetTavernRoomRuntime(context.Background(), 8, 21)
	if !errors.Is(err, ErrTavernRoomForbidden) {
		t.Fatalf("err = %v, want ErrTavernRoomForbidden", err)
	}
}

func TestGetTavernRoomRuntimeRejectsEndedRoom(t *testing.T) {
	repo := &tavernRepoStub{
		script: &TavernScript{ID: 3, Title: "Runtime Script"},
		room:   &TavernRoom{ID: 21, ScriptID: 3, OwnerID: 7, Title: "Runtime Room", Status: TavernRoomStatusCompleted},
	}
	svc := NewBizDecipherService(repo, nil, nil)
	_, err := svc.GetTavernRoomRuntime(context.Background(), 7, 21)
	if !errors.Is(err, ErrTavernRoomState) {
		t.Fatalf("err = %v, want ErrTavernRoomState", err)
	}
}

func TestCreateTavernRuntimeSessionReturnsOneTimeToken(t *testing.T) {
	repo := &tavernRepoStub{
		script: &TavernScript{ID: 3, Title: "Runtime Script"},
		room:   &TavernRoom{ID: 21, ScriptID: 3, OwnerID: 7, Title: "Runtime Room", Status: TavernRoomStatusLobby, BillingMode: "free"},
	}
	svc := NewBizDecipherService(repo, nil, nil)
	session, err := svc.CreateTavernRuntimeSession(context.Background(), 7, 21)
	if err != nil {
		t.Fatalf("CreateTavernRuntimeSession returned error: %v", err)
	}
	if session.Token == "" || repo.sessionInput.TokenHash == "" {
		t.Fatalf("token = %q token hash = %q, want both set", session.Token, repo.sessionInput.TokenHash)
	}
	if session.Token == repo.sessionInput.TokenHash || len(repo.sessionInput.TokenHash) != 64 {
		t.Fatalf("token hash = %q should be a sha256 hash, token = %q", repo.sessionInput.TokenHash, session.Token)
	}
	if session.Config == nil || session.Config.Bridge.LaunchURL != "/tavern-stage?room=21" {
		t.Fatalf("unexpected runtime config: %+v", session.Config)
	}
	if !repo.sessionInput.ExpiresAt.After(time.Now().UTC()) {
		t.Fatalf("expires_at = %v should be in the future", repo.sessionInput.ExpiresAt)
	}
}

func TestGetTavernRuntimeSessionRejectsDifferentUser(t *testing.T) {
	repo := &tavernRepoStub{
		script:  &TavernScript{ID: 3, Title: "Runtime Script"},
		room:    &TavernRoom{ID: 21, ScriptID: 3, OwnerID: 7, Title: "Runtime Room", Status: TavernRoomStatusLobby, BillingMode: "free"},
		session: &TavernRuntimeSession{ID: 31, RoomID: 21, UserID: 7, RuntimeID: "tavern-room-21-user-7", Status: TavernRuntimeSessionStatusActive, ExpiresAt: time.Now().UTC().Add(time.Hour)},
	}
	svc := NewBizDecipherService(repo, nil, nil)
	_, err := svc.GetTavernRuntimeSession(context.Background(), 8, "runtime-token")
	if !errors.Is(err, ErrTavernRoomForbidden) {
		t.Fatalf("err = %v, want ErrTavernRoomForbidden", err)
	}
	if repo.touchedSessionID != 0 {
		t.Fatalf("touched session id = %d, want 0", repo.touchedSessionID)
	}
}

func TestGetTavernRuntimeSessionReturnsConfigAndTouches(t *testing.T) {
	repo := &tavernRepoStub{
		script:  &TavernScript{ID: 3, Title: "Runtime Script", OpeningPrompt: "Open"},
		room:    &TavernRoom{ID: 21, ScriptID: 3, OwnerID: 7, Title: "Runtime Room", Status: TavernRoomStatusRunning, BillingMode: "free"},
		session: &TavernRuntimeSession{ID: 31, RoomID: 21, UserID: 7, RuntimeID: "tavern-room-21-user-7", Status: TavernRuntimeSessionStatusActive, ExpiresAt: time.Now().UTC().Add(time.Hour)},
	}
	svc := NewBizDecipherService(repo, nil, nil)
	session, err := svc.GetTavernRuntimeSession(context.Background(), 7, "runtime-token")
	if err != nil {
		t.Fatalf("GetTavernRuntimeSession returned error: %v", err)
	}
	if session.Config == nil || session.Config.RuntimeID != "tavern-room-21-user-7" {
		t.Fatalf("config = %+v, want runtime config", session.Config)
	}
	if repo.touchedSessionID != 31 {
		t.Fatalf("touched session id = %d, want 31", repo.touchedSessionID)
	}
}

func TestListTavernRoomTurnsAllowsOwnerAndJoinedPlayer(t *testing.T) {
	repo := &tavernRepoStub{
		room:  &TavernRoom{ID: 21, OwnerID: 7, Status: TavernRoomStatusRunning},
		turns: []TavernRoomTurn{{ID: 1, RoomID: 21, TurnIndex: 1, Body: "first"}},
	}
	svc := NewBizDecipherService(repo, nil, nil)
	turns, err := svc.ListTavernRoomTurns(context.Background(), 7, 21, 0, 50)
	if err != nil {
		t.Fatalf("ListTavernRoomTurns owner returned error: %v", err)
	}
	if len(turns) != 1 || turns[0].Body != "first" {
		t.Fatalf("turns = %+v, want one persisted turn", turns)
	}
	repo.joined = true
	if _, err := svc.ListTavernRoomTurns(context.Background(), 8, 21, 0, 50); err != nil {
		t.Fatalf("ListTavernRoomTurns joined player returned error: %v", err)
	}
}

func TestListTavernRoomTurnsRejectsNonParticipant(t *testing.T) {
	repo := &tavernRepoStub{room: &TavernRoom{ID: 21, OwnerID: 7, Status: TavernRoomStatusRunning}}
	svc := NewBizDecipherService(repo, nil, nil)
	_, err := svc.ListTavernRoomTurns(context.Background(), 8, 21, 0, 50)
	if !errors.Is(err, ErrTavernRoomForbidden) {
		t.Fatalf("err = %v, want ErrTavernRoomForbidden", err)
	}
}

func TestAppendTavernRoomTurnNormalizesAndPersists(t *testing.T) {
	repo := &tavernRepoStub{room: &TavernRoom{ID: 21, OwnerID: 7, Status: TavernRoomStatusRunning}}
	svc := NewBizDecipherService(repo, nil, nil)
	turn, err := svc.AppendTavernRoomTurn(context.Background(), 7, 21, TavernRoomTurnInput{
		ClientMessageID: "  client-1  ",
		Body:            "  inspect the ledger  ",
	})
	if err != nil {
		t.Fatalf("AppendTavernRoomTurn returned error: %v", err)
	}
	if repo.turnInput.ClientMessageID != "client-1" || repo.turnInput.Body != "inspect the ledger" {
		t.Fatalf("normalized input = %+v", repo.turnInput)
	}
	if turn.Body != "inspect the ledger" || turn.ClientMessageID != "client-1" {
		t.Fatalf("turn = %+v, want normalized persisted turn", turn)
	}
}

func TestAppendTavernRoomTurnRejectsInvalidInput(t *testing.T) {
	repo := &tavernRepoStub{room: &TavernRoom{ID: 21, OwnerID: 7, Status: TavernRoomStatusRunning}}
	svc := NewBizDecipherService(repo, nil, nil)
	if _, err := svc.AppendTavernRoomTurn(context.Background(), 7, 21, TavernRoomTurnInput{Body: "missing id"}); err == nil {
		t.Fatal("expected missing client_message_id to be rejected")
	}
	if _, err := svc.AppendTavernRoomTurn(context.Background(), 7, 21, TavernRoomTurnInput{ClientMessageID: "client-2", Body: "   "}); err == nil {
		t.Fatal("expected empty body to be rejected")
	}
}
