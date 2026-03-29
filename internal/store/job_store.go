package store

type JobStore interface {
	MarkRunningJobsInterrupted() error
}
