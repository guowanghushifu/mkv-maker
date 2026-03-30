# Review Command And Progress Design

## Goal

Extend the current single-remux review page so the user can see:

- the final `mkvmerge` command that will be executed
- the current remux progress as a percentage-based progress bar with a numeric percentage

This builds on the existing in-memory single-task design. It does not reintroduce queueing, persistence, or history pages.

## Product Decisions

### Command Display

The review page should show the exact `mkvmerge` command corresponding to the current task.

Display format:

- multi-line
- optimized for readability rather than shell copy-paste
- first line shows `mkvmerge`
- subsequent lines show one argument per line

The purpose is inspection and confirmation, not terminal execution.

### Progress Display

The review page should show:

- a horizontal progress bar
- a numeric percentage such as `42%`

Progress values range from `0` to `100`.

State behavior:

- `running`: show latest parsed percentage
- `succeeded`: show `100%`
- `failed`: keep the last known percentage

If no parseable percentage has been seen yet, the progress bar should remain at `0%`.

## Recommended Backend Shape

The in-memory remux task state should be extended with two additional fields:

- `commandPreview`
- `progressPercent`

### commandPreview

This is a formatted, multi-line representation of the final command derived from the current remux draft.

It should be computed when the task starts, before `mkvmerge` is executed, using the same underlying argument construction used for real execution.

### progressPercent

This is the latest known integer percentage parsed from `mkvmerge` output.

The manager should update this field whenever command output includes a parseable progress value.

The manager should not invent progress when output does not provide one.

## Progress Parsing Rules

The parser should only react to explicit percentage information in command output.

Rules:

- parse numeric percentages from `mkvmerge` output when clearly present
- clamp values into `0..100`
- ignore non-parseable log lines
- do not estimate progress from elapsed time, file size, or line counts

This keeps the progress bar honest and avoids misleading pseudo-progress.

## API Impact

`GET /api/jobs/current` should return the current task enriched with:

- `commandPreview`
- `progressPercent`

`GET /api/jobs/current/log` remains unchanged and still returns the full log text.

The frontend should not parse progress from the log on its own if structured progress is already available from the current-task endpoint.

## Review Page Layout

The current task panel should gain two new sections:

1. `Progress`
2. `MKVMerge Command`

Suggested order inside the current-task panel:

- task status badge
- output file/path
- progress percentage and bar
- formatted command block
- error message when present
- raw log viewer

The progress bar and command block should be visually secondary to the task status, but more prominent than the raw log.

## Error Handling

If command preview generation fails, task start should fail rather than continuing with missing execution metadata. The command display should always reflect the actual command that will be executed.

If progress parsing never finds any percentages, the UI should simply stay at `0%` until terminal state handling updates it.

## Testing Strategy

Add or update tests for:

- current task response includes formatted `commandPreview`
- current task response includes parsed `progressPercent`
- progress parser ignores unrelated log lines
- succeeded tasks report `100%`
- failed tasks preserve last known percentage
- review page renders the progress bar, numeric percentage, and formatted command block

## Non-Goals

This change does not add:

- copy-to-clipboard command UX
- estimated time remaining
- queue position
- multiple concurrent progress bars
- progress inference from anything other than explicit command output
