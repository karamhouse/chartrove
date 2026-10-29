# Maintenance baseline

This records independent checks on October 28–29, 2026, starting at main
`de7818a9174b24884ad55ce81ad21a5f3a2a6f8b`. GitHub remains the issue and PR
ledger; this document describes reproduction and technical decisions, not a
second backlog.

The dated [backlog review](MAINTENANCE_BACKLOG_REVIEW.md) retains the complete
classification for review before publication to GitHub.

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
| Exact username lookup | A real, in-memory repository reproduced HTTP 200 for empty and omitted usernames supplied with the first account's password. An explicit parameterized condition fixes the omitted zero-value filter. Both module graphs pass six repository-backed sign-in cases and the existing six handler cases. The full root handler suite passed in 53 seconds; five existing repository lookup/current-user cases passed in 19 seconds. |
| Auth vulnerability scan after JWT upgrade | No reachable vulnerabilities in `./backend/pkg/auth`; dependency-level findings elsewhere remain. This is not a clean whole-application security assessment. |
| Full root vulnerability scan after JWT upgrade | 14 reachable advisories remain, involving pgx, x/net, x/text, x/image, and circl. No JWT findings remain. |
| Go network/crypto family | Root and desktop module verification passed. The root package sweep passed its test assertions but failed three packages on newly enforced nonconstant-format vet checks. After replacing those three print calls, the affected packages passed and full `go vet ./...` exited 0. Desktop packages passed after adding the new transitive checksums; embedded backend tests passed in 45 seconds under the unchanged 120-second limit. |
| Vulnerability scans after network/crypto upgrade | Root: six called findings; desktop: three. All three desktop called findings concern pgx. The root's additional CIRCL and WebP traces end at initialization, which does not demonstrate use of their vulnerable operations. No called JWT, x/net, or x/text findings remain. |
| Database fixture cleanup | Full database and handler packages passed in 127 and 29 seconds. No database, WAL, or shared-memory files remained in their private temporary directory. One `related.json` upload temporary file remained and led to the separate handler fix below. |
| Related-resource upload cleanup | New regressions reproduced retained upload files after both a successful import and an early source-lookup failure. The handler now closes and removes its temporary upload on every return after storage. The full backend suite passed, including the handler package in 58 seconds; the private fixture directory was empty afterward. |
| pgx security update | The dollar-quoted placeholder regression failed on pgx 5.4.3 in simple-protocol mode and passed on 5.9.2 in both graphs. Normal parameter binding passed before and after. Both module graphs verified, full backend tests and vet passed, desktop unit tests passed, and encrypted embedded-backend tests passed in 57 seconds under the unchanged 120-second limit. The full backend database package took 519 seconds, within its unchanged default timeout. |
| Vulnerability scans after pgx upgrade | No pgx findings at any reported level. Three called findings remain in the root graph, all initialization-only CIRCL/WebP traces; the desktop has zero called findings. Both graphs still have imported/required dependency advisories, including Gin, jwx, protobuf, Logrus, x/crypto, and x/net. Zero called findings is not a clean dependency audit or proof of exploit absence. |
| Portable FHIR date columns | Both graphs pass SQLite migration/reopen tests preserving existing dates, time zones, and nulls. Both also pass fresh PostgreSQL migration, synthetic account/date persistence, reopen, and the driver regressions. PostgreSQL integration took 53 seconds in the root graph and 40 seconds in the desktop graph. Desktop encrypted-backend checks passed in 48 seconds under the unchanged 120-second limit; vet passed for both changed Go packages. |
| Broader checks after the date-column change | The model package passed in 32 seconds. The full database package hit its unchanged ten-minute timeout near the end of the SQLite suite, with no assertion failure reported. All five unfinished groups then passed in 43 seconds with no skips, including the full SQLite encryption/reopen suite. The whole-package run remains a timeout; the cause of its longer cumulative runtime is unverified. |
| PostgreSQL startup | Disposable tests reproduced connection credentials in application Info logs and creation of an empty-password placeholder account that interfered with first-user setup. Startup now omits the connection string and does not create that account. Both graphs pass fresh/reopened account counts, date persistence, and parameter-binding regressions in about ten seconds each. This tests successful connection logging, not every driver error message. |
| Legacy account migration | Replaying migration `20240212142126` on disposable data reproduced deletion of a password-bearing account named `admin`. The deletion now targets only the historical empty-password placeholder. Both SQLite module graphs preserve real account IDs, hashes, and roles while still removing the placeholder. Both PostgreSQL graphs preserve the real admin account and credentials; their integration suites passed in 25 seconds. Desktop encrypted-backend checks passed in 27 seconds within the unchanged 120-second limit. |
| Full backend after account fixes | `go test -json -p 1 ./backend/... -count=1` exited 0: 14 tested packages, 265 test/subtest pass events, no test skips or failures. The database package took 252 seconds and handlers took 40 seconds. No timeout was changed; the earlier broad-run timeout remains unexplained. |
| Initialization failures (October 29) | A disposable schema collision reproduced SQLite and PostgreSQL startup succeeding after final FHIR migration failed. The error now propagates. PostgreSQL also closes its pool after migration or job-recovery failure; tests verify zero remaining connections for each failed initialization. Both module graphs pass the regressions and existing PostgreSQL persistence checks. SQLite recovery preserves an unrelated synthetic patient. The full root database suite passed in 411 seconds under the unchanged ten-minute timeout; desktop encrypted-backend checks passed in 38 seconds under the unchanged 120-second limit. |
| JWX v3 migration (October 29) | Red regressions reproduced custom-field injection and wrong-key-type panics on v2.0.11. They pass on v3.4.0 in both graphs, as do legacy key round trips and independently verified RS256 assertions. Module verification and full vet pass in both graphs. The full backend sweep exited 0: 14 tested packages, 274 test/subtest pass events, no test skips or failures. The database package took 600.404 seconds during high system load; no timeout was changed. Desktop host tests pass with existing linker warnings, and encrypted embedded-backend checks pass in 74 seconds under the unchanged 120-second limit. |
| Vulnerability scans after JWX migration | govulncheck 1.9.0, database updated October 27: no JWX findings at any reported level in either graph. Root: three called advisories, all initialization-only CIRCL/WebP traces; nine advisories at imported-or-called level, 19 across all reported levels. Desktop: zero called, four imported, six across all levels. Imported/required Gin, Logrus, protobuf, x/crypto, x/net, and root OAuth2 findings still require review. JSON scan exit 0 is not a clean vulnerability result. |
| HTTP body logging (October 29) | Synthetic tests reproduced credential/FHIR bodies in debug/trace logs and eager request consumption even at Info level. The shared HTTP middleware no longer copies or logs bodies. It preserves the original request reader/closer, handler responses, and status/method/path/length metadata. All root web packages pass (web 25 seconds, handlers 61 seconds); desktop privacy and encrypted embedded-backend checks pass (24 seconds for the latter, with the unchanged 120-second limit). Focused race tests and middleware vet pass. This does not sanitize separately logged errors, headers, or route metadata. |
| Frontend production dependency audit | Yarn reported 97 affected dependency paths and 65 distinct advisories, including three critical paths involving `@babel/traverse` and `elliptic`. The command exited nonzero. These are dependency findings, not demonstrated application exploits. |

