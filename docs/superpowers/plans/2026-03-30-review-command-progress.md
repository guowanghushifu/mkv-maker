# Review Command And Progress Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show the final executed `mkvmerge` command plus a percentage progress bar on the review page for the current remux task.

**Architecture:** Extend the in-memory remux task model with `commandPreview` and `progressPercent`, both computed and maintained on the backend. The backend formats the actual command when the task starts and parses explicit percentages from `mkvmerge` output, while the frontend simply renders the structured fields inside the existing current-task panel.

**Tech Stack:** Go, React 18, TypeScript, Vitest, Testing Library

---

## File Map

- Create: `internal/remux/progress.go`
  Responsibility: format human-readable command preview and parse explicit percentages from `mkvmerge` output
- Create: `internal/remux/progress_test.go`
  Responsibility: command formatting and progress parsing tests
- Modify: `internal/remux/manager.go`
  Responsibility: store `commandPreview` and `progressPercent` on current/latest task state and update progress during execution
- Modify: `internal/remux/manager_test.go`
  Responsibility: manager lifecycle tests covering command preview, running progress, success `100%`, and failed-task percentage retention
- Modify: `internal/remux/job_runner.go`
  Responsibility: expose draft/args details needed for command preview and keep execution output available for progress parsing
- Modify: `web/src/api/types.ts`
  Responsibility: include `commandPreview` and `progressPercent` on `Job`
- Modify: `web/src/App.tsx`
  Responsibility: keep current-task snapshot hydration aligned with new fields
- Modify: `web/src/features/review/ReviewPage.tsx`
  Responsibility: render progress bar, numeric percentage, and command block inside `Current Remux`
- Modify: `web/src/styles/app.css`
  Responsibility: style progress bar and command preview block
- Modify: `web/src/test/ReviewPage.test.tsx`
  Responsibility: assert progress percentage/bar and formatted command rendering
- Modify: `web/src/test/App.test.tsx`
  Responsibility: assert terminal-task hydration includes command/progress data

### Task 1: Add Backend Command Preview And Progress Parsing

**Files:**
- Create: `internal/remux/progress.go`
- Create: `internal/remux/progress_test.go`
- Modify: `internal/remux/manager.go`
- Modify: `internal/remux/manager_test.go`
- Modify: `internal/remux/job_runner.go`

- [ ] **Step 1: Write the failing backend tests**

```go
func TestFormatCommandPreviewRendersReadableMultiLineCommand(t *testing.T) {
	got := FormatCommandPreview("mkvmerge", []string{
		"--output", "/remux/Nightcrawler.mkv",
		"--audio-tracks", "2,3",
		"/bd_input/Nightcrawler/BDMV/PLAYLIST/00003.MPLS",
	})

	if !strings.HasPrefix(got, "mkvmerge\n") {
		t.Fatalf("expected preview to start with mkvmerge, got %q", got)
	}
	if !strings.Contains(got, "\n  --output\n  /remux/Nightcrawler.mkv\n") {
		t.Fatalf("expected output arg in multiline preview, got %q", got)
	}
	if !strings.Contains(got, "/bd_input/Nightcrawler/BDMV/PLAYLIST/00003.MPLS") {
		t.Fatalf("expected input path in preview, got %q", got)
	}
}

func TestExtractProgressPercentParsesExplicitMkvmmergePercentages(t *testing.T) {
	tests := []struct {
		line string
		want int
		ok   bool
	}{
		{line: "Progress: 42%", want: 42, ok: true},
		{line: "#GUI#progress 77%", want: 77, ok: true},
		{line: "muxing took 3 seconds", want: 0, ok: false},
	}

	for _, tc := range tests {
		got, ok := ExtractProgressPercent(tc.line)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("ExtractProgressPercent(%q) = (%d, %t), want (%d, %t)", tc.line, got, ok, tc.want, tc.ok)
		}
	}
}

func TestManagerSuccessTransitionSetsCommandPreviewAndHundredPercent(t *testing.T) {
	manager := NewManager(&stubRunner{output: "Progress: 42%\nProgress: 100%"})
	defer manager.Close()

	task, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00003.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00003.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.ID != task.ID {
		t.Fatalf("expected same task id, got %q", done.ID)
	}
	if done.ProgressPercent != 100 {
		t.Fatalf("expected 100 percent, got %d", done.ProgressPercent)
	}
	if !strings.HasPrefix(done.CommandPreview, "mkvmerge\n") {
		t.Fatalf("expected command preview, got %q", done.CommandPreview)
	}
}

func TestManagerFailureKeepsLastKnownProgressPercent(t *testing.T) {
	manager := NewManager(&stubRunner{
		output: "Progress: 63%\nstderr output",
		err:    errors.New("runner exploded"),
	})
	defer manager.Close()

	_, err := manager.Start(StartRequest{
		SourceName:   "Nightcrawler Disc",
		OutputName:   "Nightcrawler.mkv",
		OutputPath:   "/remux/Nightcrawler.mkv",
		PlaylistName: "00003.MPLS",
		PayloadJSON:  validPayloadJSON("Nightcrawler Disc", "/bd_input/Nightcrawler", "00003.MPLS", "/remux/Nightcrawler.mkv"),
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	done := waitForTerminalTask(t, manager)
	if done.Status != "failed" {
		t.Fatalf("expected failed status, got %q", done.Status)
	}
	if done.ProgressPercent != 63 {
		t.Fatalf("expected last known progress 63, got %d", done.ProgressPercent)
	}
}
```

