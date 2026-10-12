package app

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"ai-inbox/internal/domain"
)

// Workers runs the task and outbox processing loops.
// Both loops are safe to run in multiple instances (SKIP LOCKED).
type Workers struct {
	queue    domain.TaskQueue
	outbox   domain.OutboxRepository
	qualify  *Qualify
	deliver  *Deliver
	log      *slog.Logger
	interval time.Duration
}

func NewWorkers(queue domain.TaskQueue, outbox domain.OutboxRepository, qualify *Qualify, deliver *Deliver, log *slog.Logger) *Workers {
	return &Workers{queue: queue, outbox: outbox, qualify: qualify, deliver: deliver, log: log, interval: time.Second}
}

// SetInterval overrides the polling interval (used in tests).
func (w *Workers) SetInterval(d time.Duration) { w.interval = d }

func (w *Workers) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return w.taskLoop(gctx) })
	g.Go(func() error { return w.outboxLoop(gctx) })
	return g.Wait()
}

func (w *Workers) taskLoop(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		for {
			task, err := w.queue.Claim(ctx)
			if err != nil {
				w.log.Error("claim task", "err", err)
				break
			}
			if task == nil {
				break
			}
			w.handleTask(ctx, *task)
		}
	}
}

func (w *Workers) handleTask(ctx context.Context, task domain.Task) {
	var err error
	switch task.Kind {
	case "qualify":
		err = w.qualify.Handle(ctx, task)
	default:
		w.log.Error("unknown task kind", "kind", task.Kind)
	}
	if err != nil {
		w.log.Error("task failed", "kind", task.Kind, "id", task.ID, "attempt", task.Attempts, "err", err)
		_ = w.queue.Fail(ctx, task.ID, task.Attempts)
		return
	}
	_ = w.queue.Finish(ctx, task.ID)
}

func (w *Workers) outboxLoop(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		for {
			item, err := w.outbox.Claim(ctx)
			if err != nil {
				w.log.Error("claim outbox", "err", err)
				break
			}
			if item == nil {
				break
			}
			if err := w.deliver.Handle(ctx, *item); err != nil {
				_ = w.outbox.Fail(ctx, item.ID, item.Attempts)
			}
		}
	}
}
