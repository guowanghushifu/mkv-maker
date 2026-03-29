package queue

import "github.com/wangdazhuo/mkv-maker/internal/store"

type Manager struct {
	store  store.JobStore
	worker any
}

func NewManager(jobStore store.JobStore, worker any) *Manager {
	return &Manager{
		store:  jobStore,
		worker: worker,
	}
}

func (m *Manager) Recover() error {
	if m == nil || m.store == nil {
		return nil
	}
	return m.store.MarkRunningJobsInterrupted()
}
