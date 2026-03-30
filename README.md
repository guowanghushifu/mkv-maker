# mkv-remux-web

`mkv-remux-web` is a web UI + Go backend for preparing MKV remux jobs from Blu-ray sources.
Current product scope is **BDMV-only input** and **BDInfo text is required** to build draft metadata.

## Runtime environment variables

The server uses these environment variables:

- `APP_PASSWORD` (required): login password for the web app
- `APP_DATA_DIR` (default: `/app/data`): application log directory
- `BD_INPUT_DIR` (default: `/bd_input`): mounted BDMV source directory
- `REMUX_OUTPUT_DIR` (default: `/remux`): output directory for remuxed files
- `LISTEN_ADDR` (default: `:8080`): HTTP listen address

## Docker (local)

Build:

```bash
./scripts/docker-build.sh
```

Optional custom image tag:

```bash
IMAGE_TAG=mkv-remux-web:test ./scripts/docker-build.sh
```

Run:

```bash
APP_PASSWORD=change-me ./scripts/docker-run.sh
```

Optional host mount overrides:

- `APP_DATA_HOST_DIR` (default: `$PWD/.data`): host directory for application logs
- `BD_INPUT_HOST_DIR` (default: `$PWD/bd_input`)
- `REMUX_OUTPUT_HOST_DIR` (default: `$PWD/remux_output`)

The container publishes `http://localhost:8080`, serves the web UI at `/`, and serves API routes under `/api/*`.

`mkvtoolnix` is installed from the official MKVToolNix Debian repository for `trixie`, following the vendor instructions at:
- https://mkvtoolnix.download/downloads.html#debian

## Docker Hub publish workflow

Manual release workflow: `.github/workflows/docker-publish.yml`.

Configure GitHub repository secrets:

- `DOCKERHUB_USERNAME`
- `DOCKERHUB_TOKEN` (Docker Hub access token)

Then run the **Docker Publish** workflow with:

- `image_tag` (for example `v0.1.0`)
- `push_latest` (`true` to also push `latest`)

Image is pushed as:

- `${DOCKERHUB_USERNAME}/mkv-remux-web:<image_tag>`
- `${DOCKERHUB_USERNAME}/mkv-remux-web:latest` (optional)
