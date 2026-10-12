package app

import (
	"context"
	"errors"
	"testing"

	"ai-inbox/internal/domain"
)

func TestQualifyHotAutosendsAndNotifies(t *testing.T) {
	s := newFakeStore()
	o := newFakeOutbox()
	n := &fakeNotifier{}
	llm := &fakeLLM{result: domain.QualificationResult{
		Score: domain.ScoreHot, Intent: "заказ", DraftReply: "Здравствуйте!", Confidence: 0.9,
		TokensIn: 100, TokensOut: 50, CostUSD: 0.001,
	}}
	m := &fakeMeter{}
	q := NewQualify(s, o, llm, m, testLogger())
	q.SetNotifier(n)
	_, channelID, convID := seedConversation(t, s, domain.StatusNew)

	task := domain.Task{ID: 1, Kind: "qualify", Attempts: 1,
		Payload: []byte(`{"conversation_id":"` + convID.String() + `"}`)}
	if err := q.Handle(context.Background(), task); err != nil {
		t.Fatal(err)
	}

	pending := o.pendingTexts()
	if len(pending) != 1 || pending[0] != "Здравствуйте!" {
		t.Fatalf("want draft in outbox, got %v", pending)
	}
	if len(n.leads) != 1 || n.leads[0].Score != "hot" {
		t.Fatalf("want 1 hot lead notification, got %+v", n.leads)
	}
	if m.records != 1 {
		t.Fatalf("want usage recorded, got %d", m.records)
	}
	conv, _ := s.GetConversation(context.Background(), convID)
	if conv.Status != domain.StatusAI || conv.Score != "hot" {
		t.Fatalf("conversation must be ai/hot, got %s/%s", conv.Status, conv.Score)
	}
	_ = channelID
}

func TestQualifyWarmNotifiesWithoutAutosend(t *testing.T) {
	s := newFakeStore()
	o := newFakeOutbox()
	n := &fakeNotifier{}
	llm := &fakeLLM{result: domain.QualificationResult{
		Score: domain.ScoreWarm, Intent: "вопрос", DraftReply: "Уточните детали", Confidence: 0.7,
	}}
	q := NewQualify(s, o, llm, &fakeMeter{}, testLogger())
	q.SetNotifier(n)
	_, _, convID := seedConversation(t, s, domain.StatusNew)

	task := domain.Task{ID: 1, Kind: "qualify", Attempts: 1,
		Payload: []byte(`{"conversation_id":"` + convID.String() + `"}`)}
	if err := q.Handle(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if len(o.pendingTexts()) != 0 {
		t.Fatalf("warm must not auto-send, got %v", o.pendingTexts())
	}
	if len(n.leads) != 1 {
		t.Fatalf("want 1 notification, got %d", len(n.leads))
	}
}

func TestQualifyLLMFailureRetriesWithoutNotify(t *testing.T) {
	s := newFakeStore()
	o := newFakeOutbox()
	n := &fakeNotifier{}
	llm := &fakeLLM{err: errors.New("llm down")}
	q := NewQualify(s, o, llm, &fakeMeter{}, testLogger())
	q.SetNotifier(n)
	_, _, convID := seedConversation(t, s, domain.StatusNew)

	task := domain.Task{ID: 1, Kind: "qualify", Attempts: 1,
		Payload: []byte(`{"conversation_id":"` + convID.String() + `"}`)}
	if err := q.Handle(context.Background(), task); err == nil {
		t.Fatal("want error to trigger retry")
	}
	if len(n.leads) != 0 {
		t.Fatalf("early attempts must not notify, got %d", len(n.leads))
	}
}

func TestQualifyFallbackNotifyOnLastAttempt(t *testing.T) {
	s := newFakeStore()
	o := newFakeOutbox()
	n := &fakeNotifier{}
	llm := &fakeLLM{err: errors.New("llm down")}
	q := NewQualify(s, o, llm, &fakeMeter{}, testLogger())
	q.SetNotifier(n)
	_, _, convID := seedConversation(t, s, domain.StatusNew)

	// The lead must never be lost: on the last attempt the owner is notified.
	task := domain.Task{ID: 1, Kind: "qualify", Attempts: MaxQualifyAttempts,
		Payload: []byte(`{"conversation_id":"` + convID.String() + `"}`)}
	if err := q.Handle(context.Background(), task); err == nil {
		t.Fatal("want error (task will be marked failed)")
	}
	if len(n.leads) != 1 || !n.leads[0].Fallback {
		t.Fatalf("want 1 fallback notification, got %+v", n.leads)
	}
	if n.leads[0].CustomerText == "" {
		t.Fatal("fallback notification must contain customer text")
	}
}

func TestQualifySkipsNonNewConversation(t *testing.T) {
	s := newFakeStore()
	o := newFakeOutbox()
	llm := &fakeLLM{result: domain.QualificationResult{Score: domain.ScoreHot, DraftReply: "x", Confidence: 0.9}}
	q := NewQualify(s, o, llm, &fakeMeter{}, testLogger())
	_, _, convID := seedConversation(t, s, domain.StatusHuman)

	task := domain.Task{ID: 1, Kind: "qualify", Attempts: 1,
		Payload: []byte(`{"conversation_id":"` + convID.String() + `"}`)}
	if err := q.Handle(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if llm.calls != 0 {
		t.Fatalf("human conversation must not be qualified, llm called %d times", llm.calls)
	}
}
