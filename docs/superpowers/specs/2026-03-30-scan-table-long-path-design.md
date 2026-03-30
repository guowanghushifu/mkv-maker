# Scan Table Long Path Design

## Goal

Keep the scan results table inside the page width when source paths are very long, while still allowing the full path to be inspected.

## Approved Design

- Keep the existing scan results table structure on the scan page.
- Make the entire scan table slightly denser by reducing header and cell font sizes.
- Switch the scan table to a fixed-width layout so one long path cannot force the whole table wider than its container.
- Render the path value inside a dedicated element that stays on one line, truncates with an ellipsis, and exposes the full path through the `title` attribute on hover.
- Preserve the existing horizontal overflow wrapper as a fallback for narrow viewports.

## Scope

- Modify the scan page component markup as needed for the path cell.
- Update scan table CSS only.
- Add a frontend regression test covering the long-path hover/title behavior.
