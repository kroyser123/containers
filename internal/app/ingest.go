// Package app contains use cases. It imports ONLY the domain layer;
// all infrastructure is injected through domain ports.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"ai-inbox/internal/domain"
)

// Ingest is the single entry point for inbound channel messages:
// deduplication + routing by conversation status.
type Ingest struct {
	leads    domain.LeadRepository
	tenants  domain.TenantRepository
	queue    domain.TaskQueue
	notifier domain.OwnerNotifier
	log      *slog.Logger
}

func NewIngest(leads domain.LeadRepository, tenants domain.TenantRepository, queue domain.TaskQueue, log *slog.Logger) *Ingest {
	return &Ingest{leads: leads, tenants: tenants, queue: queue, log: log}
}

func (s *Ingest) SetNotifier(n domain.OwnerNotifier) { s.notifier = n }

func (s *Ingest) Handle(ctx context.Context, msg domain.InboundMessage) {
	ch, err := s.tenants.GetChannel(ctx, msg.ChannelID)
	if err != nil {
		s.log.Error("ingest: channel not found", "channel", msg.ChannelID, "err", err)
		return
	}
	convID, _, inserted, err := s.leads.UpsertConversationAndMessage(ctx, ch.TenantID, msg.ChannelID, msg.ExternalChatID, msg.ExternalMsgID, msg.Text)
	if err != nil {
		s.log.Error("ingest: save message", "err", err)
		return
	}
	if !inserted {
		return // duplicate webhook delivery
	}
	conv, tenant, err := s.leads.GetConversationWithTenant(ctx, convID)
	if err != nil {
		s.log.Error("ingest: load conversation", "err", err)
		return
	}
	switch conv.Status {
	case domain.StatusNew:
		if err := s.queue.Enqueue(ctx, "qualify", map[string]string{"conversation_id": convID.String()}); err != nil {
			s.log.Error("ingest: enqueue task", "err", err)
		}
	case domain.StatusAI:
		s.relay(ctx, tenant.OwnerChatID, convID, msg, true)
	case domain.StatusHuman:
		s.relay(ctx, tenant.OwnerChatID, convID, msg, false)
	case domain.StatusRejected:
		// ignore
	}
}

func (s *Ingest) relay(ctx context.Context, ownerChatID int64, convID uuid.UUID, m domain.InboundMessage, withButtons bool) {
	if s.notifier == nil {
		return
	}
	if err := s.notifier.RelayCustomerMessage(ctx, ownerChatID, convID, m.Text, m.SenderName, withButtons); err != nil {
		s.log.Error("relay to owner", "err", err)
	}
}

// qualifyPayload is the task payload for the qualify queue.
type qualifyPayload struct {
	ConversationID string `json:"conversation_id"`
}

func parseQualifyPayload(raw json.RawMessage) (uuid.UUID, error) {
	var p qualifyPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return uuid.Nil, fmt.Errorf("bad task payload: %w", err)
	}
	id, err := uuid.Parse(p.ConversationID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("bad conversation id: %w", err)
	}
	return id, nil
}
