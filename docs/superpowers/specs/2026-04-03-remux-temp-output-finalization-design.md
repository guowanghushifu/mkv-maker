# Remux Temporary Output Finalization Design

## Goal

Change remux execution so `mkvmerge` writes to a temporary output file first and only exposes the final `.mkv` path after the command succeeds.

For a target such as `/remux/布达佩斯大饭店 (2014) - 2160p.BluRay.DV.HDR.HEVC.DTS-HD.MA.5.1.mkv`, the remux process should:

- write to `/remux/布达佩斯大饭店 (2014) - 2160p.BluRay.DV.HDR.HEVC.DTS-HD.MA.5.1.mkv.tmp`
- rename that file to the final `.mkv` path only after `mkvmerge` exits successfully
- delete the `.tmp` file if the remux is canceled or fails

## Product Decisions

### User-Facing Path Stays Stable

The task model, review page, and API responses should continue to treat the configured `.mkv` path as the output path.

The user asked to change execution behavior, not the visible job destination. The UI should therefore keep showing the final path instead of the temporary `.tmp` path.

### Temporary Output Naming

The temporary file name is derived mechanically from the final target path:

- temporary path = `final path + ".tmp"`

Examples:

- `/remux/Movie.mkv` -> `/remux/Movie.mkv.tmp`
- `/remux/Movie` -> `/remux/Movie.tmp`

No timestamp, PID, or random suffix is needed because the application already guarantees only one remux task runs at a time.

### Cleanup Policy

Any non-successful execution should remove the temporary file.

That includes:

- user-triggered emergency stop
- app shutdown while a remux is running
- `mkvmerge` process failure
- post-process rename failure after `mkvmerge` succeeds

The user explicitly chose not to keep failed `.tmp` files for inspection.

### Command Preview

The command preview shown in the review page should reflect the real command sent to `mkvmerge`, which means its `--output` value should be the temporary `.tmp` path.

That keeps the preview accurate and makes it obvious that the muxer is not writing directly to the final destination anymore.

## Backend Design

### Execution Ownership

Temporary-file lifecycle should live in `internal/remux/job_runner.go`, not in the manager or HTTP layer.

Rationale:

- `JobRunner` already owns command construction and execution draft preparation
- `Manager` should stay focused on task state, progress, and cancellation
- success finalization and failure cleanup are execution concerns tied directly to the `mkvmerge` invocation

### Execution Draft Handling

The start request still carries the final output path. `buildExecutionDraft()` should continue returning a draft whose `OutputPath` is the final target path.

When preparing the actual command invocation, `JobRunner` should derive a second draft for execution:

- final draft: used as the canonical task destination
- temporary execution draft: identical except `OutputPath` points to `final + ".tmp"`

This keeps the task snapshot stable while allowing the real `mkvmerge` command to write to the temporary file.

### Temporary File Lifecycle

The execution sequence should be:

1. Build the canonical draft from the request.
2. Derive the temporary output path from `draft.OutputPath`.
3. Best-effort remove any stale file already present at that temporary path.
4. Run `mkvmerge` with the temporary execution draft.
5. If the command succeeds, rename the temporary path to the final path.
6. If any step after command start does not end in success, delete the temporary file.

Cleanup should be best-effort and should not overwrite the primary command error when both execution and cleanup fail. The task failure message should still describe the main remux failure or cancellation outcome.

### Finalization Semantics

The rename from `.tmp` to the final target should happen only after `cmd.Wait()` returns success.

If the rename fails:

- the task becomes `failed`
- the failure message should surface the rename error
- the `.tmp` file should then be deleted

The implementation should use `os.Rename()` so a same-filesystem move remains atomic.

### Cancellation And Failure Handling

The existing cancellation path already flows through `Manager.finishExecution()`. This should remain true.

No new task status is needed. Canceled tasks should continue to end as:

- status: `failed`
- message: `remux canceled`

The only added behavior is that the temporary output file must be deleted before the task reaches its terminal state.

### Stale Temporary Files Before Start

Because failed and canceled tasks are now supposed to clean up, stale `.tmp` files should be rare. Even so, the runner should remove a pre-existing temporary file before launching `mkvmerge`.

This avoids two bad outcomes:

- `mkvmerge` appending to or conflicting with a leftover partial file
- a previous crash leaving a stale temp artifact that breaks the next run

This pre-start removal should target only the derived temporary path, never the final configured output path.

## Testing Strategy

Backend tests should cover:

- command preview uses the `.tmp` output path while the task still reports the final `.mkv` path
- successful execution renames the `.tmp` file to the final path
- failed execution deletes the `.tmp` file
- canceled execution deletes the `.tmp` file
- a stale pre-existing `.tmp` file is removed before a new run starts
- rename failures surface an execution error and do not leave a `.tmp` behind

Most of this can be covered in `internal/remux/job_runner_test.go` with a file-writing stub runner plus a few manager-level cancellation assertions in `internal/remux/manager_test.go`.

## Non-Goals

This change does not add:

- a visible temporary-path field in the API or UI
- support for keeping failed partial mux outputs
- multiple concurrent remux tasks with unique temp naming
- cross-filesystem copy-and-replace fallback for rename failures
