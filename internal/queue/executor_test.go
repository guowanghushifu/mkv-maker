package queue

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guowanghushifu/mkv-maker/internal/remux"
	"github.com/guowanghushifu/mkv-maker/internal/store"
)

type stubExecutionStore struct {
	claimed  store.ExecutionJob
	claimOK  bool
	claimErr error

	running   []string
	completed []string
	failed    map[string]string
	logs      map[string][]string
}

func newStubExecutionStore() *stubExecutionStore {
	return &stubExecutionStore{
		failed: map[string]string{},
		logs:   map[string][]string{},
	}
}

func (s *stubExecutionStore) ClaimNextQueuedJob() (store.ExecutionJob, bool, error) {
	if s.claimErr != nil {
		return store.ExecutionJob{}, false, s.claimErr
	}
	if !s.claimOK {
		return store.ExecutionJob{}, false, nil
	}
	s.claimOK = false
	return s.claimed, true, nil
}

func (s *stubExecutionStore) MarkJobRunning(id string) error {
	s.running = append(s.running, id)
	return nil
}

func (s *stubExecutionStore) MarkJobCompleted(id string) error {
	s.completed = append(s.completed, id)
	return nil
}

func (s *stubExecutionStore) MarkJobFailed(id, message string) error {
	s.failed[id] = message
	return nil
}

func (s *stubExecutionStore) AppendJobLog(id, content string) error {
	s.logs[id] = append(s.logs[id], content)
	return nil
}

type stubRunner struct {
	lastDraft remux.Draft
	output    string
	err       error
}

func (r *stubRunner) Run(_ context.Context, draft remux.Draft) (string, error) {
	r.lastDraft = draft
	return r.output, r.err
}

func TestExecutorExecuteNextReturnsFalseWhenNoQueuedJobs(t *testing.T) {
	jobStore := newStubExecutionStore()
	executor := NewExecutor(jobStore, &stubRunner{})

	processed, err := executor.ExecuteNext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteNext returned error: %v", err)
	}
	if processed {
		t.Fatal("expected no job to be processed")
	}
}

func TestExecutorExecuteNextRunsClaimedJobAndMarksCompleted(t *testing.T) {
	jobStore := newStubExecutionStore()
	jobStore.claimOK = true
	jobStore.claimed = store.ExecutionJob{
		ID: "job-1",
		PayloadJSON: `{
			"source":{"name":"Nightcrawler Disc","path":"/bd_input/Nightcrawler","type":"bdmv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{
				"title":"Nightcrawler",
				"playlistName":"00800.MPLS",
				"dvMergeEnabled":true,
				"video":{"name":"Main Video","codec":"HEVC","resolution":"2160p","hdrType":"HDR.DV"},
				"audio":[{"id":"1","name":"English Atmos","language":"eng","selected":true,"default":true}],
				"subtitles":[{"id":"3","name":"English PGS","language":"eng","selected":true,"default":false,"forced":true}]
			},
			"outputFilename":"Nightcrawler - 2160p.mkv",
			"outputPath":"/remux/Nightcrawler - 2160p.mkv"
		}`,
	}
	runner := &stubRunner{output: "mkvmerge done"}
	executor := NewExecutor(jobStore, runner)

	processed, err := executor.ExecuteNext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteNext returned error: %v", err)
	}
	if !processed {
		t.Fatal("expected one queued job to be processed")
	}

	if len(jobStore.running) != 1 || jobStore.running[0] != "job-1" {
		t.Fatalf("expected job-1 to be marked running, got %+v", jobStore.running)
	}
	if len(jobStore.completed) != 1 || jobStore.completed[0] != "job-1" {
		t.Fatalf("expected job-1 to be marked completed, got %+v", jobStore.completed)
	}
	if len(jobStore.failed) != 0 {
		t.Fatalf("expected no failed jobs, got %+v", jobStore.failed)
	}
	logText := strings.Join(jobStore.logs["job-1"], "\n")
	if !strings.Contains(logText, "mkvmerge done") {
		t.Fatalf("expected command output to be appended to log, got %q", logText)
	}

	expectedInput := filepath.Join("/bd_input/Nightcrawler", "BDMV", "PLAYLIST", "00800.MPLS")
	if runner.lastDraft.SourcePath != expectedInput {
		t.Fatalf("expected resolved source path %q, got %q", expectedInput, runner.lastDraft.SourcePath)
	}
	if len(runner.lastDraft.Subtitles) != 1 {
		t.Fatalf("expected subtitles to be reconstructed in draft, got %+v", runner.lastDraft.Subtitles)
	}
}

func TestExecutorExecuteNextMarksFailureWhenRunnerErrors(t *testing.T) {
	jobStore := newStubExecutionStore()
	jobStore.claimOK = true
	jobStore.claimed = store.ExecutionJob{
		ID: "job-2",
		PayloadJSON: `{
			"source":{"name":"Nightcrawler Disc","path":"/bd_input/Nightcrawler","type":"bdmv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{"playlistName":"00800.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
			"outputFilename":"Nightcrawler - 2160p.mkv",
			"outputPath":"/remux/Nightcrawler - 2160p.mkv"
		}`,
	}
	runner := &stubRunner{
		output: "stderr output",
		err:    errors.New("exec: \"mkvmerge\": executable file not found in $PATH"),
	}
	executor := NewExecutor(jobStore, runner)

	processed, err := executor.ExecuteNext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteNext returned error: %v", err)
	}
	if !processed {
		t.Fatal("expected one queued job to be processed")
	}
	if _, ok := jobStore.failed["job-2"]; !ok {
		t.Fatalf("expected job to be marked failed, got %+v", jobStore.failed)
	}
	if !strings.Contains(jobStore.failed["job-2"], "mkvmerge") {
		t.Fatalf("expected failure message to mention mkvmerge, got %q", jobStore.failed["job-2"])
	}
	if len(jobStore.completed) != 0 {
		t.Fatalf("expected no completed jobs, got %+v", jobStore.completed)
	}
	logText := strings.Join(jobStore.logs["job-2"], "\n")
	if !strings.Contains(logText, "executable file not found") {
		t.Fatalf("expected missing binary error in job log, got %q", logText)
	}
}
