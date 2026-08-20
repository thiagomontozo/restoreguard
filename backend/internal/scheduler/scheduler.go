package scheduler

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"time"
)

type Scheduler struct {
	db       *pgxpool.Pool
	interval time.Duration
	logger   *slog.Logger
}

func New(db *pgxpool.Pool, interval time.Duration, logger *slog.Logger) *Scheduler {
	return &Scheduler{db: db, interval: interval, logger: logger}
}
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}
func (s *Scheduler) tick(ctx context.Context) {
	var locked bool
	if err := s.db.QueryRow(ctx, "SELECT pg_try_advisory_lock(81726354)").Scan(&locked); err != nil || !locked {
		return
	}
	defer s.db.Exec(context.Background(), "SELECT pg_advisory_unlock(81726354)")
	_, err := s.db.Exec(ctx, `UPDATE scheduler_jobs SET lease_owner='restoreguard-scheduler',lease_until=now()+interval '2 minutes',updated_at=now() WHERE next_run_at<=now() AND (lease_until IS NULL OR lease_until<now())`)
	if err != nil {
		s.logger.Error("scheduler tick failed", "error", err)
	}
}
