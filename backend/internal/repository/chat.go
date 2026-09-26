package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type chatRepository struct{ db *sql.DB }

func NewChatRepository(db *sql.DB) service.ChatRepository {
	return &chatRepository{db: db}
}

func (r *chatRepository) ListChatChannels(ctx context.Context) ([]service.ChatChannel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, slug, title, description, kind, sort_order, created_at
		FROM biz_chat_channels WHERE kind = 'public' ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	channels := make([]service.ChatChannel, 0, 4)
	for rows.Next() {
		var channel service.ChatChannel
		if err := rows.Scan(&channel.ID, &channel.Slug, &channel.Title, &channel.Description, &channel.Kind, &channel.SortOrder, &channel.CreatedAt); err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func (r *chatRepository) GetChatChannelBySlug(ctx context.Context, slug string) (*service.ChatChannel, error) {
	var channel service.ChatChannel
	err := r.db.QueryRowContext(ctx, `SELECT id, slug, title, description, kind, sort_order, created_at
		FROM biz_chat_channels WHERE slug = $1`, slug).
		Scan(&channel.ID, &channel.Slug, &channel.Title, &channel.Description, &channel.Kind, &channel.SortOrder, &channel.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrChatChannelNotFound
	}
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

const chatMessageColumns = `
	m.id, m.channel_id, c.slug, m.sender_user_id,
	COALESCE(NULLIF(bp.display_name, ''), NULLIF(u.username, ''), split_part(u.email, '@', 1), 'User'),
	COALESCE(bp.avatar_url, ''), m.client_message_id, m.body, m.created_at,
	COALESCE((SELECT rp.id FROM biz_token_packets rp WHERE rp.message_id=m.id),0)`

func chatMessageFrom() string {
	return ` FROM biz_chat_messages m
		JOIN biz_chat_channels c ON c.id = m.channel_id
		JOIN users u ON u.id = m.sender_user_id
		LEFT JOIN biz_profiles bp ON bp.user_id = m.sender_user_id`
}

func scanChatMessage(row scanner) (*service.ChatMessage, error) {
	var message service.ChatMessage
	if err := row.Scan(
		&message.ID, &message.ChannelID, &message.ChannelSlug, &message.SenderUserID,
		&message.SenderName, &message.SenderAvatarURL, &message.ClientMessageID,
		&message.Body, &message.CreatedAt, &message.TokenPacketID,
	); err != nil {
		return nil, err
	}
	return &message, nil
}

// ListChatMessages returns messages in ascending ID order. With After == 0 it
// returns the newest page so a fresh client sees recent history; with After > 0
// it returns everything newer, which is also the reconnect backfill path.
func (r *chatRepository) ListChatMessages(ctx context.Context, channelID, afterID int64, limit int) ([]service.ChatMessage, error) {
	if afterID > 0 {
		return r.queryChatMessages(ctx, `SELECT `+chatMessageColumns+chatMessageFrom()+
			` WHERE m.channel_id = $1 AND m.id > $2 ORDER BY m.id ASC LIMIT $3`, channelID, afterID, limit)
	}
	items, err := r.queryChatMessages(ctx, `SELECT `+chatMessageColumns+chatMessageFrom()+
		` WHERE m.channel_id = $1 ORDER BY m.id DESC LIMIT $2`, channelID, limit)
	if err != nil {
		return nil, err
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return items, nil
}

func (r *chatRepository) queryChatMessages(ctx context.Context, statement string, args ...any) ([]service.ChatMessage, error) {
	rows, err := r.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ChatMessage, 0, 16)
	for rows.Next() {
		item, scanErr := scanChatMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *chatRepository) AppendChatMessage(ctx context.Context, channelID, senderID int64, input service.ChatMessageInput) (*service.ChatMessage, error) {
	existing, err := r.chatMessageByClientID(ctx, channelID, senderID, input.ClientMessageID)
	if err == nil {
		if existing.Body != input.Body {
			return nil, service.ErrChatMessageIdempotency
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var id int64
	err = r.db.QueryRowContext(ctx, `INSERT INTO biz_chat_messages
		(channel_id, sender_user_id, client_message_id, body) VALUES ($1, $2, $3, $4)
		ON CONFLICT (channel_id, sender_user_id, client_message_id) DO NOTHING
		RETURNING id`, channelID, senderID, input.ClientMessageID, input.Body).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		concurrent, readErr := r.chatMessageByClientID(ctx, channelID, senderID, input.ClientMessageID)
		if readErr != nil {
			return nil, readErr
		}
		if concurrent.Body != input.Body {
			return nil, service.ErrChatMessageIdempotency
		}
		return concurrent, nil
	}
	if err != nil {
		return nil, err
	}
	return scanChatMessage(r.db.QueryRowContext(ctx, `SELECT `+chatMessageColumns+chatMessageFrom()+` WHERE m.id = $1`, id))
}

func (r *chatRepository) chatMessageByClientID(ctx context.Context, channelID, senderID int64, clientMessageID string) (*service.ChatMessage, error) {
	return scanChatMessage(r.db.QueryRowContext(ctx, `SELECT `+chatMessageColumns+chatMessageFrom()+
		` WHERE m.channel_id = $1 AND m.sender_user_id = $2 AND m.client_message_id = $3`,
		channelID, senderID, clientMessageID))
}
