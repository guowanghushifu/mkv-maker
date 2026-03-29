# Memory Auth + Single Remux + Track Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove SQLite entirely, replace auth and remux persistence with in-memory services, keep live remux progress on the review page, and refactor the track editor into compact drag-sortable tables.

**Architecture:** The backend becomes a single-process in-memory application. Auth uses signed cookies validated without server-side session storage. Remux execution is coordinated by one in-memory manager that holds the running or most recent task plus log output. The frontend keeps the review page active after submit and polls current-task endpoints while the track editor moves to dense table-based editing with drag sorting and checkbox-style default selection.

**Tech Stack:** Go, Chi, React 18, TypeScript, Vitest, Testing Library, native HTML drag-and-drop

---

## File Map

- Create: `internal/auth/cookie.go`
  Responsibility: signed cookie issue/verify helpers and auth service
- Create: `internal/auth/cookie_test.go`
  Responsibility: signed cookie round-trip, invalid signature, expiry coverage
- Create: `internal/remux/manager.go`
  Responsibility: in-memory current/remux task state, start conflict handling, live log buffering
- Create: `internal/remux/manager_test.go`
  Responsibility: manager concurrency, current-task lookup, log retrieval, terminal state coverage
- Keep and modify: `internal/remux/command_builder.go`
  Responsibility: mkvmerge CLI argument building only
- Keep and modify or replace: `internal/remux/job_runner.go`
  Responsibility: execute one in-memory task through the manager, append logs, mark success/failure
- Modify: `internal/http/handlers/auth.go`
  Responsibility: use signed-cookie auth instead of DB-backed sessions
- Modify: `internal/http/middleware/auth.go`
  Responsibility: validate signed cookies via auth service interface
- Modify: `internal/http/handlers/jobs.go`
  Responsibility: validate payload, start in-memory remux, expose `/current` and `/current/log`
- Modify: `internal/http/handlers/jobs_test.go`
  Responsibility: conflict handling, no-current-task 404s, current-task status/log behavior
- Modify: `internal/http/router.go`
  Responsibility: keep `POST /api/jobs`, add `GET /api/jobs/current`, `GET /api/jobs/current/log`, remove list/get-by-id/log-by-id handlers
- Modify: `internal/http/router_test.go`
  Responsibility: route coverage for current-task endpoints
- Modify: `internal/app/app.go`
  Responsibility: remove DB initialization, wire auth signer and in-memory remux manager
- Delete: `internal/store/db.go`
- Delete: `internal/store/migrate.go`
- Delete: `internal/store/session_store.go`
- Delete: `internal/store/session_store_test.go`
- Delete: `internal/store/job_store.go`
- Delete: `internal/store/job_store_test.go`
- Delete: `internal/queue/manager.go`
- Delete: `internal/queue/manager_test.go`
- Delete: `internal/queue/executor.go`
- Delete: `internal/queue/executor_test.go`
- Modify: `go.mod`
  Responsibility: remove `modernc.org/sqlite`
- Modify: `go.sum`
  Responsibility: drop sqlite dependency hashes
- Modify: `web/src/api/types.ts`
  Responsibility: current-task-only statuses and shapes
- Modify: `web/src/api/client.ts`
  Responsibility: fetch current-task/current-log, surface 404 as “no current task”
- Modify: `web/src/App.tsx`
  Responsibility: remove jobs page flow, poll current task while running
- Modify: `web/src/components/Layout.tsx`
  Responsibility: remove jobs step
- Modify: `web/src/components/StatusBadge.tsx`
  Responsibility: only `running`, `succeeded`, `failed`
- Delete: `web/src/features/jobs/JobsPage.tsx`
- Modify: `web/src/features/review/ReviewPage.tsx`
  Responsibility: current-task status/log panel and start wording
- Create: `web/src/features/draft/trackTable.ts`
  Responsibility: pure reorder/default helper logic
