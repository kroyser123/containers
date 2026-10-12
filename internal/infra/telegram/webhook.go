package telegram

import (
	"strconv"

	"github.com/google/uuid"

	"ai-inbox/internal/domain"
)

// Update is the subset of a Telegram webhook update we care about.
type Update struct {
	UpdateID int64     `json:"update_id"`
	Message  *TGMessage `json:"message"`
}

type TGMessage struct {
	MessageID int `json:"message_id"`
	From      *struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
	} `json:"from"`
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	Text string `json:"text"`
}

// InboundFromUpdate converts a webhook update into a normalized inbound message.
// Returns false for updates without a text message (edited, callbacks, media...).
func InboundFromUpdate(channelID uuid.UUID, u Update) (domain.InboundMessage, bool) {
	if u.Message == nil || u.Message.Text == "" {
		return domain.InboundMessage{}, false
	}
	m := u.Message
	return domain.InboundMessage{
		ChannelID:      channelID,
		ExternalChatID: strconv.FormatInt(m.Chat.ID, 10),
		ExternalMsgID:  strconv.Itoa(m.MessageID),
		Text:           m.Text,
		SenderName:     senderName(m),
	}, true
}

func senderName(m *TGMessage) string {
	if m.From != nil {
		if m.From.FirstName != "" || m.From.LastName != "" {
			return m.From.FirstName + " " + m.From.LastName
		}
		if m.From.Username != "" {
			return m.From.Username
		}
	}
	return "клиент"
}
