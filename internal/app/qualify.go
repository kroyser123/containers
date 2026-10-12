package app

import (
	"context"
	"log/slog"

	"ai-inbox/internal/domain"
)

// MaxQualifyAttempts: after this many failed attempts the task dies,
// but the lead is never lost — the owner gets a fallback notification.
const MaxQualifyAttempts = 5

// Qualify runs the LLM qualification pipeline for one conversation.
type Qualify struct {
	leads    domain.LeadRepository
	outbox   domain.OutboxRepository
	llm      domain.LLMQualifier
	notifier domain.OwnerNotifier
	meter    domain.UsageMeter
	log      *slog.Logger
}

func NewQualify(leads domain.LeadRepository, outbox domain.OutboxRepository, llm domain.LLMQualifier, meter domain.UsageMeter, log *slog.Logger) *Qualify {
	return &Qualify{leads: leads, outbox: outbox, llm: llm, meter: meter, log: log}
}

func (s *Qualify) SetNotifier(n domain.OwnerNotifier) { s.notifier = n }

// Handle processes one qualify task. A returned error makes the worker
// retry the task with backoff.
func (s *Qualify) Handle(ctx context.Context, task domain.Task) error {
	convID, err := parseQualifyPayload(task.Payload)
	if err != nil {
		s.log.Error("qualify: bad payload", "err", err)
		return nil // not retryable: drop the task
	}
	conv, tenant, err := s.leads.GetConversationWithTenant(ctx, convID)
	if err != nil {
		return err
	}
	if conv.Status != domain.StatusNew {
		return nil // already qualified, taken or rejected meanwhile
	}
	msgs, err := s.leads.ListLastMessages(ctx, convID, 10)
	if err != nil {
		return err
	}
	transcript := make([]domain.TranscriptMessage, 0, len(msgs))
	var lastCustomerText string
	for _, m := range msgs {
		role := "client"
		if m.Direction == "out" {
			role = "business"
		} else {
			lastCustomerText = m.Text
		}
		transcript = append(transcript, domain.TranscriptMessage{Role: role, Text: m.Text})
	}

	res, err := s.llm.Qualify(ctx, tenant.Name, domain.BusinessCard(tenant.LLMContext), transcript)
	if err != nil {
		s.log.Error("qualify: llm failed", "conversation", convID, "attempt", task.Attempts, "err", err)
		// Never lose a lead: on the last attempt notify the owner to reply manually.
		if task.Attempts >= MaxQualifyAttempts && s.notifier != nil {
			n := domain.LeadNotification{
				ConversationID: convID,
				ChannelType:    conv.ChannelType,
				Score:          "unknown",
				Intent:         "ИИ недоступен",
				CustomerText:   lastCustomerText,
				Fallback:       true,
			}
			if nerr := s.notifier.NotifyLead(ctx, tenant.OwnerChatID, n); nerr != nil {
				s.log.Error("qualify: fallback notify", "err", nerr)
			}
		}
		return err
	}

	if s.meter != nil {
		if err := s.meter.RecordLLMUsage(ctx, tenant.ID, "llm", res.TokensIn, res.TokensOut, res.CostUSD); err != nil {
			s.log.Error("qualify: meter usage", "err", err) // non-fatal
		}
	}
	if err := s.leads.SaveQualification(ctx, convID, res); err != nil {
		return err
	}
	qualified, err := s.leads.MarkConversationQualified(ctx, convID, string(res.Score))
	if err != nil {
		return err
	}
	if !qualified {
		return nil // taken/rejected while we were qualifying
	}

	dec := domain.ApplyPolicy(domain.ParsePolicy(tenant.Policy), res.Score, res.DraftReply, res.Confidence)
	if dec.AutoSend {
		if err := s.outbox.Enqueue(ctx, conv.ChannelID, convID, dec.Draft); err != nil {
			return err
		}
	}
	if dec.Notify && s.notifier != nil {
		n := domain.LeadNotification{
			ConversationID: convID,
			ChannelType:    conv.ChannelType,
			Score:          string(res.Score),
			Intent:         res.Intent,
			DraftReply:     res.DraftReply,
			CustomerText:   lastCustomerText,
			Confidence:     res.Confidence,
		}
		if err := s.notifier.NotifyLead(ctx, tenant.OwnerChatID, n); err != nil {
			s.log.Error("qualify: notify owner", "err", err)
		}
	}
	return nil
}
