# Build Time Sidebar Design

## Summary

Replace the low-value sidebar session card with a build-time card that shows the Docker image build timestamp in GMT+8 using the format `YYYY.MM.DD HH:mm`.

## Goals

- Replace the current sidebar copy `Remux 会话 / 当前操作上下文` with `构建时间 / <timestamp>`.
- Generate the timestamp during Docker image builds, not at container startup.
- Keep the timestamp calculation fixed to GMT+8.
- Make the injection path work for both local image builds and GitHub Actions image builds.
- Keep local non-Docker frontend development functional with a clear fallback value when no build time is injected.

## Non-Goals

- Adding a backend API or config endpoint for build metadata.
- Showing commit SHA, version tags, or runtime start time in the UI.
- Changing the layout or visual styling of the sidebar card beyond its displayed content.

## Current State

- The sidebar footer card is rendered by `web/src/components/Layout.tsx`.
- Its current text comes from `web/src/i18n.ts` as `Remux 会话` and `当前操作上下文`.
- The Docker image build has two entry points:
  - local builds through `scripts/docker-build.sh`
  - GitHub Actions builds through `.github/workflows/docker-publish.yml`
- The web build currently does not receive any build metadata.

## Approved Design

### UI Content

- Keep the existing sidebar card structure and styling.
- Change the title to `构建时间`.
- Change the subtitle/body text to the injected build timestamp string.
- When no build timestamp is injected, show a stable fallback string rather than leaving the field blank.

### Build-Time Injection

- Standardize on a single build argument named `BUILD_TIME`.
- Generate `BUILD_TIME` immediately before invoking Docker image builds.
- Use `TZ=Asia/Shanghai date '+%Y.%m.%d %H:%M'` so the produced string matches the required GMT+8 format.
- Pass `BUILD_TIME` into the `web-build` stage of the Dockerfile.
- Export it there as `VITE_BUILD_TIME` before running `npm run build`, so Vite bakes the value into the frontend bundle.

### Build Entrypoints

- Update `scripts/docker-build.sh` to compute `BUILD_TIME` and add `--build-arg BUILD_TIME=...`.
- Update `.github/workflows/docker-publish.yml` to compute the same GMT+8 timestamp in a dedicated step and pass it to `docker/build-push-action` through `build-args`.
- Do not rely on `scripts/docker-build.sh` for CI correctness; GitHub Actions must inject the value independently.

### Frontend Read Path

- Add a small frontend build-metadata read path based on `import.meta.env.VITE_BUILD_TIME`.
- Keep the logic local to the shell layout or a tiny helper so the change stays narrow.
- Replace the old session-card copy keys with build-time-specific keys in `web/src/i18n.ts`.

## Data Flow

1. The build entry point generates `BUILD_TIME` in GMT+8.
2. Docker receives `BUILD_TIME` as a build argument.
3. The `web-build` stage maps it to `VITE_BUILD_TIME`.
4. Vite statically injects the string into the built frontend.
5. The sidebar card reads `VITE_BUILD_TIME` and renders it under the `构建时间` title.

## Error Handling

- If `BUILD_TIME` is omitted during a non-Docker frontend build, the UI should show a deterministic fallback string.
- No runtime parsing or timezone conversion should happen in the browser; the displayed value should be treated as final preformatted text.
- If GitHub Actions or the local script fails to produce the build argument, the frontend still builds, but the fallback value makes the missing injection visible.

## Testing

- Add frontend tests for the sidebar card that cover:
  - injected build time is rendered
  - fallback text is rendered when no build time is injected
- Run targeted frontend tests for the layout component.
- No dedicated backend tests are needed because the feature is entirely build-time plus frontend display.

## Risks

- Docker layer caching can preserve an older frontend bundle if `BUILD_TIME` is not passed correctly at every build entry point.
- Direct `npm run build` usage outside Docker will not reflect a real image build time, so the fallback must remain intentional and clear.

## Acceptance Criteria

- The sidebar card title is `构建时间`.
- The sidebar card content displays a timestamp like `2026.04.02 18:30`.
- Local Docker builds inject the timestamp through `scripts/docker-build.sh`.
- GitHub Actions Docker builds inject the timestamp through the workflow itself.
- The Dockerfile passes the value into the Vite build as `VITE_BUILD_TIME`.
- The frontend has test coverage for injected and fallback display states.
