# Environment Variable Exposure Design

## Summary

Adjust the project's environment-variable defaults and documentation so that common users only configure `APP_PASSWORD`, and opt into ISO auto-scan only when needed.

## Goals

- Change `ENABLE_ISO_SCAN` to default to disabled.
- Stop exposing `APP_DATA_DIR`, `BD_INPUT_DIR`, `REMUX_OUTPUT_DIR`, and `LISTEN_ADDR` in user-facing documentation and examples.
- Keep default runtime behavior working through existing internal defaults.
- Document two Docker Compose deployment modes:
  - standard BDMV mode with ISO auto-scan disabled and no extra container privileges
  - ISO auto-scan mode with `ENABLE_ISO_SCAN=1` and the required loop-mount privileges

## Non-Goals

- Removing internal support for overriding path or listen-address environment variables.
- Changing runtime mount points, output paths, or HTTP port defaults.
- Adding a privileged helper script for ISO auto-scan local runs.

## Current State

- Config loading enables ISO scanning by default.
- README documents six environment variables for server usage, including four internal path/listen variables that normal users do not need to set.
- README has a single Docker Compose example that combines ISO auto-scan with elevated container privileges.
- `scripts/docker-run.sh` explicitly forwards the four internal path/listen environment variables even though they match the runtime defaults.

## Proposed Design

### Configuration

- Change config loading so `ENABLE_ISO_SCAN` defaults to `false`.
- Preserve explicit overrides:
  - `ENABLE_ISO_SCAN=1` enables ISO scanning.
  - `ENABLE_ISO_SCAN=0` disables ISO scanning.
- Update config tests to cover default disabled behavior and explicit enable/disable cases.

### User Documentation

- Reduce the documented user-facing environment variables to:
  - `APP_PASSWORD`
  - `ENABLE_ISO_SCAN`
  - `SESSION_COOKIE_SECURE`
- Remove `APP_DATA_DIR`, `BD_INPUT_DIR`, `REMUX_OUTPUT_DIR`, and `LISTEN_ADDR` from the environment-variable list and usage examples.
- Rewrite the ISO notes to make the default behavior explicit:
  - default deployment does not scan `.iso`
  - enabling ISO auto-scan requires Linux plus loop-mount capability inside the container
  - if those requirements are unavailable, users should keep ISO scan disabled and mount ISO content on the host before exposing BDMV directories to the container

### Docker Compose Examples

- Split the README example into two Compose snippets:
  - standard BDMV deployment
    - omits `ENABLE_ISO_SCAN` or shows it as `0`
    - does not include `cap_add`, `security_opt`, or loop-device mappings
  - ISO auto-scan deployment
    - sets `ENABLE_ISO_SCAN: "1"`
    - includes `SYS_ADMIN`, relaxed seccomp/apparmor, and loop-device mappings
- Keep the volume paths in examples unchanged so the examples still match the runtime defaults.

### Local Docker Run Script

- Simplify `scripts/docker-run.sh` so it only passes `APP_PASSWORD` and relies on image/runtime defaults for:
  - `/app/data`
  - `/bd_input`
  - `/remux`
  - `:8080`
- Keep host-side bind mount environment variables in the script, since they are shell-script inputs rather than application environment variables exposed to users inside the container.

## Data Flow And Behavior

- The server continues to resolve internal directories and listen address from existing defaults unless advanced users override them manually.
- The only default behavior change is that `.iso` files are no longer scanned unless `ENABLE_ISO_SCAN=1` is set.
- Standard BDMV deployments no longer imply privileged container settings in the documentation.

## Error Handling

- No new error paths are introduced.
- Existing ISO mount failures remain unchanged when users explicitly enable ISO scanning without the required privileges; documentation will direct users to either grant the required privileges or keep ISO scanning disabled.

## Testing

- Update unit tests in config loading to assert:
  - default `ENABLE_ISO_SCAN` is `false`
  - `ENABLE_ISO_SCAN=1` enables scan
  - `ENABLE_ISO_SCAN=0` disables scan
- No automated test coverage is planned for README or shell script changes beyond targeted manual review.

## Risks

- Users who implicitly depended on default ISO scanning will need to set `ENABLE_ISO_SCAN=1`.
- Some existing deployments may still set the hidden path/listen variables manually; this remains supported but undocumented.

## Acceptance Criteria

- Config defaults ISO scanning to disabled.
- README no longer documents `APP_DATA_DIR`, `BD_INPUT_DIR`, `REMUX_OUTPUT_DIR`, or `LISTEN_ADDR`.
- README contains separate Compose examples for non-ISO and ISO-auto-scan deployments.
- `scripts/docker-run.sh` no longer passes the four internal path/listen environment variables.
