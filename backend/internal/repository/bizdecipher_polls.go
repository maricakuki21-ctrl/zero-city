package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const communityPollSelect = `SELECT
	p.id, p.post_id, post.user_id,
	COALESCE(NULLIF(profile.display_name, ''), NULLIF(u.username, ''), split_part(u.email, '@', 1), '用户 #' || post.user_id::text),
	post.title, post.body,
	CASE WHEN p.closed_at IS NOT NULL OR (p.closes_at IS NOT NULL AND p.closes_at <= NOW()) THEN 'closed' ELSE 'open' END,
	p.closes_at, p.closed_at, p.created_at,
	COALESCE((SELECT ballot.option_id FROM community_poll_ballots ballot WHERE ballot.poll_id = p.id AND ballot.user_id = $1), 0),
	($1 > 0 AND (post.user_id = $1 OR $2)),
	COALESCE((SELECT jsonb_agg(jsonb_build_object(
		'id', ranked.id, 'label', ranked.label, 'position', ranked.position, 'vote_count', ranked.vote_count
	) ORDER BY ranked.position) FROM (
		SELECT option.id, option.label, option.position, COUNT(ballot.user_id)::int AS vote_count
		FROM community_poll_options option
		LEFT JOIN community_poll_ballots ballot ON ballot.option_id = option.id
		WHERE option.poll_id = p.id
		GROUP BY option.id, option.label, option.position
	) ranked), '[]'::jsonb),
	(SELECT COUNT(*)::int FROM community_poll_ballots ballot WHERE ballot.poll_id = p.id)
FROM community_polls p
JOIN community_posts post ON post.id = p.post_id
JOIN users u ON u.id = post.user_id
LEFT JOIN biz_profiles profile ON profile.user_id = post.user_id`

type communityPollScanner interface {
	Scan(dest ...any) error
}

func (r *bizDecipherRepository) ListCommunityPolls(ctx context.Context, viewerID int64, viewerIsAdmin bool, limit int) ([]service.CommunityPoll, error) {
	rows, err := r.db.QueryContext(ctx, communityPollSelect+`
		WHERE post.deleted_at IS NULL AND post.private = FALSE AND post.status NOT IN ('hidden', 'deleted')
		ORDER BY (p.closed_at IS NULL AND (p.closes_at IS NULL OR p.closes_at > NOW())) DESC, p.id DESC
		LIMIT $3`, viewerID, viewerIsAdmin, limit)
	if err != nil {
		return nil, fmt.Errorf("list community polls: %w", err)
	}
	defer rows.Close()

	polls := make([]service.CommunityPoll, 0)
	for rows.Next() {
		poll, scanErr := scanCommunityPoll(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		polls = append(polls, *poll)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate community polls: %w", err)
	}
	return polls, nil
}

func (r *bizDecipherRepository) getCommunityPoll(ctx context.Context, pollID, viewerID int64, viewerIsAdmin bool) (*service.CommunityPoll, error) {
	poll, err := scanCommunityPoll(r.db.QueryRowContext(ctx, communityPollSelect+`
		WHERE p.id = $3 AND post.deleted_at IS NULL AND post.private = FALSE AND post.status NOT IN ('hidden', 'deleted')`, viewerID, viewerIsAdmin, pollID))
	if err == sql.ErrNoRows {
		return nil, service.ErrCommunityPollNotFound
	}
	return poll, err
}

func scanCommunityPoll(scanner communityPollScanner) (*service.CommunityPoll, error) {
	var poll service.CommunityPoll
	var closesAt, closedAt sql.NullTime
	var optionsJSON []byte
	if err := scanner.Scan(
		&poll.ID, &poll.PostID, &poll.OwnerUserID, &poll.Author, &poll.Title, &poll.Body, &poll.Status,
		&closesAt, &closedAt, &poll.CreatedAt, &poll.ViewerOptionID, &poll.CanClose, &optionsJSON, &poll.TotalVotes,
	); err != nil {
		return nil, err
	}
	if closesAt.Valid {
		poll.ClosesAt = &closesAt.Time
	}
	if closedAt.Valid {
		poll.ClosedAt = &closedAt.Time
	}
	if err := json.Unmarshal(optionsJSON, &poll.Options); err != nil {
		return nil, fmt.Errorf("decode community poll options: %w", err)
	}
	return &poll, nil
}
