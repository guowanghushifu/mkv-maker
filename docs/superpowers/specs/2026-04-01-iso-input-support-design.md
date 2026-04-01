# ISO Input Support Design

## Goal

Add Linux-only Blu-ray ISO input support to the existing single-remux workflow so the application can discover `.iso` files under `/bd_input`, automatically mount them on demand under `/bd_input/iso_auto_mount`, remux from the mounted BDMV structure, and clean up stale mounts safely without interrupting later work.

## Scope

This design covers:

- scanning `.iso` files as first-class input sources
- adding an environment variable to enable or disable ISO source scanning
- reserving `/bd_input/iso_auto_mount` as application-owned mount workspace
- mounting ISO sources automatically during resolve and remux submission
- preserving the existing BDMV workflow for extracted folders
- keeping a one-hour idle timeout for mounted ISO sources
- adding a manual "Release Mounted ISOs" action on the scan page
- cleaning residual mounts on startup
- cleaning mounted ISO workspaces on application shutdown
- best-effort unmount behavior that never blocks future work

This design does not cover:

- non-Linux hosts
- rootless Docker
- Windows or macOS mount support
- concurrent remux execution
- persistent mount metadata in SQLite or on disk
- automatic cleanup triggered by Back to Scan, Start Next Remux, or source switching

## Runtime Constraints

- The host runtime is Linux.
- The service runs in Docker with enough privileges to execute `mount -o loop,ro` and `umount`.
- The container runs as `root`.
- The container is expected to have `SYS_ADMIN`, loop devices, and relaxed seccomp/AppArmor rules as needed for mount syscalls.
- The image must include `mount` and `umount` from `util-linux`.
- ISO mounts are read-only.
- `BD_INPUT_DIR` remains `/bd_input` by default.
- `/bd_input/iso_auto_mount` is reserved for this application only and must not be used for user-managed inputs.
- Add `ENABLE_ISO_SCAN` with default enabled.
- `ENABLE_ISO_SCAN` only controls whether the scanner returns ISO sources.
- If `ENABLE_ISO_SCAN=0`, extracted BDMV scanning still works normally and ISO-specific entry points become unreachable through the UI.

## Product Behavior

### Source Discovery

The scan flow must return two source types:

- `bdmv`
- `iso`

`bdmv` keeps the current meaning: an extracted Blu-ray directory containing a valid `BDMV` structure.

`iso` represents a `.iso` file directly under the input tree. The scanner should walk the input directory, discover ISO files conservatively, and skip the reserved `/bd_input/iso_auto_mount` subtree entirely so the application never re-discovers its own mounted workspaces as user inputs.

ISO scanning is gated by `ENABLE_ISO_SCAN`:

- default: enabled
- disabled: the scanner does not return `iso` sources at all

This flag only affects source discovery. It does not change resolve, remux, or cleanup behavior because those flows remain inaccessible unless an ISO source is first returned by scan.

### ISO Source Representation

Each ISO scan result should include:

- stable `id`
- human-readable `name`
- absolute `path`
- `type = "iso"`
- file `size`
- file `modifiedAt`

The `id` must not rely only on the basename. It should be derived from a normalized relative path under the input root so that two ISO files with the same filename in different folders do not collide.

The `name` should default to the ISO filename without the `.iso` extension.

### Mount Workspace

All automatic mounts live under:

- `/bd_input/iso_auto_mount`

Each ISO gets its own deterministic mount directory:

- `/bd_input/iso_auto_mount/{safe-id}`

`safe-id` must be filesystem-safe, deterministic, and collision-resistant for the scanned ISO source id.

The scanner must never return anything below `/bd_input/iso_auto_mount` as a user-selectable source.

## Workflow Changes

### Scan Step

The scan page continues to list available sources, now including ISO files. The toolbar adds a new button to the left of `Scan Sources`:

- Chinese: `释放已挂载 ISO`
- English: `Release Mounted ISOs`

This button manually releases every mounted ISO workspace that is not in use by a running remux task.

### BDInfo Resolve Step

The application still requires the user to paste BDInfo before advancing.

When the user clicks the BDInfo continue button:

- if the selected source is `bdmv`, the current resolve flow remains unchanged
- if the selected source is `iso`, the backend first ensures that the ISO is mounted under `/bd_input/iso_auto_mount/{safe-id}`
- once mounted, the backend validates that the mount contains a valid Blu-ray `BDMV` structure
- the existing playlist resolution and track inspection logic then runs against that mounted directory

The user does not manually mount anything and does not need to rescan after the ISO is mounted.

### Remux Submission

When the user submits a remux job:

