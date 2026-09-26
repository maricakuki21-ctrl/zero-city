package repository

import (
	"context"
	"database/sql"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) GetCommunityParticipation(ctx context.Context, userID int64) (*service.CommunityParticipation, error) {
	out := &service.CommunityParticipation{UserID: userID}
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(p.level, 0), u.role = 'admin', COALESCE(p.reason, '')
		FROM users u LEFT JOIN community_participation p ON p.user_id = u.id
		WHERE u.id = $1 AND u.deleted_at IS NULL AND u.status = 'active'`, userID).
		Scan(&out.Level, &out.IsAdmin, &out.Reason)
	if err == sql.ErrNoRows {
		return nil, infraerrors.Forbidden("COMMUNITY_ACCOUNT_UNAVAILABLE", "账号不可参与社区")
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *bizDecipherRepository) SetCommunityParticipation(ctx context.Context, actorID, userID int64, level int, reason string) (*service.CommunityParticipation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var target int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL AND status = 'active' FOR UPDATE`, userID).Scan(&target)
	if err == sql.ErrNoRows {
		return nil, infraerrors.NotFound("COMMUNITY_USER_NOT_FOUND", "用户不存在或已停用")
	}
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO community_participation(user_id, level, reason, updated_by) VALUES ($1,$2,$3,$4)
		ON CONFLICT (user_id) DO UPDATE SET level = EXCLUDED.level, reason = EXCLUDED.reason,
			updated_by = EXCLUDED.updated_by, updated_at = NOW()
		WHERE (community_participation.level, community_participation.reason) IS DISTINCT FROM (EXCLUDED.level, EXCLUDED.reason)`,
		userID, level, reason, actorID)
	if err != nil {
		return nil, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if changed > 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO community_participation_history(user_id, level, reason, actor_id) VALUES ($1,$2,$3,$4)`, userID, level, reason, actorID)
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetCommunityParticipation(ctx, userID)
}

func (r *bizDecipherRepository) GetCommunityPostLocation(ctx context.Context, postID int64) (string, string, error) {
	var district, channel string
	err := r.db.QueryRowContext(ctx, `SELECT district, channel FROM community_posts
		WHERE id = $1 AND private = FALSE AND deleted_at IS NULL AND status NOT IN ('hidden','deleted','rejected')`, postID).
		Scan(&district, &channel)
	if err == sql.ErrNoRows {
		err = infraerrors.NotFound("COMMUNITY_POST_NOT_FOUND", "帖子不存在或不可访问")
	}
	return district, channel, err
}