- [ ] **Step 2: Run the backend tests and verify red**

Run: `go test ./internal/remux -run 'TestFormatCommandPreviewRendersReadableMultiLineCommand|TestExtractProgressPercentParsesExplicitMkvmmergePercentages|TestManagerSuccessTransitionSetsCommandPreviewAndHundredPercent|TestManagerFailureKeepsLastKnownProgressPercent' -v`

Expected: FAIL because `FormatCommandPreview`, `ExtractProgressPercent`, `Task.CommandPreview`, and `Task.ProgressPercent` do not exist yet.

- [ ] **Step 3: Implement minimal backend support**

```go
type Task struct {
	ID              string `json:"id"`
	SourceName      string `json:"sourceName"`
	OutputName      string `json:"outputName"`
	OutputPath      string `json:"outputPath"`
	PlaylistName    string `json:"playlistName"`
	CreatedAt       string `json:"createdAt"`
	Status          string `json:"status"`
	Message         string `json:"message,omitempty"`
	CommandPreview  string `json:"commandPreview,omitempty"`
	ProgressPercent int    `json:"progressPercent"`
}
```

```go
func FormatCommandPreview(binary string, args []string) string {
	binary = strings.TrimSpace(binary)
	if binary == "" {
		binary = "mkvmerge"
	}
	lines := []string{binary}
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		lines = append(lines, "  "+arg)
	}
	return strings.Join(lines, "\n")
}

func ExtractProgressPercent(line string) (int, bool) {
	match := progressPattern.FindStringSubmatch(line)
	if len(match) != 2 {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return value, true
}
```

```go
func (r *JobRunner) BuildExecutionDraft(req StartRequest) (Draft, error) {
	return buildExecutionDraft(req)
}

func (r *JobRunner) CommandPreview(req StartRequest) (string, error) {
	draft, err := buildExecutionDraft(req)
	if err != nil {
		return "", err
	}
	return FormatCommandPreview("mkvmerge", BuildMKVMergeArgs(draft)), nil
}
```

```go
state := &taskState{
	task: Task{
		ID:              id,
		SourceName:      strings.TrimSpace(req.SourceName),
		OutputName:      strings.TrimSpace(req.OutputName),
		OutputPath:      strings.TrimSpace(req.OutputPath),
		PlaylistName:    strings.TrimSpace(req.PlaylistName),
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Status:          "running",
		CommandPreview:  preview,
		ProgressPercent: 0,
	},
	log: logLine("remux started"),
}
```

```go
for _, line := range strings.Split(output, "\n") {
	if percent, ok := ExtractProgressPercent(line); ok {
		m.updateProgress(state, percent)
	}
}
if err == nil {
	m.updateProgress(state, 100)
}
```

- [ ] **Step 4: Run the remux package tests and confirm green**

Run: `go test ./internal/remux -v`

Expected: PASS with command preview and progress percent covered in manager tests.

- [ ] **Step 5: Commit the backend progress work**

```bash
git add internal/remux/progress.go internal/remux/progress_test.go internal/remux/manager.go internal/remux/manager_test.go internal/remux/job_runner.go
git commit -m "feat: expose remux command preview and progress"
```

### Task 2: Render Command Preview And Progress Bar On Review Page

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/App.tsx`
- Modify: `web/src/features/review/ReviewPage.tsx`
- Modify: `web/src/styles/app.css`
- Modify: `web/src/test/ReviewPage.test.tsx`
- Modify: `web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing frontend tests**

