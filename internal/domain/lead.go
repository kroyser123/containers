// Package domain is the pure core: entities, domain services and ports.
// It must not import anything outside the standard library and uuid.
package domain

import (
	"encoding/json"

	"github.com/google/uuid"
)

type ConversationStatus string

const (
	StatusNew      ConversationStatus = "new"
	StatusAI       ConversationStatus = "ai"
	StatusHuman    ConversationStatus = "human"
	StatusRejected ConversationStatus = "rejected"
)

type Score string

const (
	ScoreHot  Score = "hot"
	ScoreWarm Score = "warm"
	ScoreCold Score = "cold"
)

// InboundMessage is a normalized inbound message from any channel.
type InboundMessage struct {
	ChannelID      uuid.UUID
	ExternalChatID string
	ExternalMsgID  string
	Text           string
	SenderName     string
}

type Tenant struct {
	ID          uuid.UUID
	Name        string
	OwnerChatID int64
	LLMContext  json.RawMessage
	Policy      json.RawMessage
}

type Channel struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Type        string
	Credentials []byte
	ExternalID  string
	Status      string
}

type Conversation struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ChannelID      uuid.UUID
	ExternalChatID string
	Status         ConversationStatus
	Score          string
	ChannelType    string
}

// Message is a transcript entry.
type Message struct {
	Direction string // "in" | "out"
	Text      string
}

// TranscriptMessage feeds the LLM qualifier.
type TranscriptMessage struct {
	Role string // "client" | "business"
	Text string
}

// QualificationResult is the structured LLM output including usage for metering.
type QualificationResult struct {
	Score      Score   `json:"score"`
	Intent     string  `json:"intent"`
	DraftReply string  `json:"draft_reply"`
	Confidence float64 `json:"confidence"`
	TokensIn   int     `json:"-"`
	TokensOut  int     `json:"-"`
	CostUSD    float64 `json:"-"`
}

// BusinessCard extracts the business description from tenant LLM context.
func BusinessCard(raw json.RawMessage) string {
	var ctx struct {
		BusinessCard string `json:"business_card"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &ctx)
	}
	return ctx.BusinessCard
}
