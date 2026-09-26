package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	TavernScriptStatusDraft    = "draft"
	TavernScriptStatusPending  = "pending"
	TavernScriptStatusListed   = "listed"
	TavernScriptStatusRejected = "rejected"
	TavernScriptStatusArchived = "archived"
	TavernScriptStatusDelisted = "delisted"

	TavernRoomStatusDraft     = "draft"
	TavernRoomStatusLobby     = "lobby"
	TavernRoomStatusRunning   = "running"
	TavernRoomStatusPaused    = "paused"
	TavernRoomStatusCompleted = "completed"
	TavernRoomStatusCancelled = "cancelled"

	TavernRuntimeSessionStatusActive  = "active"
	TavernRuntimeSessionStatusRevoked = "revoked"
)

const tavernRuntimeSessionTTL = 2 * time.Hour

var (
	ErrTavernRoomNotJoinable           = errors.New("tavern room is not joinable")
	ErrTavernRoomFull                  = errors.New("tavern room is at capacity")
	ErrTavernRoomForbidden             = errors.New("no permission for tavern room")
	ErrTavernRoomState                 = errors.New("tavern room state does not allow this action")
	ErrTavernInsufficientCredit        = errors.New("insufficient credit for tavern room entry")
	ErrTavernBalanceBillingUnsupported = errors.New("tavern balance billing is not available yet")
	ErrTavernTurnConflict              = errors.New("tavern turn conflicts with an existing idempotency key")
)

type TavernScriptInput struct {
	Title              string   `json:"title"`
	Summary            string   `json:"summary"`
	Description        string   `json:"description"`
	Genre              string   `json:"genre"`
	Status             string   `json:"status"`
	Visibility         string   `json:"visibility"`
	PlayerMin          int      `json:"player_min"`
	PlayerMax          int      `json:"player_max"`
	EstimatedMinutes   int      `json:"estimated_minutes"`
	Difficulty         string   `json:"difficulty"`
	Tags               []string `json:"tags"`
	NPCCards           []string `json:"npc_cards"`
	HostBrief          string   `json:"host_brief"`
	OpeningPrompt      string   `json:"opening_prompt"`
	SafetyNotes        string   `json:"safety_notes"`
	PricingMode        string   `json:"pricing_mode"`
	EntryCreditCost    int      `json:"entry_credit_cost"`
	EntryBalanceCost   float64  `json:"entry_balance_cost"`
	AuthorRevenueShare float64  `json:"author_revenue_share"`
}

type TavernScriptQuery struct {
	Keyword string
	Genre   string
	Status  string
	Sort    string
	Limit   int
}

type TavernScriptReviewInput struct {
	Status       string   `json:"status"`
	ReviewNote   string   `json:"review_note"`
	QualityScore *float64 `json:"quality_score,omitempty"`
}

type TavernRoomInput struct {
	ScriptID         int64          `json:"script_id"`
	PackageID        int64          `json:"-"`
	Title            string         `json:"title"`
	Visibility       string         `json:"visibility"`
	HostMode         string         `json:"host_mode"`
	BillingMode      string         `json:"billing_mode"`
	EntryCreditCost  int            `json:"entry_credit_cost"`
	EntryBalanceCost float64        `json:"entry_balance_cost"`
	MaxPlayers       int            `json:"max_players"`
	RoomConfig       map[string]any `json:"room_config"`
}

type TavernRoomQuery struct {
	OwnerID int64
	Status  string
	Limit   int
}

