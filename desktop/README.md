# Fasten Community desktop

An initial macOS desktop revival using this fork's current frontend and backend, with a new Wails v3 host. This is a local development build, ad hoc signed and not notarized. It does not replace the purchased Fasten app.

## Build

Use Go 1.26 or newer, macOS with Xcode Command Line Tools, and Node 20.19.0 (the frontend version tested here). The separate desktop Go module pins Wails to v3.0.0. The existing Angular 14 frontend is retained.

With Node 20 selected in your shell:

```sh
cd frontend
corepack yarn install --frozen-lockfile --ignore-optional --registry https://registry.npmjs.org --network-concurrency 4
cd ..
sh desktop/build.sh
```

The result is `dist/FastenCommunity.zip`, containing `FastenCommunity.app` for the build machine's architecture. Signing takes place in an OS temporary directory because macOS File Provider can attach metadata to `.app` directories inside Documents that prevents signing. The ZIP preserves the signed bundle. The build does not install the app or change system trust settings.

After a successful frontend build, `sh desktop/build.sh --skip-frontend` can rebuild only the native package when frontend inputs have not changed. It uses the existing `dist/frontend` output. A normal build regenerates frontend output and synchronizes the embedded assets to remove stale files.

## Data and startup

The default data directory on macOS is `~/Library/Application Support/FastenCommunity`. The host creates it privately, keeps a persistent JWT signing key in a mode-0600 file, and locks it against a second desktop instance. This directory is separate from the purchased application's storage.

Database encryption is enabled by default. The fork's existing encryption-key setup and restore screens handle creation and unlock. Save the recovery key when prompted; the host does not persist it. Existing purchased-app databases and their migrations have not been validated and must not be used as test inputs.

For a separate development database, launch the extracted executable with an explicit private directory:

```sh
FastenCommunity.app/Contents/MacOS/fasten-community --data-dir /path/to/private/test-directory
```

`--unencrypted` is available only for disposable synthetic-data testing. `--frontend-dir` selects a development asset directory. Normal startup embeds the fork's `/api` routes in Wails without opening an application TCP listener or requiring hosted installation registration.

The native bridge uses the runtime served by the pinned host. It validates external URLs, restricts privileged actions to the main window, and restricts callback completion to the popup for the matching state. OAuth callback data is delivered directly to the main window, not broadcast to other provider windows. Real provider OAuth and hosted CCDA conversion remain unvalidated external capabilities.

## Tests

From the repository root:

```sh
go test ./backend/pkg/web ./backend/pkg/event_bus -count=1 -timeout=120s
go test -race ./backend/pkg/web ./backend/pkg/event_bus -count=1 -timeout=120s
cd frontend
node --test desktop-tests/bridge.test.mjs
node node_modules/@angular/cli/bin/ng.js test --watch=false --browsers=ChromeHeadlessCI
```

From `desktop/`, run `go test ./...` for host tests. To exercise the embedded backend against the desktop module's actual dependency graph, also run:

```sh
go test github.com/fastenhealth/fasten-onprem/backend/pkg/web -run TestEmbedded -count=1 -timeout=120s
```

The native executable supports `--smoke-test --unencrypted --data-dir <fresh-private-directory>`. Smoke mode refuses an existing database or an omitted data directory. It tests the real WebView/Go binding, fresh database API, synthetic signup/sign-in, URL rejection, JSON API errors, popup callback delivery, and isolation from an unrelated window. It prints a `DESKTOP_SMOKE` JSON result, quits automatically, and exits nonzero on failure. Its local test routes do not exist in normal mode.

Observed on October 26, 2026: seven embedded-backend integration tests passed, including encrypted create/unlock/reopen, bad-key rejection, concurrent health polling, persistence, and synthetic FHIR import without outbound HTTP or temporary-document residue. The cancellation regression passed with the real event bus. Desktop host tests and four bridge tests passed. The frontend suite reported 286 successes and one skipped test. The packaged executable passed all eight native smoke checks after extraction and passed strict signature verification. Its encryption onboarding UI was also verified visually; a staged build's actual UI signed into the synthetic account after restart and reached the dashboard.

The full backend suite is **not green**: `TestRepositoryGraphTestSuite/TestGetFlattenedResourceGraph` and `TestResourceHandlerTestSuite` exceeded the 120-second test limit during fixture imports. Those failures were not diagnosed or independently reproduced on an unmodified baseline. Missing generated `related_versions.json` initially prevented two packages from compiling; running the existing targeted generator resolved that setup failure, and both packages then compiled. No timeout or acceptance gate was weakened.

## Restored dependency provenance

The fork requires `github.com/fastenhealth/fasten-sources v0.6.25`, whose original repository and Go proxy archive were unavailable. A surviving public mirror retains release commit `18c9783658fdd109483dac88ae55cb13a4b68408`. The replacement is constrained to that exact original version and pins `github.com/andhines/fasten-sources v0.0.0-20241106031723-18c9783658fd`.

Rehashing the mirror archive using the original module/version prefix produced the exact checksum already recorded by this fork:

```text
h1:PpgI/jkd3a8y9gYMX5tLdmrAXu8b8KhIU97rek88QCY=
```

Its `go.mod` checksum also matched the original. This restores the pinned release's contents rather than substituting a newer provider library. Both root and desktop module files retain the explicit replacement for reproducibility.

`healthit.yaml` was excluded throughout implementation. The installed purchased app and its medical records were not read or changed. Test databases, temporary apps/downloads, and the community app's newly created synthetic WebView cache are disposable test state; they are removed after validation. No changes were pushed.