- if the source is `bdmv`, keep current behavior
- if the source is `iso`, the backend calls `EnsureMounted` again before building the execution draft
- if the ISO was previously auto-unmounted by the idle janitor, this remount is automatic
- the mounted ISO is marked `in use` for the lifetime of the running remux

The remux command still runs against a resolved BDMV playlist path. ISO support is an input-preparation concern, not a new remux execution mode.

### After Remux Completion

When a remux task finishes, whether succeeded or failed:

- mark the ISO mount idle
- immediately attempt best-effort cleanup for that ISO mount
- cleanup means `umount` first, then removing `/bd_input/iso_auto_mount/{safe-id}`

Cleanup failures must never change the remux result. A succeeded remux remains succeeded even if cleanup fails.

## Mount Lifecycle

### Why Cleanup Is Not Driven by Navigation

The current frontend navigation actions do not reliably communicate user intent to release a specific ISO mount. Adding release behavior to Back to Scan, Start Next Remux, or source switching would require extra UI-to-API orchestration for every path and would still be brittle.

This design deliberately avoids navigation-coupled cleanup and instead uses:

- explicit manual release on the scan page
- one-hour idle timeout
- cleanup after remux completion
- startup cleanup
- shutdown cleanup

### Idle Timeout

Mounted ISO workspaces that are not in use by a running remux task are eligible for automatic cleanup after one hour of inactivity.

Inactivity is measured from the last server-side touch time for that mounted ISO. A touch occurs when:

- the ISO is mounted or reused during resolve
- the ISO is mounted or reused during remux submission
- the ISO is explicitly kept active through reuse of the same source

The idle janitor must not clean any ISO currently marked `in use`.

### Manual Release

The scan-page release button triggers a backend batch cleanup of all non-running mounted ISO workspaces.

This operation is best-effort:

- in-use mounts are skipped
- unmount failures are logged and counted
- directory removal failures are logged and counted
- the overall request still succeeds and returns cleanup statistics

### Startup Cleanup

On application startup, before the server begins handling requests, the application scans `/bd_input/iso_auto_mount` for residual managed workspaces.

Because `/bd_input/iso_auto_mount` is reserved exclusively for this application, the cleanup logic may treat its child directories as application-owned candidates. A child directory containing a valid `BDMV` structure is definitive evidence of a previous managed mount and must be cleaned.

For each candidate workspace:

- attempt `umount`
- attempt to remove the workspace directory
- log failures
- continue with the next workspace

Startup must continue even if some cleanup operations fail.

### Shutdown Cleanup

On application shutdown, attempt best-effort cleanup of all tracked ISO mount workspaces.

This includes:

- mounts currently tracked in memory
- janitor shutdown
- unmounting remaining workspaces
- removing their directories

Shutdown must never hang forever waiting on cleanup. Failures are logged and ignored after best effort.

## Backend Architecture

### New ISO Auto-Mount Manager

Add a dedicated backend component responsible only for ISO mount lifecycle. It should not own BDInfo parsing, playlist inspection, or remux command assembly.

Responsibilities:

- create mount directories under `/bd_input/iso_auto_mount`
- mount ISO files read-only with loopback
- validate the mounted BDMV structure
- track mount state in memory
- remount on demand after idle cleanup
- release idle mounts manually
- clean expired mounts
- clean residual mounts on startup
- clean tracked mounts on shutdown

### Tracked State

The in-memory state per ISO source should include:

- source id
- ISO path
- mount path
- last touched timestamp
- `inUse` flag

This state does not need durable persistence because:

- the service runs a single-process, single-remux manager
- startup residual cleanup handles restart leftovers
- a new mount can always be reconstructed from the ISO source path

### Core Operations

The manager should expose these operations:

- `EnsureMounted(source)`  
  Returns a mounted BDMV root path for the given ISO source. Reuses a healthy existing mount when possible. Otherwise creates the mount directory, runs `mount -o loop,ro`, validates the BDMV structure, updates tracked state, and returns the mounted source root.

- `Touch(sourceID)`  
  Refreshes idle timeout state for an already tracked mount.

- `MarkInUse(sourceID)`  
  Marks a tracked ISO as protected from idle and manual cleanup while a remux task is running.

- `MarkIdle(sourceID)`  
  Clears the in-use flag after task completion.

- `ReleaseIdleMounted()`  
  Batch-releases all tracked or residual ISO mounts that are not currently in use.

- `CleanupExpiredIdle(now)`  
  Releases every idle mount older than the one-hour timeout.

- `CleanupResidualMountDirs()`  
  Best-effort startup cleanup for `/bd_input/iso_auto_mount`.

- `CleanupAll()`  
  Best-effort shutdown cleanup for all tracked mounts.

## API Changes

