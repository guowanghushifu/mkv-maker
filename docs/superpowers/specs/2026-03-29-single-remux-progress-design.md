# Single Remux Progress Design

## Goal

Simplify the current product from a queued remux tool into a single-task remux tool:

- only one Blu-ray remux can exist at a time
- queue semantics and queue UI are removed
- after the user confirms the remux, the review page stays visible and shows the current task status plus live log output

This change keeps the existing scan, BDInfo, draft editing, and filename workflow intact.

## Product Decision

The product no longer models multiple pending jobs.

Instead, it models one current remux task with these states:

- `running`
- `succeeded`
- `failed`

Optionally, the backend may keep the most recent finished task record so the review page can still show the last result after refresh, but there is no history list and no list endpoint in the UI.

## Recommended Backend Shape

Replace queue-driven execution with a single-task runner:

- `POST /api/jobs` starts a remux task immediately in the background
- `GET /api/jobs/current` returns the current or most recent task status
- `GET /api/jobs/current/log` returns the current or most recent task log text

If a task is already `running`, `POST /api/jobs` returns `409 Conflict`.

This preserves the current task payload validation, persistence, and log storage model while removing queue management, pending state, and task ordering behavior.

## Persistence Model

Keep the existing SQLite-backed task record storage, but simplify semantics:

- there is no queued state
- there is no claim-next operation
- there is no worker loop polling the database
- task creation writes a single record and the server immediately launches execution for that record

The database may still use the existing `jobs` table for minimal migration risk. In that case:

- new records should start in `running`
- completed records become `succeeded` or `failed`
- any startup recovery logic should turn stale `running` records into `failed` with a clear message that the process ended before completion

This is an implementation detail. The API and UI should treat it as a single remux task, not as a queue.

## Execution Flow

When the user clicks the confirm button on the review page:

1. The frontend sends the validated payload to `POST /api/jobs`
2. The backend validates source path, playlist, output path, and payload structure
3. The backend creates the task record and log file
4. The backend starts a goroutine that runs `mkvmerge`
5. The backend appends lifecycle lines and command output to the task log as output arrives
6. The backend updates task status to `succeeded` or `failed`

The HTTP request should return as soon as task startup is accepted. It must not block until remux completion.

## Progress Model

The product does not need percentage progress.

Progress is represented by:

- a status badge for the current stage
- live log text streaming through polling

The minimum stage vocabulary is:

- `running`
- `succeeded`
- `failed`

The log should include lifecycle markers such as:

- remux started
- command output
- remux completed
- remux failed with concise error message

## Frontend Changes

Remove the dedicated jobs/history page and keep the user on the review page after submission.

The review page should gain a current-task panel with:

- output filename and output path
- current status badge
- failure message when applicable
- a log viewer that refreshes while the task is running

Interaction changes:

- the submit button label changes from queue-oriented wording to direct execution wording such as `Start Remux`
- after successful submission, the review form remains visible in a read-only or effectively read-only state while the current task panel is shown beneath it
- while a task is running, the frontend polls `GET /api/jobs/current` and `GET /api/jobs/current/log`
- polling stops once the task reaches `succeeded` or `failed`

The workflow should still allow the user to start a new remux after the current one finishes. It must not allow starting another while one is already running.

## Error Handling

Backend errors:

- invalid payload remains `400`
- source or playlist validation failures remain `400`
- task already running returns `409`
- log retrieval for no known task returns `404`

Frontend behavior:

- if submission returns `409`, show a clear message that a remux is already in progress
- if polling fails temporarily, keep the last known task state visible and surface a concise refresh error
- if the task fails, show both the failure badge and the live log so the user can inspect the command output

## Deletions

The following product concepts should be removed:

- queue manager loop
- queue package logic whose only purpose is polling or claiming queued work
- queued and pending UI wording
- jobs list page
- manual refresh flow for a history page

If some internal types remain temporarily for migration safety, they should not leak queue language into the UI.

## Testing Strategy

Add or update tests for:

- `POST /api/jobs` starts a task immediately and returns the created running task
- `POST /api/jobs` rejects a second start attempt while another task is running
- current-task status endpoint returns `running`, then terminal state after execution
- current-task log endpoint returns incremental output
- startup recovery no longer leaves orphaned running tasks pretending to be active
- review page shows current status and log output after starting remux
- review page stops polling once the task reaches a terminal state

## Migration Notes

This should be implemented as a simplification, not a broad rewrite:

- preserve the existing draft payload shape
- preserve current path-validation behavior
- preserve log-file writing
- prefer adapting the current job store over inventing a second persistence model

The main change is replacing queue orchestration with direct single-task orchestration and moving task visibility into the review page.
