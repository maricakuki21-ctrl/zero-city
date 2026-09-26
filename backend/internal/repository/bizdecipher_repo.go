package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type bizDecipherRepository struct{ db *sql.DB }

func NewBizDecipherRepository(db *sql.DB) service.BizDecipherRepository {
	return &bizDecipherRepository{db: db}
}

func nextSharedPoolSettlementHour(now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	return now.UTC().Truncate(time.Hour).Add(time.Hour)
}

func insertSharedPoolSettlementRuleVersionTx(
	ctx context.Context,
	tx *sql.Tx,
	poolID int64,
	hourlySeatFee float64,
	hourlyMinUsageWaiver float64,
	platformFeePercent float64,
	source string,
	createdBy int64,
	effectiveFrom time.Time,
) error {
	if tx == nil || poolID <= 0 {
		return nil
	}
	if effectiveFrom.IsZero() {
		effectiveFrom = nextSharedPoolSettlementHour(time.Now())
	}
	if source == "" {
		source = "system"
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO shared_pool_settlement_rule_versions
		(pool_id, hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, effective_from, source, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, 0))
		ON CONFLICT (pool_id, effective_from) DO UPDATE SET
			hourly_seat_fee = EXCLUDED.hourly_seat_fee,
			hourly_min_usage_waiver = EXCLUDED.hourly_min_usage_waiver,
			platform_fee_percent = EXCLUDED.platform_fee_percent,
			source = EXCLUDED.source,
			created_by = EXCLUDED.created_by,
			created_at = NOW()`,
		poolID,
		math.Max(0, hourlySeatFee),
		math.Max(0, hourlyMinUsageWaiver),
		math.Max(0, math.Min(100, platformFeePercent)),
		effectiveFrom.UTC(),
		source,
		createdBy,
	)
	return err
}

type sharedPoolSettlementRule struct {
	HourlySeatFee        float64
	HourlyMinUsageWaiver float64
	PlatformFeePercent   float64
	EffectiveFrom        time.Time
}

func sharedPoolSettlementRuleAtTx(
	ctx context.Context,
	tx *sql.Tx,
	poolID int64,
	at time.Time,
	fallback sharedPoolSettlementRule,
) (sharedPoolSettlementRule, error) {
	if tx == nil || poolID <= 0 {
		return fallback, nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	var rule sharedPoolSettlementRule
	err := tx.QueryRowContext(ctx, `SELECT hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, effective_from
		FROM shared_pool_settlement_rule_versions
		WHERE pool_id = $1 AND effective_from <= $2
		ORDER BY effective_from DESC, id DESC
		LIMIT 1`, poolID, at.UTC()).Scan(
		&rule.HourlySeatFee,
		&rule.HourlyMinUsageWaiver,
		&rule.PlatformFeePercent,
		&rule.EffectiveFrom,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		return fallback, err
	}
	return rule, nil
}

func jsonBytes(v any) []byte { b, _ := json.Marshal(v); return b }
func scanStringArray(raw []byte) []string {
	var out []string
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		return []string{}
	}
	return out
}

const bizProfileColumns = `user_id, COALESCE(handle, ''), display_name, avatar_url, bio, profile_visibility, profile_theme, background_card_key, background_card_rarity, background_card_serial_no, background_card_edition_no, background_card_edition_supply, created_at, updated_at`

func scanBizProfile(s scanner) (*service.BizProfile, error) {
	var p service.BizProfile
	if err := s.Scan(
		&p.UserID, &p.Handle, &p.DisplayName, &p.AvatarURL, &p.Bio,
		&p.ProfileVisibility, &p.ProfileTheme, &p.BackgroundCardKey, &p.BackgroundCardRarity,
		&p.BackgroundCardSerialNo, &p.BackgroundCardEditionNo, &p.BackgroundCardEditionSupply,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *bizDecipherRepository) EnsureProfile(ctx context.Context, userID int64) (*service.BizProfile, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO biz_profiles (user_id, display_name)
		SELECT u.id, COALESCE(NULLIF(TRIM(u.username), ''), split_part(u.email, '@', 1), 'Decoder')
		FROM users u
		WHERE u.id = $1
		ON CONFLICT (user_id) DO UPDATE SET updated_at = biz_profiles.updated_at
		RETURNING `+bizProfileColumns, userID)
	return scanBizProfile(row)
}

func (r *bizDecipherRepository) UpdateProfileIdentity(ctx context.Context, userID int64, displayName, avatarURL string) (*service.BizProfile, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE biz_profiles
		SET display_name = $2, avatar_url = $3, updated_at = NOW()
		WHERE user_id = $1
		RETURNING `+bizProfileColumns, userID, displayName, avatarURL)
	return scanBizProfile(row)
}

func (r *bizDecipherRepository) UpsertContributor(ctx context.Context, userID int64, input service.BizContributorApplication) (*service.BizContributorProfile, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO contributor_profiles (user_id, status, capacity_types, settlement_method)
		VALUES ($1, 'pending', $2::jsonb, $3)
		ON CONFLICT (user_id) DO UPDATE SET capacity_types = EXCLUDED.capacity_types, settlement_method = EXCLUDED.settlement_method, status = 'pending', updated_at = NOW()
		RETURNING id, user_id, status, capacity_types, settlement_method, risk_notes, admin_notes, created_at, updated_at`, userID, string(jsonBytes(input.CapacityTypes)), input.SettlementMethod)
	p, err := scanContributor(row)
	if err != nil {
		return nil, err
	}
	if input.Label != "" || input.CapacityHint != "" {
		_, _ = r.db.ExecContext(ctx, `INSERT INTO capacity_contributions (contributor_id, resource_type, label, capacity_hint) VALUES ($1, $2, $3, $4)`, p.ID, firstOr(input.CapacityTypes, "other"), input.Label, input.CapacityHint)
	}
	return p, nil
}

func (r *bizDecipherRepository) UpsertOperator(ctx context.Context, userID int64, input service.BizOperatorApplication) (*service.BizOperatorProfile, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO operator_profiles (user_id, status, skills_json, gateway_user_id)
		VALUES ($1, 'pending', $2::jsonb, $1)
		ON CONFLICT (user_id) DO UPDATE SET skills_json = EXCLUDED.skills_json, status = 'pending', updated_at = NOW()
		RETURNING id, user_id, status, skills_json, rank, completed_tasks, quality_score, created_at, updated_at`, userID, string(jsonBytes(input.Skills)))
	return scanOperator(row)
}

func (r *bizDecipherRepository) CreateCustomRequest(ctx context.Context, userID int64, input service.BizCustomRequestInput) (*service.BizCustomRequest, error) {
	var deadline any
	if input.Deadline != "" {
		if t, err := time.Parse(time.RFC3339, input.Deadline); err == nil {
			deadline = t
		}
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO custom_requests (user_id, goal, context, budget_range, deadline, references_json)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		RETURNING id, user_id, goal, context, budget_range, deadline, references_json, status, created_at`, userID, input.Goal, input.Context, input.Budget, deadline, string(jsonBytes(input.References)))
	return scanCustomRequest(row)
}

func (r *bizDecipherRepository) ListCreditLedger(ctx context.Context, userID int64, limit int) ([]service.BizCreditLedgerEntry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, source_type, source_id, amount, balance_after, status, note, created_by, created_at, posted_at FROM credit_ledger WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`, userID, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanCreditRows(rows)
}

func (r *bizDecipherRepository) ListAllCreditLedger(ctx context.Context, limit int) ([]service.BizCreditLedgerEntry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, source_type, source_id, amount, balance_after, status, note, created_by, created_at, posted_at FROM credit_ledger ORDER BY created_at DESC LIMIT $1`, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanCreditRows(rows)
}

func (r *bizDecipherRepository) ListContributors(ctx context.Context, limit int) ([]service.BizContributorProfile, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, status, capacity_types, settlement_method, risk_notes, admin_notes, created_at, updated_at FROM contributor_profiles ORDER BY created_at DESC LIMIT $1`, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.BizContributorProfile
	for rows.Next() {
		p, err := scanContributor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) ListOperators(ctx context.Context, limit int) ([]service.BizOperatorProfile, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, status, skills_json, rank, completed_tasks, quality_score, created_at, updated_at FROM operator_profiles ORDER BY created_at DESC LIMIT $1`, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.BizOperatorProfile
	for rows.Next() {
		p, err := scanOperator(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) ListCustomRequests(ctx context.Context, limit int) ([]service.BizCustomRequest, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, user_id, goal, context, budget_range, deadline, references_json, status, created_at FROM custom_requests ORDER BY created_at DESC LIMIT $1`, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.BizCustomRequest
	for rows.Next() {
		p, err := scanCustomRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) GrantCredit(ctx context.Context, input service.BizCreditGrantInput) (*service.BizCreditLedgerEntry, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var balance float64
	if err := tx.QueryRowContext(ctx, `UPDATE users SET credit_balance = credit_balance + $1, updated_at = NOW() WHERE id = $2 RETURNING credit_balance`, input.Amount, input.UserID).Scan(&balance); err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, created_by, posted_at) VALUES ($1, $2, $3, $4, $5, 'posted', $6, $7, NOW()) RETURNING id, user_id, source_type, source_id, amount, balance_after, status, note, created_by, created_at, posted_at`, input.UserID, input.SourceType, input.SourceID, input.Amount, balance, input.Note, input.CreatedBy)
	entry, err := scanCredit(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return entry, nil
}

func (r *bizDecipherRepository) EnsureStarterCreditLedger(ctx context.Context, input service.BizStarterCreditInput) (*service.BizCreditLedgerEntry, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var existing service.BizCreditLedgerEntry
	var createdBy sql.NullInt64
	var posted sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT id, user_id, source_type, source_id, amount, balance_after, status, note, created_by, created_at, posted_at FROM credit_ledger WHERE user_id = $1 AND source_type = 'starter' ORDER BY created_at ASC LIMIT 1`, input.UserID).Scan(&existing.ID, &existing.UserID, &existing.SourceType, &existing.SourceID, &existing.Amount, &existing.BalanceAfter, &existing.Status, &existing.Note, &createdBy, &existing.CreatedAt, &posted)
	if err == nil {
		if createdBy.Valid {
			existing.CreatedBy = &createdBy.Int64
		}
		if posted.Valid {
			existing.PostedAt = &posted.Time
		}
		return &existing, false, nil
	}
	if err != sql.ErrNoRows {
		return nil, false, err
	}

	var balance float64
	if err := tx.QueryRowContext(ctx, `SELECT credit_balance FROM users WHERE id = $1 FOR UPDATE`, input.UserID).Scan(&balance); err != nil {
		return nil, false, err
	}
	sourceID := input.SignupSource
	if sourceID == "" {
		sourceID = "signup"
	}
	note := input.Note
	if note == "" {
		note = "Starter credits granted on signup"
	}
	row := tx.QueryRowContext(ctx, `INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at) VALUES ($1, 'starter', $2, $3, $4, 'posted', $5, NOW()) ON CONFLICT DO NOTHING RETURNING id, user_id, source_type, source_id, amount, balance_after, status, note, created_by, created_at, posted_at`, input.UserID, sourceID, input.Amount, balance, note)
	entry, err := scanCredit(row)
	if err == sql.ErrNoRows {
		row = tx.QueryRowContext(ctx, `SELECT id, user_id, source_type, source_id, amount, balance_after, status, note, created_by, created_at, posted_at FROM credit_ledger WHERE user_id = $1 AND source_type = 'starter' ORDER BY created_at ASC LIMIT 1`, input.UserID)
		entry, err = scanCredit(row)
		if err != nil {
			return nil, false, err
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return entry, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return entry, true, nil
}

func (r *bizDecipherRepository) GetPoolStatus(ctx context.Context) (*service.BizPoolStatus, error) {
	status := &service.BizPoolStatus{Health: "healthy"}
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM capacity_contributions WHERE status = 'active'`).Scan(&status.ActiveResources)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM capacity_contributions WHERE status = 'testing'`).Scan(&status.TestingResources)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM contributor_profiles WHERE status = 'pending'`).Scan(&status.PendingContributors)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM custom_requests WHERE status IN ('submitted', 'reviewing', 'quoted', 'accepted', 'assigned')`).Scan(&status.OpenRequests)
	return status, nil
}

// GetPlatformStatus aggregates gateway-level health from the real channels and
// upstream accounts tables. This is deliberately distinct from GetPoolStatus
// (marketplace contributors) so the UI can show platform health separately.
func (r *bizDecipherRepository) GetPlatformStatus(ctx context.Context) (*service.PlatformStatus, error) {
	status := &service.PlatformStatus{}
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM channels WHERE status = 'active'`).Scan(&status.ActiveChannels)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM channels`).Scan(&status.TotalChannels)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE status = 'active' AND deleted_at IS NULL`).Scan(&status.ActiveAccounts)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE status = 'error' AND deleted_at IS NULL`).Scan(&status.ErrorAccounts)
	_ = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE deleted_at IS NULL`).Scan(&status.TotalAccounts)

	// Derive an aggregate verdict. "down" when nothing can serve traffic;
	// "degraded" when capacity exists but some part is unhealthy; otherwise
	// "operational".
	switch {
	case status.ActiveChannels == 0 || status.ActiveAccounts == 0:
		status.Health = "down"
	case status.ErrorAccounts > 0 || status.ActiveChannels < status.TotalChannels:
		status.Health = "degraded"
	default:
		status.Health = "operational"
	}
	return status, nil
}

func listCommunityPostsWhere(ctx context.Context, db *sql.DB, where string, args []any, limit int) ([]service.CommunityPost, error) {
	args = append(args, clampLimit(limit))
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT p.id, p.user_id,
		       COALESCE(NULLIF(bp.display_name, ''), split_part(u.email, '@', 1), 'Decoder') AS author,
		       p.kind, p.title, p.body, p.tags, p.district, p.channel, p.private, p.status, p.pinned,
		       p.catches, p.replies, p.views, p.source_type, p.source_id,
		       p.scenario, p.subject_type, p.subject_id, p.subject_title, p.action_type, p.evidence, p.trust_signals,
		       p.created_at, p.updated_at
		FROM community_posts p
		JOIN users u ON u.id = p.user_id
		LEFT JOIN biz_profiles bp ON bp.user_id = p.user_id
		%s
		ORDER BY p.pinned DESC, p.created_at DESC
		LIMIT $%d`, where, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	posts, err := scanCommunityPostRows(rows)
	if err != nil {
		return nil, err
	}
	for i := range posts {
		comments, err := listCommunityComments(ctx, db, posts[i].ID, 5, true)
		if err != nil {
			return nil, err
		}
		posts[i].Comments = comments
	}
	return posts, nil
}

func appendCommunityPostFilters(where string, args []any, query service.CommunityPostQuery) (string, []any) {
	if query.District != "" {
		args = append(args, query.District)
		where += fmt.Sprintf(" AND p.district = $%d", len(args))
	}
	if query.Channel != "" {
		args = append(args, query.Channel)
		where += fmt.Sprintf(" AND p.channel = $%d", len(args))
	}
	if query.Kind != "" {
		args = append(args, query.Kind)
		where += fmt.Sprintf(" AND p.kind = $%d", len(args))
	}
	if query.SourceType != "" {
		args = append(args, query.SourceType)
		where += fmt.Sprintf(" AND (p.source_type = $%d OR p.subject_type = $%d)", len(args), len(args))
	}
	if query.SourceID != "" {
		args = append(args, query.SourceID)
		where += fmt.Sprintf(" AND (p.source_id = $%d OR p.subject_id = $%d)", len(args), len(args))
	}
	if query.SubjectType != "" && query.SubjectType != query.SourceType {
		args = append(args, query.SubjectType)
		where += fmt.Sprintf(" AND p.subject_type = $%d", len(args))
	}
	if query.SubjectID != "" && query.SubjectID != query.SourceID {
		args = append(args, query.SubjectID)
		where += fmt.Sprintf(" AND p.subject_id = $%d", len(args))
	}
	if query.Scenario != "" {
		args = append(args, query.Scenario)
		where += fmt.Sprintf(" AND p.scenario = $%d", len(args))
	}
	if query.ActionType != "" {
		args = append(args, query.ActionType)
		where += fmt.Sprintf(" AND p.action_type = $%d", len(args))
	}
	return where, args
}

func (r *bizDecipherRepository) ListCommunityPosts(ctx context.Context, query service.CommunityPostQuery) ([]service.CommunityPost, error) {
	args := []any{}
	where := "WHERE p.deleted_at IS NULL AND p.status NOT IN ('hidden', 'deleted', 'rejected') AND p.private = FALSE"
	where, args = appendCommunityPostFilters(where, args, query)
	return listCommunityPostsWhere(ctx, r.db, where, args, query.Limit)
}

func (r *bizDecipherRepository) ListSharedPoolCommunitySummaries(ctx context.Context, poolIDs []int64, limit int) ([]service.SharedPoolCommunitySummary, error) {
	if len(poolIDs) == 0 {
		return []service.SharedPoolCommunitySummary{}, nil
	}
	args := make([]any, 0, len(poolIDs))
	placeholders := make([]string, 0, len(poolIDs))
	for i, id := range poolIDs {
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}
	poolExpr := fmt.Sprintf("ANY(ARRAY[%s]::bigint[])", strings.Join(placeholders, ","))
	query := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(p.subject_id, ''), NULLIF(p.source_id, ''))::bigint AS pool_id,
		       COUNT(*) AS total_posts,
		       COUNT(*) FILTER (WHERE p.kind = 'pool' OR p.scenario IN ('resource_decision', 'usage_intel')) AS discussion_posts,
		       COUNT(*) FILTER (WHERE p.kind IN ('feedback', 'support') OR p.scenario IN ('feedback_triage', 'incident_support')) AS feedback_posts,
		       COUNT(*) FILTER (WHERE p.scenario = 'incident_support' OR p.action_type = 'report') AS incident_posts,
		       COUNT(*) FILTER (WHERE p.kind IN ('feedback', 'support') OR p.scenario IN ('feedback_triage', 'incident_support') OR p.action_type = 'report') AS risk_signals,
		       COALESCE((array_agg(p.title ORDER BY p.created_at DESC, p.id DESC))[1], '') AS last_post_title,
		       MAX(p.created_at) AS last_post_at
		FROM community_posts p
		WHERE p.deleted_at IS NULL
		  AND p.status NOT IN ('hidden', 'deleted', 'rejected')
		  AND p.private = FALSE
		  AND (p.subject_type = 'shared_pool' OR p.source_type = 'shared_pool')
		  AND COALESCE(NULLIF(p.subject_id, ''), NULLIF(p.source_id, '')) ~ '^[0-9]+$'
		  AND COALESCE(NULLIF(p.subject_id, ''), NULLIF(p.source_id, ''))::bigint = %s
		GROUP BY pool_id`, poolExpr)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	summariesByPool := map[int64]*service.SharedPoolCommunitySummary{}
	for rows.Next() {
		var summary service.SharedPoolCommunitySummary
		if err := rows.Scan(&summary.PoolID, &summary.TotalPosts, &summary.DiscussionPosts, &summary.FeedbackPosts, &summary.IncidentPosts, &summary.RiskSignals, &summary.LastPostTitle, &summary.LastPostAt); err != nil {
			return nil, err
		}
		summariesByPool[summary.PoolID] = &summary
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]service.SharedPoolCommunitySummary, 0, len(poolIDs))
	for _, poolID := range poolIDs {
		summary := summariesByPool[poolID]
		if summary == nil {
			summary = &service.SharedPoolCommunitySummary{PoolID: poolID, LatestPosts: []service.CommunityPost{}}
		}
		posts, err := r.ListCommunityPosts(ctx, service.CommunityPostQuery{SourceType: "shared_pool", SourceID: strconv.FormatInt(poolID, 10), Limit: limit})
		if err != nil {
			return nil, err
		}
		summary.LatestPosts = posts
		out = append(out, *summary)
	}
	return out, nil
}

func (r *bizDecipherRepository) ListPublicCommunityPostsByUser(ctx context.Context, userID int64, limit int) ([]service.CommunityPost, error) {
	args := []any{userID}
	where := "WHERE p.deleted_at IS NULL AND p.status NOT IN ('hidden', 'deleted') AND p.private = FALSE AND p.user_id = $1"
	return listCommunityPostsWhere(ctx, r.db, where, args, limit)
}

func (r *bizDecipherRepository) ListMyCommunityPosts(ctx context.Context, userID int64, kind string, limit int) ([]service.CommunityPost, error) {
	args := []any{userID}
	where := "WHERE p.deleted_at IS NULL AND p.status <> 'deleted' AND p.user_id = $1"
	kind = strings.TrimSpace(kind)
	if kind != "" && kind != "all" {
		args = append(args, kind)
		where += fmt.Sprintf(" AND p.kind = $%d", len(args))
	}
	return listCommunityPostsWhere(ctx, r.db, where, args, limit)
}

func (r *bizDecipherRepository) AdminListCommunityPosts(ctx context.Context, query service.CommunityPostQuery) ([]service.CommunityPost, error) {
	args := []any{}
	where := "WHERE p.deleted_at IS NULL"
	where, args = appendCommunityPostFilters(where, args, query)
	if query.Status != "" {
		args = append(args, query.Status)
		where += fmt.Sprintf(" AND p.status = $%d", len(args))
	}
	if query.PrivateOnly {
		where += " AND p.private = TRUE"
	}
	return listCommunityPostsWhere(ctx, r.db, where, args, query.Limit)
}

func (r *bizDecipherRepository) UpdateCommunityPostStatus(ctx context.Context, postID int64, status string) (*service.CommunityPost, error) {
	return r.UpdateCommunityPostModeration(ctx, postID, status, nil)
}

func (r *bizDecipherRepository) UpdateCommunityPostModeration(ctx context.Context, postID int64, status string, pinned *bool) (*service.CommunityPost, error) {
	deletedExpr := "deleted_at"
	if status == "deleted" {
		deletedExpr = "NOW()"
	}
	args := []any{postID, status}
	pinnedExpr := "pinned"
	if pinned != nil {
		args = append(args, *pinned)
		pinnedExpr = fmt.Sprintf("$%d", len(args))
	}
	row := r.db.QueryRowContext(ctx, fmt.Sprintf(`
		UPDATE community_posts
		SET status = $2, pinned = %s, deleted_at = %s, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, user_id, '' AS author, kind, title, body, tags, district, channel, private, status, pinned, catches, replies, views, source_type, source_id, scenario, subject_type, subject_id, subject_title, action_type, evidence, trust_signals, created_at, updated_at`, pinnedExpr, deletedExpr), args...)
	post, err := scanCommunityPost(row)
	if err != nil {
		return nil, err
	}
	post.Author = "Decoder"
	return post, nil
}

func (r *bizDecipherRepository) UpdateOwnedCommunityPostStatus(ctx context.Context, postID, userID int64, status string) (*service.CommunityPost, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE community_posts
		SET status = $3, updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL AND status NOT IN ('hidden', 'deleted', 'rejected')
		RETURNING id, user_id, '' AS author, kind, title, body, tags, district, channel, private, status, pinned, catches, replies, views, source_type, source_id, scenario, subject_type, subject_id, subject_title, action_type, evidence, trust_signals, created_at, updated_at`, postID, userID, status)
	post, err := scanCommunityPost(row)
	if err != nil {
		return nil, err
	}
	post.Author = "Decoder"
	comments, err := listCommunityComments(ctx, r.db, post.ID, 5, true)
	if err != nil {
		return nil, err
	}
	post.Comments = comments
	return post, nil
}

func (r *bizDecipherRepository) CreateCommunityPost(ctx context.Context, userID int64, input service.CommunityPostInput) (*service.CommunityPost, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO community_posts (user_id, kind, title, body, tags, private, source_type, source_id, scenario, subject_type, subject_id, subject_title, action_type, evidence, trust_signals, district, channel)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8, $9, $10, $11, $12, $13, $14::jsonb, $15::jsonb, $16, $17)
		RETURNING id, user_id, '' AS author, kind, title, body, tags, district, channel, private, status, pinned, catches, replies, views, source_type, source_id, scenario, subject_type, subject_id, subject_title, action_type, evidence, trust_signals, created_at, updated_at`,
		userID, input.Kind, input.Title, input.Body, string(jsonBytes(input.Tags)), input.Private, input.SourceType, input.SourceID, input.Scenario, input.SubjectType, input.SubjectID, input.SubjectTitle, input.ActionType, string(input.Evidence), string(input.TrustSignals), input.District, input.Channel)
	post, err := scanCommunityPost(row)
	if err != nil {
		return nil, err
	}
	post.Author = "Decoder"
	return post, nil
}

func (r *bizDecipherRepository) GetProfileByHandleOrID(ctx context.Context, target string) (*service.BizProfile, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, sql.ErrNoRows
	}
	if id, err := strconv.ParseInt(target, 10, 64); err == nil && id > 0 {
		row := r.db.QueryRowContext(ctx, `SELECT `+bizProfileColumns+` FROM biz_profiles WHERE user_id = $1`, id)
		return scanBizProfile(row)
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+bizProfileColumns+` FROM biz_profiles WHERE handle = $1`, target)
	return scanBizProfile(row)
}

func (r *bizDecipherRepository) GetZeroCityProfileStats(ctx context.Context, userID int64) (*service.ZeroCityProfileStats, error) {
	stats := &service.ZeroCityProfileStats{}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM zero_city_follows WHERE following_user_id = $1`, userID).Scan(&stats.Followers); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM zero_city_follows WHERE follower_user_id = $1`, userID).Scan(&stats.Following); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM community_posts WHERE user_id = $1 AND private = FALSE AND status NOT IN ('hidden', 'deleted') AND deleted_at IS NULL`, userID).Scan(&stats.CommunityPosts); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM shared_pools WHERE owner_id = $1 AND listed = TRUE AND governance_status NOT IN ('banned', 'suppressed') AND lifecycle_state <> 'archived'`, userID).Scan(&stats.SharedPools); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM checkin_collectible_cards WHERE user_id = $1`, userID).Scan(&stats.CollectibleCards); err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *bizDecipherRepository) GetZeroCityFollowState(ctx context.Context, viewerID, targetID int64) (*service.ZeroCityFollowState, error) {
	state := &service.ZeroCityFollowState{}
	if viewerID > 0 && targetID > 0 && viewerID != targetID {
		if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM zero_city_follows WHERE follower_user_id = $1 AND following_user_id = $2)`, viewerID, targetID).Scan(&state.IsFollowing); err != nil {
			return nil, err
		}
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM zero_city_follows WHERE following_user_id = $1`, targetID).Scan(&state.Followers); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM zero_city_follows WHERE follower_user_id = $1`, targetID).Scan(&state.Following); err != nil {
		return nil, err
	}
	return state, nil
}

func (r *bizDecipherRepository) FollowZeroCityProfileTx(ctx context.Context, followerID, followingID int64) (*service.ZeroCityFollowState, error) {
	if _, err := r.db.ExecContext(ctx, `INSERT INTO zero_city_follows (follower_user_id, following_user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, followerID, followingID); err != nil {
		return nil, err
	}
	return r.GetZeroCityFollowState(ctx, followerID, followingID)
}

func (r *bizDecipherRepository) UnfollowZeroCityProfileTx(ctx context.Context, followerID, followingID int64) (*service.ZeroCityFollowState, error) {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM zero_city_follows WHERE follower_user_id = $1 AND following_user_id = $2`, followerID, followingID); err != nil {
		return nil, err
	}
	return r.GetZeroCityFollowState(ctx, followerID, followingID)
}

func (r *bizDecipherRepository) UpdateProfileBackgroundCard(ctx context.Context, userID int64, input service.ZeroCityProfileBackgroundInput) (*service.BizProfile, error) {
	cardKey := strings.TrimSpace(input.CardKey)
	if cardKey == "" {
		row := r.db.QueryRowContext(ctx, `
			UPDATE biz_profiles
			SET background_card_key = '', background_card_rarity = '', background_card_serial_no = NULL,
			    background_card_edition_no = NULL, background_card_edition_supply = NULL, updated_at = NOW()
			WHERE user_id = $1
			RETURNING `+bizProfileColumns, userID)
		return scanBizProfile(row)
	}
	args := []any{userID, cardKey}
	where := `WHERE c.user_id = $1 AND c.card_key = $2`
	if input.SerialNo != nil && *input.SerialNo > 0 {
		args = append(args, *input.SerialNo)
		where += ` AND c.serial_no = $3`
	}
	row := r.db.QueryRowContext(ctx, `
		WITH selected AS (
			SELECT c.card_key, c.rarity, c.serial_no, c.edition_no, c.edition_supply
			FROM checkin_collectible_cards c
			`+where+`
			ORDER BY c.created_at DESC, c.id DESC
			LIMIT 1
		)
		UPDATE biz_profiles bp
		SET background_card_key = selected.card_key,
		    background_card_rarity = selected.rarity,
		    background_card_serial_no = selected.serial_no,
		    background_card_edition_no = selected.edition_no,
		    background_card_edition_supply = selected.edition_supply,
		    updated_at = NOW()
		FROM selected
		WHERE bp.user_id = $1
		RETURNING `+bizProfileColumns, args...)
	return scanBizProfile(row)
}

func listCommunityComments(ctx context.Context, db *sql.DB, postID int64, limit int, publicOnly bool) ([]service.CommunityComment, error) {
	where := "WHERE c.post_id = $1 AND c.deleted_at IS NULL"
	if publicOnly {
		where += " AND c.status NOT IN ('hidden', 'deleted')"
	} else {
		where += " AND c.status <> 'deleted'"
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`
		SELECT c.id, c.post_id, c.user_id,
		       COALESCE(NULLIF(bp.display_name, ''), split_part(u.email, '@', 1), 'Decoder') AS author,
		       c.body, c.helper_role, c.official, c.status, c.created_at, c.updated_at
		FROM community_comments c
		JOIN users u ON u.id = c.user_id
		LEFT JOIN biz_profiles bp ON bp.user_id = c.user_id
		%s
		ORDER BY c.official DESC, c.created_at ASC
		LIMIT $2`, where), postID, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanCommunityCommentRows(rows)
}

func (r *bizDecipherRepository) ListCommunityComments(ctx context.Context, postID int64, limit int) ([]service.CommunityComment, error) {
	if _, _, err := r.GetCommunityPostLocation(ctx, postID); err != nil {
		return nil, err
	}
	return listCommunityComments(ctx, r.db, postID, limit, true)
}

func (r *bizDecipherRepository) UpdateCommunityCommentStatus(ctx context.Context, commentID int64, status string, official bool) (*service.CommunityComment, error) {
	deletedExpr := "deleted_at"
	if status == "deleted" {
		deletedExpr = "NOW()"
	}
	row := r.db.QueryRowContext(ctx, fmt.Sprintf(`
		UPDATE community_comments
		SET status = $2, official = $3, deleted_at = %s, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, post_id, user_id, '' AS author, body, helper_role, official, status, created_at, updated_at`, deletedExpr), commentID, status, official)
	comment, err := scanCommunityComment(row)
	if err != nil {
		return nil, err
	}
	comment.Author = "Decoder"
	return comment, nil
}

func (r *bizDecipherRepository) CreateCommunityComment(ctx context.Context, postID, userID int64, input service.CommunityCommentInput) (*service.CommunityComment, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var postExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM community_posts WHERE id = $1 AND deleted_at IS NULL AND private = FALSE AND status NOT IN ('hidden', 'deleted', 'rejected'))`, postID).Scan(&postExists); err != nil {
		return nil, err
	}
	if !postExists {
		return nil, sql.ErrNoRows
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO community_comments (post_id, user_id, body, helper_role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, post_id, user_id, '' AS author, body, helper_role, official, status, created_at, updated_at`,
		postID, userID, input.Body, input.HelperRole)
	comment, err := scanCommunityComment(row)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE community_posts SET replies = replies + 1, updated_at = NOW() WHERE id = $1`, postID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	comment.Author = "Decoder"
	return comment, nil
}

func (r *bizDecipherRepository) AcceptCommunityCommentTx(ctx context.Context, postID, commentID, ownerID int64) (*service.CommunityPost, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var postOwnerID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id
		FROM community_posts
		WHERE id = $1 AND deleted_at IS NULL AND status NOT IN ('hidden', 'deleted', 'rejected')
		FOR UPDATE`, postID).Scan(&postOwnerID); err != nil {
		return nil, err
	}
	if postOwnerID != ownerID {
		return nil, sql.ErrNoRows
	}

	var commentExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM community_comments
			WHERE id = $1 AND post_id = $2 AND deleted_at IS NULL AND status <> 'deleted'
		)`, commentID, postID).Scan(&commentExists); err != nil {
		return nil, err
	}
	if !commentExists {
		return nil, sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE community_comments
		SET status = CASE WHEN id = $2 THEN 'accepted' WHEN status = 'accepted' THEN 'visible' ELSE status END,
		    updated_at = NOW()
		WHERE post_id = $1 AND deleted_at IS NULL`, postID, commentID); err != nil {
		return nil, err
	}

	row := tx.QueryRowContext(ctx, `
		UPDATE community_posts
		SET status = CASE WHEN status IN ('open', 'reviewing') THEN 'answered' ELSE status END,
		    updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, user_id, '' AS author, kind, title, body, tags, district, channel, private, status, pinned, catches, replies, views, source_type, source_id, scenario, subject_type, subject_id, subject_title, action_type, evidence, trust_signals, created_at, updated_at`, postID)
	post, err := scanCommunityPost(row)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	post.Author = "Decoder"
	comments, err := listCommunityComments(ctx, r.db, post.ID, 5, true)
	if err != nil {
		return nil, err
	}
	post.Comments = comments
	return post, nil
}

