# Single Remux + Track Editor Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace queued remux execution with a single background remux task shown live on the review page, and refactor the track editor into compact drag-sortable audio/subtitle tables with checkbox-style default controls.

**Architecture:** Keep the existing SQLite-backed `jobs` table, but change its semantics from queued work to a single current remux record. The backend starts remux immediately after request acceptance, exposes current-task and current-log endpoints, and the frontend polls those endpoints from the review page. On the editor page, keep the existing draft shape but move row ordering and default exclusivity into focused frontend helpers used by a denser table-based UI.

**Tech Stack:** Go, Chi, SQLite, React 18, TypeScript, Vitest, Testing Library, native HTML drag-and-drop

---

## File Map

- Modify: `internal/store/job_store.go`
  Responsibility: single-task persistence APIs, current-task queries, startup recovery semantics, log bootstrap text
- Modify: `internal/store/job_store_test.go`
  Responsibility: store behavior for running/current tasks, recovery, log access, conflict detection
- Create: `internal/remux/job_runner.go`
  Responsibility: execute one persisted remux task immediately, append lifecycle logs, mark completion/failure
- Create: `internal/remux/job_runner_test.go`
  Responsibility: direct execution tests without queue polling
- Modify: `internal/http/handlers/jobs.go`
  Responsibility: create-start conflict handling, `current` status endpoint, `current/log` endpoint
- Modify: `internal/http/handlers/jobs_test.go`
  Responsibility: handler tests for immediate start, conflict, current-task retrieval, log retrieval
- Modify: `internal/http/router.go`
  Responsibility: expose `GET /api/jobs/current` and `GET /api/jobs/current/log`, remove list route usage
- Modify: `internal/http/router_test.go`
  Responsibility: route coverage for current-task endpoints
- Modify: `internal/app/app.go`
  Responsibility: startup recovery, runner wiring, remove queue manager loop
- Delete: `internal/queue/manager.go`
- Delete: `internal/queue/manager_test.go`
- Delete: `internal/queue/executor.go`
- Delete: `internal/queue/executor_test.go`
- Modify: `web/src/api/types.ts`
  Responsibility: remove queue-only statuses and list assumptions
- Modify: `web/src/api/client.ts`
  Responsibility: current-task fetch/log fetch methods, submit conflict handling
- Modify: `web/src/App.tsx`
  Responsibility: remove jobs step/page state, keep review active after submit, poll current remux while running
- Modify: `web/src/components/Layout.tsx`
  Responsibility: remove jobs step from workflow indicator
- Modify: `web/src/components/StatusBadge.tsx`
  Responsibility: single-task status labels only
- Delete: `web/src/features/jobs/JobsPage.tsx`
- Modify: `web/src/features/review/ReviewPage.tsx`
  Responsibility: current-task panel, status badge, live log viewer, start button wording
- Create: `web/src/features/draft/trackTable.ts`
  Responsibility: reorder helpers and exclusive-default helpers for track tables
- Modify: `web/src/features/draft/TrackEditorPage.tsx`
  Responsibility: table-based editor UI, drag-and-drop wiring, checkbox default behavior
- Modify: `web/src/styles/app.css`
  Responsibility: compact table styling, review current-task panel styling, remove old jobs-page styling
- Modify: `web/src/test/TrackEditorPage.test.tsx`
  Responsibility: default exclusivity, deselect-clears-default, drag reorder smoke coverage, inline edits
- Create: `web/src/test/trackTable.test.ts`
  Responsibility: pure helper tests for reorder/default logic
- Create: `web/src/test/ReviewPage.test.tsx`
  Responsibility: current-task panel rendering and running-state polling surface
- Modify: `web/src/test/App.test.tsx`
  Responsibility: smoke test remains aligned with new workflow shell

### Task 1: Refactor Job Store for Single Current Task

**Files:**
- Modify: `internal/store/job_store.go`
- Modify: `internal/store/job_store_test.go`

- [ ] **Step 1: Write the failing store tests for single-task semantics**

```go
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
```

- [ ] **Step 2: Run the store tests and verify they fail for missing APIs**

Run: `go test ./internal/store -run 'TestSQLiteJobStoreCreateRunningJobAndGetCurrent|TestSQLiteJobStoreCreateRunningJobRejectsWhenAnotherTaskIsRunning|TestSQLiteJobStoreMarkRunningJobsFailedOnRecovery' -v`

Expected: FAIL with undefined `CreateRunningJob`, `GetCurrentJob`, `ErrJobAlreadyRunning`, or mismatched recovery behavior.

