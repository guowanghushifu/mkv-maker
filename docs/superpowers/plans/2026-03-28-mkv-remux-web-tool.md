# MKV Remux Web Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Dockerized single-user Go + React web application that scans extracted BDMV sources, requires matching BDInfo input to resolve the target playlist, lets the user edit export tracks, and produces remuxed MKV jobs with persistent history.

**Architecture:** A Go HTTP server exposes authenticated JSON APIs, serves the React SPA, persists state in SQLite, and runs a single sequential worker for remux jobs. The frontend is a Vite React app that guides the user through scan, required BDInfo input, track editing, review, and job history flows. There is no manual playlist-selection page; the workflow resolves the target playlist exclusively from the user-provided BDInfo log.

**Tech Stack:** Go, chi router, modernc SQLite driver, React, TypeScript, Vite, Vitest, Testing Library, Docker, GitHub Actions

---

## Planned Repository Structure

## Requirements Update (2026-03-29)

The approved workflow changed after this plan was first written. These rules supersede any conflicting older text below:

- BDInfo input is required and cannot be skipped.
- Only extracted `BDMV` folders are supported as remux inputs.
- The target playlist is determined from the user-provided BDInfo log.
- Manual playlist selection UI is removed from scope.
- `GET /api/sources/{id}/playlists` is removed from scope.
- `web/src/features/playlists/PlaylistPage.tsx` is removed from scope.
- Any existing playlist-ranking helper code is secondary internal logic only; it is not part of the primary user flow.

### Root files

- Create: `.gitignore`
- Create: `README.md`
- Create: `Dockerfile`
- Create: `go.mod`
- Create: `go.sum`

### Backend

- Create: `cmd/server/main.go`
- Create: `internal/app/app.go`
- Create: `internal/config/config.go`
- Create: `internal/http/router.go`
- Create: `internal/http/middleware/auth.go`
- Create: `internal/http/handlers/auth.go`
- Create: `internal/http/handlers/config.go`
- Create: `internal/http/handlers/sources.go`
- Create: `internal/http/handlers/bdinfo.go`
- Create: `internal/http/handlers/drafts.go`
- Create: `internal/http/handlers/jobs.go`
- Create: `internal/store/db.go`
- Create: `internal/store/migrate.go`
- Create: `internal/store/session_store.go`
- Create: `internal/store/job_store.go`
- Create: `internal/media/scanner.go`
- Create: `internal/media/scanner_test.go`
- Create: `internal/config/config_test.go`
- Create: `internal/http/router_test.go`
- Create: `internal/media/bdinfo/parser.go`
- Create: `internal/media/bdinfo/parser_test.go`
- Create: `internal/media/analyzer/types.go`
- Create: `internal/media/analyzer/service.go`
- Create: `internal/media/analyzer/service_test.go`
- Create: `internal/remux/draft.go`
- Create: `internal/remux/draft_test.go`
- Create: `internal/remux/filename.go`
- Create: `internal/remux/filename_test.go`
- Create: `internal/remux/command_builder.go`
- Create: `internal/remux/command_builder_test.go`
- Create: `internal/queue/manager.go`
- Create: `internal/queue/manager_test.go`

### Frontend

- Create: `web/package.json`
- Create: `web/package-lock.json`
- Create: `web/tsconfig.json`
- Create: `web/vite.config.ts`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/api/client.ts`
- Create: `web/src/api/types.ts`
- Create: `web/src/components/Layout.tsx`
- Create: `web/src/components/StatusBadge.tsx`
- Create: `web/src/features/auth/LoginPage.tsx`
- Create: `web/src/features/sources/ScanPage.tsx`
- Create: `web/src/features/bdinfo/BDInfoPage.tsx`
- Create: `web/src/features/draft/TrackEditorPage.tsx`
- Create: `web/src/features/review/ReviewPage.tsx`
- Create: `web/src/features/jobs/JobsPage.tsx`
- Create: `web/src/styles/app.css`
- Create: `web/src/test/App.test.tsx`
- Create: `web/src/test/TrackEditorPage.test.tsx`

### Tooling and release

- Create: `scripts/docker-build.sh`
- Create: `scripts/docker-run.sh`
- Create: `.github/workflows/docker-publish.yml`

## Task 1: Bootstrap Repository and Tooling

**Files:**
- Create: `.gitignore`
- Create: `README.md`
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `web/package.json`
- Create: `web/tsconfig.json`
- Create: `web/vite.config.ts`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing frontend smoke test**

```tsx
// web/src/test/App.test.tsx
import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import App from '../App';

