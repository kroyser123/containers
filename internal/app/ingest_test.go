package app

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"

	"ai-inbox/internal/domain"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func seedConversation(t *testing.T, s *fakeStore, status domain.ConversationStatus) (tenantID, channelID, convID uuid.UUID) {
	t.Helper()
	tenant := &domain.Tenant{ID: uuid.New(), Name: "Тест", OwnerChatID: 111, LLMContext: []byte(`{}`), Policy: []byte(`{}`)}
	s.addTenant(tenant)
	channel := &domain.Channel{ID: uuid.New(), TenantID: tenant.ID, Type: "telegram", ExternalID: "testbot", Status: "active"}
	s.addChannel(channel)
	convID, _, _, err := s.UpsertConversationAndMessage(context.Background(), tenant.ID, channel.ID, "777", "1", "Сколько стоит ремонт?")
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.convs[convID].Status = status
	s.mu.Unlock()
	return tenant.ID, channel.ID, convID
}

func TestIngestNewEnqueuesQualify(t *testing.T) {
	s := newFakeStore()
	q := newFakeQueue()
	ing := NewIngest(s, s, q, testLogger())
	tenantID, channelID, _ := seedConversation(t, s, domain.StatusNew)

	ing.Handle(context.Background(), domain.InboundMessage{
		ChannelID: channelID, ExternalChatID: "888", ExternalMsgID: "5", Text: "Нужно срочно сделать заказ",
	})
	if q.pendingCount() != 1 {
		t.Fatalf("want 1 task, got %d", q.pendingCount())
	}
	_ = tenantID
}

func TestIngestDuplicateIgnored(t *testing.T) {
	s := newFakeStore()
	q := newFakeQueue()
	ing := NewIngest(s, s, q, testLogger())
	_, channelID, _ := seedConversation(t, s, domain.StatusNew)

	msg := domain.InboundMessage{ChannelID: channelID, ExternalChatID: "888", ExternalMsgID: "5", Text: "Привет"}
	ing.Handle(context.Background(), msg)
	ing.Handle(context.Background(), msg) // duplicate webhook delivery
	if q.pendingCount() != 1 {
		t.Fatalf("duplicate must not create a second task, got %d", q.pendingCount())
	}
}

func TestIngestAIRelaysWithButtons(t *testing.T) {
	s := newFakeStore()
	q := newFakeQueue()
	n := &fakeNotifier{}
	ing := NewIngest(s, s, q, testLogger())
	ing.SetNotifier(n)
	_, channelID, _ := seedConversation(t, s, domain.StatusAI)

	ing.Handle(context.Background(), domain.InboundMessage{
		ChannelID: channelID, ExternalChatID: "777", ExternalMsgID: "2", Text: "Ещё вопрос",
	})
	if len(n.relays) != 1 || !n.relays[0].withButtons {
		t.Fatalf("want 1 relay with buttons, got %+v", n.relays)
	}
	if q.pendingCount() != 0 {
		t.Fatalf("ai conversation must not enqueue qualify, got %d", q.pendingCount())
	}
}

func TestIngestHumanRelaysWithoutButtons(t *testing.T) {
	s := newFakeStore()
	n := &fakeNotifier{}
	ing := NewIngest(s, s, newFakeQueue(), testLogger())
	ing.SetNotifier(n)
	_, channelID, _ := seedConversation(t, s, domain.StatusHuman)

	ing.Handle(context.Background(), domain.InboundMessage{
		ChannelID: channelID, ExternalChatID: "777", ExternalMsgID: "3", Text: "Когда начнёте?",
	})
	if len(n.relays) != 1 || n.relays[0].withButtons {
		t.Fatalf("want 1 relay without buttons, got %+v", n.relays)
	}
}

func TestIngestRejectedIgnored(t *testing.T) {
	s := newFakeStore()
	q := newFakeQueue()
	n := &fakeNotifier{}
	ing := NewIngest(s, s, q, testLogger())
	ing.SetNotifier(n)
	_, channelID, _ := seedConversation(t, s, domain.StatusRejected)

	ing.Handle(context.Background(), domain.InboundMessage{
		ChannelID: channelID, ExternalChatID: "777", ExternalMsgID: "4", Text: "Спам",
	})
	if len(n.relays) != 0 || q.pendingCount() != 0 {
		t.Fatalf("rejected conversation must be ignored, relays=%d tasks=%d", len(n.relays), q.pendingCount())
	}
}
