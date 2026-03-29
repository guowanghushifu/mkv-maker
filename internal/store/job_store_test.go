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
	if !strings.Contains(logBody, "queued") {
		t.Fatalf("expected queued log text, got %q", logBody)
	}
	if !strings.Contains(logBody, "00800.MPLS") {
		t.Fatalf("expected playlist in log text, got %q", logBody)
	}
	logPath := filepath.Join(logsDir, created.ID+".log")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log file %s to exist: %v", logPath, err)
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
