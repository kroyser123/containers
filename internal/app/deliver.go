package app

import (
	"context"
	"log/slog"

	"ai-inbox/internal/domain"
)

// Deliver processes outbox items: sends them through channel adapters
// and records the sent messages.
type Deliver struct {
	leads  domain.LeadRepository
	outbox domain.OutboxRepository
	sender domain.MessageSender
	log    *slog.Logger
}

func NewDeliver(leads domain.LeadRepository, outbox domain.OutboxRepository, sender domain.MessageSender, log *slog.Logger) *Deliver {
	return &Deliver{leads: leads, outbox: outbox, sender: sender, log: log}
}

// Handle processes one claimed outbox item. A returned error schedules a retry.
func (s *Deliver) Handle(ctx context.Context, item domain.OutboxItem) error {
	conv, err := s.leads.GetConversation(ctx, item.ConversationID)
	if err != nil {
		s.log.Error("deliver: conversation", "err", err)
		return err
	}
	extMsgID, err := s.sender.Send(ctx, item.ChannelID, conv.ExternalChatID, item.Text)
	if err != nil {
		s.log.Error("deliver: send", "attempt", item.Attempts, "err", err)
		return err
	}
	if err := s.outbox.MarkSent(ctx, item.ID, item.ConversationID, extMsgID, item.Text); err != nil {
		s.log.Error("deliver: mark sent", "err", err)
		return err
	}
	return nil
}