```tsx
it('renders progress percentage, progress bar, and formatted command preview', () => {
  render(
    <ReviewPage
      source={source}
      bdinfo={bdinfo}
      draft={draft}
      outputFilename="Nightcrawler - 2160p.mkv"
      outputPath="/remux/Nightcrawler - 2160p.mkv"
      submitting={false}
      startDisabled={false}
      submitError={null}
      currentJob={{
        id: 'job-123',
        sourceName: 'Nightcrawler Disc',
        outputName: 'Nightcrawler - 2160p.mkv',
        outputPath: '/remux/Nightcrawler - 2160p.mkv',
        playlistName: '00003.MPLS',
        createdAt: '2026-03-30T00:00:00Z',
        status: 'running',
        progressPercent: 42,
        commandPreview: 'mkvmerge\\n  --output\\n  /remux/Nightcrawler - 2160p.mkv',
      }}
      currentLog="[2026-03-30T00:00:01Z] Progress: 42%"
      onBack={() => {}}
      onSubmit={() => {}}
    />,
  )

  expect(screen.getByText('42%')).toBeInTheDocument()
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '42')
  expect(screen.getByText(/mkvmerge/i)).toBeInTheDocument()
  expect(screen.getByText(/--output/i)).toBeInTheDocument()
})
```

```tsx
it('hydrates terminal tasks with command preview and 100 percent progress', async () => {
  const terminalJob = {
    id: 'job-999',
    sourceName: 'Nightcrawler Disc',
    outputName: 'Nightcrawler - 2160p.mkv',
    outputPath: '/remux/Nightcrawler - 2160p.mkv',
    playlistName: '00003.MPLS',
    createdAt: '2026-03-30T00:00:00Z',
    status: 'succeeded',
    progressPercent: 100,
    commandPreview: 'mkvmerge\\n  --output\\n  /remux/Nightcrawler - 2160p.mkv',
  }
  installFetchMock({
    currentJob: null,
    currentLog: '[2026-03-30T00:00:01Z] Progress: 100%\\n[2026-03-30T00:00:02Z] completed',
    submittedJob: terminalJob,
    submitStatus: 200,
    submitMessage: '',
  })
  render(<App />)

  await goToReviewStep()
  fireEvent.click(screen.getByRole('button', { name: /start remux/i }))

  await screen.findByText(/current remux/i)
  expect(await screen.findByText('100%')).toBeInTheDocument()
  expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100')
  expect(screen.getByText(/mkvmerge/i)).toBeInTheDocument()
  expect(screen.getByText(/--output/i)).toBeInTheDocument()
})
```

- [ ] **Step 2: Run the frontend tests and verify red**

Run: `cd web && npm test -- --run src/test/ReviewPage.test.tsx src/test/App.test.tsx`

Expected: FAIL because `progressPercent`, `commandPreview`, and progress bar UI are not implemented yet.

- [ ] **Step 3: Extend job types and render the new review sections**

```ts
export type Job = {
  id: string
  sourceName: string
  outputName: string
  outputPath?: string
  playlistName: string
  createdAt: string
  status: JobStatus
  message?: string
  commandPreview?: string
  progressPercent?: number
}
```

```tsx
const progressPercent =
  currentJob?.status === 'succeeded'
    ? 100
    : Math.max(0, Math.min(100, currentJob?.progressPercent ?? 0))
```

```tsx
<div className="current-job-progress">
  <div className="row">
    <h4>Progress</h4>
    <strong>{progressPercent}%</strong>
  </div>
  <div
    className="progress-bar"
    role="progressbar"
    aria-valuemin={0}
    aria-valuemax={100}
    aria-valuenow={progressPercent}
  >
    <div className="progress-bar-fill" style={{ width: `${progressPercent}%` }} />
  </div>
</div>

{currentJob?.commandPreview ? (
  <div className="current-job-command">
    <h4>MKVMerge Command</h4>
    <pre className="command-preview">{currentJob.commandPreview}</pre>
  </div>
) : null}
```

- [ ] **Step 4: Run frontend tests and confirm green**

Run: `cd web && npm test -- --run src/test/ReviewPage.test.tsx src/test/App.test.tsx`

Expected: PASS with command preview and progress UI covered.

- [ ] **Step 5: Commit the frontend review update**

```bash
git add web/src/api/types.ts web/src/App.tsx web/src/features/review/ReviewPage.tsx web/src/styles/app.css web/src/test/ReviewPage.test.tsx web/src/test/App.test.tsx
git commit -m "feat: show remux command and progress on review"
```

### Task 3: Final Verification

**Files:**
- Review only changed backend/frontend files

- [ ] **Step 1: Run focused backend verification**

Run: `go test ./internal/remux ./internal/http/handlers -v`

Expected: PASS with current-task payload, command preview, and progress handling green.

- [ ] **Step 2: Run focused frontend verification**

Run: `cd web && npm test -- --run`

Expected: PASS with review page tests, app tests, and existing frontend tests all green.

- [ ] **Step 3: Build frontend once**

Run: `cd web && npm run build`

Expected: PASS and emit production assets without type errors.

- [ ] **Step 4: Commit cleanup if needed**

```bash
git add internal web
git commit -m "test: cover review progress and command preview"
```