type TavernScript struct {
	EntryPriceUSD      string     `json:"entry_price_usd"`
	ID                 int64      `json:"id"`
	UserID             int64      `json:"user_id"`
	Author             string     `json:"author"`
	Title              string     `json:"title"`
	Slug               string     `json:"slug"`
	Summary            string     `json:"summary"`
	Description        string     `json:"description"`
	Genre              string     `json:"genre"`
	Status             string     `json:"status"`
	Visibility         string     `json:"visibility"`
	PlayerMin          int        `json:"player_min"`
	PlayerMax          int        `json:"player_max"`
	EstimatedMinutes   int        `json:"estimated_minutes"`
	Difficulty         string     `json:"difficulty"`
	Tags               []string   `json:"tags"`
	NPCCards           []string   `json:"npc_cards"`
	HostBrief          string     `json:"host_brief"`
	OpeningPrompt      string     `json:"opening_prompt"`
	SafetyNotes        string     `json:"safety_notes"`
	PricingMode        string     `json:"pricing_mode"`
	EntryCreditCost    int        `json:"entry_credit_cost"`
	EntryBalanceCost   float64    `json:"entry_balance_cost"`
	AuthorRevenueShare float64    `json:"author_revenue_share"`
	QualityScore       float64    `json:"quality_score"`
	ReviewNote         string     `json:"review_note"`
	ReviewedBy         *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt         *time.Time `json:"reviewed_at,omitempty"`
	PublishedAt        *time.Time `json:"published_at,omitempty"`
	ArchivedAt         *time.Time `json:"archived_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type TavernRoom struct {
	TicketPriceUSD     *string        `json:"ticket_price_usd,omitempty"`
	ID                 int64          `json:"id"`
	ScriptID           int64          `json:"script_id"`
	PackageID          *int64         `json:"package_id,omitempty"`
	PackageVersion     string         `json:"package_version"`
	PackageRuntimeKind string         `json:"package_runtime_kind"`
	PackageStatus      string         `json:"package_status"`
	OwnerID            int64          `json:"owner_id"`
	Owner              string         `json:"owner"`
	ScriptTitle        string         `json:"script_title"`
	Title              string         `json:"title"`
	Status             string         `json:"status"`
	Visibility         string         `json:"visibility"`
	HostMode           string         `json:"host_mode"`
	BillingMode        string         `json:"billing_mode"`
	EntryCreditCost    int            `json:"entry_credit_cost"`
	EntryBalanceCost   float64        `json:"entry_balance_cost"`
	MaxPlayers         int            `json:"max_players"`
	CurrentPlayers     int            `json:"current_players"`
	CurrentUserJoined  bool           `json:"current_user_joined"`
	CurrentPhase       string         `json:"current_phase"`
	RoomConfig         map[string]any `json:"room_config"`
	StartedAt          *time.Time     `json:"started_at,omitempty"`
	EndedAt            *time.Time     `json:"ended_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type TavernRuntimeConfig struct {
	RuntimeID   string                   `json:"runtime_id"`
	Room        TavernRoom               `json:"room"`
	Script      TavernScript             `json:"script"`
	Package     *TavernGamePackage       `json:"package,omitempty"`
	Participant TavernRuntimeParticipant `json:"participant"`
	Bridge      TavernRuntimeBridge      `json:"bridge"`
	Gateway     TavernRuntimeGateway     `json:"gateway"`
	Budget      TavernRuntimeBudget      `json:"budget"`
	Prompts     TavernRuntimePrompts     `json:"prompts"`
}

type TavernRuntimeSession struct {
	ID        int64                `json:"id"`
	Token     string               `json:"token,omitempty"`
	RoomID    int64                `json:"room_id"`
	UserID    int64                `json:"user_id"`
	RuntimeID string               `json:"runtime_id"`
	Status    string               `json:"status"`
	ExpiresAt time.Time            `json:"expires_at"`
	CreatedAt time.Time            `json:"created_at"`
	UpdatedAt time.Time            `json:"updated_at"`
	Config    *TavernRuntimeConfig `json:"config,omitempty"`
}

type TavernRuntimeSessionInput struct {
	TokenHash string
	RoomID    int64
	UserID    int64
	RuntimeID string
	ExpiresAt time.Time
}

type TavernRuntimeParticipant struct {
	UserID int64  `json:"user_id"`
	Role   string `json:"role"`
}

type TavernRuntimeBridge struct {
	Provider                string `json:"provider"`
	Mode                    string `json:"mode"`
	ConfigVersion           string `json:"config_version"`
	ProtocolVersion         string `json:"protocol_version"`
	SandboxMode             string `json:"sandbox_mode"`
	ExternalRuntimeIsolated bool   `json:"external_runtime_isolated"`
	AGPLIsolated            bool   `json:"agpl_isolated"`
	LaunchURL               string `json:"launch_url"`
}

type TavernRuntimeGateway struct {
	Source        string `json:"source"`
	ModelStrategy string `json:"model_strategy"`
	ProxyPath     string `json:"proxy_path"`
}

type TavernRuntimeBudget struct {
	BillingMode      string  `json:"billing_mode"`
	EntryCreditCost  int     `json:"entry_credit_cost"`
	EntryBalanceCost float64 `json:"entry_balance_cost"`
	TurnBudget       int     `json:"turn_budget"`
}

