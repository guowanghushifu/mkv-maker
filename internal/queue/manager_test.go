package queue

import (
	"context"
	"testing"
	"time"
)

type JobRecord struct {
	ID     string
	Status string
}

type memoryStore struct {
	jobs         map[string]JobRecord
	recoverCalls int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{jobs: map[string]JobRecord{}}
}

func (m *memoryStore) MarkRunningJobsInterrupted() error {
	m.recoverCalls++
	for id, job := range m.jobs {
		if job.Status == "running" {
			job.Status = "interrupted"
			m.jobs[id] = job
		}
	}
	return nil
}

func TestRecoverRunningJobsMarksThemInterrupted(t *testing.T) {
	store := newMemoryStore()
	store.jobs["job-1"] = JobRecord{ID: "job-1", Status: "running"}
	manager := NewManager(store, nil)

	if err := manager.Recover(); err != nil {
		t.Fatalf("Recover returned error: %v", err)
	}
	if store.jobs["job-1"].Status != "interrupted" {
		t.Fatalf("expected running job to become interrupted, got %q", store.jobs["job-1"].Status)
	}
}

type stubWorker struct {
	calls int
}

func (w *stubWorker) ExecuteNext(context.Context) (bool, error) {
	w.calls++
	return false, nil
}

func TestRunRecoversBeforePollingWorker(t *testing.T) {
	store := newMemoryStore()
	worker := &stubWorker{}
	manager := NewManager(store, worker)
	manager.pollInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		manager.Run(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("manager did not stop after context cancellation")
	}

	if store.recoverCalls != 1 {
		t.Fatalf("expected one startup recovery call, got %d", store.recoverCalls)
	}
	if worker.calls == 0 {
		t.Fatal("expected worker to be polled at least once")
	}
}