func (r *bizDecipherRepository) GetTokenPowerLeaderboard(ctx context.Context, now time.Time, limit int) (*service.TokenPowerLeaderboard, error) {
	weekStart := startOfTokenPowerWeek(now)
	weekEnd := weekStart.AddDate(0, 0, 7).Add(-time.Nanosecond)
	rewardCap := 200.0
	rows, err := r.db.QueryContext(ctx, `
		WITH ranked AS (
			SELECT
				u.id AS user_id,
				COALESCE(NULLIF(bp.display_name, ''), split_part(u.email, '@', 1), 'Decoder') AS display_name,
				COALESCE(SUM(GREATEST(COALESCE(ul.actual_cost, 0), 0)), 0)::float8 AS effective_spend,
				COUNT(ul.id)::bigint AS request_count,
				COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0)::bigint AS token_count
			FROM usage_logs ul
			JOIN users u ON u.id = ul.user_id
			LEFT JOIN biz_profiles bp ON bp.user_id = u.id
			WHERE ul.created_at >= $1 AND ul.created_at < $2 AND COALESCE(ul.actual_cost, 0) > 0
			GROUP BY u.id, bp.display_name, u.email
		)
		SELECT ROW_NUMBER() OVER (ORDER BY effective_spend DESC, request_count DESC) AS rank,
		       user_id, display_name, effective_spend, request_count, token_count,
		       $3::float8 AS reward_cap,
		       CASE WHEN ROW_NUMBER() OVER (ORDER BY effective_spend DESC, request_count DESC) = 1 THEN LEAST(effective_spend, $3::float8) ELSE 0 END AS reward_amount,
		       'pending_review' AS review_status
		FROM ranked
		WHERE effective_spend > 0
		ORDER BY effective_spend DESC, request_count DESC
		LIMIT $4`, weekStart, weekStart.AddDate(0, 0, 7), rewardCap, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.TokenPowerLeaderboardRow{}
	for rows.Next() {
		var item service.TokenPowerLeaderboardRow
		if err := rows.Scan(&item.Rank, &item.UserID, &item.DisplayName, &item.EffectiveSpend, &item.RequestCount, &item.TokenCount, &item.RewardCap, &item.RewardAmount, &item.ReviewStatus); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &service.TokenPowerLeaderboard{
		WeekStart: weekStart.Format("2006-01-02"),
		WeekEnd:   weekEnd.Format("2006-01-02"),
		RewardCap: rewardCap,
		Rules: []string{
			"Only successful paid usage counts toward Token Power.",
			"Free credits, refunds, abnormal traffic, and suspected farming are excluded during review.",
			"Weekly #1 receives in-site balance rebate up to $200 after manual review.",
		},
		Rows: out,
	}, nil
}

func startOfTokenPowerWeek(t time.Time) time.Time {
	local := t.UTC()
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	return start.AddDate(0, 0, -(weekday - 1))
}

type scanner interface{ Scan(dest ...any) error }

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func scanContributor(s scanner) (*service.BizContributorProfile, error) {
	var p service.BizContributorProfile
	var raw []byte
	if err := s.Scan(&p.ID, &p.UserID, &p.Status, &raw, &p.SettlementMethod, &p.RiskNotes, &p.AdminNotes, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	p.CapacityTypes = scanStringArray(raw)
	return &p, nil
}
func scanOperator(s scanner) (*service.BizOperatorProfile, error) {
	var p service.BizOperatorProfile
	var raw []byte
	if err := s.Scan(&p.ID, &p.UserID, &p.Status, &raw, &p.Rank, &p.CompletedTasks, &p.QualityScore, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	p.Skills = scanStringArray(raw)
	return &p, nil
}
func scanCustomRequest(s scanner) (*service.BizCustomRequest, error) {
	var p service.BizCustomRequest
	var raw []byte
	var deadline sql.NullTime
	if err := s.Scan(&p.ID, &p.UserID, &p.Goal, &p.Context, &p.Budget, &deadline, &raw, &p.Status, &p.CreatedAt); err != nil {
		return nil, err
	}
	if deadline.Valid {
		p.Deadline = &deadline.Time
	}
	p.References = scanStringArray(raw)
	return &p, nil
}
func scanCredit(s scanner) (*service.BizCreditLedgerEntry, error) {
	var e service.BizCreditLedgerEntry
	var createdBy sql.NullInt64
	var posted sql.NullTime
	if err := s.Scan(&e.ID, &e.UserID, &e.SourceType, &e.SourceID, &e.Amount, &e.BalanceAfter, &e.Status, &e.Note, &createdBy, &e.CreatedAt, &posted); err != nil {
		return nil, err
	}
	if createdBy.Valid {
		e.CreatedBy = &createdBy.Int64
	}
	if posted.Valid {
		e.PostedAt = &posted.Time
	}
	return &e, nil
}
func scanCreditRows(rows *sql.Rows) ([]service.BizCreditLedgerEntry, error) {
	var out []service.BizCreditLedgerEntry
	for rows.Next() {
		e, err := scanCredit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func scanCommunityPost(s scanner) (*service.CommunityPost, error) {
	var post service.CommunityPost
	var rawTags []byte
	var rawEvidence []byte
	var rawTrustSignals []byte
	if err := s.Scan(
		&post.ID, &post.UserID, &post.Author, &post.Kind, &post.Title, &post.Body, &rawTags,
		&post.District, &post.Channel, &post.Private, &post.Status, &post.Pinned, &post.Catches, &post.Replies, &post.Views,
		&post.SourceType, &post.SourceID, &post.Scenario, &post.SubjectType, &post.SubjectID, &post.SubjectTitle,
		&post.ActionType, &rawEvidence, &rawTrustSignals, &post.CreatedAt, &post.UpdatedAt,
	); err != nil {
		return nil, err
	}
	post.Tags = scanStringArray(rawTags)
	post.Evidence = json.RawMessage(rawEvidence)
	post.TrustSignals = json.RawMessage(rawTrustSignals)
	if len(post.Evidence) == 0 {
		post.Evidence = json.RawMessage(`[]`)
	}
	if len(post.TrustSignals) == 0 {
		post.TrustSignals = json.RawMessage(`{}`)
	}
	return &post, nil
}

func scanCommunityPostRows(rows *sql.Rows) ([]service.CommunityPost, error) {
	out := []service.CommunityPost{}
	for rows.Next() {
		post, err := scanCommunityPost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *post)
	}
	return out, rows.Err()
}

func scanCommunityComment(s scanner) (*service.CommunityComment, error) {
	var comment service.CommunityComment
	if err := s.Scan(&comment.ID, &comment.PostID, &comment.UserID, &comment.Author, &comment.Body, &comment.HelperRole, &comment.Official, &comment.Status, &comment.CreatedAt, &comment.UpdatedAt); err != nil {
		return nil, err
	}
	return &comment, nil
}

func scanCommunityCommentRows(rows *sql.Rows) ([]service.CommunityComment, error) {
	out := []service.CommunityComment{}
	for rows.Next() {
		comment, err := scanCommunityComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *comment)
	}
	return out, rows.Err()
}

func scanSharedPoolBalanceLedger(s scanner) (*service.SharedPoolBalanceLedgerEntry, error) {
	var e service.SharedPoolBalanceLedgerEntry
	var poolID sql.NullInt64
	var posted sql.NullTime
	if err := s.Scan(&e.ID, &e.UserID, &poolID, &e.SourceType, &e.SourceID, &e.Amount, &e.BalanceAfter, &e.Status, &e.Note, &e.CreatedAt, &posted); err != nil {
		return nil, err
	}
	if poolID.Valid {
		e.PoolID = &poolID.Int64
	}
	if posted.Valid {
		e.PostedAt = &posted.Time
	}
	e.AssetType = "balance"
	return &e, nil
}

func scanSharedPoolBalanceLedgerRows(rows *sql.Rows) ([]service.SharedPoolBalanceLedgerEntry, error) {
	out := []service.SharedPoolBalanceLedgerEntry{}
	for rows.Next() {
		e, err := scanSharedPoolBalanceLedger(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}
func clampLimit(v int) int {
	if v <= 0 {
		return 50
	}
	if v > 200 {
		return 200
	}
	return v
}
func firstOr(v []string, fallback string) string {
	if len(v) == 0 || v[0] == "" {
		return fallback
	}
	return v[0]
}

func inviteRewardSourceID(rewardKind string, inviteeUserID, directInviterID int64) string {
	switch rewardKind {
	case "indirect":
		return fmt.Sprintf("indirect:invitee:%d", inviteeUserID)
	case "milestone":
		// Milestone uses inviteeUserID as the milestone count threshold
		return fmt.Sprintf("milestone:count:%d", inviteeUserID)
	default:
		// Keep backward compatibility with the first deployed direct reward format.
		return fmt.Sprintf("invitee:%d", inviteeUserID)
	}
}

func inviteRewardSourceIDPrefix(rewardKind string) string {
	switch rewardKind {
	case "indirect":
		return "indirect:invitee:%"
	case "milestone":
		return "milestone:count:%"
	default:
		return "invitee:%"
	}
}

// HasInviteReward checks whether the inviter has already received an invite
// reward for the given invitee. Reads user_affiliates.invite_reward_count and
// cross-checks with credit_ledger for extra safety.
func (r *bizDecipherRepository) HasInviteReward(ctx context.Context, inviterID, inviteeUserID int64, rewardKind string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM credit_ledger WHERE user_id = $1 AND source_type = 'invite_reward' AND source_id = $2`,
		inviterID, inviteRewardSourceID(rewardKind, inviteeUserID, 0),
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *bizDecipherRepository) CountInviteRewards(ctx context.Context, inviterID int64, rewardKind string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM credit_ledger WHERE user_id = $1 AND source_type = 'invite_reward' AND source_id LIKE $2`,
		inviterID, inviteRewardSourceIDPrefix(rewardKind),
	).Scan(&count)
	return count, err
}

// GrantInviteRewardTx atomically grants an invite point reward to the inviter:
// 1. Verifies inviter has a user_affiliates record
// 2. Updates users.credit_balance because invite rewards are non-withdrawable points
// 3. Inserts a credit_ledger entry with source_type='invite_reward' for idempotency/history
// 4. Increments user_affiliates.invite_reward_count
// Returns sql.ErrNoRows if the inviter has no affiliate profile (should not happen
// if inviter_id was resolved from user_affiliates).
func (r *bizDecipherRepository) GrantInviteRewardTx(ctx context.Context, input service.InviteRewardInput) (*service.BizCreditLedgerEntry, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Verify inviter exists in user_affiliates
	var affExists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_affiliates WHERE user_id = $1)`,
		input.InviterID,
	).Scan(&affExists); err != nil {
		return nil, err
	}
	if !affExists {
		return nil, sql.ErrNoRows
	}

	// 2. Double-check no duplicate (within the transaction)
	sourceID := inviteRewardSourceID(input.RewardKind, input.InviteeUserID, input.DirectInviterID)
	var existingCount int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM credit_ledger WHERE user_id = $1 AND source_type = 'invite_reward' AND source_id = $2`,
		input.InviterID, sourceID,
	).Scan(&existingCount); err != nil {
		return nil, err
	}
	if existingCount > 0 {
		return nil, sql.ErrNoRows // Already granted
	}

	// 3. Update non-withdrawable credit balance and get new value
	var balance float64
	if err := tx.QueryRowContext(ctx,
		`UPDATE users SET credit_balance = credit_balance + $1, updated_at = NOW() WHERE id = $2 RETURNING credit_balance`,
		input.Amount, input.InviterID,
	).Scan(&balance); err != nil {
		return nil, err
	}

	// 4. Insert ledger entry
	note := fmt.Sprintf("Invite reward: user %d invited user %d", input.InviterID, input.InviteeUserID)
	switch input.RewardKind {
	case "indirect":
		note = fmt.Sprintf("Invite growth reward: user %d earned from direct inviter %d inviting user %d", input.InviterID, input.DirectInviterID, input.InviteeUserID)
	case "milestone":
		note = fmt.Sprintf("Invite milestone bonus: user %d reached %d invites", input.InviterID, input.InviteeUserID)
	}
	row := tx.QueryRowContext(ctx,
		`INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at)
		 VALUES ($1, 'invite_reward', $2, $3, $4, 'posted', $5, NOW())
		 RETURNING id, user_id, source_type, source_id, amount, balance_after, status, note, created_by, created_at, posted_at`,
		input.InviterID, sourceID, input.Amount, balance, note,
	)
	entry, err := scanCredit(row)
	if err != nil {
		return nil, err
	}

	// 5. Increment reward counter (skip for milestone - it's a one-time bonus)
	if input.RewardKind != "milestone" {
		counterColumn := "invite_reward_count"
		if input.RewardKind == "indirect" {
			counterColumn = "invite_indirect_reward_count"
		}
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`UPDATE user_affiliates SET %s = %s + 1, updated_at = NOW() WHERE user_id = $1`, counterColumn, counterColumn),
			input.InviterID,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return entry, nil
}

// ---------------------------------------------------------------------------
// Promo campaigns
// ---------------------------------------------------------------------------

func scanPromoCampaign(s scanner) (*service.PromoCampaign, error) {
	var p service.PromoCampaign
	if err := s.Scan(
		&p.ID, &p.Name, &p.Type, &p.Title, &p.Description, &p.Enabled,
		&p.CreditAmount, &p.StartAt, &p.EndAt, &p.Target, &p.AutoHide,
		&p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &p, nil
}

const promoCampaignColumns = `id, name, type, title, description, enabled, credit_amount, start_at, end_at, target, auto_hide, created_at, updated_at`

// GetActivePromoCampaign returns the single enabled campaign whose [start_at, end_at]
// window contains NOW(). If multiple match, the most recently created one wins.
// Returns (nil, nil) when no campaign is active.
func (r *bizDecipherRepository) GetActivePromoCampaign(ctx context.Context) (*service.PromoCampaign, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+promoCampaignColumns+`
		FROM promo_campaigns
		WHERE enabled = TRUE AND start_at <= NOW() AND end_at >= NOW()
		ORDER BY created_at DESC
		LIMIT 1`)
	p, err := scanPromoCampaign(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// HasPromoClaim reports whether the user has already claimed the campaign.
func (r *bizDecipherRepository) HasPromoClaim(ctx context.Context, promoID, userID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM promo_claims WHERE promo_id = $1 AND user_id = $2)`,
		promoID, userID,
	).Scan(&exists)
	return exists, err
}

// ClaimPromoTx atomically validates the campaign is active, that the user has not
// already claimed it, credits users.credit_balance, writes a credit_ledger entry,
// and records the claim. Idempotent per (promo, user): a duplicate returns AlreadyDone.
func (r *bizDecipherRepository) ClaimPromoTx(ctx context.Context, promoID, userID int64) (*service.PromoClaimResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Lock and validate the campaign is currently active.
	var amount float64
	var enabled bool
	if err := tx.QueryRowContext(ctx,
		`SELECT credit_amount, enabled FROM promo_campaigns
		 WHERE id = $1 AND start_at <= NOW() AND end_at >= NOW()
		 FOR UPDATE`,
		promoID,
	).Scan(&amount, &enabled); err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrPromoNotActive
		}
		return nil, err
	}
	if !enabled {
		return nil, service.ErrPromoNotActive
	}

	// 2. Idempotency: if already claimed, return the existing claim.
	var existingAmount float64
	existsErr := tx.QueryRowContext(ctx,
		`SELECT amount FROM promo_claims WHERE promo_id = $1 AND user_id = $2`,
		promoID, userID,
	).Scan(&existingAmount)
	if existsErr == nil {
		// Already claimed; report current credit balance without crediting again.
		var bal float64
		_ = tx.QueryRowContext(ctx, `SELECT credit_balance FROM users WHERE id = $1`, userID).Scan(&bal)
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &service.PromoClaimResult{
			PromoID:      promoID,
			Amount:       existingAmount,
			BalanceAfter: bal,
			AlreadyDone:  true,
		}, nil
	} else if existsErr != sql.ErrNoRows {
		return nil, existsErr
	}

	// 3. Credit the user's non-withdrawable credit balance.
	var balance float64
	if err := tx.QueryRowContext(ctx,
		`UPDATE users SET credit_balance = credit_balance + $1, updated_at = NOW() WHERE id = $2 RETURNING credit_balance`,
		amount, userID,
	).Scan(&balance); err != nil {
		return nil, err
	}

	// 4. Write the ledger entry.
	sourceID := fmt.Sprintf("promo:%d", promoID)
	note := fmt.Sprintf("Promo bonus: campaign %d", promoID)
	var ledgerID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at)
		 VALUES ($1, 'promo_bonus', $2, $3, $4, 'posted', $5, NOW())
		 RETURNING id`,
		userID, sourceID, amount, balance, note,
	).Scan(&ledgerID); err != nil {
		return nil, err
	}

	// 5. Record the claim (unique index guards against races).
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO promo_claims (promo_id, user_id, amount, ledger_id) VALUES ($1, $2, $3, $4)`,
		promoID, userID, amount, ledgerID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.PromoClaimResult{
		PromoID:      promoID,
		Amount:       amount,
		BalanceAfter: balance,
		AlreadyDone:  false,
	}, nil
}

func (r *bizDecipherRepository) ListPromoCampaigns(ctx context.Context, limit int) ([]service.PromoCampaign, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+promoCampaignColumns+` FROM promo_campaigns ORDER BY created_at DESC LIMIT $1`,
		clampLimit(limit),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.PromoCampaign
	for rows.Next() {
		p, err := scanPromoCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) CreatePromoCampaign(ctx context.Context, input service.PromoCampaignInput) (*service.PromoCampaign, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO promo_campaigns (name, type, title, description, enabled, credit_amount, start_at, end_at, target, auto_hide)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING `+promoCampaignColumns,
		input.Name, input.Type, input.Title, input.Description, input.Enabled,
		input.CreditAmount, input.StartAt, input.EndAt, input.Target, input.AutoHide,
	)
	return scanPromoCampaign(row)
}

func (r *bizDecipherRepository) UpdatePromoCampaign(ctx context.Context, id int64, input service.PromoCampaignInput) (*service.PromoCampaign, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE promo_campaigns SET
			name = $2, type = $3, title = $4, description = $5, enabled = $6,
			credit_amount = $7, start_at = $8, end_at = $9, target = $10, auto_hide = $11,
			updated_at = NOW()
		WHERE id = $1
		RETURNING `+promoCampaignColumns,
		id, input.Name, input.Type, input.Title, input.Description, input.Enabled,
		input.CreditAmount, input.StartAt, input.EndAt, input.Target, input.AutoHide,
	)
	return scanPromoCampaign(row)
}

// ---------------------------------------------------------------------------
// Shared pools (Account Square marketplace, read-only phase)
// ---------------------------------------------------------------------------

const sharedPoolColumns = `id, name, description, avatar_url, card_skin_key, card_skin_rarity, status_note, disabled_reason, featured_score, owner_id, owner_label, tier, status, listed, rate_multiplier, max_users, current_users, min_balance_admission, hourly_seat_fee, hourly_min_usage_waiver, today_availability, seven_day_availability, avg_latency_ms, last_probe_at, last_probe_success, last_probe_error_type, last_probe_error_message, consecutive_probe_failures, last_successful_probe_at, upstream_base_url, upstream_api_key <> '', proxy_id, proxy_url, proxy_region, proxy_status, account_concurrency, user_concurrency, account_mode_enabled, oauth_provider, verification_mode, verification_exemption_reason, owner_share_percent, platform_fee_percent, quality_score, rank_weight, reward_score, penalty_score, market_score, governance_status, governance_note, admin_note, complaint_count, like_count, total_calls, successful_calls, failed_calls, lifecycle_state, archived_at, archived_by, archive_reason, source_kind, owner_paused, config_version, native_onboarding_state`

func qualifySharedPoolColumns(alias string) string {
	if alias == "" {
		return sharedPoolColumns
	}
	cols := strings.Split(sharedPoolColumns, ", ")
	for i, col := range cols {
		if strings.Contains(col, " <> ") {
			parts := strings.SplitN(col, " <> ", 2)
			cols[i] = alias + "." + parts[0] + " <> " + parts[1]
		} else {
			cols[i] = alias + "." + col
		}
	}
	return strings.Join(cols, ", ")
}

func scanSharedPool(s scanner) (*service.SharedPool, error) {
	var p service.SharedPool
	var ownerID sql.NullInt64
	var proxyID sql.NullInt64
	var lastProbeAt sql.NullTime
	var lastProbeSuccess sql.NullBool
	var lastSuccessfulProbeAt sql.NullTime
	var archivedAt sql.NullTime
	var archivedBy sql.NullInt64
	var cardSkinKey sql.NullString
	var cardSkinRarity sql.NullString
	if err := s.Scan(
		&p.ID, &p.Name, &p.Description, &p.AvatarURL, &cardSkinKey, &cardSkinRarity, &p.StatusNote, &p.DisabledReason, &p.FeaturedScore,
		&ownerID, &p.OwnerLabel, &p.Tier, &p.Status, &p.Listed,
		&p.RateMultiplier, &p.MaxUsers, &p.CurrentUsers, &p.MinBalanceAdmission,
		&p.HourlySeatFee, &p.HourlyMinUsageWaiver, &p.TodayAvailability, &p.SevenDayAvail, &p.AvgLatencyMs,
		&lastProbeAt, &lastProbeSuccess, &p.LastProbeErrorType, &p.LastProbeErrorMessage, &p.ConsecutiveProbeFailures, &lastSuccessfulProbeAt,
		&p.UpstreamBaseURL, &p.HasUpstreamKey, &proxyID, &p.ProxyURL, &p.ProxyRegion, &p.ProxyStatus,
		&p.AccountConcurrency, &p.UserConcurrency, &p.AccountModeEnabled, &p.OAuthProvider, &p.VerificationMode, &p.VerificationExemptionReason,
		&p.OwnerSharePercent, &p.PlatformFeePercent, &p.QualityScore, &p.RankWeight,
		&p.RewardScore, &p.PenaltyScore, &p.MarketScore, &p.GovernanceStatus, &p.GovernanceNote, &p.AdminNote,
		&p.ComplaintCount, &p.LikeCount, &p.TotalCalls, &p.SuccessfulCalls, &p.FailedCalls,
		&p.LifecycleState, &archivedAt, &archivedBy, &p.ArchiveReason, &p.SourceKind, &p.OwnerPaused, &p.ConfigVersion, &p.NativeOnboardingState,
	); err != nil {
		return nil, err
	}
	if ownerID.Valid {
		p.OwnerID = &ownerID.Int64
	}
	if proxyID.Valid {
		p.ProxyID = &proxyID.Int64
	}
	if lastProbeAt.Valid {
		p.LastProbeAt = &lastProbeAt.Time
	}
	if lastProbeSuccess.Valid {
		p.LastProbeSuccess = &lastProbeSuccess.Bool
	}
	if lastSuccessfulProbeAt.Valid {
		p.LastSuccessfulProbeAt = &lastSuccessfulProbeAt.Time
	}
	if archivedAt.Valid {
		p.ArchivedAt = &archivedAt.Time
	}
	if archivedBy.Valid {
		p.ArchivedBy = &archivedBy.Int64
	}
	if cardSkinKey.Valid {
		p.CardSkinKey = cardSkinKey.String
	}
	if cardSkinRarity.Valid {
		p.CardSkinRarity = cardSkinRarity.String
	}
	p.Models = []string{}
	p.ModelConfigs = []service.SharedPoolModelConfig{}
	p.BillingActivationRequired = p.NativeOnboardingState != "" && p.NativeOnboardingState != service.SharedPoolOnboardingLegacyExisting && p.NativeOnboardingState != service.SharedPoolOnboardingBillingActive
	return &p, nil
}

func applySharedPoolProbeMetadata(p *service.SharedPool, metadata service.SharedPoolProbeMetadata) {
	if p == nil {
		return
	}
	p.LastProbeCheckLevel = strings.TrimSpace(metadata.CheckLevel)
	p.LastProbeGateRequired = metadata.GateRequired
	p.LastProbeGatePassed = metadata.GatePassed
	p.LastProbeFullCheckPassed = metadata.FullCheckPassed
	p.LastProbeFullCheckTotal = metadata.FullCheckTotal
	p.LastProbeFullCheckScore = metadata.FullCheckScore
}

func (r *bizDecipherRepository) hydrateSharedPoolProbeSummaries(ctx context.Context, pools []service.SharedPool) error {
	if len(pools) == 0 {
		return nil
	}
	poolByID := make(map[int64]*service.SharedPool, len(pools))
	ids := make([]int64, 0, len(pools))
	for i := range pools {
		if pools[i].ID <= 0 {
			continue
		}
		poolByID[pools[i].ID] = &pools[i]
		ids = append(ids, pools[i].ID)
	}
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT DISTINCT ON (history.pool_id) history.pool_id, history.metadata
		FROM shared_pool_probe_histories history
		JOIN shared_pools pool ON pool.id = history.pool_id
		WHERE history.pool_id IN (%s)
		  AND history.account_id IS NULL
		  AND history.config_version = pool.config_version
		  AND COALESCE(history.metadata->>'check_level', '') = 'full'
		ORDER BY history.pool_id, history.checked_at DESC, history.id DESC`, strings.Join(placeholders, ", ")), args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var poolID int64
		var raw []byte
		if err := rows.Scan(&poolID, &raw); err != nil {
			return err
		}
		pool := poolByID[poolID]
		if pool == nil || len(raw) == 0 {
			continue
		}
		var metadata service.SharedPoolProbeMetadata
		if err := json.Unmarshal(raw, &metadata); err != nil {
			continue
		}
		applySharedPoolProbeMetadata(pool, metadata)
	}
	return rows.Err()
}

func (r *bizDecipherRepository) hydrateSharedPoolAccountSummaries(ctx context.Context, pools []service.SharedPool) error {
	if len(pools) == 0 {
		return nil
	}
	poolByID := make(map[int64]*service.SharedPool, len(pools))
	ids := make([]int64, 0, len(pools))
	for i := range pools {
		if pools[i].ID <= 0 {
			continue
		}
		poolByID[pools[i].ID] = &pools[i]
		ids = append(ids, pools[i].ID)
	}
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}
	inClause := strings.Join(placeholders, ", ")
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		WITH account_summary AS (
			SELECT
				spa.pool_id,
				COUNT(*)::int AS total_accounts,
				COUNT(*) FILTER (WHERE ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> '')))::int AS configured_accounts,
				COUNT(*) FILTER (
					WHERE spa.status IN ('active', 'limited', 'testing')
						AND spa.schedulable = TRUE
						AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE))
						AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> ''))
						AND (spa.gate_required = FALSE OR spa.gate_passed = TRUE)
				)::int AS schedulable_accounts,
				COUNT(*) FILTER (WHERE spa.gate_required = TRUE AND spa.gate_passed = FALSE)::int AS gate_blocked_accounts,
				COUNT(*) FILTER (WHERE spa.status IN ('disabled', 'offline', 'expired') OR spa.schedulable = FALSE OR (spa.auto_pause_on_expired = TRUE AND spa.expires_at <= NOW()))::int AS disabled_accounts,
				COALESCE(SUM(CASE WHEN spa.status IN ('active', 'limited', 'testing') AND spa.schedulable = TRUE AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE)) AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> '')) AND (spa.gate_required = FALSE OR spa.gate_passed = TRUE) THEN GREATEST(spa.account_concurrency, 0) ELSE 0 END), 0)::int AS total_account_concurrency,
				COALESCE(SUM(CASE WHEN spa.status IN ('active', 'limited', 'testing') AND spa.schedulable = TRUE AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE)) AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> '')) AND (spa.gate_required = FALSE OR spa.gate_passed = TRUE) THEN GREATEST(spa.user_concurrency, 0) ELSE 0 END), 0)::int AS total_user_concurrency,
				COALESCE(SUM(CASE WHEN spa.status IN ('active', 'limited', 'testing') AND spa.schedulable = TRUE AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE)) AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> '')) AND (spa.gate_required = FALSE OR spa.gate_passed = TRUE) THEN GREATEST(spa.rpm_limit, 0) ELSE 0 END), 0)::int AS total_rpm_limit,
				COALESCE(AVG(spa.full_check_score) FILTER (WHERE spa.full_check_total > 0), 0)::double precision AS average_full_check_score,
				COUNT(*) FILTER (WHERE spa.full_check_total > 0 AND spa.gate_passed = TRUE)::int AS full_check_passed_accounts,
				COUNT(*) FILTER (WHERE spa.full_check_total > 0)::int AS full_check_total_accounts,
				COALESCE(SUM(spa.total_calls), 0)::bigint AS total_calls,
				COALESCE(SUM(spa.successful_calls), 0)::bigint AS successful_calls,
				COALESCE(SUM(spa.failed_calls), 0)::bigint AS failed_calls,
				CASE WHEN COALESCE(SUM(spa.total_calls), 0) > 0 THEN COALESCE(SUM(spa.successful_calls), 0)::double precision * 100 / SUM(spa.total_calls) ELSE 0 END AS success_rate,
				MAX(spa.last_probe_at) AS last_probe_at
			FROM shared_pool_accounts spa
			WHERE spa.pool_id IN (%s) AND spa.deleted_at IS NULL
			GROUP BY spa.pool_id
		), model_summary AS (
			SELECT
				spa.pool_id,
				COALESCE(jsonb_agg(DISTINCT cfg.model_name) FILTER (WHERE COALESCE(cfg.model_open, TRUE) = TRUE AND COALESCE(NULLIF(TRIM(cfg.model_name), ''), '') <> ''), '[]'::jsonb) AS model_coverage
			FROM shared_pool_accounts spa
			LEFT JOIN LATERAL jsonb_to_recordset(spa.model_configs) AS cfg(model_name text, model_open boolean) ON TRUE
			WHERE spa.pool_id IN (%s)
				AND spa.deleted_at IS NULL
				AND spa.status IN ('active', 'limited', 'testing')
				AND spa.schedulable = TRUE
				AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE))
				AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> ''))
				AND (spa.gate_required = FALSE OR spa.gate_passed = TRUE)
			GROUP BY spa.pool_id
		)
		SELECT
			a.pool_id,
			a.total_accounts,
			a.configured_accounts,
			a.schedulable_accounts,
			a.gate_blocked_accounts,
			a.disabled_accounts,
			a.total_account_concurrency,
			a.total_user_concurrency,
			a.total_rpm_limit,
			a.average_full_check_score,
			a.full_check_passed_accounts,
			a.full_check_total_accounts,
			a.total_calls,
			a.successful_calls,
			a.failed_calls,
			a.success_rate,
			COALESCE(m.model_coverage, '[]'::jsonb),
			a.last_probe_at
		FROM account_summary a
		LEFT JOIN model_summary m ON m.pool_id = a.pool_id`, inClause, inClause), args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var poolID int64
		var summary service.SharedPoolAccountSummary
		var modelCoverageRaw []byte
		var lastProbeAt sql.NullTime
		if err := rows.Scan(
			&poolID,
			&summary.TotalAccounts,
			&summary.ConfiguredAccounts,
			&summary.SchedulableAccounts,
			&summary.GateBlockedAccounts,
			&summary.DisabledAccounts,
			&summary.TotalAccountConcurrency,
			&summary.TotalUserConcurrency,
			&summary.TotalRPMLimit,
			&summary.AverageFullCheckScore,
			&summary.FullCheckPassedAccounts,
			&summary.FullCheckTotalAccounts,
			&summary.TotalCalls,
			&summary.SuccessfulCalls,
			&summary.FailedCalls,
			&summary.SuccessRate,
			&modelCoverageRaw,
			&lastProbeAt,
		); err != nil {
			return err
		}
		summary.ModelCoverage = scanStringArray(modelCoverageRaw)
		if lastProbeAt.Valid {
			summary.LastProbeAt = &lastProbeAt.Time
		}
		if pool := poolByID[poolID]; pool != nil {
			pool.AccountSummary = summary
		}
	}
	return rows.Err()
}

func (r *bizDecipherRepository) hydrateSharedPoolOwnerCardAssets(ctx context.Context, pools []service.SharedPool) error {
	if len(pools) == 0 {
		return nil
	}
	ownerIDs := make([]int64, 0, len(pools))
	seen := map[int64]struct{}{}
	for i := range pools {
		if pools[i].OwnerID == nil || *pools[i].OwnerID <= 0 {
			continue
		}
		ownerID := *pools[i].OwnerID
		if _, ok := seen[ownerID]; ok {
			continue
		}
		seen[ownerID] = struct{}{}
		ownerIDs = append(ownerIDs, ownerID)
	}
	if len(ownerIDs) == 0 {
		return nil
	}

	rows, err := r.db.QueryContext(ctx, `
		WITH owner_ids AS (
			SELECT unnest($1::bigint[]) AS owner_id
		), ranked_cards AS (
			SELECT
				c.user_id,
				c.card_key,
				c.rarity,
				c.serial_no,
				c.edition_no,
				c.edition_supply,
				CASE c.rarity
					WHEN 'mythic' THEN 6
					WHEN 'legendary' THEN 5
					WHEN 'epic' THEN 4
					WHEN 'rare' THEN 3
					WHEN 'good' THEN 2
					WHEN 'common' THEN 1
					ELSE 0
				END AS rarity_rank,
				ROW_NUMBER() OVER (
					PARTITION BY c.user_id
					ORDER BY
						CASE c.rarity
							WHEN 'mythic' THEN 6
							WHEN 'legendary' THEN 5
							WHEN 'epic' THEN 4
							WHEN 'rare' THEN 3
							WHEN 'good' THEN 2
							WHEN 'common' THEN 1
							ELSE 0
						END DESC,
						c.created_at DESC,
						c.id DESC
				) AS rn
			FROM checkin_collectible_cards c
			JOIN owner_ids oi ON oi.owner_id = c.user_id
		), card_counts AS (
			SELECT user_id, COUNT(*)::bigint AS collectible_count, MAX(rarity_rank) AS highest_rank
			FROM ranked_cards
			GROUP BY user_id
		), background_cards AS (
			SELECT
				bp.user_id,
				NULLIF(TRIM(bp.background_card_key), '') AS card_key,
				NULLIF(TRIM(bp.background_card_rarity), '') AS rarity,
				bp.background_card_serial_no,
				bp.background_card_edition_no,
				bp.background_card_edition_supply,
				NULLIF(TRIM(bp.display_name), '') AS display_name
			FROM biz_profiles bp
			JOIN owner_ids oi ON oi.owner_id = bp.user_id
		)
		SELECT
			oi.owner_id,
			COALESCE(cc.collectible_count, 0),
			CASE COALESCE(cc.highest_rank, 0)
				WHEN 6 THEN 'mythic'
				WHEN 5 THEN 'legendary'
				WHEN 4 THEN 'epic'
				WHEN 3 THEN 'rare'
				WHEN 2 THEN 'good'
				WHEN 1 THEN 'common'
				ELSE ''
			END AS highest_rarity,
			COALESCE(bg.card_key, rc.card_key, '') AS featured_card_key,
			COALESCE(bg.rarity, rc.rarity, '') AS featured_card_rarity,
			COALESCE(bg.background_card_serial_no, rc.serial_no) AS featured_card_serial_no,
			COALESCE(bg.background_card_edition_no, rc.edition_no) AS featured_card_edition_no,
			COALESCE(bg.background_card_edition_supply, rc.edition_supply) AS featured_card_supply,
			(bg.card_key IS NOT NULL) AS profile_background_ready,
			bg.display_name
		FROM owner_ids oi
		LEFT JOIN card_counts cc ON cc.user_id = oi.owner_id
		LEFT JOIN ranked_cards rc ON rc.user_id = oi.owner_id AND rc.rn = 1
		LEFT JOIN background_cards bg ON bg.user_id = oi.owner_id`, pq.Array(ownerIDs))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	assetByOwner := map[int64]service.SharedPoolOwnerCardAsset{}
	displayNameByOwner := map[int64]string{}
	for rows.Next() {
		var ownerID int64
		var asset service.SharedPoolOwnerCardAsset
		var featuredSerialNo sql.NullInt64
		var featuredEditionNo sql.NullInt64
		var featuredSupply sql.NullInt64
		var displayName sql.NullString
		if err := rows.Scan(
			&ownerID,
			&asset.CollectibleCount,
			&asset.HighestRarity,
			&asset.FeaturedCardKey,
			&asset.FeaturedCardRarity,
			&featuredSerialNo,
			&featuredEditionNo,
			&featuredSupply,
			&asset.ProfileBackgroundReady,
			&displayName,
		); err != nil {
			return err
		}
		if featuredSerialNo.Valid {
			asset.FeaturedCardSerialNo = &featuredSerialNo.Int64
		}
		if featuredEditionNo.Valid {
			asset.FeaturedCardEditionNo = &featuredEditionNo.Int64
		}
		if featuredSupply.Valid {
			asset.FeaturedCardSupply = &featuredSupply.Int64
		}
		assetByOwner[ownerID] = asset
		if displayName.Valid {
			displayNameByOwner[ownerID] = displayName.String
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range pools {
		if pools[i].OwnerID == nil {
			continue
		}
		if asset, ok := assetByOwner[*pools[i].OwnerID]; ok {
			pools[i].OwnerCardAsset = asset
		}
		// Prefer the owner's public profile name over the stored snapshot label
		// so renamed users are not shown as an email or stale username.
		if name, ok := displayNameByOwner[*pools[i].OwnerID]; ok && name != "" {
			pools[i].OwnerLabel = name
		}
	}
	return nil
}

func scanModelCatalogEntry(s scanner) (*service.ModelCatalogEntry, error) {
	var m service.ModelCatalogEntry
	var tagsRaw, aliasesRaw, modalitiesInRaw, modalitiesOutRaw []byte
	if err := s.Scan(
		&m.ID, &m.Provider, &m.ModelName, &m.DisplayName, &m.Family, &m.TierLabel,
		&tagsRaw, &aliasesRaw, &m.DefaultRateMultiplier, &m.DefaultRankWeight,
		&m.Mainstream, &m.Enabled, &m.SortOrder,
		&modalitiesInRaw, &modalitiesOutRaw, &m.AdapterKind, &m.ContextWindow,
		&m.Orchestrator, &m.RuntimeRole,
	); err != nil {
		return nil, err
	}
	m.CapabilityTags = scanStringArray(tagsRaw)
	m.Aliases = scanStringArray(aliasesRaw)
	m.ModalitiesIn = scanStringArray(modalitiesInRaw)
	m.ModalitiesOut = scanStringArray(modalitiesOutRaw)
	return &m, nil
}

const modelCatalogColumns = `id, provider, model_name, display_name, family, tier_label,
	capability_tags, aliases, default_rate_multiplier, default_rank_weight,
	mainstream, enabled, sort_order, modalities_in, modalities_out,
	adapter_kind, context_window, orchestrator, runtime_role`

func (r *bizDecipherRepository) ListModelCatalog(ctx context.Context) ([]service.ModelCatalogEntry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+modelCatalogColumns+` FROM model_catalog WHERE enabled = TRUE ORDER BY mainstream DESC, sort_order, provider, model_name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.ModelCatalogEntry{}
	for rows.Next() {
		m, err := scanModelCatalogEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateModelCatalogProfile writes an admin capability matrix edit. Only the
// fields present in the update are changed; the row is returned in full so the
// caller can verify the stored profile without a second read.
func (r *bizDecipherRepository) UpdateModelCatalogProfile(ctx context.Context, id int64, input service.ModelCatalogProfileUpdate) (*service.ModelCatalogEntry, error) {
	var modalitiesIn, modalitiesOut *string
	if len(input.ModalitiesIn) > 0 {
		encoded, err := json.Marshal(input.ModalitiesIn)
		if err != nil {
			return nil, err
		}
		value := string(encoded)
		modalitiesIn = &value
	}
	if len(input.ModalitiesOut) > 0 {
		encoded, err := json.Marshal(input.ModalitiesOut)
		if err != nil {
			return nil, err
		}
		value := string(encoded)
		modalitiesOut = &value
	}
	var adapterKind, runtimeRole *string
	if strings.TrimSpace(input.AdapterKind) != "" {
		value := input.AdapterKind
		adapterKind = &value
	}
	if strings.TrimSpace(input.RuntimeRole) != "" {
		value := input.RuntimeRole
		runtimeRole = &value
	}
	row := r.db.QueryRowContext(ctx, `
		UPDATE model_catalog SET
			modalities_in  = COALESCE($2::jsonb, modalities_in),
			modalities_out = COALESCE($3::jsonb, modalities_out),
			adapter_kind   = COALESCE($4::varchar, adapter_kind),
			context_window = COALESCE($5::int, context_window),
			orchestrator   = COALESCE($6::boolean, orchestrator),
			runtime_role   = COALESCE($7::varchar, runtime_role),
			updated_at     = NOW()
		WHERE id = $1
		RETURNING `+modelCatalogColumns,
		id, modalitiesIn, modalitiesOut, adapterKind, input.ContextWindow, input.Orchestrator, runtimeRole)
	entry, err := scanModelCatalogEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func normalizeSharedPoolPriceKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func matchSharedPoolPriceSnapshot(priceMap map[string]*service.SharedPoolPriceSnapshot, provider, model string, aliases []string) *service.SharedPoolPriceSnapshot {
	if len(priceMap) == 0 {
		return nil
	}
	candidates := make([]string, 0, len(aliases)+1)
	candidates = append(candidates, model)
	candidates = append(candidates, aliases...)

	provider = normalizeSharedPoolPriceKey(provider)
	keys := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = normalizeSharedPoolPriceKey(candidate)
		if candidate == "" {
			continue
		}
		if provider != "" {
			// A known provider is a hard pricing boundary. Falling back to a
			// provider-less model key here could silently apply another vendor's
			// price when two vendors expose the same model or alias.
			keys = append(keys, provider+":"+candidate)
			continue
		}
		keys = append(keys, candidate)
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if snapshot, ok := priceMap[key]; ok && snapshot != nil {
			copy := *snapshot
			return &copy
		}
	}
	return nil
}

func (r *bizDecipherRepository) hydrateSharedPoolSettlementRules(ctx context.Context, pools []service.SharedPool) error {
	if len(pools) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(pools))
	args := make([]any, 0, len(pools)+1)
	args = append(args, time.Now().UTC())
	for i := range pools {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+2))
		args = append(args, pools[i].ID)
	}
	currentRows, err := r.db.QueryContext(ctx, `SELECT DISTINCT ON (pool_id)
		pool_id, hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, effective_from
		FROM shared_pool_settlement_rule_versions
		WHERE effective_from <= $1 AND pool_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY pool_id, effective_from DESC, id DESC`, args...)
	if err != nil {
		return err
	}
	idx := make(map[int64]int, len(pools))
	for i := range pools {
		idx[pools[i].ID] = i
	}
	for currentRows.Next() {
		var poolID int64
		var hourlyFee, waiverMin, platformFee float64
		var effectiveFrom time.Time
		if err := currentRows.Scan(&poolID, &hourlyFee, &waiverMin, &platformFee, &effectiveFrom); err != nil {
			_ = currentRows.Close()
			return err
		}
		if i, ok := idx[poolID]; ok {
			pools[i].HourlySeatFee = hourlyFee
			pools[i].HourlyMinUsageWaiver = waiverMin
			pools[i].PlatformFeePercent = platformFee
			pools[i].OwnerSharePercent = 100 - platformFee
			effective := effectiveFrom
			pools[i].SettlementRuleEffective = &effective
		}
	}
	if err := currentRows.Err(); err != nil {
		_ = currentRows.Close()
		return err
	}
	if err := currentRows.Close(); err != nil {
		return err
	}

	pendingRows, err := r.db.QueryContext(ctx, `SELECT DISTINCT ON (pool_id)
		pool_id, hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, effective_from
		FROM shared_pool_settlement_rule_versions
		WHERE effective_from > $1 AND pool_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY pool_id, effective_from ASC, id DESC`, args...)
	if err != nil {
		return err
	}
	defer func() { _ = pendingRows.Close() }()
	for pendingRows.Next() {
		var poolID int64
		var hourlyFee, waiverMin, platformFee float64
		var effectiveFrom time.Time
		if err := pendingRows.Scan(&poolID, &hourlyFee, &waiverMin, &platformFee, &effectiveFrom); err != nil {
			return err
		}
		if i, ok := idx[poolID]; ok {
			pendingHourlyFee := hourlyFee
			pendingWaiverMin := waiverMin
			pendingPlatformFee := platformFee
			pendingEffective := effectiveFrom
			pools[i].PendingHourlySeatFee = &pendingHourlyFee
			pools[i].PendingHourlyMinUsageWaiver = &pendingWaiverMin
			pools[i].PendingPlatformFeePercent = &pendingPlatformFee
			pools[i].PendingRuleEffective = &pendingEffective
		}
	}
	return pendingRows.Err()
}

