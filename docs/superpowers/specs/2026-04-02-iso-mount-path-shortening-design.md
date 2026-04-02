# ISO Auto-Mount Path Shortening Design

## Goal

Replace the current long, doubly escaped ISO auto-mount directory names under `/bd_input/iso_auto_mount` with shorter, human-readable names that preserve Chinese characters and remain stable and collision-resistant.

## Problem

The current mount-directory name is derived in two steps:

1. `stableISOID(root, path)` converts the ISO relative path under `BD_INPUT_DIR` into a URL-escaped string.
2. `sanitizeID(sourceID)` URL-escapes that already escaped string again when building the mount directory path.

This produces directory names like:

`/bd_input/iso_auto_mount/%25E9%2580%259F...%252F...`

The current behavior has two problems:

- the directory name is much longer than the original ISO filename or parent path
- percent-escaped text is hard to read and debug, especially after the second escaping pass

## Constraints

- The mounted directory must remain stable across scans and requests for the same ISO.
- Different ISO files with the same filename in different directories must still map to different mount directories.
- Chinese characters in the readable prefix must be preserved.
- The readable prefix must not be URL-escaped.
- The readable prefix must be capped at 40 characters.
- The hash suffix must be derived from the normalized relative path under `BD_INPUT_DIR`.
- Existing API-level `source.id` behavior is not changed by this design.
- The change only affects the auto-mount directory name under `/bd_input/iso_auto_mount`.

## Recommended Design

Split the auto-mount directory name into two parts:

- a readable prefix based on the ISO filename without extension
- a short stable hash based on the normalized relative path under `BD_INPUT_DIR`

Final directory format:

`<readable-prefix>-<hash>`

Example shape:

`/bd_input/iso_auto_mount/速度与激情7-Furious-Seven-2015-2in1-3fa4d2c9b1e7`

## Directory Name Algorithm

### 1. Hash Input

Use the normalized relative path from `BD_INPUT_DIR` to the ISO file:

- compute `rel = filepath.Rel(inputRoot, isoPath)`
- normalize with `filepath.Clean(rel)`
- convert separators to `/` with `filepath.ToSlash(...)`

This normalized relative path is the canonical uniqueness input.

### 2. Hash Output

Compute:

- `sha256(normalizedRelativePath)`
- hex-encode the digest
- take the first 12 lowercase hex characters

Using 12 hex characters gives a stable suffix that is short enough for readability and strong enough to avoid practical collisions in this tool.

Example:

- hash input: `速度与激情7.Furious Seven 2015 2in1 2160p UHD Blu-ray HEVC DTS-X-x-man@HDSky/Furious_Seven_2015_2in1_ULTRA_HD.iso`
- hash suffix: `3fa4d2c9b1e7`

## Readable Prefix Algorithm

The prefix comes from the ISO filename stem only, not the full relative path.

For:

`Furious_Seven_2015_2in1_ULTRA_HD.iso`

use:

`Furious_Seven_2015_2in1_ULTRA_HD`

Transform the stem with these rules:

- keep Chinese characters
- keep ASCII letters and digits
- treat whitespace as separator
- treat `_`, `-`, `.`, `(`, `)`, `[`, `]` as separators
- replace separator runs with a single `-`
- trim leading and trailing `-`
- preserve character case or normalize to a single chosen case consistently

For this project, normalize to the original visible casing rather than forcing lowercase, so operator-facing names remain easier to recognize.

If the resulting prefix is empty, use:

- `iso`

### 40-Character Limit

Apply the 40-character cap to Unicode characters, not bytes.

That means:

- count runes
- truncate the prefix to at most 40 runes
- then append `-<hash>`

This keeps readable names short without breaking Chinese text in the middle of a byte sequence.

## Examples

### Example 1: Mixed Chinese and English

Input ISO path:

`/bd_input/速度与激情7.Furious Seven 2015 2in1 2160p UHD Blu-ray HEVC DTS-X-x-man@HDSky/Furious_Seven_2015_2in1_ULTRA_HD.iso`

Readable stem:

`Furious_Seven_2015_2in1_ULTRA_HD`

Readable prefix after normalization:

`Furious-Seven-2015-2in1-ULTRA-HD`

Final mount directory:

`/bd_input/iso_auto_mount/Furious-Seven-2015-2in1-ULTRA-HD-3fa4d2c9b1e7`

### Example 2: Chinese Filename

If the ISO filename itself is:

`速度与激情7.iso`

Readable prefix:

`速度与激情7`

Final mount directory:

`/bd_input/iso_auto_mount/速度与激情7-3fa4d2c9b1e7`

### Example 3: Same Filename in Different Directories

Paths:

- `folder-a/Movie.iso`
- `folder-b/Movie.iso`

Readable prefixes are both:

`Movie`

But the normalized relative paths differ, so the hash suffix differs, producing different mount directories.

## What Does Not Change

- `SourceEntry.ID` remains whatever the scanner uses for API routing and frontend selection.
- Route behavior and source identity contracts are unchanged by this mount-path shortening design.
- Cleanup logic still works against whatever directory exists under `/bd_input/iso_auto_mount`.
- Existing startup/manual residual cleanup continues to remove old auto-mount directories, including legacy percent-escaped names.

## Compatibility and Migration

No data migration is required.

Behavior after rollout:

- newly mounted ISOs use the new short readable path format
- old doubly escaped directories are still eligible for cleanup by:
  - startup cleanup
  - manual `Release Mounted ISOs`
  - idle janitor
  - shutdown cleanup

This means the new naming format can ship without a one-time conversion step.

## Testing Requirements

Add tests that prove:

- mount directory names no longer contain `%25`
- mount directory names preserve Chinese characters
- mount directory names are not URL-escaped
- readable prefix is capped at 40 characters
- same filename in different directories yields different mount paths
- legacy escaped residual directories are still removable by cleanup logic

## Recommended Implementation Boundary

Do not reuse `SourceEntry.ID` for the mount directory name anymore.

Instead:

- keep `stableISOID(...)` for source identity
- add a separate helper dedicated to mount directory generation, for example:
  - `buildMountDirName(inputRoot, isoPath string) string`

That helper should:

- derive the readable prefix from the ISO filename stem
- derive the hash from the normalized relative path
- return `<prefix>-<hash>`

This keeps route identity and filesystem naming as separate concerns.
