package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type chatRepoStub struct {
	channels  map[string]ChatChannel
	messages  []ChatMessage
	appendErr error
	appended  []ChatMessageInput
	lastLimit int
}

func (s *chatRepoStub) ListChatChannels(context.Context) ([]ChatChannel, error) {
	out := make([]ChatChannel, 0, len(s.channels))
	for _, channel := range s.channels {
		out = append(out, channel)
	}
	return out, nil
}

func (s *chatRepoStub) GetChatChannelBySlug(_ context.Context, slug string) (*ChatChannel, error) {
	channel, ok := s.channels[slug]
	if !ok {
		return nil, ErrChatChannelNotFound
	}
	return &channel, nil
}

func (s *chatRepoStub) ListChatMessages(_ context.Context, _ int64, _ int64, limit int) ([]ChatMessage, error) {
	s.lastLimit = limit
	return s.messages, nil
}

func (s *chatRepoStub) AppendChatMessage(_ context.Context, channelID, senderID int64, input ChatMessageInput) (*ChatMessage, error) {
	if s.appendErr != nil {
		return nil, s.appendErr
	}
	s.appended = append(s.appended, input)
	return &ChatMessage{ID: 11, ChannelID: channelID, SenderUserID: senderID, Body: input.Body, CreatedAt: time.Now()}, nil
}

func newChatServiceStub() (*ChatService, *chatRepoStub) {
	repo := &chatRepoStub{channels: map[string]ChatChannel{
		"lobby":  {ID: 1, Slug: "lobby", Title: "零号城大厅", Kind: ChatChannelKindPublic},
		"secret": {ID: 2, Slug: "secret", Title: "私密", Kind: ChatChannelKindPrivate},
	}}
	return NewChatService(repo), repo
}

func TestChatServiceSendMessage_validatesAndTrims(t *testing.T) {
	svc, repo := newChatServiceStub()

	_, err := svc.SendMessage(context.Background(), "lobby", 7, ChatMessageInput{ClientMessageID: "c1", Body: "   "})
	require.Error(t, err)

	saved, err := svc.SendMessage(context.Background(), " lobby ", 7, ChatMessageInput{ClientMessageID: " c1 ", Body: " 你好 "})
	require.NoError(t, err)
	require.Equal(t, "你好", saved.Body)
	require.Len(t, repo.appended, 1)
	require.Equal(t, "c1", repo.appended[0].ClientMessageID)
}

func TestChatServiceSendMessage_rejectsPrivateChannel(t *testing.T) {
	svc, repo := newChatServiceStub()

	_, err := svc.SendMessage(context.Background(), "secret", 7, ChatMessageInput{ClientMessageID: "c1", Body: "nope"})
	require.ErrorIs(t, err, ErrChatChannelForbidden)
	require.Empty(t, repo.appended)
}

func TestChatServiceSendMessage_surfacesIdempotencyConflict(t *testing.T) {
	svc, repo := newChatServiceStub()
	repo.appendErr = ErrChatMessageIdempotency

	_, err := svc.SendMessage(context.Background(), "lobby", 7, ChatMessageInput{ClientMessageID: "c1", Body: "first"})
	require.ErrorIs(t, err, ErrChatMessageIdempotency)
}

func TestChatServiceListMessages_capsLimitAndDerivesCursor(t *testing.T) {
	svc, repo := newChatServiceStub()
	repo.messages = []ChatMessage{{ID: 4}, {ID: 9}, {ID: 13}}

	page, err := svc.ListMessages(context.Background(), "lobby", 7, ChatMessageQuery{After: 3, Limit: 5000})
	require.NoError(t, err)
	require.Equal(t, int64(13), page.NextCursor)
	require.Len(t, page.Items, 3)
	require.Equal(t, chatMaxLimit, repo.lastLimit)

	empty, err := svc.ListMessages(context.Background(), "lobby", 7, ChatMessageQuery{After: 13})
	require.NoError(t, err)
	require.Equal(t, int64(13), empty.NextCursor)
	require.Equal(t, chatDefaultLimit, repo.lastLimit)
}
