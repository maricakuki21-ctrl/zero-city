package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const tavernScriptColumns = `
	s.id, s.user_id,
	COALESCE(NULLIF(bp.display_name, ''), split_part(u.email, '@', 1), 'User') AS author,
	s.title, s.slug, s.summary, s.description, s.genre, s.status, s.visibility,
	s.player_min, s.player_max, s.estimated_minutes, s.difficulty,
	s.tags, s.npc_cards, s.host_brief, s.opening_prompt, s.safety_notes,
	s.pricing_mode, s.entry_credit_cost, s.entry_balance_cost, s.author_revenue_share,
	s.quality_score, s.review_note, s.reviewed_by, s.reviewed_at,
	s.published_at, s.archived_at, s.created_at, s.updated_at, s.entry_price_usd::text`

const tavernRoomColumns = `
	r.id, r.script_id, r.package_id,
	COALESCE(p.version, '') AS package_version,
	COALESCE(p.runtime_kind, '') AS package_runtime_kind,
	COALESCE(p.status, '') AS package_status,
	r.owner_id,
	COALESCE(NULLIF(bp.display_name, ''), split_part(u.email, '@', 1), 'User') AS owner,
	s.title AS script_title,
	r.title, r.status, r.visibility, r.host_mode, r.billing_mode,
	r.entry_credit_cost, r.entry_balance_cost, r.max_players, r.current_players,
	r.current_phase, r.room_config, r.started_at, r.ended_at, r.created_at, r.updated_at, r.ticket_price_usd::text`

const tavernRoomTurnColumns = `
	t.id, t.room_id, t.author_user_id, t.author_name, t.author_role,
	t.turn_index, t.kind, t.client_message_id, t.body, t.created_at`

func tavernScriptBaseQuery() string {
	return `SELECT ` + tavernScriptColumns + `
		FROM tavern_scripts s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN biz_profiles bp ON bp.user_id = s.user_id`
}

func tavernRoomBaseQuery() string {
	return `SELECT ` + tavernRoomColumns + `
		FROM tavern_rooms r
		JOIN tavern_scripts s ON s.id = r.script_id
		LEFT JOIN tavern_game_packages p ON p.id = r.package_id
		JOIN users u ON u.id = r.owner_id
		LEFT JOIN biz_profiles bp ON bp.user_id = r.owner_id`
}

func scanTavernScript(s scanner) (*service.TavernScript, error) {
	var script service.TavernScript
	var tagsRaw, npcRaw []byte
	var reviewedBy sql.NullInt64
	var reviewedAt, publishedAt, archivedAt sql.NullTime
	if err := s.Scan(
		&script.ID, &script.UserID, &script.Author,
		&script.Title, &script.Slug, &script.Summary, &script.Description, &script.Genre, &script.Status, &script.Visibility,
		&script.PlayerMin, &script.PlayerMax, &script.EstimatedMinutes, &script.Difficulty,
		&tagsRaw, &npcRaw, &script.HostBrief, &script.OpeningPrompt, &script.SafetyNotes,
		&script.PricingMode, &script.EntryCreditCost, &script.EntryBalanceCost, &script.AuthorRevenueShare,
		&script.QualityScore, &script.ReviewNote, &reviewedBy, &reviewedAt,
		&publishedAt, &archivedAt, &script.CreatedAt, &script.UpdatedAt, &script.EntryPriceUSD,
	); err != nil {
		return nil, err
	}
	script.Tags = scanStringArray(tagsRaw)
	script.NPCCards = scanStringArray(npcRaw)
	if reviewedBy.Valid {
		script.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		script.ReviewedAt = &reviewedAt.Time
	}
	if publishedAt.Valid {
		script.PublishedAt = &publishedAt.Time
	}
	if archivedAt.Valid {
		script.ArchivedAt = &archivedAt.Time
	}
	return &script, nil
}

