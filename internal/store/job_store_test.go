package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

type singleTaskRepositoryContract interface {
	CreateRunningJob(input CreateJobInput) (APIJob, error)
	GetCurrentJob() (APIJob, error)
	GetCurrentJobLog() (string, error)
	GetExecutionJob(id string) (ExecutionJob, error)
	MarkRunningJobsFailed(message string) error
}

var _ singleTaskRepositoryContract = (JobsRepository)(nil)

func TestSQLiteJobStoreCreateListGetAndLog(t *testing.T) {
	db := openJobsTestDB(t)
	logsDir := t.TempDir()
	jobStore := NewSQLiteJobStore(db, logsDir)

	created, err := jobStore.CreateQueuedJob(CreateJobInput{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.TrueHD.7.1.Atmos.mkv",
		OutputPath:   "/remux/Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.TrueHD.7.1.Atmos.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  `{"source":{"name":"Nightcrawler Disc"}}`,
	})
	if err != nil {
		t.Fatalf("CreateQueuedJob returned error: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty job id")
	}
	if created.Status != "queued" {
		t.Fatalf("expected queued status, got %q", created.Status)
	}
	if created.CreatedAt == "" {
		t.Fatal("expected createdAt timestamp")
	}

	items, err := jobStore.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one job in list, got %d", len(items))
	}
	if items[0].ID != created.ID {
		t.Fatalf("expected listed id %q, got %q", created.ID, items[0].ID)
	}

	got, err := jobStore.GetJob(created.ID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.OutputPath != created.OutputPath {
		t.Fatalf("expected output path %q, got %q", created.OutputPath, got.OutputPath)
	}
	rawDraftJSON := getDraftJSONForJob(t, db, created.ID)
	if !strings.Contains(rawDraftJSON, `"source":{"name":"Nightcrawler Disc"}`) {
		t.Fatalf("expected draft_json to preserve payload, got %q", rawDraftJSON)
	}

	logBody, err := jobStore.GetJobLog(created.ID)
	if err != nil {
		t.Fatalf("GetJobLog returned error: %v", err)
	}
	if !strings.Contains(logBody, "remux started") {
		t.Fatalf("expected remux started log text, got %q", logBody)
	}
	if !strings.Contains(logBody, "00800.MPLS") {
		t.Fatalf("expected playlist in log text, got %q", logBody)
	}
	logPath := filepath.Join(logsDir, created.ID+".log")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log file %s to exist: %v", logPath, err)
	}
}

func TestSQLiteJobStoreCreateRunningJobAndGetCurrent(t *testing.T) {
	db := openJobsTestDB(t)
	jobStore := NewSQLiteJobStore(db, t.TempDir())

	created, err := jobStore.CreateRunningJob(CreateJobInput{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler - 2160p.mkv",
		OutputPath:   "/remux/Nightcrawler - 2160p.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  `{"source":{"name":"Nightcrawler Disc"}}`,
	})
	if err != nil {
		t.Fatalf("CreateRunningJob returned error: %v", err)
	}
	if created.Status != "running" {
		t.Fatalf("expected running status, got %q", created.Status)
	}

	current, err := jobStore.GetCurrentJob()
	if err != nil {
		t.Fatalf("GetCurrentJob returned error: %v", err)
	}
	if current.ID != created.ID {
		t.Fatalf("expected current id %q, got %q", created.ID, current.ID)
	}
}

func TestSQLiteJobStoreCreateRunningJobRejectsWhenAnotherTaskIsRunning(t *testing.T) {
	db := openJobsTestDB(t)
	jobStore := NewSQLiteJobStore(db, t.TempDir())

	if _, err := jobStore.CreateRunningJob(CreateJobInput{
		SourceName:   "Disc A",
		OutputName:   "Disc A.mkv",
		OutputPath:   "/remux/Disc A.mkv",
		PlaylistName: "00001.MPLS",
		PayloadJSON:  `{"source":{"name":"Disc A"}}`,
	}); err != nil {
		t.Fatalf("first CreateRunningJob returned error: %v", err)
	}

	_, err := jobStore.CreateRunningJob(CreateJobInput{
		SourceName:   "Disc B",
		OutputName:   "Disc B.mkv",
		OutputPath:   "/remux/Disc B.mkv",
		PlaylistName: "00002.MPLS",
		PayloadJSON:  `{"source":{"name":"Disc B"}}`,
	})
	if !errors.Is(err, ErrJobAlreadyRunning) {
		t.Fatalf("expected ErrJobAlreadyRunning, got %v", err)
	}
}

func TestSQLiteJobStoreMarkRunningJobsFailedOnRecovery(t *testing.T) {
	db := openJobsTestDB(t)
	jobStore := NewSQLiteJobStore(db, t.TempDir())

	created, err := jobStore.CreateRunningJob(CreateJobInput{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler - 2160p.mkv",
		OutputPath:   "/remux/Nightcrawler - 2160p.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  `{"source":{"name":"Nightcrawler Disc"}}`,
	})
	if err != nil {
		t.Fatalf("CreateRunningJob returned error: %v", err)
	}

	if err := jobStore.MarkRunningJobsFailed("process ended before completion"); err != nil {
		t.Fatalf("MarkRunningJobsFailed returned error: %v", err)
	}

	got, err := jobStore.GetJob(created.ID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("expected failed status, got %q", got.Status)
	}
	if !strings.Contains(got.Message, "process ended before completion") {
		t.Fatalf("expected recovery message, got %q", got.Message)
	}
}