type TavernRuntimePrompts struct {
	HostBrief     string   `json:"host_brief"`
	OpeningPrompt string   `json:"opening_prompt"`
	SafetyNotes   string   `json:"safety_notes"`
	NPCCards      []string `json:"npc_cards"`
}

type TavernRoomTurn struct {
	ID              int64     `json:"id"`
	RoomID          int64     `json:"room_id"`
	AuthorUserID    int64     `json:"author_user_id"`
	AuthorName      string    `json:"author_name"`
	AuthorRole      string    `json:"author_role"`
	TurnIndex       int       `json:"turn_index"`
	Kind            string    `json:"kind"`
	ClientMessageID string    `json:"client_message_id"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"created_at"`
}

type TavernRoomTurnInput struct {
	ClientMessageID string `json:"client_message_id"`
	Body            string `json:"body"`
}

// TavernRoomTurnRepository is optional so the existing service interface and
// lightweight test repositories remain compatible. Production repositories
// implement it; callers fail closed when they do not.
type TavernRoomTurnRepository interface {
	ListTavernRoomTurns(ctx context.Context, roomID int64, afterIndex, limit int) ([]TavernRoomTurn, error)
	AppendTavernRoomTurnTx(ctx context.Context, roomID, userID int64, input TavernRoomTurnInput) (*TavernRoomTurn, error)
}

func (s *BizDecipherService) ListTavernScripts(ctx context.Context, query TavernScriptQuery) ([]TavernScript, error) {
	query.Status = TavernScriptStatusListed
	query.Genre = normalizeTavernGenreFilter(query.Genre)
	query.Sort = normalizeTavernScriptSort(query.Sort)
	return s.repo.ListTavernScripts(ctx, query)
}

func (s *BizDecipherService) GetTavernScript(ctx context.Context, id int64) (*TavernScript, error) {
	if id <= 0 {
		return nil, errors.New("invalid tavern script id")
	}
	script, err := s.repo.GetTavernScript(ctx, id, false)
	if err != nil {
		return nil, err
	}
	if script == nil || script.Status != TavernScriptStatusListed || script.Visibility != "public" {
		return nil, sql.ErrNoRows
	}
	return script, nil
}

func (s *BizDecipherService) ListMyTavernScripts(ctx context.Context, userID int64, status string, limit int) ([]TavernScript, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	return s.repo.ListMyTavernScripts(ctx, userID, normalizeTavernScriptStatusFilter(status), limit)
}

func (s *BizDecipherService) CreateTavernScript(ctx context.Context, userID int64, input TavernScriptInput) (*TavernScript, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	input = normalizeTavernScriptInput(input)
	if input.Title == "" {
		return nil, errors.New("tavern script title is required")
	}
	if input.Summary == "" {
		return nil, errors.New("tavern script summary is required")
	}
	if input.Description == "" {
		return nil, errors.New("tavern script description is required")
	}
	switch input.Status {
	case "", TavernScriptStatusDraft:
		input.Status = TavernScriptStatusDraft
	case TavernScriptStatusPending:
		input.Status = TavernScriptStatusPending
	default:
		return nil, errors.New("tavern script can only be created as draft or pending")
	}
	return s.repo.CreateTavernScript(ctx, userID, capabilityAssetSlugBase(input.Title), input)
}

func (s *BizDecipherService) AdminListTavernScripts(ctx context.Context, query TavernScriptQuery) ([]TavernScript, error) {
	query.Status = normalizeTavernScriptStatusFilter(query.Status)
	query.Genre = normalizeTavernGenreFilter(query.Genre)
	query.Sort = normalizeTavernScriptSort(query.Sort)
	return s.repo.AdminListTavernScripts(ctx, query)
}

func (s *BizDecipherService) AdminReviewTavernScript(ctx context.Context, scriptID, reviewerID int64, input TavernScriptReviewInput) (*TavernScript, error) {
	if scriptID <= 0 {
		return nil, errors.New("invalid tavern script id")
	}
	if reviewerID <= 0 {
		return nil, errors.New("invalid reviewer id")
	}
	input.Status = normalizeTavernScriptReviewStatus(input.Status)
	if input.Status == "" {
		return nil, errors.New("invalid tavern script review status")
	}
	input.ReviewNote = strings.TrimSpace(input.ReviewNote)
	if input.QualityScore != nil {
		score := clampFloat(*input.QualityScore, 0, 100)
		input.QualityScore = &score
	}
	return s.repo.AdminReviewTavernScript(ctx, scriptID, reviewerID, input)
}