func scanTavernRoom(s scanner) (*service.TavernRoom, error) {
	var room service.TavernRoom
	var configRaw []byte
	var packageID sql.NullInt64
	var startedAt, endedAt sql.NullTime
	if err := s.Scan(
		&room.ID, &room.ScriptID, &packageID,
		&room.PackageVersion, &room.PackageRuntimeKind, &room.PackageStatus,
		&room.OwnerID, &room.Owner, &room.ScriptTitle,
		&room.Title, &room.Status, &room.Visibility, &room.HostMode, &room.BillingMode,
		&room.EntryCreditCost, &room.EntryBalanceCost, &room.MaxPlayers, &room.CurrentPlayers,
		&room.CurrentPhase, &configRaw, &startedAt, &endedAt, &room.CreatedAt, &room.UpdatedAt, &room.TicketPriceUSD,
	); err != nil {
		return nil, err
	}
	if len(configRaw) > 0 {
		_ = json.Unmarshal(configRaw, &room.RoomConfig)
	}
	if packageID.Valid {
		value := packageID.Int64
		room.PackageID = &value
	}
	if room.RoomConfig == nil {
		room.RoomConfig = map[string]any{}
	}
	if startedAt.Valid {
		room.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		room.EndedAt = &endedAt.Time
	}
	return &room, nil
}

func scanTavernRoomTurn(s scanner) (*service.TavernRoomTurn, error) {
	var turn service.TavernRoomTurn
	var authorUserID sql.NullInt64
	if err := s.Scan(
		&turn.ID, &turn.RoomID, &authorUserID, &turn.AuthorName, &turn.AuthorRole,
		&turn.TurnIndex, &turn.Kind, &turn.ClientMessageID, &turn.Body, &turn.CreatedAt,
	); err != nil {
		return nil, err
	}
	if authorUserID.Valid {
		turn.AuthorUserID = authorUserID.Int64
	}
	return &turn, nil
}

func tavernScriptOrderBy(sort string, admin bool) string {
	switch sort {
	case "latest":
		return "s.published_at DESC NULLS LAST, s.created_at DESC, s.id DESC"
	case "popular":
		return "s.quality_score DESC, s.published_at DESC NULLS LAST, s.id DESC"
	case "updated":
		return "s.updated_at DESC, s.id DESC"
	case "quality":
		fallthrough
	default:
		if admin {
			return "s.updated_at DESC, s.id DESC"
		}
		return "s.quality_score DESC, s.published_at DESC NULLS LAST, s.id DESC"
	}
}

func (r *bizDecipherRepository) ListTavernScripts(ctx context.Context, query service.TavernScriptQuery) ([]service.TavernScript, error) {
	return r.listTavernScripts(ctx, query, false)
}

func (r *bizDecipherRepository) AdminListTavernScripts(ctx context.Context, query service.TavernScriptQuery) ([]service.TavernScript, error) {
	return r.listTavernScripts(ctx, query, true)
}

