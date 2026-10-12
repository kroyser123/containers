package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ai-inbox/internal/domain"
)

// OutboxRepo implements domain.OutboxRepository.
type OutboxRepo struct{ pool *pgxpool.Pool }

func NewOutboxRepo(pool *pgxpool.Pool) *OutboxRepo { return &OutboxRepo{pool: pool} }

func (r *OutboxRepo) Enqueue(ctx context.Context, channelID, conversationID uuid.UUID, text string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO outbox (channel_id, conversation_id, text) VALUES ($1, $2, $3)`,
		channelID.String(), conversationID.String(), text)
	return err
}

func (r *OutboxRepo) Claim(ctx context.Context) (*domain.OutboxItem, error) {
	rows, err := r.pool.Query(ctx, `
		UPDATE outbox SET status = 'running', attempts = attempts + 1
		WHERE id = (SELECT id FROM outbox WHERE status = 'pending' AND run_after <= now()
		            ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1)
		RETURNING id, channel_id::text, conversation_id::text, text, attempts`)
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var item domain.OutboxItem
	var channelID, conversationID string
	if err := rows.Scan(&item.ID, &channelID, &conversationID, &item.Text, &item.Attempts); err != nil {
		return nil, err
	}
	item.ChannelID = uuid.MustParse(channelID)
	item.ConversationID = uuid.MustParse(conversationID)
	return &item, nil
}

func (r *OutboxRepo) MarkSent(ctx context.Context, id int64, conversationID uuid.UUID, externalMsgID, text string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE outbox SET status = 'sent', sent_at = now() WHERE id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO messages (conversation_id, direction, external_msg_id, text)
		 VALUES ($1, 'out', $2, $3) ON CONFLICT (conversation_id, external_msg_id) DO NOTHING`,
		conversationID.String(), externalMsgID, text); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *OutboxRepo) Fail(ctx context.Context, id int64, attempts int) error {
	var status string
	var runAfter time.Time
	if attempts >= maxAttempts {
		status = "failed"
		runAfter = time.Now()
	} else {
		status = "pending"
		runAfter = time.Now().Add(backoff(attempts))
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE outbox SET status = $2, run_after = $3 WHERE id = $1`, id, status, runAfter)
	return err
}

const maxAttempts = 5

func backoff(attempts int) time.Duration {
	d := 30 * time.Second << (attempts - 1)
	if d > 10*time.Minute {
		d = 10 * time.Minute
	}
	return d
}
