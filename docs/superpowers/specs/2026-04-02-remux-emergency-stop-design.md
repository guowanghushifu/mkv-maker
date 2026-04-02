# Remux Emergency Stop Design

## Goal

Allow the user to stop the currently running `mkvmerge` task from the final review page when they notice the remux configuration is wrong after clicking `Start Remux`.

The stop control should appear immediately to the right of `Start Remux` and should cancel the in-flight remux without requiring the user to wait for `mkvmerge` to finish naturally.

## Product Decisions

### Review Page Action Layout

The primary action row keeps `Back` first.

The running-task actions in the same row become:

- `Start Remux`
- `Emergency Stop Remux`

`Emergency Stop Remux` only needs to be useful when a task is actually running, so the button should be disabled when there is no running task. While a stop request is in flight, the button should stay disabled to avoid duplicate requests.

The secondary `Start Next Remux` action remains where it is today.

### Cancellation Semantics

Stopping a remux is a task cancellation request, not a graceful finish.

For this change, canceled tasks will reuse the existing terminal status shape:

- status: `failed`
- message: explicit cancellation text such as `Remux canceled.`

This avoids a larger cross-stack status expansion while still letting the UI distinguish user cancellation from generic execution failure through the message text and log output.

### Post-Stop User Flow

After cancellation:

- the user stays on the review page
- the latest task panel stays visible
- the task log shows the cancellation outcome
- the user can click `Back` to correct metadata and start again

The workflow should not automatically reset or navigate away on stop.

## Backend Design

### Manager API

`internal/remux.Manager` should gain a task-scoped stop method for the currently running job.

Recommended shape:

- store a cancel function on the current task state
- add `StopCurrent() error`

`StopCurrent()` should:

- return `ErrTaskNotFound` if nothing is running
- call the running task's cancel function exactly once
- avoid canceling completed historical tasks

The existing manager-wide app shutdown path should remain separate. `Close()` still shuts down the service, while `StopCurrent()` only targets the active remux.

### Execution Context

Today the manager executes remux work with a manager-wide context. That couples app shutdown and task cancellation too tightly.

The running job should instead be created with a child context derived from the manager lifetime context. That child context belongs to the active task and is the one canceled by `StopCurrent()`.

This keeps these behaviors distinct:

- app shutdown cancels everything still running
- emergency stop cancels only the active remux

### HTTP API

Expose a dedicated stop endpoint for the current task in `internal/http/handlers/jobs.go`.

Recommended behavior:

- route: `POST /api/jobs/current/stop`
- `202 Accepted` when a running task was signaled for cancellation
- `404 Not Found` when no task is currently running
- `500 Internal Server Error` for unexpected failures

The stop endpoint does not need a request body.

### ISO Lease Handling

The current task-finished hook already releases ISO resources after terminal completion. Cancellation should flow through the same finish path, so ISO-backed remux jobs still release their lease after the canceled task reaches terminal state.

No separate stop-specific ISO cleanup path is needed.

## Frontend Design

### API Client And Workflow Hook

`web/src/api/client.ts` should add a method for the stop endpoint.

`web/src/useRemuxWorkflow.ts` should add:

- a `stoppingJob` state flag
- a `handleStopCurrentJob()` action

The stop action should:

- no-op if no task is running
- call the API stop method
- refresh current job and log state after the request resolves
- surface a localized error if the request fails

Polling can stay unchanged. Once the canceled task transitions away from `running`, the existing polling effect will naturally stop.

### Review Page Props And Rendering

`web/src/features/review/ReviewPage.tsx` should accept:

- `stoppingJob`
- `onStopCurrentJob`

Rendering rules:

- place the emergency stop button immediately to the right of `Start Remux`
- disable `Start Remux` while `submitting`, `startDisabled`, or `stoppingJob`
- disable `Emergency Stop Remux` when there is no running task or while `stoppingJob`

No new panel is needed. The existing current-task console remains the place where the stop outcome is shown.

### Copy Updates

Add localized labels for:

- `Emergency Stop Remux`
- `Stopping...`
- stop failure message

Keep the existing task status labels unchanged.

## Testing Strategy

Backend tests should cover:

- `StopCurrent()` returns not found when no task is running
- `StopCurrent()` cancels a running task and leaves a terminal task with cancellation message
- the stop HTTP endpoint returns `202` for a running task
- the stop HTTP endpoint returns `404` when nothing is running

Frontend tests should cover:

- review page renders the emergency stop button next to `Start Remux`
- the stop button is disabled when there is no running task
- the workflow hook calls the stop endpoint and refreshes the current task snapshot
- localized stop labels and stop error handling appear correctly

## Non-Goals

This change does not add:

- multiple concurrent remux task cancellation
- resumable remux
- partial output cleanup after cancellation
- a new `canceled` task status enum
- automatic navigation back to the editor after stop