// loadPoolModels attaches the supported model names to each pool in one query.
func (r *bizDecipherRepository) loadPoolModels(ctx context.Context, pools []service.SharedPool) error {
	if len(pools) == 0 {
		return nil
	}
	if err := r.hydrateSharedPoolSettlementRules(ctx, pools); err != nil {
		return err
	}
	idx := make(map[int64]int, len(pools))
	placeholders := make([]string, 0, len(pools))
	args := make([]any, 0, len(pools))
	for i := range pools {
		idx[pools[i].ID] = i
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, pools[i].ID)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT spm.pool_id, spm.provider, spm.model_name, spm.upstream_model_name, COALESCE(NULLIF(spm.display_name, ''), COALESCE(NULLIF(mc.display_name, ''), spm.model_name)),
			COALESCE(mc.model_name, '') AS canonical_model_name,
			spm.model_aliases, spm.rate_multiplier, spm.rank_weight, spm.five_hour_protection_percent,
			spm.seven_day_protection_percent, spm.daily_protection_percent, spm.min_balance_admission, spm.hourly_seat_fee,
			sp.hourly_min_usage_waiver, spm.max_concurrency, spm.model_open, spm.tags
		 FROM shared_pool_models spm
		 JOIN shared_pools sp ON sp.id = spm.pool_id
		 LEFT JOIN LATERAL (
			SELECT catalog.model_name, catalog.display_name, catalog.aliases
			FROM model_catalog catalog
			WHERE catalog.enabled = TRUE
			  AND LOWER(catalog.provider) = LOWER(spm.provider)
			  AND (
				LOWER(catalog.model_name) = LOWER(spm.model_name)
				OR EXISTS (
					SELECT 1
					FROM jsonb_array_elements_text(
						CASE WHEN jsonb_typeof(catalog.aliases) = 'array' THEN catalog.aliases ELSE '[]'::jsonb END
					) AS official_alias(value)
					WHERE LOWER(official_alias.value) = LOWER(spm.model_name)
				)
			  )
			ORDER BY CASE WHEN LOWER(catalog.model_name) = LOWER(spm.model_name) THEN 0 ELSE 1 END,
			         catalog.sort_order, catalog.id
			LIMIT 1
		 ) mc ON TRUE
		 WHERE spm.enabled = TRUE AND spm.pool_id IN (`+strings.Join(placeholders, ",")+`)
		 ORDER BY spm.pool_id, spm.sort_order`,
		args...,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var poolID int64
		var cfg service.SharedPoolModelConfig
		var aliasesRaw, tagsRaw []byte
		if err := rows.Scan(
			&poolID, &cfg.Provider, &cfg.ModelName, &cfg.UpstreamModelName, &cfg.DisplayName, &cfg.CanonicalModelName, &aliasesRaw,
			&cfg.RateMultiplier, &cfg.RankWeight, &cfg.FiveHourProtectionPercent,
			&cfg.SevenDayProtectionPercent, &cfg.DailyProtectionPercent, &cfg.MinBalanceAdmission, &cfg.HourlySeatFee,
			&cfg.HourlyMinUsageWaiver, &cfg.MaxConcurrency, &cfg.ModelOpen, &tagsRaw,
		); err != nil {
			return err
		}
		cfg.Aliases = scanStringArray(aliasesRaw)
		cfg.Tags = scanStringArray(tagsRaw)
		if i, ok := idx[poolID]; ok {
			cfg.HourlySeatFee = pools[i].HourlySeatFee
			cfg.HourlyMinUsageWaiver = pools[i].HourlyMinUsageWaiver
			if cfg.ModelOpen {
				pools[i].Models = append(pools[i].Models, cfg.ModelName)
			}
			pools[i].ModelConfigs = append(pools[i].ModelConfigs, cfg)
		}
	}
	return rows.Err()
}

func (r *bizDecipherRepository) ListSharedPools(ctx context.Context, filter service.SharedPoolFilter) ([]service.SharedPool, error) {
	// Build a parameterized WHERE clause. Public marketplace rows must be listed
	// and backed by either legacy pool-level credentials or at least one configured
	// pool account. Pure catalog rows without real upstream resources stay hidden.
	clauses := []string{"TRUE"}
	view := strings.TrimSpace(strings.ToLower(filter.View))
	if view == "" {
		view = "public"
	}
	if !filter.IncludeUnlisted {
		clauses = append(clauses,
			"sp.lifecycle_state <> 'archived'",
			`(
				(NULLIF(TRIM(sp.upstream_base_url), '') IS NOT NULL AND NULLIF(TRIM(sp.upstream_api_key), '') IS NOT NULL)
				OR EXISTS (
					SELECT 1 FROM shared_pool_accounts spa_visible
					WHERE spa_visible.pool_id = sp.id
						AND spa_visible.deleted_at IS NULL
						AND spa_visible.status IN ('active', 'limited', 'testing')
						AND spa_visible.schedulable = TRUE
						AND (spa_visible.expires_at IS NULL OR spa_visible.expires_at > NOW() OR (spa_visible.auth_type <> 'oauth' AND spa_visible.auto_pause_on_expired = FALSE))
						AND ((spa_visible.auth_type IN ('apikey', 'api_key') AND NULLIF(TRIM(spa_visible.upstream_base_url), '') IS NOT NULL AND NULLIF(TRIM(spa_visible.upstream_api_key), '') IS NOT NULL) OR (spa_visible.auth_type = 'oauth' AND NULLIF(TRIM(spa_visible.credentials_encrypted), '') IS NOT NULL))
					)
					OR (sp.native_onboarding_state = 'billing_active' AND EXISTS (
						SELECT 1 FROM shared_pool_supply_dispositions native_disposition
						JOIN accounts native_account ON native_account.id = native_disposition.canonical_account_id
						WHERE native_disposition.pool_id = sp.id
							AND native_disposition.source_kind = 'pool_account'
							AND native_disposition.disposition = 'mapped'
							AND native_account.deleted_at IS NULL
							AND native_account.status = 'active'
							AND native_account.schedulable = TRUE
					))
				)`,
			"sp.listed = TRUE",
			"sp.governance_status NOT IN ('banned', 'suppressed')",
		)
		switch view {
		case "observation":
			clauses = append(clauses, `(sp.governance_status = 'watch' OR sp.status IN ('limited', 'offline', 'maintenance') OR COALESCE(sp.consecutive_probe_failures, 0) >= 3)`)
		default:
			clauses = append(clauses, `sp.status = 'healthy'`, `COALESCE(sp.consecutive_probe_failures, 0) < 3`, `COALESCE(sp.governance_status, 'normal') NOT IN ('watch', 'suppressed')`)
		}
	} else {
		switch strings.TrimSpace(strings.ToLower(filter.Lifecycle)) {
		case "current":
			clauses = append(clauses,
				"sp.lifecycle_state <> 'archived'",
				"sp.listed = TRUE",
				"sp.status NOT IN ('offline', 'maintenance')",
				"COALESCE(sp.governance_status, 'normal') NOT IN ('watch', 'suppressed', 'banned')",
				"COALESCE(sp.consecutive_probe_failures, 0) < 3",
			)
		case "attention":
			clauses = append(clauses,
				"sp.lifecycle_state <> 'archived'",
				`(
					sp.listed = FALSE
					OR sp.status IN ('offline', 'maintenance')
					OR COALESCE(sp.governance_status, 'normal') IN ('watch', 'suppressed', 'banned')
					OR COALESCE(sp.consecutive_probe_failures, 0) >= 3
				)`,
			)
		case "archived":
			clauses = append(clauses, "sp.lifecycle_state = 'archived'")
		case "", "all":
			// Admin explicitly requested the complete lifecycle inventory.
		default:
			return nil, fmt.Errorf("invalid shared pool lifecycle filter %q", filter.Lifecycle)
		}
	}
	args := []any{}
	argN := 1

	if kw := strings.TrimSpace(filter.Keyword); kw != "" {
		clauses = append(clauses, fmt.Sprintf(`(
			sp.name ILIKE '%%' || $%d || '%%'
			OR sp.owner_label ILIKE '%%' || $%d || '%%'
			OR EXISTS (SELECT 1 FROM shared_pool_models m WHERE m.pool_id = sp.id AND m.model_name ILIKE '%%' || $%d || '%%')
		)`, argN, argN, argN))
		args = append(args, kw)
		argN++
	}
	if model := strings.TrimSpace(filter.Model); model != "" && model != "all" {
		clauses = append(clauses, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM shared_pool_models m
			LEFT JOIN model_catalog mc ON mc.provider = m.provider AND mc.model_name = m.model_name
			WHERE m.pool_id = sp.id AND m.enabled = TRUE AND (
				m.model_name = $%d
				OR m.model_name ILIKE $%d
				OR m.model_aliases ? $%d
				OR mc.aliases ? $%d
			)
		)`, argN, argN, argN, argN))
		args = append(args, model)
		argN++
	}
	if st := strings.TrimSpace(filter.Status); st != "" && st != "all" {
		clauses = append(clauses, fmt.Sprintf("sp.status = $%d", argN))
		args = append(args, st)
		argN++
	}
	if filter.MinAvailability > 0 {
		clauses = append(clauses, fmt.Sprintf("sp.today_availability >= $%d", argN))
		args = append(args, filter.MinAvailability)
		argN++
	}

	orderBy := "sp.market_score DESC, sp.featured_score DESC, sp.rank_weight DESC, sp.quality_score DESC, sp.today_availability DESC, sp.rate_multiplier ASC"
	switch filter.SortBy {
	case "availability":
		orderBy = "sp.today_availability DESC, sp.market_score DESC"
	case "rate":
		orderBy = "sp.rate_multiplier ASC, sp.market_score DESC"
	case "latency":
		orderBy = "sp.avg_latency_ms ASC, sp.market_score DESC"
	case "users":
		orderBy = "sp.current_users DESC, sp.market_score DESC"
	case "newest":
		orderBy = "sp.created_at DESC"
	case "score", "weight", "recommended":
		orderBy = "sp.market_score DESC, sp.featured_score DESC, sp.rank_weight DESC, sp.quality_score DESC, sp.today_availability DESC, sp.rate_multiplier ASC"
	}

	query := `SELECT ` + qualifySharedPoolColumns("sp") + ` FROM shared_pools sp WHERE ` +
		strings.Join(clauses, " AND ") +
		` ORDER BY ` + orderBy + fmt.Sprintf(" LIMIT $%d", argN)
	args = append(args, clampLimit(filter.Limit))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.SharedPool
	for rows.Next() {
		p, err := scanSharedPool(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.loadPoolModels(ctx, out); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolProbeSummaries(ctx, out); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolAccountSummaries(ctx, out); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolOwnerCardAssets(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *bizDecipherRepository) GetSharedPool(ctx context.Context, id int64) (*service.SharedPool, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+sharedPoolColumns+` FROM shared_pools sp WHERE sp.id = $1 AND sp.listed = TRUE AND sp.governance_status NOT IN ('banned', 'suppressed')
			AND sp.lifecycle_state <> 'archived'
			AND (
				(NULLIF(TRIM(sp.upstream_base_url), '') IS NOT NULL AND NULLIF(TRIM(sp.upstream_api_key), '') IS NOT NULL)
				OR EXISTS (
					SELECT 1 FROM shared_pool_accounts spa_visible
					WHERE spa_visible.pool_id = sp.id
						AND spa_visible.deleted_at IS NULL
						AND spa_visible.status IN ('active', 'limited', 'testing')
						AND spa_visible.schedulable = TRUE
						AND (spa_visible.expires_at IS NULL OR spa_visible.expires_at > NOW() OR (spa_visible.auth_type <> 'oauth' AND spa_visible.auto_pause_on_expired = FALSE))
						AND ((spa_visible.auth_type IN ('apikey', 'api_key') AND NULLIF(TRIM(spa_visible.upstream_base_url), '') IS NOT NULL AND NULLIF(TRIM(spa_visible.upstream_api_key), '') IS NOT NULL) OR (spa_visible.auth_type = 'oauth' AND NULLIF(TRIM(spa_visible.credentials_encrypted), '') IS NOT NULL))
					)
					OR (sp.native_onboarding_state = 'billing_active' AND EXISTS (
						SELECT 1 FROM shared_pool_supply_dispositions native_disposition
						JOIN accounts native_account ON native_account.id = native_disposition.canonical_account_id
						WHERE native_disposition.pool_id = sp.id
							AND native_disposition.source_kind = 'pool_account'
							AND native_disposition.disposition = 'mapped'
							AND native_account.deleted_at IS NULL
							AND native_account.status = 'active'
							AND native_account.schedulable = TRUE
					))
				)`,
		id,
	)
	p, err := scanSharedPool(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	items := []service.SharedPool{*p}
	if err := r.loadPoolModels(ctx, items); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolProbeSummaries(ctx, items); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolAccountSummaries(ctx, items); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolOwnerCardAssets(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (r *bizDecipherRepository) ListPublicSharedPoolsByOwner(ctx context.Context, ownerID int64, limit int) ([]service.SharedPool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+sharedPoolColumns+` FROM shared_pools sp WHERE sp.owner_id = $1 AND sp.listed = TRUE AND sp.governance_status NOT IN ('banned', 'suppressed')
		AND sp.lifecycle_state <> 'archived'
		AND (
			(NULLIF(TRIM(sp.upstream_base_url), '') IS NOT NULL AND NULLIF(TRIM(sp.upstream_api_key), '') IS NOT NULL)
			OR EXISTS (
				SELECT 1 FROM shared_pool_accounts spa_visible
				WHERE spa_visible.pool_id = sp.id
					AND spa_visible.deleted_at IS NULL
					AND spa_visible.status IN ('active', 'limited', 'testing')
					AND spa_visible.schedulable = TRUE
					AND (spa_visible.expires_at IS NULL OR spa_visible.expires_at > NOW() OR (spa_visible.auth_type <> 'oauth' AND spa_visible.auto_pause_on_expired = FALSE))
					AND ((spa_visible.auth_type IN ('apikey', 'api_key') AND NULLIF(TRIM(spa_visible.upstream_base_url), '') IS NOT NULL AND NULLIF(TRIM(spa_visible.upstream_api_key), '') IS NOT NULL) OR (spa_visible.auth_type = 'oauth' AND NULLIF(TRIM(spa_visible.credentials_encrypted), '') IS NOT NULL))
				)
				OR (sp.native_onboarding_state = 'billing_active' AND EXISTS (
					SELECT 1 FROM shared_pool_supply_dispositions native_disposition
					JOIN accounts native_account ON native_account.id = native_disposition.canonical_account_id
					WHERE native_disposition.pool_id = sp.id
						AND native_disposition.source_kind = 'pool_account'
						AND native_disposition.disposition = 'mapped'
						AND native_account.deleted_at IS NULL
						AND native_account.status = 'active'
						AND native_account.schedulable = TRUE
				))
			)
			ORDER BY sp.market_score DESC, sp.featured_score DESC, sp.created_at DESC LIMIT $2`, ownerID, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.SharedPool{}
	for rows.Next() {
		p, err := scanSharedPool(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.loadPoolModels(ctx, out); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolProbeSummaries(ctx, out); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolAccountSummaries(ctx, out); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolOwnerCardAssets(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *bizDecipherRepository) UpdateSharedPoolGovernanceTx(ctx context.Context, poolID int64, adminUserID int64, input service.SharedPoolGovernanceInput) (*service.SharedPool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	beforeRow := tx.QueryRowContext(ctx, `SELECT `+sharedPoolColumns+` FROM shared_pools WHERE id = $1 FOR UPDATE`, poolID)
	beforePool, err := scanSharedPool(beforeRow)
	if err != nil {
		return nil, err
	}
	if beforePool.NativeOnboardingState != "" && beforePool.NativeOnboardingState != service.SharedPoolOnboardingLegacyExisting && beforePool.NativeOnboardingState != service.SharedPoolOnboardingBillingActive {
		if input.Listed != nil && *input.Listed {
			return nil, service.ErrBillingActivationRequired
		}
		if input.Status != nil && (*input.Status == "healthy" || *input.Status == "limited") {
			return nil, service.ErrBillingActivationRequired
		}
	}

	sets := []string{"updated_at = NOW()"}
	args := []any{}
	addSet := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if input.PlatformFeePercent != nil {
		platformFeePercent := math.Max(0, math.Min(100, *input.PlatformFeePercent))
		addSet("platform_fee_percent", platformFeePercent)
		addSet("owner_share_percent", 100-platformFeePercent)
	}
	if input.FeaturedScore != nil {
		addSet("featured_score", *input.FeaturedScore)
	}
	if input.RewardScore != nil {
		addSet("reward_score", *input.RewardScore)
	}
	if input.PenaltyScore != nil {
		addSet("penalty_score", *input.PenaltyScore)
	}
	if input.GovernanceStatus != nil {
		addSet("governance_status", *input.GovernanceStatus)
	}
	if input.GovernanceNote != nil {
		addSet("governance_note", *input.GovernanceNote)
	}
	if input.AdminNote != nil {
		addSet("admin_note", *input.AdminNote)
	}
	if input.Listed != nil && (beforePool.NativeOnboardingState == "" || beforePool.NativeOnboardingState == service.SharedPoolOnboardingLegacyExisting || beforePool.NativeOnboardingState == service.SharedPoolOnboardingBillingActive) {
		addSet("listed", *input.Listed)
	}
	if input.Status != nil && (beforePool.NativeOnboardingState == "" || beforePool.NativeOnboardingState == service.SharedPoolOnboardingLegacyExisting || beforePool.NativeOnboardingState == service.SharedPoolOnboardingBillingActive) {
		addSet("status", *input.Status)
	}
	args = append(args, poolID)
	row := tx.QueryRowContext(ctx, `UPDATE shared_pools SET `+strings.Join(sets, ", ")+fmt.Sprintf(" WHERE id = $%d RETURNING ", len(args))+sharedPoolColumns, args...)
	pool, err := scanSharedPool(row)
	if err != nil {
		return nil, err
	}
	if pool.LifecycleState != "archived" {
		nextLifecycle := "suspended"
		if pool.Listed && (pool.Status == "healthy" || pool.Status == "limited") {
			nextLifecycle = "operating"
		} else if pool.LifecycleState == "draft" {
			nextLifecycle = "draft"
		}
		if nextLifecycle != pool.LifecycleState {
			pool, err = scanSharedPool(tx.QueryRowContext(ctx,
				`UPDATE shared_pools SET lifecycle_state = $2, updated_at = NOW()
				 WHERE id = $1 AND lifecycle_state <> 'archived'
				 RETURNING `+sharedPoolColumns,
				poolID, nextLifecycle,
			))
			if err != nil {
				return nil, err
			}
		}
	}
	if beforePool.HourlySeatFee != pool.HourlySeatFee || beforePool.HourlyMinUsageWaiver != pool.HourlyMinUsageWaiver || beforePool.PlatformFeePercent != pool.PlatformFeePercent {
		if err := insertSharedPoolSettlementRuleVersionTx(ctx, tx, poolID, pool.HourlySeatFee, pool.HourlyMinUsageWaiver, pool.PlatformFeePercent, "admin_governance", adminUserID, nextSharedPoolSettlementHour(time.Now())); err != nil {
			return nil, err
		}
	}
	reason := ""
	if input.AdminNote != nil {
		reason = strings.TrimSpace(*input.AdminNote)
	}
	if reason == "" && input.GovernanceNote != nil {
		reason = strings.TrimSpace(*input.GovernanceNote)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_governance_logs (pool_id, admin_user_id, action, before_value, after_value, reason)
		VALUES ($1, NULLIF($2, 0), 'update_governance', $3::jsonb, $4::jsonb, $5)`, poolID, adminUserID, string(jsonBytes(beforePool)), string(jsonBytes(pool)), reason)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	items := []service.SharedPool{*pool}
	if err := r.loadPoolModels(ctx, items); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolAccountSummaries(ctx, items); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolOwnerCardAssets(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

func scanSharedPoolProbeHistory(s scanner) (*service.SharedPoolProbeHistory, error) {
	var h service.SharedPoolProbeHistory
	var ownerID sql.NullInt64
	var accountID sql.NullInt64
	var metadataRaw []byte
	if err := s.Scan(&h.ID, &h.PoolID, &accountID, &ownerID, &h.ModelName, &h.UpstreamModelName, &h.ProbeType, &h.Success, &h.HTTPStatus, &h.ErrorType, &h.ErrorMessage, &h.LatencyMs, &h.CheckedAt, &h.CreatedAt, &metadataRaw); err != nil {
		return nil, err
	}
	if accountID.Valid {
		h.AccountID = &accountID.Int64
	}
	if ownerID.Valid {
		h.OwnerID = &ownerID.Int64
	}
	if len(metadataRaw) > 0 {
		_ = json.Unmarshal(metadataRaw, &h.Metadata)
	}
	return &h, nil
}

func (r *bizDecipherRepository) ListSharedPoolProbeHistories(ctx context.Context, poolID, accountID, ownerID int64, limit int) ([]service.SharedPoolProbeHistory, error) {
	if accountID == 0 {
		if history, native, err := r.nativePoolAccountHistory(ctx, poolID, ownerID, limit); native || err != nil {
			return history, err
		}
	}
	query := `SELECT h.id, h.pool_id, h.account_id, h.owner_id, h.model_name, h.upstream_model_name, h.probe_type, h.success, h.http_status, h.error_type, h.error_message, h.latency_ms, h.checked_at, h.created_at, h.metadata
		FROM shared_pool_probe_histories h WHERE h.pool_id = $1`
	args := []any{poolID}
	if accountID > 0 {
		args = append(args, accountID)
		query += fmt.Sprintf(` AND h.account_id = $%d`, len(args))
	}
	if ownerID > 0 {
		args = append(args, ownerID)
		query += fmt.Sprintf(` AND EXISTS (
			SELECT 1 FROM shared_pools sp
			WHERE sp.id = h.pool_id AND sp.owner_id = $%d
		)`, len(args))
	}
	query += fmt.Sprintf(` ORDER BY h.checked_at DESC, h.id DESC LIMIT $%d`, len(args)+1)
	args = append(args, clampLimit(limit))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.SharedPoolProbeHistory{}
	for rows.Next() {
		h, err := scanSharedPoolProbeHistory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *h)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) GetLatestSharedPoolFullCheckHistory(ctx context.Context, poolID int64) (*service.SharedPoolProbeHistory, error) {
	if r == nil || r.db == nil || poolID <= 0 {
		return nil, errors.New("invalid shared pool full-check identity")
	}
	history, err := scanSharedPoolProbeHistory(r.db.QueryRowContext(ctx, `
		SELECT h.id, h.pool_id, h.account_id, h.owner_id, h.model_name,
		       h.upstream_model_name, h.probe_type, h.success, h.http_status,
		       h.error_type, h.error_message, h.latency_ms, h.checked_at,
		       h.created_at, h.metadata
		FROM shared_pool_probe_histories h
		JOIN shared_pools sp ON sp.id = h.pool_id
		WHERE h.pool_id = $1
		  AND h.config_version = sp.config_version
		  AND COALESCE(h.metadata->>'check_level', '') = 'full'
		ORDER BY h.checked_at DESC, h.id DESC
		LIMIT 1`, poolID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return history, err
}

func (r *bizDecipherRepository) RecordSharedPoolProbeHistory(ctx context.Context, input service.SharedPoolProbeHistoryInput) error {
	if input.PoolID <= 0 {
		return nil
	}
	probeType := strings.TrimSpace(input.ProbeType)
	switch probeType {
	case "manual", "publish_gate", "scheduled", "scheduled_full":
	default:
		probeType = "manual"
	}
	checkedAt := input.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	var ownerID any
	if input.OwnerID > 0 {
		ownerID = input.OwnerID
	}
	var accountID any
	if input.AccountID > 0 {
		accountID = input.AccountID
	}
	metadata := jsonBytes(input.Metadata)
	_, err := r.db.ExecContext(ctx, `INSERT INTO shared_pool_probe_histories
		(pool_id, account_id, owner_id, model_name, upstream_model_name, probe_type, success, http_status, error_type, error_message, latency_ms, checked_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb)`,
		input.PoolID, accountID, ownerID, strings.TrimSpace(input.ModelName), strings.TrimSpace(input.UpstreamModelName), probeType,
		input.Success, input.HTTPStatus, strings.TrimSpace(input.ErrorType), strings.TrimSpace(input.ErrorMessage), input.LatencyMs, checkedAt, string(metadata))
	return err
}

func (r *bizDecipherRepository) ApplySharedPoolAccountProbeResult(ctx context.Context, input service.SharedPoolProbeHistoryInput) error {
	if input.PoolID <= 0 || input.AccountID <= 0 {
		return nil
	}
	checkedAt := input.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	statusExpr := `CASE WHEN status IN ('testing', 'limited', 'offline') THEN 'active' ELSE status END`
	if !input.Success {
		statusExpr = `status`
		if input.HTTPStatus == 401 || input.HTTPStatus == 402 || input.HTTPStatus == 403 {
			statusExpr = `'offline'`
		} else if input.HTTPStatus == 429 || strings.TrimSpace(input.ErrorType) == "rate_limited" {
			statusExpr = `'limited'`
		} else if input.HTTPStatus == 0 || input.HTTPStatus >= 500 || strings.TrimSpace(input.ErrorType) == "timeout" || strings.TrimSpace(input.ErrorType) == "network_error" {
			statusExpr = `CASE WHEN status = 'active' THEN 'testing' ELSE status END`
		}
	}
	fullProbe := strings.EqualFold(strings.TrimSpace(input.Metadata.CheckLevel), "full") || input.Metadata.GateRequired
	_, err := r.db.ExecContext(ctx, `UPDATE shared_pool_accounts SET
		last_probe_at = $4,
		last_probe_success = $5,
		last_probe_error_type = CASE WHEN $5 THEN '' ELSE $6 END,
		last_probe_error_message = CASE WHEN $5 THEN '' ELSE $7 END,
		last_successful_probe_at = CASE WHEN $5 THEN $4 ELSE last_successful_probe_at END,
		gate_required = CASE WHEN $8 THEN $9 ELSE gate_required END,
		gate_passed = CASE WHEN $8 THEN $10 ELSE gate_passed END,
		full_check_score = CASE WHEN $8 THEN $11 ELSE full_check_score END,
		full_check_passed = CASE WHEN $8 THEN $12 ELSE full_check_passed END,
		full_check_total = CASE WHEN $8 THEN $13 ELSE full_check_total END,
		status = `+statusExpr+`,
		updated_at = NOW()
	WHERE pool_id = $1 AND id = $2 AND owner_id = $3 AND deleted_at IS NULL`,
		input.PoolID, input.AccountID, input.OwnerID, checkedAt, input.Success, strings.TrimSpace(input.ErrorType), strings.TrimSpace(input.ErrorMessage),
		fullProbe, input.Metadata.GateRequired, input.Metadata.GatePassed, input.Metadata.FullCheckScore, input.Metadata.FullCheckPassed, input.Metadata.FullCheckTotal)
	if err != nil {
		return err
	}
	// Account-mode pools never hit ApplySharedPoolProbeResult; roll account probe
	// histories into pool-level availability / latency so market + daily rewards work.
	if rollErr := r.rollupSharedPoolMetricsFromAccountProbes(ctx, input.PoolID); rollErr != nil {
		return rollErr
	}
	return nil
}

// Shared-pool observation / auto-delist thresholds for consecutive probe failures.
// 3+ → observation (governance watch, still listed); 9+ → auto delist.
const (
	sharedPoolObservationFailureThreshold = 3
	sharedPoolDelistFailureThreshold      = 9
	sharedPoolFullCheckPassingScore       = 70.0
)

const (
	sharedPoolObservationNote = "连续探测失败，已进入观察期；请尽快修复上游后重新检测。连续失败达到 9 次将自动下架。"
	sharedPoolDelistNote      = "连续探测失败过多，已自动下架。修复上游后请重新触发满血检测再上架。"
	sharedPoolObservationGov  = "auto_observation"
	sharedPoolDelistGov       = "auto_delist_consecutive_failures"
)

// rollupSharedPoolMetricsFromAccountProbes recomputes pool-level probe fields from
// shared_pool_probe_histories that belong to accounts of this pool.
// - today_availability: success rate over the last 24h
// - seven_day_availability: success rate over the last 7d
// - avg_latency_ms: mean latency of successful probes in the last 24h
// - last_probe_*: pool health derived from each eligible account's latest probe
// - consecutive_probe_failures: minimum trailing failures across all eligible accounts
// - any healthy account: keep the pool healthy/listed and clear auto observation markers
// - all accounts fail ≥3 rounds: governance_status=watch, status=limited, keep listed
// - all accounts fail ≥9 rounds: listed=false, status=offline
func (r *bizDecipherRepository) rollupSharedPoolMetricsFromAccountProbes(ctx context.Context, poolID int64) error {
	return r.rollupSharedPoolMetricsFromAccountProbesWithExecutor(ctx, r.db, poolID)
}

func (r *bizDecipherRepository) rollupSharedPoolMetricsFromAccountProbesWithExecutor(ctx context.Context, executor sharedPoolProbeQueryExecutor, poolID int64) error {
	if poolID <= 0 {
		return nil
	}

	// Snapshot before update so we can write governance logs for transitions.
	var beforeStatus, beforeGov, beforeNote string
	var beforeListed bool
	var beforeFailures int
	_ = executor.QueryRowContext(ctx, `
		SELECT status, listed, COALESCE(governance_status, 'normal'), COALESCE(status_note, ''), COALESCE(consecutive_probe_failures, 0)
		FROM shared_pools WHERE id = $1`, poolID).
		Scan(&beforeStatus, &beforeListed, &beforeGov, &beforeNote, &beforeFailures)

	const q = `
WITH eligible_accounts AS (
	SELECT spa.id
	FROM shared_pool_accounts spa
	WHERE spa.pool_id = $1
		AND spa.deleted_at IS NULL
		AND spa.status IN ('active', 'limited', 'testing', 'offline')
		AND spa.schedulable = TRUE
		AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE))
		AND (
			(spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '')
			OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> '')
		)
),
account_histories AS (
	SELECT h.*
	FROM shared_pool_probe_histories h
	JOIN eligible_accounts ea ON ea.id = h.account_id
	WHERE h.pool_id = $1 AND h.checked_at <= statement_timestamp()
),
windowed AS (
	SELECT
		COUNT(*) FILTER (WHERE checked_at >= statement_timestamp() - INTERVAL '24 hours')::int AS total_24h,
		COUNT(*) FILTER (WHERE checked_at >= statement_timestamp() - INTERVAL '24 hours' AND success = TRUE)::int AS success_24h,
		COUNT(*) FILTER (WHERE checked_at >= statement_timestamp() - INTERVAL '7 days')::int AS total_7d,
		COUNT(*) FILTER (WHERE checked_at >= statement_timestamp() - INTERVAL '7 days' AND success = TRUE)::int AS success_7d,
		COALESCE(AVG(latency_ms) FILTER (
			WHERE checked_at >= statement_timestamp() - INTERVAL '24 hours'
			  AND success = TRUE
			  AND latency_ms > 0
		), 0)::double precision AS avg_latency_24h
	FROM account_histories
),
latest AS (
	SELECT
		success,
		error_type,
		error_message,
		checked_at,
		probe_type,
		COALESCE((metadata->>'gate_passed')::boolean, FALSE) AS gate_passed,
		COALESCE((metadata->>'full_check_score')::double precision, 0) AS full_check_score
	FROM account_histories
	ORDER BY checked_at DESC, id DESC
	LIMIT 1
),
latest_per_account AS (
	SELECT DISTINCT ON (account_id)
		account_id,
		success,
		checked_at
	FROM account_histories
	ORDER BY account_id, checked_at DESC, id DESC
),
account_trail_fail AS (
	SELECT account_id, COUNT(*) FILTER (WHERE success = FALSE AND success_seen = 0)::int AS consecutive_failures
	FROM (
		SELECT
			account_id,
			success,
			SUM(CASE WHEN success THEN 1 ELSE 0 END) OVER (
				PARTITION BY account_id
				ORDER BY checked_at DESC, id DESC
				ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
			) AS success_seen
		FROM account_histories
	) ranked
	GROUP BY account_id
),
pool_health AS (
	SELECT
		COUNT(ea.id)::int AS eligible_accounts,
		COUNT(lpa.account_id)::int AS probed_accounts,
		COUNT(*) FILTER (WHERE lpa.success = TRUE)::int AS healthy_accounts,
		CASE
			WHEN COUNT(*) FILTER (WHERE lpa.success = TRUE) > 0 THEN 0
			WHEN COUNT(lpa.account_id) < COUNT(ea.id) THEN 0
			ELSE COALESCE(MIN(atf.consecutive_failures), 0)
		END::int AS consecutive_failures
	FROM eligible_accounts ea
	LEFT JOIN latest_per_account lpa ON lpa.account_id = ea.id
	LEFT JOIN account_trail_fail atf ON atf.account_id = ea.id
),
last_ok AS (
	SELECT checked_at
	FROM account_histories
	WHERE success = TRUE
	ORDER BY checked_at DESC, id DESC
	LIMIT 1
)
UPDATE shared_pools sp SET
	today_availability = CASE
		WHEN w.total_24h > 0 THEN LEAST(100, GREATEST(0, ROUND(w.success_24h::numeric * 100 / w.total_24h, 2)))
		ELSE 0
	END,
	seven_day_availability = CASE
		WHEN w.total_7d > 0 THEN LEAST(100, GREATEST(0, ROUND(w.success_7d::numeric * 100 / w.total_7d, 2)))
		ELSE 0
	END,
	avg_latency_ms = CASE
		WHEN w.avg_latency_24h > 0 THEN ROUND(w.avg_latency_24h)::int
		ELSE sp.avg_latency_ms
	END,
	last_probe_at = COALESCE(l.checked_at, sp.last_probe_at),
	last_probe_success = CASE
		WHEN l.checked_at IS NULL THEN sp.last_probe_success
		WHEN ph.healthy_accounts > 0 THEN TRUE
		ELSE FALSE
	END,
	last_probe_error_type = CASE
		WHEN l.checked_at IS NULL THEN sp.last_probe_error_type
		WHEN ph.healthy_accounts > 0 THEN ''
		ELSE COALESCE(l.error_type, '')
	END,
	last_probe_error_message = CASE
		WHEN l.checked_at IS NULL THEN sp.last_probe_error_message
		WHEN ph.healthy_accounts > 0 THEN ''
		ELSE COALESCE(l.error_message, '')
	END,
	last_successful_probe_at = COALESCE(lo.checked_at, sp.last_successful_probe_at),
	consecutive_probe_failures = ph.consecutive_failures,
	status = CASE
		WHEN sp.owner_paused OR sp.status = 'maintenance' THEN sp.status
		WHEN l.checked_at IS NULL THEN sp.status
		WHEN ph.healthy_accounts > 0 AND sp.status IN ('offline', 'limited', 'testing') THEN 'healthy'
		WHEN ph.consecutive_failures >= $3 THEN 'offline'
		WHEN ph.consecutive_failures >= $2 THEN 'limited'
		WHEN ph.healthy_accounts = 0 AND l.error_type = 'rate_limited' THEN 'limited'
		ELSE sp.status
	END,
	listed = CASE
		WHEN sp.owner_paused THEN FALSE
		WHEN l.checked_at IS NULL THEN sp.listed
		WHEN ph.healthy_accounts > 0
			AND l.success
			AND l.probe_type IN ('manual', 'publish_gate', 'scheduled_full')
			AND l.gate_passed = TRUE
			AND l.full_check_score >= $6
			AND COALESCE(sp.lifecycle_state, 'active') NOT IN ('archived', 'suspended')
			AND sp.status <> 'maintenance'
			AND (
				COALESCE(sp.governance_status, 'normal') IN ('normal', 'boosted')
				OR (sp.governance_status = 'watch' AND sp.governance_note IN ($4, $5))
			) THEN TRUE
		WHEN ph.healthy_accounts > 0 THEN sp.listed
		WHEN ph.consecutive_failures >= $3 THEN FALSE
		ELSE sp.listed
	END,
	governance_status = CASE
		WHEN l.checked_at IS NULL THEN sp.governance_status
		WHEN sp.governance_status IN ('banned', 'suppressed') THEN sp.governance_status
		WHEN ph.consecutive_failures >= $2 AND (sp.governance_status IN ('normal', 'boosted') OR (sp.governance_status = 'watch' AND sp.governance_note IN ($4, $5))) THEN 'watch'
		WHEN ph.healthy_accounts > 0 AND sp.governance_status = 'watch' AND sp.governance_note IN ($4, $5) THEN 'normal'
		ELSE sp.governance_status
	END,
	governance_note = CASE
		WHEN l.checked_at IS NULL THEN sp.governance_note
		WHEN sp.governance_status IN ('banned', 'suppressed') THEN sp.governance_note
		WHEN ph.consecutive_failures >= $3 AND (sp.governance_status IN ('normal', 'boosted') OR (sp.governance_status = 'watch' AND sp.governance_note IN ($4, $5))) THEN $5
		WHEN ph.consecutive_failures >= $2 AND (sp.governance_status IN ('normal', 'boosted') OR (sp.governance_status = 'watch' AND sp.governance_note IN ($4, $5))) THEN $4
		WHEN ph.healthy_accounts > 0 AND sp.governance_status = 'watch' AND sp.governance_note IN ($4, $5) THEN ''
		ELSE sp.governance_note
	END,
	status_note = CASE
		WHEN l.checked_at IS NULL THEN sp.status_note
		WHEN ph.consecutive_failures >= $3 THEN $5
		WHEN ph.consecutive_failures >= $2 THEN $4
		WHEN ph.healthy_accounts > 0 AND sp.status_note IN ($4, $5) THEN ''
		ELSE sp.status_note
	END,
	quality_score = CASE
		WHEN w.total_24h > 0 THEN LEAST(100, GREATEST(
			sp.quality_score,
			CASE
				WHEN (w.success_24h::numeric * 100 / w.total_24h) >= 95 THEN 70
				WHEN (w.success_24h::numeric * 100 / w.total_24h) >= 80 THEN 60
				ELSE sp.quality_score
			END
		))
		ELSE sp.quality_score
	END,
	updated_at = NOW()
FROM windowed w
CROSS JOIN pool_health ph
LEFT JOIN latest l ON TRUE
LEFT JOIN last_ok lo ON TRUE
WHERE sp.id = $1 AND sp.native_onboarding_state = 'legacy_existing' AND sp.deleted_at IS NULL`
	_, err := executor.ExecContext(ctx, q, poolID,
		sharedPoolObservationFailureThreshold,
		sharedPoolDelistFailureThreshold,
		sharedPoolObservationNote,
		sharedPoolDelistNote,
		sharedPoolFullCheckPassingScore,
	)
	if err != nil {
		return err
	}

	var afterStatus, afterGov, afterNote string
	var afterListed bool
	var afterFailures int
	if scanErr := executor.QueryRowContext(ctx, `
		SELECT status, listed, COALESCE(governance_status, 'normal'), COALESCE(status_note, ''), COALESCE(consecutive_probe_failures, 0)
		FROM shared_pools WHERE id = $1`, poolID).
		Scan(&afterStatus, &afterListed, &afterGov, &afterNote, &afterFailures); scanErr != nil {
		return nil
	}

	action := ""
	reason := afterNote
	switch {
	case beforeListed && !afterListed:
		action = sharedPoolDelistGov
	case beforeGov != "watch" && afterGov == "watch":
		action = sharedPoolObservationGov
	case beforeGov == "watch" && (beforeNote == sharedPoolObservationNote || beforeNote == sharedPoolDelistNote) && afterGov == "normal" && afterFailures == 0:
		action = "auto_observation_cleared"
		reason = "探测恢复成功，已退出观察期"
	case beforeListed && afterListed && beforeFailures < sharedPoolDelistFailureThreshold && afterFailures >= sharedPoolDelistFailureThreshold:
		action = sharedPoolDelistGov
	}
	if action == "" {
		return nil
	}
	beforePayload := map[string]any{
		"status": beforeStatus, "listed": beforeListed, "governance_status": beforeGov,
		"status_note": beforeNote, "consecutive_probe_failures": beforeFailures,
	}
	afterPayload := map[string]any{
		"status": afterStatus, "listed": afterListed, "governance_status": afterGov,
		"status_note": afterNote, "consecutive_probe_failures": afterFailures,
	}
	_, _ = executor.ExecContext(ctx, `INSERT INTO shared_pool_governance_logs (pool_id, admin_user_id, action, before_value, after_value, reason)
		VALUES ($1, NULL, $2, $3::jsonb, $4::jsonb, $5)`,
		poolID, action, string(jsonBytes(beforePayload)), string(jsonBytes(afterPayload)), reason)
	return nil
}

// isSharedPoolFullProbeType reports whether the probe type constitutes a
// "full check" for auto-governance purposes. Only full-check probes can
// trigger auto-listing (Behavior 1); basic scheduled probes cannot.
func isSharedPoolFullProbeType(probeType string) bool {
	switch strings.ToLower(strings.TrimSpace(probeType)) {
	case "manual", "publish_gate", "scheduled_full":
		return true
	}
	return false
}

func sharedPoolProbeAllowsAutoListing(input service.SharedPoolProbeHistoryInput) bool {
	return input.Success &&
		isSharedPoolFullProbeType(input.ProbeType) &&
		input.Metadata.GatePassed &&
		input.Metadata.FullCheckScore >= sharedPoolFullCheckPassingScore
}

func sharedPoolAutoObservationManaged(governanceStatus, governanceNote string) bool {
	status := strings.ToLower(strings.TrimSpace(governanceStatus))
	if status == "normal" || status == "boosted" {
		return true
	}
	if status != "watch" {
		return false
	}
	note := strings.TrimSpace(governanceNote)
	return note == sharedPoolObservationNote || note == sharedPoolDelistNote
}

func (r *bizDecipherRepository) ApplySharedPoolProbeResult(ctx context.Context, input service.SharedPoolProbeHistoryInput) error {
	if input.PoolID <= 0 {
		return nil
	}
	checkedAt := input.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}

	var beforeStatus, beforeGov, beforeNote string
	var beforeListed bool
	var beforeFailures int
	_ = r.db.QueryRowContext(ctx, `
		SELECT status, listed, COALESCE(governance_status, 'normal'), COALESCE(status_note, ''), COALESCE(consecutive_probe_failures, 0)
		FROM shared_pools WHERE id = $1`, input.PoolID).
		Scan(&beforeStatus, &beforeListed, &beforeGov, &beforeNote, &beforeFailures)

	if input.Success {
		// Behavior 1 — Auto-list on passing full probe:
		// When probe_type is a full check and the model-aware gate passes (70+),
		// set listed = TRUE (no-op if already listed).
		// Also clear auto observation markers when probe recovers.
		shouldAutoList := sharedPoolProbeAllowsAutoListing(input)
		result, err := r.db.ExecContext(ctx, `UPDATE shared_pools SET
			last_probe_at = $2,
			last_probe_success = TRUE,
			last_probe_error_type = '',
			last_probe_error_message = '',
			consecutive_probe_failures = 0,
			last_successful_probe_at = $2,
			avg_latency_ms = CASE WHEN $3 > 0 THEN $3 ELSE avg_latency_ms END,
			status = CASE WHEN NOT owner_paused AND status IN ('offline', 'limited', 'testing') THEN 'healthy' ELSE status END,
			`+sharedPoolProbeAvailabilityAssignments+`
			quality_score = LEAST(100, GREATEST(quality_score, 70)),
			listed = CASE
				WHEN owner_paused THEN FALSE
				WHEN $4 AND status <> 'maintenance' AND COALESCE(lifecycle_state, 'active') NOT IN ('archived', 'suspended')
					AND (governance_status IN ('normal', 'boosted') OR
						(governance_status = 'watch' AND governance_note IN ($5, $6))) THEN TRUE
				ELSE listed END,
			governance_status = CASE
				WHEN governance_status = 'watch' AND governance_note IN ($5, $6) THEN 'normal'
				ELSE governance_status
			END,
			governance_note = CASE
				WHEN governance_status = 'watch' AND governance_note IN ($5, $6) THEN ''
				ELSE governance_note
			END,
			status_note = CASE
				WHEN status_note IN ($5, $6) THEN ''
				ELSE status_note
			END,
			updated_at = NOW()
		WHERE id = $1 AND native_onboarding_state = 'legacy_existing' AND deleted_at IS NULL`, input.PoolID, checkedAt, input.LatencyMs, shouldAutoList,
			sharedPoolObservationNote, sharedPoolDelistNote)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected == 0 {
			return err
		}
		if beforeGov == "watch" && (beforeNote == sharedPoolObservationNote || beforeNote == sharedPoolDelistNote) {
			var afterStatus, afterGov, afterNote string
			var afterListed bool
			var afterFailures int
			if err := r.db.QueryRowContext(ctx, `
				SELECT status, listed, COALESCE(governance_status, 'normal'), COALESCE(status_note, ''), COALESCE(consecutive_probe_failures, 0)
				FROM shared_pools WHERE id = $1`, input.PoolID).
				Scan(&afterStatus, &afterListed, &afterGov, &afterNote, &afterFailures); err != nil {
				return err
			}
			if afterGov != "normal" {
				return nil
			}
			afterPayload := map[string]any{
				"status": afterStatus, "listed": afterListed,
				"governance_status": afterGov, "status_note": afterNote, "consecutive_probe_failures": afterFailures,
			}
			beforePayload := map[string]any{
				"status": beforeStatus, "listed": beforeListed, "governance_status": beforeGov,
				"status_note": beforeNote, "consecutive_probe_failures": beforeFailures,
			}
			_, _ = r.db.ExecContext(ctx, `INSERT INTO shared_pool_governance_logs (pool_id, admin_user_id, action, before_value, after_value, reason)
				VALUES ($1, NULL, 'auto_observation_cleared', $2::jsonb, $3::jsonb, $4)`,
				input.PoolID, string(jsonBytes(beforePayload)), string(jsonBytes(afterPayload)), "探测恢复成功，已退出观察期")
		}
		return nil
	}

	// Behavior 2 — Observation then auto-delist on consecutive failures:
	// ≥3 failures → observation (watch, limited, keep listed)
	// ≥9 failures → auto delist (offline, listed=false)
	nextFailures := beforeFailures + 1
	statusExpr := `status`
	if input.HTTPStatus == 429 || strings.TrimSpace(input.ErrorType) == "rate_limited" {
		statusExpr = `'limited'`
	} else if nextFailures >= sharedPoolDelistFailureThreshold {
		statusExpr = `'offline'`
	} else if nextFailures >= sharedPoolObservationFailureThreshold {
		statusExpr = `'limited'`
	} else if input.HTTPStatus == 0 || input.HTTPStatus >= 500 || strings.TrimSpace(input.ErrorType) == "timeout" || strings.TrimSpace(input.ErrorType) == "network_error" {
		statusExpr = `'limited'`
	}

	result, err := r.db.ExecContext(ctx, `UPDATE shared_pools SET
		last_probe_at = $2,
		last_probe_success = FALSE,
		last_probe_error_type = $3,
		last_probe_error_message = $4,
		consecutive_probe_failures = consecutive_probe_failures + 1,
		status = CASE WHEN owner_paused OR status = 'maintenance' THEN status ELSE `+statusExpr+` END,
		`+sharedPoolProbeAvailabilityAssignments+`
		quality_score = GREATEST(quality_score - 2, 0),
		listed = CASE
			WHEN owner_paused OR consecutive_probe_failures + 1 >= $5 THEN FALSE
			ELSE listed
		END,
		governance_status = CASE
			WHEN governance_status IN ('banned', 'suppressed') THEN governance_status
			WHEN consecutive_probe_failures + 1 >= $6
				AND (governance_status IN ('normal', 'boosted') OR (governance_status = 'watch' AND governance_note IN ($7, $8))) THEN 'watch'
			ELSE governance_status
		END,
		governance_note = CASE
			WHEN governance_status IN ('banned', 'suppressed') THEN governance_note
			WHEN consecutive_probe_failures + 1 >= $5
				AND (governance_status IN ('normal', 'boosted') OR (governance_status = 'watch' AND governance_note IN ($7, $8))) THEN $8
			WHEN consecutive_probe_failures + 1 >= $6
				AND (governance_status IN ('normal', 'boosted') OR (governance_status = 'watch' AND governance_note IN ($7, $8))) THEN $7
			ELSE governance_note
		END,
		status_note = CASE
			WHEN consecutive_probe_failures + 1 >= $5 THEN $8
			WHEN consecutive_probe_failures + 1 >= $6 THEN $7
			ELSE status_note
		END,
		updated_at = NOW()
	WHERE id = $1 AND native_onboarding_state = 'legacy_existing' AND deleted_at IS NULL`,
		input.PoolID, checkedAt, strings.TrimSpace(input.ErrorType), strings.TrimSpace(input.ErrorMessage),
		sharedPoolDelistFailureThreshold, sharedPoolObservationFailureThreshold,
		sharedPoolObservationNote, sharedPoolDelistNote,
	)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected == 0 {
		return err
	}

	action := ""
	reason := ""
	switch {
	case nextFailures >= sharedPoolDelistFailureThreshold && beforeListed:
		action = sharedPoolDelistGov
		reason = sharedPoolDelistNote
	case nextFailures >= sharedPoolObservationFailureThreshold && beforeGov != "watch":
		action = sharedPoolObservationGov
		reason = sharedPoolObservationNote
	}
	if action == "" {
		return nil
	}
	var afterStatus, afterGov, afterNote string
	var afterListed bool
	var afterFailures int
	if err := r.db.QueryRowContext(ctx, `
		SELECT status, listed, COALESCE(governance_status, 'normal'), COALESCE(status_note, ''), COALESCE(consecutive_probe_failures, 0)
		FROM shared_pools WHERE id = $1`, input.PoolID).
		Scan(&afterStatus, &afterListed, &afterGov, &afterNote, &afterFailures); err != nil {
		return err
	}
	if (action == sharedPoolDelistGov && afterListed) || (action == sharedPoolObservationGov && afterGov != "watch") {
		return nil
	}
	beforePayload := map[string]any{
		"status": beforeStatus, "listed": beforeListed, "governance_status": beforeGov,
		"status_note": beforeNote, "consecutive_probe_failures": beforeFailures,
	}
	afterPayload := map[string]any{
		"status": afterStatus, "listed": afterListed, "governance_status": afterGov,
		"status_note": afterNote, "consecutive_probe_failures": afterFailures,
	}
	_, _ = r.db.ExecContext(ctx, `INSERT INTO shared_pool_governance_logs (pool_id, admin_user_id, action, before_value, after_value, reason)
		VALUES ($1, NULL, $2, $3::jsonb, $4::jsonb, $5)`,
		input.PoolID, action, string(jsonBytes(beforePayload)), string(jsonBytes(afterPayload)), reason)
	return nil
}

const sharedPoolProbeCandidateAccountStatuses = "'active', 'limited', 'testing', 'offline'"

func (r *bizDecipherRepository) ListSharedPoolProbeCandidates(ctx context.Context, limit int) ([]service.SharedPoolProbeCandidate, error) {
	query := fmt.Sprintf(`WITH account_candidates AS (
		SELECT
			sp.id AS pool_id,
			spa.id AS account_id,
			COALESCE(sp.owner_id, spa.owner_id, 0) AS owner_id,
			spa.upstream_base_url,
			spa.upstream_api_key,
			COALESCE(NULLIF(account_model.model_name, ''), NULLIF(pool_model.model_name, ''), '') AS probe_model,
			COALESCE(NULLIF(account_model.upstream_model_name, ''), NULLIF(account_model.model_name, ''), NULLIF(pool_model.upstream_model_name, ''), NULLIF(pool_model.model_name, ''), '') AS upstream_model_name,
			spa.proxy_url,
			((spa.gate_required = TRUE AND spa.gate_passed = FALSE) OR spa.full_check_total <= 0) AS full_probe_required,
			COALESCE(spa.last_probe_at, '1970-01-01'::timestamptz) AS last_probe_at,
			sp.market_score,
			0 AS scope_order
		FROM shared_pool_accounts spa
		JOIN shared_pools sp ON sp.id = spa.pool_id AND sp.owner_id = spa.owner_id
		LEFT JOIN LATERAL (
			SELECT model_name, upstream_model_name
			FROM jsonb_to_recordset(spa.model_configs) AS cfg(model_name text, upstream_model_name text, model_open boolean)
			WHERE COALESCE(cfg.model_open, TRUE) = TRUE AND COALESCE(NULLIF(TRIM(cfg.model_name), ''), '') <> ''
			ORDER BY cfg.model_name ASC
			LIMIT 1
		) account_model ON TRUE
		LEFT JOIN LATERAL (
			SELECT model_name, upstream_model_name
			FROM shared_pool_models
			WHERE pool_id = sp.id AND enabled = TRUE AND model_open = TRUE
			ORDER BY sort_order ASC, id ASC
			LIMIT 1
		) pool_model ON TRUE
		WHERE sp.listed = TRUE
			AND sp.lifecycle_state <> 'archived'
			AND sp.governance_status NOT IN ('banned', 'suppressed')
			AND spa.deleted_at IS NULL
			AND spa.status IN (%s)
			AND spa.schedulable = TRUE
			AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE))
			AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> ''))
			AND (spa.last_probe_at IS NULL OR spa.last_probe_at < NOW() - INTERVAL '5 minutes')
	), pool_candidates AS (
		SELECT
			sp.id AS pool_id,
			0::bigint AS account_id,
			COALESCE(sp.owner_id, 0) AS owner_id,
			sp.upstream_base_url,
			sp.upstream_api_key,
			COALESCE(NULLIF(spm.model_name, ''), '') AS probe_model,
			COALESCE(NULLIF(spm.upstream_model_name, ''), NULLIF(spm.model_name, ''), '') AS upstream_model_name,
			sp.proxy_url,
			FALSE AS full_probe_required,
			COALESCE(sp.last_probe_at, '1970-01-01'::timestamptz) AS last_probe_at,
			sp.market_score,
			1 AS scope_order
		FROM shared_pools sp
		LEFT JOIN LATERAL (
			SELECT model_name, upstream_model_name
			FROM shared_pool_models
			WHERE pool_id = sp.id AND enabled = TRUE AND model_open = TRUE
			ORDER BY sort_order ASC, id ASC
			LIMIT 1
		) spm ON TRUE
		WHERE sp.listed = TRUE
			AND sp.lifecycle_state <> 'archived'
			AND sp.governance_status NOT IN ('banned', 'suppressed')
			AND btrim(sp.upstream_base_url) <> ''
			AND btrim(sp.upstream_api_key) <> ''
			AND (sp.last_probe_at IS NULL OR sp.last_probe_at < NOW() - INTERVAL '5 minutes')
			AND NOT EXISTS (
				SELECT 1
				FROM shared_pool_accounts spa
				WHERE spa.pool_id = sp.id
					AND spa.deleted_at IS NULL
					AND spa.schedulable = TRUE
					AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE))
					AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> ''))
			)
	), candidates AS (
		SELECT * FROM account_candidates
		UNION ALL
		SELECT * FROM pool_candidates
	)
	SELECT pool_id, account_id, owner_id, upstream_base_url, upstream_api_key, probe_model, upstream_model_name, proxy_url, full_probe_required
	FROM candidates
	WHERE COALESCE(NULLIF(TRIM(probe_model), ''), NULLIF(TRIM(upstream_model_name), '')) IS NOT NULL
	ORDER BY scope_order ASC, last_probe_at ASC, market_score DESC
	LIMIT $1`, sharedPoolProbeCandidateAccountStatuses)
	rows, err := r.db.QueryContext(ctx, query, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.SharedPoolProbeCandidate{}
	for rows.Next() {
		var item service.SharedPoolProbeCandidate
		if err := rows.Scan(&item.PoolID, &item.AccountID, &item.OwnerID, &item.UpstreamBaseURL, &item.UpstreamAPIKey, &item.ProbeModel, &item.UpstreamModelName, &item.ProxyURL, &item.FullProbeRequired); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) RunSharedPoolProbeAggregation(ctx context.Context, now time.Time, limit int) (*service.SharedPoolProbeAggregationSummary, error) {
	return &service.SharedPoolProbeAggregationSummary{}, nil
}

func scanSharedPoolGovernanceLog(s scanner) (*service.SharedPoolGovernanceLog, error) {
	var log service.SharedPoolGovernanceLog
	var adminUserID sql.NullInt64
	if err := s.Scan(&log.ID, &log.PoolID, &adminUserID, &log.Action, &log.BeforeValue, &log.AfterValue, &log.Reason, &log.CreatedAt); err != nil {
		return nil, err
	}
	if adminUserID.Valid {
		log.AdminUserID = &adminUserID.Int64
	}
	return &log, nil
}

func (r *bizDecipherRepository) ListSharedPoolGovernanceLogs(ctx context.Context, poolID int64, limit int) ([]service.SharedPoolGovernanceLog, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, pool_id, admin_user_id, action, before_value, after_value, reason, created_at
		FROM shared_pool_governance_logs WHERE pool_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2`, poolID, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.SharedPoolGovernanceLog{}
	for rows.Next() {
		log, err := scanSharedPoolGovernanceLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *log)
	}
	return out, rows.Err()
}