The packaged desktop smoke checks cover the native binding, direct SMART rejection
without registration, fresh database API, local account creation and sign-in,
native URL validation, API/SPA error separation, and callback ownership/isolation.
They do not establish live provider interoperability. After the pgx, upload-cleanup,
and portable-date updates, `sh desktop/build.sh --skip-frontend` at source commit
`db0a65899` reused the unchanged,
validated frontend output. ZIP integrity, extracted app strict signature verification,
and all 10 native smoke checks passed again. The tested ZIP SHA-256 is
`0dfcb42d8f2db8b86d992d82d45a22064dc027b91c1f741265e676c6f8146db1`.
This artifact predates the exact-username lookup change.
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
See CONTRIBUTING for reproduction. A separate startup probe reproduced the exact
`datetime` / SQLSTATE 42704 failure reported in
[#109](https://github.com/karamhouse/chartrove/issues/109), originally reported
upstream by kehralexander. The following model-generator slice fixes that startup
defect by letting the driver infer date-column types from `time.Time`. SQLite keeps
`datetime`; PostgreSQL uses `timestamptz`. Generation no longer reads application
configuration. All 56 regenerated files were compared against their previous
contents: the only changes remove 123 hard-coded date type tags, preserving the
search-runtime pooling and every other generated statement. This does not establish
coverage of all PostgreSQL resource queries or migration of a production database.

The JWX slice selects v3.4.0 in both module graphs. The old v2.1.7 Renovate
target in #268 is insufficient: the upstream
[v3.3.0 security release](https://github.com/lestrrat-go/jwx/releases/tag/v3.3.0)
fixes GHSA-4cf7-xm37-g63h and identifies v2 as unmaintained. Before upgrading,
synthetic tests reproduced JSON field-name injection and a panic when a public
or symmetric JWK was deserialized as an RSA private key. The migration uses the
v3 import/getter/algorithm APIs and returns an error for the wrong key type.
Existing serialized RSA fixtures still round trip. Dynamic-client assertions
retain RS256, key IDs, issuer/subject/audience, UUID IDs, and a two-minute expiry;
an independent JWT implementation verifies them and rejects altered claims or
the wrong key. No provider connection or real credential is used by these tests.

Required JWX family changes include blackmagic 1.0.4, dsig 1.4.0, httprc/v3
3.0.6, option/v2 2.0.0, secp256k1/v4 4.4.1, go-json 0.11.2, asm 1.2.1,
fastjson 1.6.10, and Testify 1.12.1 with go.yaml.in/yaml/v3 3.0.5. Three
obsolete v2 helper requirements were removed after checking both module graphs.
The Go minimum, SQLCipher, Wails, OAuth2, and provider replacements are unchanged.

The next slice implements [#271](https://github.com/karamhouse/chartrove/pull/271)
locally by updating Logrus from 1.9.0 to 1.10.2 in both graphs. Only that module's
version and checksums change. Upstream's
[v1.10.0 notes](https://github.com/sirupsen/logrus/releases/tag/v1.10.0) document
formatting and concurrency changes, including byte-slice text output and generic
PanicLevel calls; these must not be mistaken for an application logging-policy
change. This maintenance slice retains existing logger configuration and tests
body privacy, access metadata, and status handling through the application.
Authentication and all root web packages pass, desktop host/middleware and
encrypted-backend tests pass, and module verification and full vet pass in both
graphs. Existing native linker warnings remain.

Fresh govulncheck 1.9.0 module scans from `backend/cmd/fasten` and
`desktop/cmd/fasten-desktop` include 104 and 105 modules respectively and explicitly
identify Logrus 1.10.2. GO-2025-4188 is absent; 18 root and five desktop advisory IDs
remain. These are module-level results, not new call-reachability analyses. Run
`govulncheck -scan=module -json` without package patterns from those source
directories: this scanner version rejects patterns in module mode and fails at
the source-free repository/module roots.

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
