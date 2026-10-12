package telegram

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// Registry implements domain.ChannelRegistry: it (un)registers channel bot
// webhooks at the Telegram side.
type Registry struct {
	baseURL string
}

func NewRegistry(publicBaseURL string) *Registry {
	return &Registry{baseURL: strings.TrimRight(publicBaseURL, "/")}
}

func (r *Registry) Register(ctx context.Context, channelID uuid.UUID, token string) error {
	api := NewBotAPI(token)
	return api.SetWebhook(ctx, r.baseURL+"/webhooks/telegram/"+channelID.String())
}

func (r *Registry) Unregister(ctx context.Context, _ uuid.UUID, token string) error {
	api := NewBotAPI(token)
	return api.DeleteWebhook(ctx)
}
