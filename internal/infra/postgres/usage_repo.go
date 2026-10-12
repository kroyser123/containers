package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UsageRepo implements domain.UsageMeter.
type UsageRepo struct{ pool *pgxpool.Pool }

func NewUsageRepo(pool *pgxpool.Pool) *UsageRepo { return &UsageRepo{pool: pool} }

func (r *UsageRepo) RecordLLMUsage(ctx context.Context, tenantID uuid.UUID, model string, tokensIn, tokensOut int, costUSD float64) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO llm_usage (tenant_id, model, prompt_tokens, completion_tokens, cost_usd)
		 VALUES ($1, $2, $3, $4, $5)`,
		tenantID.String(), model, tokensIn, tokensOut, costUSD)
	return err
}
