package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const governanceRuleSelect = `SELECT
	gr.id, gr.source_poll_id, gr.title, gr.body, gr.status, gr.tally_snapshot,
	gr.adopted_by_user_id,
	COALESCE(NULLIF(profile.display_name, ''), NULLIF(adopter.username, ''), split_part(adopter.email, '@', 1), ''),
	gr.adopted_at, gr.revoked_by_user_id, gr.revoked_at, gr.revoked_reason
FROM community_governance_rules gr
LEFT JOIN users adopter ON adopter.id = gr.adopted_by_user_id
LEFT JOIN biz_profiles profile ON profile.user_id = gr.adopted_by_user_id`

type governanceRuleScanner interface {
	Scan(dest ...any) error
}

func scanGovernanceRule(row governanceRuleScanner) (*service.GovernanceRule, error) {
	var (
		rule         service.GovernanceRule
		sourcePollID sql.NullInt64
		adoptedBy    sql.NullInt64
		revokedBy    sql.NullInt64
		revokedAt    sql.NullTime
		tally        []byte
	)
	if err := row.Scan(&rule.ID, &sourcePollID, &rule.Title, &rule.Body, &rule.Status, &tally,
		&adoptedBy, &rule.AdoptedBy, &rule.AdoptedAt, &revokedBy, &revokedAt, &rule.RevokedReason); err != nil {
		return nil, err
	}
	if sourcePollID.Valid {
		value := sourcePollID.Int64
		rule.SourcePollID = &value
	}
	if adoptedBy.Valid {
		value := adoptedBy.Int64
		rule.AdoptedByUserID = &value
	}
	if revokedBy.Valid {
		value := revokedBy.Int64
		rule.RevokedByUserID = &value
	}
	if revokedAt.Valid {
		value := revokedAt.Time
		rule.RevokedAt = &value
	}
	rule.TallySnapshot = map[string]any{}
	if len(tally) > 0 {
		if err := json.Unmarshal(tally, &rule.TallySnapshot); err != nil {
			return nil, fmt.Errorf("decode governance rule tally: %w", err)
		}
	}
	return &rule, nil
}

