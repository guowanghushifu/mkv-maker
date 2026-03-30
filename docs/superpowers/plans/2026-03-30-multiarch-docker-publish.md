# Multi-Arch Docker Publish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Support `linux/amd64` and `linux/arm64` images from the same Dockerfile, upgrade local build tooling to Buildx, and publish multi-arch images to both Docker Hub and GHCR under shared tags including `latest`.

**Architecture:** Make the Dockerfile architecture-aware by using BuildKit target variables in the Go build stage instead of hardcoding `amd64`. Upgrade the local build script to `docker buildx build` with a safe single-arch default and explicit multi-arch mode. Replace the current GitHub Actions publish workflow with a Buildx/QEMU pipeline that logs into Docker Hub and GHCR, generates consistent tags, and pushes a multi-arch manifest list for each published tag.

**Tech Stack:** Docker Buildx, QEMU, GitHub Actions, Docker metadata-action, shell scripts

---

## File Map

- Modify: `Dockerfile`
  Responsibility: make Go build architecture-aware using `TARGETOS` / `TARGETARCH`
- Modify: `scripts/docker-build.sh`
  Responsibility: local Buildx-based image build flow with safe single-arch default and explicit multi-arch mode
- Modify: `.github/workflows/docker-publish.yml`
  Responsibility: multi-arch `amd64` + `arm64` publish to Docker Hub and GHCR with shared tags and cache
- Modify: `README.md`
  Responsibility: document local multi-arch build inputs and new registry publish behavior if needed

### Task 1: Make Dockerfile Architecture-Aware

**Files:**
- Modify: `Dockerfile`

- [ ] **Step 1: Write the failing architecture-safety check**

Create a temporary grep-based verification step in your shell session:

```bash
rg -n "GOARCH=amd64|GOOS=linux GOARCH=amd64|TARGETARCH|TARGETOS" Dockerfile
```

Expected before implementation:
- Match exists for hardcoded `GOARCH=amd64`
- No `TARGETARCH` / `TARGETOS` usage in the build stage

- [ ] **Step 2: Verify the current Dockerfile is hardcoded to amd64**

Run: `rg -n "GOARCH=amd64|GOOS=linux GOARCH=amd64" Dockerfile`

Expected: a hit in the Go build stage showing the current hardcoded `amd64` build.

- [ ] **Step 3: Update the Dockerfile to use BuildKit target variables**

Apply this change:

```dockerfile
FROM node:24-trixie-slim AS web-build
WORKDIR /src/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-trixie AS go-build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /out/server ./cmd/server

FROM debian:trixie-slim AS runtime
WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /etc/apt/keyrings && \
    wget -O /etc/apt/keyrings/gpg-pub-moritzbunkus.gpg https://mkvtoolnix.download/gpg-pub-moritzbunkus.gpg && \
    printf '%s\n' \
      'deb [signed-by=/etc/apt/keyrings/gpg-pub-moritzbunkus.gpg] https://mkvtoolnix.download/debian/ trixie main' \
      > /etc/apt/sources.list.d/mkvtoolnix.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends mkvtoolnix && \
    rm -rf /var/lib/apt/lists/*

ENV APP_DATA_DIR=/app/data \
    BD_INPUT_DIR=/bd_input \
    REMUX_OUTPUT_DIR=/remux \
    LISTEN_ADDR=:8080 \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    LANGUAGE=C.UTF-8

COPY --from=go-build /out/server /app/server
COPY --from=web-build /src/web/dist /app/web/dist

RUN mkdir -p /app/data /bd_input /remux

EXPOSE 8080

CMD ["/app/server"]
```

- [ ] **Step 4: Verify the Dockerfile no longer hardcodes amd64**

Run: `rg -n "GOARCH=amd64|TARGETARCH|TARGETOS|BUILDPLATFORM" Dockerfile`

Expected:
- No hardcoded `GOARCH=amd64`
- Positive matches for `TARGETARCH`, `TARGETOS`, and `BUILDPLATFORM`

- [ ] **Step 5: Commit the Dockerfile change**

```bash
git add Dockerfile
git commit -m "build: support multi-arch docker builds"
```

### Task 2: Upgrade Local Docker Build Script To Buildx

**Files:**
- Modify: `scripts/docker-build.sh`
- Modify: `README.md`

- [ ] **Step 1: Write the failing script-behavior checks**

Use grep-based checks to confirm the current script is still plain `docker build`:

```bash
rg -n "docker build |buildx|PLATFORMS|PUSH|--load|--platform" scripts/docker-build.sh
```

Expected before implementation:
- `docker build` present
- no `buildx`
- no `PLATFORMS`

- [ ] **Step 2: Replace the script with a Buildx-aware implementation**

Use this implementation:

```bash
#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${IMAGE_TAG:-mkv-remux-web:local}"
NO_CACHE="${NO_CACHE:-0}"
PLATFORMS="${PLATFORMS:-}"
PUSH="${PUSH:-0}"

build_args=(buildx build)

if [[ "${NO_CACHE}" == "1" ]]; then
  build_args+=(--no-cache)
fi

if [[ -n "${PLATFORMS}" ]]; then
  build_args+=(--platform "${PLATFORMS}")
fi

if [[ "${PUSH}" == "1" ]]; then
  build_args+=(--push)
elif [[ -z "${PLATFORMS}" || "${PLATFORMS}" != *","* ]]; then
  build_args+=(--load)
else
  echo "Multi-arch builds cannot be loaded into the local Docker daemon."
  echo "Set PUSH=1 to publish, or provide a single platform."
  exit 1
fi

build_args+=(-t "${IMAGE_TAG}" .)

docker "${build_args[@]}"
echo "Built image: ${IMAGE_TAG}"
```

