package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ai-inbox/internal/domain"
)

// LeadRepo implements domain.LeadRepository.
type LeadRepo struct{ pool *pgxpool.Pool }

func NewLeadRepo(pool *pgxpool.Pool) *LeadRepo { return &LeadRepo{pool: pool} }

func (r *LeadRepo) UpsertConversationAndMessage(ctx context.Context, tenantID, channelID uuid.UUID, externalChatID, externalMsgID, text string) (uuid.UUID, domain.ConversationStatus, bool, error) {
	const q = `
	WITH conv AS (
		INSERT INTO conversations (tenant_id, channel_id, external_chat_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (channel_id, external_chat_id) DO UPDATE SET updated_at = now()
		RETURNING id::text, status
	), ins AS (
		INSERT INTO messages (conversation_id, direction, external_msg_id, text)
		SELECT id, 'in', $4, $5 FROM conv
		ON CONFLICT (conversation_id, external_msg_id) DO NOTHING
		RETURNING conversation_id
	)
	SELECT c.id, c.status, EXISTS(SELECT 1 FROM ins) FROM conv c`
	var convID, status string
	var inserted bool
	err := r.pool.QueryRow(ctx, q, tenantID.String(), channelID.String(), externalChatID, externalMsgID, text).
		Scan(&convID, &status, &inserted)
	if err != nil {
		return uuid.Nil, "", false, mapErr(err)
	}
	return uuid.MustParse(convID), domain.ConversationStatus(status), inserted, nil
}

func (r *LeadRepo) GetConversation(ctx context.Context, id uuid.UUID) (*domain.Conversation, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.id::text, c.tenant_id::text, c.channel_id::text, c.external_chat_id, c.status, COALESCE(c.score, ''), ch.type
		 FROM conversations c JOIN channels ch ON ch.id = c.channel_id
		 WHERE c.id = $1`, id.String())
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, domain.ErrNotFound
	}
	return scanConversation(rows)
}

func (r *LeadRepo) GetConversationWithTenant(ctx context.Context, id uuid.UUID) (*domain.Conversation, *domain.Tenant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT c.id::text, c.tenant_id::text, c.channel_id::text, c.external_chat_id, c.status, COALESCE(c.score, ''), ch.type,
		        t.id::text, t.name, t.owner_tg_chat_id, t.llm_context::text, t.policy::text
		 FROM conversations c
		 JOIN channels ch ON ch.id = c.channel_id
		 JOIN tenants t ON t.id = c.tenant_id
		 WHERE c.id = $1`, id.String())
	if err != nil {
		return nil, nil, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil, domain.ErrNotFound
	}
	var conv domain.Conversation
	var idStr, tenantStr, channelStr, tID string
	var t domain.Tenant
	var llmCtx, policy string
	if err := rows.Scan(&idStr, &tenantStr, &channelStr, &conv.ExternalChatID, &conv.Status, &conv.Score, &conv.ChannelType,
		&tID, &t.Name, &t.OwnerChatID, &llmCtx, &policy); err != nil {
		return nil, nil, err
	}
	conv.ID = uuid.MustParse(idStr)
	conv.TenantID = uuid.MustParse(tenantStr)
	conv.ChannelID = uuid.MustParse(channelStr)
	t.ID = uuid.MustParse(tID)
	t.LLMContext = json.RawMessage(llmCtx)
	t.Policy = json.RawMessage(policy)
	return &conv, &t, nil
}

func (r *LeadRepo) ListLastMessages(ctx context.Context, conversationID uuid.UUID, limit int) ([]domain.Message, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT direction, text FROM messages
		 WHERE conversation_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2`,
		conversationID.String(), limit)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var out []domain.Message
	for rows.Next() {
		var m domain.Message
		if err := rows.Scan(&m.Direction, &m.Text); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func (r *LeadRepo) MarkConversationQualified(ctx context.Context, id uuid.UUID, score string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE conversations SET status = 'ai', score = $2, updated_at = now()
		 WHERE id = $1 AND status = 'new'`, id.String(), score)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *LeadRepo) TakeConversation(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE conversations SET status = 'human', updated_at = now()
		 WHERE id = $1 AND status IN ('new', 'ai')`, id.String())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *LeadRepo) RejectConversation(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE conversations SET status = 'rejected', updated_at = now()
		 WHERE id = $1 AND status IN ('new', 'ai')`, id.String())
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *LeadRepo) SaveQualification(ctx context.Context, conversationID uuid.UUID, res domain.QualificationResult) error {
	raw, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO qualifications (conversation_id, score, intent, draft_reply, confidence, raw)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		conversationID.String(), string(res.Score), res.Intent, res.DraftReply, res.Confidence, []byte(raw))
	return err
}

func (r *LeadRepo) ConversationStats(ctx context.Context, tenantID uuid.UUID) (map[string]int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT status, count(*) FROM conversations WHERE tenant_id = $1 GROUP BY status`, tenantID.String())
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

func (r *LeadRepo) SaveOwnerForward(ctx context.Context, ownerMsgID int64, conversationID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO owner_forwards (owner_msg_id, conversation_id) VALUES ($1, $2)
		 ON CONFLICT (owner_msg_id) DO NOTHING`, ownerMsgID, conversationID.String())
	return err
}

func (r *LeadRepo) GetConversationByOwnerForward(ctx context.Context, ownerMsgID int64) (uuid.UUID, error) {
	var convID string
	err := r.pool.QueryRow(ctx,
		`SELECT conversation_id::text FROM owner_forwards WHERE owner_msg_id = $1`, ownerMsgID).Scan(&convID)
	if err != nil {
		return uuid.Nil, mapErr(err)
	}
	return uuid.MustParse(convID), nil
}

func scanConversation(rows pgx.Rows) (*domain.Conversation, error) {
	var c domain.Conversation
	var id, tenant, channel string
	if err := rows.Scan(&id, &tenant, &channel, &c.ExternalChatID, &c.Status, &c.Score, &c.ChannelType); err != nil {
		return nil, err
	}
	c.ID = uuid.MustParse(id)
	c.TenantID = uuid.MustParse(tenant)
	c.ChannelID = uuid.MustParse(channel)
	return &c, nil
}
