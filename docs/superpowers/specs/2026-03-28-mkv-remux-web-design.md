# MKV Remux Web Tool Design

## Goal

Build a Linux Dockerized web tool that scans Blu-ray ISO files or extracted BDMV folders, requires the user to provide a matching BDInfo log to determine the target playlist, lets a single user edit export tracks, and produces remuxed MKV files with controlled track metadata and predictable output naming.

## Scope

The first version covers:

- Running as a single-container web application on Linux
- Single-user access protected by one password
- Input sources from a configured directory containing Blu-ray ISO files or extracted Blu-ray folders
- Required BDInfo log parsing to determine the target playlist and improve track naming
- No manual playlist selection flow; the user must provide a valid BDInfo log for the selected source
- Track selection, naming, language editing, default-track flags, and ordering for audio and subtitle tracks
- UHD Dolby Vision enhancement-layer merge into the main video stream when supported by the source and `mkvmerge`
- Sequential background job execution with persistent history and logs
- Docker build artifacts, local build/run scripts, and a manually triggered GitHub Actions workflow for Docker Hub publishing

Out of scope for the first version:

- Loop mounting ISO files inside the container
- Public multi-user hosting
- Advanced role-based auth
- Distributed workers or concurrent remux execution
- Automatic task resume after container restart

## Product Constraints

- Runtime OS target is Linux inside Docker
- The container does not mount ISO loop devices itself
- Inputs are limited to:
  - `.iso` files readable directly by external Blu-ray tooling
  - extracted Blu-ray folders containing a valid `BDMV` structure
- Input directory comes from environment variable `BD_INPUT_DIR`, default `/bd_input`
- Output directory comes from environment variable `REMUX_OUTPUT_DIR`, default `/remux`
- Application data directory comes from environment variable `APP_DATA_DIR`, default `/app/data`
- Password comes from environment variable `APP_PASSWORD` and is required

## Recommended Architecture

Use a single-process application with:

- Go backend for API, auth, queue management, persistence, CLI orchestration, and static asset serving
- React + TypeScript frontend for the browser workflow
- SQLite for persistent application state
- Text log files for per-job execution logs

This keeps the system small and operationally simple while still supporting the interaction depth needed for playlist inspection, track editing, and queued remux jobs.

## External Tooling Strategy

The application orchestrates established media CLI tools instead of reimplementing Blu-ray parsing or muxing:

- `mkvmerge`
  - creates the final MKV
  - applies track order, names, languages, and default flags
  - merges Dolby Vision enhancement data into the main video track when supported by the source and installed version
- `ffprobe`
  - verifies stream details for the playlist identified by the user-provided BDInfo log
- `mediainfo`
  - supplements HDR, audio layout, and codec labeling when needed

BDInfo is the required source of playlist selection and preferred human-readable track labels. The backend validates that the referenced playlist exists in the scanned Blu-ray source before allowing export drafting.

## User Model and Security

The tool is designed for one user or one trusted household environment.

Authentication model:

- A login screen requires the static password from `APP_PASSWORD`
- Successful login creates an `httpOnly` session cookie
- One authenticated session model is sufficient
- Logout is supported
- If `APP_PASSWORD` is empty, the service refuses to start

Security boundaries:

- No filesystem browsing outside configured input/output/data roots
- Source selection always comes from scan results rather than arbitrary raw user paths
- Job execution uses validated absolute paths resolved from scanned items
- Logs must avoid printing plaintext password values or secrets

## End-to-End Workflow

### 1. Login

The user opens the web UI, enters the configured password, and receives an authenticated session.

### 2. Scan Sources

The user clicks a scan button. The backend scans `BD_INPUT_DIR` and identifies:

- files ending in `.iso`
- directories containing `BDMV/index.bdmv` or `BDMV/PLAYLIST`

The UI displays the results in a list with:

- display name
- source type (`ISO` or `BDMV Folder`)
- absolute or normalized source path
- size
- modification time