describe('App', () => {
  it('renders the application shell title', () => {
    render(<App />);
    expect(screen.getByRole('heading', { name: /MKV Remux Tool/i })).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the frontend test to verify it fails**

Run: `npm --prefix web test -- --run web/src/test/App.test.tsx`

Expected: FAIL with a module-not-found error for `../App` or missing Vite/Vitest config.

- [ ] **Step 3: Write the minimal bootstrap implementation**

```json
// web/package.json
{
  "name": "mkv-remux-web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "test": "vitest"
  },
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "^6.0.0",
    "@testing-library/react": "^16.0.0",
    "@types/react": "^19.0.0",
    "@types/react-dom": "^19.0.0",
    "@vitejs/plugin-react": "^5.0.0",
    "jsdom": "^26.0.0",
    "typescript": "^5.8.0",
    "vite": "^7.0.0",
    "vitest": "^3.0.0"
  }
}
```

```tsx
// web/src/App.tsx
export default function App() {
  return (
    <main>
      <h1>MKV Remux Tool</h1>
    </main>
  );
}
```

```tsx
// web/src/main.tsx
import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

```go
// cmd/server/main.go
package main

import "fmt"

func main() {
	fmt.Println("mkv-remux-web server bootstrap")
}
```

```gitignore
# .gitignore
.DS_Store
node_modules/
web/dist/
coverage/
.superpowers/
app.db
```

- [ ] **Step 4: Run the frontend test to verify it passes**

Run: `npm --prefix web test -- --run web/src/test/App.test.tsx`

Expected: PASS with `1 passed`.

- [ ] **Step 5: Commit the bootstrap**

```bash
git add .gitignore README.md go.mod cmd/server/main.go web/package.json web/tsconfig.json web/vite.config.ts web/index.html web/src/main.tsx web/src/App.tsx web/src/test/App.test.tsx
git commit -m "chore: bootstrap go and react toolchains"
```

## Task 2: Implement Backend Config, Database, and Session Auth

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/store/db.go`
- Create: `internal/store/migrate.go`
- Create: `internal/store/session_store.go`
- Create: `internal/http/middleware/auth.go`
- Create: `internal/http/handlers/auth.go`
- Create: `internal/http/handlers/config.go`
- Create: `internal/http/router.go`
- Create: `internal/app/app.go`
- Test: `internal/config/config_test.go`
- Test: `internal/store/session_store_test.go`

- [ ] **Step 1: Write the failing backend tests for config and login**

```go
// internal/config/config_test.go
package config

import (
	"testing"
)

func TestLoadRejectsEmptyPassword(t *testing.T) {
	t.Setenv("APP_PASSWORD", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected empty APP_PASSWORD to fail")
	}
}
```

```go
// internal/store/session_store_test.go
package store

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSessionStoreCreatesAndValidatesSession(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	token, err := store.Create("127.0.0.1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if ok := store.Valid(token); !ok {
		t.Fatal("expected created session token to validate")
	}
}

func openTestDB(t *testing.T) *sql.DB {
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
```

- [ ] **Step 2: Run the backend tests to verify they fail**

Run: `go test ./internal/config ./internal/store -run 'TestLoadRejectsEmptyPassword|TestSessionStoreCreatesAndValidatesSession' -v`

Expected: FAIL because `Load`, `openTestDB`, `NewSessionStore`, or `Valid` do not exist yet.

- [ ] **Step 3: Write the minimal backend config and session implementation**

```go
// internal/config/config.go
package config

import (
	"errors"
	"os"
)

type Config struct {
	AppPassword    string
	InputDir       string
	OutputDir      string
	DataDir        string
	ListenAddr     string
	SessionMaxAge  int
}

func Load() (Config, error) {
	cfg := Config{
		AppPassword:   os.Getenv("APP_PASSWORD"),
		InputDir:      getenvDefault("BD_INPUT_DIR", "/bd_input"),
		OutputDir:     getenvDefault("REMUX_OUTPUT_DIR", "/remux"),
		DataDir:       getenvDefault("APP_DATA_DIR", "/app/data"),
		ListenAddr:    getenvDefault("LISTEN_ADDR", ":8080"),
		SessionMaxAge: 86400,
	}
	if cfg.AppPassword == "" {
		return Config{}, errors.New("APP_PASSWORD is required")
	}
	return cfg, nil
}

func getenvDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
```

```go
// internal/store/session_store.go
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
)

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) Create(remoteAddr string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	_, err := s.db.Exec(`insert into sessions(token, remote_addr) values(?, ?)`, token, remoteAddr)
	return token, err
}

func (s *SessionStore) Valid(token string) bool {
	var count int
	_ = s.db.QueryRow(`select count(1) from sessions where token = ?`, token).Scan(&count)
	return count == 1
}
```

```go
// internal/store/migrate.go
package store

import "database/sql"

func Migrate(db *sql.DB) error {
	stmts := []string{
		`create table if not exists sessions (
			token text primary key,
			remote_addr text not null,
			created_at datetime not null default current_timestamp
		);`,
		`create table if not exists jobs (
			id text primary key,
			status text not null,
			draft_json text not null,
			output_path text not null default '',
			log_path text not null default '',
			error_text text not null default '',
			created_at datetime not null default current_timestamp,
			started_at datetime,
			finished_at datetime
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
```

```go
// internal/store/db.go
package store

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}
```

- [ ] **Step 4: Run the backend tests to verify they pass**

Run: `go test ./internal/config ./internal/store -run 'TestLoadRejectsEmptyPassword|TestSessionStoreCreatesAndValidatesSession' -v`

Expected: PASS with both tests green.

- [ ] **Step 5: Commit the backend foundation**

```bash
git add internal/config/config.go internal/config/config_test.go internal/store/db.go internal/store/migrate.go internal/store/session_store.go internal/store/session_store_test.go internal/http/middleware/auth.go internal/http/handlers/auth.go internal/http/handlers/config.go internal/http/router.go internal/app/app.go
git commit -m "feat: add config loading and session auth foundation"
```

## Task 3: Implement Source Scanning

**Files:**
- Create: `internal/media/scanner.go`
- Test: `internal/media/scanner_test.go`
- Modify: `internal/http/handlers/sources.go`
- Modify: `internal/http/router.go`

- [ ] **Step 1: Write the failing source scan tests**

```go
// internal/media/scanner_test.go
package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScannerFindsBDMVFoldersOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "DiscA", "BDMV", "PLAYLIST"), 0o755); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner()
	items, err := scanner.Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}
```

- [ ] **Step 2: Run the source scan test to verify it fails**

Run: `go test ./internal/media -run TestScannerFindsBDMVFoldersOnly -v`

Expected: FAIL because `NewScanner` or `Scan` does not exist.

- [ ] **Step 3: Write the minimal source scanner**

```go
// internal/media/scanner.go
package media

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type SourceType string

const (
	SourceBDMV SourceType = "bdmv"
)

type SourceEntry struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	Type       SourceType `json:"type"`
	Size       int64      `json:"size"`
	ModifiedAt time.Time  `json:"modifiedAt"`
}