func (r *bizDecipherRepository) ListGovernanceRules(ctx context.Context, limit int) ([]service.GovernanceRule, error) {
	rows, err := r.db.QueryContext(ctx, governanceRuleSelect+`
		WHERE gr.status = 'adopted'
		ORDER BY gr.adopted_at DESC, gr.id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list governance rules: %w", err)
	}
	defer rows.Close()

	rules := make([]service.GovernanceRule, 0)
	for rows.Next() {
		rule, scanErr := scanGovernanceRule(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		rules = append(rules, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate governance rules: %w", err)
	}
	return rules, nil
}

// AdoptCommunityPollAsRuleTx turns a closed poll into an adopted rule. The vote
// tally is frozen into the rule so a later vote change cannot rewrite history.
func (r *bizDecipherRepository) AdoptCommunityPollAsRuleTx(
	ctx context.Context,
	pollID, actorID int64,
	allowAdmin bool,
) (*service.GovernanceRule, error) {
	if !allowAdmin {
		return nil, service.ErrGovernanceForbidden
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		ownerUserID int64
		title       string
		body        string
		isClosed    bool
	)
	err = tx.QueryRowContext(ctx, `SELECT
		post.user_id, post.title, post.body,
		(p.closed_at IS NOT NULL OR (p.closes_at IS NOT NULL AND p.closes_at <= NOW()))
	FROM community_polls p
	JOIN community_posts post ON post.id = p.post_id
	WHERE p.id = $1 AND post.deleted_at IS NULL AND post.private = FALSE
	AND post.status NOT IN ('hidden', 'deleted') FOR UPDATE OF p, post`, pollID).Scan(&ownerUserID, &title, &body, &isClosed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGovernanceRuleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load governance poll: %w", err)
	}
	if !isClosed {
		return nil, service.ErrGovernancePollNotClosed
	}
	if ownerUserID != actorID && !allowAdmin {
		return nil, service.ErrGovernanceForbidden
	}

	tally, err := communityPollTallyJSON(ctx, tx, pollID)
	if err != nil {
		return nil, err
	}

	var ruleID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO community_governance_rules
		(source_poll_id, title, body, status, tally_snapshot, adopted_by_user_id, adopted_at)
		VALUES ($1, $2, $3, 'adopted', $4::jsonb, $5, NOW())
		ON CONFLICT (source_poll_id) WHERE source_poll_id IS NOT NULL AND status = 'adopted'
		DO NOTHING
		RETURNING id`, pollID, title, body, tally, actorID).Scan(&ruleID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGovernanceRuleConflict
	}
	if err != nil {
		return nil, fmt.Errorf("adopt governance rule: %w", err)
	}

	rule, err := scanGovernanceRule(tx.QueryRowContext(ctx, governanceRuleSelect+` WHERE gr.id = $1`, ruleID))
	if err != nil {
		return nil, fmt.Errorf("load adopted governance rule: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rule, nil
}

func communityPollTallyJSON(ctx context.Context, tx *sql.Tx, pollID int64) (string, error) {
	var total int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*)::int FROM community_poll_ballots WHERE poll_id = $1`, pollID).Scan(&total); err != nil {
		return "", fmt.Errorf("count governance ballots: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT option.label, COUNT(ballot.user_id)::int
		FROM community_poll_options option
		LEFT JOIN community_poll_ballots ballot ON ballot.option_id = option.id
		WHERE option.poll_id = $1
		GROUP BY option.id, option.label, option.position
		ORDER BY option.position`, pollID)
	if err != nil {
		return "", fmt.Errorf("load governance tally: %w", err)
	}
	defer rows.Close()

	type optionTally struct {
		Label     string `json:"label"`
		VoteCount int    `json:"vote_count"`
	}
	options := make([]optionTally, 0, 4)
	for rows.Next() {
		var entry optionTally
		if err := rows.Scan(&entry.Label, &entry.VoteCount); err != nil {
			return "", err
		}
		options = append(options, entry)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{"total_votes": total, "options": options})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func (r *bizDecipherRepository) RevokeGovernanceRuleTx(
	ctx context.Context,
	ruleID, actorID int64,
	reason string,
) (*service.GovernanceRule, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `UPDATE community_governance_rules
		SET status = 'revoked', revoked_by_user_id = $2, revoked_at = NOW(), revoked_reason = $3, updated_at = NOW()
		WHERE id = $1 AND status = 'adopted'`, ruleID, actorID, reason)
	if err != nil {
		return nil, fmt.Errorf("revoke governance rule: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, service.ErrGovernanceRuleNotFound
	}

	rule, err := scanGovernanceRule(tx.QueryRowContext(ctx, governanceRuleSelect+` WHERE gr.id = $1`, ruleID))
	if err != nil {
		return nil, fmt.Errorf("load revoked governance rule: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rule, nil
}

func (r *bizDecipherRepository) ListCommunityBadges(ctx context.Context) ([]service.CommunityBadge, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, badge_key, name, description, sort_order FROM community_badges ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("list community badges: %w", err)
	}
	defer rows.Close()

	badges := make([]service.CommunityBadge, 0)
	for rows.Next() {
		var badge service.CommunityBadge
		if err := rows.Scan(&badge.ID, &badge.Key, &badge.Name, &badge.Description, &badge.SortOrder); err != nil {
			return nil, err
		}
		badges = append(badges, badge)
	}
	return badges, rows.Err()
}

const userBadgeSelect = `SELECT ub.id, badge.badge_key, badge.name, badge.description, ub.user_id,
	ub.reason, ub.granted_by_user_id, ub.granted_at, ub.revoked_at, ub.revoked_reason
FROM community_user_badges ub
JOIN community_badges badge ON badge.id = ub.badge_id`

func scanUserBadge(row governanceRuleScanner) (*service.CommunityBadgeGrant, error) {
	var (
		grant      service.CommunityBadgeGrant
		grantedBy  sql.NullInt64
		revokedAt  sql.NullTime
	)
	if err := row.Scan(&grant.GrantID, &grant.BadgeKey, &grant.BadgeName, &grant.Description, &grant.UserID,
		&grant.Reason, &grantedBy, &grant.GrantedAt, &revokedAt, &grant.RevokedReason); err != nil {
		return nil, err
	}
	if grantedBy.Valid {
		value := grantedBy.Int64
		grant.GrantedBy = &value
	}
	if revokedAt.Valid {
		value := revokedAt.Time
		grant.RevokedAt = &value
	}
	return &grant, nil
}

func (r *bizDecipherRepository) ListUserBadges(ctx context.Context, userID int64) ([]service.CommunityBadgeGrant, error) {
	rows, err := r.db.QueryContext(ctx, userBadgeSelect+`
		WHERE ub.user_id = $1 AND ub.revoked_at IS NULL
		ORDER BY badge.sort_order, ub.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user badges: %w", err)
	}
	defer rows.Close()

	grants := make([]service.CommunityBadgeGrant, 0)
	for rows.Next() {
		grant, scanErr := scanUserBadge(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		grants = append(grants, *grant)
	}
	return grants, rows.Err()
}

func (r *bizDecipherRepository) GrantCommunityBadgeTx(
	ctx context.Context,
	badgeKey string,
	userID, actorID int64,
	reason string,
) (*service.CommunityBadgeGrant, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var badgeID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM community_badges WHERE badge_key = $1`, badgeKey).Scan(&badgeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrGovernanceBadgeNotFound
		}
		return nil, fmt.Errorf("load community badge: %w", err)
	}
	var targetExists bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`, userID).Scan(&targetExists); err != nil {
		return nil, err
	}
	if !targetExists {
		return nil, service.ErrGovernanceInvalid
	}

	var grantID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO community_user_badges
		(badge_id, user_id, reason, granted_by_user_id, granted_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (badge_id, user_id) WHERE revoked_at IS NULL DO NOTHING
		RETURNING id`, badgeID, userID, reason, actorID).Scan(&grantID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGovernanceBadgeAlreadyHeld
	}
	if err != nil {
		return nil, fmt.Errorf("grant community badge: %w", err)
	}

	grant, err := scanUserBadge(tx.QueryRowContext(ctx, userBadgeSelect+` WHERE ub.id = $1`, grantID))
	if err != nil {
		return nil, fmt.Errorf("load granted community badge: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return grant, nil
}

func (r *bizDecipherRepository) RevokeCommunityBadgeTx(
	ctx context.Context,
	badgeKey string,
	userID, actorID int64,
	reason string,
) (*service.CommunityBadgeGrant, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var grantID int64
	err = tx.QueryRowContext(ctx, `UPDATE community_user_badges ub
		SET revoked_at = NOW(), revoked_by_user_id = $3, revoked_reason = $4
		FROM community_badges badge
		WHERE ub.badge_id = badge.id AND badge.badge_key = $1 AND ub.user_id = $2 AND ub.revoked_at IS NULL
		RETURNING ub.id`, badgeKey, userID, actorID, strings.TrimSpace(reason)).Scan(&grantID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGovernanceBadgeNotGranted
	}
	if err != nil {
		return nil, fmt.Errorf("revoke community badge: %w", err)
	}
	grant, err := scanUserBadge(tx.QueryRowContext(ctx, userBadgeSelect+` WHERE ub.id = $1`, grantID))
	if err != nil {
		return nil, fmt.Errorf("load revoked community badge: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return grant, nil
}
