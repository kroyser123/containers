package telegram

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/google/uuid"
)

// TokenSource provides a bot token for a channel (wired in the composition root).
type TokenSource interface {
	Token(ctx context.Context, channelID uuid.UUID) (string, error)
}

// Sender implements domain.MessageSender: it sends messages through channel
// bots, caching one BotAPI client per channel.
type Sender struct {
	src   TokenSource
	mu    sync.Mutex
	apis  map[uuid.UUID]*BotAPI
}

func NewSender(src TokenSource) *Sender {
	return &Sender{src: src, apis: make(map[uuid.UUID]*BotAPI)}
}

func (s *Sender) api(ctx context.Context, channelID uuid.UUID) (*BotAPI, error) {
	s.mu.Lock()
	api, ok := s.apis[channelID]
	s.mu.Unlock()
	if ok {
		return api, nil
	}
	token, err := s.src.Token(ctx, channelID)
	if err != nil {
		return nil, err
	}
	api = NewBotAPI(token)
	s.mu.Lock()
	s.apis[channelID] = api
	s.mu.Unlock()
	return api, nil
}

func (s *Sender) Send(ctx context.Context, channelID uuid.UUID, externalChatID, text string) (string, error) {
	chatID, err := strconv.ParseInt(externalChatID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("bad chat id %q: %w", externalChatID, err)
	}
	api, err := s.api(ctx, channelID)
	if err != nil {
		return "", err
	}
	msgID, err := api.SendMessage(ctx, chatID, text)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(msgID), nil
}
