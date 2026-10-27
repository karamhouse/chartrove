# Desktop revival implementation plan

**Goal:** Run this fork in a native macOS Wails host and validate an isolated fresh database.

**Architecture:** Keep the fork's current Angular frontend. Add an embedded backend lifecycle with no TCP listener or mandatory hosted registration, then mount it in a pinned Wails v3 application. Replace historical Wails globals with a typed adapter using the runtime served by the host.

**Spec:** The user-approved desktop revival assessment accompanying this checkout, and the user's instruction to rebuild the Go host/bridge, leave `healthit.yaml` untouched, and test a fresh database.

**Constraints:** Preserve existing app/data and unrelated changes. Never read, scan, modify, or copy `healthit.yaml`. Do not push. Native Git owns the local task branch. Tests use temporary databases and synthetic accounts only.

## Tasks

- [x] Restore the unavailable pinned source dependency from a surviving public mirror only after verifying its original module checksum; record provenance.
- [x] Write failing backend integration tests for fresh database startup, registration-free local use, account persistence after restart, encryption unlock and incorrect-key failure, API/static routing separation, shutdown and canceled streams.
- [x] Add `web.NewEmbeddedApplication(config.Interface, *logrus.Entry, event_bus.Interface, http.Handler)` with `ServeHTTP`, `UnlockDatabase`, `Reinitialize`, and `Close`; publish new routes only after initialization succeeds. Adapt the encryption handler for synchronous embedded unlock while preserving the standalone route. Avoid logging database keys.
- [x] Write failing desktop boundary tests for per-user/explicit data paths, single-instance data ownership, URL validation, and generated key persistence. Implement desktop configuration/service helpers and the Wails host in `desktop/` with its own pinned Go module.
- [x] Write failing bridge behavior tests, add a typed runtime adapter, and update external-link and callback callers. Keep browser behavior and callback state matching. Build the existing `desktop_prod` frontend profile.
- [x] Build the native host and app bundle, run tests and scoped race checks, launch with a new temporary database, and verify UI/bridge behavior, account restart persistence, and lack of an application TCP listener. Record any upstream test/build failures separately.
- [x] Review the patch, scan changed files, and document build/run commands and acceptance evidence. Final commit, archive identity, and temporary-state cleanup receipts are recorded by the delivery pipeline outside the source checkout, after the source commit exists.

## Review focus

- A wrong encryption key must retain the locked handler and never create a successful unlock response.
- Closing the application must cancel pending streams and release the database.
- API 404s must remain JSON errors, never Angular's index page.
- Hosted URLs must not be a required startup dependency or receive synthetic test data.
- A second instance must not concurrently migrate or open the same desktop database.

The recovered UI is reference evidence only; current fork behavior remains the implementation base. Provider OAuth, hosted conversion, signing, and existing-database migration are separate acceptance work.
