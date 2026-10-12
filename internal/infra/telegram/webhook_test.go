package telegram

import (
	"testing"

	"github.com/google/uuid"
)

func TestInboundFromUpdate(t *testing.T) {
	channelID := uuid.New()
	u := Update{
		UpdateID: 1,
		Message: &TGMessage{
			MessageID: 42,
			From: &struct {
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
				Username  string `json:"username"`
			}{FirstName: "Иван", LastName: "Петров"},
			Text: "Сколько стоит ремонт?",
		},
	}
	u.Message.Chat.ID = 123456

	msg, ok := InboundFromUpdate(channelID, u)
	if !ok {
		t.Fatal("want ok for text message")
	}
	if msg.ChannelID != channelID || msg.ExternalChatID != "123456" || msg.ExternalMsgID != "42" {
		t.Fatalf("bad message: %+v", msg)
	}
	if msg.Text != "Сколько стоит ремонт?" || msg.SenderName != "Иван Петров" {
		t.Fatalf("bad content: %+v", msg)
	}
}

func TestInboundFromUpdateNoText(t *testing.T) {
	u := Update{UpdateID: 1} // no message
	if _, ok := InboundFromUpdate(uuid.New(), u); ok {
		t.Fatal("want not-ok for update without message")
	}
	u.Message = &TGMessage{MessageID: 1}
	u.Message.Chat.ID = 1
	if _, ok := InboundFromUpdate(uuid.New(), u); ok {
		t.Fatal("want not-ok for empty text")
	}
}