- Modify: `web/src/features/draft/TrackEditorPage.tsx`
  Responsibility: dense table UI with drag handles and checkbox default behavior
- Modify: `web/src/styles/app.css`
  Responsibility: review current-task panel styling and compact track table styling
- Create: `web/src/test/ReviewPage.test.tsx`
  Responsibility: current-task panel rendering
- Create: `web/src/test/trackTable.test.ts`
  Responsibility: pure helper tests
- Modify: `web/src/test/TrackEditorPage.test.tsx`
  Responsibility: drag/default/deselect behavior
- Modify: `web/src/test/App.test.tsx`
  Responsibility: keep shell smoke test aligned with updated workflow

### Task 1: Replace SQLite Session/Auth with Signed Cookies

**Files:**
- Create: `internal/auth/cookie.go`
- Create: `internal/auth/cookie_test.go`
- Modify: `internal/http/handlers/auth.go`
- Modify: `internal/http/middleware/auth.go`
- Delete: `internal/store/session_store.go`
- Delete: `internal/store/session_store_test.go`

- [ ] **Step 1: Write the failing auth tests**

```go
func TestCookieAuthIssueAndValidate(t *testing.T) {
	auth := NewCookieAuth("app-password", time.Hour)

	token, err := auth.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	valid, err := auth.Valid(token)
	if err != nil {
		t.Fatalf("Valid returned error: %v", err)
	}
	if !valid {
		t.Fatal("expected issued token to validate")
	}
}

func TestCookieAuthRejectsTamperedToken(t *testing.T) {
	auth := NewCookieAuth("app-password", time.Hour)

	token, err := auth.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	valid, err := auth.Valid(token+"x")
	if err != nil {
		t.Fatalf("Valid returned error: %v", err)
	}
	if valid {
		t.Fatal("expected tampered token to be rejected")
	}
}

func TestCookieAuthRejectsExpiredToken(t *testing.T) {
	auth := NewCookieAuth("app-password", 0)

	token, err := auth.Issue()
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	valid, err := auth.Valid(token)
	if err != nil {
		t.Fatalf("Valid returned error: %v", err)
	}
	if valid {
		t.Fatal("expected token to be expired")
	}
}
```

```go
func TestAuthHandlerLoginSetsSignedCookie(t *testing.T) {
	auth := auth.NewCookieAuth("secret", time.Hour)
	handler := &AuthHandler{
		AppPassword:   "secret",
		Auth:          auth,
		SessionMaxAge: 3600,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"password":"secret"}`))
	w := httptest.NewRecorder()
	handler.Login(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Value == "" {
		t.Fatal("expected signed auth cookie")
	}
}
```

- [ ] **Step 2: Run the auth tests and verify red**

Run: `go test ./internal/auth ./internal/http/handlers ./internal/http/middleware -run 'TestCookieAuth|TestAuthHandlerLoginSetsSignedCookie' -v`

Expected: FAIL because the cookie auth service and handler wiring do not exist yet.

- [ ] **Step 3: Implement the signed-cookie auth service**

```go
type CookieAuth struct {
	secret    []byte
	maxAge    time.Duration
	clockNow  func() time.Time
}

func NewCookieAuth(password string, maxAge time.Duration) *CookieAuth {
	sum := sha256.Sum256([]byte("mkv-maker:" + password))
	return &CookieAuth{
		secret:   sum[:],
		maxAge:   maxAge,
		clockNow: time.Now,
	}
}

func (a *CookieAuth) Issue() (string, error) {
	expiresAt := a.clockNow().UTC().Add(a.maxAge).Unix()
	payload := strconv.FormatInt(expiresAt, 10)
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "." + sig)), nil
}

func (a *CookieAuth) Valid(token string) (bool, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return false, nil
	}
	parts := strings.Split(string(decoded), ".")
	if len(parts) != 2 {
		return false, nil
	}
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := hex.DecodeString(parts[1])
	if err != nil || !hmac.Equal(expected, actual) {
		return false, nil
	}
	expiresAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false, nil
	}
	return a.clockNow().UTC().Unix() < expiresAt, nil
}
```

- [ ] **Step 4: Update auth handler and middleware to use the auth service**

```go
type TokenAuth interface {
	Issue() (string, error)
	Valid(token string) (bool, error)
}