func (s *BizDecipherService) CreateTavernRoom(ctx context.Context, ownerID int64, input TavernRoomInput) (*TavernRoom, error) {
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	input = normalizeTavernRoomInput(input)
	if input.ScriptID <= 0 {
		return nil, errors.New("tavern script id is required")
	}
	if input.Title == "" {
		return nil, errors.New("tavern room title is required")
	}
	script, err := s.repo.GetTavernScript(ctx, input.ScriptID, false)
	if err != nil {
		return nil, err
	}
	if script == nil || script.Status != TavernScriptStatusListed || script.Visibility != "public" {
		return nil, sql.ErrNoRows
	}
	if (script.EntryPriceUSD == "" || script.EntryPriceUSD == "0.00000000") && (input.EntryBalanceCost > 0 || input.BillingMode == "balance" || input.BillingMode == "hybrid") {
		return nil, ErrTavernBalanceBillingUnsupported
	}
	if input.MaxPlayers < script.PlayerMin || input.MaxPlayers > script.PlayerMax {
		input.MaxPlayers = script.PlayerMax
	}
	pkg, err := s.repo.GetLatestPublishedTavernGamePackage(ctx, input.ScriptID)
	if err != nil {
		return nil, err
	}
	if !tavernGamePackagePublishedForRoom(pkg) {
		return nil, ErrTavernGamePackageUnavailable
	}
	input.PackageID = pkg.ID
	return s.repo.CreateTavernRoom(ctx, ownerID, input)
}

func (s *BizDecipherService) ListMyTavernRooms(ctx context.Context, ownerID int64, status string, limit int) ([]TavernRoom, error) {
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	return s.repo.ListMyTavernRooms(ctx, TavernRoomQuery{OwnerID: ownerID, Status: normalizeTavernRoomStatusFilter(status), Limit: limit})
}

func (s *BizDecipherService) GetTavernRoomRuntime(ctx context.Context, userID, roomID int64) (*TavernRuntimeConfig, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if roomID <= 0 {
		return nil, errors.New("invalid tavern room id")
	}
	room, err := s.repo.GetTavernRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, sql.ErrNoRows
	}
	if room.Status == TavernRoomStatusCompleted || room.Status == TavernRoomStatusCancelled {
		return nil, ErrTavernRoomState
	}
	role := "player"
	if room.OwnerID == userID {
		role = "owner"
	} else {
		joined, err := s.repo.IsTavernRoomParticipant(ctx, roomID, userID)
		if err != nil {
			return nil, err
		}
		if !joined {
			return nil, ErrTavernRoomForbidden
		}
		room.CurrentUserJoined = true
	}
	script, err := s.repo.GetTavernScript(ctx, room.ScriptID, true)
	if err != nil {
		return nil, err
	}
	if script == nil {
		return nil, sql.ErrNoRows
	}
	var pkg *TavernGamePackage
	if room.PackageID != nil {
		pkg, err = s.GetTavernGamePackageForRoom(ctx, *room.PackageID)
		if err != nil {
			return nil, err
		}
	}
	return tavernRuntimeConfigFromRoom(userID, role, room, script, pkg), nil
}

func (s *BizDecipherService) CreateTavernRuntimeSession(ctx context.Context, userID, roomID int64) (*TavernRuntimeSession, error) {
	config, err := s.GetTavernRoomRuntime(ctx, userID, roomID)
	if err != nil {
		return nil, err
	}
	token, tokenHash, err := newTavernRuntimeToken()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(tavernRuntimeSessionTTL)
	session, err := s.repo.CreateTavernRuntimeSession(ctx, TavernRuntimeSessionInput{
		TokenHash: tokenHash,
		RoomID:    config.Room.ID,
		UserID:    userID,
		RuntimeID: config.RuntimeID,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, err
	}
	session.Token = token
	session.Config = config
	return session, nil
}

func (s *BizDecipherService) GetTavernRuntimeSession(ctx context.Context, userID int64, token string) (*TavernRuntimeSession, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("runtime session token is required")
	}
	session, err := s.repo.GetTavernRuntimeSessionByTokenHash(ctx, tavernRuntimeTokenHash(token))
	if err != nil {
		return nil, err
	}
	if session == nil || session.Status != TavernRuntimeSessionStatusActive || time.Now().UTC().After(session.ExpiresAt) {
		return nil, sql.ErrNoRows
	}
	if session.UserID != userID {
		return nil, ErrTavernRoomForbidden
	}
	config, err := s.GetTavernRoomRuntime(ctx, userID, session.RoomID)
	if err != nil {
		return nil, err
	}
	session.Config = config
	if err := s.repo.TouchTavernRuntimeSession(ctx, session.ID); err != nil {
		return nil, err
	}
	return session, nil
}

