package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *bizDecipherRepository) CreateCommunityPollTx(ctx context.Context, userID int64, input service.CommunityPollInput) (*service.CommunityPoll, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create community poll: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var postID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO community_posts
		(user_id, kind, title, body, tags, district, channel, private, status, scenario, action_type)
		VALUES ($1, 'feedback', $2, $3, '["治理","投票"]'::jsonb, 'governance', 'votes', FALSE, 'open', 'announcement', 'discuss')
		RETURNING id`, userID, input.Title, input.Body).Scan(&postID)
	if err != nil {
		return nil, fmt.Errorf("create poll community post: %w", err)
	}

	var pollID int64
	if input.ProposalKind == "" {
		input.ProposalKind = "general"
	}
	if input.MinimumVotes <= 0 {
		input.MinimumVotes = 3
	}
	if input.SupportPercent < 51 {
		input.SupportPercent = 60
	}
	if input.DisplayDays <= 0 {
		input.DisplayDays = 7
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO community_polls
		(post_id, closes_at, proposal_kind, minimum_votes, support_percent, display_days)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, postID, input.ClosesAt,
		input.ProposalKind, input.MinimumVotes, input.SupportPercent, input.DisplayDays).Scan(&pollID); err != nil {
		return nil, fmt.Errorf("create community poll: %w", err)
	}
	for index, label := range input.Options {
		if _, err := tx.ExecContext(ctx, `INSERT INTO community_poll_options (poll_id, position, label) VALUES ($1, $2, $3)`, pollID, index+1, label); err != nil {
			return nil, fmt.Errorf("create community poll option: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit community poll: %w", err)
	}
	return r.getCommunityPoll(ctx, pollID, userID, false)
}

func (r *bizDecipherRepository) VoteCommunityPollTx(ctx context.Context, pollID, userID, optionID int64) (*service.CommunityPoll, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin community poll vote: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var ownerID int64
	var closed bool
	err = tx.QueryRowContext(ctx, `SELECT post.user_id, (p.closed_at IS NOT NULL OR (p.closes_at IS NOT NULL AND p.closes_at <= NOW())) FROM community_polls p JOIN community_posts post ON post.id = p.post_id WHERE p.id = $1 AND post.deleted_at IS NULL AND NOT post.private AND post.status NOT IN ('hidden', 'deleted') FOR UPDATE`, pollID).Scan(&ownerID, &closed)
	if err == sql.ErrNoRows {
		return nil, service.ErrCommunityPollNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock community poll for vote: %w", err)
	}
	if closed {
		return nil, service.ErrCommunityPollClosed
	}

	var optionExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM community_poll_options WHERE id = $1 AND poll_id = $2)`, optionID, pollID).Scan(&optionExists); err != nil {
		return nil, fmt.Errorf("check community poll option: %w", err)
	}
	if !optionExists {
		return nil, service.ErrCommunityPollInvalid
	}

	var existingOptionID int64
	err = tx.QueryRowContext(ctx, `SELECT option_id FROM community_poll_ballots WHERE poll_id = $1 AND user_id = $2`, pollID, userID).Scan(&existingOptionID)
	switch {
	case err == nil && existingOptionID != optionID:
		return nil, service.ErrCommunityPollAlreadyVoted
	case err != nil && err != sql.ErrNoRows:
		return nil, fmt.Errorf("read community poll ballot: %w", err)
	case err == sql.ErrNoRows:
		if _, err := tx.ExecContext(ctx, `INSERT INTO community_poll_ballots (poll_id, option_id, user_id) VALUES ($1, $2, $3)`, pollID, optionID, userID); err != nil {
			return nil, fmt.Errorf("create community poll ballot: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit community poll vote: %w", err)
	}
	return r.getCommunityPoll(ctx, pollID, userID, ownerID == userID)
}

func (r *bizDecipherRepository) CloseCommunityPollTx(ctx context.Context, pollID, actorID int64, allowAdmin bool) (*service.CommunityPoll, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin close community poll: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var ownerID int64
	var closed bool
	err = tx.QueryRowContext(ctx, `SELECT post.user_id, (p.closed_at IS NOT NULL OR (p.closes_at IS NOT NULL AND p.closes_at <= NOW())) FROM community_polls p JOIN community_posts post ON post.id = p.post_id WHERE p.id = $1 FOR UPDATE`, pollID).Scan(&ownerID, &closed)
	if err == sql.ErrNoRows {
		return nil, service.ErrCommunityPollNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock community poll for close: %w", err)
	}
	if ownerID != actorID && !allowAdmin {
		return nil, service.ErrCommunityPollForbidden
	}
	{
		if _, err := tx.ExecContext(ctx, `UPDATE community_polls SET closed_at = NOW(), closed_by_user_id = $2, updated_at = NOW() WHERE id = $1 AND closed_at IS NULL AND (proposal_kind = 'announcement' OR closes_at IS NULL OR closes_at > NOW())`, pollID, actorID); err != nil {
			return nil, fmt.Errorf("close community poll: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit community poll close: %w", err)
	}
	return r.getCommunityPoll(ctx, pollID, actorID, allowAdmin)
}