### 3. Required BDInfo Paste

After selecting a source, the user must paste a BDInfo log into a text area.

The parser attempts to extract:

- source title or label
- playlist name such as `00800.MPLS`
- video track labels
- audio track labels, codecs, languages, channel layouts, and Atmos or DTS:X descriptors
- subtitle labels and languages

This step is mandatory. Parse failure is fatal for the workflow and the user must provide a valid BDInfo log before continuing.

### 4. Resolve Target Playlist

The backend extracts the playlist name from BDInfo, validates that it can be matched against the scanned source, and directly resolves that playlist into draft track data.

If the BDInfo log does not contain a recognizable playlist name, or if the selected source does not contain the referenced playlist, the workflow stops and the user must correct the source selection or provide a matching BDInfo log.

### 5. Edit Track Export Draft

The track-editing screen presents:

- the video track summary, including codec, resolution, HDR, and Dolby Vision indicators
- selectable audio tracks
- selectable subtitle tracks

For audio and subtitle tracks, the user can:

- choose whether to export the track
- edit track name
- edit language
- set default-track flag
- reorder selected tracks

For the video track, the user can:

- edit the final video track name

Validation rules:

- the export must contain one video track
- at most one selected audio track can be marked default
- at most one selected subtitle track can be marked default
- track order in the UI becomes the `mkvmerge` output order

### 6. Generate Output Name

The UI shows a generated filename and lets the user edit it before job submission.

Filename template:

`{title} - {resolution}.BluRay.{hdr}.{videoCodec}.{defaultAudioCodec}.mkv`

Generation rules:

- `title`
  - user override first
  - then parsed BDInfo title if available
  - then cleaned source file or directory name
- `resolution`
  - for example `2160p` or `1080p`
- `hdr`
  - `HDR` for HDR10-only
  - `HDR.DV` when Dolby Vision is present in the final output
  - omitted entirely when neither applies
- `videoCodec`
  - for example `HEVC` or `AVC`
- `defaultAudioCodec`
  - derived from the chosen default audio track
  - prefer expressive labels such as `TrueHD.7.1.Atmos` or `DTS-HD.MA.5.1`

The final generated filename is sanitized to remove illegal filesystem characters and redundant punctuation.

### 7. Review and Submit

The review page shows:

- source
- selected playlist
- final track list and order
- output path
- final filename
- whether Dolby Vision merge is enabled

After confirmation, the user creates a background job.

### 8. Queue and Execution

Jobs enter a persistent sequential queue:

- only one job runs at a time
- later jobs remain pending
- history stays visible after completion or failure

## Source Discovery and Parsing Details

### Source Scan

The scan logic should traverse the configured input directory conservatively and recognize:

- `.iso` files
- directories that contain Blu-ray structure markers

The initial version should keep traversal shallow and predictable, favoring direct children or a small bounded depth rather than walking arbitrarily deep trees.

### Playlist Resolution

Primary source of truth for playlist selection is the user-provided BDInfo log, with the selected Blu-ray source used to validate that the referenced playlist exists.

BDInfo is used for:

- target playlist selection
- human-friendly track names
- Dolby Vision enhancement-layer detection cues

Execution still depends on validating the selected source and applying the extracted playlist to the remux command arguments.

### Track Metadata Resolution

The final draft merges data from several sources:

- required human-readable labels and playlist identity from BDInfo
- technical verification from `ffprobe` and `mediainfo`

Priority guidance:

- playlist identity and human-facing track names come from BDInfo
- naming fields used for output filename generation may fall back to `ffprobe` or `mediainfo`

## Dolby Vision Handling

For UHD Blu-ray sources, when a separate Dolby Vision enhancement track exists and the installed `mkvmerge` supports merging it into the main HEVC video track:

- the backend enables DV merge in the generated command
- the UI clearly marks that Dolby Vision will be preserved
- the output naming logic uses `HDR.DV`

