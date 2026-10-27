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
