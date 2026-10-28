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

Use a private OS temporary directory for `TMPDIR` while testing. Existing
database-suite helpers can retain temporary databases. Keep synthetic fixtures
and database files inside the task directory and remove them after all tests
exit. Never use the default desktop data directory for smoke validation.

## Independently observed checks

| Check | Result and limit |
| --- | --- |
| Root `go mod verify` | Passed before dependency edits. |
| Root `go vet ./...` | Passed after the JWT slice. |
| Desktop `go test ./...` | Passed; linker emitted macOS deployment-target warnings. |
| Embedded backend under desktop graph | Passed with the existing 120-second limit, both before and after the JWT slice. Covers encrypted reopen, wrong-key rejection, account persistence, synthetic FHIR import, and shutdown. |
| JavaScript desktop bridge | Five tests passed. |
| Frontend frozen install | Completed without lockfile edits; inherited peer-dependency warnings remain. |
| Frontend Karma suite | ChromeHeadlessCI executed 323 successful tests and one skipped test. Browser shutdown then failed/disconnected; this is not a clean passing command. The earlier ChromeHeadless attempt disconnected before executing tests. No timeout was increased. |
| Full backend suite | `go test -json -p 1 ./backend/... -count=1` exited successfully: 14 tested packages, 246 test/subtest pass events, no test skips. The database package completed in 380 seconds under the unchanged default timeout. An earlier default-parallel run failed in that package and its diagnostic output exceeded capture; the cause remains unverified. The isolated graph suite also passed in 32 seconds. |
| Packaged macOS desktop | `sh desktop/build.sh` completed, strict signature verification passed, and the packaged executable's native smoke suite passed all 10 checks using a fresh disposable database. Existing bundle-budget, CSS, and CommonJS warnings remain. |
| Root vulnerability scan before JWT upgrade | govulncheck 1.9.0 reported 16 reachable advisories across six modules, plus findings in imported/required dependencies not shown as called. This is static reachability, not proof of exploitation. |
| JWT regression slice | New tests reproduced acceptance of empty verification keys and panic on missing/null expiry. Fixed unit and HTTP middleware tests pass in both graphs; targeted race tests pass. |
| Auth vulnerability scan after JWT upgrade | No reachable vulnerabilities in `./backend/pkg/auth`; dependency-level findings elsewhere remain. This is not a clean whole-application security assessment. |
| Full root vulnerability scan after JWT upgrade | 14 reachable advisories remain, involving pgx, x/net, x/text, x/image, and circl. No JWT findings remain. |
| Frontend production dependency audit | Yarn reported 97 affected dependency paths and 65 distinct advisories, including three critical paths involving `@babel/traverse` and `elliptic`. The command exited nonzero. These are dependency findings, not demonstrated application exploits. |

The packaged desktop smoke checks cover the native binding, direct SMART rejection
without registration, fresh database API, local account creation and sign-in,
native URL validation, API/SPA error separation, and callback ownership/isolation.
They do not establish live provider interoperability. The tested ZIP SHA-256 was
`b027badba7de856ad2d8c2f02fffa5c8854fd0a2d7ef28406646f4f0e012859a`.
The OAuth classic-script fix is included in that artifact; its generated HTML no
longer includes the standalone ES module as a classic script.

GitHub's repository-wide Actions permission is disabled. Six inherited workflows
remain configured, including upstream cloud, CDN, container, and release
publication. Their configured `active` state does not mean they ran. No CI result
is inferred from their absence, and no workflow was enabled during this work.

The two previously skipped JWT tests have been replaced with synthetic user and
access-token round trips and rejection cases. Validation retains HS256 and
existing token metadata. Both module graphs select JWT v4.5.2, which fixes
[the parser allocation advisory](https://github.com/golang-jwt/jwt/security/advisories/GHSA-mh63-6h87-95cp).
Empty-key rejection does not resolve all of issue #52: server default-key
enforcement remains a separate concern.

## Dependency migration order

1. Establish effective authentication regressions and take the JWT v4 patch in
   both modules. Then adapt the reproduced classic-script OAuth fix from
   [#208](https://github.com/karamhouse/chartrove/pull/208), preserving David
   Radcliffe's authorship and original commit reference.
2. Use serialized backend package scheduling for the reproducible baseline and
   investigate the default-parallel failure separately. Resolve browser-runner
   shutdown before treating the whole application baseline as green. Audit inherited workflows by purpose and
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
