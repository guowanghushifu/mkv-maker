# Track Editor Table Design

## Goal

Refine the track-editing page so it behaves more like a compact desktop remux tool:

- default-track control uses checkbox visuals
- track ordering uses drag-and-drop instead of move-up and move-down buttons
- track metadata is shown in compact table-style layouts inspired by MKVToolNix GUI

This change only affects the track-editing experience. It does not change the broader scan, BDInfo, review, or remux workflow.

## Product Decisions

### Default Track

The UI should display default-track controls as checkboxes, not radio buttons.

Behavior remains exclusive within each group:

- audio tracks may have at most one default track
- subtitle tracks may have at most one default track
- checking one default box clears any other default box in the same group
- unselecting a track from export also clears its default flag

This keeps the current export rules while matching the interaction style requested by the user.

### Reordering

All rows in the audio and subtitle tables are draggable, including rows that are not currently selected for export.

Only selected tracks are exported, but their export order is determined by their current row order inside the table.

Reordering rules:

- audio tracks reorder only inside the audio table
- subtitle tracks reorder only inside the subtitle table
- there are no move-up or move-down buttons
- dragging updates the underlying array order directly

### Layout Direction

Use two separate compact tables:

- one for audio tracks
- one for subtitle tracks

This is preferred over a single mixed grid because:

- it matches how remux tools usually separate track categories
- it keeps the page denser and easier to scan
- it avoids mixing unrelated defaults and metadata into one overloaded table

The video track should remain outside these tables as a concise read-only summary plus editable video name field.

## Page Structure

The page should be organized into four vertical sections:

1. Global draft fields
2. Video summary strip
3. Audio tracks table
4. Subtitle tracks table

### 1. Global Draft Fields

Keep the existing top-level controls for:

- title
- video track name
- output filename preview or editable output filename field

These remain above the tables because they describe the whole remux draft rather than one individual track row.

### 2. Video Summary Strip

Render video details in a compact summary block rather than a row inside the audio or subtitle tables.

The strip should show:

- edited video track name
- codec
- resolution
- HDR or Dolby Vision markers

The video track is not part of the draggable audio or subtitle ordering UI.

### 3. Audio Table

The audio table should use dense desktop-style rows with columns similar to:

- drag handle
- include checkbox
- track name editor
- language editor
- default checkbox
- details

The details column is read-only and should show codec and layout labels when available, for example:

- `TrueHD 7.1 Atmos`
- `DTS-HD MA 5.1`
- `AC-3 2.0`

### 4. Subtitle Table

The subtitle table mirrors the audio table with the same compact structure:

- drag handle
- include checkbox
- track name editor
- language editor
- default checkbox
- details

The details column should show subtitle format or similar descriptors when available, such as `PGS`.

## Visual Direction

The page should take visual cues from desktop remux tooling instead of generic web forms:

- compact row height
- subtle table borders
- slightly darker table headers
- clear alignment between columns
- restrained highlight for selected rows
- reduced contrast for rows that are not selected for export

The goal is not to clone MKVToolNix exactly, but to achieve the same feel of dense, utilitarian track management.

## Interaction Details

### Drag and Drop

Each row should have a visible drag handle at the left edge.

When the user drags a row:

- the target row position is indicated visually
- dropping the row updates the corresponding track array order
- the review page later reflects the selected tracks in this new order

Drag interaction should be explicit and discoverable. The handle is preferred over making the entire row draggable because it reduces accidental drags while editing inline inputs.

### Inline Editing

Track name and language should stay editable directly inside the table.

These fields should use compact controls sized to fit the table without expanding row height excessively.

### Selected State

Rows with `selected = false` remain visible and draggable, but look visually muted.

This preserves context for all available tracks while making it obvious which tracks will be exported.

### Default State

The default checkbox is enabled only when the row is selected for export.

If the row is deselected:

- the default checkbox becomes unchecked
- the default checkbox becomes disabled until the row is selected again

## Responsive Behavior

Desktop layout is the priority.

On smaller screens:

- the tables may scroll horizontally
- the structure should stay table-like rather than switching to stacked cards

This keeps behavior consistent and avoids introducing a second editing model just for mobile.

## Implementation Boundaries

The change should be limited mainly to:

- `web/src/features/draft/TrackEditorPage.tsx`
- `web/src/styles/app.css`
- track-editor frontend tests

Avoid unnecessary changes to backend draft payloads or review-page ordering logic. The current data model already supports ordered arrays and selected/default flags.

If helpful for cleanliness, extract a small local table-row helper component inside the same feature area, but do not over-engineer this into a new component system.

## Testing Strategy

Update frontend tests to verify:

- checking one default checkbox clears the previous default in the same group
- deselecting a defaulted track clears its default flag
- dragging a row changes the array order passed to `onChange`
- inline edits for name and language still update draft state

If drag-and-drop is implemented with a library that is awkward to test end-to-end in unit tests, cover the reorder helper logic directly and add at least one component-level smoke test around drag wiring.

## Non-Goals

This change does not include:

- cross-group dragging between audio and subtitle tables
- percentage-based remux progress
- changes to BDInfo parsing or output naming rules
- additional bulk actions such as select-all or reset-to-source-order
