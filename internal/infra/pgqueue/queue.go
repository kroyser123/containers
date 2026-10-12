// Package pgqueue implements domain.TaskQueue on PostgreSQL (SKIP LOCKED).
// Safe for horizontal scaling: multiple instances never claim the same task.
package pgqueue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ai-inbox/internal/domain"
)

type Queue struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Queue { return &Queue{pool: pool} }

func (q *Queue) Enqueue(ctx context.Context, kind string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = q.pool.Exec(ctx, `INSERT INTO tasks (kind, payload) VALUES ($1, $2)`, kind, []byte(b))
	return err
}

func (q *Queue) Claim(ctx context.Context) (*domain.Task, error) {
	rows, err := q.pool.Query(ctx, `
		UPDATE tasks SET status = 'running', attempts = attempts + 1
		WHERE id = (SELECT id FROM tasks WHERE status = 'pending' AND run_after <= now()
		            ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1)
		RETURNING id, kind, payload::text, attempts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	var t domain.Task
	var payload string
	if err := rows.Scan(&t.ID, &t.Kind, &payload, &t.Attempts); err != nil {
		return nil, err
	}
	t.Payload = json.RawMessage(payload)
	return &t, nil
}

func (q *Queue) Finish(ctx context.Context, id int64) error {
	_, err := q.pool.Exec(ctx, `UPDATE tasks SET status = 'done' WHERE id = $1`, id)
	return err
}

func (q *Queue) Fail(ctx context.Context, id int64, attempts int) error {
	var status string
	var runAfter time.Time
	if attempts >= 5 {
		status, runAfter = "failed", time.Now()
	} else {
		status = "pending"
		runAfter = time.Now().Add(backoff(attempts))
	}
	_, err := q.pool.Exec(ctx,
		`UPDATE tasks SET status = $2, run_after = $3 WHERE id = $1`, id, status, runAfter)
	return err
}

func backoff(attempts int) time.Duration {
	d := 30 * time.Second << (attempts - 1)
	if d > 10*time.Minute {
		d = 10 * time.Minute
	}
	return d
}
