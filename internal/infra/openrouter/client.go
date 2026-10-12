// Package openrouter implements domain.LLMQualifier against any
// OpenAI-compatible API (OpenRouter by default), with model fallback
// and usage/cost reporting for metering.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-inbox/internal/domain"
)

const systemPromptTemplate = `Ты — виртуальный ассистент компании «%s». Твоя задача: квалифицировать входящее обращение клиента и подготовить первый ответ от имени компании.

Информация о компании:
%s

Правила квалификации:
- "hot" — клиент готов к покупке или заказу: спрашивает цену, сроки, хочет начать, пишет конкретную задачу.
- "warm" — есть интерес, но клиент задаёт общие вопросы, сравнивает или уточняет детали.
- "cold" — спам, реклама, рекрутинг, партнёрские предложения или обращение не по профилю компании.

Правила ответа:
- Пиши на русском, вежливо, от лица компании, 1–3 предложения.
- Не обещай того, чего нет в информации о компании.
- Если данных не хватает — задай один уточняющий вопрос.

Верни СТРОГО JSON без markdown-обёрток:
{"score":"hot|warm|cold","intent":"краткое описание запроса клиента","draft_reply":"текст ответа","confidence":0.0}
Поле confidence — уверенность классификации от 0.0 до 1.0.`

type Client struct {
	baseURL       string
	apiKey        string
	model         string
	fallbackModel string
	http          *http.Client
}

func New(baseURL, apiKey, model, fallbackModel string) *Client {
	return &Client{
		baseURL:       strings.TrimRight(baseURL, "/"),
		apiKey:        apiKey,
		model:         model,
		fallbackModel: fallbackModel,
		http:          &http.Client{Timeout: 60 * time.Second},
	}
}

type chatRequest struct {
	Model    string        `json:"model,omitempty"`
	Models   []string      `json:"models,omitempty"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Qualify(ctx context.Context, businessName, businessCard string, transcript []domain.TranscriptMessage) (domain.QualificationResult, error) {
	var b strings.Builder
	for _, m := range transcript {
		if m.Role == "business" {
			b.WriteString("Компания: ")
		} else {
			b.WriteString("Клиент: ")
		}
		b.WriteString(m.Text)
		b.WriteString("\n")
	}

	req := chatRequest{
		Messages: []chatMessage{
			{Role: "system", Content: buildSystemPrompt(businessName, businessCard)},
			{Role: "user", Content: "Переписка с клиентом:\n" + b.String()},
		},
	}
	if c.fallbackModel != "" {
		req.Models = []string{c.model, c.fallbackModel}
	} else {
		req.Model = c.model
	}

	raw, err := json.Marshal(req)
	if err != nil {
		return domain.QualificationResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return domain.QualificationResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return domain.QualificationResult{}, fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()
	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return domain.QualificationResult{}, fmt.Errorf("llm response decode (status %s): %w", resp.Status, err)
	}
	if cr.Error != nil {
		return domain.QualificationResult{}, fmt.Errorf("llm error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return domain.QualificationResult{}, fmt.Errorf("llm returned no choices (status %s)", resp.Status)
	}
	res, err := parseResult(cr.Choices[0].Message.Content)
	if err != nil {
		return domain.QualificationResult{}, err
	}
	res.TokensIn = cr.Usage.PromptTokens
	res.TokensOut = cr.Usage.CompletionTokens
	res.CostUSD = cr.Usage.Cost
	return res, nil
}

func buildSystemPrompt(businessName, businessCard string) string {
	if businessCard == "" {
		businessCard = "(информация не заполнена — задай уточняющие вопросы клиенту)"
	}
	return fmt.Sprintf(systemPromptTemplate, businessName, businessCard)
}

func parseResult(content string) (domain.QualificationResult, error) {
	content = stripFences(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return domain.QualificationResult{}, fmt.Errorf("no JSON object in LLM response: %q", content)
	}
	var res domain.QualificationResult
	if err := json.Unmarshal([]byte(content[start:end+1]), &res); err != nil {
		return domain.QualificationResult{}, fmt.Errorf("parse LLM JSON: %w", err)
	}
	switch res.Score {
	case domain.ScoreHot, domain.ScoreWarm, domain.ScoreCold:
	default:
		return domain.QualificationResult{}, fmt.Errorf("invalid score %q", res.Score)
	}
	if res.DraftReply == "" {
		return domain.QualificationResult{}, fmt.Errorf("empty draft reply")
	}
	return res, nil
}

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i >= 0 {
			s = s[i+1:]
		}
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	return strings.TrimSpace(s)
}