type Scanner struct{}

func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) Scan(root string) ([]SourceEntry, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var out []SourceEntry
	for _, entry := range entries {
		fullPath := filepath.Join(root, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		if entry.IsDir() && isBDMVRoot(fullPath) {
			out = append(out, SourceEntry{
				ID:         entry.Name(),
				Name:       entry.Name(),
				Path:       fullPath,
				Type:       SourceBDMV,
				Size:       0,
				ModifiedAt: info.ModTime(),
			})
		}
	}

	slices.SortFunc(out, func(a, b SourceEntry) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

func isBDMVRoot(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "BDMV", "PLAYLIST")); err == nil {
		return true
	}
	_, err := os.Stat(filepath.Join(path, "BDMV", "index.bdmv"))
	return err == nil
}
```

- [ ] **Step 4: Run the source scan test to verify it passes**

Run: `go test ./internal/media -run TestScannerFindsBDMVFoldersOnly -v`

Expected: PASS with the scanner returning both sources.

- [ ] **Step 5: Commit source scanning**

```bash
git add internal/media/scanner.go internal/media/scanner_test.go internal/http/handlers/sources.go internal/http/router.go
git commit -m "feat: add blu-ray source scanning"
```

## Task 4: Implement BDInfo Parsing

**Files:**
- Create: `internal/media/bdinfo/parser.go`
- Create: `internal/media/bdinfo/parser_test.go`
- Modify: `internal/http/handlers/bdinfo.go`

- [ ] **Step 1: Write the failing BDInfo parser test**

```go
// internal/media/bdinfo/parser_test.go
package bdinfo

import "testing"

func TestParseExtractsPlaylistAndTracks(t *testing.T) {
	logText := `
PLAYLIST: 00800.MPLS
VIDEO: MPEG-H HEVC Video / 57999 kbps / 2160p / 23.976 fps / 16:9 / Main 10 / HDR10 / BT.2020
AUDIO: English / Dolby TrueHD/Atmos Audio / 7.1 / 48 kHz / 3984 kbps / 24-bit
SUBTITLE: English / 20.123 kbps
`

	parsed, err := Parse(logText)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if parsed.PlaylistName != "00800.MPLS" {
		t.Fatalf("expected playlist 00800.MPLS, got %q", parsed.PlaylistName)
	}
	if len(parsed.AudioTracks) != 1 || len(parsed.SubtitleTracks) != 1 {
		t.Fatal("expected one audio track and one subtitle track")
	}
}
```

- [ ] **Step 2: Run the BDInfo parser test to verify it fails**

Run: `go test ./internal/media/bdinfo -run TestParseExtractsPlaylistAndTracks -v`

Expected: FAIL because `Parse` and result types do not exist.

- [ ] **Step 3: Write the minimal BDInfo parser**

```go
// internal/media/bdinfo/parser.go
package bdinfo

import (
	"bufio"
	"strings"
)

type Parsed struct {
	Title         string       `json:"title"`
	PlaylistName  string       `json:"playlistName"`
	VideoTracks   []TrackLabel `json:"videoTracks"`
	AudioTracks   []TrackLabel `json:"audioTracks"`
	SubtitleTracks []TrackLabel `json:"subtitleTracks"`
}

type TrackLabel struct {
	RawLine   string `json:"rawLine"`
	Language  string `json:"language"`
	Name      string `json:"name"`
}

func Parse(input string) (Parsed, error) {
	var parsed Parsed
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "PLAYLIST:"):
			parsed.PlaylistName = strings.TrimSpace(strings.TrimPrefix(line, "PLAYLIST:"))
		case strings.HasPrefix(line, "VIDEO:"):
			parsed.VideoTracks = append(parsed.VideoTracks, TrackLabel{RawLine: line, Name: strings.TrimSpace(strings.TrimPrefix(line, "VIDEO:"))})
		case strings.HasPrefix(line, "AUDIO:"):
			parsed.AudioTracks = append(parsed.AudioTracks, parseTrack("AUDIO:", line))
		case strings.HasPrefix(line, "SUBTITLE:"):
			parsed.SubtitleTracks = append(parsed.SubtitleTracks, parseTrack("SUBTITLE:", line))
		}
	}
	return parsed, scanner.Err()
}

