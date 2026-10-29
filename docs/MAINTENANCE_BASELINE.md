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

On October 29, a fresh `sh desktop/build.sh` at executable source commit
`32f40d4bf6aa4cdebad38aa83d5bc1f3add1f236` rebuilt the production frontend with
Node 20.19.0 and packaged all subsequent account, initialization, JWX, body-logging,
and Logrus fixes. That checkpoint's artifact had SHA-256
`372c1ffaf13c0e25df1c85a207a7ada455dde235e835b90ed6984b519f87ffaa`.
Archive integrity, extracted-app strict signature verification, and all ten native
smoke checks passed; the process exited 0. Build metadata confirms JWX v3.4.0 and
Logrus v1.10.2. Its modified-worktree flag reflects the preserved unrelated
`.DS_Store` and `.serena` residue; there were no uncommitted source changes.
Existing bundle-budget, CSS/CommonJS, sandbox-extension, and heartbeat warnings
remain. The extracted app and fresh mode-0700 synthetic database directory were
removed after process exit and an open-file check. The smoke run uses an
unencrypted disposable database; encrypted persistence is covered separately by
the desktop-graph integration tests, not by this native smoke mode.

The subsequent package at executable source
`2cce0bcec6cff6dcfea55cf11145b227818c84ba` includes x/net 0.62.0 and the Gin
family. `sh desktop/build.sh --skip-frontend` reused the unchanged production
frontend from the fresh build above. ZIP integrity, strict app signature, and
all ten native smoke checks passed, with smoke process exit 0. Executable build
metadata confirms Gin 1.12.0, protobuf 1.36.12, quic-go 0.59.1, and x/net 0.62.0.
That checkpoint's ZIP SHA-256 is
`bf1ced038983ffe4d6bea8fab0af0a757c5899f7fdb7a233d6b33b71a859cefe`.
The modified-worktree flag again reflects only preserved unrelated residue.
The extracted app and disposable database were removed after exit and open-file
checks. This package predates the following CIRCL work.

The latest retained `dist/Chartrove.zip` was built at executable source
`24134f89171cfaa27f6ba973c5014b18cb21b261` with the same validated frontend
output. Its SHA-256 is
`658155c574f724ceaf3da183ef57855459bba72c1dafffaa842e94c97b14e5d9`.
ZIP integrity, strict app signature, and all ten native smoke checks passed;
the smoke process exited 0. Build metadata confirms CIRCL 1.6.5, Gin 1.12.0,
x/image 0.47.0, and OAuth2 0.37.0. The PDF asset fix is included. Existing
sandbox-extension and heartbeat warnings remain. Uncommitted source was absent
at build time; the modified-worktree flag again reflects unrelated residue.
The extracted app and disposable database were removed after exit and open-file
checks. The original logo, licenses, and healthit.yaml have no diff from the
baseline main commit.

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

