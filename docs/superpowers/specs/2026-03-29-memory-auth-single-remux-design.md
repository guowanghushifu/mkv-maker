# Memory Auth Single Remux Design

## Goal

Further simplify the application so it no longer depends on SQLite at all:

- remove SQLite-backed persistence
- keep password protection
- keep only one in-process remux task
- keep live task status and log output on the review page

This design supersedes the persistence model from the earlier single-remux design. The track-editor table design remains unchanged.

## Product Decisions

### Authentication

The application keeps password protection, but no longer stores sessions in a database.

Successful login should issue a signed cookie that contains:

- expiry timestamp
- signature

The server validates the cookie signature and expiry on each request. No server-side session lookup is required.

Consequences:

- service restart invalidates all existing login sessions if the signing secret changes
- with a stable signing secret derived from runtime config, existing cookies may remain valid until expiry
- there is no session table and no session cleanup work

For this product, a stable process-local signing secret derived from configured application inputs is acceptable. The system does not need multi-device session management, revocation lists, or session history.

### Remux Task State

The application holds remux task state only in memory.

At runtime it tracks:

- one currently running remux task, or none
- the most recent finished task, if any
- the current or most recent task log text

Consequences:

- service restart clears current and recent task state
- a running remux interrupted by service shutdown is simply lost
- there is no recovery, no persistence, and no post-restart job history

This is explicitly acceptable for the simplified single-user local-tool scope.

## Runtime Architecture

Replace database-backed stores with two in-process components:

1. signed-cookie auth service
2. current remux manager

### Signed-Cookie Auth Service

Responsibilities:

- issue a signed cookie after password validation
- validate cookie signature and expiry
- clear the cookie on logout

This replaces the SQLite session store and removes all database dependencies from auth flow.

### Current Remux Manager

Responsibilities:

- reject a new start request if a remux is already running
- create the in-memory current task record
- run `mkvmerge` in a background goroutine
- append lifecycle and command output logs
- expose current or most recent task state

This replaces the SQLite job store and queue manager.

## API Shape

The simplified API remains:

- `POST /api/jobs`
- `GET /api/jobs/current`
- `GET /api/jobs/current/log`

Semantics:

- `POST /api/jobs` validates payload and immediately starts the in-memory task
- `GET /api/jobs/current` returns the running task, otherwise the most recent finished task
- `GET /api/jobs/current/log` returns the log for that same task

If a task is already running, `POST /api/jobs` returns `409 Conflict`.

If no task has ever been started in the current process lifetime, the current-task endpoints return `404`.

## Task Lifecycle

The runtime task state uses only these statuses:

- `running`
- `succeeded`
- `failed`

The manager should update task state as follows:

1. create running task
2. append `remux started`
3. append command output as it arrives
4. mark `succeeded` and append `completed`, or mark `failed` and append concise failure message

No queued, pending, interrupted, or recovery-specific state remains.

## Frontend Impact

The frontend behavior from the earlier single-remux progress design still stands:

- the review page keeps focus after submit
- the page shows current task status and live log output
- polling continues while status is `running`
- polling stops on `succeeded` or `failed`

The frontend should not assume task persistence across server restart. If the current-task endpoints start returning `404`, the UI should treat that as “no current task in memory”.

## Files to Remove

The following persistence-oriented files should be deleted:

- `internal/store/db.go`
- `internal/store/migrate.go`
- `internal/store/session_store.go`
- `internal/store/session_store_test.go`
- `internal/store/job_store.go`
- `internal/store/job_store_test.go`

Any queue-only files should also be removed as part of the same simplification:

- `internal/queue/manager.go`
- `internal/queue/manager_test.go`
- `internal/queue/executor.go`
- `internal/queue/executor_test.go`

The `modernc.org/sqlite` dependency should be removed from the module once no code depends on it.

## App Wiring Changes

`internal/app/app.go` should no longer:

- open `app.db`
- run migrations
- keep a `*sql.DB`
- close a database handle on shutdown

Instead, app startup should:

- initialize the auth signer
- initialize the in-memory remux manager
- wire handlers directly to those components

Shutdown only needs to stop any in-flight background work if the process is exiting cleanly.

## Testing Strategy

Add or update tests for:

- signed cookie login success and failure
- auth middleware accepting valid signed cookies and rejecting invalid or expired ones
- in-memory remux manager rejecting concurrent starts
- current-task endpoints returning `404` before any task starts
- current-task endpoints returning running and terminal task state correctly
- current-task log endpoint returning live in-memory log output

Delete or replace tests that only exist to verify SQLite migration or store behavior.

## Non-Goals

This simplification does not add:

- persistent history
- restart recovery
- durable logs
- session revocation lists
- multi-user auth state

The product intentionally becomes a simpler, process-local tool with password protection and ephemeral runtime state.