func keyPreview(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return key
	}
	return key[:9] + "****" + key[len(key)-4:]
}

func scanSharedPoolAccessKey(s scanner) (*service.SharedPoolAccessKey, error) {
	var k service.SharedPoolAccessKey
	var modelsRaw []byte
	if err := s.Scan(&k.ID, &k.PoolID, &k.PoolName, &k.UserID, &k.APIKeyID, &k.Name, &k.KeyPreview, &k.Status, &modelsRaw, &k.TotalUsed, &k.LastUsedAt, &k.CreatedAt); err != nil {
		return nil, err
	}
	k.AllowedModels = scanStringArray(modelsRaw)
	setSharedPoolAccessKeyDefaults(&k)
	return &k, nil
}

func scanSharedPoolAccessKeyRuntime(s scanner) (*service.SharedPoolAccessKey, error) {
	var k service.SharedPoolAccessKey
	var rawKey string
	var modelsRaw []byte
	var effectiveRateMultiplier float64
	if err := s.Scan(&k.ID, &k.PoolID, &k.PoolName, &k.UserID, &k.APIKeyID, &k.Name, &rawKey, &k.Status, &modelsRaw, &k.TotalUsed, &k.LastUsedAt, &k.CreatedAt, &k.OwnerID, &k.AccountID, &k.AuthType, &k.UpstreamBaseURL, &k.UpstreamAPIKey, &k.OAuthCredentialsEncrypted, &k.ExpiresAt, &k.ProxyURL, &k.RateMultiplier, &k.OwnerSharePercent, &k.AccountConcurrency, &k.UserConcurrency, &k.ModelConcurrency, &k.AccountMode, &effectiveRateMultiplier, &k.PublishedModelName, &k.UpstreamModelName, &k.CanonicalModelName, &k.Provider); err != nil {
		return nil, err
	}
	k.KeyPreview = keyPreview(rawKey)
	k.AllowedModels = scanStringArray(modelsRaw)
	if effectiveRateMultiplier > 0 {
		k.RateMultiplier = effectiveRateMultiplier
	}
	setSharedPoolAccessKeyDefaults(&k)
	return &k, nil
}

func setSharedPoolAccessKeyDefaults(k *service.SharedPoolAccessKey) {
	if k == nil {
		return
	}
	if k.AccountConcurrency <= 0 {
		k.AccountConcurrency = 1
	}
	if k.UserConcurrency <= 0 {
		k.UserConcurrency = 1
	}
}

func scanSharedPoolAccount(s scanner) (*service.SharedPoolAccount, error) {
	var a service.SharedPoolAccount
	var proxyID sql.NullInt64
	var tlsProfileID sql.NullInt64
	var modelConfigsRaw []byte
	var lastProbeAt sql.NullTime
	var lastProbeSuccess sql.NullBool
	var lastSuccessfulProbeAt sql.NullTime
	var lastUsedAt sql.NullTime
	var expiresAt sql.NullTime
	if err := s.Scan(
		&a.ID, &a.PoolID, &a.OwnerID, &a.Name, &a.Description, &a.Provider, &a.AuthType,
		&a.UpstreamBaseURL, &a.HasUpstreamKey, &a.HasOAuthCredentials, &a.KeyPreview, &a.CredentialFingerprint,
		&expiresAt, &a.AutoPauseOnExpired, &a.Schedulable, &a.Status, &a.StatusNote, &a.DisabledReason,
		&a.GroupName, &proxyID, &a.ProxyURL, &a.ProxyRegion, &a.ProxyStatus, &a.AccountWeight, &a.Priority,
		&a.RPMLimit, &a.AccountConcurrency, &a.UserConcurrency, &tlsProfileID, &a.TTLSeconds,
		&a.CachePolicy, &a.RoutingPolicy, &modelConfigsRaw, &lastProbeAt, &lastProbeSuccess,
		&a.LastProbeErrorType, &a.LastProbeErrorMessage, &lastSuccessfulProbeAt, &a.FullCheckScore,
		&a.FullCheckPassed, &a.FullCheckTotal, &a.GateRequired, &a.GatePassed,
		&a.TotalCalls, &a.SuccessfulCalls, &a.FailedCalls, &lastUsedAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		a.ExpiresAt = &expiresAt.Time
	}
	if proxyID.Valid {
		a.ProxyID = &proxyID.Int64
	}
	if tlsProfileID.Valid {
		a.TLSProfileID = &tlsProfileID.Int64
	}
	if len(a.CachePolicy) == 0 {
		a.CachePolicy = json.RawMessage(`{}`)
	}
	if len(a.RoutingPolicy) == 0 {
		a.RoutingPolicy = json.RawMessage(`{}`)
	}
	a.ModelConfigs = scanSharedPoolAccountModelConfigs(modelConfigsRaw)
	if lastProbeAt.Valid {
		a.LastProbeAt = &lastProbeAt.Time
	}
	if lastProbeSuccess.Valid {
		a.LastProbeSuccess = &lastProbeSuccess.Bool
	}
	if lastSuccessfulProbeAt.Valid {
		a.LastSuccessfulProbeAt = &lastSuccessfulProbeAt.Time
	}
	if lastUsedAt.Valid {
		a.LastUsedAt = &lastUsedAt.Time
	}
	return &a, nil
}

func scanSharedPoolAccountModelConfigs(raw []byte) []service.SharedPoolModelConfig {
	if len(raw) == 0 {
		return []service.SharedPoolModelConfig{}
	}
	var out []service.SharedPoolModelConfig
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []service.SharedPoolModelConfig{}
	}
	return out
}

func sharedPoolAccountModelConfigsFromInput(configs []service.SharedPoolModelInput) []service.SharedPoolModelConfig {
	out := make([]service.SharedPoolModelConfig, 0, len(configs))
	for _, cfg := range sharedPoolModelConfigsFromInput(configs) {
		cfg.DisplayName = cfg.ModelName
		out = append(out, cfg)
	}
	return out
}

const sharedPoolAccountColumns = `id, pool_id, owner_id, name, description, provider, auth_type, upstream_base_url, upstream_api_key <> '', credentials_encrypted <> '', key_preview, credential_fingerprint, expires_at, auto_pause_on_expired, schedulable, status, status_note, disabled_reason, group_name, proxy_id, proxy_url, proxy_region, proxy_status, account_weight, priority, rpm_limit, account_concurrency, user_concurrency, tls_profile_id, ttl_seconds, cache_policy, routing_policy, model_configs, last_probe_at, last_probe_success, last_probe_error_type, last_probe_error_message, last_successful_probe_at, full_check_score, full_check_passed, full_check_total, gate_required, gate_passed, total_calls, successful_calls, failed_calls, last_used_at, created_at, updated_at`

func openSharedPoolModelNames(configs []service.SharedPoolModelInput) []string {
	out := make([]string, 0, len(configs))
	for _, cfg := range configs {
		if cfg.ModelOpen && strings.TrimSpace(cfg.ModelName) != "" {
			out = append(out, strings.TrimSpace(cfg.ModelName))
		}
	}
	return out
}

func sharedPoolModelConfigsFromInput(configs []service.SharedPoolModelInput) []service.SharedPoolModelConfig {
	out := make([]service.SharedPoolModelConfig, 0, len(configs))
	for _, cfg := range configs {
		if strings.TrimSpace(cfg.ModelName) == "" {
			continue
		}
		out = append(out, service.SharedPoolModelConfig{
			Provider:                  normalizeSharedPoolProviderName(cfg.Provider),
			ModelName:                 strings.TrimSpace(cfg.ModelName),
			UpstreamModelName:         strings.TrimSpace(cfg.UpstreamModelName),
			RateMultiplier:            cfg.RateMultiplier,
			FiveHourProtectionPercent: cfg.FiveHourProtectionPercent,
			SevenDayProtectionPercent: cfg.SevenDayProtectionPercent,
			DailyProtectionPercent:    cfg.DailyProtectionPercent,
			MaxConcurrency:            cfg.MaxConcurrency,
			ModelOpen:                 cfg.ModelOpen,
		})
	}
	return out
}

func sharedPoolModelUpdateRequested(input service.UpdateSharedPoolInput) bool {
	return input.ModelsSet || input.ModelConfigsSet
}

func sharedPoolModelRateSyncRequested(input service.UpdateSharedPoolInput) bool {
	return input.RateMultiplierSet && input.SyncModelRates
}