func tavernRuntimeConfigFromRoom(userID int64, role string, room *TavernRoom, script *TavernScript, pkg *TavernGamePackage) *TavernRuntimeConfig {
	config := &TavernRuntimeConfig{
		RuntimeID: fmt.Sprintf("tavern-room-%d-user-%d", room.ID, userID),
		Room:      *room,
		Script:    *script,
		Package:   pkg,
		Participant: TavernRuntimeParticipant{
			UserID: userID,
			Role:   role,
		},
		Bridge: TavernRuntimeBridge{
			Provider:                "sillytavern",
			Mode:                    "declarative_no_remote_code",
			ConfigVersion:           TavernGamePackageProtocolV1,
			ProtocolVersion:         TavernGamePackageProtocolV1,
			SandboxMode:             "no_remote_code",
			ExternalRuntimeIsolated: true,
			AGPLIsolated:            false,
			LaunchURL:               fmt.Sprintf("/tavern-stage?room=%d", room.ID),
		},
		Gateway: TavernRuntimeGateway{
			Source:        "workbench_canonical",
			ModelStrategy: "owner_authorized_quote",
			ProxyPath:     fmt.Sprintf("/api/v1/biz/tavern/rooms/%d/ai-turns", room.ID),
		},
		Budget: TavernRuntimeBudget{
			BillingMode:      room.BillingMode,
			EntryCreditCost:  room.EntryCreditCost,
			EntryBalanceCost: room.EntryBalanceCost,
			TurnBudget:       runtimeTurnBudget(room.RoomConfig, pkg),
		},
		Prompts: TavernRuntimePrompts{
			HostBrief:     script.HostBrief,
			OpeningPrompt: script.OpeningPrompt,
			SafetyNotes:   script.SafetyNotes,
			NPCCards:      script.NPCCards,
		},
	}
	if pkg != nil {
		config.Bridge.Provider = pkg.RuntimeKind
		config.Bridge.ConfigVersion = pkg.ProtocolVersion
		config.Bridge.ProtocolVersion = pkg.ProtocolVersion
	}
	return config
}

func (s *BizDecipherService) OpenTavernRoom(ctx context.Context, ownerID, roomID int64) (*TavernRoom, error) {
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	if roomID <= 0 {
		return nil, errors.New("invalid tavern room id")
	}
	return s.repo.OpenTavernRoom(ctx, roomID, ownerID)
}

func (s *BizDecipherService) JoinTavernRoom(ctx context.Context, userID, roomID int64) (*TavernRoom, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if roomID <= 0 {
		return nil, errors.New("invalid tavern room id")
	}
	return s.repo.JoinTavernRoomTx(ctx, roomID, userID)
}

func (s *BizDecipherService) StartTavernRoom(ctx context.Context, ownerID, roomID int64) (*TavernRoom, error) {
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	if roomID <= 0 {
		return nil, errors.New("invalid tavern room id")
	}
	return s.repo.StartTavernRoom(ctx, roomID, ownerID)
}

func (s *BizDecipherService) CompleteTavernRoom(ctx context.Context, ownerID, roomID int64) (*TavernRoom, error) {
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	if roomID <= 0 {
		return nil, errors.New("invalid tavern room id")
	}
	return s.repo.CompleteTavernRoom(ctx, roomID, ownerID)
}

