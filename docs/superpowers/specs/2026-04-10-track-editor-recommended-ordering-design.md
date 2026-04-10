# Track Editor Recommended Ordering Design

## Goal

Extend the track editor `猜你喜欢` bulk action so it not only resets selection by rule, but also reorders the resulting track list so every selected track appears before every unselected track.

This is a frontend-only behavior change. It must not change API payload shape, backend draft generation, drag-and-drop ordering behavior, or manual per-track toggle behavior.

## Product Decisions

### Reorder Trigger

The reorder happens only when the user clicks `猜你喜欢` for audio or subtitles.

It does not run when:

- the user manually toggles a track include switch
- the user changes the default switch
- the user drags rows
- the user reorders rows with the keyboard

### Stable Selected-First Ordering

After `猜你喜欢` recalculates `selected` and `default`, the target track group must be reordered into two stable partitions:

- selected tracks first
- unselected tracks second

Within each partition, the original relative order must be preserved.

Example:

- before recommendation: `A1(selected)`, `A2(unselected)`, `A3(selected)`, `A4(unselected)`
- after recommendation ordering: `A1(selected)`, `A3(selected)`, `A2(unselected)`, `A4(unselected)`

This keeps the recommendation easy to review without destroying the source-order relationship inside the selected set.

### Default Track Rule

The default-track rule remains the same as the current recommendation behavior:

- only selected tracks may remain default
- at most one track may remain default
- if a selected track was already default before the reset, keep that track as default
- otherwise use the first selected track as default

Because selected tracks move to the front, the default track will always appear inside the leading selected block.

## Frontend Design

### Helper Ownership

The selected-first reorder should live inside the existing recommendation helper in `web/src/features/draft/trackTable.ts`.

`applyRecommendedTrackSelection` should continue to own the complete recommendation result for a `DraftTrack[]`:

- decide which tracks are selected
- enforce a valid exclusive default
- return the final selected-first stable ordering

This keeps the page component free of recommendation-specific list manipulation and ensures audio and subtitle buttons stay identical.

### Page Integration

`web/src/features/draft/TrackEditorPage.tsx` should continue to call `applyRecommendedTrackSelection` directly for both bulk-action buttons.

No component-level sorting logic should be added in the click handlers.

## Testing Strategy

Add or update frontend tests to cover:

- helper-level recommendation output moves selected audio tracks to the front while preserving selected relative order
- helper-level recommendation output moves selected subtitle tracks to the front while preserving unselected relative order
- page-level audio `猜你喜欢` click returns the reordered list in `onChange`
- page-level subtitle `猜你喜欢` click returns the reordered list in `onChange`
- default-track selection remains valid after the reorder

## Non-Goals

This change does not include:

- auto-sorting after any manual toggle
- changing drag-and-drop behavior
- changing source indexes
- introducing a general-purpose sort control for track tables