### Source Scan Response

Extend the existing source response contract so the frontend receives both `bdmv` and `iso`.

When `ENABLE_ISO_SCAN` is disabled, the scan response returns only `bdmv` sources.

Frontend types must change from:

- `type SourceType = 'bdmv'`

to:

- `type SourceType = 'bdmv' | 'iso'`

The scan UI should render a specific badge for ISO sources, for example:

- Chinese: `ISO 文件`
- English: `ISO File`

### Resolve Endpoint

The current resolve endpoint must accept either source type:

- `bdmv` resolves directly
- `iso` resolves through `EnsureMounted`

The response stays draft-compatible with the current editor page. The mount path remains an internal backend concern and should not require a separate frontend step.

### Manual Release Endpoint

Add a new authenticated endpoint:

- `POST /api/iso/release-mounted`

Response shape:

```json
{
  "released": 2,
  "skippedInUse": 1,
  "failed": 0
}
```

The endpoint always performs best-effort cleanup and returns a result summary. A failed unmount for one ISO must not prevent the endpoint from attempting the rest.

## Remux Integration

### Job Validation

Job creation must stop rejecting non-BDMV input sources. Instead:

- `bdmv` jobs keep current validation
- `iso` jobs validate the ISO path under the input root, remount if needed, and then resolve the playlist path from the mounted BDMV directory

### In-Use Protection

The remux manager currently permits only one running task at a time. That simplifies ISO protection:

- when an ISO-backed job starts, mark that ISO `in use`
- when the job ends, mark it idle and attempt immediate cleanup
- manual release and janitor skip any `in use` ISO

This ensures the scan-page cleanup button and idle janitor cannot unmount a source while `mkvmerge` is actively reading from it.

## Failure Handling

### Mount Failure

Mount failure is fatal for the request that needs the source:

- resolve fails if the ISO cannot be mounted
- job creation fails if the ISO cannot be mounted

Typical causes include:

- missing loop device
- insufficient container privileges
- corrupt ISO
- mount path creation failure
- mounted content not containing a valid BDMV structure

These failures should return a clear API error message and leave no partially trusted mounted state in memory.

### Unmount Failure

Unmount failure is never fatal for future work.

Rules:

- log the failure
- try to continue cleanup of other ISO workspaces
- do not fail later scans
- do not block later resolve or remux requests
- do not convert a completed remux into a failed remux

If a later `EnsureMounted` sees an existing still-mounted workspace for the same ISO and it validates correctly, reuse it instead of treating the earlier unmount failure as terminal.

### Directory Removal Failure

Directory removal failure also uses best-effort semantics:

- log it
- continue processing
- allow future cleanup attempts to remove it later

## Security and Safety Boundaries

- ISO file paths must remain under `BD_INPUT_DIR`
- mount paths must remain under `/bd_input/iso_auto_mount`
- the backend must never mount an arbitrary path provided by raw user input
- source selection continues to originate only from scanner results
- the reserved auto-mount root is skipped by scanning to prevent self-recursion
- ISO mounts are read-only
- mount directory names must be sanitized and deterministic

## Testing Strategy

### Scanner Tests

Add tests proving that:

- `.iso` files are returned as `iso` sources
- extracted BDMV directories are still returned as `bdmv`
- `/bd_input/iso_auto_mount` is skipped entirely
- duplicate basenames in different folders produce stable non-colliding ids

### Mount Manager Tests

Use a fake command runner or command executor abstraction so tests do not require real mount privileges.

Cover:

- initial mount success
- mount reuse when already mounted
- remount after prior idle cleanup
- in-use protection
- idle timeout cleanup
- manual batch release
- startup residual cleanup
- shutdown cleanup
- best-effort handling when unmount fails

### Handler Tests

Cover:

- resolve for `iso` source mounts and continues successfully
- resolve fails cleanly on mount failure
- job creation for `iso` source remounts as needed
- manual release endpoint returns cleanup summary
- in-use ISO mounts are skipped by manual release

### Frontend Tests

Cover:

- scan page renders ISO badge
- scan page shows the new release button left of `Scan Sources`
- release button triggers the new API and tolerates partial-failure summaries
- workflow continues normally for `iso` sources through scan, BDInfo, editor, and review

## Rollout Notes

- README and Docker documentation must be updated to describe Linux-only ISO support and required container privileges
- README and Docker documentation must describe `ENABLE_ISO_SCAN`, default-on behavior, and the recommendation to disable it in containers that do not have mount privileges
- `/bd_input/iso_auto_mount` must be documented as application-reserved
- operators should understand that ISO mount cleanup is best-effort and stale mounts can be retried by startup cleanup, manual release, or the idle janitor
