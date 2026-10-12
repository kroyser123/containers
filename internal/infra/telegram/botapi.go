// Package telegram is a thin Telegram Bot API client for channel bots.
// Channel bots run in webhook mode: the application stays stateless and
// horizontally scalable (no long polling, no per-channel goroutines).
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const apiBase = "https://api.telegram.org"

// BotAPI is a minimal Telegram Bot API client bound to one bot token.
type BotAPI struct {
	token string
	http  *http.Client
}

func NewBotAPI(token string) *BotAPI {
	return &BotAPI{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
}

func (b *BotAPI) call(ctx context.Context, method string, payload any, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/bot%s/%s", apiBase, b.token, method), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, err)
	}
	defer resp.Body.Close()
	var ar apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return fmt.Errorf("telegram %s: decode (status %d): %w", method, resp.StatusCode, err)
	}
	if !ar.OK {
		return fmt.Errorf("telegram %s: %s (status %d)", method, ar.Description, resp.StatusCode)
	}
	if result != nil {
		return json.Unmarshal(ar.Result, result)
	}
	return nil
}

// GetMe validates the token and returns the bot username.
func (b *BotAPI) GetMe(ctx context.Context) (string, error) {
	var me struct {
		Username string `json:"username"`
	}
	if err := b.call(ctx, "getMe", map[string]any{}, &me); err != nil {
		return "", err
	}
	return me.Username, nil
}

// SetWebhook registers the webhook URL for this bot.
func (b *BotAPI) SetWebhook(ctx context.Context, url string) error {
	return b.call(ctx, "setWebhook", map[string]string{"url": url}, nil)
}

// DeleteWebhook removes the webhook.
func (b *BotAPI) DeleteWebhook(ctx context.Context) error {
	return b.call(ctx, "deleteWebhook", map[string]string{}, nil)
}

// SendMessage sends a text message and returns its message id.
func (b *BotAPI) SendMessage(ctx context.Context, chatID int64, text string) (int, error) {
	var msg struct {
		MessageID int `json:"message_id"`
	}
	err := b.call(ctx, "sendMessage", map[string]any{
		"chat_id": chatID,
		"text":    text,
	}, &msg)
	return msg.MessageID, err
}