const createSharedPoolQuery = `INSERT INTO shared_pools (
	owner_id, owner_label, name, description, avatar_url, status_note, disabled_reason, tier, status, listed, lifecycle_state, rate_multiplier, max_users, current_users,
	min_balance_admission, hourly_seat_fee, hourly_min_usage_waiver, today_availability, seven_day_availability, avg_latency_ms,
	upstream_base_url, upstream_api_key, proxy_id, proxy_url, proxy_region, proxy_status,
	account_concurrency, user_concurrency, account_mode_enabled, oauth_provider, verification_mode, verification_exemption_reason, platform_fee_percent, quality_score, native_onboarding_state
) VALUES ($1, $2, $3, $4, $5, $6, $7, 'Standard', $8::text, $9::boolean,
	CASE WHEN $9::boolean AND $8::text IN ('healthy', 'limited') THEN 'operating' ELSE 'draft' END,
	$10, $11, 0, $12, $13, $14, 0, 0, 0, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, 90.0, 'legacy_existing')
RETURNING ` + sharedPoolColumns

func (r *bizDecipherRepository) CreateSharedPoolTx(ctx context.Context, input service.CreateSharedPoolInput) (*service.SharedPool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	ownerLabel := "shared"
	_ = tx.QueryRowContext(ctx, `
		SELECT COALESCE(NULLIF(TRIM(bp.display_name), ''), NULLIF(TRIM(u.username), ''), u.email, 'shared')
		FROM users u
		LEFT JOIN biz_profiles bp ON bp.user_id = u.id
		WHERE u.id = $1`, input.OwnerID).Scan(&ownerLabel)
	row := tx.QueryRowContext(ctx, createSharedPoolQuery,
		input.OwnerID, ownerLabel, input.Name, input.Description, input.AvatarURL, input.StatusNote, input.DisabledReason,
		input.Status, input.Listed, input.RateMultiplier, input.MaxUsers,
		input.MinBalanceAdmission, input.HourlySeatFee, input.HourlyMinUsageWaiver, input.UpstreamBaseURL, input.UpstreamAPIKey, input.ProxyID, input.ProxyURL,
		input.ProxyRegion, input.ProxyStatus, input.AccountConcurrency, input.UserConcurrency, input.AccountModeEnabled, input.OAuthProvider, input.VerificationMode, input.VerificationExemptionReason, input.PlatformFeePercent,
	)
	pool, err := scanSharedPool(row)
	if err != nil {
		return nil, err
	}
	if err := insertSharedPoolSettlementRuleVersionTx(ctx, tx, pool.ID, pool.HourlySeatFee, pool.HourlyMinUsageWaiver, pool.PlatformFeePercent, "bootstrap", input.OwnerID, time.Now().UTC().Truncate(time.Hour)); err != nil {
		return nil, err
	}
	for i, cfg := range input.ModelConfigs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO shared_pool_models (pool_id, provider, model_name, upstream_model_name, sort_order, enabled, rate_multiplier, rank_weight, five_hour_protection_percent, seven_day_protection_percent, daily_protection_percent, min_balance_admission, hourly_seat_fee, max_concurrency, model_open)
			 VALUES ($1, COALESCE(NULLIF($2, ''), (SELECT provider FROM model_catalog WHERE model_name = $3 AND enabled = TRUE ORDER BY mainstream DESC, sort_order LIMIT 1), 'openai'), $3, $4, $5, TRUE, $6, 100, $7, $8, $9, $10, $11, $12, $13)
			 ON CONFLICT (pool_id, model_name) DO UPDATE SET enabled = TRUE, sort_order = EXCLUDED.sort_order, provider = EXCLUDED.provider, upstream_model_name = EXCLUDED.upstream_model_name, rate_multiplier = EXCLUDED.rate_multiplier, five_hour_protection_percent = EXCLUDED.five_hour_protection_percent, seven_day_protection_percent = EXCLUDED.seven_day_protection_percent, daily_protection_percent = EXCLUDED.daily_protection_percent, min_balance_admission = EXCLUDED.min_balance_admission, hourly_seat_fee = EXCLUDED.hourly_seat_fee, max_concurrency = EXCLUDED.max_concurrency, model_open = EXCLUDED.model_open`,
			pool.ID, normalizeSharedPoolProviderName(cfg.Provider), cfg.ModelName, cfg.UpstreamModelName, i, cfg.RateMultiplier, cfg.FiveHourProtectionPercent, cfg.SevenDayProtectionPercent,
			cfg.DailyProtectionPercent, input.MinBalanceAdmission, input.HourlySeatFee, cfg.MaxConcurrency, cfg.ModelOpen,
		); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	pool.Models = openSharedPoolModelNames(input.ModelConfigs)
	pool.ModelConfigs = sharedPoolModelConfigsFromInput(input.ModelConfigs)
	createdPools := []service.SharedPool{*pool}
	if err := r.hydrateSharedPoolSettlementRules(ctx, createdPools); err != nil {
		return nil, err
	}
	for i := range createdPools[0].ModelConfigs {
		createdPools[0].ModelConfigs[i].HourlySeatFee = createdPools[0].HourlySeatFee
		createdPools[0].ModelConfigs[i].HourlyMinUsageWaiver = createdPools[0].HourlyMinUsageWaiver
	}
	return &createdPools[0], nil
}

func (r *bizDecipherRepository) UpdateSharedPoolTx(ctx context.Context, poolID, ownerID int64, input service.UpdateSharedPoolInput) (*service.SharedPool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var existingAPIKey string
	var existingHourlyFee float64
	var existingWaiverMin float64
	var existingPlatformFee float64
	var existingAccountMode bool
	var existingConfigVersion int64
	var existingGovernanceStatus string
	if err := tx.QueryRowContext(ctx, `SELECT upstream_api_key, hourly_seat_fee, hourly_min_usage_waiver, platform_fee_percent, account_mode_enabled, config_version, COALESCE(governance_status, 'normal') FROM shared_pools WHERE id = $1 AND owner_id = $2 FOR UPDATE`, poolID, ownerID).Scan(&existingAPIKey, &existingHourlyFee, &existingWaiverMin, &existingPlatformFee, &existingAccountMode, &existingConfigVersion, &existingGovernanceStatus); err != nil {
		return nil, err
	}
	if input.ExpectedConfigVersion > 0 && existingConfigVersion != input.ExpectedConfigVersion {
		return nil, service.ErrSharedPoolConcurrentUpdate
	}
	if input.Listed && !service.SharedPoolGovernanceAllowsOwnerListing(existingGovernanceStatus) {
		return nil, service.ErrSharedPoolGovernanceBlocked
	}
	modeChanged := existingAccountMode != input.AccountModeEnabled
	if modeChanged {
		input.Listed = false
	}
	upstreamAPIKey := strings.TrimSpace(input.UpstreamAPIKey)
	if input.AccountModeEnabled {
		upstreamAPIKey = ""
	} else if upstreamAPIKey == "" {
		upstreamAPIKey = existingAPIKey
	}
	row := tx.QueryRowContext(ctx, `UPDATE shared_pools SET
		name = $1,
		description = $2,
		avatar_url = $3,
		status_note = $4,
		disabled_reason = $5,
		status = $6::text,
		listed = $7,
		rate_multiplier = $8,
		max_users = $9,
		min_balance_admission = $10,
		hourly_seat_fee = $11,
		hourly_min_usage_waiver = $12,
		upstream_base_url = $13,
		upstream_api_key = $14,
		owner_share_percent = owner_share_percent,
		proxy_id = $15,
		proxy_url = $16,
		proxy_region = $17,
		proxy_status = $18,
		account_concurrency = $19,
		user_concurrency = $20,
		account_mode_enabled = $21,
		oauth_provider = $22,
		verification_mode = $23,
		verification_exemption_reason = $24,
		lifecycle_state = CASE
			WHEN $7::boolean = TRUE AND $6::text IN ('healthy', 'limited') THEN 'operating'
			WHEN lifecycle_state = 'draft' THEN 'draft'
			ELSE 'suspended'
		END,
		updated_at = NOW()
		WHERE id = $25 AND owner_id = $26 AND lifecycle_state <> 'archived'
		RETURNING `+sharedPoolColumns,
		input.Name, input.Description, input.AvatarURL, input.StatusNote, input.DisabledReason,
		input.Status, input.Listed, input.RateMultiplier, input.MaxUsers,
		input.MinBalanceAdmission, input.HourlySeatFee, input.HourlyMinUsageWaiver, input.UpstreamBaseURL, upstreamAPIKey,
		input.ProxyID, input.ProxyURL, input.ProxyRegion, input.ProxyStatus, input.AccountConcurrency, input.UserConcurrency,
		input.AccountModeEnabled, input.OAuthProvider, input.VerificationMode, input.VerificationExemptionReason, poolID, ownerID,
	)
	pool, err := scanSharedPool(row)
	if err != nil {
		return nil, err
	}
	if modeChanged {
		if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_access_keys SET status = 'disabled', updated_at = NOW() WHERE pool_id = $1 AND status = 'active'`, poolID); err != nil {
			return nil, err
		}
		if input.AccountModeEnabled {
			if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_accounts SET gate_passed = FALSE, full_check_score = 0, full_check_passed = 0, full_check_total = 0, updated_at = NOW() WHERE pool_id = $1 AND deleted_at IS NULL`, poolID); err != nil {
				return nil, err
			}
		}
	}
	if existingHourlyFee != pool.HourlySeatFee || existingWaiverMin != pool.HourlyMinUsageWaiver {
		if err := insertSharedPoolSettlementRuleVersionTx(ctx, tx, poolID, pool.HourlySeatFee, pool.HourlyMinUsageWaiver, existingPlatformFee, "owner_update", ownerID, nextSharedPoolSettlementHour(time.Now())); err != nil {
			return nil, err
		}
	}
	if sharedPoolModelUpdateRequested(input) {
		if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_models SET enabled = FALSE WHERE pool_id = $1`, poolID); err != nil {
			return nil, err
		}
		for i, cfg := range input.ModelConfigs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO shared_pool_models (pool_id, provider, model_name, upstream_model_name, sort_order, enabled, rate_multiplier, rank_weight, five_hour_protection_percent, seven_day_protection_percent, daily_protection_percent, min_balance_admission, hourly_seat_fee, max_concurrency, model_open)
				 VALUES ($1, COALESCE(NULLIF($2, ''), (SELECT provider FROM model_catalog WHERE model_name = $3 AND enabled = TRUE ORDER BY mainstream DESC, sort_order LIMIT 1), 'openai'), $3, $4, $5, TRUE, $6, 100, $7, $8, $9, $10, $11, $12, $13)
				 ON CONFLICT (pool_id, model_name) DO UPDATE SET enabled = TRUE, sort_order = EXCLUDED.sort_order, provider = EXCLUDED.provider, upstream_model_name = EXCLUDED.upstream_model_name, rate_multiplier = EXCLUDED.rate_multiplier, five_hour_protection_percent = EXCLUDED.five_hour_protection_percent, seven_day_protection_percent = EXCLUDED.seven_day_protection_percent, daily_protection_percent = EXCLUDED.daily_protection_percent, min_balance_admission = EXCLUDED.min_balance_admission, hourly_seat_fee = EXCLUDED.hourly_seat_fee, max_concurrency = EXCLUDED.max_concurrency, model_open = EXCLUDED.model_open`,
				poolID, normalizeSharedPoolProviderName(cfg.Provider), cfg.ModelName, cfg.UpstreamModelName, i, cfg.RateMultiplier, cfg.FiveHourProtectionPercent, cfg.SevenDayProtectionPercent,
				cfg.DailyProtectionPercent, input.MinBalanceAdmission, input.HourlySeatFee, cfg.MaxConcurrency, cfg.ModelOpen,
			); err != nil {
				return nil, err
			}
		}
		modelsRaw := string(jsonBytes(openSharedPoolModelNames(input.ModelConfigs)))
		if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_access_keys SET allowed_models = $1::jsonb, updated_at = NOW() WHERE pool_id = $2 AND status = 'active'`, modelsRaw, poolID); err != nil {
			return nil, err
		}
	}
	if sharedPoolModelRateSyncRequested(input) {
		if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_models SET rate_multiplier = $2 WHERE pool_id = $1 AND enabled = TRUE`, poolID, input.RateMultiplier); err != nil {
			return nil, err
		}
	}
	// Model routing triggers can advance the pool's optimistic-lock fence after
	// the UPDATE ... RETURNING row was scanned. Return the final version so
	// the owner's next PATCH is not rejected as stale immediately after saving.
	if err := tx.QueryRowContext(ctx,
		`SELECT config_version FROM shared_pools WHERE id = $1 AND owner_id = $2`,
		poolID, ownerID,
	).Scan(&pool.ConfigVersion); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	pool.Models = openSharedPoolModelNames(input.ModelConfigs)
	pool.ModelConfigs = sharedPoolModelConfigsFromInput(input.ModelConfigs)
	if sharedPoolModelRateSyncRequested(input) {
		for i := range pool.ModelConfigs {
			pool.ModelConfigs[i].RateMultiplier = input.RateMultiplier
		}
	}
	updatedPools := []service.SharedPool{*pool}
	if err := r.hydrateSharedPoolSettlementRules(ctx, updatedPools); err != nil {
		logger.LegacyPrintf("repository.bizdecipher", "[SharedPool] update committed but settlement hydration failed: pool=%d err=%v", poolID, err)
		return pool, nil
	}
	for i := range updatedPools[0].ModelConfigs {
		updatedPools[0].ModelConfigs[i].HourlySeatFee = updatedPools[0].HourlySeatFee
		updatedPools[0].ModelConfigs[i].HourlyMinUsageWaiver = updatedPools[0].HourlyMinUsageWaiver
	}
	return &updatedPools[0], nil
}

func (r *bizDecipherRepository) GetSharedPoolUpstreamRuntime(ctx context.Context, poolID, ownerID int64) (*service.SharedPoolUpstreamRuntime, error) {
	var runtime service.SharedPoolUpstreamRuntime
	if err := r.db.QueryRowContext(ctx, `SELECT
		sp.upstream_base_url,
		sp.upstream_api_key,
		sp.proxy_url,
		COALESCE((
			SELECT COALESCE(NULLIF(spm.upstream_model_name, ''), spm.model_name)
			FROM shared_pool_models spm
			WHERE spm.pool_id = sp.id AND spm.enabled = TRUE
			ORDER BY spm.sort_order ASC, spm.id ASC
			LIMIT 1
		), '')
		FROM shared_pools sp
		WHERE sp.id = $1 AND sp.owner_id = $2 AND sp.native_onboarding_state = 'legacy_existing'`, poolID, ownerID).Scan(&runtime.UpstreamBaseURL, &runtime.UpstreamAPIKey, &runtime.ProxyURL, &runtime.ProbeModel); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrBillingActivationRequired
		}
		return nil, err
	}
	return &runtime, nil
}

func (r *bizDecipherRepository) GetSharedPoolAccountUpstreamRuntime(ctx context.Context, poolID, accountID, ownerID int64) (*service.SharedPoolUpstreamRuntime, error) {
	var runtime service.SharedPoolUpstreamRuntime
	if err := r.db.QueryRowContext(ctx, `SELECT
		spa.auth_type,
		spa.upstream_base_url,
		spa.upstream_api_key,
		spa.credentials_encrypted,
		spa.proxy_url,
		COALESCE((
			SELECT COALESCE(NULLIF(elem->>'upstream_model_name', ''), elem->>'model_name')
			FROM jsonb_array_elements(spa.model_configs) elem
			WHERE COALESCE(NULLIF(elem->>'model_name', ''), '') <> ''
				AND COALESCE((elem->>'model_open')::boolean, TRUE)
			ORDER BY elem->>'model_name'
			LIMIT 1
		), ''),
		spa.expires_at
		FROM shared_pool_accounts spa
		JOIN shared_pools sp ON sp.id = spa.pool_id AND sp.owner_id = spa.owner_id
		WHERE spa.pool_id = $1 AND spa.id = $2 AND spa.owner_id = $3 AND spa.deleted_at IS NULL AND sp.native_onboarding_state = 'legacy_existing'`, poolID, accountID, ownerID).Scan(&runtime.AuthType, &runtime.UpstreamBaseURL, &runtime.UpstreamAPIKey, &runtime.CredentialsEncrypted, &runtime.ProxyURL, &runtime.ProbeModel, &runtime.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrBillingActivationRequired
		}
		return nil, err
	}
	return &runtime, nil
}

func (r *bizDecipherRepository) GetSharedPoolGovernanceSummary(ctx context.Context, now time.Time) (*service.SharedPoolGovernanceSummary, error) {
	if now.IsZero() {
		now = time.Now()
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	var summary service.SharedPoolGovernanceSummary
	summary.SnapshotAt = now
	summary.PlatformFeeBasis = "用户实际扣款 - 池主永久收益总账入账"
	summary.AutoGovernanceSummary = "探测聚合会按连续失败、可用率和错误类型自动调整池状态；管理员的抽水、加权、降权、封禁和备注会写入治理日志。"
	row := r.db.QueryRowContext(ctx, `
		WITH usage_today AS (
			SELECT
				COUNT(*) FILTER (WHERE source_type = 'share_pool_usage') AS usage_count,
				COUNT(DISTINCT pool_id) FILTER (WHERE source_type = 'share_pool_usage') AS active_pools,
				COALESCE(SUM(-amount) FILTER (WHERE source_type = 'share_pool_usage'), 0) AS api_usage_charges,
				COALESCE(SUM(owner_payout_amount) FILTER (
					WHERE source_type = 'share_pool_usage' AND owner_payout_amount IS NOT NULL
				), 0) AS api_owner_payout_current,
				COALESCE(SUM(-amount) FILTER (WHERE source_type = 'pool_seat_fee'), 0) AS seat_fee_charges,
				COALESCE(SUM(amount) FILTER (WHERE source_type = 'pool_owner_payout'), 0) AS seat_owner_payout,
				COALESCE(SUM(amount) FILTER (
					WHERE source_type = 'share_pool_payout'
					  AND settlement_destination = 'legacy_balance'
				), 0) AS api_owner_payout_legacy
			FROM shared_pool_balance_ledger
			WHERE status = 'posted' AND posted_at >= $1 AND posted_at < $2
		), pool_stats AS (
			SELECT
				COUNT(*) FILTER (WHERE listed = TRUE) AS listed_pools,
				COUNT(*) FILTER (WHERE status = 'healthy') AS healthy_pools,
				COUNT(*) FILTER (WHERE status = 'limited') AS limited_pools,
				COUNT(*) FILTER (WHERE status IN ('offline', 'maintenance')) AS offline_pools,
				COUNT(*) FILTER (WHERE governance_status = 'watch') AS watch_pools,
				COUNT(*) FILTER (WHERE governance_status = 'suppressed') AS suppressed_pools,
				COUNT(*) FILTER (WHERE governance_status = 'banned') AS banned_pools,
				COALESCE(SUM(total_calls), 0) AS total_calls,
				COALESCE(SUM(successful_calls), 0) AS successful_calls,
				COALESCE(SUM(failed_calls), 0) AS failed_calls,
				COALESCE(AVG(today_availability), 0) AS average_availability
			FROM shared_pools
		), complaints AS (
			SELECT COUNT(*) AS open_complaints
			FROM shared_pool_complaints
			WHERE status = 'open'
		)
		SELECT
			u.api_usage_charges,
			u.api_owner_payout_current + u.api_owner_payout_legacy AS api_owner_payout,
			GREATEST(u.api_usage_charges - u.api_owner_payout_current - u.api_owner_payout_legacy, 0) AS api_platform_fee,
			u.seat_fee_charges,
			u.seat_owner_payout,
			GREATEST(u.seat_fee_charges - u.seat_owner_payout, 0) AS seat_platform_fee,
			u.usage_count,
			u.active_pools,
			c.open_complaints,
			p.listed_pools,
			p.healthy_pools,
			p.limited_pools,
			p.offline_pools,
			p.watch_pools,
			p.suppressed_pools,
			p.banned_pools,
			p.total_calls,
			p.successful_calls,
			p.failed_calls,
			p.average_availability
		FROM usage_today u CROSS JOIN pool_stats p CROSS JOIN complaints c`, dayStart, dayEnd)
	if err := row.Scan(
		&summary.TodayAPIUsageCharges,
		&summary.TodayAPIOwnerPayout,
		&summary.TodayAPIPlatformFee,
		&summary.TodaySeatFeeCharges,
		&summary.TodaySeatOwnerPayout,
		&summary.TodaySeatPlatformFee,
		&summary.TodayUsageCount,
		&summary.TodayActivePools,
		&summary.OpenComplaints,
		&summary.ListedPools,
		&summary.HealthyPools,
		&summary.LimitedPools,
		&summary.OfflinePools,
		&summary.WatchPools,
		&summary.SuppressedPools,
		&summary.BannedPools,
		&summary.TotalCalls,
		&summary.SuccessfulCalls,
		&summary.FailedCalls,
		&summary.AverageAvailability,
	); err != nil {
		return nil, err
	}
	summary.TodayGrossCharges = summary.TodayAPIUsageCharges + summary.TodaySeatFeeCharges
	summary.TodayOwnerPayout = summary.TodayAPIOwnerPayout + summary.TodaySeatOwnerPayout
	summary.TodayPlatformFee = math.Max(summary.TodayGrossCharges-summary.TodayOwnerPayout, 0)
	return &summary, nil
}

func (r *bizDecipherRepository) ListMySharedPoolLedger(ctx context.Context, ownerID int64, limit int) (*service.SharedPoolLedgerView, error) {
	ledgerLimit := clampLimit(limit)
	wallet := emptySharedPoolOwnerWallet(ownerID)
	walletErr := r.db.QueryRowContext(ctx, `
		SELECT
			w.owner_id,
			w.available_amount,
			w.pending_amount,
			w.frozen_amount,
			w.transferred_amount,
			COALESCE((
				SELECT SUM(e.net_amount)
				FROM shared_pool_owner_earnings_ledger e
				WHERE e.owner_id = w.owner_id
					AND e.event_type = 'earning'
					AND e.status IN ('available', 'settled')
			), 0),
			w.version,
			w.updated_at
		FROM shared_pool_owner_wallets w
		WHERE w.owner_id = $1`,
		ownerID,
	).Scan(
		&wallet.OwnerID,
		&wallet.AvailableAmount,
		&wallet.PendingAmount,
		&wallet.FrozenAmount,
		&wallet.TransferredAmount,
		&wallet.TotalEarned,
		&wallet.Version,
		&wallet.UpdatedAt,
	)
	if walletErr != nil && !errors.Is(walletErr, sql.ErrNoRows) {
		return nil, walletErr
	}

	earningsRows, err := r.db.QueryContext(ctx, `
		SELECT
			id, owner_id, pool_id, account_id, price_version_id,
			event_type, operation_id, request_id,
			pool_name_snapshot, owner_label_snapshot, model_snapshot, pricing_source_snapshot,
			gross_amount, platform_fee_amount, net_amount,
			wallet_delta, available_after, status,
			available_at, metadata, created_at, posted_at
		FROM shared_pool_owner_earnings_ledger
		WHERE owner_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2`,
		ownerID, ledgerLimit,
	)
	if err != nil {
		return nil, err
	}
	earnings := []service.SharedPoolOwnerEarningsEntry{}
	for earningsRows.Next() {
		entry, scanErr := scanSharedPoolOwnerEarningsEntry(earningsRows)
		if scanErr != nil {
			_ = earningsRows.Close()
			return nil, scanErr
		}
		earnings = append(earnings, *entry)
	}
	if err := earningsRows.Err(); err != nil {
		_ = earningsRows.Close()
		return nil, err
	}
	if err := earningsRows.Close(); err != nil {
		return nil, err
	}

	activityRows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, pool_id, source_type, source_id, amount, balance_after, status, note, created_at, posted_at
		FROM shared_pool_balance_ledger
		WHERE user_id = $1 AND source_type IN ('share_pool_usage', 'pool_seat_fee')
		ORDER BY created_at DESC
		LIMIT $2`, ownerID, ledgerLimit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = activityRows.Close() }()
	activity, err := scanSharedPoolBalanceLedgerRows(activityRows)
	if err != nil {
		return nil, err
	}

	legacyRows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, pool_id, source_type, source_id, amount, balance_after, status, note, created_at, posted_at
		FROM shared_pool_balance_ledger
		WHERE user_id = $1
			AND source_type IN ('share_pool_payout', 'pool_owner_payout')
			AND settlement_destination = 'legacy_balance'
		UNION ALL
		SELECT id, user_id, NULL::BIGINT AS pool_id, source_type, source_id, amount, balance_after, status, note, created_at, posted_at
		FROM credit_ledger
		WHERE user_id = $1
			AND source_type IN ('share_pool_payout', 'pool_owner_payout')
			AND NOT EXISTS (
				SELECT 1 FROM shared_pool_balance_ledger b
				WHERE b.user_id = credit_ledger.user_id
					AND b.source_type = credit_ledger.source_type
					AND b.source_id = credit_ledger.source_id
			)
		ORDER BY created_at DESC
		LIMIT $2`, ownerID, ledgerLimit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = legacyRows.Close() }()
	legacyWithdrawable, err := scanSharedPoolBalanceLedgerRows(legacyRows)
	if err != nil {
		return nil, err
	}
	for i := range legacyWithdrawable {
		if legacyWithdrawable[i].SourceType == "share_pool_payout" || legacyWithdrawable[i].SourceType == "pool_owner_payout" {
			if legacyWithdrawable[i].PoolID == nil {
				legacyWithdrawable[i].AssetType = "balance_legacy"
			}
		}
	}
	withdrawable := make([]service.SharedPoolBalanceLedgerEntry, 0, len(earnings)+len(legacyWithdrawable))
	for _, earning := range earnings {
		if earning.EventType != "earning" || earning.Status == "reversed" {
			continue
		}
		withdrawable = append(withdrawable, service.SharedPoolBalanceLedgerEntry{
			ID:           earning.ID,
			UserID:       earning.OwnerID,
			PoolID:       earning.PoolID,
			SourceType:   "owner_wallet_earning",
			SourceID:     earning.OperationID,
			AssetType:    "owner_wallet",
			Amount:       earning.NetAmount,
			BalanceAfter: earning.AvailableAfter,
			Status:       earning.Status,
			Note:         fmt.Sprintf("%s · %s", earning.PoolNameSnapshot, earning.ModelSnapshot),
			CreatedAt:    earning.CreatedAt,
			PostedAt:     earning.PostedAt,
		})
	}
	withdrawable = append(withdrawable, legacyWithdrawable...)

	incentiveRows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, source_type, source_id, amount, balance_after, status, note, created_by, created_at, posted_at
		FROM credit_ledger
		WHERE user_id = $1 AND source_type = 'shared_pool_stability_reward'
		ORDER BY created_at DESC
		LIMIT $2`, ownerID, ledgerLimit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = incentiveRows.Close() }()
	incentives, err := scanCreditRows(incentiveRows)
	if err != nil {
		return nil, err
	}
	if incentives == nil {
		incentives = []service.BizCreditLedgerEntry{}
	}
	for i := range incentives {
		incentives[i].AssetType = "credit_balance"
	}

	return &service.SharedPoolLedgerView{
		Wallet:             wallet,
		Earnings:           earnings,
		Activity:           activity,
		Withdrawable:       withdrawable,
		LegacyWithdrawable: legacyWithdrawable,
		Incentives:         incentives,
	}, nil
}

func (r *bizDecipherRepository) ListSharedPoolOwnerEarningsPage(ctx context.Context, ownerID, poolID, beforeID int64, limit int) (*service.SharedPoolOwnerEarningsPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, owner_id, pool_id, account_id, price_version_id,
			event_type, operation_id, request_id,
			pool_name_snapshot, owner_label_snapshot, model_snapshot, pricing_source_snapshot,
			gross_amount, platform_fee_amount, net_amount,
			wallet_delta, available_after, status,
			available_at, metadata, created_at, posted_at
		FROM shared_pool_owner_earnings_ledger
		WHERE owner_id = $1
			AND ($2::BIGINT = 0 OR id < $2)
			AND ($3::BIGINT = 0 OR pool_id = $3)
		ORDER BY id DESC
		LIMIT $4`,
		ownerID, beforeID, poolID, limit+1,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.SharedPoolOwnerEarningsEntry, 0, limit+1)
	for rows.Next() {
		entry, scanErr := scanSharedPoolOwnerEarningsEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	page := &service.SharedPoolOwnerEarningsPage{
		Items:   items,
		HasMore: hasMore,
	}
	if hasMore && len(items) > 0 {
		page.NextBeforeID = items[len(items)-1].ID
	}
	return page, nil
}

func (r *bizDecipherRepository) AdminListSharedPoolOwnerEarningsPage(ctx context.Context, filter service.AdminSharedPoolOwnerEarningsFilter) (*service.AdminSharedPoolOwnerEarningsPage, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}
	const filterSQL = `
		WHERE ($1::BIGINT = 0 OR earning.owner_id = $1)
		  AND ($2::BIGINT = 0 OR earning.pool_id = $2)
		  AND ($3::TEXT = '' OR earning.status = $3)
		  AND (
			$4::TEXT = ''
			OR ($4 = 'earning' AND earning.event_type = 'earning')
			OR ($4 = 'api' AND earning.event_type = 'earning' AND earning.metadata->>'source_type' = 'share_pool_payout')
			OR ($4 = 'seat' AND earning.event_type = 'earning' AND earning.metadata->>'source_type' = 'pool_owner_payout')
			OR ($4 = 'transfer' AND earning.event_type = 'transfer_to_balance')
			OR ($4 = 'adjustment' AND earning.event_type IN ('adjustment', 'reversal'))
		  )`

	var page service.AdminSharedPoolOwnerEarningsPage
	if err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*), COUNT(DISTINCT earning.owner_id),
			COALESCE(SUM(earning.gross_amount) FILTER (WHERE earning.event_type = 'earning' AND earning.status <> 'reversed'), 0),
			COALESCE(SUM(earning.platform_fee_amount) FILTER (WHERE earning.event_type = 'earning' AND earning.status <> 'reversed'), 0),
			COALESCE(SUM(earning.net_amount) FILTER (WHERE earning.event_type = 'earning' AND earning.status <> 'reversed'), 0)
		FROM shared_pool_owner_earnings_ledger earning
		`+filterSQL,
		filter.OwnerID, filter.PoolID, filter.Status, filter.Kind,
	).Scan(
		&page.MatchingEntries,
		&page.MatchingOwners,
		&page.MatchingGrossAmount,
		&page.MatchingPlatformFee,
		&page.MatchingNetAmount,
	); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			earning.id, earning.owner_id, earning.pool_id, earning.account_id, earning.price_version_id,
			earning.event_type, earning.operation_id, earning.request_id,
			earning.pool_name_snapshot, earning.owner_label_snapshot, earning.model_snapshot, earning.pricing_source_snapshot,
			earning.gross_amount, earning.platform_fee_amount, earning.net_amount,
			earning.wallet_delta, earning.available_after, earning.status,
			earning.available_at, earning.metadata, earning.created_at, earning.posted_at,
			COALESCE(owner.email, ''), COALESCE(owner.username, '')
		FROM shared_pool_owner_earnings_ledger earning
		LEFT JOIN users owner ON owner.id = earning.owner_id
		`+filterSQL+`
		  AND ($5::BIGINT = 0 OR earning.id < $5)
		ORDER BY earning.id DESC
		LIMIT $6`,
		filter.OwnerID, filter.PoolID, filter.Status, filter.Kind, filter.BeforeID, filter.Limit+1,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AdminSharedPoolOwnerEarningsEntry, 0, filter.Limit+1)
	for rows.Next() {
		var item service.AdminSharedPoolOwnerEarningsEntry
		entry, scanErr := scanSharedPoolOwnerEarningsEntryWithExtras(rows, &item.OwnerEmail, &item.OwnerUsername)
		if scanErr != nil {
			return nil, scanErr
		}
		item.SharedPoolOwnerEarningsEntry = *entry
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	page.HasMore = len(items) > filter.Limit
	if page.HasMore {
		items = items[:filter.Limit]
	}
	page.Items = items
	if page.HasMore && len(items) > 0 {
		page.NextBeforeID = items[len(items)-1].ID
	}
	return &page, nil
}

func (r *bizDecipherRepository) GetOwnedSharedPool(ctx context.Context, poolID, ownerID int64) (*service.SharedPool, error) {
	for attempt := 0; attempt < 2; attempt++ {
		var beforeVersion int64
		if err := r.db.QueryRowContext(ctx,
			`SELECT config_version FROM shared_pools WHERE id = $1 AND owner_id = $2 AND lifecycle_state <> 'archived'`,
			poolID, ownerID,
		).Scan(&beforeVersion); err != nil {
			return nil, err
		}
		pool, err := scanSharedPool(r.db.QueryRowContext(ctx,
			`SELECT `+sharedPoolColumns+` FROM shared_pools WHERE id = $1 AND owner_id = $2 AND lifecycle_state <> 'archived'`,
			poolID,
			ownerID,
		))
		if err != nil {
			return nil, err
		}
		pools := []service.SharedPool{*pool}
		if err := r.loadPoolModels(ctx, pools); err != nil {
			return nil, err
		}
		var afterVersion int64
		if err := r.db.QueryRowContext(ctx,
			`SELECT config_version FROM shared_pools WHERE id = $1 AND owner_id = $2 AND lifecycle_state <> 'archived'`,
			poolID, ownerID,
		).Scan(&afterVersion); err != nil {
			return nil, err
		}
		if beforeVersion == afterVersion {
			pools[0].ConfigVersion = afterVersion
			return &pools[0], nil
		}
	}
	return nil, service.ErrSharedPoolConcurrentUpdate
}

func (r *bizDecipherRepository) ListMySharedPools(ctx context.Context, ownerID int64) ([]service.SharedPool, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+sharedPoolColumns+` FROM shared_pools WHERE owner_id = $1 AND lifecycle_state <> 'archived' AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 200`, ownerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.SharedPool{}
	for rows.Next() {
		p, err := scanSharedPool(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.loadPoolModels(ctx, out); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolProbeSummaries(ctx, out); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolOwnerCardAssets(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *bizDecipherRepository) DeleteSharedPoolTx(ctx context.Context, poolID, ownerID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var lifecycleState string
	if err := tx.QueryRowContext(ctx,
		`SELECT lifecycle_state
		 FROM shared_pools
		 WHERE id = $1 AND owner_id = $2
		 FOR UPDATE`,
		poolID, ownerID,
	).Scan(&lifecycleState); err != nil {
		return err
	}
	if lifecycleState == "archived" {
		if err := retireSharedPoolSupplyTx(ctx, tx, poolID, 0); err != nil {
			return err
		}
		if err := retireSharedPoolGroupTx(ctx, tx, poolID); err != nil {
			return err
		}
		return tx.Commit()
	}

	var activeSeats int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM pool_seat_bindings WHERE pool_id = $1 AND status IN ('active', 'held')`, poolID).Scan(&activeSeats); err != nil {
		return err
	}
	if activeSeats > 0 {
		return fmt.Errorf("仍有 %d 个活跃成员或席位；可先停用，移除成员后再删除", activeSeats)
	}

	var pendingUsage bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM shared_pool_usage_reservations WHERE pool_id=$1
		AND status IN ('reserved','forwarding','settlement_pending','review_required')
	)`, poolID).Scan(&pendingUsage); err != nil { return err }
	if pendingUsage { return errors.New("仍有在途调用或待核对用量；可先停用，完成结算后再删除") }

	const archiveReason = "池主归档：保留池子、账号、账本和历史记录"
	if err := retireSharedPoolSupplyTx(ctx, tx, poolID, 0); err != nil {
		return err
	}
	if err := retireSharedPoolGroupTx(ctx, tx, poolID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pools
		SET lifecycle_state = 'archived',
			archived_at = NOW(),
			deleted_at = NOW(),
			archived_by = $2,
			archive_reason = $3,
			listed = FALSE,
			status = 'offline',
			status_note = '已归档，不再接收新成员或调用',
			current_users = 0,
			updated_at = NOW()
		WHERE id = $1 AND owner_id = $2`, poolID, ownerID, archiveReason); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO shared_pool_archive_events
		(pool_id, actor_id, action, before_state, after_state, reason)
		VALUES ($1, $2, 'archive', $3, 'archived', $4)`,
		poolID, ownerID, lifecycleState, archiveReason,
	); err != nil {
		return err
	}
	// Credentials and access-key bindings remain as historical evidence. They
	// are only made unschedulable/revoked, never deleted.
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_accounts
		SET schedulable = FALSE,
			status = 'disabled',
			disabled_reason = COALESCE(NULLIF(disabled_reason, ''), 'pool archived by owner'),
			updated_at = NOW()
		WHERE pool_id = $1 AND deleted_at IS NULL`, poolID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_access_keys
		SET status = 'revoked', updated_at = NOW()
		WHERE pool_id = $1 AND status = 'active'`, poolID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *bizDecipherRepository) RestoreSharedPoolTx(ctx context.Context, poolID, actorID int64, reason, operationID string) (*service.SharedPool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	before, err := scanSharedPool(tx.QueryRowContext(ctx,
		`SELECT `+sharedPoolColumns+` FROM shared_pools WHERE id = $1 FOR UPDATE`,
		poolID,
	))
	if err != nil {
		return nil, err
	}
	if before.LifecycleState != "archived" {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return before, nil
	}

	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "管理员恢复为草稿，需重新配置并通过满血检测后上架"
	}
	if len([]rune(reason)) > 500 {
		reason = string([]rune(reason)[:500])
	}
	operationID = strings.TrimSpace(operationID)
	if len(operationID) > 160 {
		return nil, errors.New("operation id is too long")
	}

	restored, err := scanSharedPool(tx.QueryRowContext(ctx, `UPDATE shared_pools
		SET lifecycle_state = 'draft',
			archived_at = NULL,
			deleted_at = NULL,
			archived_by = NULL,
			archive_reason = '',
			listed = FALSE,
			status = 'maintenance',
			status_note = '已恢复为草稿；重新配置并通过满血检测后才能上架',
			updated_at = NOW()
		WHERE id = $1 AND lifecycle_state = 'archived'
		RETURNING `+sharedPoolColumns, poolID))
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO shared_pool_archive_events
		(pool_id, actor_id, action, before_state, after_state, reason, operation_id)
		VALUES ($1, NULLIF($2, 0), 'restore', 'archived', 'draft', $3, $4)
		ON CONFLICT (pool_id, action, operation_id) WHERE operation_id <> '' DO NOTHING`,
		poolID, actorID, reason, operationID,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	items := []service.SharedPool{*restored}
	if err := r.loadPoolModels(ctx, items); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolProbeSummaries(ctx, items); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolAccountSummaries(ctx, items); err != nil {
		return nil, err
	}
	if err := r.hydrateSharedPoolOwnerCardAssets(ctx, items); err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (r *bizDecipherRepository) ListSharedPoolAccounts(ctx context.Context, poolID, ownerID int64) ([]service.SharedPoolAccount, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+sharedPoolAccountColumns+`
		FROM shared_pool_accounts
		WHERE pool_id = $1 AND owner_id = $2 AND deleted_at IS NULL
		ORDER BY priority ASC, account_weight DESC, created_at DESC
		LIMIT 500`, poolID, ownerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.SharedPoolAccount{}
	for rows.Next() {
		account, err := scanSharedPoolAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := r.hydrateNativeSharedPoolAccounts(ctx, out, poolID, ownerID); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *bizDecipherRepository) CreateSharedPoolAccount(ctx context.Context, input service.SharedPoolAccountInput) (*service.SharedPoolAccount, error) {
	modelConfigs := sharedPoolAccountModelConfigsFromInput(input.ModelConfigs)
	row := r.db.QueryRowContext(ctx, `INSERT INTO shared_pool_accounts (
		pool_id, owner_id, name, description, provider, auth_type, upstream_base_url, upstream_api_key, credentials_encrypted, key_preview, credential_fingerprint,
		expires_at, auto_pause_on_expired, schedulable, status, status_note, disabled_reason, group_name, proxy_id, proxy_url, proxy_region, proxy_status,
		account_weight, priority, rpm_limit, account_concurrency, user_concurrency, tls_profile_id, ttl_seconds,
		cache_policy, routing_policy, model_configs, gate_required, gate_passed, full_check_score, full_check_passed, full_check_total
	) SELECT
		sp.id, sp.owner_id, $3, $4, $5, $6, $7, $8, $9, $10, $11,
		$12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22,
		$23, $24, $25, $26, $27, $28, $29,
		$30::jsonb, $31::jsonb, $32::jsonb, $33, $34, $35, $36, $37
		FROM shared_pools sp
		WHERE sp.id = $1 AND sp.owner_id = $2
		RETURNING `+sharedPoolAccountColumns,
		input.PoolID, input.OwnerID, input.Name, input.Description, input.Provider, input.AuthType, input.UpstreamBaseURL, input.UpstreamAPIKey, input.CredentialsEncrypted, keyPreview(input.UpstreamAPIKey), input.CredentialFingerprint,
		input.ExpiresAt, input.AutoPauseOnExpired, input.Schedulable, input.Status, input.StatusNote, input.DisabledReason, input.GroupName, input.ProxyID, input.ProxyURL, input.ProxyRegion, input.ProxyStatus,
		input.AccountWeight, input.Priority, input.RPMLimit, input.AccountConcurrency, input.UserConcurrency, input.TLSProfileID, input.TTLSeconds,
		string(input.CachePolicy), string(input.RoutingPolicy), string(jsonBytes(modelConfigs)), input.GateRequired, input.GatePassed, input.FullCheckScore, input.FullCheckPassed, input.FullCheckTotal,
	)
	account, err := scanSharedPoolAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPoolForbidden
	}
	return account, err
}

