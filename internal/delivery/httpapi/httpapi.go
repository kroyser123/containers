// Package httpapi is the HTTP delivery layer: channel webhooks and health.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"ai-inbox/internal/app"
	"ai-inbox/internal/domain"
	"ai-inbox/internal/infra/telegram"
)

func New(ingest *app.Ingest) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Post("/webhooks/telegram/{channelID}", telegramWebhook(ingest))
	// Avito Messenger API webhook stub (real integration: week 2).
	r.Post("/webhooks/avito/{channelID}", avitoWebhook(ingest))
	return r
}

func telegramWebhook(ingest *app.Ingest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		channelID, err := uuid.Parse(chi.URLParam(r, "channelID"))
		if err != nil {
			http.Error(w, "bad channel id", http.StatusBadRequest)
			return
		}
		var update telegram.Update
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, "bad update", http.StatusBadRequest)
			return
		}
		msg, ok := telegram.InboundFromUpdate(channelID, update)
		if !ok {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Telegram expects a fast 200; process asynchronously.
		// Delivery is idempotent, retries are safe.
		go ingest.Handle(context.Background(), msg)
		w.WriteHeader(http.StatusOK)
	}
}

type avitoPayload struct {
	ExternalChatID string `json:"external_chat_id"`
	ExternalMsgID  string `json:"external_msg_id"`
	Text           string `json:"text"`
	SenderName     string `json:"sender_name"`
}

func avitoWebhook(ingest *app.Ingest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		channelID, err := uuid.Parse(chi.URLParam(r, "channelID"))
		if err != nil {
			http.Error(w, "bad channel id", http.StatusBadRequest)
			return
		}
		var p avitoPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}
		if p.ExternalChatID == "" || p.ExternalMsgID == "" {
			http.Error(w, "external_chat_id and external_msg_id are required", http.StatusBadRequest)
			return
		}
		go ingest.Handle(context.Background(), domain.InboundMessage{
			ChannelID:      channelID,
			ExternalChatID: p.ExternalChatID,
			ExternalMsgID:  p.ExternalMsgID,
			Text:           p.Text,
			SenderName:     p.SenderName,
		})
		w.WriteHeader(http.StatusOK)
	}
}
