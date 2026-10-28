# Maintenance baseline

This records independent checks on October 28, 2026, starting at main
`de7818a9174b24884ad55ce81ad21a5f3a2a6f8b`. GitHub remains the issue and PR
ledger; this document describes reproduction and technical decisions, not a
second backlog.

## Environment and preparation

The isolated macOS arm64 worktree used Go 1.27.3, Node 20.19.0, and Yarn
1.22.22. Its tracked files were restored from sparse checkout without changing
the original checkout. No Nix shell was used. Root and desktop Go modules remain
separate. The desktop uses Wails 3.0.0 and the original exact-version SQLCipher
and fasten-sources mirror replacements.

The frozen frontend lock resolves Angular core 14.2.12, CLI 14.2.10, TypeScript
4.6.4, and RxJS 6.5.5. Node 20 is the documented fork baseline but falls outside
Angular 14's original supported Node versions. This is an existing compatibility
debt, not a claim of upstream support.

Install frontend dependencies using the frozen command in CONTRIBUTING, then
run `make prepare-backend` before backend checks. A fresh checkout otherwise
fails to compile the executable because `related_versions.json` is absent.
Preparation must not run a broad dependency update or vendor operation.

Use a private OS temporary directory for `TMPDIR` while testing. Database-suite
helpers now use test-scoped directories, close initial file handles, and close
successful repository pools before directory cleanup. Keep synthetic fixtures
inside the task directory and remove leftovers after all tests exit. Never use
the default desktop data directory for smoke validation.

## Independently observed checks

| Check | Result and limit |
| --- | --- |
| Root `go mod verify` | Passed before dependency edits. |
| Root `go vet ./...` | Passed after the JWT slice. |
| Desktop `go test ./...` | Passed; linker emitted macOS deployment-target warnings. |
| Embedded backend under desktop graph | Passed with the existing 120-second limit, both before and after the JWT slice. Covers encrypted reopen, wrong-key rejection, account persistence, synthetic FHIR import, and shutdown. |
| JavaScript desktop bridge | Five tests passed. |
| Frontend frozen install | Completed without lockfile edits; inherited peer-dependency warnings remain. |
| Frontend Karma suite | A diagnostic rerun using the existing configuration plus debug logging exited 0: 323 successful tests, one skipped, browser process exit 0, and cleanup complete. The launcher still warned that shutdown exceeded two seconds. Earlier attempts disconnected or failed to exit cleanly; their cause remains unverified. No timeouts or assertions were relaxed. |
| Full backend suite | `go test -json -p 1 ./backend/... -count=1` exited successfully: 14 tested packages, 246 test/subtest pass events, no test skips. The database package completed in 380 seconds under the unchanged default timeout. An earlier default-parallel run failed in that package and its diagnostic output exceeded capture; the cause remains unverified. The isolated graph suite also passed in 32 seconds. |
| Packaged macOS desktop | `sh desktop/build.sh` completed, strict signature verification passed, and the packaged executable's native smoke suite passed all 10 checks using a fresh disposable database. Existing bundle-budget, CSS, and CommonJS warnings remain. |
| Root vulnerability scan before JWT upgrade | govulncheck 1.9.0 reported 16 reachable advisories across six modules, plus findings in imported/required dependencies not shown as called. This is static reachability, not proof of exploitation. |
| JWT regression slice | New tests reproduced acceptance of empty verification keys and panic on missing/null expiry. Fixed unit and HTTP middleware tests pass in both graphs; targeted race tests pass. |
| Sign-in rejection slice | Six synthetic cases cover valid sign-in, absent/nil users, wrapped not-found errors, wrong passwords, and database failures. The full root handler suite passed in 58 seconds; desktop-graph sign-in tests and embedded integration tests also passed (77 seconds for the latter, within the unchanged 120-second limit). |
| Auth vulnerability scan after JWT upgrade | No reachable vulnerabilities in `./backend/pkg/auth`; dependency-level findings elsewhere remain. This is not a clean whole-application security assessment. |
| Full root vulnerability scan after JWT upgrade | 14 reachable advisories remain, involving pgx, x/net, x/text, x/image, and circl. No JWT findings remain. |
| Go network/crypto family | Root and desktop module verification passed. The root package sweep passed its test assertions but failed three packages on newly enforced nonconstant-format vet checks. After replacing those three print calls, the affected packages passed and full `go vet ./...` exited 0. Desktop packages passed after adding the new transitive checksums; embedded backend tests passed in 45 seconds under the unchanged 120-second limit. |
| Vulnerability scans after network/crypto upgrade | Root: six called findings; desktop: three. All three desktop called findings concern pgx. The root's additional CIRCL and WebP traces end at initialization, which does not demonstrate use of their vulnerable operations. No called JWT, x/net, or x/text findings remain. |
| Database fixture cleanup | Full database and handler packages passed in 127 and 29 seconds. No database, WAL, or shared-memory files remained in their private temporary directory. One `related.json` upload temporary file remained and led to the separate handler fix below. |
| Related-resource upload cleanup | New regressions reproduced retained upload files after both a successful import and an early source-lookup failure. The handler now closes and removes its temporary upload on every return after storage. The full backend suite passed, including the handler package in 58 seconds; the private fixture directory was empty afterward. |
| pgx security update | The dollar-quoted placeholder regression failed on pgx 5.4.3 in simple-protocol mode and passed on 5.9.2 in both graphs. Normal parameter binding passed before and after. Both module graphs verified, full backend tests and vet passed, desktop unit tests passed, and encrypted embedded-backend tests passed in 57 seconds under the unchanged 120-second limit. The full backend database package took 519 seconds, within its unchanged default timeout. |
| Vulnerability scans after pgx upgrade | No pgx findings at any reported level. Three called findings remain in the root graph, all initialization-only CIRCL/WebP traces; the desktop has zero called findings. Both graphs still have imported/required dependency advisories, including Gin, jwx, protobuf, Logrus, x/crypto, and x/net. Zero called findings is not a clean dependency audit or proof of exploit absence. |
| Frontend production dependency audit | Yarn reported 97 affected dependency paths and 65 distinct advisories, including three critical paths involving `@babel/traverse` and `elliptic`. The command exited nonzero. These are dependency findings, not demonstrated application exploits. |