func parseTrack(prefix, line string) TrackLabel {
	body := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	parts := strings.Split(body, "/")
	label := TrackLabel{RawLine: line, Name: body}
	if len(parts) > 0 {
		label.Language = strings.TrimSpace(parts[0])
	}
	return label
}
```

- [ ] **Step 4: Run the BDInfo parser test to verify it passes**

Run: `go test ./internal/media/bdinfo -run TestParseExtractsPlaylistAndTracks -v`

Expected: PASS with playlist and track counts matching the fixture.

- [ ] **Step 5: Commit BDInfo parsing**

```bash
git add internal/media/bdinfo/parser.go internal/media/bdinfo/parser_test.go internal/http/handlers/bdinfo.go
git commit -m "feat: parse bdinfo playlist and track labels"
```

## Task 5: Implement Playlist Resolution and Filename Generation

**Files:**
- Create: `internal/media/analyzer/types.go`
- Create: `internal/media/analyzer/service.go`
- Create: `internal/media/analyzer/service_test.go`
- Create: `internal/remux/draft.go`
- Create: `internal/remux/draft_test.go`
- Create: `internal/remux/filename.go`
- Create: `internal/remux/filename_test.go`
- Modify: `internal/http/handlers/drafts.go`

- [ ] **Step 1: Write the failing tests for draft resolution and filename generation**

```go
// internal/remux/filename_test.go
package remux

import "testing"

