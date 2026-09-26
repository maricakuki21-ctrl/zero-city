package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	ChatChannelKindPublic  = "public"
	ChatChannelKindPrivate = "private"

	chatDefaultLimit = 30
	chatMaxLimit     = 100
	chatMaxBodyRunes = 2000
)

var (
	ErrChatChannelNotFound    = infraerrors.NotFound("CHAT_CHANNEL_NOT_FOUND", "chat channel not found")
	ErrChatChannelForbidden   = infraerrors.Forbidden("CHAT_CHANNEL_FORBIDDEN", "chat channel is not open to this account")
	ErrChatMessageIdempotency = infraerrors.Conflict("CHAT_MESSAGE_IDEMPOTENCY_CONFLICT", "client_message_id was already used with different message content")
)

// ChatChannel is a named conversation surface. Public channels are readable and
// writable by every signed-in account; private channels are reserved for later
// direct/inquiry reuse and are rejected here until a membership source exists.
type ChatChannel struct {
	ID          int64     `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Kind        string    `json:"kind"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChatMessage is the persisted wire shape. ID doubles as the reconnect cursor:
// clients send the highest ID they have and receive everything newer.
type ChatMessage struct {
	TokenPacketID   int64     `json:"token_packet_id,omitempty"`
	ID              int64     `json:"id"`
	ChannelID       int64     `json:"channel_id"`
	ChannelSlug     string    `json:"channel_slug"`
	SenderUserID    int64     `json:"sender_user_id"`
	SenderName      string    `json:"sender_name"`
	SenderAvatarURL string    `json:"sender_avatar_url"`
	ClientMessageID string    `json:"client_message_id"`
	Body            string    `json:"body"`
	CreatedAt       time.Time `json:"created_at"`
}

type ChatMessageInput struct {
	ClientMessageID string `json:"client_message_id"`
	Body            string `json:"body"`
}

type ChatMessageQuery struct {
	After int64
	Limit int
}

type ChatMessagePage struct {
	Items []ChatMessage `json:"items"`
	// NextCursor is the highest returned message ID, or the requested After when
	// the page was empty. Clients store it and poll with it again.
	NextCursor int64 `json:"next_cursor"`
	HasMore    bool  `json:"has_more"`
}

type ChatRepository interface {
	ListChatChannels(ctx context.Context) ([]ChatChannel, error)
	GetChatChannelBySlug(ctx context.Context, slug string) (*ChatChannel, error)
	ListChatMessages(ctx context.Context, channelID, afterID int64, limit int) ([]ChatMessage, error)
	AppendChatMessage(ctx context.Context, channelID, senderID int64, input ChatMessageInput) (*ChatMessage, error)
}

type ChatService struct {
	repo ChatRepository
}

func NewChatService(repo ChatRepository) *ChatService {
	return &ChatService{repo: repo}
}

func (s *ChatService) ListChannels(ctx context.Context) ([]ChatChannel, error) {
	return s.repo.ListChatChannels(ctx)
}

func (s *ChatService) ListMessages(ctx context.Context, slug string, userID int64, q ChatMessageQuery) (ChatMessagePage, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" || userID <= 0 || q.After < 0 {
		return ChatMessagePage{}, badChat("invalid chat channel")
	}
	if q.Limit <= 0 {
		q.Limit = chatDefaultLimit
	}
	if q.Limit > chatMaxLimit {
		q.Limit = chatMaxLimit
	}
	channel, err := s.repo.GetChatChannelBySlug(ctx, slug)
	if err != nil {
		return ChatMessagePage{}, err
	}
	if channel.Kind != ChatChannelKindPublic {
		return ChatMessagePage{}, ErrChatChannelForbidden
	}
	items, err := s.repo.ListChatMessages(ctx, channel.ID, q.After, q.Limit)
	if err != nil {
		return ChatMessagePage{}, err
	}
	page := ChatMessagePage{Items: items, NextCursor: q.After}
	for i := range items {
		if items[i].ID > page.NextCursor {
			page.NextCursor = items[i].ID
		}
	}
	return page, nil
}

func (s *ChatService) SendMessage(ctx context.Context, slug string, senderID int64, in ChatMessageInput) (*ChatMessage, error) {
	slug = strings.TrimSpace(slug)
	in.ClientMessageID = strings.TrimSpace(in.ClientMessageID)
	in.Body = strings.TrimSpace(in.Body)
	if slug == "" || senderID <= 0 || in.ClientMessageID == "" || utf8.RuneCountInString(in.ClientMessageID) > 100 ||
		in.Body == "" || utf8.RuneCountInString(in.Body) > chatMaxBodyRunes {
		return nil, badChat("invalid chat message")
	}
	channel, err := s.repo.GetChatChannelBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if channel.Kind != ChatChannelKindPublic {
		return nil, ErrChatChannelForbidden
	}
	return s.repo.AppendChatMessage(ctx, channel.ID, senderID, in)
}

func badChat(message string) error {
	return infraerrors.BadRequest("CHAT_INVALID_ARGUMENT", message)
}