If DV merge is unavailable because the source lacks a separate enhancement track or the parsed metadata does not indicate Dolby Vision, the output falls back to normal HDR logic.

This behavior should be explicit in the review page so the user can verify it before running the job.

## Data Model

### SourceEntry

Represents one scanned Blu-ray source.

Fields:

- `id`
- `name`
- `path`
- `type`
- `size`
- `modifiedAt`

### PlaylistInfo

Represents one available playlist under a source.

Fields:

- `name`
- `duration`
- `size`
- `chapters`
- `videoSummary`
- `featureScore`
- `isFeatureCandidate`

### TrackInfo

Represents one resolved exportable track.

Fields:

- `id`
- `kind` (`video`, `audio`, `subtitle`)
- `codec`
- `language`
- `name`
- `selected`
- `default`
- `order`

Video-specific fields:

- `resolution`
- `hdrType`
- `dolbyVision`

Audio-specific derived fields can include:

- `channels`
- `immersiveExtension`

### RemuxDraft

Represents the current editable export configuration before submission.

Fields:

- selected source snapshot
- selected playlist
- parsed or inferred title
- video track draft
- audio track drafts
- subtitle track drafts
- generated filename
- user-edited filename
- Dolby Vision merge enabled flag

### Job

Represents one queued or finished remux request.

Fields:

- `id`
- `status`
- `createdAt`
- `startedAt`
- `finishedAt`
- `outputPath`
- `logPath`
- serialized `draftSnapshot`
- failure summary, if any

## Job State Model

Statuses:

- `pending`
- `running`
- `completed`
- `failed`
- `interrupted`

Restart behavior:

- jobs that were `running` at startup recovery time become `interrupted`
- pending jobs remain visible but are not auto-resumed
- the queue does not restart automatically after a container restart

This avoids accidental remux execution after a crash or maintenance restart.

## Persistence Model

Use SQLite for application state:

- session records
- job metadata
- source snapshots if caching is useful
- saved draft snapshots associated with jobs

Use separate log files for job execution logs:

- one log file per job
- database stores the log path

Suggested data layout:

- `/app/data/app.db`
- `/app/data/logs/<job-id>.log`

SQLite is preferred over raw JSON state files because it provides safer updates, easier list queries, and a cleaner path for future enhancements.

## Frontend Information Architecture

The frontend should be a guided workflow rather than a general-purpose dashboard.

Primary views:

- login page
- scan page
- BDInfo paste page
- track editing page
- review and submit page
- jobs page

Key interaction requirements:

- scan results shown in a clear list or table
- large BDInfo paste text area
- editable track rows for audio and subtitle tracks
- order controls for selected audio and subtitle tracks
- live filename preview as draft fields change
- read-only execution summary before submit
- job list with statuses and log viewing

The UI should make it obvious that BDInfo is required and that the pasted log must match the selected source.

## Backend API Shape

Planned HTTP endpoints:

- `POST /api/login`
- `POST /api/logout`
- `GET /api/config`
- `POST /api/sources/scan`
- `GET /api/sources`
- `POST /api/bdinfo/parse`
- `POST /api/sources/:id/resolve`
- `POST /api/drafts/preview-filename`
- `POST /api/jobs`
- `GET /api/jobs`
- `GET /api/jobs/:id`
- `GET /api/jobs/:id/log`

Responsibilities:

- source scan endpoints return discovered media candidates
- BDInfo parse endpoint returns extracted metadata and fails when the log cannot identify a playlist
- resolve endpoint combines source validation with required BDInfo hints into a concrete editable draft
- preview filename endpoint applies naming rules without creating a job
- jobs endpoints manage queue creation, listing, status retrieval, and log access

## Remux Execution Strategy

When a job starts:

1. Validate referenced source and output directories still exist
2. Reconstruct the immutable job draft snapshot
3. Build the `mkvmerge` command with:
   - track selection
   - track order
   - track names
   - languages
   - default flags
   - output file path
   - Dolby Vision merge options when applicable
