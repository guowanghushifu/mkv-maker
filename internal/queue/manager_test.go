package queue

import "testing"

type JobRecord struct {
	ID     string
	Status string
}

type memoryStore struct {
	jobs map[string]JobRecord
}

func newMemoryStore() *memoryStore {
	return &memoryStore{jobs: map[string]JobRecord{}}
}

func (m *memoryStore) MarkRunningJobsInterrupted() error {
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