The subsequent x/net slice selects v0.62.0 in both graphs, beyond #277's v0.61.0
target. It fixes [GO-2026-6860](https://pkg.go.dev/vuln/GO-2026-6860) and
[GO-2026-6866](https://pkg.go.dev/vuln/GO-2026-6866). No other module version or Go
minimum changes. The upstream comparison also includes socket and QUIC changes;
neither application imports those changed packages. The upstream HTML package
tests pass under both graphs, as do root authentication/web packages, desktop
host tests, and encrypted embedded-backend checks (12 seconds, unchanged
120-second limit). Both module graphs verify. Module-level scans explicitly
include x/net 0.62.0 and report no x/net findings, with 16 root and three desktop
advisory IDs still present. No new exploitation or call-reachability claim is
made. The recorded native ZIP at `32f40d4bf` predates this update.

The Gin slice implements [#263](https://github.com/karamhouse/chartrove/pull/263)
locally with Gin 1.12.0, protobuf 1.36.12, and the required binding/serialization
family in both graphs. It also exceeds historical #200's Gin 1.9.1 target.
The [Gin release](https://github.com/gin-gonic/gin/releases/tag/v1.12.0)
adds BSON binding and HTTP/3 support; application handlers retain explicit JSON
binding, and no HTTP/3 listener is enabled. The MongoDB module supplies Gin's BSON
serializer, not an application database. The required quic-go 0.59.0 initially
introduced GO-2026-5676, so the final selection uses the upstream
[0.59.1 security backport](https://github.com/quic-go/quic-go/releases/tag/v0.59.1).
The obsolete chenzhuoyu/base64x requirement was removed after inspecting both
graphs; the new Sonic family uses cloudwego/base64x. Go minimums and provider,
local-backend, and SQLCipher replacements are unchanged.

After targeted preparation, the full backend command exited 0 with all 14 tested
packages and 290 test/subtest pass events, without test skips or failures. The
database package took 113.513 seconds. Desktop host and privacy tests pass;
encrypted embedded-backend checks, including synthetic multipart FHIR import,
pass in 11.187 seconds under the unchanged 120-second limit. Both module graphs
verify and full `go vet ./...` passes in each. Fresh module scans contain 108 root
and 109 desktop modules, explicitly
select the three versions above, and contain no Gin, protobuf, or quic-go
findings. Fourteen root advisory IDs remain; the desktop's sole remaining ID is
GO-2026-5932 for deprecated `golang.org/x/crypto/openpgp`. Neither production
import graph includes that package. Both do import the distinct ProtonMail
OpenPGP fork; this evidence does not establish the security of that fork or
justify removing other uses of x/crypto. These are module-level scan results,
not a new whole-application reachability assessment.

The CIRCL slice selects 1.6.5 in both graphs, superseding #205's historical
1.3.7 target. Only CIRCL's version and checksums change; Go minimums and the
distinct ProtonMail versions selected by each graph remain unchanged. The
[1.6.5 release](https://github.com/cloudflare/circl/releases/tag/v1.6.5) includes
stricter Ed25519/Ed448 key parsing and point validation beyond the fixes in
1.6.3. Production imports use CIRCL's X25519, X448, Ed25519, and Ed448 through
ProtonMail and the GitHub client. The previously flagged Kyber, FourQ, and P-384
operations were not demonstrated as application exploit paths.

Nine upstream crypto packages pass in each graph: those four CIRCL packages,
Goldilocks, P-384, and ProtonMail ECDH/ECDSA/EdDSA. Root authentication, JWK,
and all web packages pass; desktop host tests and encrypted embedded-backend
checks pass (10.382 seconds, unchanged 120-second limit). Both graphs verify.
Fresh module scans explicitly select CIRCL 1.6.5 and report no CIRCL findings:
11 root advisory IDs and the same one desktop OpenPGP ID remain. The native
package at `2cce0bcec` predates this patch; the later `24134f891` native checkpoint
includes it.

The image slice selects x/image 0.47.0 in both graphs, changing only that module
and its checksums. Its Go 1.26, x/sys 0.49, and x/text 0.43 requirements are
already selected. The server previously selected 0.18.0; the desktop selected
0.45.0. Upstream's 0.45.0-to-0.47.0 comparison changes only go.mod/go.sum.
CCITT, RIFF, and WebP upstream tests pass; imported VP8/VP8L packages compile but
have no standalone test files (WebP exercises their decoding paths). Existing
PDF component-tree and handler tests pass, as do desktop-graph PDF component and
embedded-backend tests (12.363 seconds, unchanged 120-second limit). These
component tests do not establish successful full PDF rendering from an installed
application; a follow-up full-render test exposed a missing source-relative logo.
Both modules verify. Fresh module scans explicitly select x/image 0.47.0 and
report no image findings, leaving OAuth2 and deprecated OpenPGP in the root
graph, and only deprecated OpenPGP in the desktop graph. The package at
`2cce0bcec` predates the CIRCL and image slices; `24134f891` includes them.

The following PDF portability regression renders a synthetic summary from an
empty temporary working directory. Before the fix, generation returned success
but embedded the text `could not load image` instead of the source-relative
Fasten logo. The original JPEG is now embedded in the Go binary and passed to
Maroto's bytes-image constructor; the asset and attribution are unchanged.
The regression checks a complete PDF with an image object and no error text.
Both module graphs pass it and all PDF component tests. Only the header fixture
was regenerated: it now records the JPEG byte prefix/size and its actual height.
Root handler tests and desktop encrypted-backend tests also pass (10.956 seconds
for the latter, unchanged 120-second limit). No real patient data was used.

The OAuth2 slice selects 0.37.0 in both graphs, changing only that module and its
checksums. The Go minimum and provider replacement are unchanged. The server
previously selected 0.11.0; the desktop selected 0.35.0. The difference from
0.35.0 is the Go minimum plus a Google-specific endpoint correction; neither
production graph imports the Google subpackage. GO-2025-3488 affects the JWS
parser before 0.27.0; that subpackage is also absent from both production graphs.

Four synthetic compatibility cases exercise the real provider factory and real
application credential setters against a local TLS server, without enabling the
library's refresh-bypassing test mode. They cover valid-token reuse, rotation
with a changed access token, preservation of an omitted refresh token, and a
rejected grant leaving stored credentials unchanged. The tests also fetch a
synthetic FHIR Patient with the expected bearer token. Remote discovery is
replaced by an explicit local definition. All cases pass on the old root version
and on 0.37.0 in both graphs. They do not establish live provider interoperability
or same-access-token rotation behavior. Upstream OAuth2 and JWS tests pass.

The full combined backend sweep exits 0 with 14 tested packages and 296
test/subtest pass events, no test skips or failures, and the database package
finishing in 134.539 seconds under its unchanged timeout. Desktop host, model,
and encrypted-backend tests pass (10.164 seconds for the latter, unchanged
120-second limit). Both module graphs verify and full vet passes in each.

Fresh full `govulncheck -json ./...` scans after the combined updates include
110 root modules across 30 package roots and 109 desktop modules across seven
package roots. Both report zero called or imported advisories and exactly one
module-only finding, GO-2026-5932 for deprecated x/crypto OpenPGP. The scans
explicitly identify Gin 1.12.0, CIRCL 1.6.5, x/image 0.47.0, and OAuth2 0.37.0.
The absence of package/call traces agrees with the separate production import
inventory. This does not establish whole-application security: frontend
advisories, legacy integration behavior, and unexamined execution paths remain.

## Angular 14 migration checkpoint (2026-10-29)

The application Angular family is aligned at 14.3.0, CLI/build tooling at
14.2.13, CDK at 14.2.7, and TypeScript at 4.8.4. This advances the same-major
work represented by [#234](https://github.com/karamhouse/chartrove/pull/234)
and [#223](https://github.com/karamhouse/chartrove/pull/223). Node 16.20.2
and Yarn 1.22.22 provide the historically supported Angular 14 compatibility
range for this step; neither Node 16 nor Angular 14 is a supported final target.
No global/project runtime configuration was changed.

The global webpack 5.74.0 override is removed so build-angular can select its
required 5.76.1. The independent `webpack@5` lock entry remains 5.74.0, and
transitive Angular `~14.2.12` entries also remain. This is not a complete
webpack or nested Angular graph modernization. Yarn is explicitly selected in
`angular.json`; `corepack yarn exec ng version` confirms the selected versions.

Both migration-only checks exit 0 using the installed CLI and
`NG_DISABLE_VERSION_CHECK=1`. That switch prevents the old CLI from downloading
the latest CLI before executing; it does not force peer compatibility or relax
validation. There are no eligible 14.2-to-14.3 source migrations. The initial
automatic CLI 22 replacement attempts exited before migration because they
required a newer Node runtime; no source changes came from those attempts.

The `desktop_prod` production build exits 0 with a 9.45 MB initial bundle,
unchanged budgets, and hash `46f16b1a59b24b21`. Karma exits 0 with 323 successful
tests and one skip on Chrome Headless 156. The desktop bridge test file also
exits 0 under Node 16. Existing CommonJS, invalid legacy CSS `hover-focus()`,
bundle warning, and incomplete Angular test-fixture declaration diagnostics
remain. No assertion, timeout, or budget was relaxed.

Browser plugin not available; Playwright checked the production assets against
a newly created loopback-only SQLite backend using a synthetic account.
Account creation, dashboard navigation, the record wizard, nested encounter
dialog, Create tab, and date input rendered at 1200x1098 and 390x844. The
desktop bundle reports the expected missing Wails runtime when served over
ordinary HTTP; this is not native-host validation. A separate real defect was
reproduced: an empty encounter count response throws while opening the dialog.
The terminology search returned no result, so record persistence was not
established by this browser check. The standalone server also still performs
legacy Fasten installation registration during startup; fully offline server
startup remains an independence gap. The retained native ZIP above predates
this frontend checkpoint.

Known peer debt remains: ng-bootstrap 10 targets Angular 12 and Bootstrap 4,
and ngx-datatable requires at least RxJS 6.6.3 while the application still selects
6.5.5. Coordinate ng-bootstrap 13, Bootstrap 5.2, Popper, CSS/API adaptation,
and RxJS 6.6 before the Angular 15 step. The fresh-installation count exception
is the next bounded behavior repair. Frontend advisories remain unresolved.

## Node 26 transition and empty record counts (2026-10-29)

The requested development runtime is now Mise Node 26.13.0, pinned in
`mise.toml` and the frontend engine range. The current TypeScript language
server and both tsserver children were restarted and their executable paths
and working directories verified against Node 26.13.0 and this isolated
worktree. Current contributor and desktop instructions no longer prescribe
Node 16 or Node 20. Historical validation observations above remain historical.
No retired runtime was reinstalled to work around compatibility failures.

Node 26 does not bundle Corepack. Invoking Corepack 0.35.0 through `npm exec`
selects the committed Yarn 1.22.22 without changing the global runtime or
installing a global Angular CLI. Angular 14's CLI explicitly warns that Node 26
is unsupported. The [official matrix](https://angular.dev/reference/versions)
lists Node 26 support starting with Angular 22; intermediate framework
migrations run on Node 26 and must report failures rather than conceal them
with an old runtime. Container and disabled workflow configurations remain
separately unvalidated and are not evidence of Node 26 compatibility.

Makefile frontend commands now use the pinned Yarn launcher for installation
and the installed CLI for builds, tests, serving, and Storybook. Build outputs
remain in `dist/`; the production serve target now selects the existing `prod`
serve configuration instead of invoking a nonexistent `yarn dist` script.
Dry-run command expansion and shell syntax pass. This does not claim that
every inherited build/serve/Storybook configuration was executed.

The browser-reproduced empty encounter count error also affected the
organization dialog, practitioner dialog, and practitioner creation page.
Each now defaults absent aggregate results to zero without losing a nonzero
count. Twenty-four HTTP-backed component regressions cover empty arrays, null
or omitted data, missing values, explicit zero, and existing counts across all
four entry points. Before the fix, 13 cases failed, including all four empty
array cases; afterward all 24 pass on Node 26.13.0.

The full Karma run on Node 26 exits 0 with 347 successful tests and one skip.
All five desktop bridge tests pass. The production `desktop_prod` build exits
0 with hash `db282c7e0e7a3f28`, a 9.45 MB initial bundle, and unchanged budgets.
Existing CSS, CommonJS, and test-fixture diagnostics remain. Playwright then
replayed sign-in, Add Condition, and Find/Create Encounter against the same
disposable synthetic database using the rebuilt assets. The empty result
renders without the earlier count exception. Only the expected missing native
Wails module diagnostic remains in this ordinary-browser flow. This does not
establish native packaging or all record persistence paths for the new frontend.

## Bootstrap 5 and Angular 14 widget peers (2026-10-29)

ng-bootstrap 13.1.1, Bootstrap 5.2.3, Popper 2.11.8, and RxJS 6.6.7 now
replace the incompatible Angular 12 widget peers and RxJS 6.5 baseline. This
advances the related work represented by
[#294](https://github.com/karamhouse/chartrove/pull/294),
[#298](https://github.com/karamhouse/chartrove/pull/298), and
[#250](https://github.com/karamhouse/chartrove/pull/250) through compatible
intermediate versions; it does not adopt the ng-bootstrap 21 destination yet.
Templates use Bootstrap 5 spacing, badges, close buttons, and input groups.
Theme Sass no longer calls removed Bootstrap 4 hover/theme helpers, and six
invalid `:hover-focus()` selectors are corrected.

On Node 26.13.0, dependency installation exits 0 without forcing peer or engine
checks. Karma exits 0 with 347 successful tests and one skip. The final
`desktop_prod` build exits 0 with hash `8944fc02ed10b108`, a 9.52 MB initial
bundle, unchanged budgets, and no former invalid-selector warnings. Existing
CommonJS, size, legacy test-fixture, and developer-tool peer warnings remain.

Playwright exercised synthetic sign-in, the record wizard, nested encounter
creation dialog, datepicker selection, dismissal, navigation, dark mode, and
the profile menu. Both 1200x900 and 390x844 views were inspected. Browser
testing caught a theme stacking rule that placed dialogs behind the new
backdrop; removing the two inherited z-index overrides restores normal clicks
using Bootstrap's stacking rules. The final replay used rebuilt assets with
no injected CSS or forced clicks. Both dialogs detach after dismissal.

The browser still reports a nested-dialog focus/aria-hidden warning and the
expected absent Wails runtime when desktop assets run over HTTP. Empty-account
navigation additionally exposes a Medical History request with no resource
IDs (400) and a Labs request with an empty code filter (500). These are concrete
follow-ups, not passing behavior. This check does not establish record saving,
all charts/imports, accessibility conformance, or native-host behavior.

The native package for source `542dc1f91` embeds those validated Bootstrap 5
assets. `dist/Chartrove.zip` has SHA-256
`a7ff1f5cc55365194e711d110014f1857f1c4f47383bcd7917626e62d41f4485`.
ZIP integrity, strict signature verification, and all ten native smoke checks
pass; the smoke process exits 0. Its extracted app and synthetic data directory
were removed after process exit and open-file checks. The smoke page verifies
native integration, not every Angular screen. Existing macOS sandbox-extension
and heartbeat diagnostics remain.

## Empty report request repair (2026-10-29)

Medical History and Labs now finish loading empty pages without sending a
graph request with no IDs or a query with an empty code filter. They also clear
previously displayed results when pagination reaches an empty page. Six
HTTP-backed regressions cover initial empty accounts, empty subsequent pages,
and populated graph/lab requests. Four cases fail before the repair; all six
pass afterward on Node 26.13.0. The initial test compile error was corrected
before this red/green comparison; a zero-test result was not counted as passing.

The production build exits 0 with hash `3b353f75e3b72235` and the unchanged
9.52 MB initial bundle budget. Playwright replays synthetic sign-in followed
by Medical History and Labs navigation. Both empty views render, all eight
observed API responses return 200, and the former 400/500 requests disappear.
Only the ordinary-browser missing-Wails-module error remains in that flow.
The native ZIP above predates this separate page-logic repair.

## Angular 15 migration checkpoint (2026-10-29)

The next sequential step selects Angular 15.2.10, CLI/build tooling 15.2.11,
CDK 15.2.9, TypeScript 4.9.5, Zone 0.12.0, ng-bootstrap 14.2.0, ng-select
10.0.4, angular-fontawesome 0.12.1, and ngx-infinite-scroll 15.0.0. Node
26.13.0 and Yarn 1.22.22 remain selected. Angular's CLI still reports Node 26
as unsupported at this intermediate step. Installation and the CLI/core/CDK
migrations exit 0 without `--force` or ignored engines. The CLI migration
removes the obsolete Karma `require.context` loader and sets ES2022 with
`useDefineForClassFields: false`; application strictness settings are unchanged.

Three concrete build failures were resolved: old and new caniuse data disagreed
on an Opera Mobile target; ngx-datatable 20's CSS files lack package subpath
exports; and the new TypeScript compiler rejects doughnut options inherited
through a generic chart type. The official
[Browserslist updater](https://github.com/browserslist/update-db) synchronizes
the locked browser data. Relative stylesheet file imports preserve their order,
and the widget base now carries the chart type as a generic parameter. Browser
font loading confirms the datatable icon asset resolves. No browser exclusion,
budget increase, or relaxed compiler check was used to hide these failures.

Four new tests render populated donut, dual-gauge, line, and grouped-bar charts
with synthetic data through the actual Chart.js canvases. They verify plotted
geometry and chart disposal, not just component construction. All four pass.
The full suite initially caught a test declaring the now-standalone NgbCollapse;
moving it to test imports fixes that compatibility error. The final full Karma
run exits 0 with 357 successful tests and one skip. All five native bridge tests
also pass on Node 26.

The production build exits 0 with hash `4d242df4e4e261f5`, a 9.54 MB initial
bundle, and unchanged budgets. Playwright replays synthetic sign-in, empty
Medical History and Labs, the record wizard, encounter Find/Create, mobile
datepicker selection, and dismissal of both dialogs. The date becomes
`2026-10-29`; no dialogs remain after dismissal. Only the expected missing
Wails module error remains in this ordinary-browser flow; the prior nested
modal focus warning was not reproduced. These checks do not establish all
record persistence, imports, providers, or accessibility behavior.

Remaining migration constraints include the View Engine-only Circlon tree
widget, Storybook's older Angular peer range, legacy Codelyzer, and lforms 34's
separate Angular 14 dependency family. The tree widget must be replaced or
migrated before dropping the compatibility compiler in Angular 16. Existing
CommonJS, bundle-size, test-fixture, and developer-tool peer diagnostics remain.
No inherited workflow was enabled and no changes were published.

The Angular 15 native package for source `07a0f7605` embeds frontend build
`4d242df4e4e261f5`. The retained `dist/Chartrove.zip` SHA-256 is
`c309bc37706d52b72de4a25fb9e4d8cf56b660412df3e1b47cf0c99d6241be51`.
ZIP integrity, strict signature verification, and all ten native smoke checks
pass, with process exit 0. The extracted app and synthetic smoke database were
removed after process exit and open-file checks. Existing macOS diagnostics
remain; the smoke page is not coverage of every Angular interaction.

## Retired tree editor removal (2026-10-29)

The Circlon tree library's sole consumer was `ReportMedicalHistoryEditorComponent`,
already explicitly marked deprecated and no longer used. A full frontend
reference search found module registration, its creation-only test, unused
imports, and commented-out calls. The active resource-creation and timeline
editing paths both open `MedicalRecordWizardComponent`. Remove the retired
editor, stale references, stylesheet, and the tree/MobX dependencies while
preserving the active wizard and backend composition APIs. Git retains the
removed source and historical authorship; no historical branch is changed.

The remaining full frontend suite exits 0 with 356 successful tests and one
existing skip. The count falls by one because the retired component's sole
creation test is removed with it; no active test is skipped or weakened.
The production build exits 0 with hash `218939895c6e4f84` and a 9.44 MB initial
bundle, approximately 100 KB below the previous step, under unchanged budgets.
No View Engine library warning remains. The Angular 15 native ZIP above
predates this removal. Karma required SIGKILL to finish shutting down its
Chrome child after the assertions; the runner still terminated with exit 0.

## Angular 16 and browser crypto compatibility (2026-10-29)

The next intermediate family is Angular 16.2.12, CLI/build tooling 16.2.16,
CDK 16.2.14, TypeScript 5.1.6, and Zone.js 0.13.3 on Node 26.13.0.
Its UI peers are ng-bootstrap 15.1.2, ng-select 11.2.0, angular-fontawesome
0.13.0, and infinite-scroll 16. Storybook's related packages move together
to 7.6.24 because the previous release's Angular peer range ended at 16.
The official CLI and CDK migrations make no source changes. The core migration
removes deprecated guard interfaces without changing guard logic.
Node 26 remains outside Angular 16's official support range.

Nine new router integration tests exercise real navigation through the three
guards. They check signed-out and non-admin rejection, authorized access,
database setup/unlock redirects, and that rejected navigation never constructs
the protected component. They pass after correcting the test fixture's
synchronous `IsAdmin` return type. This does not cover a failed health request;
the existing encryption-status guard still allows navigation on that error.
Backend authorization and encryption checks remain independently necessary.

The first production build exposed an installed dependency mismatch: a stale
nested `gen-mapping` 0.1.1 shadowed the 0.3.x version required by remapping 2.2.1.
Isolating that exact stale installed copy fixed a small optimizer reproducer
with source maps enabled. No source-map or optimization setting was disabled.

TypeScript 5.1 also exposed WebCrypto's old `randomUUID` return declaration.
Browser regressions then reproduced malformed UUIDs in the bundled polyfill:
one of three tests failed, while the RFC 7636 PKCE vector and AES-GCM
round-trip/tamper rejection passed. Update webcrypto-liner from 1.4.0 to 1.4.3,
which includes the upstream [UUID formatting repair](https://github.com/PeculiarVentures/webcrypto-core/commit/049cd7c9434c6171d8d8f4c83c9aa912fd510d3f).
Its existing Elliptic fork stays at commit
`75637c76678e83c31682fd967c2fa9ff4761b3fc`, explicitly resolved so the package's
changed Git URL does not silently advance that crypto implementation.
This preserves provenance; it is not a security endorsement of the old fork.

The package root's legacy `export import` aliases are incompatible with
webcrypto-core's type-only exports. Load the public browser-shim file directly
for its side effects, retaining the same runtime file selected by the package's
browser field and the native WebCrypto API types. Do not disable library type
checking. The temporary WebCrypto/ASN.1 resolution overrides used during
diagnosis are removed; their compatible versions remain recorded in the lockfile.
The current application does not import their internal type declarations.

The final full frontend suite exits 0 with 368 successful tests and one existing
skip. All three crypto checks now pass, as do all five desktop bridge tests.
The production build exits 0 with hash `0f9c03545569b0b5`, a 9.40 MB initial
bundle, and unchanged budgets, optimization, and source-map settings.
Playwright verifies synthetic sign-in, empty Medical History and Labs, the
record wizard, and mobile encounter date selection (`2026-10-29`) followed by
dismissing both dialogs. Seven observed API responses during the final empty
report navigation are 200. Only the expected missing Wails runtime module error
appears in the ordinary browser. Storybook's own build remains unverified at
this checkpoint. Existing test-fixture warnings and the 2 MB bundle warning
remain; GitHub Actions are still disabled and no changes are published.

The native package for source `0257d4881` embeds frontend build
`0f9c03545569b0b5`. The retained `dist/Chartrove.zip` SHA-256 is
`771d489fba80bf99f873536b282620f16990e5891e24e41c4d30c329dae9bd46`.
ZIP integrity, strict signature verification, and all ten native smoke checks
pass with process exit 0. The extracted app and disposable smoke database were
removed after exit and open-file checks. The existing macOS sandbox/heartbeat
diagnostics remain. These smoke checks use a dedicated page, not every Angular
screen.

## Angular 17 and Bootstrap 5.3 (2026-10-29)

The next intermediate family is Angular 17.3.12, CLI/build tooling 17.3.17,
CDK 17.3.10, TypeScript 5.4.5, and Zone.js 0.14.10. Related UI packages move
to ng-bootstrap 16.0.0, ng-select 12.0.7, angular-fontawesome 0.14.1, and
infinite-scroll 17.0.1. Bootstrap advances to 5.3.8 for ng-bootstrap 16's
documented 5.3 family. Storybook remains at 7.6.24, whose Angular peer range
includes 17 and 18. Node 26.13.0 remains the only runtime used; it is still
outside Angular 17's official support matrix.

Official CLI migrations rename development-server and extraction targets to
`buildTarget`; Storybook retains its own `browserTarget` schema. The core
migration escapes a literal template `@`, preserving the rendered text.
Zone.js imports use the package's public entry points, including `zone.js/testing`.

The first build and test compilation failed in Babel before assertions ran.
A redundant nested helper-compilation-targets 7.29.7 resolved lru-cache 10.4.3
instead of its declared 5.1.1 range. Its 17 package files exactly matched the
root helper, whose own cache dependency was correct. Isolating that exact
installed duplicate fixed a small browser-transform reproducer. A subsequent
frozen-lockfile install with `--check-files` and the same reproducer passed.
This is the second observed stale nested-package problem during incremental
Yarn Classic migration; a clean locked dependency installation remains needed
at the final supported-framework checkpoint to establish reproducibility.
No global cache version override or compiler bypass was introduced.

The full frontend suite exits 0 with 368 successes and one existing skip.
All five bridge tests pass. The production build exits 0 with hash
`98225fefaddd15da`, a 9.45 MB initial bundle, and unchanged source maps,
optimization, and budgets. Browser checks pass for synthetic sign-in, empty
Medical History and Labs, nested encounter creation dialogs, and selecting
`2026-10-29` at a 390-pixel viewport with no horizontal overflow. Both dialogs
dismiss normally. Ten observed API responses are 200; the only console error
is the expected missing Wails runtime in the ordinary HTTP browser.
The retained native ZIP still corresponds to Angular 16, as recorded above.
Storybook's production build also exits 0 after generating Compodoc metadata;
its asset-size and unused-TypeScript-file warnings remain. This verifies the
build, not every story's interactions.

## Angular 18 and HTTP provider migration (2026-10-29)

Advance to Angular/CDK 18.2.14, CLI/build tooling 18.2.21, TypeScript 5.5.4,
ng-bootstrap 17.0.1, ng-select 13.9.1, angular-fontawesome 0.15.0, and
infinite-scroll 18.0.0. Zone.js remains 0.14.10. The optional application-builder
migration is deferred so this checkpoint retains the existing desktop asset
layout and webpack configuration. Node 26.13.0 remains in use, still outside
Angular 18's official support matrix.

The official HTTP migration replaces module-based registration with
`provideHttpClient(withInterceptorsFromDi())` and the matching testing provider.
Seven new tests import the actual AppModule and verify both HTTP-client tokens:
local API requests receive the synthetic authorization token, unrelated origins
do not, API 401/403 responses end the session and redirect, and a 500 remains
an error without logging out. These tests exercise the registered interceptor
chain rather than constructing the interceptor directly.

Test compilation initially exposed six remaining imports of Angular's removed
`async` helper. Replace those with `waitForAsync`, leaving JavaScript async
functions intact. An editing-tool scope error briefly changed additional files;
all 275 accidental replacements were reversed before the targeted repair.
Git verified that backend, desktop, Go modules, bridge tests, and `healthit.yaml`
had no remaining changes, and both build and full tests were rerun afterward.

The final frontend suite exits 0 with 375 successes and one existing skip.
All five bridge tests pass. The production build exits 0 with hash
`4d92d8d86bcc397d` and a reported 9.91 MB initial bundle under unchanged budgets.
Angular 18 reports decimal units, so this figure is not directly comparable
with the preceding CLI's size display. Browser checks pass for synthetic
sign-in, empty reports, nested encounter dialogs, mobile date selection, and
dialog dismissal; 24 observed authenticated API responses are 200. The mobile
viewport has no horizontal overflow. A browser accessibility warning about
focus retained inside an aria-hidden nested modal remains to investigate.
The expected missing Wails module error occurs only in this ordinary HTTP
browser check. Native packaging and Storybook were last verified on Angular 16
and 17 respectively; neither is represented as validated on Angular 18 yet.

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
   then migrate one major at a time. Angular 14.2's TypeScript ceiling was below
   4.9, so the standalone TypeScript 4.9 PR was ineligible on that baseline;
   TypeScript advances only with its compatible Angular family. Keep the explicitly
   requested Node 26.13.0 runtime through sequential source migrations toward
   Angular 22, the first version with documented Node 26 support. Report
   intermediate compatibility failures without reinstalling retired runtimes.
   Verify forms, charts, imports, and native bridge behavior at each step. Follow the official
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
