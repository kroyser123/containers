package app

import (
	"context"
	"errors"
	"testing"

	"ai-inbox/internal/domain"
)

func TestOwnerReplyNoMapping(t *testing.T) {
	s := newFakeStore()
	o := newFakeOutbox()
	owner := NewOwnerActions(s, o)

	err := owner.Reply(context.Background(), 999, "ответ")
	if !errors.Is(err, domain.ErrNoForwardMapping) {
		t.Fatalf("want ErrNoForwardMapping, got %v", err)
	}
}

func TestOwnerReplyRejectedLead(t *testing.T) {
	s := newFakeStore()
	o := newFakeOutbox()
	owner := NewOwnerActions(s, o)
	_, _, convID := seedConversation(t, s, domain.StatusRejected)
	_ = s.SaveOwnerForward(context.Background(), 10, convID)

	err := owner.Reply(context.Background(), 10, "ответ")
	if !errors.Is(err, domain.ErrLeadRejected) {
		t.Fatalf("want ErrLeadRejected, got %v", err)
	}
}

func TestOwnerReplyEnqueuesOutbox(t *testing.T) {
	s := newFakeStore()
	o := newFakeOutbox()
	owner := NewOwnerActions(s, o)
	_, channelID, convID := seedConversation(t, s, domain.StatusHuman)
	_ = s.SaveOwnerForward(context.Background(), 10, convID)

	if err := owner.Reply(context.Background(), 10, "ответ клиента"); err != nil {
		t.Fatal(err)
	}
	pending := o.pendingTexts()
	if len(pending) != 1 || pending[0] != "ответ клиента" {
		t.Fatalf("want reply in outbox, got %v", pending)
	}
	_ = channelID
}

func TestOwnerTakeTwice(t *testing.T) {
	s := newFakeStore()
	owner := NewOwnerActions(s, newFakeOutbox())
	_, _, convID := seedConversation(t, s, domain.StatusAI)

	if err := owner.Take(context.Background(), convID); err != nil {
		t.Fatal(err)
	}
	err := owner.Take(context.Background(), convID)
	if !errors.Is(err, domain.ErrAlreadyHandled) {
		t.Fatalf("want ErrAlreadyHandled on second take, got %v", err)
	}
}

func TestOwnerRejectThenTake(t *testing.T) {
	s := newFakeStore()
	owner := NewOwnerActions(s, newFakeOutbox())
	_, _, convID := seedConversation(t, s, domain.StatusNew)

	if err := owner.Reject(context.Background(), convID); err != nil {
		t.Fatal(err)
	}
	err := owner.Take(context.Background(), convID)
	if !errors.Is(err, domain.ErrAlreadyHandled) {
		t.Fatalf("want ErrAlreadyHandled after reject, got %v", err)
	}
}