The packaged desktop smoke checks cover the native binding, direct SMART rejection
without registration, fresh database API, local account creation and sign-in,
native URL validation, API/SPA error separation, and callback ownership/isolation.
They do not establish live provider interoperability. After the sign-in and Go
network/crypto updates, `sh desktop/build.sh --skip-frontend` reused the unchanged,
validated frontend output. ZIP integrity, extracted app strict signature verification,
and all 10 native smoke checks passed again. The tested ZIP SHA-256 is
`b7ca212875aa4a616e7c8f7ce69f0a57ff2ba7195fce25b98fb9361db56e9074`.
The OAuth classic-script fix is included; generated HTML no longer includes the
standalone ES module as a classic script. The native run emitted a sandbox-extension
warning but exited 0 with all checks successful.

The successful frontend diagnostic command used the documented Node 20.19.0
runtime and `ng test --watch=false --progress=false --browsers=ChromeHeadlessCI
--reporters=dots`. A temporary Karma wrapper preserved the repository config,
set `basePath` to the frontend directory, and enabled `config.LOG_DEBUG`.
The isolated one-test browser lifecycle check also exited successfully.
Inherited template-import warnings and unmocked 404 responses remain test-quality
debt; the passing suite does not establish coverage of every UI behavior.

During follow-up checks, the shared Go build cache lost files. Rebuilding with
`GOCACHE` inside the private task directory avoided depending on that cache, but
the first link then failed with `no space left on device`. Check available space
before rebuilding; an isolated cache consumes additional disk space. Reclaim
only completed task-owned outputs, not other checkouts or user data.

GitHub's repository-wide Actions permission is disabled. Six inherited workflows
remain configured, including upstream cloud, CDN, container, and release
publication. Their configured `active` state does not mean they ran. No CI result
is inferred from their absence, and no workflow was enabled during this work.

The source review found these enablement blockers:

| Workflow | Required decision or repair before enablement |
| --- | --- |
| Development | Replace the mutable Packagr container with a tested toolchain, review broad generation, validate the CGO-disabled build separately from SQLCipher runtime behavior, and add desktop coverage. |
| CI | The coverage download requests `coverage`, while Development uploads `frontend-coverage` and `backend-coverage`. Review artifact compatibility and separate Codecov/Chromatic publication from validation. |
| Cloud-Deploy | Targets the upstream Fasten sandbox AWS role, bucket, and environment. Requires an independently owned deployment design. |
| CDN-Deploy | Pushes to `fastenhealth/cdn` through a mutable third-party action. Requires a Chartrove asset ownership decision. |
| Docker | Publishes images using an inherited Depot project and Fasten image metadata. Review builder ownership, permissions, provenance, and runtime checks. |
| Release | Uses mutable Packagr actions and an app token to commit and publish; its test step is commented out. Requires a reviewed release process with validation. |

