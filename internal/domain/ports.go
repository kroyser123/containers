package domain

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// LeadRepository persists conversations, messages, qualifications and owner forwards.
type LeadRepository interface {
	UpsertConversationAndMessage(ctx context.Context, tenantID, channelID uuid.UUID, externalChatID, externalMsgID, text string) (convID uuid.UUID, status ConversationStatus, inserted bool, err error)
	GetConversation(ctx context.Context, id uuid.UUID) (*Conversation, error)
	GetConversationWithTenant(ctx context.Context, id uuid.UUID) (*Conversation, *Tenant, error)
	ListLastMessages(ctx context.Context, conversationID uuid.UUID, limit int) ([]Message, error)
	MarkConversationQualified(ctx context.Context, id uuid.UUID, score string) (bool, error)
	TakeConversation(ctx context.Context, id uuid.UUID) (bool, error)
	RejectConversation(ctx context.Context, id uuid.UUID) (bool, error)
	SaveQualification(ctx context.Context, conversationID uuid.UUID, res QualificationResult) error
	ConversationStats(ctx context.Context, tenantID uuid.UUID) (map[string]int, error)
	SaveOwnerForward(ctx context.Context, ownerMsgID int64, conversationID uuid.UUID) error
	GetConversationByOwnerForward(ctx context.Context, ownerMsgID int64) (uuid.UUID, error)
}

// TenantRepository persists tenants and channels.
type TenantRepository interface {
	EnsureTenant(ctx context.Context, ownerChatID int64, name string) (*Tenant, error)
	GetTenantByOwnerChat(ctx context.Context, ownerChatID int64) (*Tenant, error)
	SetBusinessCard(ctx context.Context, tenantID uuid.UUID, text string) error
	SetPolicyFlag(ctx context.Context, tenantID uuid.UUID, key string, on bool) error
	CreateChannel(ctx context.Context, tenantID uuid.UUID, channelType string, credentials []byte, externalID string) (*Channel, error)
	GetChannel(ctx context.Context, id uuid.UUID) (*Channel, error)
	ListChannels(ctx context.Context, channelType, status string) ([]Channel, error)
	SetChannelStatus(ctx context.Context, id uuid.UUID, status string) error
}

// Task is a unit of deferred work.
type Task struct {
	ID       int64
	Kind     string
	Payload  json.RawMessage
	Attempts int
}

// TaskQueue is the deferred work queue.
type TaskQueue interface {
	Enqueue(ctx context.Context, kind string, payload any) error
	Claim(ctx context.Context) (*Task, error)
	Finish(ctx context.Context, id int64) error
	Fail(ctx context.Context, id int64, attempts int) error
}

// OutboxItem is a pending outbound message.
type OutboxItem struct {
	ID             int64
	ChannelID      uuid.UUID
	ConversationID uuid.UUID
	Text           string
	Attempts       int
}

// OutboxRepository stores outbound messages until delivered.
type OutboxRepository interface {
	Enqueue(ctx context.Context, channelID, conversationID uuid.UUID, text string) error
	Claim(ctx context.Context) (*OutboxItem, error)
	MarkSent(ctx context.Context, id int64, conversationID uuid.UUID, externalMsgID, text string) error
	Fail(ctx context.Context, id int64, attempts int) error
}

// LLMQualifier qualifies a lead and drafts the first reply.
type LLMQualifier interface {
	Qualify(ctx context.Context, businessName, businessCard string, transcript []TranscriptMessage) (QualificationResult, error)
}

// MessageSender delivers a text message through a channel.
type MessageSender interface {
	Send(ctx context.Context, channelID uuid.UUID, externalChatID, text string) (externalMsgID string, err error)
}

// ChannelRegistry connects/disconnects a channel at the provider side
// (e.g. sets or removes a Telegram webhook).
type ChannelRegistry interface {
	Register(ctx context.Context, channelID uuid.UUID, token string) error
	Unregister(ctx context.Context, channelID uuid.UUID, token string) error
}

// LeadNotification is what the owner sees for a qualified lead.
type LeadNotification struct {
	ConversationID uuid.UUID
	ChannelType    string
	Score          string
	Intent         string
	DraftReply     string
	CustomerText   string
	Confidence     float64
	// Fallback: true when the LLM is unavailable and the owner must reply manually.
	Fallback bool
}

// OwnerNotifier pushes leads and customer messages to the owner.
type OwnerNotifier interface {
	NotifyLead(ctx context.Context, ownerChatID int64, n LeadNotification) error
	RelayCustomerMessage(ctx context.Context, ownerChatID int64, convID uuid.UUID, text, senderName string, withButtons bool) error
}

// CredentialVault encrypts channel credentials at rest.
type CredentialVault interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// UsageMeter records LLM spend per tenant.
type UsageMeter interface {
	RecordLLMUsage(ctx context.Context, tenantID uuid.UUID, model string, tokensIn, tokensOut int, costUSD float64) error
}