func TestBuildFilenameIncludesHDRAndDefaultAudio(t *testing.T) {
	draft := Draft{
		Title: "Nightcrawler",
		Video: VideoTrack{
			Resolution: "2160p",
			Codec:      "HEVC",
			HDRType:    "HDR.DV",
		},
		Audio: []AudioTrack{
			{Name: "English", CodecLabel: "TrueHD.7.1.Atmos", Default: true, Selected: true},
		},
	}

	got := BuildFilename(draft)
	want := "Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.TrueHD.7.1.Atmos.mkv"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
```

```go
// internal/media/analyzer/service_test.go
package analyzer

import "testing"

func TestRankPlaylistsMarksLongestAsFeatureCandidate(t *testing.T) {
	playlists := []PlaylistInfo{
		{Name: "00001.MPLS", DurationSeconds: 600, SizeBytes: 1_000},
		{Name: "00800.MPLS", DurationSeconds: 7200, SizeBytes: 30_000},
	}

	ranked := RankPlaylists(playlists)
	if !ranked[0].IsFeatureCandidate || ranked[0].Name != "00800.MPLS" {
		t.Fatalf("expected 00800.MPLS to be the top feature candidate, got %+v", ranked[0])
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/media/analyzer ./internal/remux -run 'TestRankPlaylistsMarksLongestAsFeatureCandidate|TestBuildFilenameIncludesHDRAndDefaultAudio' -v`

Expected: FAIL because ranking and filename builders are not implemented.

- [ ] **Step 3: Write the minimal analyzer and filename implementation**

```go
// internal/media/analyzer/types.go
package analyzer

type PlaylistInfo struct {
	Name               string `json:"name"`
	DurationSeconds    int    `json:"durationSeconds"`
	SizeBytes          int64  `json:"sizeBytes"`
	ChapterCount       int    `json:"chapterCount"`
	VideoSummary       string `json:"videoSummary"`
	FeatureScore       int64  `json:"featureScore"`
	IsFeatureCandidate bool   `json:"isFeatureCandidate"`
}
```

```go
// internal/media/analyzer/service.go
package analyzer

import "slices"

func RankPlaylists(in []PlaylistInfo) []PlaylistInfo {
	out := append([]PlaylistInfo(nil), in...)
	for i := range out {
		out[i].FeatureScore = int64(out[i].DurationSeconds)*1000 + out[i].SizeBytes + int64(out[i].ChapterCount)*100
	}
	slices.SortFunc(out, func(a, b PlaylistInfo) int {
		switch {
		case a.FeatureScore > b.FeatureScore:
			return -1
		case a.FeatureScore < b.FeatureScore:
			return 1
		default:
			return 0
		}
	})
	if len(out) > 0 {
		out[0].IsFeatureCandidate = true
	}
	return out
}
```

```go
// internal/remux/filename.go
package remux

import "strings"

func BuildFilename(d Draft) string {
	audioLabel := "UnknownAudio"
	for _, track := range d.Audio {
		if track.Selected && track.Default {
			audioLabel = track.CodecLabel
			break
		}
	}

	parts := []string{
		d.Title + " - " + d.Video.Resolution,
		"BluRay",
		d.Video.HDRType,
		d.Video.Codec,
		audioLabel,
	}
	return strings.Join(compact(parts), ".") + ".mkv"
}

func compact(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		if strings.TrimSpace(item) != "" {
			out = append(out, item)
		}
	}
	return out
}
```

```go
// internal/remux/draft.go
package remux

type Draft struct {
	Title      string
	SourcePath string
	Playlist   string
	OutputPath string
	EnableDV   bool
	Video      VideoTrack
	Audio      []AudioTrack
}

type VideoTrack struct {
	Name       string
	Resolution string
	Codec      string
	HDRType    string
}

type AudioTrack struct {
	ID         string
	Name       string
	Language   string
	CodecLabel string
	Default    bool
	Selected   bool
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/media/analyzer ./internal/remux -run 'TestRankPlaylistsMarksLongestAsFeatureCandidate|TestBuildFilenameIncludesHDRAndDefaultAudio' -v`

Expected: PASS with the longest playlist recommended and the filename matching the target format.

- [ ] **Step 5: Commit draft resolution foundations**

```bash
git add internal/media/analyzer/types.go internal/media/analyzer/service.go internal/media/analyzer/service_test.go internal/remux/draft.go internal/remux/draft_test.go internal/remux/filename.go internal/remux/filename_test.go internal/http/handlers/drafts.go
git commit -m "feat: add playlist ranking and filename generation"
```

## Task 6: Implement MKVMerge Command Builder and Job Queue

**Files:**
- Create: `internal/remux/command_builder.go`
- Create: `internal/remux/command_builder_test.go`
- Create: `internal/store/job_store.go`
- Create: `internal/queue/manager.go`
- Create: `internal/queue/manager_test.go`
- Modify: `internal/http/handlers/jobs.go`

- [ ] **Step 1: Write the failing tests for command building and queue transitions**

```go
// internal/remux/command_builder_test.go
package remux

import (
	"strings"
	"testing"
)

func TestBuildMKVMergeArgsIncludesTrackMetadata(t *testing.T) {
	draft := Draft{
		OutputPath: "/remux/Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.TrueHD.7.1.Atmos.mkv",
		SourcePath: "/bd_input/Nightcrawler",
		Playlist:   "00800.MPLS",
		EnableDV:   true,
		Video:      VideoTrack{Name: "Main Video"},
		Audio:      []AudioTrack{{ID: "a1", Name: "English Atmos", Language: "eng", Default: true, Selected: true}},
	}

	args := BuildMKVMergeArgs(draft)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--track-name") || !strings.Contains(joined, "English Atmos") {
		t.Fatalf("expected mkvmerge args to include track naming, got %q", joined)
	}
	if !strings.Contains(joined, "--output") {
		t.Fatalf("expected mkvmerge args to include output path, got %q", joined)
	}
}
```

```go
// internal/queue/manager_test.go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/remux ./internal/queue -run 'TestBuildMKVMergeArgsIncludesTrackMetadata|TestRecoverRunningJobsMarksThemInterrupted' -v`

Expected: FAIL because `BuildMKVMergeArgs`, queue recovery, or job store support is missing.

- [ ] **Step 3: Write the minimal command builder and queue manager**

```go
// internal/remux/command_builder.go
package remux

import "strconv"

func BuildMKVMergeArgs(d Draft) []string {
	args := []string{
		"--output", d.OutputPath,
	}
	if d.Video.Name != "" {
		args = append(args, "--track-name", "0:"+d.Video.Name)
	}
	for index, track := range d.Audio {
		if !track.Selected {
			continue
		}
		args = append(args, "--language", strconv.Itoa(index+1)+":"+track.Language)
		args = append(args, "--track-name", strconv.Itoa(index+1)+":"+track.Name)
		if track.Default {
			args = append(args, "--default-track-flag", strconv.Itoa(index+1)+":yes")
		}
	}
	if d.EnableDV {
		args = append(args, "--engage", "merge_dolby_vision")
	}
	args = append(args, d.SourcePath)
	return args
}
```

```go
// internal/queue/manager.go
package queue

type JobStore interface {
	MarkRunningJobsInterrupted() error
}

type Manager struct {
	store JobStore
}

func NewManager(store JobStore, _ any) *Manager {
	return &Manager{store: store}
}

func (m *Manager) Recover() error {
	return m.store.MarkRunningJobsInterrupted()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/remux ./internal/queue -run 'TestBuildMKVMergeArgsIncludesTrackMetadata|TestRecoverRunningJobsMarksThemInterrupted' -v`

Expected: PASS with queue recovery marking running jobs as interrupted and mkvmerge args including output and track metadata.

- [ ] **Step 5: Commit queue and command builder**

```bash
git add internal/remux/command_builder.go internal/remux/command_builder_test.go internal/store/job_store.go internal/queue/manager.go internal/queue/manager_test.go internal/http/handlers/jobs.go
git commit -m "feat: add remux command builder and queue recovery"
```

## Task 7: Implement Authenticated HTTP API Integration

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/http/router.go`
- Modify: `internal/http/middleware/auth.go`
- Modify: `internal/http/handlers/auth.go`
- Modify: `internal/http/handlers/config.go`
- Modify: `internal/http/handlers/sources.go`
- Modify: `internal/http/handlers/bdinfo.go`
- Modify: `internal/http/handlers/drafts.go`
- Modify: `internal/http/handlers/jobs.go`
- Test: `internal/http/router_test.go`

- [ ] **Step 1: Write the failing API integration test**

```go
// internal/http/router_test.go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProtectedRouteRejectsAnonymousRequests(t *testing.T) {
	router := NewRouter(TestDependencies())
	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}
```

- [ ] **Step 2: Run the API integration test to verify it fails**

Run: `go test ./internal/http -run TestProtectedRouteRejectsAnonymousRequests -v`

Expected: FAIL because `NewRouter` or its auth middleware wiring is not complete.

- [ ] **Step 3: Implement the authenticated API surface**

```go
// internal/http/router.go
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Dependencies struct {
	AuthMiddleware func(http.Handler) http.Handler
	AuthHandler    http.HandlerFunc
	LogoutHandler    http.HandlerFunc
	ConfigHandler    http.HandlerFunc
	ScanHandler      http.HandlerFunc
	SourcesHandler   http.HandlerFunc
	ResolveHandler   http.HandlerFunc
	BDInfoHandler    http.HandlerFunc
	DraftsHandler    http.HandlerFunc
	JobsHandler      http.HandlerFunc
	JobDetailHandler http.HandlerFunc
	JobLogHandler    http.HandlerFunc
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Post("/api/login", deps.AuthHandler)
	r.Post("/api/logout", deps.LogoutHandler)
	r.Group(func(protected chi.Router) {
		protected.Use(deps.AuthMiddleware)
		protected.Get("/api/config", deps.ConfigHandler)
		protected.Post("/api/sources/scan", deps.ScanHandler)
		protected.Get("/api/sources", deps.SourcesHandler)
		protected.Post("/api/sources/{id}/resolve", deps.ResolveHandler)
		protected.Post("/api/bdinfo/parse", deps.BDInfoHandler)
		protected.Post("/api/drafts/preview-filename", deps.DraftsHandler)
		protected.Get("/api/jobs", deps.JobsHandler)
		protected.Get("/api/jobs/{id}", deps.JobDetailHandler)
		protected.Get("/api/jobs/{id}/log", deps.JobLogHandler)
	})
	return r
}
```

```go
// internal/http/middleware/auth.go
package middleware

import "net/http"

func RequireSession(valid func(string) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil || !valid(cookie.Value) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run the API integration test to verify it passes**

Run: `go test ./internal/http -run TestProtectedRouteRejectsAnonymousRequests -v`

Expected: PASS with unauthenticated requests returning HTTP 401.

- [ ] **Step 5: Commit API integration**

```bash
git add internal/app/app.go internal/http/router.go internal/http/router_test.go internal/http/middleware/auth.go internal/http/handlers/auth.go internal/http/handlers/config.go internal/http/handlers/sources.go internal/http/handlers/bdinfo.go internal/http/handlers/drafts.go internal/http/handlers/jobs.go
git commit -m "feat: wire authenticated api routes"
```

## Task 8: Build the React Workflow UI

**Files:**
- Create: `web/src/api/client.ts`
- Create: `web/src/api/types.ts`
- Create: `web/src/components/Layout.tsx`
- Create: `web/src/components/StatusBadge.tsx`
- Create: `web/src/features/auth/LoginPage.tsx`
- Create: `web/src/features/sources/ScanPage.tsx`
- Create: `web/src/features/bdinfo/BDInfoPage.tsx`
- Create: `web/src/features/draft/TrackEditorPage.tsx`
- Create: `web/src/features/review/ReviewPage.tsx`
- Create: `web/src/features/jobs/JobsPage.tsx`
- Create: `web/src/styles/app.css`
- Create: `web/src/test/TrackEditorPage.test.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Write the failing UI behavior test**

```tsx
// web/src/test/TrackEditorPage.test.tsx
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { TrackEditorPage } from '../features/draft/TrackEditorPage';

describe('TrackEditorPage', () => {
  it('moves a selected audio track upward in the export order', () => {
    const onChange = vi.fn();
    render(
      <TrackEditorPage
        draft={{
          video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'HDR.DV' },
          audio: [
            { id: 'a1', name: 'English Atmos', language: 'eng', selected: true, default: true },
            { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
          ],
          subtitles: [],
        }}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /move commentary up/i }));
    expect(onChange).toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run the UI test to verify it fails**

Run: `npm --prefix web test -- --run web/src/test/TrackEditorPage.test.tsx`

Expected: FAIL because `TrackEditorPage` and UI wiring do not exist.

- [ ] **Step 3: Implement the UI workflow**

```tsx
// web/src/features/draft/TrackEditorPage.tsx
import type { Draft, DraftTrack } from '../../api/types';

type Props = {
  draft: Draft;
  onChange: (next: Draft) => void;
};

export function TrackEditorPage({ draft, onChange }: Props) {
  const moveUp = (index: number) => {
    if (index === 0) return;
    const nextAudio = [...draft.audio];
    [nextAudio[index - 1], nextAudio[index]] = [nextAudio[index], nextAudio[index - 1]];
    onChange({ ...draft, audio: nextAudio });
  };

  return (
    <section>
      <h2>Track Editor</h2>
      <ul>
        {draft.audio.map((track: DraftTrack, index: number) => (
          <li key={track.id}>
            <span>{track.name}</span>
            <button type="button" onClick={() => moveUp(index)}>
              Move {track.name} up
            </button>
          </li>
        ))}
      </ul>
    </section>
  );
}
```

```tsx
// web/src/App.tsx
import { useState } from 'react';
import { LoginPage } from './features/auth/LoginPage';
import { ScanPage } from './features/sources/ScanPage';

export default function App() {
  const [authenticated, setAuthenticated] = useState(false);

  if (!authenticated) {
    return <LoginPage onSuccess={() => setAuthenticated(true)} />;
  }

  return <ScanPage />;
}
```

- [ ] **Step 4: Run the UI test to verify it passes**

Run: `npm --prefix web test -- --run web/src/test/TrackEditorPage.test.tsx`

Expected: PASS with the reorder callback firing after the move-up action.

- [ ] **Step 5: Commit the React workflow**

```bash
git add web/src/api/client.ts web/src/api/types.ts web/src/components/Layout.tsx web/src/components/StatusBadge.tsx web/src/features/auth/LoginPage.tsx web/src/features/sources/ScanPage.tsx web/src/features/bdinfo/BDInfoPage.tsx web/src/features/draft/TrackEditorPage.tsx web/src/features/review/ReviewPage.tsx web/src/features/jobs/JobsPage.tsx web/src/styles/app.css web/src/test/TrackEditorPage.test.tsx web/src/App.tsx
git commit -m "feat: add remux workflow frontend"
```

## Task 9: Package the Application and Publish Workflow

**Files:**
- Create: `Dockerfile`
- Create: `scripts/docker-build.sh`
- Create: `scripts/docker-run.sh`
- Create: `.github/workflows/docker-publish.yml`
- Modify: `README.md`

- [ ] **Step 1: Write the failing packaging verification commands**

```bash
docker build -t mkv-remux-web:test .
npm --prefix web run build
go test ./...
```

Expected: FAIL initially because Dockerfile, scripts, and production build wiring do not exist.

- [ ] **Step 2: Add the Docker and release implementation**

```dockerfile
# Dockerfile
FROM node:22-bookworm AS web-build
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web ./
RUN npm run build

FROM golang:1.24-bookworm AS go-build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

FROM debian:bookworm-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg mediainfo mkvtoolnix ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=go-build /out/server /app/server
COPY --from=web-build /app/web/dist /app/web/dist
ENV BD_INPUT_DIR=/bd_input \
    REMUX_OUTPUT_DIR=/remux \
    APP_DATA_DIR=/app/data \
    LISTEN_ADDR=:8080
EXPOSE 8080
CMD ["/app/server"]
```

```bash
# scripts/docker-build.sh
#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${1:-mkv-remux-web:local}"
docker build -t "${IMAGE_TAG}" .
```

```bash
# scripts/docker-run.sh
#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${1:-mkv-remux-web:local}"
: "${APP_PASSWORD:?APP_PASSWORD must be set}"
: "${HOST_BD_INPUT:?HOST_BD_INPUT must be set}"
: "${HOST_REMUX_OUTPUT:?HOST_REMUX_OUTPUT must be set}"

mkdir -p "${HOST_REMUX_OUTPUT}" "${PWD}/.data"

docker run --rm -p 8080:8080 \
  -e APP_PASSWORD="${APP_PASSWORD}" \
  -v "${HOST_BD_INPUT}:/bd_input:ro" \
  -v "${HOST_REMUX_OUTPUT}:/remux" \
  -v "${PWD}/.data:/app/data" \
  "${IMAGE_TAG}"
```

```yaml
# .github/workflows/docker-publish.yml
name: docker-publish

on:
  workflow_dispatch:
    inputs:
      image_tag:
        description: Docker image tag
        required: true
        default: latest
      push_latest:
        description: Also push latest
        required: true
        type: boolean
        default: false

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}
      - uses: docker/build-push-action@v6
        with:
          context: .
          push: true
          tags: |
            ${{ secrets.DOCKERHUB_USERNAME }}/mkv-remux-web:${{ inputs.image_tag }}
            ${{ inputs.push_latest && format('{0}/mkv-remux-web:latest', secrets.DOCKERHUB_USERNAME) || '' }}
```

- [ ] **Step 3: Run the packaging verification commands**

Run: `npm --prefix web run build && go test ./... && docker build -t mkv-remux-web:test .`

Expected: PASS with frontend build succeeding, Go tests green, and Docker image built locally.

- [ ] **Step 4: Update README with runtime and release instructions**

```md
# README.md

## Environment

- `APP_PASSWORD`: required login password
- `BD_INPUT_DIR`: Blu-ray input mount, default `/bd_input`
- `REMUX_OUTPUT_DIR`: output mount, default `/remux`
- `APP_DATA_DIR`: SQLite and logs directory, default `/app/data`

## Image variants

- Public and local images use the same free CLI stack
- BDInfo input is required to determine the target playlist

## Local run

    ```bash
APP_PASSWORD=secret \
HOST_BD_INPUT=/path/to/discs \
HOST_REMUX_OUTPUT=/path/to/remux \
./scripts/docker-run.sh
    ```

## Docker Hub publish

Set repository secrets:

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN`

Trigger `.github/workflows/docker-publish.yml` manually from the Actions tab.
```

- [ ] **Step 5: Commit packaging and release automation**

```bash
git add Dockerfile scripts/docker-build.sh scripts/docker-run.sh .github/workflows/docker-publish.yml README.md
git commit -m "chore: add docker packaging and release workflow"
```

## Final Verification Pass

- [ ] Run: `go test ./...`
- [ ] Run: `npm --prefix web test -- --run`
- [ ] Run: `npm --prefix web run build`
- [ ] Run: `docker build -t mkv-remux-web:final .`
- [ ] Confirm:
  - protected routes return 401 without login
  - scan finds BDMV directories
  - BDInfo parsing populates playlist and track labels
  - filename preview matches the agreed naming convention
  - queue recovery marks previously running jobs as interrupted
  - Docker image starts with mounted `/bd_input`, `/remux`, and `/app/data`

## Notes for Execution

- Prefer `subagent-driven-development` when implementing this plan because the backend media parsing and frontend workflow can be split into separate task ownership without overlapping writes.
- Keep remux execution code behind narrow interfaces so tests can mock external commands instead of requiring real Blu-ray media.
- Do not expand scope into loop mounting, concurrent remux workers, or public multi-user auth during implementation.
