package openrouter

import (
	"context"
	"fmt"
	"strings"

	"ai-inbox/internal/domain"
)

// Mock is a rule-based stub for local development and tests (no API key).
type Mock struct{}

func NewMock() *Mock { return &Mock{} }

var hotKeywords = []string{"цена", "стоить", "стоит", "сколько", "купить", "заказать", "заказ", "срочно", "нужно сделать", "когда можете", "расценки", "прайс"}
var coldKeywords = []string{"реклам", "ваканси", "резюме", "партнёрств", "партнерств", "предлагаю услуги", "seo", "продвижен"}

func (m *Mock) Qualify(_ context.Context, businessName, _ string, transcript []domain.TranscriptMessage) (domain.QualificationResult, error) {
	var last string
	for i := len(transcript) - 1; i >= 0; i-- {
		if transcript[i].Role == "client" {
			last = transcript[i].Text
			break
		}
	}
	lower := strings.ToLower(last)

	for _, kw := range coldKeywords {
		if strings.Contains(lower, kw) {
			return domain.QualificationResult{
				Score:      domain.ScoreCold,
				Intent:     "спам / нецелевое обращение",
				DraftReply: "Здравствуйте! Спасибо за сообщение, но это не по нашему профилю. Хорошего дня!",
				Confidence: 0.8,
			}, nil
		}
	}
	for _, kw := range hotKeywords {
		if strings.Contains(lower, kw) {
			return domain.QualificationResult{
				Score:      domain.ScoreHot,
				Intent:     last,
				DraftReply: fmt.Sprintf("Здравствуйте! Спасибо за обращение в «%s». Чтобы точно назвать условия, уточните, пожалуйста, детали задачи — ответим сразу.", businessName),
				Confidence: 0.85,
			}, nil
		}
	}
	return domain.QualificationResult{
		Score:      domain.ScoreWarm,
		Intent:     last,
		DraftReply: fmt.Sprintf("Здравствуйте! Спасибо за интерес к «%s». Расскажите подробнее, что нужно — подскажем варианты и сроки.", businessName),
		Confidence: 0.6,
	}, nil
}