func (s *BizDecipherService) CancelTavernRoom(ctx context.Context, ownerID, roomID int64) (*TavernRoom, error) {
	if ownerID <= 0 {
		return nil, errors.New("invalid owner id")
	}
	if roomID <= 0 {
		return nil, errors.New("invalid tavern room id")
	}
	if _, err := s.prepareTavernRefunds(ctx, roomID, ownerID); err != nil {
		return nil, err
	}
	room, err := s.repo.CancelTavernRoom(ctx, roomID, ownerID)
	if err != nil {
		return nil, err
	}
	// Reload recipients: a ticket may have joined between preflight and the
	// transaction's room lock. Terminal retries repair all committed refunds.
	if _, err = s.prepareTavernRefunds(ctx, roomID, ownerID); err != nil {
		return nil, err
	}
	return room, nil
}

func (s *BizDecipherService) ListTavernRoomTurns(ctx context.Context, userID, roomID int64, afterIndex, limit int) ([]TavernRoomTurn, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if roomID <= 0 {
		return nil, errors.New("invalid tavern room id")
	}
	room, err := s.repo.GetTavernRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, sql.ErrNoRows
	}
	if room.OwnerID != userID {
		joined, err := s.repo.IsTavernRoomParticipant(ctx, roomID, userID)
		if err != nil {
			return nil, err
		}
		if !joined {
			return nil, ErrTavernRoomForbidden
		}
	}
	turnRepo, ok := s.repo.(TavernRoomTurnRepository)
	if !ok {
		return nil, errors.New("tavern room turns are not available")
	}
	if afterIndex < 0 {
		afterIndex = 0
	}
	return turnRepo.ListTavernRoomTurns(ctx, roomID, afterIndex, limit)
}

func (s *BizDecipherService) AppendTavernRoomTurn(ctx context.Context, userID, roomID int64, input TavernRoomTurnInput) (*TavernRoomTurn, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if roomID <= 0 {
		return nil, errors.New("invalid tavern room id")
	}
	input.ClientMessageID = strings.TrimSpace(input.ClientMessageID)
	input.Body = strings.TrimSpace(input.Body)
	if input.ClientMessageID == "" || len(input.ClientMessageID) > 96 {
		return nil, errors.New("client_message_id is required and must be at most 96 characters")
	}
	if input.Body == "" {
		return nil, errors.New("tavern turn body is required")
	}
	if len([]rune(input.Body)) > 4000 {
		return nil, errors.New("tavern turn body must be at most 4000 characters")
	}
	turnRepo, ok := s.repo.(TavernRoomTurnRepository)
	if !ok {
		return nil, errors.New("tavern room turns are not available")
	}
	return turnRepo.AppendTavernRoomTurnTx(ctx, roomID, userID, input)
}

func newTavernRuntimeToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, tavernRuntimeTokenHash(token), nil
}

func tavernRuntimeTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func runtimeTurnBudget(config map[string]any, pkg *TavernGamePackage) int {
	if pkg != nil && pkg.Manifest.Limits.MaxTurns > 0 {
		return pkg.Manifest.Limits.MaxTurns
	}
	if config != nil {
		if value, ok := config["turn_budget"]; ok {
			switch typed := value.(type) {
			case float64:
				if typed > 0 {
					return int(typed)
				}
			case int:
				if typed > 0 {
					return typed
				}
			case string:
				if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil && parsed > 0 {
					return parsed
				}
			}
		}
	}
	return 30
}

func normalizeTavernScriptInput(input TavernScriptInput) TavernScriptInput {
	input.Title = truncateCapabilityString(strings.TrimSpace(input.Title), 160)
	input.Summary = truncateCapabilityString(strings.TrimSpace(input.Summary), 360)
	input.Description = strings.TrimSpace(input.Description)
	input.Genre = normalizeTavernGenre(input.Genre)
	input.Status = strings.TrimSpace(input.Status)
	input.Visibility = normalizeTavernVisibility(input.Visibility)
	if input.PlayerMin <= 0 {
		input.PlayerMin = 1
	}
	if input.PlayerMax <= 0 {
		input.PlayerMax = 6
	}
	if input.PlayerMin > input.PlayerMax {
		input.PlayerMin = input.PlayerMax
	}
	if input.PlayerMax > 12 {
		input.PlayerMax = 12
	}
	if input.EstimatedMinutes <= 0 {
		input.EstimatedMinutes = 60
	}
	if input.EstimatedMinutes < 10 {
		input.EstimatedMinutes = 10
	}
	if input.EstimatedMinutes > 480 {
		input.EstimatedMinutes = 480
	}
	input.Difficulty = normalizeTavernDifficulty(input.Difficulty)
	input.Tags = normalizeCapabilityStringList(input.Tags, 10, 40)
	input.NPCCards = normalizeCapabilityStringList(input.NPCCards, 24, 120)
	input.HostBrief = strings.TrimSpace(input.HostBrief)
	input.OpeningPrompt = strings.TrimSpace(input.OpeningPrompt)
	input.SafetyNotes = strings.TrimSpace(input.SafetyNotes)
	input.PricingMode = normalizeTavernPricing(input.PricingMode)
	if input.EntryCreditCost < 0 {
		input.EntryCreditCost = 0
	}
	if input.EntryBalanceCost < 0 {
		input.EntryBalanceCost = 0
	}
	input.AuthorRevenueShare = clampFloat(input.AuthorRevenueShare, 0, 100)
	return input
}

