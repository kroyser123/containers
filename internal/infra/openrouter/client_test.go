package openrouter

import (
	"context"
	"testing"

	"ai-inbox/internal/domain"
)

func TestMockHot(t *testing.T) {
	res, err := NewMock().Qualify(context.Background(), "Ремонт", "", []domain.TranscriptMessage{{Role: "client", Text: "Сколько стоит ремонт кухни?"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Score != domain.ScoreHot {
		t.Fatalf("want hot, got %s", res.Score)
	}
}

func TestMockCold(t *testing.T) {
	res, err := NewMock().Qualify(context.Background(), "Ремонт", "", []domain.TranscriptMessage{{Role: "client", Text: "Предлагаю услуги по SEO продвижению вашего сайта"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Score != domain.ScoreCold {
		t.Fatalf("want cold, got %s", res.Score)
	}
}

func TestMockWarm(t *testing.T) {
	res, err := NewMock().Qualify(context.Background(), "Ремонт", "", []domain.TranscriptMessage{{Role: "client", Text: "Здравствуйте, расскажите чем занимаетесь"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Score != domain.ScoreWarm {
		t.Fatalf("want warm, got %s", res.Score)
	}
}

func TestParseResultFences(t *testing.T) {
	res, err := parseResult("```json\n{\"score\":\"hot\",\"intent\":\"x\",\"draft_reply\":\"ok\",\"confidence\":0.8}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if res.Score != domain.ScoreHot || res.DraftReply != "ok" || res.Confidence != 0.8 {
		t.Fatalf("bad parse: %+v", res)
	}
}

func TestParseResultInvalidScore(t *testing.T) {
	if _, err := parseResult(`{"score":"super","draft_reply":"x"}`); err == nil {
		t.Fatal("want error for invalid score")
	}
}
