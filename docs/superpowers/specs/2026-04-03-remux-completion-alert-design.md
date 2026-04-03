# Remux Completion Alert Design

## Goal

Notify the user when a remux finishes successfully without requiring them to keep watching the review page.

On `succeeded`, the browser should:

- play a short completion chime
- show a system notification when the browser supports it and permission is granted

The reminder should be best-effort and must never interfere with the remux workflow itself.

## Product Decisions

### Success-Only Trigger

Alerts only fire for `succeeded`.

They must not fire for:

- `failed`
- user-initiated stop flows that surface as `failed`
- historical completed jobs loaded when the page first opens or refreshes

### Single-Fire Semantics

Each remux job should alert at most once per browser tab.

The frontend should track whether the current tab is eligible to alert for a given job. A job becomes eligible when the user starts that remux from the current page. This avoids false positives for historical jobs restored from backend state.

The alert should fire when the same eligible job transitions from `running` to `succeeded`.

The frontend does not need extra logic for the extreme case where a newly started remux is already `succeeded` in the first post-submit snapshot. This feature is only required for the normal path where the review page observes the task running before it later succeeds.

### Best-Effort Browser Notification

Browser notification support should be additive, not required.

Rules:

- if `Notification` is unsupported, skip it
- if notification permission is `denied`, skip it
- if the page is not in a context where notification permission can be used, skip it
- if notification creation throws, swallow the error and keep the UI unchanged

The chime remains the primary reminder because it does not depend on system notification permission.

### Permission Request Timing

Do not request notification permission on page load.

Instead, when the user clicks `Start Remux`, the frontend may prepare alert capabilities during that user gesture:

- warm or resume the audio context for later playback
- request notification permission if the browser supports it and permission is still `default`

If permission is not granted, remux submission still proceeds normally.

## Frontend Design

### Alert Helper

Add a small browser-only helper module in the web frontend that owns completion alerts.

Recommended responsibilities:

- prepare browser capabilities during a user gesture
- play a short synthesized success chime
- show a notification for a completed remux

The helper should not depend on React rendering. It should expose small async functions that can be called from the workflow hook.

### Chime Implementation

Use the browser `Web Audio API` instead of a bundled audio file.

Reasons:

- no new static asset pipeline work
- no extra request at completion time
- easy to keep the sound short and unobtrusive

Implementation guidance:

- reuse a single `AudioContext` when possible
- synthesize a short two-note or rising-tone chime around 0.5 to 1 second total
- if audio playback fails because the browser blocks it, fail silently

### Notification Content

The notification should be concise and derived from data already present in the review workflow.

Recommended content:

- title: `Remux completed`
- body: the output filename, optionally with playlist or source context if the filename alone is ambiguous

If supported, clicking the notification should focus the existing browser tab.

### Workflow Hook Integration

`web/src/useRemuxWorkflow.ts` remains the correct integration point because it already owns:

- remux submission
- current job polling
- current job status transitions

Add minimal alert state to the hook, for example:

- the job id currently armed for completion alerts
- the last job id already alerted in this tab
- the previous seen status for the current job id when useful for transition checks

Expected behavior:

1. When `Start Remux` is clicked, prepare the alert helper during the same user gesture.
2. When submit succeeds, arm alerts for the returned job id.
3. While polling updates `currentJob`, detect whether the armed job has now succeeded.
4. Fire the chime first, then attempt the browser notification.
5. Mark that job id as already alerted so repeated polls do not replay it.

The alert path must not mutate workflow navigation or task status display.

## Testing Strategy

Frontend tests should cover:

- no alert for a historical `succeeded` job loaded on initial render
- one alert when an armed job changes from `running` to `succeeded`
- no repeated alert when polling returns the same `succeeded` job multiple times
- no alert for `failed`
- permission preparation is attempted from the submit path without blocking remux submission

Tests should mock the alert helper instead of trying to verify real audio playback or real desktop notifications.

No backend tests are needed because this feature does not change the API contract.

## Non-Goals

This change does not add:

- a persistent notification preferences UI
- server-side push events or websockets
- browser notifications for failed remux jobs
- custom in-page toast notifications
- a downloadable or user-configurable alert sound
