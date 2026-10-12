package app

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"ai-inbox/internal/domain"
)

// OwnerActions implements owner decisions on leads: take, reject, reply.
// It returns domain errors; the delivery layer translates them for the user.
type OwnerActions struct {
	leads  domain.LeadRepository
	outbox domain.OutboxRepository
}

func NewOwnerActions(leads domain.LeadRepository, outbox domain.OutboxRepository) *OwnerActions {
	return &OwnerActions{leads: leads, outbox: outbox}
}

func (s *OwnerActions) Take(ctx context.Context, convID uuid.UUID) error {
	ok, err := s.leads.TakeConversation(ctx, convID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrAlreadyHandled
	}
	return nil
}

func (s *OwnerActions) Reject(ctx context.Context, convID uuid.UUID) error {
	ok, err := s.leads.RejectConversation(ctx, convID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrAlreadyHandled
	}
	return nil
}

// Reply sends the owner's reply (a Telegram reply to a relayed customer
// message) back to the customer via the channel.
func (s *OwnerActions) Reply(ctx context.Context, replyToMsgID int64, text string) error {
	convID, err := s.leads.GetConversationByOwnerForward(ctx, replyToMsgID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNoForwardMapping
		}
		return err
	}
	conv, err := s.leads.GetConversation(ctx, convID)
	if err != nil {
		return err
	}
	if conv.Status == domain.StatusRejected {
		return domain.ErrLeadRejected
	}
	return s.outbox.Enqueue(ctx, conv.ChannelID, conv.ID, text)
}