func (r *bizDecipherRepository) UpdateSharedPoolAccount(ctx context.Context, accountID int64, input service.SharedPoolAccountInput) (*service.SharedPoolAccount, error) {
	modelConfigs := sharedPoolAccountModelConfigsFromInput(input.ModelConfigs)
	upstreamAPIKey := strings.TrimSpace(input.UpstreamAPIKey)
	row := r.db.QueryRowContext(ctx, `UPDATE shared_pool_accounts spa SET
		name = $4,
		description = $5,
		provider = $6,
		auth_type = $7,
		upstream_base_url = $8,
		upstream_api_key = CASE WHEN $9 = '' THEN spa.upstream_api_key ELSE $9 END,
		credentials_encrypted = CASE WHEN $10 = '' THEN spa.credentials_encrypted ELSE $10 END,
		key_preview = CASE WHEN $9 = '' THEN spa.key_preview ELSE $11 END,
		credential_fingerprint = CASE WHEN $12 = '' THEN spa.credential_fingerprint ELSE $12 END,
		expires_at = COALESCE($13, spa.expires_at),
		auto_pause_on_expired = CASE WHEN $39 THEN $14 ELSE spa.auto_pause_on_expired END,
		schedulable = CASE WHEN $40 THEN $15 ELSE spa.schedulable END,
		status = $16,
		status_note = $17,
		disabled_reason = $18,
		group_name = $19,
		proxy_id = $20,
		proxy_url = $21,
		proxy_region = $22,
		proxy_status = $23,
		account_weight = $24,
		priority = $25,
		rpm_limit = $26,
		account_concurrency = $27,
		user_concurrency = $28,
		tls_profile_id = $29,
		ttl_seconds = $30,
		cache_policy = $31::jsonb,
		routing_policy = $32::jsonb,
		model_configs = $33::jsonb,
		gate_required = $34,
		gate_passed = $35,
		full_check_score = $36,
		full_check_passed = $37,
		full_check_total = $38,
		updated_at = NOW()
		FROM shared_pools sp
		WHERE spa.id = $1 AND spa.pool_id = $2 AND spa.owner_id = $3 AND spa.deleted_at IS NULL
			AND sp.id = spa.pool_id AND sp.owner_id = spa.owner_id
		RETURNING spa.`+strings.ReplaceAll(sharedPoolAccountColumns, ", ", ", spa."),
		accountID, input.PoolID, input.OwnerID,
		input.Name, input.Description, input.Provider, input.AuthType, input.UpstreamBaseURL, upstreamAPIKey, input.CredentialsEncrypted, keyPreview(upstreamAPIKey), input.CredentialFingerprint,
		input.ExpiresAt, input.AutoPauseOnExpired, input.Schedulable, input.Status, input.StatusNote, input.DisabledReason, input.GroupName, input.ProxyID, input.ProxyURL, input.ProxyRegion, input.ProxyStatus,
		input.AccountWeight, input.Priority, input.RPMLimit, input.AccountConcurrency, input.UserConcurrency, input.TLSProfileID, input.TTLSeconds,
		string(input.CachePolicy), string(input.RoutingPolicy), string(jsonBytes(modelConfigs)), input.GateRequired, input.GatePassed, input.FullCheckScore, input.FullCheckPassed, input.FullCheckTotal,
		input.AutoPauseOnExpiredSet, input.SchedulableSet,
	)
	account, err := scanSharedPoolAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPoolForbidden
	}
	return account, err
}

func (r *bizDecipherRepository) FindSharedPoolAccountByFingerprint(ctx context.Context, poolID, ownerID int64, fingerprint string) (*service.SharedPoolAccount, error) {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return nil, sql.ErrNoRows
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+sharedPoolAccountColumns+`
		FROM shared_pool_accounts
		WHERE pool_id = $1 AND owner_id = $2 AND credential_fingerprint = $3 AND deleted_at IS NULL
		LIMIT 1`, poolID, ownerID, fingerprint)
	return scanSharedPoolAccount(row)
}

func (r *bizDecipherRepository) DeleteSharedPoolAccount(ctx context.Context, poolID, accountID, ownerID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Qualify spa.* columns: shared_pools also has status/disabled_reason/updated_at.
	// Unqualified SET RHS previously caused PostgreSQL "column reference is ambiguous" -> HTTP 500.
	result, err := tx.ExecContext(ctx, `UPDATE shared_pool_accounts spa
		SET deleted_at = NOW(),
		    status = 'disabled',
		    disabled_reason = COALESCE(NULLIF(spa.disabled_reason, ''), 'deleted by pool owner'),
		    schedulable = FALSE,
		    updated_at = NOW()
		FROM shared_pools sp
		WHERE spa.id = $1 AND spa.pool_id = $2 AND spa.owner_id = $3 AND spa.deleted_at IS NULL
			AND sp.id = spa.pool_id AND sp.owner_id = spa.owner_id`, accountID, poolID, ownerID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return service.ErrPoolForbidden
	}
	if err := retireSharedPoolSupplyTx(ctx, tx, poolID, accountID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *bizDecipherRepository) CreateSharedPoolAccessKeyTx(ctx context.Context, poolID, userID int64, name, rawKey string) (*service.SharedPoolAccessKeyResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var poolName string
	var status string
	var listed bool
	var lifecycleState string
	var nativeOnboardingState string
	var maxUsers int
	var currentUsers int
	var minBalance float64
	if err := tx.QueryRowContext(ctx, `SELECT name, status, listed, lifecycle_state, max_users, current_users, min_balance_admission, native_onboarding_state FROM shared_pools WHERE id = $1 FOR UPDATE`, poolID).
		Scan(&poolName, &status, &listed, &lifecycleState, &maxUsers, &currentUsers, &minBalance, &nativeOnboardingState); err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrPoolNotJoinable
		}
		return nil, err
	}
	if nativeOnboardingState != "" && nativeOnboardingState != service.SharedPoolOnboardingLegacyExisting && nativeOnboardingState != service.SharedPoolOnboardingBillingActive {
		return nil, service.ErrBillingActivationRequired
	}
	if !listed || lifecycleState != "operating" || (status != "healthy" && status != "limited") {
		return nil, service.ErrPoolNotJoinable
	}
	var seatID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM pool_seat_bindings WHERE pool_id = $1 AND user_id = $2 AND status = 'active' FOR UPDATE`, poolID, userID).Scan(&seatID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPoolSeatRequired
		}
		return nil, err
	}
	var modelsRaw []byte
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(model_name ORDER BY sort_order, id), '[]'::jsonb) FROM shared_pool_models WHERE pool_id = $1 AND enabled = TRUE AND model_open = TRUE`, poolID).Scan(&modelsRaw); err != nil {
		return nil, err
	}
	var existing service.SharedPoolAccessKey
	var existingRawKey string
	var existingModelsRaw []byte
	if err := tx.QueryRowContext(ctx, `WITH existing AS (
			SELECT sak.id
			FROM shared_pool_access_keys sak JOIN api_keys ak ON ak.id = sak.api_key_id
			WHERE sak.pool_id = $1 AND sak.user_id = $2 AND sak.status = 'active' AND ak.deleted_at IS NULL
			ORDER BY sak.created_at ASC, sak.id ASC
			LIMIT 1
		)
		UPDATE shared_pool_access_keys sak SET
			account_mode = TRUE,
			allowed_models = $3::jsonb,
			updated_at = CASE
				WHEN sak.account_mode = TRUE AND sak.allowed_models = $3::jsonb THEN sak.updated_at
				ELSE NOW()
			END
		FROM existing, shared_pools sp, api_keys ak
		WHERE sak.id = existing.id AND sp.id = sak.pool_id AND ak.id = sak.api_key_id
		RETURNING sak.id, sak.pool_id, sp.name, sak.user_id, sak.api_key_id, sak.name, ak.key, sak.status, sak.allowed_models, sak.total_used, sak.last_used_at, sak.created_at`, poolID, userID, string(modelsRaw)).
		Scan(&existing.ID, &existing.PoolID, &existing.PoolName, &existing.UserID, &existing.APIKeyID, &existing.Name, &existingRawKey, &existing.Status, &existingModelsRaw, &existing.TotalUsed, &existing.LastUsedAt, &existing.CreatedAt); err == nil {
		existing.KeyPreview = keyPreview(existingRawKey)
		existing.AllowedModels = scanStringArray(existingModelsRaw)
		existing.AccountMode = true
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &service.SharedPoolAccessKeyResult{AccessKey: existing, AlreadyHeld: true}, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if maxUsers > 0 && currentUsers > maxUsers {
		return nil, service.ErrPoolFull
	}
	if minBalance > 0 {
		var balance float64
		if err := tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1`, userID).Scan(&balance); err != nil {
			return nil, err
		}
		if balance < minBalance {
			return nil, service.ErrPoolInsufficientBalance
		}
	}

	lockRows, err := tx.QueryContext(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(-5408070899302829231)^userID)
	if err != nil {
		return nil, err
	}
	_ = lockRows.Close()

	var apiKeyID int64
	var apiKeyRaw string
	newAPIKey := false
	if err := tx.QueryRowContext(ctx, `SELECT id, key FROM api_keys
		WHERE user_id = $1 AND status = 'active' AND deleted_at IS NULL AND group_id IS NULL AND key LIKE 'sk-share-%'
		ORDER BY created_at ASC, id ASC
		LIMIT 1
		FOR UPDATE`, userID).Scan(&apiKeyID, &apiKeyRaw); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if err := tx.QueryRowContext(ctx, `INSERT INTO api_keys (user_id, key, name, group_id, status) VALUES ($1, $2, $3, NULL, 'active') RETURNING id, key`, userID, rawKey, name).Scan(&apiKeyID, &apiKeyRaw); err != nil {
			return nil, err
		}
		newAPIKey = true
	}

	row := tx.QueryRowContext(ctx, `UPDATE shared_pool_access_keys sak SET
			name = $4,
			status = 'active',
			allowed_models = $5::jsonb,
			account_mode = TRUE,
			updated_at = NOW()
		WHERE sak.id = (
			SELECT id FROM shared_pool_access_keys
			WHERE pool_id = $1 AND user_id = $2 AND api_key_id = $3
			ORDER BY created_at ASC, id ASC
			LIMIT 1
		)
		RETURNING sak.id, sak.pool_id, $6::text, sak.user_id, sak.api_key_id, sak.name, $7::text, sak.status, sak.allowed_models, sak.total_used, sak.last_used_at, sak.created_at`,
		poolID, userID, apiKeyID, name, string(modelsRaw), poolName, keyPreview(apiKeyRaw),
	)
	accessKey, err := scanSharedPoolAccessKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		row = tx.QueryRowContext(ctx, `INSERT INTO shared_pool_access_keys (pool_id, user_id, api_key_id, name, allowed_models, account_mode)
		VALUES ($1, $2, $3, $4, $5::jsonb, TRUE)
		RETURNING id, pool_id, $6::text, user_id, api_key_id, name, $7::text, status, allowed_models, total_used, last_used_at, created_at`,
			poolID, userID, apiKeyID, name, string(modelsRaw), poolName, keyPreview(apiKeyRaw),
		)
		accessKey, err = scanSharedPoolAccessKey(row)
	}
	if err != nil {
		return nil, err
	}
	accessKey.AccountMode = true
	if newAPIKey {
		accessKey.Key = rawKey
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.SharedPoolAccessKeyResult{AccessKey: *accessKey}, nil
}

func (r *bizDecipherRepository) ListMySharedPoolAccessKeys(ctx context.Context, userID int64) ([]service.SharedPoolAccessKey, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT sak.id, sak.pool_id, sp.name, sak.user_id, sak.api_key_id, sak.name, ak.key, sak.status, sak.allowed_models, sak.total_used, sak.last_used_at, sak.created_at, sak.account_mode
		FROM shared_pool_access_keys sak JOIN shared_pools sp ON sp.id = sak.pool_id JOIN api_keys ak ON ak.id = sak.api_key_id
		WHERE sak.user_id = $1 AND ak.deleted_at IS NULL ORDER BY sak.created_at DESC LIMIT 200`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.SharedPoolAccessKey{}
	for rows.Next() {
		var rawKey string
		var k service.SharedPoolAccessKey
		var modelsRaw []byte
		if err := rows.Scan(&k.ID, &k.PoolID, &k.PoolName, &k.UserID, &k.APIKeyID, &k.Name, &rawKey, &k.Status, &modelsRaw, &k.TotalUsed, &k.LastUsedAt, &k.CreatedAt, &k.AccountMode); err != nil {
			return nil, err
		}
		k.Key = rawKey
		k.KeyPreview = keyPreview(rawKey)
		k.AllowedModels = scanStringArray(modelsRaw)
		out = append(out, k)
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) GetSharedPoolAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel string) (*service.SharedPoolAccessKey, error) {
	return r.getSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel, "", false)
}

// GetSharedPoolCompactAccessKeyByAPIKeyID uses the ordinary model/price
// eligibility graph, then restricts account scheduling to OAuth credentials
// with current, model-bound Codex compact evidence.
func (r *bizDecipherRepository) GetSharedPoolCompactAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel string) (*service.SharedPoolAccessKey, error) {
	if strings.TrimSpace(reqModel) == "" {
		return nil, nil
	}
	return r.getSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel, "", true)
}

func (r *bizDecipherRepository) GetSharedPoolMediaAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel, endpointType string) (*service.SharedPoolAccessKey, error) {
	endpointType = strings.ToLower(strings.TrimSpace(endpointType))
	switch endpointType {
	case service.SharedPoolEndpointImageGeneration, service.SharedPoolEndpointImageEdit, service.SharedPoolEndpointVideo:
	default:
		return nil, nil
	}
	if strings.TrimSpace(reqModel) == "" {
		return nil, nil
	}
	return r.getSharedPoolAccessKeyByAPIKeyID(ctx, apiKeyID, reqModel, endpointType, false)
}

func (r *bizDecipherRepository) getSharedPoolAccessKeyByAPIKeyID(ctx context.Context, apiKeyID int64, reqModel, mediaEndpointType string, compactRequest bool) (*service.SharedPoolAccessKey, error) {
	args := []any{apiKeyID}
	excludedAccountIDs, excludedAccessKeyIDs := service.SharedPoolRouteExclusions(ctx)
	accountExclusionFilter := ""
	if len(excludedAccountIDs) > 0 {
		placeholders := make([]string, 0, len(excludedAccountIDs))
		for _, accountID := range excludedAccountIDs {
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
			args = append(args, accountID)
		}
		accountExclusionFilter = "\n\t\t\tAND spa.id NOT IN (" + strings.Join(placeholders, ", ") + ")"
	}
	accessKeyExclusionFilter := ""
	if len(excludedAccessKeyIDs) > 0 {
		placeholders := make([]string, 0, len(excludedAccessKeyIDs))
		for _, accessKeyID := range excludedAccessKeyIDs {
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
			args = append(args, accessKeyID)
		}
		accessKeyExclusionFilter = "\n\t\t\tAND sak.id NOT IN (" + strings.Join(placeholders, ", ") + ")"
	}
	requestedModel := strings.TrimSpace(reqModel)
	modelFilter := ""
	modelRateJoin := ""
	mediaCredentialFilter := ""
	compactCredentialFilter := ""
	if compactRequest {
		compactCredentialFilter = `
			AND sp.account_mode_enabled = TRUE
			AND spa_req.id IS NOT NULL`
	}
	accountJoin := `LEFT JOIN LATERAL (
		SELECT spa.id, spa.auth_type, spa.upstream_base_url, spa.upstream_api_key, spa.credentials_encrypted, spa.expires_at, spa.proxy_url, spa.account_concurrency, spa.user_concurrency,
			spa.account_weight, spa.priority, 0 AS model_concurrency, ''::text AS upstream_model_name
		FROM shared_pool_accounts spa
		WHERE spa.pool_id = sp.id
			AND sp.account_mode_enabled = TRUE
			` + accountExclusionFilter + `
			AND spa.deleted_at IS NULL
			AND spa.status IN ('active', 'limited', 'testing')
			AND spa.schedulable = TRUE
			AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE))
			AND (
				(spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '')
				OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> '')
			)
			AND (spa.gate_required = FALSE OR spa.gate_passed = TRUE)
		ORDER BY spa.priority ASC, spa.full_check_score DESC, spa.account_weight DESC, spa.total_calls ASC, spa.last_used_at ASC NULLS FIRST, spa.id ASC
		LIMIT 1
	) spa_req ON TRUE`
	modelConcurrencySelect := "0 AS model_concurrency"
	publishedModelSelect := "'' AS published_model_name"
	upstreamModelSelect := "'' AS upstream_model_name"
	canonicalModelSelect := "'' AS canonical_model_name"
	providerSelect := "'' AS provider"
	effectiveRateSelect := sharedPoolEffectiveRateMultiplierSQL("0", "sp.rate_multiplier") + " AS effective_rate_multiplier"
	if requestedModel != "" {
		args = append(args, requestedModel)
		modelArg := len(args)
		mediaModelSelect := ""
		mediaModelJoin := ""
		mediaAccountFilter := ""
		if mediaEndpointType != "" {
			args = append(args, mediaEndpointType)
			endpointArg := len(args)
			mediaModelSelect = `,
				COALESCE(media_probe.account_id, 0) AS media_probe_account_id,
				COALESCE(media_probe.account_config_version, 0) AS media_probe_account_config_version`
			mediaModelJoin = fmt.Sprintf(`JOIN shared_pool_model_endpoints media_endpoint
				ON media_endpoint.pool_model_id = spm.id
			   AND media_endpoint.endpoint_type = $%d
			   AND media_endpoint.enabled = TRUE
			   AND media_endpoint.gate_status = 'passed'
			   AND media_endpoint.media_probe_expires_at > NOW()
			JOIN shared_pool_media_endpoint_probes media_probe
				ON media_probe.id = media_endpoint.last_media_probe_id
			   AND media_probe.endpoint_id = media_endpoint.id
			   AND media_probe.pool_id = sp.id
			   AND media_probe.endpoint_type = media_endpoint.endpoint_type
			   AND media_probe.pool_config_version = sp.config_version
			   AND media_probe.result_status = 'passed'
			   AND media_probe.output_observed = TRUE
			   AND (media_probe.endpoint_type <> 'video' OR media_probe.async_terminal_observed = TRUE)
			   AND media_probe.expires_at > NOW()
			   AND (
				spm.pricing_source = 'official_catalog'
				OR (
					spm.pricing_source = 'owner_custom'
					AND spm.pricing_status = 'ready'
					AND media_endpoint.pricing_status = 'ready'
					AND EXISTS (
						SELECT 1
						FROM shared_pool_price_versions media_price
						WHERE media_price.pool_id = sp.id
						  AND media_price.pool_model_id = spm.id
						  AND media_price.endpoint_id = media_endpoint.id
						  AND media_price.source = 'owner_custom'
						  AND media_price.effective_from <= NOW()
					)
				)
			)`, endpointArg)
			mediaAccountFilter = `
				AND spa.id = NULLIF(spm_req.media_probe_account_id, 0)
				AND spa.config_version = spm_req.media_probe_account_config_version`
			mediaCredentialFilter = `
			AND (
				(sp.account_mode_enabled = TRUE
				 AND spm_req.media_probe_account_id > 0
				 AND spa_req.id = spm_req.media_probe_account_id)
				OR (sp.account_mode_enabled = FALSE
				    AND spm_req.media_probe_account_id = 0
				    AND spm_req.media_probe_account_config_version = 0)
			)`
		}
		// spm_req below performs the model/alias/protection lookup once and is also
		// the source of pricing/concurrency metadata. Requiring a non-null result
		// avoids repeating the same JSONB and protection-window scan in WHERE.
		modelFilter = " AND spm_req.model_name IS NOT NULL"
		modelRateJoin = fmt.Sprintf(`LEFT JOIN LATERAL (
			SELECT spm.model_name, spm.provider, spm.max_concurrency, spm.rate_multiplier, NULLIF(TRIM(spm.upstream_model_name), '') AS upstream_model_name`+mediaModelSelect+`
			FROM shared_pool_models spm
			`+mediaModelJoin+`
			WHERE spm.pool_id = sp.id
				AND spm.enabled = TRUE
				AND spm.model_open = TRUE
				AND (
					spm.model_name = $%d
					OR spm.model_aliases ? $%d
					OR EXISTS (
						SELECT 1
						FROM shared_pool_accounts spa_alias
						JOIN LATERAL (
							SELECT 1
							FROM jsonb_to_recordset(spa_alias.model_configs) AS cfg(model_name text, model_open boolean, aliases jsonb)
							WHERE COALESCE(cfg.model_open, TRUE) = TRUE
								AND cfg.model_name = spm.model_name
								AND COALESCE(cfg.aliases, '[]'::jsonb) ? $%d
							LIMIT 1
						) cfg ON TRUE
						WHERE spa_alias.pool_id = sp.id
							AND spa_alias.deleted_at IS NULL
							AND spa_alias.status IN ('active', 'limited', 'testing')
							AND spa_alias.schedulable = TRUE
							AND (spa_alias.expires_at IS NULL OR spa_alias.expires_at > NOW() OR (spa_alias.auth_type <> 'oauth' AND spa_alias.auto_pause_on_expired = FALSE))
							AND ((spa_alias.auth_type IN ('apikey', 'api_key') AND btrim(spa_alias.upstream_base_url) <> '' AND btrim(spa_alias.upstream_api_key) <> '') OR (spa_alias.auth_type = 'oauth' AND btrim(spa_alias.credentials_encrypted) <> ''))
							AND (spa_alias.gate_required = FALSE OR spa_alias.gate_passed = TRUE)
					)
				)
				AND NOT EXISTS (
					SELECT 1 FROM shared_pool_model_usage_windows w5
					WHERE w5.pool_id = sp.id
						AND w5.model_name = spm.model_name
						AND w5.window_type = '5h'
						AND w5.window_start >= NOW() - INTERVAL '5 hours'
						AND spm.five_hour_protection_percent < 100
					GROUP BY w5.pool_id, w5.model_name
					HAVING COALESCE(SUM(w5.cost), 0) >= spm.five_hour_protection_percent / 100.0
				)
				AND NOT EXISTS (
					SELECT 1 FROM shared_pool_model_usage_windows w1d
					WHERE w1d.pool_id = sp.id
						AND w1d.model_name = spm.model_name
						AND w1d.window_type = '1d'
						AND w1d.window_start >= NOW() - INTERVAL '1 day'
						AND spm.daily_protection_percent < 100
					GROUP BY w1d.pool_id, w1d.model_name
					HAVING COALESCE(SUM(w1d.cost), 0) >= spm.daily_protection_percent / 100.0
				)
				AND NOT EXISTS (
					SELECT 1 FROM shared_pool_model_usage_windows w7d
					WHERE w7d.pool_id = sp.id
						AND w7d.model_name = spm.model_name
						AND w7d.window_type = '7d'
						AND w7d.window_start >= NOW() - INTERVAL '7 days'
						AND spm.seven_day_protection_percent < 100
					GROUP BY w7d.pool_id, w7d.model_name
					HAVING COALESCE(SUM(w7d.cost), 0) >= spm.seven_day_protection_percent / 100.0
				)
			ORDER BY spm.sort_order ASC
			LIMIT 1
		) spm_req ON TRUE`, modelArg, modelArg, modelArg)
		compactAccountFilter := ""
		if compactRequest {
			compactAccountFilter = sharedPoolOAuthCompactAccountFilterSQL()
		}
		accountJoin = fmt.Sprintf(`LEFT JOIN LATERAL (
			SELECT spa.id, spa.auth_type, spa.upstream_base_url, spa.upstream_api_key, spa.credentials_encrypted, spa.expires_at, spa.proxy_url, spa.account_concurrency, spa.user_concurrency,
				spa.account_weight, spa.priority, COALESCE(cfg.max_concurrency, 0) AS model_concurrency,
				COALESCE(NULLIF(TRIM(cfg.upstream_model_name), ''), '') AS upstream_model_name
			FROM shared_pool_accounts spa
			JOIN LATERAL (
				SELECT cfg.model_name, cfg.upstream_model_name, cfg.max_concurrency, cfg.model_open, cfg.aliases
				FROM jsonb_to_recordset(spa.model_configs) AS cfg(model_name text, upstream_model_name text, max_concurrency int, model_open boolean, aliases jsonb)
				WHERE COALESCE(cfg.model_open, TRUE) = TRUE
					AND (
						cfg.model_name = COALESCE(spm_req.model_name, $%d)
						OR COALESCE(cfg.aliases, '[]'::jsonb) ? COALESCE(spm_req.model_name, $%d)
						OR cfg.model_name = $%d
						OR COALESCE(cfg.aliases, '[]'::jsonb) ? $%d
					)
				LIMIT 1
			) cfg ON TRUE
			WHERE spa.pool_id = sp.id
				AND sp.account_mode_enabled = TRUE
				`+accountExclusionFilter+`
				`+mediaAccountFilter+`
				AND spa.deleted_at IS NULL
				AND spa.status IN ('active', 'limited', 'testing')
				AND spa.schedulable = TRUE
				AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE))
				AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> ''))
				AND (spa.gate_required = FALSE OR spa.gate_passed = TRUE)
				`+compactAccountFilter+`
			ORDER BY spa.priority ASC, spa.full_check_score DESC, spa.account_weight DESC, spa.total_calls ASC, spa.last_used_at ASC NULLS FIRST, spa.id ASC
			LIMIT 1
		) spa_req ON TRUE`, modelArg, modelArg, modelArg, modelArg)
		modelConcurrencySelect = "COALESCE(NULLIF(spa_req.model_concurrency, 0), COALESCE(spm_req.max_concurrency, 0)) AS model_concurrency"
		publishedModelSelect = "COALESCE(spm_req.model_name, '') AS published_model_name"
		upstreamModelSelect = fmt.Sprintf("COALESCE(NULLIF(spa_req.upstream_model_name, ''), spm_req.upstream_model_name, CASE WHEN spm_req.model_name <> $%d THEN spm_req.model_name ELSE '' END, '') AS upstream_model_name", modelArg)
		if compactRequest {
			upstreamModelSelect = "COALESCE(NULLIF(spa_req.upstream_model_name, ''), spm_req.upstream_model_name, spm_req.model_name, '') AS upstream_model_name"
		}
		canonicalModelSelect = `COALESCE((
			SELECT mc.model_name
			FROM model_catalog mc
			WHERE mc.enabled = TRUE
			  AND LOWER(mc.provider) = ` + sharedPoolCanonicalProviderSQL("spm_req.provider") + `
			  AND (
				LOWER(mc.model_name) = LOWER(spm_req.model_name)
				OR EXISTS (
					SELECT 1
					FROM jsonb_array_elements_text(
						CASE WHEN jsonb_typeof(mc.aliases) = 'array' THEN mc.aliases ELSE '[]'::jsonb END
					) AS official_alias(value)
					WHERE LOWER(official_alias.value) = LOWER(spm_req.model_name)
				)
			  )
			ORDER BY CASE WHEN LOWER(mc.model_name) = LOWER(spm_req.model_name) THEN 0 ELSE 1 END,
			         mc.sort_order, mc.id
			LIMIT 1
		), '') AS canonical_model_name`
		providerSelect = "COALESCE(spm_req.provider, '') AS provider"
		// User pricing is owned by the pool model, then the pool-wide default.
		// Account JSON may describe upstream routing but must never randomize the
		// price according to whichever account the scheduler happens to select.
		effectiveRateSelect = sharedPoolEffectiveRateMultiplierSQL("spm_req.rate_multiplier", "sp.rate_multiplier") + " AS effective_rate_multiplier"
	}
	rows, err := r.db.QueryContext(ctx, `SELECT sak.id, sp.id, sp.name, sak.user_id, sak.api_key_id, sak.name, ak.key, sak.status, sak.allowed_models, sak.total_used, sak.last_used_at, sak.created_at,
		sp.owner_id, COALESCE(spa_req.id, 0) AS shared_pool_account_id,
		COALESCE(NULLIF(spa_req.auth_type, ''), 'apikey') AS auth_type,
		CASE
			WHEN spa_req.auth_type = 'oauth' THEN COALESCE(spa_req.upstream_base_url, '')
			ELSE COALESCE(NULLIF(spa_req.upstream_base_url, ''), sp.upstream_base_url)
		END AS upstream_base_url,
		CASE
			WHEN spa_req.auth_type = 'oauth' THEN ''
			ELSE COALESCE(NULLIF(spa_req.upstream_api_key, ''), sp.upstream_api_key)
		END AS upstream_api_key,
		COALESCE(spa_req.credentials_encrypted, '') AS credentials_encrypted,
		spa_req.expires_at,
		COALESCE(NULLIF(spa_req.proxy_url, ''), sp.proxy_url) AS proxy_url,
		sp.rate_multiplier, sp.owner_share_percent,
		COALESCE(NULLIF(spa_req.account_concurrency, 0), sp.account_concurrency) AS account_concurrency,
		COALESCE(NULLIF(spa_req.user_concurrency, 0), sp.user_concurrency) AS user_concurrency,
		`+modelConcurrencySelect+`, sak.account_mode,
		`+effectiveRateSelect+`, `+publishedModelSelect+`, `+upstreamModelSelect+`, `+canonicalModelSelect+`, `+providerSelect+`
		FROM shared_pool_access_keys sak
		JOIN api_keys ak ON ak.id = sak.api_key_id
		JOIN pool_seat_bindings psb_key ON psb_key.pool_id = sak.pool_id AND psb_key.user_id = sak.user_id AND psb_key.status = 'active'
		JOIN shared_pools sp ON sp.id = sak.pool_id
		`+modelRateJoin+`
		`+accountJoin+`
		WHERE sak.api_key_id = $1
			`+accessKeyExclusionFilter+`
			AND sak.status = 'active'
			AND ak.status = 'active'
			AND ak.deleted_at IS NULL
			AND sak.account_mode = TRUE
			AND sp.listed = TRUE
			AND sp.lifecycle_state = 'operating'
			AND sp.governance_status NOT IN ('banned', 'suppressed')
			AND sp.status IN ('healthy', 'limited')
			AND (
				(sp.account_mode_enabled = TRUE AND spa_req.id IS NOT NULL)
				OR (
					sp.account_mode_enabled = FALSE
					AND btrim(sp.upstream_base_url) <> ''
					AND btrim(sp.upstream_api_key) <> ''
				)
			)`+modelFilter+mediaCredentialFilter+compactCredentialFilter+`
		ORDER BY sp.rank_weight DESC, sp.quality_score DESC, sp.today_availability DESC, effective_rate_multiplier ASC, COALESCE(spa_req.priority, 100) ASC, COALESCE(spa_req.account_weight, 1) DESC, sp.total_calls ASC
		LIMIT 1`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	accessKey, err := scanSharedPoolAccessKeyRuntime(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if requestedModel == "" {
		allowedModels, err := r.listEligibleSharedPoolAllowedModels(ctx, apiKeyID)
		if err != nil {
			return nil, err
		}
		accessKey.AllowedModels = allowedModels
	} else {
		accessKey.AllowedModels = mergeSharedPoolAllowedModels(accessKey.AllowedModels, accessKey.PublishedModelName, requestedModel)
	}
	// Touch seat activity when a real request authenticates against an active seat.
	// This keeps long-running requests from being auto-released mid-flight.
	if accessKey != nil && accessKey.PoolID > 0 && accessKey.UserID > 0 {
		if _, err := r.db.ExecContext(ctx, `UPDATE pool_seat_bindings
			SET last_activity_at = NOW(), updated_at = NOW()
			WHERE pool_id = $1 AND user_id = $2 AND status = 'active'
			  AND (last_activity_at IS NULL OR last_activity_at < NOW() - INTERVAL '30 seconds')`, accessKey.PoolID, accessKey.UserID); err != nil {
			return nil, err
		}
	}
	return accessKey, nil
}

func mergeSharedPoolAllowedModels(models []string, extraModels ...string) []string {
	seen := make(map[string]struct{}, len(models)+len(extraModels))
	out := make([]string, 0, len(models)+len(extraModels))
	add := func(model string) {
		model = strings.TrimSpace(model)
		if model == "" {
			return
		}
		key := strings.ToLower(model)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, model)
	}
	for _, model := range models {
		add(model)
	}
	for _, model := range extraModels {
		add(model)
	}
	return out
}

func (r *bizDecipherRepository) listEligibleSharedPoolAllowedModels(ctx context.Context, apiKeyID int64) ([]string, error) {
	var modelsRaw []byte
	if err := r.db.QueryRowContext(ctx, `WITH eligible_bindings AS (
		SELECT
			sp.id AS pool_id,
			sp.upstream_base_url,
			sp.upstream_api_key,
			ROW_NUMBER() OVER (
				ORDER BY sp.rank_weight DESC, sp.quality_score DESC, sp.today_availability DESC, sp.rate_multiplier ASC,
					COALESCE(spa_req.priority, 100) ASC, COALESCE(spa_req.account_weight, 1) DESC, sp.total_calls ASC, sp.id ASC
			) AS pool_order
		FROM shared_pool_access_keys sak
		JOIN api_keys ak ON ak.id = sak.api_key_id
		JOIN pool_seat_bindings psb_key ON psb_key.pool_id = sak.pool_id AND psb_key.user_id = sak.user_id AND psb_key.status = 'active'
		JOIN shared_pools sp ON sp.id = sak.pool_id
		LEFT JOIN LATERAL (
			SELECT spa.id, spa.account_weight, spa.priority
			FROM shared_pool_accounts spa
			WHERE spa.pool_id = sp.id
				AND sp.account_mode_enabled = TRUE
				AND spa.deleted_at IS NULL
				AND spa.status IN ('active', 'limited', 'testing')
				AND spa.schedulable = TRUE
				AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE))
				AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> ''))
				AND (spa.gate_required = FALSE OR spa.gate_passed = TRUE)
			ORDER BY spa.priority ASC, spa.full_check_score DESC, spa.account_weight DESC, spa.total_calls ASC, spa.last_used_at ASC NULLS FIRST, spa.id ASC
			LIMIT 1
		) spa_req ON TRUE
		WHERE sak.api_key_id = $1
			AND sak.status = 'active'
			AND ak.status = 'active'
			AND ak.deleted_at IS NULL
			AND sak.account_mode = TRUE
			AND sp.listed = TRUE
			AND sp.lifecycle_state = 'operating'
			AND sp.governance_status NOT IN ('banned', 'suppressed')
			AND sp.status IN ('healthy', 'limited')
			AND (
				(sp.account_mode_enabled = TRUE AND spa_req.id IS NOT NULL)
				OR (
					sp.account_mode_enabled = FALSE
					AND btrim(sp.upstream_base_url) <> ''
					AND btrim(sp.upstream_api_key) <> ''
				)
			)
	), eligible_models AS (
		SELECT spm.model_name, eb.pool_order, spm.sort_order, spm.id AS model_id
		FROM eligible_bindings eb
		JOIN shared_pool_models spm ON spm.pool_id = eb.pool_id
		WHERE spm.enabled = TRUE
			AND spm.model_open = TRUE
			AND btrim(spm.model_name) <> ''
			AND NOT EXISTS (
				SELECT 1 FROM shared_pool_model_usage_windows w5
				WHERE w5.pool_id = eb.pool_id
					AND w5.model_name = spm.model_name
					AND w5.window_type = '5h'
					AND w5.window_start >= NOW() - INTERVAL '5 hours'
					AND spm.five_hour_protection_percent < 100
				GROUP BY w5.pool_id, w5.model_name
				HAVING COALESCE(SUM(w5.cost), 0) >= spm.five_hour_protection_percent / 100.0
			)
			AND NOT EXISTS (
				SELECT 1 FROM shared_pool_model_usage_windows w1d
				WHERE w1d.pool_id = eb.pool_id
					AND w1d.model_name = spm.model_name
					AND w1d.window_type = '1d'
					AND w1d.window_start >= NOW() - INTERVAL '1 day'
					AND spm.daily_protection_percent < 100
				GROUP BY w1d.pool_id, w1d.model_name
				HAVING COALESCE(SUM(w1d.cost), 0) >= spm.daily_protection_percent / 100.0
			)
			AND NOT EXISTS (
				SELECT 1 FROM shared_pool_model_usage_windows w7d
				WHERE w7d.pool_id = eb.pool_id
					AND w7d.model_name = spm.model_name
					AND w7d.window_type = '7d'
					AND w7d.window_start >= NOW() - INTERVAL '7 days'
					AND spm.seven_day_protection_percent < 100
				GROUP BY w7d.pool_id, w7d.model_name
				HAVING COALESCE(SUM(w7d.cost), 0) >= spm.seven_day_protection_percent / 100.0
			)
			AND (
				EXISTS (
					SELECT 1
					FROM shared_pools sp_mode
					JOIN shared_pool_accounts spa_model ON spa_model.pool_id = sp_mode.id
					JOIN LATERAL (
						SELECT 1
						FROM jsonb_to_recordset(spa_model.model_configs) AS cfg(model_name text, model_open boolean, aliases jsonb)
						WHERE COALESCE(cfg.model_open, TRUE) = TRUE
							AND (cfg.model_name = spm.model_name OR COALESCE(cfg.aliases, '[]'::jsonb) ? spm.model_name)
						LIMIT 1
					) cfg ON TRUE
					WHERE sp_mode.id = eb.pool_id
						AND sp_mode.account_mode_enabled = TRUE
						AND spa_model.deleted_at IS NULL
						AND spa_model.status IN ('active', 'limited', 'testing')
						AND spa_model.schedulable = TRUE
						AND (spa_model.expires_at IS NULL OR spa_model.expires_at > NOW() OR (spa_model.auth_type <> 'oauth' AND spa_model.auto_pause_on_expired = FALSE))
						AND ((spa_model.auth_type IN ('apikey', 'api_key') AND btrim(spa_model.upstream_base_url) <> '' AND btrim(spa_model.upstream_api_key) <> '') OR (spa_model.auth_type = 'oauth' AND btrim(spa_model.credentials_encrypted) <> ''))
						AND (spa_model.gate_required = FALSE OR spa_model.gate_passed = TRUE)
				)
				OR (
					EXISTS (
						SELECT 1 FROM shared_pools sp_mode
						WHERE sp_mode.id = eb.pool_id
							AND sp_mode.account_mode_enabled = FALSE
					)
					AND btrim(eb.upstream_base_url) <> ''
					AND btrim(eb.upstream_api_key) <> ''
				)
			)
	), deduped AS (
		SELECT DISTINCT ON (model_name) model_name, pool_order, sort_order, model_id
		FROM eligible_models
		ORDER BY model_name, pool_order, sort_order, model_id
	)
	SELECT COALESCE(jsonb_agg(model_name ORDER BY pool_order, sort_order, model_id), '[]'::jsonb)
	FROM deduped`, apiKeyID).Scan(&modelsRaw); err != nil {
		return nil, err
	}
	return scanStringArray(modelsRaw), nil
}

func (r *bizDecipherRepository) ReportSharedPoolTx(ctx context.Context, poolID, userID int64, reason string) (*service.SharedPool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `INSERT INTO shared_pool_complaints (pool_id, user_id, reason, status)
		VALUES ($1, $2, $3, 'open')
		ON CONFLICT (pool_id, user_id) DO UPDATE SET reason = EXCLUDED.reason, status = 'open', updated_at = NOW()`, poolID, userID, reason); err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `UPDATE shared_pools
		SET complaint_count = (SELECT COUNT(*) FROM shared_pool_complaints WHERE pool_id = $1 AND status = 'open'),
			quality_score = GREATEST(quality_score - 3, 0),
			rank_weight = rank_weight - 5,
			updated_at = NOW()
		WHERE id = $1
		RETURNING `+sharedPoolColumns, poolID)
	pool, err := scanSharedPool(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPoolNotJoinable
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return pool, nil
}