4. Execute the command
5. Stream stdout and stderr into the job log
6. Update job status on success or failure

The system should preserve the exact command arguments in logs or structured debug records, with sensitive data redacted where applicable.

## Error Handling

The system should treat these as first-class failure modes:

- input directory missing or unreadable
- output directory missing or unwritable
- data directory missing or unwritable
- `APP_PASSWORD` missing
- required CLI tools not installed or not executable
- source scan succeeds but the BDInfo playlist does not exist in the selected source
- BDInfo parse returns incomplete or unrecognized data
- user selects no audio or subtitle tracks that they later expected
- output filename resolves to an existing file collision
- remux command exits non-zero

User-facing behavior:

- configuration and toolchain failures should be visible early in the UI
- BDInfo parse problems should stop the workflow with a clear correction path
- remux failures should preserve logs and show concise summaries in the jobs page

## Testing Strategy

### Backend Unit Tests

- source scan recognition for `.iso` and valid `BDMV` directories
- BDInfo text parsing for playlists, audio, and subtitle names
- filename generation and sanitization
- draft validation rules
- job state transitions
- `mkvmerge` argument generation

### Frontend Tests

- login form behavior
- scan result selection flow
- BDInfo submission and validation behavior
- track selection, default flags, and ordering
- live filename preview updates
- review page rendering

### Integration Tests

- API-level tests using sample metadata fixtures
- queue execution tests with mocked external tool invocations
- restart recovery tests that mark running jobs as interrupted

Real Blu-ray content should not be required for automated tests. Use fixture outputs and mocked command execution for CI.

## Docker Packaging

The repository should produce:

- a multi-stage `Dockerfile`
- local scripts for building and running the image

Dockerfile stages:

1. frontend build stage
2. Go build stage
3. runtime stage containing:
   - Go server binary
   - built frontend assets
   - `mkvmerge`
   - `ffprobe`
   - `mediainfo`

The application listens on one HTTP port, for example `8080`.

Recommended runtime mounts:

- Blu-ray input directory to `/bd_input`
- remux output directory to `/remux`
- app data volume to `/app/data`

## Local Developer Scripts

Provide scripts for:

- local Docker build
- local Docker run

The run script should simplify:

- environment variable injection
- mount setup for input, output, and data directories
- optional image tag selection

## GitHub Actions Release Workflow

Provide a manually triggered workflow using `workflow_dispatch`.

Workflow responsibilities:

- checkout code
- log in to Docker Hub
- build with `docker buildx`
- push configured tags

Manual inputs should include:

- target image tag
- whether to also push `latest`
- optional repository override if needed

Important release constraint:

- the public CI workflow should build the application image with the free CLI toolchain required by the final remux flow
- documentation must explain that BDInfo is required input and must match the selected source

## Documentation Requirements

The implementation should include:

- README with feature overview
- environment variable documentation
- local build and run instructions
- notes on required mounts
- explanation of public image versus locally built full image
- GitHub Actions secret requirements for Docker Hub publishing

## Open Implementation Decisions Already Resolved

- Technology stack: Go backend + React frontend
- Runtime model: single container on Linux
- ISO handling: container does not perform loop mount
- Queue model: persistent sequential queue, one running job at a time
- Playlist resolution: mandatory BDInfo input, no manual playlist fallback
- Title source priority: user edit, then BDInfo, then cleaned source name
- BDInfo requirement: mandatory, no manual playlist selection fallback
- User model: single user with password-protected login
- Session model: login once, persisted by cookie
- Restart behavior: running jobs become interrupted, no automatic resume

## Implementation Readiness

This design is intentionally scoped for a first complete release that is practical to build and test without overengineering. It keeps the system centered on reliable orchestration of existing media tools, a clear guided browser workflow, and conservative persistent job handling.
