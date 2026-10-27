# Chartrove branding validation — October 27, 2026

The README, contributor guide, desktop documentation, app bundle, icon, window/menu names, page titles, visible application copy, and project support links now use Chartrove. Fasten Onprem authorship, license, copyright notices, source history, and in-app attribution remain intact.

## Verified

- Frozen Yarn install with the existing lockfile under Node 20.19.0; no dependency versions changed.
- Angular suite: 302 passed, one skipped. Fixed strict TypeScript callback field access and made the asynchronous UI test await its connection handler without changing its assertions.
- Production desktop frontend build succeeded. Inherited CommonJS, CSS-selector, and bundle-size warnings remain.
- Desktop Go module tests passed with downloads disabled; four bridge adapter tests passed.
- Shell syntax and property-list checks passed; generated C monogram icon inspected.
- Final ZIP extracted successfully and passed strict signature verification.
- Final native smoke test passed all eight checks: Go/WebView binding, fresh database API, synthetic signup, synthetic sign-in, unsafe-URL rejection, JSON API errors, popup callback delivery, and isolation from an unrelated window.
- Actual Angular UI inspected against disposable synthetic data: Chartrove sign-in, dashboard, Sources/Epic sandbox card, profile menu, support dialog, and Fasten attribution. Final package page metadata and default dashboard description report Chartrove. Quit Chartrove from the application menu exited successfully.
- Secrets scans passed for changed sources. Protected health configuration and existing health records were excluded from inspection and mutation.

Artifact: `dist/Chartrove.zip` (ad hoc signed, not notarized).
SHA-256: `10fe2c5472dfbe718f0faa42b79a6fb5fad708aaeaf4474dbd450146b8c45816`.

## Workspace relocation

The same checkout was relocated outside iCloud to `/Users/osama/Code/karamhouse/fasten-onprem`. Before removing the original, 24,055 files and one Finder resource fork were verified byte-for-byte, with matching permissions, Git HEAD, refs, and working source patch. Full history, prior desktop work, and unrelated Finder files were preserved. Only verified ignored dependencies, caches, and generated frontend output were rebuilt. Origin points to `karamhouse/chartrove`; the original fork remote is retained as `legacy-fork`.

## Limits

Production provider connections, notarization, and distribution readiness are not established. The restored provider UI passed mocked tests; the full Epic OAuth flow in that UI was not repeated during this branding validation. Earlier isolated sandbox evidence remains in PROVIDER_VALIDATION.md. An earlier keyboard-quit automation attempt left a test process running; the final package exited normally through its Quit Chartrove menu item.

Database names, data-directory selection, bundle identifier, encryption, migrations, provider identifiers, and bridge event names are unchanged. No `healthit.yaml` file or existing health-record database was read, copied, edited, or used as a test input. No remote publication was performed.

## PR 219 review follow-up

The sections above describe the earlier branding package. Review found and fixed three issues before merge:

- Repository Close returned without closing its SQL pool. It now closes the underlying pool, including failed initialization cleanup. Fresh SQLite and embedded-host regressions verify that the pool rejects further use after shutdown; repeated close remains safe.
- The pinned manual importer stores Epic's public synthetic Patient ID with a legacy leading-character trim. Repeat-import detection now recognizes both the raw and stored forms, without rewriting source identifiers. The new frontend regression failed before the fix; an actual embedded import verifies the pinned storage behavior.
- Finder metadata could make Angular fail to remove previous generated output. Packaging now builds into an empty temporary directory, then synchronizes generated files while preserving Finder metadata and removing stale bundles.

Provider documentation now distinguishes the isolated live protocol evidence from the restored Angular UI and uses the Chartrove executable path. Independent backend and desktop reviewers accepted the fixes with no outstanding critical or important findings.

Validation of the reviewed sources:

- Backend package coverage completed: the aggregate rerun passed every package except database, which reached the existing ten-minute limit under concurrent load. The database-only rerun then passed in 505.397 seconds with the same default timeout. No timeout, assertion, encryption, or password-cost settings were relaxed.
- Focused race checks for Close, concurrent unlock/health polling, and configuration isolation passed: web 49.147 seconds and database 3.296 seconds, with a 120-second limit. A broader authentication race experiment reached that limit inside the existing bcrypt password check; it reported no race.
- Angular: 303 passed and one skipped, with a clean exit using standard ChromeHeadless and coverage disabled. The earlier custom-launcher run completed all assertions but stalled during browser shutdown. Existing shallow-fixture console warnings remain.
- Four bridge adapter tests passed.
- Normal desktop packaging succeeded with Finder metadata present. Only the current main JavaScript bundle remained after synchronization. Shell syntax and diff whitespace checks passed.
- The final ZIP passed strict signature verification after extraction and all eight native smoke checks using a fresh synthetic database, including synthetic signup/sign-in and callback isolation.

Reviewed package SHA-256: `34738e2fd303dae2283a8838ecac7a06b1e276e57e86b45ff311d5775396d8e9`.

This remains an ad hoc signed development build, not notarized. Full OAuth through the restored Angular UI and production provider access remain unvalidated. Fasten attribution, license, and history are preserved. No protected health configuration or existing health-record data was used. Publication and merge state are recorded by PR 219 itself.