func (r *bizDecipherRepository) LikeSharedPoolTx(ctx context.Context, poolID, userID int64) (*service.SharedPool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM shared_pools WHERE id = $1 AND listed = TRUE AND governance_status NOT IN ('banned', 'suppressed') AND lifecycle_state <> 'archived')`, poolID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, service.ErrPoolNotJoinable
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO shared_pool_likes (pool_id, user_id) VALUES ($1, $2) ON CONFLICT (pool_id, user_id) DO NOTHING`, poolID, userID); err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `UPDATE shared_pools
		SET like_count = (SELECT COUNT(*) FROM shared_pool_likes WHERE pool_id = $1),
			updated_at = NOW()
		WHERE id = $1
		RETURNING `+sharedPoolColumns, poolID)
	pool, err := scanSharedPool(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPoolNotJoinable
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return pool, nil
}

func (r *bizDecipherRepository) UnlikeSharedPoolTx(ctx context.Context, poolID, userID int64) (*service.SharedPool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM shared_pool_likes WHERE pool_id = $1 AND user_id = $2`, poolID, userID); err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `UPDATE shared_pools
		SET like_count = (SELECT COUNT(*) FROM shared_pool_likes WHERE pool_id = $1),
			updated_at = NOW()
		WHERE id = $1
		RETURNING `+sharedPoolColumns, poolID)
	pool, err := scanSharedPool(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPoolNotJoinable
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return pool, nil
}

func (r *bizDecipherRepository) ListSharedPoolLikedIDs(ctx context.Context, userID int64, poolIDs []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if userID <= 0 || len(poolIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT pool_id FROM shared_pool_likes WHERE user_id = $1 AND pool_id = ANY($2::bigint[])`, userID, pq.Array(poolIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var poolID int64
		if err := rows.Scan(&poolID); err != nil {
			return nil, err
		}
		out[poolID] = true
	}
	return out, rows.Err()
}

func (r *bizDecipherRepository) RecordSharedPoolUsageTx(ctx context.Context, input service.SharedPoolUsageInput) error {
	sourceID := strings.TrimSpace(input.RequestID)
	if input.Success && sourceID == "" {
		return errors.New("successful shared pool usage requires a request id")
	}
	if input.AccessKeyID <= 0 || input.PoolID <= 0 || input.UserID <= 0 {
		return nil
	}
	priceSnapshot, err := sharedPoolJSONBText(input.PriceSnapshot, "shared pool price snapshot")
	if err != nil {
		return err
	}
	input.PriceSnapshot = json.RawMessage(priceSnapshot)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := declareSharedPoolLegacyRuntimeEpochTx(ctx, tx); err != nil {
		return err
	}

	reportedCost := input.Cost
	reservation, err := lockSharedPoolUsageReservationForSettlement(ctx, tx, input)
	if err != nil {
		return err
	}
	if reservation != nil {
		// Reservation-backed settlement is authoritative: caller data is checked
		// above, but all persisted route/price metadata comes from the immutable
		// row accepted before forwarding.
		input.AccountID = reservation.AccountID.Int64
		input.PriceVersionID = reservation.PriceVersionID.Int64
		input.Model = reservation.Model
		input.PricingSource = reservation.PricingSource
		priceSnapshot = string(reservation.PriceSnapshot)
		switch reservation.Status {
		case "settled", "released":
			// A final reservation proves this request was already accounted for.
			// Do not increment usage counters or owner earnings twice.
			return tx.Commit()
		case "reserved":
			if input.Success {
				return errors.New("shared pool reservation was not marked forwarding before successful settlement")
			}
		case "forwarding":
		case "settlement_pending":
			if !input.Success {
				return errors.New("pending shared pool settlement cannot be released as an upstream failure")
			}
		default:
			return fmt.Errorf("unsupported shared pool reservation status %q", reservation.Status)
		}
		if !input.Success {
			if reservation.Status == "forwarding" {
				// A transport failure after the forwarding boundary cannot prove that
				// the provider performed no work. Keep the pre-authorized amount frozen
				// and require an audited resolution instead of minting a free request.
				result, err := tx.ExecContext(ctx, `
					UPDATE shared_pool_usage_reservations
					SET status = 'review_required',
					    failure_reason = 'upstream_result_unknown',
					    finalized_at = NOW(),
					    updated_at = NOW()
					WHERE id = $1 AND status = 'forwarding'`, reservation.ID)
				if err != nil {
					return err
				}
				affected, err := result.RowsAffected()
				if err != nil {
					return err
				}
				if affected != 1 {
					return service.ErrSharedPoolReservationFinalized
				}
			} else if err := releaseSharedPoolUsageReservationTx(ctx, tx, reservation, "upstream_request_failed_before_forwarding"); err != nil {
				return err
			}
		} else if input.Cost > reservation.HoldAmount {
			// The accepted hold includes any buyer surcharge.
			input.Cost = reservation.HoldAmount
		}
	}

	if input.Cost > 0 {
		var poolOwnerID sql.NullInt64
		var fallbackPlatformFee float64
		// Settlement updates this pool's counters later. Acquire the write lock
		// now to avoid concurrent SHARE-to-UPDATE lock upgrade deadlocks.
		if err := tx.QueryRowContext(ctx, `SELECT owner_id, platform_fee_percent FROM shared_pools WHERE id = $1 FOR UPDATE`, input.PoolID).Scan(&poolOwnerID, &fallbackPlatformFee); err != nil {
			return err
		}
		settlementRuleTime := time.Now()
		if reservation != nil && !reservation.ReservedAt.IsZero() {
			// Delayed retries and audited review must use the fee rule the user
			// accepted when the hold was created, never a later policy version.
			settlementRuleTime = reservation.ReservedAt
		}
		settlementRule, err := sharedPoolSettlementRuleAtTx(ctx, tx, input.PoolID, settlementRuleTime, sharedPoolSettlementRule{PlatformFeePercent: fallbackPlatformFee})
		if err != nil {
			return err
		}
		platformFeePercent := math.Max(0, math.Min(100, settlementRule.PlatformFeePercent))
		// New quotes freeze the surcharge rate before forwarding. Unversioned
		// historical snapshots retain their original inclusive-fee split.
		var acceptedFee struct {
			Mode    string  `json:"fee_mode"`
			Percent float64 `json:"platform_fee_percent"`
		}
		if err := json.Unmarshal([]byte(priceSnapshot), &acceptedFee); err != nil {
			return err
		}
		if acceptedFee.Mode != "" && acceptedFee.Mode != service.SharedPoolFeeModeBuyerSurcharge {
			return errors.New("unsupported shared pool fee mode")
		}
		if acceptedFee.Mode == service.SharedPoolFeeModeBuyerSurcharge {
			if acceptedFee.Percent < 0 || acceptedFee.Percent > 100 {
				return errors.New("invalid shared pool surcharge rate")
			}
			platformFeePercent = acceptedFee.Percent
		}
		buyerCost := input.Cost
		usageNote := fmt.Sprintf("共享池 API 调用 · 池 #%d", input.PoolID)
		if model := strings.TrimSpace(input.Model); model != "" {
			usageNote = fmt.Sprintf("共享池 API 调用 · 池 #%d · 模型 %s", input.PoolID, model)
		}
		var usageLedgerID int64
		if sourceID != "" {
			if err := tx.QueryRowContext(ctx, `INSERT INTO shared_pool_balance_ledger (user_id, pool_id, account_id, source_type, source_id, amount, balance_after, status, note, posted_at, settlement_destination, price_version_id, pricing_source_snapshot, price_snapshot) VALUES ($1, $2, NULLIF($3, 0), 'share_pool_usage', $4, 0, 0, 'pending', $5, NULL, 'user_balance', NULLIF($6, 0), $7, $8::jsonb) ON CONFLICT DO NOTHING RETURNING id`, input.UserID, input.PoolID, input.AccountID, sourceID, usageNote, input.PriceVersionID, input.PricingSource, priceSnapshot).Scan(&usageLedgerID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("%w: shared pool usage idempotency key %q is already claimed", service.ErrSharedPoolReservationConflict, sourceID)
				}
				return err
			}
		}

		var ownerPayout float64
		settledBuyerUnits := math.Round(buyerCost * 1e8)
		ownerUnits := 0.0
		if poolOwnerID.Valid && poolOwnerID.Int64 > 0 && poolOwnerID.Int64 != input.UserID {
			if acceptedFee.Mode == service.SharedPoolFeeModeBuyerSurcharge {
				ownerUnits = math.Round(settledBuyerUnits / (1 + platformFeePercent/100))
			} else {
				ownerUnits = math.Floor(settledBuyerUnits*(100-platformFeePercent)/100 + 1e-8)
			}
			ownerUnits = math.Min(ownerUnits, settledBuyerUnits)
			ownerPayout = ownerUnits / 1e8
		}
		platformFee := (settledBuyerUnits - ownerUnits) / 1e8

		var userBalance float64
		if reservation != nil {
			settledAmount, balanceAfter, capped, settleErr := captureSharedPoolUsageReservationTx(ctx, tx, reservation, reportedCost)
			if settleErr != nil {
				return settleErr
			}
			if math.Abs(settledAmount-buyerCost) > 0.000000000001 {
				return errors.New("shared pool reservation settlement amount mismatch")
			}
			userBalance = balanceAfter
			if capped {
				usageNote += "（上游用量超过预授权，已按预授权上限结算）"
			}
		} else {
			if err := tx.QueryRowContext(ctx,
				`UPDATE users SET balance = balance - $1, updated_at = NOW()
				 WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
				 RETURNING balance`,
				buyerCost, input.UserID,
			).Scan(&userBalance); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					if sourceID != "" {
						if _, markErr := tx.ExecContext(ctx,
							`UPDATE shared_pool_balance_ledger
							 SET amount = 0, balance_after = 0, status = 'reversed', note = $2, posted_at = NOW()
							 WHERE id = $1`,
							usageLedgerID, usageNote+" (insufficient balance; shared pool access released)",
						); markErr != nil {
							return markErr
						}
					}
					if err := releaseActiveSharedPoolSeatForUserTx(ctx, tx, input.PoolID, input.UserID, "insufficient_balance"); err != nil {
						return err
					}
					if err := tx.Commit(); err != nil {
						return err
					}
					return service.ErrInsufficientBalance
				}
				return err
			}
		}
		if sourceID != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_balance_ledger SET amount = $2, balance_after = $3, status = 'posted', note = $4, posted_at = NOW(), platform_fee_amount = $5, owner_payout_amount = $6, price_version_id = NULLIF($7, 0), pricing_source_snapshot = $8, price_snapshot = $9::jsonb WHERE id = $1`, usageLedgerID, -buyerCost, userBalance, usageNote, platformFee, ownerPayout, input.PriceVersionID, input.PricingSource, priceSnapshot); err != nil {
				return err
			}
		} else {
			if err := tx.QueryRowContext(ctx, `INSERT INTO shared_pool_balance_ledger (user_id, pool_id, account_id, source_type, source_id, amount, balance_after, status, note, posted_at, settlement_destination, platform_fee_amount, owner_payout_amount, price_version_id, pricing_source_snapshot, price_snapshot) VALUES ($1, $2, NULLIF($3, 0), 'share_pool_usage', $4, $5, $6, 'posted', $7, NOW(), 'user_balance', $8, $9, NULLIF($10, 0), $11, $12::jsonb) RETURNING id`, input.UserID, input.PoolID, input.AccountID, sourceID, -buyerCost, userBalance, usageNote, platformFee, ownerPayout, input.PriceVersionID, input.PricingSource, priceSnapshot).Scan(&usageLedgerID); err != nil {
				return err
			}
		}
		if poolOwnerID.Valid && poolOwnerID.Int64 > 0 && poolOwnerID.Int64 != input.UserID {
			if ownerPayout > 0 {
				var lockedOwnerID int64
				ownerErr := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, poolOwnerID.Int64).Scan(&lockedOwnerID)
				if ownerErr != nil && !errors.Is(ownerErr, sql.ErrNoRows) {
					return ownerErr
				}
				if ownerErr == nil {
					payoutNote := fmt.Sprintf("共享池 API 分润 · 池 #%d", input.PoolID)
					ownerOperationID := sourceID
					if ownerOperationID == "" {
						ownerOperationID = fmt.Sprintf("usage-ledger:%d", usageLedgerID)
					}
					if _, err := creditSharedPoolOwnerWalletTx(ctx, tx, sharedPoolOwnerEarningCredit{
						OwnerID:        lockedOwnerID,
						PoolID:         input.PoolID,
						AccountID:      input.AccountID,
						PriceVersionID: input.PriceVersionID,
						OperationID:    ownerOperationID,
						RequestID:      sourceID,
						Model:          input.Model,
						PricingSource:  input.PricingSource,
						GrossAmount:    buyerCost,
						PlatformFee:    platformFee,
						NetAmount:      ownerPayout,
						SourceType:     "share_pool_payout",
						Note:           payoutNote,
					}); err != nil {
						return err
					}
				}
			}
		}
	}
	if reservation != nil && input.Success && input.Cost <= 0 {
		if _, _, _, err := captureSharedPoolUsageReservationTx(ctx, tx, reservation, reportedCost); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_access_keys SET total_used = total_used + $1, last_used_at = NOW(), updated_at = NOW() WHERE id = $2`, input.Cost, input.AccessKeyID); err != nil {
		return err
	}
	// Any recorded shared-pool usage (success or failed billed attempt) counts as seat activity.
	if _, err := tx.ExecContext(ctx, `UPDATE pool_seat_bindings
		SET last_activity_at = NOW(), updated_at = NOW()
		WHERE pool_id = $1 AND user_id = $2 AND status = 'active'`, input.PoolID, input.UserID); err != nil {
		return err
	}
	if input.Success {
		if err := recordSharedPoolModelUsageWindows(ctx, tx, input); err != nil {
			return err
		}
	}
	if input.AccountID > 0 {
		if input.Success {
			_, err = tx.ExecContext(ctx, `UPDATE shared_pool_accounts SET total_calls = total_calls + 1, successful_calls = successful_calls + 1, last_used_at = NOW(), updated_at = NOW() WHERE id = $1 AND pool_id = $2 AND deleted_at IS NULL`, input.AccountID, input.PoolID)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE shared_pool_accounts SET total_calls = total_calls + 1, failed_calls = failed_calls + 1, last_used_at = NOW(), updated_at = NOW() WHERE id = $1 AND pool_id = $2 AND deleted_at IS NULL`, input.AccountID, input.PoolID)
		}
		if err != nil {
			return err
		}
	}
	if input.Success {
		_, err = tx.ExecContext(ctx, `UPDATE shared_pools SET total_calls = total_calls + 1, successful_calls = successful_calls + 1, updated_at = NOW() WHERE id = $1`, input.PoolID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE shared_pools SET total_calls = total_calls + 1, failed_calls = failed_calls + 1, updated_at = NOW() WHERE id = $1`, input.PoolID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func recordSharedPoolModelUsageWindows(ctx context.Context, tx *sql.Tx, input service.SharedPoolUsageInput) error {
	if tx == nil || input.PoolID <= 0 {
		return nil
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return nil
	}
	canonicalModel, err := resolveSharedPoolPublishedModelForUsage(ctx, tx, input.PoolID, model)
	if err != nil {
		return err
	}
	if canonicalModel != "" {
		model = canonicalModel
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO shared_pool_model_usage_windows (pool_id, model_name, window_type, window_start, cost, calls, updated_at)
		VALUES
			($1, $2, '5h', date_trunc('hour', NOW()), $3, 1, NOW()),
			($1, $2, '1d', date_trunc('hour', NOW()), $3, 1, NOW()),
			($1, $2, '7d', date_trunc('day', NOW()), $3, 1, NOW())
		ON CONFLICT (pool_id, model_name, window_type, window_start)
		DO UPDATE SET cost = shared_pool_model_usage_windows.cost + EXCLUDED.cost,
			calls = shared_pool_model_usage_windows.calls + EXCLUDED.calls,
			updated_at = NOW()`, input.PoolID, model, input.Cost)
	return err
}

func resolveSharedPoolPublishedModelForUsage(ctx context.Context, tx *sql.Tx, poolID int64, model string) (string, error) {
	model = strings.TrimSpace(model)
	if tx == nil || poolID <= 0 || model == "" {
		return "", nil
	}
	var publishedModel string
	err := tx.QueryRowContext(ctx, `SELECT spm.model_name
		FROM shared_pool_models spm
		WHERE spm.pool_id = $1
			AND spm.enabled = TRUE
			AND btrim(spm.model_name) <> ''
			AND (
				spm.model_name = $2
				OR spm.model_aliases ? $2
				OR NULLIF(TRIM(spm.upstream_model_name), '') = $2
				OR EXISTS (
					SELECT 1
					FROM shared_pool_accounts spa
					JOIN LATERAL (
						SELECT 1
						FROM jsonb_to_recordset(spa.model_configs) AS cfg(model_name text, upstream_model_name text, model_open boolean, aliases jsonb)
						WHERE COALESCE(cfg.model_open, TRUE) = TRUE
							AND cfg.model_name = spm.model_name
							AND (COALESCE(cfg.aliases, '[]'::jsonb) ? $2 OR NULLIF(TRIM(cfg.upstream_model_name), '') = $2)
						LIMIT 1
					) cfg ON TRUE
					WHERE spa.pool_id = spm.pool_id
						AND spa.deleted_at IS NULL
				)
			)
		ORDER BY
			CASE
				WHEN spm.model_name = $2 THEN 0
				WHEN spm.model_aliases ? $2 THEN 1
				WHEN NULLIF(TRIM(spm.upstream_model_name), '') = $2 THEN 2
				ELSE 3
			END,
			spm.sort_order ASC,
			spm.id ASC
		LIMIT 1`, poolID, model).Scan(&publishedModel)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(publishedModel), nil
}

//   * pool_seat_fee     : negative amount, debits the seat holder
//   * pool_owner_payout : positive amount, credits the pool owner (if any)
// Every hourly charge is keyed by (seat_id, billing_hour) with a unique index,
// so a worker re-run or overlapping leader election never double-charges.
// ---------------------------------------------------------------------------

// JoinSharedPoolTx atomically enforces admission rules and creates an active
// seat. Idempotent: if the user already holds an active seat in the pool, the
// existing seat is returned with AlreadyHeld=true.
func (r *bizDecipherRepository) JoinSharedPoolTx(ctx context.Context, poolID, userID int64) (*service.JoinPoolResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Lock the pool row and read admission parameters.
	var (
		poolName     string
		status       string
		listed       bool
		lifecycle    string
		maxUsers     int
		currentUsers int
		minBalance   float64
		hourlyFee    float64
		waiverMin    float64
	)
	if err := tx.QueryRowContext(ctx,
		`SELECT name, status, listed, lifecycle_state, max_users, current_users, min_balance_admission, hourly_seat_fee, hourly_min_usage_waiver
		 FROM shared_pools WHERE id = $1 FOR UPDATE`,
		poolID,
	).Scan(&poolName, &status, &listed, &lifecycle, &maxUsers, &currentUsers, &minBalance, &hourlyFee, &waiverMin); err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrPoolNotJoinable
		}
		return nil, err
	}
	joinRule, err := sharedPoolSettlementRuleAtTx(ctx, tx, poolID, time.Now(), sharedPoolSettlementRule{
		HourlySeatFee:        hourlyFee,
		HourlyMinUsageWaiver: waiverMin,
	})
	if err != nil {
		return nil, err
	}
	hourlyFee = joinRule.HourlySeatFee
	waiverMin = joinRule.HourlyMinUsageWaiver

	// 2. Idempotency: return the existing active seat if present.
	var seat service.PoolSeat
	existsErr := tx.QueryRowContext(ctx,
		`SELECT id, pool_id, user_id, status, hourly_seat_fee, hourly_min_usage_waiver, joined_at, last_charged_at,
		        COALESCE(last_activity_at, joined_at), total_charged
		 FROM pool_seat_bindings WHERE pool_id = $1 AND user_id = $2 AND status = 'active'`,
		poolID, userID,
	).Scan(&seat.ID, &seat.PoolID, &seat.UserID, &seat.Status, &seat.HourlySeatFee, &seat.HourlyMinUsageWaiver,
		&seat.JoinedAt, &seat.LastChargedAt, &seat.LastActivityAt, &seat.TotalCharged)
	if existsErr == nil {
		seat.PoolName = poolName
		if err := hydratePoolSeatUsageProgress(ctx, tx, &seat); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &service.JoinPoolResult{Seat: seat, AlreadyHeld: true}, nil
	} else if existsErr != sql.ErrNoRows {
		return nil, existsErr
	}

	// 3. Admission checks.
	if !listed || lifecycle != "operating" || (status != "healthy" && status != "limited") {
		return nil, service.ErrPoolNotJoinable
	}
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_seat_bindings WHERE pool_id = $1 AND status = 'active'`,
		poolID,
	).Scan(&currentUsers); err != nil {
		return nil, err
	}
	if maxUsers > 0 && currentUsers >= maxUsers {
		return nil, service.ErrPoolFull
	}
	if minBalance > 0 {
		var balance float64
		if err := tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1`, userID).Scan(&balance); err != nil {
			if err == sql.ErrNoRows {
				return nil, service.ErrPoolNotJoinable
			}
			return nil, err
		}
		if balance < minBalance {
			return nil, service.ErrPoolInsufficientBalance
		}
	}

	// 4. Create the seat (snapshot the fee) and bump occupancy.
	// last_activity_at starts at join time; idle auto-release counts from here until real API use.
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO pool_seat_bindings (pool_id, user_id, status, hourly_seat_fee, hourly_min_usage_waiver, joined_at, last_charged_at, last_activity_at)
		 VALUES ($1, $2, 'active', $3, $4, NOW(), NOW(), NOW())
		 RETURNING id, pool_id, user_id, status, hourly_seat_fee, hourly_min_usage_waiver, joined_at, last_charged_at,
		           COALESCE(last_activity_at, joined_at), total_charged`,
		poolID, userID, hourlyFee, waiverMin,
	).Scan(&seat.ID, &seat.PoolID, &seat.UserID, &seat.Status, &seat.HourlySeatFee, &seat.HourlyMinUsageWaiver,
		&seat.JoinedAt, &seat.LastChargedAt, &seat.LastActivityAt, &seat.TotalCharged); err != nil {
		return nil, err
	}
	if err := syncSharedPoolCurrentUsersTx(ctx, tx, poolID); err != nil {
		return nil, err
	}
	seat.PoolName = poolName
	if err := hydratePoolSeatUsageProgress(ctx, tx, &seat); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &service.JoinPoolResult{Seat: seat, AlreadyHeld: false}, nil
}

// LeaveSharedPoolTx settles any whole overdue hours for the active seat, marks
// it released, and decrements current_users. Idempotent: if no active seat
// exists the call is a no-op.
func (r *bizDecipherRepository) LeaveSharedPoolTx(ctx context.Context, poolID, userID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Lock the active seat.
	var (
		seatID      int64
		hourlyFee   float64
		waiverMin   float64
		lastCharged time.Time
		ownerID     sql.NullInt64
	)
	if err := tx.QueryRowContext(ctx,
		`SELECT sb.id, sb.hourly_seat_fee, sb.hourly_min_usage_waiver, sb.last_charged_at, sp.owner_id
		 FROM pool_seat_bindings sb
		 JOIN shared_pools sp ON sp.id = sb.pool_id
		 WHERE sb.pool_id = $1 AND sb.user_id = $2 AND sb.status = 'active'
		 FOR UPDATE OF sb`,
		poolID, userID,
	).Scan(&seatID, &hourlyFee, &waiverMin, &lastCharged, &ownerID); err != nil {
		if err == sql.ErrNoRows {
			// Nothing to release; idempotent no-op.
			return tx.Commit()
		}
		return err
	}

	// Settle whole overdue hours up to now before releasing.
	// Always release the seat after attempting settlement. Insufficient balance must not
	// leave the user stuck in a fake "still joined" state after a successful leave call.
	if _, _, err := chargeSeatHoursTx(ctx, tx, seatID, poolID, userID, ownerID, hourlyFee, waiverMin, lastCharged, time.Now()); err != nil {
		if !errors.Is(err, service.ErrInsufficientBalance) {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE pool_seat_bindings
		 SET status = 'released', released_at = NOW(), release_reason = 'user_leave', updated_at = NOW()
		 WHERE id = $1`,
		seatID,
	); err != nil {
		return err
	}
	if err := disableSharedPoolAccessKeysForUserTx(ctx, tx, poolID, userID); err != nil {
		return err
	}
	if err := syncSharedPoolCurrentUsersTx(ctx, tx, poolID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListMySeats returns the user's seats, active first then most recently joined.
func (r *bizDecipherRepository) ListMySeats(ctx context.Context, userID int64) ([]service.PoolSeat, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT sb.id, sb.pool_id, sp.name, sb.user_id, sb.status, sb.hourly_seat_fee, sb.hourly_min_usage_waiver,
		        sb.joined_at, sb.last_charged_at, COALESCE(sb.last_activity_at, sb.joined_at),
		        sb.released_at, sb.release_reason, sb.total_charged
		 FROM pool_seat_bindings sb
		 JOIN shared_pools sp ON sp.id = sb.pool_id
		 WHERE sb.user_id = $1
		 ORDER BY (sb.status = 'active') DESC, sb.joined_at DESC
		 LIMIT 200`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPoolSeatRows(ctx, r.db, rows)
}

func scanPoolSeatRows(ctx context.Context, qr queryRower, rows *sql.Rows) ([]service.PoolSeat, error) {
	out := []service.PoolSeat{}
	for rows.Next() {
		var seat service.PoolSeat
		var releasedAt sql.NullTime
		if err := rows.Scan(&seat.ID, &seat.PoolID, &seat.PoolName, &seat.UserID, &seat.Status,
			&seat.HourlySeatFee, &seat.HourlyMinUsageWaiver, &seat.JoinedAt, &seat.LastChargedAt,
			&seat.LastActivityAt, &releasedAt, &seat.ReleaseReason, &seat.TotalCharged); err != nil {
			return nil, err
		}
		if releasedAt.Valid {
			seat.ReleasedAt = &releasedAt.Time
		}
		if err := hydratePoolSeatUsageProgress(ctx, qr, &seat); err != nil {
			return nil, err
		}
		out = append(out, seat)
	}
	return out, rows.Err()
}

// ListSharedPoolMembers returns seats for a pool only when ownerID owns it.
func (r *bizDecipherRepository) ListSharedPoolMembers(ctx context.Context, poolID, ownerID int64, status string) ([]service.PoolSeat, error) {
	var owned bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM shared_pools WHERE id = $1 AND owner_id = $2)`, poolID, ownerID).Scan(&owned); err != nil {
		return nil, err
	}
	if !owned {
		return nil, service.ErrPoolForbidden
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT sb.id, sb.pool_id, sp.name, sb.user_id, sb.status, sb.hourly_seat_fee, sb.hourly_min_usage_waiver,
		        sb.joined_at, sb.last_charged_at, COALESCE(sb.last_activity_at, sb.joined_at),
		        sb.released_at, sb.release_reason, sb.total_charged
		 FROM pool_seat_bindings sb
		 JOIN shared_pools sp ON sp.id = sb.pool_id
		 WHERE sb.pool_id = $1
		   AND ($2 = 'all' OR sb.status = $2)
		 ORDER BY (sb.status = 'active') DESC, sb.joined_at DESC
		 LIMIT 500`,
		poolID, status,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPoolSeatRows(ctx, r.db, rows)
}

// RemoveSharedPoolMemberTx settles and releases a member seat from an owned pool.
func (r *bizDecipherRepository) RemoveSharedPoolMemberTx(ctx context.Context, poolID, seatID, ownerID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var owned bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM shared_pools WHERE id = $1 AND owner_id = $2)`, poolID, ownerID).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return service.ErrPoolForbidden
	}

	var (
		seatUserID  int64
		hourlyFee   float64
		waiverMin   float64
		lastCharged time.Time
		seatStatus  string
	)
	if err := tx.QueryRowContext(ctx,
		`SELECT user_id, hourly_seat_fee, hourly_min_usage_waiver, last_charged_at, status
		 FROM pool_seat_bindings
		 WHERE id = $1 AND pool_id = $2
		 FOR UPDATE`,
		seatID, poolID,
	).Scan(&seatUserID, &hourlyFee, &waiverMin, &lastCharged, &seatStatus); err != nil {
		if err == sql.ErrNoRows {
			return service.ErrPoolForbidden
		}
		return err
	}
	if seatStatus != "active" {
		return tx.Commit()
	}

	// Owner kick must still release the seat even if the member cannot settle due fees.
	if _, _, err := chargeSeatHoursTx(ctx, tx, seatID, poolID, seatUserID, sql.NullInt64{Int64: ownerID, Valid: true}, hourlyFee, waiverMin, lastCharged, time.Now()); err != nil {
		if !errors.Is(err, service.ErrInsufficientBalance) {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE pool_seat_bindings
		 SET status = 'released', released_at = NOW(), release_reason = 'owner_removed', updated_at = NOW()
		 WHERE id = $1`,
		seatID,
	); err != nil {
		return err
	}
	if err := disableSharedPoolAccessKeysForUserTx(ctx, tx, poolID, seatUserID); err != nil {
		return err
	}
	if err := syncSharedPoolCurrentUsersTx(ctx, tx, poolID); err != nil {
		return err
	}
	return tx.Commit()
}

func disableSharedPoolAccessKeysForUserTx(ctx context.Context, tx *sql.Tx, poolID, userID int64) error {
	if tx == nil || poolID <= 0 || userID <= 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_access_keys SET status = $3, updated_at = NOW()
		WHERE pool_id = $1 AND user_id = $2 AND status <> $3`, poolID, userID, service.StatusDisabled); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE api_keys ak SET status = $3, updated_at = NOW()
		WHERE ak.id IN (SELECT api_key_id FROM shared_pool_access_keys WHERE pool_id = $1 AND user_id = $2)
			AND ak.deleted_at IS NULL
			AND ak.status <> $3
			AND NOT EXISTS (
				SELECT 1 FROM shared_pool_access_keys sak
				WHERE sak.api_key_id = ak.id AND sak.status = 'active'
			)`, poolID, userID, service.StatusDisabled)
	return err
}