- [ ] **Step 3: Implement the minimal single-task store API**

```go
var (
	ErrJobNotFound       = errors.New("job not found")
	ErrJobAlreadyRunning = errors.New("job already running")
)

func (s *SQLiteJobStore) CreateRunningJob(input CreateJobInput) (APIJob, error) {
	if s == nil || s.db == nil {
		return APIJob{}, errors.New("job store is not configured")
	}

	var runningCount int
	if err := s.db.QueryRow(`select count(1) from jobs where status = 'running'`).Scan(&runningCount); err != nil {
		return APIJob{}, err
	}
	if runningCount > 0 {
		return APIJob{}, ErrJobAlreadyRunning
	}

	id, err := generateJobID()
	if err != nil {
		return APIJob{}, err
	}
	if _, err := s.db.Exec(
		`insert into jobs(id, status, draft_json, output_path, log_path, started_at) values(?, ?, ?, ?, ?, current_timestamp)`,
		id,
		"running",
		draftJSON,
		strings.TrimSpace(input.OutputPath),
		logPath,
	); err != nil {
		return APIJob{}, err
	}
	return s.GetJob(id)
}

func (s *SQLiteJobStore) GetCurrentJob() (APIJob, error) {
	var (
		id         string
		status     string
		draftJSON  string
		outputPath string
		errorText  string
		createdAt  string
	)
	err := s.db.QueryRow(`
		select id, status, draft_json, output_path, error_text, created_at
		from jobs
		order by case when status = 'running' then 0 else 1 end, datetime(created_at) desc, id desc
		limit 1
	`).Scan(&id, &status, &draftJSON, &outputPath, &errorText, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIJob{}, ErrJobNotFound
		}
		return APIJob{}, err
	}
	return buildAPIJob(id, status, draftJSON, outputPath, errorText, createdAt), nil
}

func (s *SQLiteJobStore) MarkRunningJobsFailed(message string) error {
	_, err := s.db.Exec(`
		update jobs
		set status = 'failed',
		    error_text = ?,
		    finished_at = current_timestamp
		where status = 'running'
	`, strings.TrimSpace(message))
	return err
}

func (s *SQLiteJobStore) GetExecutionJob(id string) (ExecutionJob, error) {
	var job ExecutionJob
	err := s.db.QueryRow(`
		select id, draft_json, output_path, log_path
		from jobs
		where id = ?
	`, strings.TrimSpace(id)).Scan(&job.ID, &job.PayloadJSON, &job.OutputPath, &job.LogPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ExecutionJob{}, ErrJobNotFound
		}
		return ExecutionJob{}, err
	}
	return job, nil
}
```

- [ ] **Step 4: Update bootstrap log text and current-log behavior**

```go
func buildInitialJobLog(playlistName, outputPath string) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	lines := []string{"[" + timestamp + "] remux started"}
	if strings.TrimSpace(playlistName) != "" {
		lines = append(lines, "Resolving playlist "+strings.TrimSpace(playlistName))
	}
	if strings.TrimSpace(outputPath) != "" {
		lines = append(lines, "Preparing output "+strings.TrimSpace(outputPath))
	}
	return strings.Join(lines, "\n")
}

func (s *SQLiteJobStore) GetCurrentJobLog() (string, error) {
	job, err := s.GetCurrentJob()
	if err != nil {
		return "", err
	}
	return s.GetJobLog(job.ID)
}
```

- [ ] **Step 5: Run the store tests and confirm they pass**

Run: `go test ./internal/store -run 'TestSQLiteJobStore(CreateRunningJobAndGetCurrent|CreateRunningJobRejectsWhenAnotherTaskIsRunning|MarkRunningJobsFailedOnRecovery|CreateListGetAndLog|MarkJobFailedPersistsMessage)' -v`

Expected: PASS for the updated single-task store behavior.

- [ ] **Step 6: Commit the store refactor**

```bash
git add internal/store/job_store.go internal/store/job_store_test.go
git commit -m "refactor: simplify remux job storage"
```

### Task 2: Replace Queue Polling with Immediate Remux Execution

**Files:**
- Create: `internal/remux/job_runner.go`
- Create: `internal/remux/job_runner_test.go`
- Delete: `internal/queue/manager.go`
- Delete: `internal/queue/manager_test.go`
- Delete: `internal/queue/executor.go`
- Delete: `internal/queue/executor_test.go`

- [ ] **Step 1: Write the failing direct-runner tests**