func TestSQLiteJobStoreGetMissingReturnsErrJobNotFound(t *testing.T) {
	db := openJobsTestDB(t)
	jobStore := NewSQLiteJobStore(db, t.TempDir())

	_, err := jobStore.GetJob("missing")
	if !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestSQLiteJobStoreClaimTransitionAndPayloadReplay(t *testing.T) {
	db := openJobsTestDB(t)
	logsDir := t.TempDir()
	jobStore := NewSQLiteJobStore(db, logsDir)

	created, err := jobStore.CreateQueuedJob(CreateJobInput{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler - 2160p.mkv",
		OutputPath:   "/remux/Nightcrawler - 2160p.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON: `{
			"source":{"name":"Nightcrawler Disc","path":"/bd_input/Nightcrawler","type":"bdmv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{"playlistName":"00800.MPLS","audio":[{"id":"1","name":"English","language":"eng","selected":true,"default":true}]},
			"outputFilename":"Nightcrawler - 2160p.mkv",
			"outputPath":"/remux/Nightcrawler - 2160p.mkv"
		}`,
	})
	if err != nil {
		t.Fatalf("CreateQueuedJob returned error: %v", err)
	}

	claimed, ok, err := jobStore.ClaimNextQueuedJob()
	if err != nil {
		t.Fatalf("ClaimNextQueuedJob returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected queued job to be claimed")
	}
	if claimed.ID != created.ID {
		t.Fatalf("expected claimed job id %q, got %q", created.ID, claimed.ID)
	}
	if strings.TrimSpace(claimed.PayloadJSON) == "" {
		t.Fatal("expected claimed job to include payload JSON")
	}

	payloadJSON, err := jobStore.GetJobPayloadJSON(created.ID)
	if err != nil {
		t.Fatalf("GetJobPayloadJSON returned error: %v", err)
	}
	if !strings.Contains(payloadJSON, `"path":"/bd_input/Nightcrawler"`) {
		t.Fatalf("expected payload replay data to include source path, got %q", payloadJSON)
	}

	if err := jobStore.MarkJobRunning(created.ID); err != nil {
		t.Fatalf("MarkJobRunning returned error: %v", err)
	}
	if err := jobStore.AppendJobLog(created.ID, "\nworker started"); err != nil {
		t.Fatalf("AppendJobLog returned error: %v", err)
	}
	if err := jobStore.MarkJobCompleted(created.ID); err != nil {
		t.Fatalf("MarkJobCompleted returned error: %v", err)
	}

	got, err := jobStore.GetJob(created.ID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Status != "succeeded" {
		t.Fatalf("expected succeeded status, got %q", got.Status)
	}
	if got.Message != "" {
		t.Fatalf("expected empty error message, got %q", got.Message)
	}

	logBody, err := jobStore.GetJobLog(created.ID)
	if err != nil {
		t.Fatalf("GetJobLog returned error: %v", err)
	}
	if !strings.Contains(logBody, "worker started") {
		t.Fatalf("expected appended log content, got %q", logBody)
	}
}

func TestSQLiteJobStoreMarkJobFailedPersistsMessage(t *testing.T) {
	db := openJobsTestDB(t)
	jobStore := NewSQLiteJobStore(db, t.TempDir())

	created, err := jobStore.CreateQueuedJob(CreateJobInput{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler - 2160p.mkv",
		OutputPath:   "/remux/Nightcrawler - 2160p.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  `{"source":{"name":"Nightcrawler Disc"}}`,
	})
	if err != nil {
		t.Fatalf("CreateQueuedJob returned error: %v", err)
	}

	if err := jobStore.MarkJobRunning(created.ID); err != nil {
		t.Fatalf("MarkJobRunning returned error: %v", err)
	}
	if err := jobStore.MarkJobFailed(created.ID, "mkvmerge executable not found"); err != nil {
		t.Fatalf("MarkJobFailed returned error: %v", err)
	}

	got, err := jobStore.GetJob(created.ID)
	if err != nil {
		t.Fatalf("GetJob returned error: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("expected failed status, got %q", got.Status)
	}
	if !strings.Contains(got.Message, "not found") {
		t.Fatalf("expected failure message to be persisted, got %q", got.Message)
	}
}

func openJobsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return db
}

func getDraftJSONForJob(t *testing.T, db *sql.DB, id string) string {
	t.Helper()
	var draftJSON string
	if err := db.QueryRow(`select draft_json from jobs where id = ?`, id).Scan(&draftJSON); err != nil {
		t.Fatalf("failed to load draft_json: %v", err)
	}
	return draftJSON
}
