package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ai-inbox/internal/domain"
)

// TenantRepo implements domain.TenantRepository.
type TenantRepo struct{ pool *pgxpool.Pool }

func NewTenantRepo(pool *pgxpool.Pool) *TenantRepo { return &TenantRepo{pool: pool} }

func (r *TenantRepo) EnsureTenant(ctx context.Context, ownerChatID int64, name string) (*domain.Tenant, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO tenants (name, owner_tg_chat_id) VALUES ($1, $2)
		 ON CONFLICT (owner_tg_chat_id) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id::text, name, owner_tg_chat_id, llm_context::text, policy::text`,
		name, ownerChatID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, domain.ErrNotFound
	}
	return scanTenant(rows)
}

func (r *TenantRepo) GetTenantByOwnerChat(ctx context.Context, ownerChatID int64) (*domain.Tenant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id::text, name, owner_tg_chat_id, llm_context::text, policy::text
		 FROM tenants WHERE owner_tg_chat_id = $1`, ownerChatID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, domain.ErrNotFound
	}
	return scanTenant(rows)
}

func (r *TenantRepo) SetBusinessCard(ctx context.Context, tenantID uuid.UUID, text string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tenants SET llm_context = jsonb_set(COALESCE(llm_context, '{}'::jsonb), '{business_card}', to_jsonb($2::text))
		 WHERE id = $1`, tenantID.String(), text)
	return err
}

func (r *TenantRepo) SetPolicyFlag(ctx context.Context, tenantID uuid.UUID, key string, on bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tenants SET policy = jsonb_set(COALESCE(policy, '{}'::jsonb), ARRAY[$2], to_jsonb($3::boolean))
		 WHERE id = $1`, tenantID.String(), key, on)
	return err
}

func (r *TenantRepo) CreateChannel(ctx context.Context, tenantID uuid.UUID, channelType string, credentials []byte, externalID string) (*domain.Channel, error) {
	rows, err := r.pool.Query(ctx,
		`INSERT INTO channels (tenant_id, type, credentials_encrypted, external_id)
		 VALUES ($1, $2, $3, $4) RETURNING id::text, tenant_id::text, type, external_id, status`,
		tenantID.String(), channelType, credentials, externalID)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, domain.ErrNotFound
	}
	var ch domain.Channel
	var id, tenant string
	if err := rows.Scan(&id, &tenant, &ch.Type, &ch.ExternalID, &ch.Status); err != nil {
		return nil, err
	}
	ch.ID = uuid.MustParse(id)
	ch.TenantID = uuid.MustParse(tenant)
	return &ch, nil
}

func (r *TenantRepo) GetChannel(ctx context.Context, id uuid.UUID) (*domain.Channel, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id::text, tenant_id::text, type, credentials_encrypted, external_id, status
		 FROM channels WHERE id = $1`, id.String())
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, domain.ErrNotFound
	}
	var ch domain.Channel
	var idStr, tenantStr string
	if err := rows.Scan(&idStr, &tenantStr, &ch.Type, &ch.Credentials, &ch.ExternalID, &ch.Status); err != nil {
		return nil, err
	}
	ch.ID = uuid.MustParse(idStr)
	ch.TenantID = uuid.MustParse(tenantStr)
	return &ch, nil
}

func (r *TenantRepo) ListChannels(ctx context.Context, channelType, status string) ([]domain.Channel, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id::text, tenant_id::text, type, credentials_encrypted, external_id, status
		 FROM channels WHERE type = $1 AND status = $2`, channelType, status)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	var out []domain.Channel
	for rows.Next() {
		var ch domain.Channel
		var idStr, tenantStr string
		if err := rows.Scan(&idStr, &tenantStr, &ch.Type, &ch.Credentials, &ch.ExternalID, &ch.Status); err != nil {
			return nil, err
		}
		ch.ID = uuid.MustParse(idStr)
		ch.TenantID = uuid.MustParse(tenantStr)
		out = append(out, ch)
	}
	return out, rows.Err()
}

func (r *TenantRepo) SetChannelStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE channels SET status = $2 WHERE id = $1`, id.String(), status)
	return err
}

func scanTenant(rows pgx.Rows) (*domain.Tenant, error) {
	var t domain.Tenant
	var id string
	var llmCtx, policy string
	if err := rows.Scan(&id, &t.Name, &t.OwnerChatID, &llmCtx, &policy); err != nil {
		return nil, err
	}
	t.ID = uuid.MustParse(id)
	t.LLMContext = json.RawMessage(llmCtx)
	t.Policy = json.RawMessage(policy)
	return &t, nil
}