func (r *bizDecipherRepository) listTavernScripts(ctx context.Context, query service.TavernScriptQuery, admin bool) ([]service.TavernScript, error) {
	clauses := []string{"s.deleted_at IS NULL"}
	args := []any{}
	if query.Status != "" {
		args = append(args, query.Status)
		clauses = append(clauses, fmt.Sprintf("s.status = $%d", len(args)))
	} else if !admin {
		clauses = append(clauses, "s.status = 'listed' AND s.visibility = 'public'")
	}
	if query.Genre != "" {
		args = append(args, query.Genre)
		clauses = append(clauses, fmt.Sprintf("s.genre = $%d", len(args)))
	}
	if strings.TrimSpace(query.Keyword) != "" {
		args = append(args, "%"+strings.TrimSpace(query.Keyword)+"%")
		idx := len(args)
		clauses = append(clauses, fmt.Sprintf("(s.title ILIKE $%d OR s.summary ILIKE $%d OR s.description ILIKE $%d)", idx, idx, idx))
	}
	args = append(args, clampLimit(query.Limit))
	stmt := tavernScriptBaseQuery() + `
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY ` + tavernScriptOrderBy(query.Sort, admin) + fmt.Sprintf(" LIMIT $%d", len(args))
	rows, err := r.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.TavernScript{}
	for rows.Next() {
		script, err := scanTavernScript(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *script)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) GetTavernScript(ctx context.Context, id int64, includeUnlisted bool) (*service.TavernScript, error) {
	where := "s.deleted_at IS NULL AND s.id = $1"
	if !includeUnlisted {
		where += " AND s.status = 'listed' AND s.visibility = 'public'"
	}
	row := r.db.QueryRowContext(ctx, tavernScriptBaseQuery()+" WHERE "+where, id)
	script, err := scanTavernScript(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return script, nil
}

func (r *bizDecipherRepository) ListMyTavernScripts(ctx context.Context, userID int64, status string, limit int) ([]service.TavernScript, error) {
	clauses := []string{"s.deleted_at IS NULL", "s.user_id = $1"}
	args := []any{userID}
	if status != "" {
		args = append(args, status)
		clauses = append(clauses, fmt.Sprintf("s.status = $%d", len(args)))
	}
	args = append(args, clampLimit(limit))
	stmt := tavernScriptBaseQuery() + `
		WHERE ` + strings.Join(clauses, " AND ") + fmt.Sprintf(`
		ORDER BY s.updated_at DESC, s.id DESC LIMIT $%d`, len(args))
	rows, err := r.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.TavernScript{}
	for rows.Next() {
		script, err := scanTavernScript(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *script)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) CreateTavernScript(ctx context.Context, userID int64, slugBase string, input service.TavernScriptInput) (*service.TavernScript, error) {
	slug := fmt.Sprintf("%s-%d", slugBase, time.Now().UnixNano()%1000000)
	var scriptID int64
	if err := r.db.QueryRowContext(ctx, `
		INSERT INTO tavern_scripts (
			user_id, title, slug, summary, description, genre, status, visibility,
			player_min, player_max, estimated_minutes, difficulty,
			tags, npc_cards, host_brief, opening_prompt, safety_notes,
			pricing_mode, entry_credit_cost, entry_balance_cost, author_revenue_share,
			published_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7::text, $8,
			$9, $10, $11, $12,
			$13::jsonb, $14::jsonb, $15, $16, $17,
			$18, $19, $20, $21,
			CASE WHEN $7::text = 'listed' THEN NOW() ELSE NULL END
		)
		RETURNING id`,
		userID, input.Title, slug, input.Summary, input.Description, input.Genre, input.Status, input.Visibility,
		input.PlayerMin, input.PlayerMax, input.EstimatedMinutes, input.Difficulty,
		string(jsonBytes(input.Tags)), string(jsonBytes(input.NPCCards)), input.HostBrief, input.OpeningPrompt, input.SafetyNotes,
		input.PricingMode, input.EntryCreditCost, input.EntryBalanceCost, input.AuthorRevenueShare,
	).Scan(&scriptID); err != nil {
		return nil, err
	}
	return r.GetTavernScript(ctx, scriptID, true)
}

func (r *bizDecipherRepository) AdminReviewTavernScript(ctx context.Context, scriptID, reviewerID int64, input service.TavernScriptReviewInput) (*service.TavernScript, error) {
	qualitySQL := "quality_score"
	args := []any{scriptID, reviewerID, input.Status, input.ReviewNote}
	if input.QualityScore != nil {
		args = append(args, *input.QualityScore)
		qualitySQL = fmt.Sprintf("$%d", len(args))
	}
	stmt := fmt.Sprintf(`
		UPDATE tavern_scripts
		SET status = $3::text,
			visibility = CASE WHEN $3::text = 'listed' THEN 'public' ELSE visibility END,
			review_note = $4,
			reviewed_by = $2,
			reviewed_at = NOW(),
			published_at = CASE WHEN $3::text = 'listed' AND published_at IS NULL THEN NOW() ELSE published_at END,
			archived_at = CASE WHEN $3::text IN ('archived', 'delisted') THEN NOW() ELSE archived_at END,
			quality_score = %s,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id`, qualitySQL)
	var updatedID int64
	if err := r.db.QueryRowContext(ctx, stmt, args...).Scan(&updatedID); err != nil {
		return nil, err
	}
	return r.GetTavernScript(ctx, updatedID, true)
}

func (r *bizDecipherRepository) CreateTavernRoom(ctx context.Context, ownerID int64, input service.TavernRoomInput) (*service.TavernRoom, error) {
	var roomID int64
	if err := r.db.QueryRowContext(ctx, `
		INSERT INTO tavern_rooms (
			script_id, package_id, owner_id, title, status, visibility, host_mode, billing_mode,
			entry_credit_cost, entry_balance_cost, max_players, current_phase, room_config,ticket_price_usd,ticket_author_id
		) SELECT
			$1, $2, $3, $4, 'draft', $5, $6, CASE WHEN s.entry_price_usd>0 THEN 'balance' ELSE $7 END,
			CASE WHEN s.entry_price_usd>0 THEN 0 ELSE $8 END, CASE WHEN s.entry_price_usd>0 THEN 0 ELSE $9 END, $10, 'lobby', $11::jsonb,
			NULLIF(s.entry_price_usd,0),CASE WHEN s.entry_price_usd>0 THEN s.user_id ELSE NULL END
		FROM tavern_scripts s WHERE s.id=$1 AND s.status='listed' AND s.deleted_at IS NULL
		RETURNING id`,
		input.ScriptID, input.PackageID, ownerID, input.Title, input.Visibility, input.HostMode, input.BillingMode,
		input.EntryCreditCost, input.EntryBalanceCost, input.MaxPlayers, string(jsonBytes(input.RoomConfig)),
	).Scan(&roomID); err != nil {
		return nil, err
	}
	return r.GetTavernRoom(ctx, roomID)
}

func (r *bizDecipherRepository) GetTavernRoom(ctx context.Context, id int64) (*service.TavernRoom, error) {
	row := r.db.QueryRowContext(ctx, tavernRoomBaseQuery()+" WHERE r.deleted_at IS NULL AND r.id = $1", id)
	room, err := scanTavernRoom(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return room, nil
}

func (r *bizDecipherRepository) ListTavernRoomTurns(ctx context.Context, roomID int64, afterIndex, limit int) ([]service.TavernRoomTurn, error) {
	if afterIndex < 0 {
		afterIndex = 0
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+tavernRoomTurnColumns+`
		FROM tavern_room_turns t
		WHERE t.room_id = $1 AND t.turn_index > $2
		ORDER BY t.turn_index ASC, t.id ASC
		LIMIT $3`, roomID, afterIndex, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.TavernRoomTurn{}
	for rows.Next() {
		turn, err := scanTavernRoomTurn(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *turn)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) AppendTavernRoomTurnTx(ctx context.Context, roomID, userID int64, input service.TavernRoomTurnInput) (*service.TavernRoomTurn, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var roomStatus string
	var ownerID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, owner_id
		FROM tavern_rooms
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE`, roomID).Scan(&roomStatus, &ownerID); err != nil {
		return nil, err
	}
	if roomStatus != service.TavernRoomStatusRunning {
		return nil, service.ErrTavernRoomState
	}
	authorRole := "player"
	if ownerID == userID {
		authorRole = "owner"
	} else {
		var playerStatus string
		if err := tx.QueryRowContext(ctx, `
			SELECT status
			FROM tavern_room_players
			WHERE room_id = $1 AND user_id = $2
			FOR UPDATE`, roomID, userID).Scan(&playerStatus); err != nil {
			if err == sql.ErrNoRows {
				return nil, service.ErrTavernRoomForbidden
			}
			return nil, err
		}
		if playerStatus != "joined" {
			return nil, service.ErrTavernRoomForbidden
		}
	}

	if existing, err := scanTavernRoomTurn(tx.QueryRowContext(ctx, `
		SELECT `+tavernRoomTurnColumns+`
		FROM tavern_room_turns t
		WHERE t.room_id = $1 AND t.client_message_id = $2`, roomID, input.ClientMessageID)); err == nil {
		if existing.Body != input.Body || existing.AuthorUserID != userID {
			return nil, service.ErrTavernTurnConflict
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	var authorName string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(NULLIF(bp.display_name, ''), split_part(u.email, '@', 1), 'User')
		FROM users u
		LEFT JOIN biz_profiles bp ON bp.user_id = u.id
		WHERE u.id = $1`, userID).Scan(&authorName); err != nil {
		return nil, err
	}
	var nextIndex int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(turn_index), 0) + 1
		FROM tavern_room_turns
		WHERE room_id = $1`, roomID).Scan(&nextIndex); err != nil {
		return nil, err
	}

	turn, err := scanTavernRoomTurn(tx.QueryRowContext(ctx, `
		INSERT INTO tavern_room_turns (
			room_id, author_user_id, author_name, author_role,
			turn_index, kind, client_message_id, body
		) VALUES ($1, $2, $3, $4, $5, 'player', $6, $7)
		RETURNING id, room_id, author_user_id, author_name, author_role,
			turn_index, kind, client_message_id, body, created_at`,
		roomID, userID, authorName, authorRole, nextIndex, input.ClientMessageID, input.Body))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return turn, nil
}

func (r *bizDecipherRepository) ListMyTavernRooms(ctx context.Context, query service.TavernRoomQuery) ([]service.TavernRoom, error) {
	clauses := []string{"r.deleted_at IS NULL", `(r.owner_id = $1 OR EXISTS (
		SELECT 1
		FROM tavern_room_players trp
		WHERE trp.room_id = r.id AND trp.user_id = $1 AND trp.status = 'joined'
	))`}
	args := []any{query.OwnerID}
	if query.Status != "" {
		args = append(args, query.Status)
		clauses = append(clauses, fmt.Sprintf("r.status = $%d", len(args)))
	}
	args = append(args, clampLimit(query.Limit))
	stmt := tavernRoomBaseQuery() + `
		WHERE ` + strings.Join(clauses, " AND ") + fmt.Sprintf(`
		ORDER BY r.updated_at DESC, r.id DESC LIMIT $%d`, len(args))
	rows, err := r.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.TavernRoom{}
	for rows.Next() {
		room, err := scanTavernRoom(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *room)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.markTavernRoomsJoinedByUser(ctx, out, query.OwnerID); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *bizDecipherRepository) IsTavernRoomParticipant(ctx context.Context, roomID, userID int64) (bool, error) {
	var joined bool
	if err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM tavern_room_players
			WHERE room_id = $1 AND user_id = $2 AND status = 'joined'
		)`, roomID, userID).Scan(&joined); err != nil {
		return false, err
	}
	return joined, nil
}

func (r *bizDecipherRepository) CreateTavernRuntimeSession(ctx context.Context, input service.TavernRuntimeSessionInput) (*service.TavernRuntimeSession, error) {
	var session service.TavernRuntimeSession
	if err := r.db.QueryRowContext(ctx, `
		INSERT INTO tavern_runtime_sessions (token_hash, room_id, user_id, runtime_id, status, expires_at)
		VALUES ($1, $2, $3, $4, 'active', $5)
		RETURNING id, room_id, user_id, runtime_id, status, expires_at, created_at, updated_at`,
		input.TokenHash, input.RoomID, input.UserID, input.RuntimeID, input.ExpiresAt,
	).Scan(
		&session.ID,
		&session.RoomID,
		&session.UserID,
		&session.RuntimeID,
		&session.Status,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *bizDecipherRepository) GetTavernRuntimeSessionByTokenHash(ctx context.Context, tokenHash string) (*service.TavernRuntimeSession, error) {
	var session service.TavernRuntimeSession
	if err := r.db.QueryRowContext(ctx, `
		SELECT id, room_id, user_id, runtime_id, status, expires_at, created_at, updated_at
		FROM tavern_runtime_sessions
		WHERE token_hash = $1 AND status = 'active'`, tokenHash).Scan(
		&session.ID,
		&session.RoomID,
		&session.UserID,
		&session.RuntimeID,
		&session.Status,
		&session.ExpiresAt,
		&session.CreatedAt,
		&session.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func (r *bizDecipherRepository) TouchTavernRuntimeSession(ctx context.Context, sessionID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE tavern_runtime_sessions
		SET last_seen_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'active'`, sessionID)
	return err
}

func (r *bizDecipherRepository) markTavernRoomsJoinedByUser(ctx context.Context, rooms []service.TavernRoom, userID int64) error {
	if len(rooms) == 0 || userID <= 0 {
		return nil
	}
	ids := make([]int64, 0, len(rooms))
	for _, room := range rooms {
		ids = append(ids, room.ID)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT room_id
		FROM tavern_room_players
		WHERE user_id = $1 AND status = 'joined' AND room_id = ANY($2)`, userID, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	joined := map[int64]bool{}
	for rows.Next() {
		var roomID int64
		if err := rows.Scan(&roomID); err != nil {
			return err
		}
		joined[roomID] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for idx := range rooms {
		rooms[idx].CurrentUserJoined = joined[rooms[idx].ID]
	}
	return nil
}

func (r *bizDecipherRepository) OpenTavernRoom(ctx context.Context, roomID, ownerID int64) (*service.TavernRoom, error) {
	return r.transitionTavernRoom(ctx, roomID, ownerID, []string{service.TavernRoomStatusDraft, service.TavernRoomStatusLobby}, service.TavernRoomStatusLobby, true)
}

func (r *bizDecipherRepository) StartTavernRoom(ctx context.Context, roomID, ownerID int64) (*service.TavernRoom, error) {
	return r.transitionTavernRoom(ctx, roomID, ownerID, []string{service.TavernRoomStatusLobby}, service.TavernRoomStatusRunning, false)
}

func (r *bizDecipherRepository) CompleteTavernRoom(ctx context.Context, roomID, ownerID int64) (*service.TavernRoom, error) {
	return r.transitionTavernRoom(ctx, roomID, ownerID, []string{service.TavernRoomStatusRunning, service.TavernRoomStatusPaused}, service.TavernRoomStatusCompleted, false)
}

func (r *bizDecipherRepository) CancelTavernRoom(ctx context.Context, roomID, ownerID int64) (*service.TavernRoom, error) {
	return r.transitionTavernRoom(ctx, roomID, ownerID, []string{service.TavernRoomStatusDraft, service.TavernRoomStatusLobby}, service.TavernRoomStatusCancelled, false)
}

func (r *bizDecipherRepository) JoinTavernRoomTx(ctx context.Context, roomID, userID int64) (*service.TavernRoom, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockTavernTicketRoom(ctx, tx, roomID); err != nil {
		return nil, err
	}
	var ticketed bool
	if err := tx.QueryRowContext(ctx, `SELECT ticket_price_usd IS NOT NULL FROM tavern_rooms WHERE id=$1 AND deleted_at IS NULL`, roomID).Scan(&ticketed); err != nil {
		return nil, err
	}
	if ticketed {
		return nil, service.ErrTavernCommerceInvalid
	}

	var status, billingMode, title string
	var entryCreditCost, currentPlayers, maxPlayers int
	var entryBalanceCost float64
	if err := tx.QueryRowContext(ctx, `
		SELECT status, billing_mode, entry_credit_cost, entry_balance_cost, current_players, max_players, title
		FROM tavern_rooms
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE`, roomID).Scan(&status, &billingMode, &entryCreditCost, &entryBalanceCost, &currentPlayers, &maxPlayers, &title); err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	var existingID int64
	existsErr := tx.QueryRowContext(ctx, `SELECT id FROM tavern_room_players WHERE room_id = $1 AND user_id = $2 AND status = 'joined'`, roomID, userID).Scan(&existingID)
	if existsErr == nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		room, err := r.GetTavernRoom(ctx, roomID)
		if room != nil {
			room.CurrentUserJoined = true
		}
		return room, err
	}
	if existsErr != sql.ErrNoRows {
		return nil, existsErr
	}

	if status != service.TavernRoomStatusLobby {
		return nil, service.ErrTavernRoomNotJoinable
	}
	if maxPlayers > 0 && currentPlayers >= maxPlayers {
		return nil, service.ErrTavernRoomFull
	}
	if entryBalanceCost > 0 || (billingMode == "balance" || billingMode == "hybrid") {
		return nil, service.ErrTavernBalanceBillingUnsupported
	}
	if entryCreditCost > 0 {
		var creditAfter float64
		if err := tx.QueryRowContext(ctx, `
			UPDATE users
			SET credit_balance = credit_balance - $1, updated_at = NOW()
			WHERE id = $2 AND credit_balance >= $1
			RETURNING credit_balance`, entryCreditCost, userID).Scan(&creditAfter); err != nil {
			if err == sql.ErrNoRows {
				return nil, service.ErrTavernInsufficientCredit
			}
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at)
			VALUES ($1, 'tavern_room_entry', $2, $3, $4, 'posted', $5, NOW())`,
			userID, fmt.Sprintf("room:%d", roomID), -entryCreditCost, creditAfter, fmt.Sprintf("Tavern room entry: %s", title)); err != nil {
			return nil, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tavern_room_players (room_id, user_id, status, joined_at)
		VALUES ($1, $2, 'joined', NOW())
		ON CONFLICT (room_id, user_id) DO UPDATE
		SET status = 'joined', joined_at = COALESCE(tavern_room_players.joined_at, NOW()), left_at = NULL`, roomID, userID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tavern_rooms SET current_players = current_players + 1, updated_at = NOW() WHERE id = $1`, roomID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	room, err := r.GetTavernRoom(ctx, roomID)
	if room != nil {
		room.CurrentUserJoined = true
	}
	return room, err
}

func stringInSlice(value string, items []string) bool {
	for _, item := range items {
		if value == item {
			return true
		}
	}
	return false
}
