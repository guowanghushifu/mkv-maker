# mkv-remux-web

This project now includes a Docker runtime that builds and ships `makemkvcon` (from official MakeMKV `oss` + `bin` tarballs), plus `mkvtoolnix`, `ffmpeg`, and `mediainfo`.

## Docker Build

`Dockerfile` is multi-stage and builds:
- frontend assets (`web`)
- Go server binary (`cmd/server`)
- MakeMKV CLI runtime under `/opt/makemkv`

Build with the helper:

```bash
scripts/docker-build.sh
```

Override image name and MakeMKV version:

```bash
IMAGE_NAME=mkv-maker:dev MAKEMKV_VERSION=1.18.1 scripts/docker-build.sh
```

`MAKEMKV_VERSION` is passed to the Docker build as a build-arg and controls which official tarballs are downloaded:
- `https://www.makemkv.com/download/makemkv-oss-${MAKEMKV_VERSION}.tar.gz`
- `https://www.makemkv.com/download/makemkv-bin-${MAKEMKV_VERSION}.tar.gz`

## MakeMKV Key Behavior

The image defaults to:

```bash
MAKEMKV_KEY=BETA
```

Startup behavior in `scripts/docker-entrypoint.sh`:
- `MAKEMKV_KEY=BETA`: fetch latest beta key from the MakeMKV forum thread and write it to settings.
- `MAKEMKV_KEY=<custom>`: write the provided key to settings.
- `MAKEMKV_KEY` unset or empty: do not change the key.

Beta-key update is best-effort. If fetching/parsing fails, container startup continues.

## Persistent MakeMKV Config/Data

At container start:
- persistent config directory is ensured at `/app/data/makemkv`
- default settings are copied to `/app/data/makemkv/settings.conf` if missing
- `$HOME/.MakeMKV` is linked to `/app/data/makemkv`

Default settings file:
- destination directory: `/remux`
- MakeMKV data directory: `/app/data/makemkv/data`

## Local Run

Run with local volume mounts via helper script:

```bash
APP_PASSWORD=changeme scripts/docker-run.sh
```

With custom MakeMKV key:

```bash
APP_PASSWORD=changeme MAKEMKV_KEY='T-XXXX-XXXX-XXXX-XXXX' scripts/docker-run.sh
```

Disable key updates/writes (empty key):

```bash
APP_PASSWORD=changeme MAKEMKV_KEY='' scripts/docker-run.sh
```

By default `scripts/docker-run.sh` mounts:
- `${REPO}/.data/app` -> `/app/data`
- `${REPO}/.data/bd_input` -> `/bd_input`
- `${REPO}/.data/remux` -> `/remux`
