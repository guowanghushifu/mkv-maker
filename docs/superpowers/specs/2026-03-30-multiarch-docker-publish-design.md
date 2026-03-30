# Multi-Arch Docker Publish Design

## Goal

Extend the container build and release flow so the project supports:

- `linux/amd64`
- `linux/arm64`

and publishes both architectures under the same image tags, including a shared `latest` tag.

The publish workflow should push to:

- Docker Hub
- GitHub Container Registry (GHCR)

## Product Decisions

### Supported Architectures

The project should officially build runtime images for:

- `linux/amd64`
- `linux/arm64`

These two architectures should be built from the same Dockerfile using a multi-platform BuildKit flow.

### Registry Targets

Published release images should be pushed to both registries:

- Docker Hub
- GHCR

Both registries should receive the same logical tag set for a given publish event.

### latest Tag Semantics

When `latest` is published, it should be one multi-arch manifest tag shared across both architectures, not two separate architecture-specific `latest` tags exposed directly to users.

### Local Build Behavior

Local development builds should remain convenient:

- default local build targets the current host architecture only
- explicit multi-arch local builds are supported via `buildx`

This avoids making the common local build path slower or more fragile than necessary.

## Dockerfile Requirements

The Dockerfile must no longer hardcode `amd64` during Go compilation.

Instead, it should use BuildKit-provided target platform variables, typically:

- `TARGETOS`
- `TARGETARCH`

The Go build stage should compile the server binary for the requested target architecture so the same Dockerfile works for both `amd64` and `arm64`.

This change applies only to build-time architecture targeting. Runtime behavior should remain unchanged.

## Local Build Script Requirements

`scripts/docker-build.sh` should be upgraded from plain `docker build` to `docker buildx build`.

Default behavior:

- build only for the current host platform
- load the resulting image into the local Docker daemon for easy testing

Optional multi-arch behavior:

- allow `PLATFORMS=linux/amd64,linux/arm64`
- in multi-arch mode, avoid pretending the result will be locally loadable as a normal single-arch image unless an explicit output mode makes that valid

The script should make the mode clear so local users do not get confused about where the resulting image lives.

## GitHub Actions Requirements

The existing Docker publish workflow should be upgraded to a multi-platform Buildx release workflow.

Required capabilities:

- checkout source
- set up QEMU
- set up Docker Buildx
- log in to Docker Hub
- log in to GHCR
- generate tags and labels consistently
- build `linux/amd64,linux/arm64`
- push to both registries
- use GitHub Actions cache for Buildx layers when practical

## Tag Strategy

The workflow should produce the same semantic tags across both registries.

At minimum, the project must preserve the current manual tag input behavior for release publishing.

Recommended behavior:

- explicit version tag from workflow input
- optional `latest`

If metadata extraction is used, it should still honor the current release flow and not silently change the project’s release tagging semantics.

## Registry Naming

Docker Hub image:

- `${DOCKERHUB_USERNAME}/mkv-remux-web`

GHCR image:

- `ghcr.io/<lowercased-github-owner>/mkv-remux-web`

The workflow should normalize the GitHub owner name to lowercase before composing the GHCR image path.

## Caching

The publish workflow should use Buildx cache integration via GitHub Actions cache if possible:

- `cache-from: type=gha`
- `cache-to: type=gha,mode=max`

This is an optimization, not a functional requirement. Publish correctness matters more than cache sophistication.

## Non-Goals

This change does not add:

- Windows containers
- non-Linux runtime images
- per-architecture user-facing tags as the main release UX
- local registry publishing flows
- release automation beyond the current manual workflow trigger model unless separately requested

## Testing Strategy

Add or update verification for:

- Dockerfile correctly using target architecture variables
- local build script still working in default single-arch mode
- workflow configuration defining multi-platform builds for `amd64` and `arm64`
- workflow pushing to both Docker Hub and GHCR
- workflow tag output still matching current release expectations
