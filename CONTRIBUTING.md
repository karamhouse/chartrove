# Contributing to Chartrove

Chartrove is an independent fork of [Fasten Onprem](https://github.com/fastenhealth/fasten-onprem). Keep upstream authorship and copyright notices intact. New issues and PRs belong in [karamhouse/chartrove](https://github.com/karamhouse/chartrove).

## Toolchain

- Go 1.26 or newer; root and desktop modules retain separate dependency graphs.
- Node 26.13.0, pinned in `mise.toml`, with the committed Yarn 1.22.22 configuration.
- Angular 18.x through the local, locked CLI (currently core 18.2.14, CLI 18.2.21, CDK 18.2.14, and TypeScript 5.5.4).
- macOS with Xcode Command Line Tools for native desktop builds.

Install only the locked frontend dependencies when needed:

```sh
cd frontend
npm exec --yes --package=corepack@0.35.0 -- corepack yarn install --frozen-lockfile --ignore-optional --registry https://registry.npmjs.org --network-concurrency 4
cd ..
```

Node 26 does not bundle Corepack. The command above invokes a pinned Corepack through npm and preserves the committed Yarn selection. Use `npm exec --yes --package=corepack@0.35.0 -- corepack yarn exec ng <command>` for the local CLI without a global Yarn or Angular CLI installation. The Angular workspace explicitly selects Yarn.

The Angular 14-to-22 migration is in progress. Node 26 remains outside Angular 18's official support matrix; Angular 22 is the first release family in the current official matrix supporting Node 26. Run migration steps on Node 26 and report concrete failures; do not reinstall retired Node runtimes as a workaround. Keep TypeScript within the selected Angular compiler's supported range. See [the maintenance baseline](docs/MAINTENANCE_BASELINE.md) for current validation limits and the migration sequence.

## Repository layout

| Path | Purpose |
| --- | --- |
| `frontend/src/app` | Angular views and application services |
| `frontend/src/assets/desktop` | Native bridge adapter |
| `backend/pkg` | Go backend, FHIR handling, database, and HTTP routes |
| `desktop/cmd/fasten-desktop` | Native Wails host entry point |
| `desktop/internal` | Storage, window authorization, bridge, and native smoke checks |
| `desktop/build.sh` | Chartrove app packaging |

Go module/import paths and bridge service names still contain legacy Fasten identifiers. They are compatibility contracts, not display branding; a module migration needs coordinated backend, frontend, and bridge changes and is outside the branding update.

## Desktop development

See [desktop/README.md](desktop/README.md) for builds, flags, and signing. Use an explicitly selected, newly created private directory for testing. Never use the purchased app's storage or an existing health-record database as a fixture.

```sh
sh desktop/build.sh
```

For frontend iteration, select the existing `desktop_prod` configuration. A native executable can serve a built asset directory through `--frontend-dir`; this flag does not select or migrate a database.

## Validation

From the repository root:

```sh
make prepare-backend
go test -p 1 ./backend/... -count=1
cd frontend
node --test desktop-tests/bridge.test.mjs
node node_modules/@angular/cli/bin/ng.js test --watch=false --browsers=ChromeHeadlessCI
```

`make prepare-backend` generates the ignored `backend/resources/related_versions.json` required by the backend executable. It does not tidy, vendor, or upgrade dependencies. The backend test targets use this preparation step; dependency refresh remains the separate `dep-backend` target. Avoid the inherited Nix `.envrc` for this toolchain.

Database fixtures use test-scoped directories and close their repositories before cleanup. Run validation with `TMPDIR` set to a newly created private task directory so any leftovers from failed or interrupted tests can be identified and removed after their processes exit. Never point tests at an existing health-record directory.

From `desktop/`:

```sh
go test ./...
go test github.com/fastenhealth/fasten-onprem/backend/pkg/web -run TestEmbedded -count=1 -timeout=120s
```

The native `--smoke-test` mode requires `--unencrypted` and an explicit fresh `--data-dir`. It refuses an existing database. It validates the real WebView bridge and uses synthetic records only.

The opt-in PostgreSQL driver regression requires Docker with a local Unix socket.
Pull its pinned PostgreSQL 17 image, then run from the repository root:

```sh
docker pull postgres:17-alpine@sha256:266d3c257594e195b309ae9d1805e54c788f1d7b375f56bf6c367aac6c917e7b
go test -tags=postgres_integration ./backend/pkg/database -run '^TestPostgresIntegration$' -count=1
```

The test creates a loopback-only disposable container with a generated password
and in-memory data directory, and removes it afterward. It accepts no existing
database connection or host volume. To check the desktop dependency graph, run
the same command from `desktop/` with the package path
`github.com/fastenhealth/fasten-onprem/backend/pkg/database`.
This checks driver parameter binding, fresh repository migration, synthetic
account and FHIR date persistence, and reopen. It does not establish compatibility
of every advanced resource query or an existing production database.

Choose checks appropriate to the change. Do not relax timeouts, encryption, or test assertions to make a failing check pass. Distinguish a source check from a successful build and a real native UI check.

## Branding, data, and provider boundaries

Use Chartrove for product-facing names and link project support to this repository. Preserve Fasten attribution and external service identities. [BRANDING.md](docs/BRANDING.md) records the compatibility names that remain intentionally stable.

Leave `healthit.yaml` and existing health records untouched. Keep `fasten.db`, existing storage paths, key management, and migration behavior stable unless a separately scoped data migration has been approved.

The Epic sandbox connection uses published synthetic test data and upstream sandbox infrastructure. It does not grant production access. See [the provider validation record](desktop/PROVIDER_VALIDATION.md) and [Lighthouse replacement assessment](desktop/LIGHTHOUSE_REPLACEMENT.md).

## Pull requests

Keep changes focused and describe the resulting behavior and validation. Preserve commit attribution when adopting upstream contributions. Migrated draft PRs target historical bases to preserve their original comparisons; review and retarget/rebase them before merging. Existing review transcripts are historical context, not approvals for this fork.

The [upstream Fasten documentation](https://github.com/fastenhealth/docs/tree/main/technical) is useful historical context. Its hosted-service, deployment, and provider-registration instructions may be outdated for Chartrove.