The two previously skipped JWT tests have been replaced with synthetic user and
access-token round trips and rejection cases. Validation retains HS256 and
existing token metadata. Both module graphs select JWT v4.5.2, which fixes
[the parser allocation advisory](https://github.com/golang-jwt/jwt/security/advisories/GHSA-mh63-6h87-95cp).
Empty-key rejection does not resolve all of issue #52: server default-key
enforcement remains a separate concern.

The sign-in slice returns the same 401 body for missing accounts and incorrect
passwords without echoing the username. Unexpected database errors remain 500.
It does not claim constant-time account lookup or resolve every missing-user path
in issue #103.

The Go network/crypto update selects x/net 0.61.0, x/crypto 0.58.0, x/text 0.43.0,
x/sys 0.49.0, and x/term 0.47.0 in both graphs. Their common minimum raises the
root module to Go 1.26.0; validation used Go 1.27.3. The root's x/mod moves to
0.41.0 because x/text requires it. The desktop's SQLCipher, local backend, and
provider-library replacements remain intact. This is a targeted family update,
not a general module refresh.

After the network/crypto update, remaining called advisory IDs were GO-2024-2567, GO-2024-2606, and GO-2026-5004
(pgx, both graphs), plus GO-2025-3754, GO-2026-4550 (CIRCL), and GO-2026-5061
(WebP) in the root graph. The CIRCL advisory entries are unreviewed and module-wide;
their initialization-only traces require further assessment. No scan result here
establishes that the application is free of vulnerabilities.

The following pgx slice selects 5.9.2 in both graphs. Its required transitive
changes are pgservicefile, puddle/v2 2.2.2, x/sync 0.24.0, and root Testify 1.11.1
(already selected by the desktop). GORM, PostgreSQL driver, SQLite, and the
SQLCipher replacement versions are unchanged. The opt-in test uses a pinned
PostgreSQL 17 container, synthetic parameters, loopback networking, an in-memory
data directory, and a generated password. Cleanup removed every test container.
See CONTRIBUTING for reproduction. A separate startup probe reproduced issue
#109's operational gap: generated FHIR columns use SQLite's `datetime` type,
which fails on PostgreSQL with SQLSTATE 42704. Passing driver regressions do not
establish full PostgreSQL application support.

## Dependency migration order

1. Establish effective authentication regressions and take the JWT v4 patch in
   both modules. Then adapt the reproduced classic-script OAuth fix from
   [#208](https://github.com/karamhouse/chartrove/pull/208), preserving David
   Radcliffe's authorship and original commit reference.
2. Use serialized backend package scheduling for the reproducible baseline and
   investigate the default-parallel failure separately. Preserve evidence of
   frontend process exit as well as assertions. Audit inherited workflows by purpose and
   permissions before proposing any CI enablement.
3. Prioritize reachable Go security findings by compatible family: x/net,
   x/text, and x/crypto with a supported common Go minimum; pgx with synthetic
   database tests; jwx/jose/OAuth with token and redirect rejection tests.
4. Preserve the local desktop `fasten-onprem => ..` replacement and the recovered
   provider-library provenance. Database-family upgrades must prove encryption,
   migration, reopen, and import behavior; never bypass the version-specific
   SQLCipher replacement by updating its original module version alone.
5. Align Angular core/compiler/CLI/CDK and UI peers within the current major,
   then migrate one major at a time. Angular 14.2's TypeScript ceiling is below
   4.9, so the standalone TypeScript 4.9 PR is ineligible. Select Node from each
   destination's supported range and verify forms, charts, imports, and native
   bridge behavior at every step. Follow the official
   [version matrix](https://angular.dev/reference/versions) and
   [sequential update policy](https://angular.dev/reference/releases).
6. Handle independent library majors and developer tooling after framework
   alignment. Container/devcontainer/runtime updates and publication workflows
   need their own validation; changing action versions is not a workflow audit.

Same-major updates are candidates, not automatically safe. In particular,
pre-1.0 libraries, FHIR parsers, cryptography, database drivers, and JavaScript
engines require behavior-specific checks. Historical draft PRs retain original
bases and reviews; assess their actual diff before adopting any source change.

## Medplum decision boundary

After local import, family authorization, encrypted persistence, and repeatable
checks are reliable, evaluate an optional FHIR adapter against the existing
Medplum deployment using synthetic records. Decide authoritative ownership,
identifier mapping, provenance, consent/access control, offline synchronization,
conflict handling, and rollback before any production write or data migration.
This maintenance work does not authorize a production migration or a competing
new data store.