func normalizeTavernRoomInput(input TavernRoomInput) TavernRoomInput {
	input.Title = truncateCapabilityString(strings.TrimSpace(input.Title), 160)
	input.Visibility = normalizeTavernVisibility(input.Visibility)
	input.HostMode = normalizeTavernHostMode(input.HostMode)
	input.BillingMode = normalizeTavernPricing(input.BillingMode)
	if input.EntryCreditCost < 0 {
		input.EntryCreditCost = 0
	}
	if input.EntryBalanceCost < 0 {
		input.EntryBalanceCost = 0
	}
	if input.MaxPlayers <= 0 {
		input.MaxPlayers = 6
	}
	if input.MaxPlayers > 12 {
		input.MaxPlayers = 12
	}
	if input.RoomConfig == nil {
		input.RoomConfig = map[string]any{}
	}
	return input
}

func normalizeTavernGenre(value string) string {
	switch strings.TrimSpace(value) {
	case "mystery", "sci_fi", "fantasy", "horror", "workplace", "historical", "open_world", "other":
		return strings.TrimSpace(value)
	default:
		return "mystery"
	}
}

func normalizeTavernGenreFilter(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "all" {
		return ""
	}
	return normalizeTavernGenre(value)
}

func normalizeTavernVisibility(value string) string {
	switch strings.TrimSpace(value) {
	case "public", "private":
		return strings.TrimSpace(value)
	default:
		return "private"
	}
}

func normalizeTavernDifficulty(value string) string {
	switch strings.TrimSpace(value) {
	case "easy", "normal", "hard", "expert":
		return strings.TrimSpace(value)
	default:
		return "normal"
	}
}

func normalizeTavernPricing(value string) string {
	switch strings.TrimSpace(value) {
	case "free", "credit", "balance", "hybrid":
		return strings.TrimSpace(value)
	default:
		return "free"
	}
}

func normalizeTavernHostMode(value string) string {
	switch strings.TrimSpace(value) {
	case "ai_host", "human_host", "mixed":
		return strings.TrimSpace(value)
	default:
		return "ai_host"
	}
}

func normalizeTavernScriptStatusFilter(status string) string {
	status = strings.TrimSpace(status)
	if status == "" || status == "all" {
		return ""
	}
	switch status {
	case TavernScriptStatusDraft, TavernScriptStatusPending, TavernScriptStatusListed, TavernScriptStatusRejected, TavernScriptStatusArchived, TavernScriptStatusDelisted:
		return status
	default:
		return ""
	}
}

func normalizeTavernScriptReviewStatus(status string) string {
	switch strings.TrimSpace(status) {
	case TavernScriptStatusListed, TavernScriptStatusRejected, TavernScriptStatusDelisted:
		return strings.TrimSpace(status)
	default:
		return ""
	}
}

func normalizeTavernRoomStatusFilter(status string) string {
	status = strings.TrimSpace(status)
	if status == "" || status == "all" {
		return ""
	}
	switch status {
	case TavernRoomStatusDraft, TavernRoomStatusLobby, TavernRoomStatusRunning, TavernRoomStatusPaused, TavernRoomStatusCompleted, TavernRoomStatusCancelled:
		return status
	default:
		return ""
	}
}

func normalizeTavernScriptSort(sort string) string {
	switch strings.TrimSpace(sort) {
	case "latest", "popular", "updated", "quality":
		return strings.TrimSpace(sort)
	default:
		return "quality"
	}
}