- [ ] **Step 3: Verify the script now uses Buildx and platform inputs**

Run: `rg -n "buildx build|PLATFORMS|PUSH|--load|--platform" scripts/docker-build.sh`

Expected:
- matches for `buildx build`
- matches for `PLATFORMS`
- matches for `PUSH`
- matches for `--load` and `--platform`

- [ ] **Step 4: Document the local build modes**

Append or update README build docs with wording like:

```md
Optional local build controls:

- `NO_CACHE=1`: disable Docker layer cache
- `PLATFORMS=linux/amd64,linux/arm64`: request a multi-arch Buildx build
- `PUSH=1`: push the resulting image instead of loading it locally

Examples:

```bash
./scripts/docker-build.sh
PLATFORMS=linux/amd64 ./scripts/docker-build.sh
PLATFORMS=linux/amd64,linux/arm64 PUSH=1 IMAGE_TAG=<registry>/<image>:test ./scripts/docker-build.sh
```
```

- [ ] **Step 5: Commit the local build script update**

```bash
git add scripts/docker-build.sh README.md
git commit -m "build: add buildx local docker script"
```

### Task 3: Upgrade GitHub Actions To Multi-Arch Docker Hub + GHCR Publish

**Files:**
- Modify: `.github/workflows/docker-publish.yml`
- Modify: `README.md`

- [ ] **Step 1: Write the failing workflow checks**

Run:

```bash
rg -n "setup-qemu|setup-buildx|metadata-action|ghcr.io|platforms:|cache-from|cache-to" .github/workflows/docker-publish.yml
```

Expected before implementation:
- no matches for most multi-arch / GHCR setup items

- [ ] **Step 2: Replace the workflow with a multi-arch Buildx publish flow**

Use this workflow shape:

```yaml
name: Docker Publish

on:
  workflow_dispatch:
    inputs:
      image_tag:
        description: "Image tag to publish (for example: v0.1.0)"
        required: true
        default: "v0.1.0"
      push_latest:
        description: "Also push latest tag"
        required: false
        type: boolean
        default: false

permissions:
  contents: read
  packages: write

env:
  IMAGE_NAME: mkv-remux-web

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Lowercase GitHub Owner
        run: echo "GH_OWNER_LC=${OWNER,,}" >> "${GITHUB_ENV}"
        env:
          OWNER: ${{ github.repository_owner }}

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract Docker metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: |
            ${{ secrets.DOCKERHUB_USERNAME }}/${{ env.IMAGE_NAME }}
            ghcr.io/${{ env.GH_OWNER_LC }}/${{ env.IMAGE_NAME }}
          tags: |
            type=raw,value=${{ github.event.inputs.image_tag }}
            type=raw,value=latest,enable=${{ github.event.inputs.push_latest == 'true' }}

      - name: Build and push multi-arch image
        uses: docker/build-push-action@v6
        with:
          context: .
          file: ./Dockerfile
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

- [ ] **Step 3: Verify the workflow includes all required multi-arch and registry features**

Run:

```bash
rg -n "setup-qemu|setup-buildx|metadata-action|ghcr.io|platforms: linux/amd64,linux/arm64|cache-from: type=gha|cache-to: type=gha,mode=max" .github/workflows/docker-publish.yml
```

Expected:
- positive matches for all the above capabilities

- [ ] **Step 4: Update README release docs**

Update the publish section so it reflects:

- Docker Hub and GHCR both receive the image
- `latest` is optional and shared as a multi-arch tag
- required secrets remain:
  - `DOCKERHUB_USERNAME`
  - `DOCKERHUB_TOKEN`
- GHCR uses the built-in `GITHUB_TOKEN`

Use wording like:

```md
Published images:

- `${DOCKERHUB_USERNAME}/mkv-remux-web:<image_tag>`
- `${DOCKERHUB_USERNAME}/mkv-remux-web:latest` (optional)
- `ghcr.io/<github-owner-lowercase>/mkv-remux-web:<image_tag>`
- `ghcr.io/<github-owner-lowercase>/mkv-remux-web:latest` (optional)

The publish workflow builds a shared multi-arch manifest for `linux/amd64` and `linux/arm64`.
```

- [ ] **Step 5: Commit the workflow update**

```bash
git add .github/workflows/docker-publish.yml README.md
git commit -m "ci: publish multi-arch images to docker hub and ghcr"
```

### Task 4: Final Verification

**Files:**
- Review only changed Docker/workflow/docs files

- [ ] **Step 1: Verify Dockerfile and scripts reflect multi-arch support**

Run:

```bash
rg -n "TARGETARCH|TARGETOS|BUILDPLATFORM" Dockerfile
rg -n "buildx build|PLATFORMS|PUSH|--load|--platform" scripts/docker-build.sh
```

Expected:
- Dockerfile shows BuildKit target variables
- script shows Buildx and platform controls

- [ ] **Step 2: Verify workflow contains all required release features**

Run:

```bash
rg -n "setup-qemu|setup-buildx|login-action|ghcr.io|metadata-action|platforms: linux/amd64,linux/arm64|cache-from: type=gha|cache-to: type=gha,mode=max" .github/workflows/docker-publish.yml
```

Expected:
- positive matches for all multi-arch publish requirements

- [ ] **Step 3: Optionally run a local single-arch dry build**

Run:

```bash
IMAGE_TAG=mkv-remux-web:test ./scripts/docker-build.sh
```

Expected:
- successful local build for the host architecture

- [ ] **Step 4: Commit cleanup if needed**

```bash
git add Dockerfile scripts/docker-build.sh .github/workflows/docker-publish.yml README.md
git commit -m "docs: finalize multi-arch docker publish updates"
```
