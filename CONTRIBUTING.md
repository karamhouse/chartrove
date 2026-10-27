# Contributing to Chartrove

Chartrove is an independent fork of [Fasten Onprem](https://github.com/fastenhealth/fasten-onprem). Keep upstream authorship and copyright notices intact. New issues and PRs belong in [karamhouse/chartrove](https://github.com/karamhouse/chartrove).

## Toolchain

- Go 1.26 or newer; root and desktop modules retain separate dependency graphs.
- Node 20.19.0 and the committed Yarn 1.22.22 configuration.
- Angular 14.1.x through the local, locked CLI.
- macOS with Xcode Command Line Tools for native desktop builds.

Install only the locked frontend dependencies when needed:

```sh
cd frontend
corepack yarn install --frozen-lockfile --ignore-optional --registry https://registry.npmjs.org --network-concurrency 4
cd ..
```

Use the local CLI rather than installing or upgrading a global Angular CLI for a project change.

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
go test ./backend/... -count=1
cd frontend
node --test desktop-tests/bridge.test.mjs
node node_modules/@angular/cli/bin/ng.js test --watch=false --browsers=ChromeHeadlessCI
```

From `desktop/`:

```sh
go test ./...
go test github.com/fastenhealth/fasten-onprem/backend/pkg/web -run TestEmbedded -count=1 -timeout=120s
```

The native `--smoke-test` mode requires `--unencrypted` and an explicit fresh `--data-dir`. It refuses an existing database. It validates the real WebView bridge and uses synthetic records only.

Choose checks appropriate to the change. Do not relax timeouts, encryption, or test assertions to make a failing check pass. Distinguish a source check from a successful build and a real native UI check.

## Branding, data, and provider boundaries

Use Chartrove for product-facing names and link project support to this repository. Preserve Fasten attribution and external service identities. [BRANDING.md](docs/BRANDING.md) records the compatibility names that remain intentionally stable.

Leave `healthit.yaml` and existing health records untouched. Keep `fasten.db`, existing storage paths, key management, and migration behavior stable unless a separately scoped data migration has been approved.

The Epic sandbox connection uses published synthetic test data and upstream sandbox infrastructure. It does not grant production access. See [the provider validation record](desktop/PROVIDER_VALIDATION.md) and [Lighthouse replacement assessment](desktop/LIGHTHOUSE_REPLACEMENT.md).

## Pull requests

Keep changes focused and describe the resulting behavior and validation. Preserve commit attribution when adopting upstream contributions. Migrated draft PRs target historical bases to preserve their original comparisons; review and retarget/rebase them before merging. Existing review transcripts are historical context, not approvals for this fork.

The [upstream Fasten documentation](https://github.com/fastenhealth/docs/tree/main/technical) is useful historical context. Its hosted-service, deployment, and provider-registration instructions may be outdated for Chartrove.
