package app

import (
	"context"
	"errors"
	"testing"

	"ai-inbox/internal/domain"
)

func TestDeliverSendsAndMarksSent(t *testing.T) {
	s := newFakeStore()
	o := newFakeOutbox()
	sender := &fakeSender{}
	d := NewDeliver(s, o, sender, testLogger())
	_, channelID, convID := seedConversation(t, s, domain.StatusAI)

	if err := o.Enqueue(context.Background(), channelID, convID, "ответ"); err != nil {
		t.Fatal(err)
	}
	item, err := o.Claim(context.Background())
	if err != nil || item == nil {
		t.Fatalf("claim: %v %v", item, err)
	}
	if err := d.Handle(context.Background(), *item); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 1 || sender.sent[0] != "ответ" {
		t.Fatalf("want message sent, got %v", sender.sent)
	}
}

func TestDeliverSendErrorReturnsError(t *testing.T) {
	s := newFakeStore()
	o := newFakeOutbox()
	sender := &fakeSender{err: errors.New("telegram down")}
	d := NewDeliver(s, o, sender, testLogger())
	_, channelID, convID := seedConversation(t, s, domain.StatusAI)

	if err := o.Enqueue(context.Background(), channelID, convID, "ответ"); err != nil {
		t.Fatal(err)
	}
	item, _ := o.Claim(context.Background())
	if item == nil {
		t.Fatal("no item")
	}
	if err := d.Handle(context.Background(), *item); err == nil {
		t.Fatal("want error to schedule retry")
	}
}