func syncSharedPoolCurrentUsersTx(ctx context.Context, tx *sql.Tx, poolID int64) error {
	if tx == nil || poolID <= 0 {
		return nil
	}
	var lockedPoolID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM shared_pools WHERE id = $1 FOR UPDATE`,
		poolID,
	).Scan(&lockedPoolID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE shared_pools pool
		SET current_users = (
				SELECT COUNT(*)
				FROM pool_seat_bindings seat
				WHERE seat.pool_id = pool.id
					AND seat.status = 'active'
			),
			updated_at = NOW()
		WHERE pool.id = $1`,
		lockedPoolID,
	)
	return err
}

func releaseActiveSharedPoolSeatForUserTx(ctx context.Context, tx *sql.Tx, poolID, userID int64, reason string) error {
	if tx == nil || poolID <= 0 || userID <= 0 {
		return nil
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE pool_seat_bindings
		 SET status = 'released', released_at = NOW(), release_reason = $3, updated_at = NOW()
		 WHERE pool_id = $1 AND user_id = $2 AND status = 'active'`,
		poolID, userID, reason,
	)
	if err != nil {
		return err
	}
	if err := disableSharedPoolAccessKeysForUserTx(ctx, tx, poolID, userID); err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows <= 0 {
		return nil
	}
	return syncSharedPoolCurrentUsersTx(ctx, tx, poolID)
}

func releaseSharedPoolSeatTx(ctx context.Context, tx *sql.Tx, seatID, poolID, userID int64, reason string) error {
	if tx == nil || seatID <= 0 || poolID <= 0 || userID <= 0 {
		return nil
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE pool_seat_bindings
		 SET status = 'released', released_at = NOW(), release_reason = $2, updated_at = NOW()
		 WHERE id = $1 AND status = 'active'`,
		seatID, reason,
	)
	if err != nil {
		return err
	}
	if err := disableSharedPoolAccessKeysForUserTx(ctx, tx, poolID, userID); err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows <= 0 {
		return nil
	}
	return syncSharedPoolCurrentUsersTx(ctx, tx, poolID)
}

func releaseSharedPoolSeatForInsufficientBalanceTx(ctx context.Context, tx *sql.Tx, seatID, poolID, userID int64) error {
	return releaseSharedPoolSeatTx(ctx, tx, seatID, poolID, userID, "insufficient_balance")
}

// GrantSharedPoolStabilityRewards grants non-withdrawable daily stability credits
// for each eligible listed owner pool in the previous complete UTC day.
// Base tiers are ranked per owner (1st/2nd/3rd pool only); quality, real-use
// and owner excellence bonuses stack on top. Each pool grant is idempotent on
// (pool_id, owner_id, reward_hour=day_start); owner excellence is idempotent
// on credit_ledger source_id.
func (r *bizDecipherRepository) GrantSharedPoolStabilityRewards(ctx context.Context, now time.Time) (*service.ChargeSeatsSummary, error) {
	if now.IsZero() {
		now = time.Now()
	}
	rewardDay := previousCompleteUTCDay(now)
	if rewardDay.IsZero() {
		return &service.ChargeSeatsSummary{}, nil
	}
	dayStart := rewardDay
	dayEnd := rewardDay.Add(24 * time.Hour)

	rows, err := r.db.QueryContext(ctx, `
		SELECT
			sp.id,
			sp.owner_id,
			COALESCE(sp.today_availability, 0),
			COALESCE(sp.seven_day_availability, 0),
			COALESCE(sp.avg_latency_ms, 0),
			COALESCE(sp.consecutive_probe_failures, 0),
			COALESCE(sp.quality_score, 0),
			EXISTS (
				SELECT 1
				FROM shared_pool_balance_ledger ubl
				WHERE ubl.pool_id = sp.id
					AND ubl.source_type = 'share_pool_usage'
					AND ubl.status = 'posted'
					AND ubl.created_at >= $1
					AND ubl.created_at < $2
					AND ubl.amount < 0
			) AS has_real_calls,
			EXISTS (
				SELECT 1
				FROM pool_seat_bindings psb
				WHERE psb.pool_id = sp.id
					AND psb.status = 'active'
					AND (
						COALESCE(psb.hourly_seat_fee, 0) > 0
						OR EXISTS (
							SELECT 1
							FROM shared_pool_balance_ledger sbl
							WHERE sbl.pool_id = sp.id
								AND sbl.user_id = psb.user_id
								AND sbl.source_type IN ('pool_seat_fee', 'share_pool_usage')
								AND sbl.status = 'posted'
								AND sbl.created_at >= $1
								AND sbl.created_at < $2
						)
					)
			) AS has_paid_or_seat_users
		FROM shared_pools sp
		JOIN users u ON u.id = sp.owner_id AND u.deleted_at IS NULL
		WHERE sp.owner_id IS NOT NULL
			AND sp.listed = TRUE
			AND sp.lifecycle_state = 'operating'
			AND sp.status = 'healthy'
			AND sp.governance_status NOT IN ('banned', 'suppressed')
			AND (
				(btrim(sp.upstream_base_url) <> '' AND btrim(sp.upstream_api_key) <> '')
				OR EXISTS (
					SELECT 1
					FROM shared_pool_accounts spa
					WHERE spa.pool_id = sp.id
						AND spa.owner_id = sp.owner_id
						AND spa.deleted_at IS NULL
						AND spa.status IN ('active', 'limited', 'testing')
						AND spa.schedulable = TRUE
						AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE))
						AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> ''))
						AND (spa.gate_required = FALSE OR spa.gate_passed = TRUE)
				)
			)
		ORDER BY sp.owner_id ASC, sp.id ASC
		LIMIT 2000`, dayStart, dayEnd)
	if err != nil {
		return nil, err
	}

	var candidates []sharedPoolStabilityPoolMetrics
	for rows.Next() {
		var item sharedPoolStabilityPoolMetrics
		if err := rows.Scan(
			&item.PoolID,
			&item.OwnerID,
			&item.TodayAvailability,
			&item.SevenDayAvailability,
			&item.AvgLatencyMS,
			&item.ConsecutiveProbeFailures,
			&item.QualityScore,
			&item.HasRealCalls,
			&item.HasPaidOrSeatUsers,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
		candidates = append(candidates, item)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Rank pools within each owner for base tier allocation.
	byOwner := map[int64][]sharedPoolStabilityPoolMetrics{}
	for _, c := range candidates {
		byOwner[c.OwnerID] = append(byOwner[c.OwnerID], c)
	}
	rankByPool := map[int64]int{}
	for _, pools := range byOwner {
		for poolID, rank := range rankSharedPoolStabilityPools(pools) {
			rankByPool[poolID] = rank
		}
	}

	summary := &service.ChargeSeatsSummary{}
	ownersSeen := map[int64]struct{}{}
	for _, item := range candidates {
		ownersSeen[item.OwnerID] = struct{}{}
		rank := rankByPool[item.PoolID]
		breakdown := computeSharedPoolStabilityPoolBreakdown(item, rank)
		if breakdown.TotalAmount <= 0 {
			continue
		}
		granted, amount, err := r.grantOneSharedPoolStabilityReward(ctx, item.PoolID, item.OwnerID, rewardDay, breakdown)
		if err != nil {
			return summary, err
		}
		if granted {
			summary.StabilityRewards++
			summary.TotalStabilityCredit += amount
		}
	}

	// Owner excellence bonus once per owner per day (quality 80 / certified 150).
	for ownerID := range ownersSeen {
		granted, amount, err := r.grantSharedPoolOwnerExcellenceReward(ctx, ownerID, rewardDay)
		if err != nil {
			return summary, err
		}
		if granted {
			summary.StabilityRewards++
			summary.TotalStabilityCredit += amount
		}
	}
	return summary, nil
}

func (r *bizDecipherRepository) grantOneSharedPoolStabilityReward(
	ctx context.Context,
	poolID, ownerID int64,
	rewardDay time.Time,
	breakdown sharedPoolStabilityBreakdown,
) (bool, float64, error) {
	amount := breakdown.TotalAmount
	if amount <= 0 {
		return false, 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var lockedPoolID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT sp.id
		FROM shared_pools sp
		JOIN users u ON u.id = sp.owner_id AND u.deleted_at IS NULL
		WHERE sp.id = $1
			AND sp.owner_id = $2
			AND sp.listed = TRUE
			AND sp.lifecycle_state = 'operating'
			AND sp.status = 'healthy'
			AND sp.governance_status NOT IN ('banned', 'suppressed')
			AND (
				(btrim(sp.upstream_base_url) <> '' AND btrim(sp.upstream_api_key) <> '')
				OR EXISTS (
					SELECT 1
					FROM shared_pool_accounts spa
					WHERE spa.pool_id = sp.id
						AND spa.owner_id = sp.owner_id
						AND spa.deleted_at IS NULL
						AND spa.status IN ('active', 'limited', 'testing')
						AND spa.schedulable = TRUE
						AND (spa.expires_at IS NULL OR spa.expires_at > NOW() OR (spa.auth_type <> 'oauth' AND spa.auto_pause_on_expired = FALSE))
						AND ((spa.auth_type IN ('apikey', 'api_key') AND btrim(spa.upstream_base_url) <> '' AND btrim(spa.upstream_api_key) <> '') OR (spa.auth_type = 'oauth' AND btrim(spa.credentials_encrypted) <> ''))
						AND (spa.gate_required = FALSE OR spa.gate_passed = TRUE)
				)
			)
		FOR UPDATE OF sp, u`, poolID, ownerID).Scan(&lockedPoolID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, tx.Commit()
		}
		return false, 0, err
	}

	// Keep reward_hour as day-start timestamp so existing unique index works.
	sourceID := fmt.Sprintf("pool:%d:day:%s", poolID, rewardDay.UTC().Format("2006-01-02"))
	note := formatSharedPoolStabilityNote(poolID, rewardDay, breakdown.NoteParts, 0)

	var rewardID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO shared_pool_stability_rewards (pool_id, owner_id, reward_hour, credit_amount, status, note)
		 VALUES ($1, $2, $3, $4, 'posted', $5)
		 ON CONFLICT (pool_id, owner_id, reward_hour) DO NOTHING
		 RETURNING id`,
		poolID, ownerID, rewardDay.UTC(), amount, note,
	).Scan(&rewardID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, tx.Commit()
		}
		return false, 0, err
	}

	var balanceAfter float64
	if err := tx.QueryRowContext(ctx,
		`UPDATE users SET credit_balance = credit_balance + $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL RETURNING credit_balance`,
		amount, ownerID,
	).Scan(&balanceAfter); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, tx.Commit()
		}
		return false, 0, err
	}

	var ledgerID int64
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at)
		 VALUES ($1, 'shared_pool_stability_reward', $2, $3, $4, 'posted', $5, NOW())
		 RETURNING id`,
		ownerID, sourceID, amount, balanceAfter, note,
	).Scan(&ledgerID); err != nil {
		return false, 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE shared_pool_stability_rewards SET credit_ledger_id = $2 WHERE id = $1`, rewardID, ledgerID); err != nil {
		return false, 0, err
	}
	if err := tx.Commit(); err != nil {
		return false, 0, err
	}
	return true, amount, nil
}

// grantSharedPoolOwnerExcellenceReward grants quality/certified owner daily bonus once.
func (r *bizDecipherRepository) grantSharedPoolOwnerExcellenceReward(ctx context.Context, ownerID int64, rewardDay time.Time) (bool, float64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var tierRaw string
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(pool_owner_reward_tier, 'none')
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE`, ownerID).Scan(&tierRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, tx.Commit()
		}
		// Column may not exist yet on very old DBs; treat as none.
		if strings.Contains(err.Error(), "pool_owner_reward_tier") {
			return false, 0, tx.Commit()
		}
		return false, 0, err
	}
	tier := normalizeSharedPoolStabilityOwnerTier(tierRaw)
	amount := sharedPoolStabilityOwnerExcellenceAmount(tier)
	if amount <= 0 {
		return false, 0, tx.Commit()
	}

	sourceID := fmt.Sprintf("owner-excellence:%d:day:%s", ownerID, rewardDay.UTC().Format("2006-01-02"))
	note := fmt.Sprintf("Shared pool owner excellence daily reward: tier=%s day %s", tier, rewardDay.UTC().Format("2006-01-02"))

	// Idempotent via credit_ledger unique (user_id, source_type, source_id) when present;
	// also check existence first for environments without the unique index.
	var existing int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM credit_ledger
		WHERE user_id = $1 AND source_type = 'shared_pool_stability_reward' AND source_id = $2`,
		ownerID, sourceID,
	).Scan(&existing); err != nil {
		return false, 0, err
	}
	if existing > 0 {
		return false, 0, tx.Commit()
	}

	var balanceAfter float64
	if err := tx.QueryRowContext(ctx,
		`UPDATE users SET credit_balance = credit_balance + $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL RETURNING credit_balance`,
		amount, ownerID,
	).Scan(&balanceAfter); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, 0, tx.Commit()
		}
		return false, 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO credit_ledger (user_id, source_type, source_id, amount, balance_after, status, note, posted_at)
		 VALUES ($1, 'shared_pool_stability_reward', $2, $3, $4, 'posted', $5, NOW())`,
		ownerID, sourceID, amount, balanceAfter, note,
	); err != nil {
		return false, 0, err
	}
	if err := tx.Commit(); err != nil {
		return false, 0, err
	}
	return true, amount, nil
}

// ReleaseIdleSharedPoolSeats settles and releases active seats idle for SharedPoolSeatIdleTimeout.
// Idle clock is last_activity_at (join time until first real shared-pool API use).
func (r *bizDecipherRepository) ReleaseIdleSharedPoolSeats(ctx context.Context, now time.Time) (*service.ChargeSeatsSummary, error) {
	if now.IsZero() {
		now = time.Now()
	}
	idleBefore := now.Add(-service.SharedPoolSeatIdleTimeout)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM pool_seat_bindings
		 WHERE status = 'active'
		   AND COALESCE(last_activity_at, joined_at) <= $1
		 ORDER BY COALESCE(last_activity_at, joined_at) ASC
		 LIMIT 1000`,
		idleBefore,
	)
	if err != nil {
		return nil, err
	}
	var seatIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		seatIDs = append(seatIDs, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	summary := &service.ChargeSeatsSummary{}
	for _, seatID := range seatIDs {
		released, hours, charged, err := r.releaseOneIdleSeat(ctx, seatID, now)
		if err != nil {
			return summary, err
		}
		if released {
			summary.IdleSeatsReleased++
			summary.SeatsProcessed++
			summary.HoursCharged += hours
			summary.TotalCharged += charged
		}
	}
	return summary, nil
}

// releaseOneIdleSeat settles whole overdue hours then releases one idle seat.
func (r *bizDecipherRepository) releaseOneIdleSeat(ctx context.Context, seatID int64, now time.Time) (bool, int, float64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		poolID       int64
		userID       int64
		hourlyFee    float64
		waiverMin    float64
		lastCharged  time.Time
		lastActivity time.Time
		status       string
		ownerID      sql.NullInt64
	)
	if err := tx.QueryRowContext(ctx,
		`SELECT sb.pool_id, sb.user_id, sb.hourly_seat_fee, sb.hourly_min_usage_waiver, sb.last_charged_at,
		        COALESCE(sb.last_activity_at, sb.joined_at), sb.status, sp.owner_id
		 FROM pool_seat_bindings sb
		 JOIN shared_pools sp ON sp.id = sb.pool_id
		 WHERE sb.id = $1
		 FOR UPDATE OF sb`,
		seatID,
	).Scan(&poolID, &userID, &hourlyFee, &waiverMin, &lastCharged, &lastActivity, &status, &ownerID); err != nil {
		if err == sql.ErrNoRows {
			return false, 0, 0, tx.Commit()
		}
		return false, 0, 0, err
	}
	if status != "active" {
		return false, 0, 0, tx.Commit()
	}
	if lastActivity.After(now.Add(-service.SharedPoolSeatIdleTimeout)) {
		// Activity arrived after the candidate snapshot; skip this cycle.
		return false, 0, 0, tx.Commit()
	}

	hours, charged, err := chargeSeatHoursTx(ctx, tx, seatID, poolID, userID, ownerID, hourlyFee, waiverMin, lastCharged, now)
	if err != nil && !errors.Is(err, service.ErrInsufficientBalance) {
		return false, 0, 0, err
	}
	// chargeSeatHoursTx already released on insufficient balance; still ensure seat is released.
	if err := releaseSharedPoolSeatTx(ctx, tx, seatID, poolID, userID, "idle_timeout"); err != nil {
		return false, 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return false, 0, 0, err
	}
	return true, hours, charged, nil
}

func (r *bizDecipherRepository) ChargeDueSeats(ctx context.Context, now time.Time) (*service.ChargeSeatsSummary, error) {
	// Snapshot candidate seats that have at least one full hour due.
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM pool_seat_bindings
		 WHERE status = 'active' AND last_charged_at <= ($1::timestamptz - INTERVAL '1 hour')
		 ORDER BY last_charged_at ASC
		 LIMIT 1000`,
		now,
	)
	if err != nil {
		return nil, err
	}
	var seatIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		seatIDs = append(seatIDs, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	summary := &service.ChargeSeatsSummary{}
	for _, seatID := range seatIDs {
		hours, charged, err := r.chargeOneSeat(ctx, seatID, now)
		if err != nil {
			return summary, err
		}
		if hours > 0 {
			summary.SeatsProcessed++
			summary.HoursCharged += hours
			summary.TotalCharged += charged
		}
	}
	return summary, nil
}

// chargeOneSeat charges a single active seat within its own transaction.
func (r *bizDecipherRepository) chargeOneSeat(ctx context.Context, seatID int64, now time.Time) (int, float64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		poolID           int64
		userID           int64
		hourlyFee        float64
		waiverMin        float64
		lastCharged      time.Time
		status           string
		ownerID          sql.NullInt64
		governanceStatus string
		listed           bool
		poolStatus       string
	)
	if err := tx.QueryRowContext(ctx,
		`SELECT sb.pool_id, sb.user_id, sb.hourly_seat_fee, sb.hourly_min_usage_waiver, sb.last_charged_at, sb.status,
		        sp.owner_id, COALESCE(sp.governance_status, ''), COALESCE(sp.listed, FALSE), COALESCE(sp.status, '')
		 FROM pool_seat_bindings sb
		 JOIN shared_pools sp ON sp.id = sb.pool_id
		 WHERE sb.id = $1
		 FOR UPDATE OF sb, sp`,
		seatID,
	).Scan(&poolID, &userID, &hourlyFee, &waiverMin, &lastCharged, &status, &ownerID, &governanceStatus, &listed, &poolStatus); err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, tx.Commit()
		}
		return 0, 0, err
	}
	if status != "active" {
		return 0, 0, tx.Commit()
	}
	poolServiceable := listed && governanceStatus != "banned" && (poolStatus == "healthy" || poolStatus == "limited")
	if !poolServiceable {
		if err := releaseSharedPoolSeatTx(ctx, tx, seatID, poolID, userID, "pool_unavailable"); err != nil {
			return 0, 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, 0, err
		}
		return 0, 0, nil
	}

	hours, charged, err := chargeSeatHoursTx(ctx, tx, seatID, poolID, userID, ownerID, hourlyFee, waiverMin, lastCharged, now)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientBalance) {
			if commitErr := tx.Commit(); commitErr != nil {
				return 0, 0, commitErr
			}
			return hours, charged, nil
		}
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return hours, charged, nil
}

func sharedPoolUsageAmountForWindow(ctx context.Context, qr queryRower, poolID, userID int64, start, end time.Time) (float64, error) {
	if qr == nil || poolID <= 0 || userID <= 0 || start.IsZero() || !end.After(start) {
		return 0, nil
	}
	var amount float64
	if err := qr.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(-amount), 0)
		 FROM shared_pool_balance_ledger
		 WHERE pool_id = $1
		   AND user_id = $2
		   AND source_type = 'share_pool_usage'
		   AND status = 'posted'
		   AND amount < 0
		   AND created_at >= $3
		   AND created_at < $4`,
		poolID, userID, start, end,
	).Scan(&amount); err != nil {
		return 0, err
	}
	return amount, nil
}

func normalizePoolSeatUsageProgress(seat *service.PoolSeat) {
	if seat == nil {
		return
	}
	if seat.CurrentHourUsageAmount < 0 {
		seat.CurrentHourUsageAmount = 0
	}
	if seat.LastActivityAt.IsZero() {
		seat.LastActivityAt = seat.JoinedAt
	}
	if seat.Status == "active" && !seat.LastActivityAt.IsZero() {
		releaseAt := seat.LastActivityAt.Add(service.SharedPoolSeatIdleTimeout)
		seat.IdleReleaseAt = &releaseAt
	} else {
		seat.IdleReleaseAt = nil
	}
	if seat.HourlyMinUsageWaiver <= 0 {
		seat.CurrentHourWaiverRemain = 0
		seat.CurrentHourWaiverMet = false
		return
	}
	remaining := seat.HourlyMinUsageWaiver - seat.CurrentHourUsageAmount
	if remaining < 0 {
		remaining = 0
	}
	seat.CurrentHourWaiverRemain = remaining
	seat.CurrentHourWaiverMet = seat.CurrentHourUsageAmount >= seat.HourlyMinUsageWaiver
}

func hydratePoolSeatUsageProgress(ctx context.Context, qr queryRower, seat *service.PoolSeat) error {
	if seat == nil || seat.Status != "active" || seat.PoolID <= 0 || seat.UserID <= 0 || seat.LastChargedAt.IsZero() {
		normalizePoolSeatUsageProgress(seat)
		return nil
	}
	var hourlyFee, waiverMin float64
	err := qr.QueryRowContext(ctx, `SELECT hourly_seat_fee, hourly_min_usage_waiver
		FROM shared_pool_settlement_rule_versions
		WHERE pool_id = $1 AND effective_from <= $2
		ORDER BY effective_from DESC, id DESC
		LIMIT 1`, seat.PoolID, seat.LastChargedAt.UTC()).Scan(&hourlyFee, &waiverMin)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		seat.HourlySeatFee = hourlyFee
		seat.HourlyMinUsageWaiver = waiverMin
	}
	amount, err := sharedPoolUsageAmountForWindow(ctx, qr, seat.PoolID, seat.UserID, seat.LastChargedAt, seat.LastChargedAt.Add(time.Hour))
	if err != nil {
		return err
	}
	seat.CurrentHourUsageAmount = amount
	normalizePoolSeatUsageProgress(seat)
	return nil
}

// chargeSeatHoursTx charges all whole hours in [lastCharged, now) for the seat,
// inside the given transaction. It advances last_charged_at one hour at a time,
// writes the double-entry ledger rows, and records an idempotent per-hour charge.
// Returns (processedHours, totalDebited). A zero-fee or already-recorded hour
// still advances the clock. Safe to call when no full hour is due (no-op).
func chargeSeatHoursTx(
	ctx context.Context,
	tx *sql.Tx,
	seatID, poolID, userID int64,
	ownerID sql.NullInt64,
	hourlyFee float64,
	waiverMin float64,
	lastCharged time.Time,
	now time.Time,
) (int, float64, error) {
	hoursDue := int(now.Sub(lastCharged) / time.Hour)
	if hoursDue <= 0 {
		return 0, 0, nil
	}

	var total float64
	processedHours := 0
	cursor := lastCharged
	for i := 0; i < hoursDue; i++ {
		periodStart := cursor
		periodEnd := cursor.Add(time.Hour)
		billingHour := periodStart.Truncate(time.Hour)
		cursor = periodEnd

		settlementRule, err := sharedPoolSettlementRuleAtTx(ctx, tx, poolID, billingHour, sharedPoolSettlementRule{
			HourlySeatFee:        hourlyFee,
			HourlyMinUsageWaiver: waiverMin,
		})
		if err != nil {
			return 0, 0, err
		}
		windowHourlyFee := settlementRule.HourlySeatFee
		windowWaiverMin := settlementRule.HourlyMinUsageWaiver

		usageAmount, err := sharedPoolUsageAmountForWindow(ctx, tx, poolID, userID, periodStart, periodEnd)
		if err != nil {
			return 0, 0, err
		}
		feeAmount := windowHourlyFee
		waivedAmount := 0.0
		waiverApplied := false
		if windowHourlyFee > 0 && windowWaiverMin > 0 && usageAmount > 0 {
			waiverRatio := math.Min(1, usageAmount/windowWaiverMin)
			waivedAmount = windowHourlyFee * waiverRatio
			feeAmount = math.Max(0, windowHourlyFee-waivedAmount)
			waiverApplied = waivedAmount > 0
		}
		// The holder balance and shared-pool balance ledger use eight decimal
		// places, while the owner wallet keeps twelve. Quantize once before either
		// side is written so the wallet never receives a fraction the holder did
		// not actually lose after NUMERIC scale conversion.
		windowHourlyFee = math.Round(windowHourlyFee*1e8) / 1e8
		feeAmount = quantizeSharedPoolLedgerDebit(feeAmount)
		if feeAmount > windowHourlyFee {
			feeAmount = windowHourlyFee
		}
		waivedAmount = math.Max(0, math.Round((windowHourlyFee-feeAmount)*1e8)/1e8)
		waiverApplied = waivedAmount > 0

		// Idempotency guard: claim the billing hour before touching balances.
		var chargeID int64
		if err := tx.QueryRowContext(ctx,
			`INSERT INTO pool_seat_charges (seat_id, pool_id, user_id, owner_id, billing_hour, amount, usage_amount, waived_amount, waiver_threshold, waiver_applied)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			 ON CONFLICT (seat_id, billing_hour) DO NOTHING
			 RETURNING id`,
			seatID, poolID, userID, ownerID, billingHour, feeAmount, usageAmount, waivedAmount, windowWaiverMin, waiverApplied,
		).Scan(&chargeID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				processedHours++
				continue
			}
			return 0, 0, err
		}

		var feeLedgerID sql.NullInt64
		var payoutLedgerID sql.NullInt64

		if feeAmount > 0 {
			sourceID := fmt.Sprintf("seat:%d:%s", seatID, billingHour.UTC().Format(time.RFC3339))
			hourLabel := billingHour.UTC().Format("2006-01-02 15:04 UTC")
			note := fmt.Sprintf("共享池席位费 · 池 #%d · 结算窗口 %s", poolID, hourLabel)
			if waiverApplied && waivedAmount > 0 {
				note = fmt.Sprintf("共享池席位费 · 池 #%d · 结算窗口 %s · 本窗消费抵扣 %.4f", poolID, hourLabel, waivedAmount)
			}
			var id int64
			if err := tx.QueryRowContext(ctx,
				`INSERT INTO shared_pool_balance_ledger (user_id, pool_id, source_type, source_id, amount, balance_after, status, note, posted_at, settlement_destination)
				 VALUES ($1, $2, 'pool_seat_fee', $3, 0, 0, 'pending', $4, NULL, 'user_balance')
				 ON CONFLICT DO NOTHING
				 RETURNING id`,
				userID, poolID, sourceID, note,
			).Scan(&id); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return 0, 0, fmt.Errorf("duplicate pool seat fee ledger for %s", sourceID)
				}
				return 0, 0, err
			}
			feeLedgerID = sql.NullInt64{Int64: id, Valid: true}

			// Debit the seat holder from withdrawable balance only after the ledger idempotency guard is claimed.
			var holderBalance float64
			if err := tx.QueryRowContext(ctx,
				`UPDATE users SET balance = balance - $1, updated_at = NOW()
				 WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
				 RETURNING balance`,
				feeAmount, userID,
			).Scan(&holderBalance); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					if _, markErr := tx.ExecContext(ctx,
						`UPDATE shared_pool_balance_ledger
						 SET amount = 0, balance_after = 0, status = 'reversed', note = $2, posted_at = NOW()
						 WHERE id = $1`,
						id, note+" (insufficient balance; seat released)",
					); markErr != nil {
						return 0, 0, markErr
					}
					if _, markChargeErr := tx.ExecContext(ctx,
						`UPDATE pool_seat_charges SET amount = 0, fee_ledger_id = $2 WHERE id = $1`,
						chargeID, id,
					); markChargeErr != nil {
						return 0, 0, markChargeErr
					}
					if processedHours > 0 || total > 0 {
						newLastCharged := lastCharged.Add(time.Duration(processedHours) * time.Hour)
						if _, updateSeatErr := tx.ExecContext(ctx,
							`UPDATE pool_seat_bindings SET last_charged_at = $2, total_charged = total_charged + $3, updated_at = NOW() WHERE id = $1`,
							seatID, newLastCharged, total,
						); updateSeatErr != nil {
							return 0, 0, updateSeatErr
						}
					}
					if err := releaseSharedPoolSeatForInsufficientBalanceTx(ctx, tx, seatID, poolID, userID); err != nil {
						return 0, 0, err
					}
					return processedHours, total, service.ErrInsufficientBalance
				}
				return 0, 0, err
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE shared_pool_balance_ledger
				 SET amount = $2, balance_after = $3, status = 'posted', note = $4, posted_at = NOW()
				 WHERE id = $1`,
				id, -feeAmount, holderBalance, note,
			); err != nil {
				return 0, 0, err
			}

			// Credit the pool owner's dedicated earnings wallet. The owner must
			// explicitly transfer funds to the ordinary site balance.
			if ownerID.Valid && ownerID.Int64 != userID {
				var lockedOwnerID int64
				ownerErr := tx.QueryRowContext(ctx,
					`SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
					ownerID.Int64,
				).Scan(&lockedOwnerID)
				if ownerErr != nil && ownerErr != sql.ErrNoRows {
					return 0, 0, ownerErr
				}
				if ownerErr == nil {
					payoutSource := fmt.Sprintf("seat:%d:%s", seatID, billingHour.UTC().Format(time.RFC3339))
					payoutNote := fmt.Sprintf("共享池席位费分润 · 池 #%d · 结算窗口 %s", poolID, hourLabel)
					credit, err := creditSharedPoolOwnerWalletTx(ctx, tx, sharedPoolOwnerEarningCredit{
						OwnerID:       lockedOwnerID,
						PoolID:        poolID,
						OperationID:   payoutSource,
						PricingSource: "seat_fee",
						GrossAmount:   feeAmount,
						NetAmount:     feeAmount,
						SourceType:    "pool_owner_payout",
						Note:          payoutNote,
					})
					if err != nil {
						return 0, 0, err
					}
					if credit.BalanceLedgerID > 0 {
						payoutLedgerID = sql.NullInt64{Int64: credit.BalanceLedgerID, Valid: true}
					}
				}
			}
			total += feeAmount
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE pool_seat_charges SET fee_ledger_id = $2, payout_ledger_id = $3 WHERE id = $1`,
			chargeID, feeLedgerID, payoutLedgerID,
		); err != nil {
			return 0, 0, err
		}
		processedHours++
	}

	// Advance the seat clock by the whole hours we processed and accrue total.
	newLastCharged := lastCharged.Add(time.Duration(processedHours) * time.Hour)
	if _, err := tx.ExecContext(ctx,
		`UPDATE pool_seat_bindings SET last_charged_at = $2, total_charged = total_charged + $3, updated_at = NOW() WHERE id = $1`,
		seatID, newLastCharged, total,
	); err != nil {
		return 0, 0, err
	}

	return processedHours, total, nil
}

func quantizeSharedPoolLedgerDebit(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return math.Ceil(value*1e8-1e-8) / 1e8
}

// ── Card Skin repo ──────────────────────────────────────────────────────────

// GetUserCollectibleCardRarity returns the server-owned rarity for a card in a
// user's collection. Callers use it instead of trusting a client rarity path.
func (r *bizDecipherRepository) GetUserCollectibleCardRarity(ctx context.Context, userID int64, cardKey string) (string, error) {
	var rarity string
	err := r.db.QueryRowContext(ctx,
		`SELECT LOWER(BTRIM(rarity))
		 FROM checkin_collectible_cards
		 WHERE user_id = $1 AND card_key = $2
		 ORDER BY id DESC
		 LIMIT 1`,
		userID, strings.TrimSpace(cardKey),
	).Scan(&rarity)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return rarity, err
}

// SetPoolCardSkinTx 设置或清除共享池的收藏卡底色（ownerID 用于二次保险校验）。
// cardKey == "" 时表示清除皮肤。
func (r *bizDecipherRepository) SetPoolCardSkinTx(ctx context.Context, poolID, ownerID int64, cardKey, cardRarity string) error {
	var skinKey, skinRarity any
	if cardKey != "" {
		skinKey = cardKey
		skinRarity = cardRarity
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE shared_pools
		 SET card_skin_key = $1, card_skin_rarity = $2, updated_at = NOW()
		 WHERE id = $3 AND owner_id = $4`,
		skinKey, skinRarity, poolID, ownerID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("shared pool card skin update did not match an owned pool")
	}
	return nil
}
