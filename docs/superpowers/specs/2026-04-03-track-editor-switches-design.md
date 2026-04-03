# Track Editor Switches Design

## Goal

Change the track editor page so the `Include` and `Default` controls for audio and subtitle tracks use switch-style controls instead of checkbox visuals.

This is a presentation and interaction-shape update only. The underlying remux draft model and track-selection rules stay the same.

## Product Decisions

### Behavior Stays the Same

The switch UI must preserve the current track rules:

- `Include` still controls whether a track is exported
- `Default` still requires the track to be included
- audio tracks still allow at most one default track
- subtitle tracks still allow at most one default track
- turning off an included default track still clears its default flag

This change must not alter payload shape, review-page output, or remux command generation.

### Both Columns Move Together

The visual update applies to both track booleans in the editor tables:

- `Include`
- `Default`

Changing only one of them would make the table feel inconsistent, so both columns should adopt the same switch treatment in the same release.

### Switch Semantics

The controls should expose switch semantics in the DOM rather than only looking like switches.

Recommended accessibility behavior:

- expose `role="switch"` with checked state
- keep the existing localized accessible names for each row
- preserve disabled state for `Default` when the track is not included

This keeps keyboard and screen-reader behavior aligned with the new visual treatment.

## Frontend Design

### Reusable Switch Component

Add a small reusable frontend `Switch` component rather than styling raw table-local markup inline.

Recommended component surface:

- `checked`
- `disabled`
- `aria-label`
- `onChange`

The component should remain lightweight and local to the existing frontend component layer. It does not need variants, labels, or form-library abstractions.

### Track Editor Integration

Update the track editor table cells in `web/src/features/draft/TrackEditorPage.tsx` so that:

- the `Include` column renders the shared `Switch`
- the `Default` column renders the shared `Switch`
- current change handlers keep calling the same selection/default helpers

No changes are needed to the draft helpers in `web/src/features/draft/trackTable.ts` because the exclusivity and clearing rules are already correct there.

### Visual Direction

The switch styling should fit the existing utilitarian workflow UI:

- compact enough to sit inside the table without increasing row height noticeably
- clear on/off contrast
- visible focus treatment
- muted disabled appearance for `Default` when the row is not included

The table should remain dense and desktop-like. If the current column width becomes cramped, widen the `Include` and `Default` columns slightly rather than shrinking the switch.

## Accessibility and Interaction

The new controls must continue to support:

- mouse click
- keyboard activation
- disabled state
- row-specific accessible labels such as `Include English Atmos` and `Default Commentary`

The UI should not add extra text labels inside the cells. The column headers already provide visible meaning, and the row-level aria labels already provide context for assistive technology.

## Testing Strategy

Update frontend tests to verify:

- the track editor now exposes switch controls for `Include` and `Default`
- enabling a new default audio switch still clears the previous default in the same group
- turning off an included default subtitle track still clears its default flag
- disabled default controls remain non-interactive when the track is not included

Existing reorder, inline edit, and layout tests should remain intact unless their queries need minor updates because of the new switch semantics.

## Implementation Boundaries

This work should stay limited mainly to:

- `web/src/components/`
- `web/src/features/draft/TrackEditorPage.tsx`
- `web/src/styles/`
- track editor frontend tests

Avoid unrelated refactoring in the editor page. The goal is to change control shape and accessibility semantics, not to redesign the table layout or revisit track business logic.

## Non-Goals

This change does not include:

- changes to track-selection rules
- new bulk actions such as select-all or clear-all
- review-page UI changes
- backend or API changes
- a broader design-system rollout of switches across the whole app