```go
type stubRunStore struct {
	jobs      map[string]store.ExecutionJob
	completed []string
	failed    map[string]string
	logs      map[string][]string
}

func newStubRunStore() *stubRunStore {
	return &stubRunStore{
		jobs:   map[string]store.ExecutionJob{},
		failed: map[string]string{},
		logs:   map[string][]string{},
	}
}

func (s *stubRunStore) GetExecutionJob(id string) (store.ExecutionJob, error) {
	job, ok := s.jobs[id]
	if !ok {
		return store.ExecutionJob{}, store.ErrJobNotFound
	}
	return job, nil
}

func (s *stubRunStore) MarkJobCompleted(id string) error {
	s.completed = append(s.completed, id)
	return nil
}

func (s *stubRunStore) MarkJobFailed(id, message string) error {
	s.failed[id] = message
	return nil
}

func (s *stubRunStore) AppendJobLog(id, content string) error {
	s.logs[id] = append(s.logs[id], content)
	return nil
}

type stubRunner struct {
	output string
	err    error
}

func (r *stubRunner) Run(_ context.Context, draft Draft) (string, error) {
	return r.output, r.err
}

func TestJobRunnerRunJobMarksCompletionAndAppendsOutput(t *testing.T) {
	store := newStubRunStore()
	store.jobs["job-1"] = store.ExecutionJob{
		ID: "job-1",
		PayloadJSON: `{
			"source":{"name":"Nightcrawler Disc","path":"/bd_input/Nightcrawler","type":"bdmv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{"title":"Nightcrawler","playlistName":"00800.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
			"outputPath":"/remux/Nightcrawler - 2160p.mkv"
		}`,
		OutputPath: "/remux/Nightcrawler - 2160p.mkv",
	}

	runner := NewJobRunner(store, &stubRunner{output: "mkvmerge progress"})
	if err := runner.RunJob(context.Background(), "job-1"); err != nil {
		t.Fatalf("RunJob returned error: %v", err)
	}
	if len(store.completed) != 1 || store.completed[0] != "job-1" {
		t.Fatalf("expected job-1 completed, got %+v", store.completed)
	}
	if !strings.Contains(strings.Join(store.logs["job-1"], "\n"), "mkvmerge progress") {
		t.Fatalf("expected runner output in log, got %+v", store.logs["job-1"])
	}
}

func TestJobRunnerRunJobMarksFailureWhenRunnerErrors(t *testing.T) {
	store := newStubRunStore()
	store.jobs["job-2"] = store.ExecutionJob{
		ID: "job-2",
		PayloadJSON: `{
			"source":{"name":"Nightcrawler Disc","path":"/bd_input/Nightcrawler","type":"bdmv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{"playlistName":"00800.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
			"outputPath":"/remux/Nightcrawler - 2160p.mkv"
		}`,
		OutputPath: "/remux/Nightcrawler - 2160p.mkv",
	}

	runner := NewJobRunner(store, &stubRunner{err: errors.New("exec: \"mkvmerge\": executable file not found in $PATH")})
	if err := runner.RunJob(context.Background(), "job-2"); err != nil {
		t.Fatalf("RunJob returned error: %v", err)
	}
	if _, ok := store.failed["job-2"]; !ok {
		t.Fatalf("expected failed job, got %+v", store.failed)
	}
}
```

- [ ] **Step 2: Run the direct-runner tests and verify they fail**

Run: `go test ./internal/remux -run 'TestJobRunnerRunJobMarksCompletionAndAppendsOutput|TestJobRunnerRunJobMarksFailureWhenRunnerErrors' -v`

Expected: FAIL because `NewJobRunner` and `RunJob` do not exist yet.

- [ ] **Step 3: Implement direct execution without queue polling**

```go
type JobExecutionStore interface {
	GetJobPayloadJSON(id string) (string, error)
	MarkJobCompleted(id string) error
	MarkJobFailed(id, message string) error
	AppendJobLog(id, content string) error
	GetExecutionJob(id string) (store.ExecutionJob, error)
}

type JobRunner struct {
	store  JobExecutionStore
	runner CommandRunner
}

func NewJobRunner(jobStore JobExecutionStore, runner CommandRunner) *JobRunner {
	if runner == nil {
		runner = MKVMergeRunner{}
	}
	return &JobRunner{store: jobStore, runner: runner}
}

func (r *JobRunner) RunJob(ctx context.Context, id string) error {
	job, err := r.store.GetExecutionJob(strings.TrimSpace(id))
	if err != nil {
		return err
	}
	output, runErr := r.runner.Run(ctx, draft)
	if output != "" {
		_ = r.store.AppendJobLog(job.ID, normalizeLogChunk(output))
	}
	if runErr != nil {
		message := normalizeRunnerError(runErr)
		_ = r.store.AppendJobLog(job.ID, logLine(message))
		return r.store.MarkJobFailed(job.ID, message)
	}
	_ = r.store.AppendJobLog(job.ID, logLine("completed"))
	return r.store.MarkJobCompleted(job.ID)
}
```

- [ ] **Step 4: Run the remux tests and confirm the direct runner passes**

Run: `go test ./internal/remux -run 'TestJobRunnerRunJobMarksCompletionAndAppendsOutput|TestJobRunnerRunJobMarksFailureWhenRunnerErrors' -v`

Expected: PASS with no `internal/queue` dependency.

- [ ] **Step 5: Remove queue-only files**

```bash
rm internal/queue/manager.go
rm internal/queue/manager_test.go
rm internal/queue/executor.go
rm internal/queue/executor_test.go
```

- [ ] **Step 6: Commit the execution refactor**

```bash
git add internal/remux/job_runner.go internal/remux/job_runner_test.go internal/queue/manager.go internal/queue/manager_test.go internal/queue/executor.go internal/queue/executor_test.go
git commit -m "refactor: run remux jobs immediately"
```

### Task 3: Expose Current Remux Endpoints and App Wiring

**Files:**
- Modify: `internal/http/handlers/jobs.go`
- Modify: `internal/http/handlers/jobs_test.go`
- Modify: `internal/http/router.go`
- Modify: `internal/http/router_test.go`
- Modify: `internal/app/app.go`

- [ ] **Step 1: Write failing handler tests for current-task APIs and conflict handling**

```go
func TestJobsHandlerCreateStartsRunningJob(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Nightcrawler", "BDMV")
	if err := os.MkdirAll(filepath.Join(sourcePath, "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "PLAYLIST", "00800.MPLS"), []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	outputRoot := t.TempDir()
	h := NewJobsHandler(stubJobsRepository{
		createRunningFn: func(input store.CreateJobInput) (store.APIJob, error) {
			return store.APIJob{
				ID:           "job-123",
				SourceName:   input.SourceName,
				OutputName:   input.OutputName,
				OutputPath:   input.OutputPath,
				PlaylistName: input.PlaylistName,
				CreatedAt:    "2026-03-29T12:00:00Z",
				Status:       "running",
			}, nil
		},
	}, inputRoot, outputRoot, nil)

	reqBody := `{
		"source":{"id":"Nightcrawler","name":"Nightcrawler Disc","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"},
		"draft":{"sourceId":"Nightcrawler","playlistName":"00800.MPLS"},
		"outputFilename":"Nightcrawler - 2160p.mkv",
		"outputPath":"` + filepath.Join(outputRoot, "Nightcrawler - 2160p.mkv") + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var body store.APIJob
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if body.Status != "running" {
		t.Fatalf("expected running status, got %+v", body)
	}
}

func TestJobsHandlerCreateReturnsConflictWhenTaskAlreadyRunning(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Nightcrawler", "BDMV")
	if err := os.MkdirAll(filepath.Join(sourcePath, "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "PLAYLIST", "00800.MPLS"), []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	outputRoot := t.TempDir()
	h := NewJobsHandler(stubJobsRepository{
		createRunningFn: func(input store.CreateJobInput) (store.APIJob, error) {
			return store.APIJob{}, store.ErrJobAlreadyRunning
		},
	}, inputRoot, outputRoot, nil)

	reqBody := `{
		"source":{"id":"Nightcrawler","name":"Nightcrawler Disc","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"},
		"draft":{"sourceId":"Nightcrawler","playlistName":"00800.MPLS"},
		"outputFilename":"Nightcrawler - 2160p.mkv",
		"outputPath":"` + filepath.Join(outputRoot, "Nightcrawler - 2160p.mkv") + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestJobsHandlerCurrentAndCurrentLog(t *testing.T) {
	h := NewJobsHandler(stubJobsRepository{
		currentFn: func() (store.APIJob, error) {
			return store.APIJob{ID: "job-123", Status: "running", OutputName: "Nightcrawler.mkv"}, nil
		},
		currentLogFn: func() (string, error) {
			return "[2026-03-29T12:00:00Z] remux started", nil
		},
	}, t.TempDir(), t.TempDir(), nil)

	statusReq := httptest.NewRequest(http.MethodGet, "/api/jobs/current", nil)
	statusW := httptest.NewRecorder()
	h.Current(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusW.Code)
	}
}
```

- [ ] **Step 2: Run the handler and router tests to verify red**

Run: `go test ./internal/http/handlers ./internal/http -run 'TestJobsHandler(CreateStartsRunningJob|CreateReturnsConflictWhenTaskAlreadyRunning|CurrentAndCurrentLog)|TestProtectedGetCurrentJobUsesCurrentHandler' -v`

Expected: FAIL because repository stubs, routes, and `Current` handlers do not exist yet.

- [ ] **Step 3: Implement handler API changes**

```go
type JobsRepository interface {
	CreateRunningJob(input CreateJobInput) (APIJob, error)
	GetCurrentJob() (APIJob, error)
	GetCurrentJobLog() (string, error)
	GetExecutionJob(id string) (ExecutionJob, error)
	MarkJobCompleted(id string) error
	MarkJobFailed(id, message string) error
	AppendJobLog(id, content string) error
}

func (h *JobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// existing payload validation remains
	job, err := h.Store.CreateRunningJob(input)
	if errors.Is(err, store.ErrJobAlreadyRunning) {
		http.Error(w, "remux already running", http.StatusConflict)
		return
	}
	// accepted response as before, then start async runner
	go func(jobID string) {
		if h.Runner == nil {
			return
		}
		_ = h.Runner.RunJob(context.Background(), jobID)
	}(job.ID)
}

func (h *JobsHandler) Current(w http.ResponseWriter, r *http.Request) {
	job, err := h.Store.GetCurrentJob()
	// map ErrJobNotFound to 404, otherwise encode JSON
}

func (h *JobsHandler) CurrentLog(w http.ResponseWriter, r *http.Request) {
	logText, err := h.Store.GetCurrentJobLog()
	// map ErrJobNotFound to 404, otherwise write plain text
}
```

- [ ] **Step 4: Wire the new routes and startup recovery**

```go
router := httpapi.NewRouter(httpapi.Dependencies{
	RequireAuth:    middleware.RequireAuth(sessionStore),
	Login:          authHandler.Login,
	Logout:         authHandler.Logout,
	ConfigGet:      configHandler.Get,
	SourcesScan:    sourcesHandler.Scan,
	SourcesList:    sourcesHandler.List,
	SourcesResolve: sourcesHandler.Resolve,
	BDInfoParse:    bdinfoHandler.Parse,
	DraftsPreview:  draftsHandler.PreviewFilename,
	JobsCreate:     jobsHandler.Create,
	JobsCurrent:    jobsHandler.Current,
	JobsCurrentLog: jobsHandler.CurrentLog,
})

jobStore := store.NewSQLiteJobStore(db, filepath.Join(cfg.DataDir, "logs"))
if err := jobStore.MarkRunningJobsFailed("process ended before completion"); err != nil {
	_ = db.Close()
	_ = logFile.Close()
	return nil, err
}
jobsHandler := handlers.NewJobsHandler(jobStore, cfg.InputDir, cfg.OutputDir, remux.NewJobRunner(jobStore, nil))
```

- [ ] **Step 5: Run the HTTP tests and confirm green**

Run: `go test ./internal/http ./internal/http/handlers -v`

Expected: PASS with `/api/jobs/current` and `/api/jobs/current/log` covered and queue routes removed from the app surface.

- [ ] **Step 6: Commit the HTTP/app wiring**

```bash
git add internal/http/handlers/jobs.go internal/http/handlers/jobs_test.go internal/http/router.go internal/http/router_test.go internal/app/app.go
git commit -m "feat: expose current remux task endpoints"
```

### Task 4: Move the Review Flow to a Live Current-Task Panel

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`
- Modify: `web/src/App.tsx`
- Modify: `web/src/components/Layout.tsx`
- Modify: `web/src/components/StatusBadge.tsx`
- Modify: `web/src/features/review/ReviewPage.tsx`
- Delete: `web/src/features/jobs/JobsPage.tsx`
- Modify: `web/src/styles/app.css`
- Create: `web/src/test/ReviewPage.test.tsx`
- Modify: `web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing review-page tests**

```tsx
it('renders the current remux panel when a task is present', () => {
  const source = { id: 'disc-1', name: 'Nightcrawler Disc', path: '/bd_input/Nightcrawler/BDMV', type: 'bdmv', size: 1, modifiedAt: '2026-03-29T12:00:00Z' } as const;
  const bdinfo = { playlistName: '00800.MPLS', rawText: 'PLAYLIST REPORT', audioLabels: [], subtitleLabels: [] } as const;
  const draft = {
    title: 'Nightcrawler',
    outputDir: '/remux',
    dvMergeEnabled: true,
    video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'HDR.DV' },
    audio: [],
    subtitles: [],
  } as const;
  render(
    <ReviewPage
      source={source}
      bdinfo={bdinfo}
      draft={draft}
      outputFilename="Nightcrawler - 2160p.mkv"
      outputPath="/remux/Nightcrawler - 2160p.mkv"
      submitting={false}
      currentJob={{
        id: 'job-123',
        sourceName: 'Nightcrawler Disc',
        outputName: 'Nightcrawler - 2160p.mkv',
        outputPath: '/remux/Nightcrawler - 2160p.mkv',
        playlistName: '00800.MPLS',
        createdAt: '2026-03-29T12:00:00Z',
        status: 'running',
      }}
      currentLog={'[2026-03-29T12:00:00Z] remux started'}
      onBack={() => {}}
      onSubmit={() => {}}
    />,
  );

  expect(screen.getByText(/current remux/i)).toBeInTheDocument();
  expect(screen.getByText(/running/i)).toBeInTheDocument();
  expect(screen.getByText(/remux started/i)).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the frontend tests and verify they fail**

Run: `cd web && npm test -- --run ReviewPage.test.tsx App.test.tsx`

Expected: FAIL because `currentJob`, `currentLog`, and jobs-step removals are not implemented.

- [ ] **Step 3: Update frontend types and API client**

```ts
export type JobStatus = 'running' | 'succeeded' | 'failed';

export type Job = {
  id: string;
  sourceName: string;
  outputName: string;
  outputPath?: string;
  playlistName: string;
  createdAt: string;
  status: JobStatus;
  message?: string;
};

async currentJob(token?: string): Promise<Job | null> {
  try {
    return normalizeJob(await requestJSON<Job>(`${basePath}/jobs/current`, { method: 'GET' }, token));
  } catch (error) {
    if (error instanceof Error && error.message.includes('404')) {
      return null;
    }
    throw error;
  }
}

async currentJobLog(token?: string): Promise<string> {
  const response = await fetch(`${basePath}/jobs/current/log`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (response.status === 404) {
    return '';
  }
  if (!response.ok) {
    throw new Error(`Request failed with status ${response.status}`);
  }
  return await response.text();
}
```

- [ ] **Step 4: Refactor app state to keep Review active and poll while running**

```tsx
const [currentJob, setCurrentJob] = useState<Job | null>(null);
const [currentJobLog, setCurrentJobLog] = useState('');

useEffect(() => {
  if (!token || currentJob?.status !== 'running') {
    return;
  }
  let cancelled = false;
  const interval = window.setInterval(async () => {
    const [job, log] = await Promise.all([
      api.currentJob(token ?? undefined),
      api.currentJobLog(token ?? undefined),
    ]);
    if (!cancelled) {
      setCurrentJob(job);
      setCurrentJobLog(log);
    }
  }, 1500);
  return () => {
    cancelled = true;
    window.clearInterval(interval);
  };
}, [token, currentJob?.status]);

const handleSubmitJob = async () => {
  const job = await api.submitJob(payload, token ?? undefined);
  setCurrentJob(job);
  setCurrentJobLog('');
  setJobsError(null);
  setStep('review');
};
```

- [ ] **Step 5: Render the current-task panel and remove jobs-page workflow**

```tsx
export type WorkflowStep = 'login' | 'scan' | 'bdinfo' | 'editor' | 'review';

const stepOrder: WorkflowStep[] = ['login', 'scan', 'bdinfo', 'editor', 'review'];

<ReviewPage
  source={selectedSource}
  bdinfo={parsedBDInfo}
  draft={draft}
  outputFilename={outputFilename || filenamePreview}
  outputPath={outputPath}
  submitting={submittingJob}
  currentJob={currentJob}
  currentLog={currentJobLog}
  onBack={() => setStep('editor')}
  onSubmit={handleSubmitJob}
/>
```

```tsx
{currentJob ? (
  <section className="info-box current-job-panel">
    <div className="row">
      <h3>Current Remux</h3>
      <StatusBadge status={currentJob.status} />
    </div>
    <p><strong>Output:</strong> {currentJob.outputName}</p>
    <p><strong>Path:</strong> {currentJob.outputPath}</p>
    {currentJob.message ? <p className="error-text">{currentJob.message}</p> : null}
    <pre className="job-log">{currentLog || 'Waiting for log output...'}</pre>
  </section>
) : null}
```

- [ ] **Step 6: Run the frontend review tests and confirm green**

Run: `cd web && npm test -- --run ReviewPage.test.tsx App.test.tsx`

Expected: PASS with no `JobsPage` import or jobs workflow step left in the UI.

- [ ] **Step 7: Commit the review-flow refactor**

```bash
git add web/src/api/types.ts web/src/api/client.ts web/src/App.tsx web/src/components/Layout.tsx web/src/components/StatusBadge.tsx web/src/features/review/ReviewPage.tsx web/src/styles/app.css web/src/test/ReviewPage.test.tsx web/src/test/App.test.tsx web/src/features/jobs/JobsPage.tsx
git commit -m "feat: show live remux progress on review page"
```

### Task 5: Refactor Track Editor into Compact Drag-Sortable Tables

**Files:**
- Create: `web/src/features/draft/trackTable.ts`
- Modify: `web/src/features/draft/TrackEditorPage.tsx`
- Modify: `web/src/styles/app.css`
- Create: `web/src/test/trackTable.test.ts`
- Modify: `web/src/test/TrackEditorPage.test.tsx`

- [ ] **Step 1: Write the failing helper tests for reorder/default logic**

```ts
import { describe, expect, it } from 'vitest';
import { moveTrackRow, setExclusiveDefault, toggleTrackSelected } from '../features/draft/trackTable';

describe('trackTable helpers', () => {
  it('moves a row to a new index', () => {
    const next = moveTrackRow(
      [
        { id: 'a1', name: 'English', language: 'eng', selected: true, default: true },
        { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
      ],
      'a2',
      'a1',
    );
    expect(next.map((track) => track.id)).toEqual(['a2', 'a1']);
  });

  it('keeps only one default track in the group', () => {
    const next = setExclusiveDefault(
      [
        { id: 'a1', name: 'English', language: 'eng', selected: true, default: true },
        { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
      ],
      'a2',
    );
    expect(next.find((track) => track.id === 'a1')?.default).toBe(false);
    expect(next.find((track) => track.id === 'a2')?.default).toBe(true);
  });
});
```

- [ ] **Step 2: Run the helper tests and verify red**

Run: `cd web && npm test -- --run trackTable.test.ts`

Expected: FAIL because the helper module does not exist yet.

- [ ] **Step 3: Implement focused helper logic**

```ts
import type { DraftTrack } from '../../api/types';

export function moveTrackRow(tracks: DraftTrack[], sourceId: string, targetId: string): DraftTrack[] {
  const fromIndex = tracks.findIndex((track) => track.id === sourceId);
  const toIndex = tracks.findIndex((track) => track.id === targetId);
  if (fromIndex < 0 || toIndex < 0 || fromIndex === toIndex) {
    return tracks;
  }
  const next = [...tracks];
  const [moved] = next.splice(fromIndex, 1);
  next.splice(toIndex, 0, moved);
  return next;
}

export function setExclusiveDefault(tracks: DraftTrack[], trackId: string): DraftTrack[] {
  return tracks.map((track) => ({
    ...track,
    default: track.id === trackId && track.selected,
  }));
}

export function toggleTrackSelected(tracks: DraftTrack[], trackId: string): DraftTrack[] {
  return tracks.map((track) =>
    track.id === trackId
      ? { ...track, selected: !track.selected, default: !track.selected ? track.default : false }
      : track,
  );
}
```

- [ ] **Step 4: Replace vertical forms with dense drag-enabled tables**

```tsx
<div className="track-editor-table-wrap">
  <table className="track-editor-table">
    <thead>
      <tr>
        <th scope="col" aria-label="Drag" />
        <th scope="col">Include</th>
        <th scope="col">Track</th>
        <th scope="col">Language</th>
        <th scope="col">Default</th>
        <th scope="col">Details</th>
      </tr>
    </thead>
    <tbody>
      {draft.audio.map((track) => (
        <tr
          key={track.id}
          className={track.selected ? 'is-selected' : 'is-muted'}
          draggable
          onDragStart={(event) => handleDragStart(event, 'audio', track.id)}
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => handleDrop(event, 'audio', track.id)}
        >
          <td className="drag-cell"><button type="button" className="drag-handle" aria-label={`Drag ${track.name}`}>⋮⋮</button></td>
          <td><input type="checkbox" checked={track.selected} onChange={() => updateAudio(toggleTrackSelected(draft.audio, track.id))} /></td>
          <td><input value={track.name} onChange={(event) => updateAudioName(track.id, event.target.value)} /></td>
          <td><input value={track.language} onChange={(event) => updateAudioLanguage(track.id, event.target.value)} /></td>
          <td><input type="checkbox" checked={track.default} disabled={!track.selected} onChange={() => updateAudio(setExclusiveDefault(draft.audio, track.id))} /></td>
          <td><span className="track-detail-chip">{track.codecLabel || track.name}</span></td>
        </tr>
      ))}
    </tbody>
  </table>
</div>
```

- [ ] **Step 5: Write/update component tests for checkbox default and drag reorder smoke coverage**

```tsx
it('clears the previous default audio track when a new default is checked', () => {
  const onChange = vi.fn();
  render(
    <TrackEditorPage
      draft={{
        video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p' },
        audio: [
          { id: 'a1', name: 'English Atmos', language: 'eng', selected: true, default: true },
          { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
        ],
        subtitles: [],
      }}
      onChange={onChange}
    />,
  );

  fireEvent.click(screen.getByRole('checkbox', { name: /default commentary/i }));

  expect(onChange).toHaveBeenCalledWith(
    expect.objectContaining({
      audio: expect.arrayContaining([
        expect.objectContaining({ id: 'a1', default: false }),
        expect.objectContaining({ id: 'a2', default: true }),
      ]),
    }),
  );
});
```

```tsx
it('reorders subtitle rows on drop', () => {
  const onChange = vi.fn();
  render(
    <TrackEditorPage
      draft={{
        video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p' },
        audio: [],
        subtitles: [
          { id: 's1', name: 'English PGS', language: 'eng', selected: true, default: false },
          { id: 's2', name: 'Commentary Subs', language: 'eng', selected: true, default: false },
        ],
      }}
      onChange={onChange}
    />,
  );

  const dragData = { value: '' };
  fireEvent.dragStart(screen.getByRole('row', { name: /commentary subs/i }), {
    dataTransfer: {
      setData: (_type: string, value: string) => { dragData.value = value; },
      getData: () => dragData.value,
      effectAllowed: 'move',
    },
  });
  fireEvent.drop(screen.getByRole('row', { name: /english pgs/i }), {
    dataTransfer: { getData: () => dragData.value },
  });

  expect(onChange).toHaveBeenCalled();
});
```

- [ ] **Step 6: Run the track-editor tests and confirm green**

Run: `cd web && npm test -- --run TrackEditorPage.test.tsx trackTable.test.ts`

Expected: PASS with no move-up/down buttons and with helper-based reorder/default behavior covered.

- [ ] **Step 7: Commit the track-editor refactor**

```bash
git add web/src/features/draft/trackTable.ts web/src/features/draft/TrackEditorPage.tsx web/src/styles/app.css web/src/test/trackTable.test.ts web/src/test/TrackEditorPage.test.tsx
git commit -m "feat: refactor track editor tables"
```

### Task 6: Final Verification and Cleanup

**Files:**
- Modify: `README.md` if queue wording appears in user-facing docs
- Review: all changed backend/frontend files

- [ ] **Step 1: Search for stale queue/jobs-history wording**

Run: `rg -n "queue|queued|Jobs Page|history" internal web README.md`

Expected: Only intentional internal/store comments or migration-safe references remain. Remove stale UI wording such as `Queue Remux Job`, `Queueing...`, or `Jobs`.

- [ ] **Step 2: Run the backend test suites touched by this work**

Run: `go test ./internal/store ./internal/remux ./internal/http ./internal/http/handlers -v`

Expected: PASS with current-task store, runner, and handler coverage green.

- [ ] **Step 3: Run the frontend test suites touched by this work**

Run: `cd web && npm test -- --run App.test.tsx ReviewPage.test.tsx TrackEditorPage.test.tsx trackTable.test.ts`

Expected: PASS with live review panel and table editor coverage green.

- [ ] **Step 4: Build the frontend once after the refactor**

Run: `cd web && npm run build`

Expected: PASS and emit `dist` assets without type errors.

- [ ] **Step 5: Commit final cleanup if needed**

```bash
git add README.md internal web
git commit -m "chore: remove queue UI remnants"
```