type AuthHandler struct {
	AppPassword   string
	Auth          TokenAuth
	SessionMaxAge int
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// existing password validation remains
	token, err := h.Auth.Issue()
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   h.SessionMaxAge,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func RequireAuth(auth TokenAuth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			valid, err := auth.Valid(cookie.Value)
			if err != nil || !valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 5: Run the auth package and middleware tests**

Run: `go test ./internal/auth ./internal/http/handlers ./internal/http/middleware -v`

Expected: PASS with no `internal/store/session_store.go` dependency.

- [ ] **Step 6: Commit the auth refactor**

```bash
git add internal/auth/cookie.go internal/auth/cookie_test.go internal/http/handlers/auth.go internal/http/middleware/auth.go internal/store/session_store.go internal/store/session_store_test.go
git commit -m "refactor: replace sqlite sessions with signed cookies"
```

### Task 2: Replace Persistent Jobs/Queue with In-Memory Remux Manager

**Files:**
- Create: `internal/remux/manager.go`
- Create: `internal/remux/manager_test.go`
- Keep and modify: `internal/remux/job_runner.go`
- Delete: `internal/store/job_store.go`
- Delete: `internal/store/job_store_test.go`
- Delete: `internal/queue/manager.go`
- Delete: `internal/queue/manager_test.go`
- Delete: `internal/queue/executor.go`
- Delete: `internal/queue/executor_test.go`

- [ ] **Step 1: Write the failing manager tests**

```go
func TestManagerStartRejectsWhenJobAlreadyRunning(t *testing.T) {
	manager := NewManager(&stubRunner{})
	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  `{"source":{"name":"Nightcrawler Disc"}}`,
	})
	if err != nil {
		t.Fatalf("first Start returned error: %v", err)
	}

	_, err = manager.Start(StartRequest{
		SourceName:   "Second Disc",
		OutputName:   "Second.mkv",
		OutputPath:   "/remux/Second.mkv",
		PlaylistName: "00002.MPLS",
		PayloadJSON:  `{"source":{"name":"Second Disc"}}`,
	})
	if !errors.Is(err, ErrTaskAlreadyRunning) {
		t.Fatalf("expected ErrTaskAlreadyRunning, got %v", err)
	}
}

func TestManagerCurrentReturnsRunningAndLatestLog(t *testing.T) {
	manager := NewManager(&stubRunner{output: "mkvmerge progress"})

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON: `{
			"source":{"name":"Nightcrawler Disc","path":"/bd_input/Nightcrawler","type":"bdmv"},
			"bdinfo":{"playlistName":"00800.MPLS"},
			"draft":{"playlistName":"00800.MPLS","video":{"name":"Main Video","codec":"HEVC","resolution":"2160p"},"audio":[],"subtitles":[]},
			"outputPath":"/remux/Nightcrawler.mkv"
		}`,
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if task.Status != "running" {
		t.Fatalf("expected running status, got %q", task.Status)
	}

	current, err := manager.Current()
	if err != nil {
		t.Fatalf("Current returned error: %v", err)
	}
	if current.ID != task.ID {
		t.Fatalf("expected current id %q, got %q", task.ID, current.ID)
	}
}
```

- [ ] **Step 2: Run the manager tests and verify red**

Run: `go test ./internal/remux -run 'TestManager(StartRejectsWhenJobAlreadyRunning|CurrentReturnsRunningAndLatestLog)' -v`

Expected: FAIL because the in-memory manager does not exist yet.

- [ ] **Step 3: Implement the in-memory manager and direct runner**

```go
type Task struct {
	ID           string `json:"id"`
	SourceName   string `json:"sourceName"`
	OutputName   string `json:"outputName"`
	OutputPath   string `json:"outputPath"`
	PlaylistName string `json:"playlistName"`
	CreatedAt    string `json:"createdAt"`
	Status       string `json:"status"`
	Message      string `json:"message,omitempty"`
}

type StartRequest struct {
	SourceName   string
	OutputName   string
	OutputPath   string
	PlaylistName string
	PayloadJSON  string
}

type Manager struct {
	mu      sync.RWMutex
	runner  CommandRunner
	current *taskState
	latest  *taskState
}

func (m *Manager) Start(req StartRequest) (Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current != nil && m.current.task.Status == "running" {
		return Task{}, ErrTaskAlreadyRunning
	}
	state := newTaskState(req)
	m.current = state
	m.latest = state
	go m.run(state)
	return state.task, nil
}

func (m *Manager) Current() (Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.current != nil {
		return m.current.task, nil
	}
	if m.latest != nil {
		return m.latest.task, nil
	}
	return Task{}, ErrTaskNotFound
}

func (m *Manager) CurrentLog() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.current != nil {
		return m.current.log.String(), nil
	}
	if m.latest != nil {
		return m.latest.log.String(), nil
	}
	return "", ErrTaskNotFound
}
```

- [ ] **Step 4: Delete persistent store and queue files**

```bash
rm internal/store/job_store.go
rm internal/store/job_store_test.go
rm internal/queue/manager.go
rm internal/queue/manager_test.go
rm internal/queue/executor.go
rm internal/queue/executor_test.go
```

- [ ] **Step 5: Run remux tests after the in-memory conversion**

Run: `go test ./internal/remux -v`

Expected: PASS with current-task semantics backed only by memory.

- [ ] **Step 6: Commit the in-memory remux manager**

```bash
git add internal/remux/manager.go internal/remux/manager_test.go internal/remux/job_runner.go internal/store/job_store.go internal/store/job_store_test.go internal/queue/manager.go internal/queue/manager_test.go internal/queue/executor.go internal/queue/executor_test.go
git commit -m "refactor: replace queued jobs with in-memory remux manager"
```

### Task 3: Rewire Handlers and App Startup for In-Memory Services

**Files:**
- Modify: `internal/http/handlers/jobs.go`
- Modify: `internal/http/handlers/jobs_test.go`
- Modify: `internal/http/router.go`
- Modify: `internal/http/router_test.go`
- Modify: `internal/app/app.go`
- Delete: `internal/store/db.go`
- Delete: `internal/store/migrate.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Write the failing handler tests**

```go
func TestJobsHandlerCurrentReturnsNotFoundWithoutTask(t *testing.T) {
	manager := remux.NewManager(&stubRunner{})
	h := NewJobsHandler(manager, "/bd_input", "/remux")

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/current", nil)
	w := httptest.NewRecorder()
	h.Current(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestJobsHandlerCreateReturnsConflictWhenTaskRunning(t *testing.T) {
	inputRoot := t.TempDir()
	sourcePath := filepath.Join(inputRoot, "Nightcrawler", "BDMV")
	if err := os.MkdirAll(filepath.Join(sourcePath, "PLAYLIST"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourcePath, "PLAYLIST", "00800.MPLS"), []byte("playlist"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	outputRoot := t.TempDir()
	manager := remux.NewManager(&stubRunner{})
	_, _ = manager.Start(remux.StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00800.MPLS",
		PayloadJSON:  `{"source":{"name":"Nightcrawler Disc"}}`,
	})

	h := NewJobsHandler(manager, inputRoot, outputRoot)
	reqBody := `{
		"source":{"id":"Nightcrawler","name":"Nightcrawler Disc","path":"` + sourcePath + `","type":"bdmv"},
		"bdinfo":{"playlistName":"00800.MPLS","rawText":"PLAYLIST REPORT:\nName: 00800.MPLS"},
		"draft":{"sourceId":"Nightcrawler","playlistName":"00800.MPLS"},
		"outputFilename":"Nightcrawler.mkv",
		"outputPath":"` + filepath.Join(outputRoot, "Nightcrawler.mkv") + `"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Create(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run the handler and router tests and verify red**

Run: `go test ./internal/http ./internal/http/handlers -run 'TestJobsHandler(CurrentReturnsNotFoundWithoutTask|CreateReturnsConflictWhenTaskRunning)|TestProtectedGetCurrentJobUsesCurrentHandler' -v`

Expected: FAIL because the handler constructor and route dependencies still expect store-backed jobs.

- [ ] **Step 3: Update handler and router dependencies**

```go
type CurrentTaskService interface {
	Start(req remux.StartRequest) (remux.Task, error)
	Current() (remux.Task, error)
	CurrentLog() (string, error)
}

type JobsHandler struct {
	Tasks     CurrentTaskService
	InputDir  string
	OutputDir string
}

func (h *JobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// existing payload validation remains
	task, err := h.Tasks.Start(remux.StartRequest{
		SourceName:   strings.TrimSpace(req.Source.Name),
		OutputName:   strings.TrimSpace(req.OutputFilename),
		OutputPath:   strings.TrimSpace(req.OutputPath),
		PlaylistName: playlistName,
		PayloadJSON:  payloadJSON,
	})
	if errors.Is(err, remux.ErrTaskAlreadyRunning) {
		http.Error(w, "remux already running", http.StatusConflict)
		return
	}
	json.NewEncoder(w).Encode(task)
}
```

```go
type Dependencies struct {
	RequireAuth    func(http.Handler) http.Handler
	Login          http.HandlerFunc
	Logout         http.HandlerFunc
	ConfigGet      http.HandlerFunc
	SourcesScan    http.HandlerFunc
	SourcesList    http.HandlerFunc
	SourcesResolve http.HandlerFunc
	BDInfoParse    http.HandlerFunc
	DraftsPreview  http.HandlerFunc
	JobsCreate     http.HandlerFunc
	JobsCurrent    http.HandlerFunc
	JobsCurrentLog http.HandlerFunc
}
```

- [ ] **Step 4: Remove database startup and sqlite module dependency**

```go
type App struct {
	Config   config.Config
	Handler  http.Handler
	logFile  *os.File
	cancelFn context.CancelFunc
}

func New(cfg config.Config) (*App, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	logFile, err := initAppLogger(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	cookieAuth := auth.NewCookieAuth(cfg.AppPassword, time.Duration(cfg.SessionMaxAge)*time.Second)
	authHandler := &handlers.AuthHandler{
		AppPassword:   cfg.AppPassword,
		Auth:          cookieAuth,
		SessionMaxAge: cfg.SessionMaxAge,
	}
	remuxManager := remux.NewManager(nil)
	jobsHandler := handlers.NewJobsHandler(remuxManager, cfg.InputDir, cfg.OutputDir)

	router := httpapi.NewRouter(httpapi.Dependencies{
		RequireAuth:    middleware.RequireAuth(cookieAuth),
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
	return &App{Config: cfg, Handler: withFrontend(router, filepath.Join("web", "dist")), logFile: logFile}, nil
}
```

- [ ] **Step 5: Run HTTP/app tests and tidy modules**

Run: `go test ./internal/http ./internal/http/handlers ./internal/app -v`

Run: `go mod tidy`

Expected: PASS and removal of `modernc.org/sqlite` from `go.mod`.

- [ ] **Step 6: Commit the handler/app rewiring**

```bash
git add internal/http/handlers/jobs.go internal/http/handlers/jobs_test.go internal/http/router.go internal/http/router_test.go internal/app/app.go internal/store/db.go internal/store/migrate.go go.mod go.sum
git commit -m "refactor: wire app with in-memory auth and remux services"
```

### Task 4: Move Review Flow to Current-Task Polling

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
      currentLog="[2026-03-29T12:00:00Z] remux started"
      onBack={() => {}}
      onSubmit={() => {}}
    />,
  )

  expect(screen.getByText(/current remux/i)).toBeInTheDocument()
  expect(screen.getByText(/running/i)).toBeInTheDocument()
  expect(screen.getByText(/remux started/i)).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the frontend tests and verify red**

Run: `cd web && npm test -- --run ReviewPage.test.tsx App.test.tsx`

Expected: FAIL because the review page and app state still assume a jobs page and older job API shapes.

- [ ] **Step 3: Update the client and app polling flow**

```ts
export type JobStatus = 'running' | 'succeeded' | 'failed';

async currentJob(token?: string): Promise<Job | null> {
  const response = await fetch(`${basePath}/jobs/current`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (response.status === 404) {
    return null;
  }
  if (!response.ok) {
    throw new Error(`Request failed with status ${response.status}`);
  }
  return normalizeJob((await response.json()) as Job);
}
```

```tsx
const [currentJob, setCurrentJob] = useState<Job | null>(null)
const [currentJobLog, setCurrentJobLog] = useState('')

useEffect(() => {
  if (!token || currentJob?.status !== 'running') {
    return
  }
  let cancelled = false
  const timer = window.setInterval(async () => {
    const [job, log] = await Promise.all([
      api.currentJob(token),
      api.currentJobLog(token),
    ])
    if (!cancelled) {
      setCurrentJob(job)
      setCurrentJobLog(log)
    }
  }, 1500)
  return () => {
    cancelled = true
    window.clearInterval(timer)
  }
}, [token, currentJob?.status])
```

- [ ] **Step 4: Remove jobs-page workflow and render the current-task panel**

```tsx
export type WorkflowStep = 'login' | 'scan' | 'bdinfo' | 'editor' | 'review'

const stepOrder: WorkflowStep[] = ['login', 'scan', 'bdinfo', 'editor', 'review']
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

- [ ] **Step 5: Run the review-flow tests**

Run: `cd web && npm test -- --run ReviewPage.test.tsx App.test.tsx`

Expected: PASS with no `JobsPage` import and no jobs workflow step remaining.

- [ ] **Step 6: Commit the review-flow update**

```bash
git add web/src/api/types.ts web/src/api/client.ts web/src/App.tsx web/src/components/Layout.tsx web/src/components/StatusBadge.tsx web/src/features/review/ReviewPage.tsx web/src/features/jobs/JobsPage.tsx web/src/styles/app.css web/src/test/ReviewPage.test.tsx web/src/test/App.test.tsx
git commit -m "feat: show current remux progress on review page"
```

### Task 5: Refactor Track Editor into Compact Drag-Sortable Tables

**Files:**
- Create: `web/src/features/draft/trackTable.ts`
- Modify: `web/src/features/draft/TrackEditorPage.tsx`
- Modify: `web/src/styles/app.css`
- Create: `web/src/test/trackTable.test.ts`
- Modify: `web/src/test/TrackEditorPage.test.tsx`

- [ ] **Step 1: Write the failing helper tests**

```ts
import { describe, expect, it } from 'vitest'
import { moveTrackRow, setExclusiveDefault, toggleTrackSelected } from '../features/draft/trackTable'

describe('trackTable helpers', () => {
  it('moves a row to a new index', () => {
    const next = moveTrackRow(
      [
        { id: 'a1', name: 'English', language: 'eng', selected: true, default: true },
        { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
      ],
      'a2',
      'a1',
    )
    expect(next.map((track) => track.id)).toEqual(['a2', 'a1'])
  })

  it('keeps only one default track in the group', () => {
    const next = setExclusiveDefault(
      [
        { id: 'a1', name: 'English', language: 'eng', selected: true, default: true },
        { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
      ],
      'a2',
    )
    expect(next.find((track) => track.id === 'a1')?.default).toBe(false)
    expect(next.find((track) => track.id === 'a2')?.default).toBe(true)
  })
})
```

- [ ] **Step 2: Run the helper tests and verify red**

Run: `cd web && npm test -- --run trackTable.test.ts`

Expected: FAIL because the helper module does not exist yet.

- [ ] **Step 3: Implement helper logic and compact table UI**

```ts
export function moveTrackRow(tracks: DraftTrack[], sourceId: string, targetId: string): DraftTrack[] {
  const fromIndex = tracks.findIndex((track) => track.id === sourceId)
  const toIndex = tracks.findIndex((track) => track.id === targetId)
  if (fromIndex < 0 || toIndex < 0 || fromIndex === toIndex) {
    return tracks
  }
  const next = [...tracks]
  const [moved] = next.splice(fromIndex, 1)
  next.splice(toIndex, 0, moved)
  return next
}

export function setExclusiveDefault(tracks: DraftTrack[], trackId: string): DraftTrack[] {
  return tracks.map((track) => ({
    ...track,
    default: track.id === trackId && track.selected,
  }))
}

export function toggleTrackSelected(tracks: DraftTrack[], trackId: string): DraftTrack[] {
  return tracks.map((track) =>
    track.id === trackId
      ? { ...track, selected: !track.selected, default: !track.selected ? track.default : false }
      : track,
  )
}
```

```tsx
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
        <td className="drag-cell">
          <button type="button" className="drag-handle" aria-label={`Drag ${track.name}`}>⋮⋮</button>
        </td>
        <td><input type="checkbox" checked={track.selected} onChange={() => updateAudio(toggleTrackSelected(draft.audio, track.id))} /></td>
        <td><input value={track.name} onChange={(event) => updateAudioName(track.id, event.target.value)} /></td>
        <td><input value={track.language} onChange={(event) => updateAudioLanguage(track.id, event.target.value)} /></td>
        <td><input type="checkbox" checked={track.default} disabled={!track.selected} onChange={() => updateAudio(setExclusiveDefault(draft.audio, track.id))} /></td>
        <td><span className="track-detail-chip">{track.codecLabel || track.name}</span></td>
      </tr>
    ))}
  </tbody>
</table>
```

- [ ] **Step 4: Update track editor tests**

```tsx
it('clears the previous default audio track when a new default is checked', () => {
  const onChange = vi.fn()
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
  )

  fireEvent.click(screen.getByRole('checkbox', { name: /default commentary/i }))

  expect(onChange).toHaveBeenCalled()
})
```

- [ ] **Step 5: Run the track-editor tests**

Run: `cd web && npm test -- --run TrackEditorPage.test.tsx trackTable.test.ts`

Expected: PASS with drag/default/deselect behavior covered.

- [ ] **Step 6: Commit the track-editor refactor**

```bash
git add web/src/features/draft/trackTable.ts web/src/features/draft/TrackEditorPage.tsx web/src/styles/app.css web/src/test/trackTable.test.ts web/src/test/TrackEditorPage.test.tsx
git commit -m "feat: refactor track editor tables"
```

### Task 6: Final Verification and Cleanup

**Files:**
- Modify: `README.md` if SQLite or queue wording remains
- Review: all touched files

- [ ] **Step 1: Search for stale SQLite and queue wording**

Run: `rg -n "sqlite|queue|queued|app.db|session store|job store" internal web README.md go.mod`

Expected: Only intentional historical doc references remain; remove stale runtime wording from product-facing text.

- [ ] **Step 2: Run backend verification**

Run: `go test ./...`

Expected: PASS with no sqlite dependency and no queue package remaining.

- [ ] **Step 3: Run frontend verification**

Run: `cd web && npm test -- --run`

Expected: PASS with review current-task panel and track editor table tests green.

- [ ] **Step 4: Build the frontend**

Run: `cd web && npm run build`

Expected: PASS and emit `dist` assets without type errors.

- [ ] **Step 5: Commit cleanup if needed**

```bash
git add README.md go.mod go.sum internal web
git commit -m "chore: remove sqlite and queue remnants"
```
