package queue

import (
	"context"
	"time"

	"github.com/guowanghushifu/mkv-maker/internal/store"
)

type Worker interface {
	ExecuteNext(ctx context.Context) (bool, error)
}

type Manager struct {
	store        store.JobStore
	worker       Worker
	pollInterval time.Duration
}

func NewManager(jobStore store.JobStore, worker Worker) *Manager {
	return &Manager{
		store:        jobStore,
		worker:       worker,
		pollInterval: 1 * time.Second,
	}
}

func (m *Manager) Recover() error {
	if m == nil || m.store == nil {
		return nil
	}
	return m.store.MarkRunningJobsInterrupted()
}

func (m *Manager) Run(ctx context.Context) {
	if m == nil {
		return
	}
	if err := m.Recover(); err != nil {
		waitForNextPoll(ctx, m.pollInterval)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if m.worker == nil {
			waitForNextPoll(ctx, m.pollInterval)
			continue
		}

		processed, err := m.worker.ExecuteNext(ctx)
		if err != nil {
			waitForNextPoll(ctx, m.pollInterval)
			continue
		}
		if processed {
			continue
		}
		waitForNextPoll(ctx, m.pollInterval)
	}
}

func waitForNextPoll(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
