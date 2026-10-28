# Desktop restart validation

Chartrove is an independent fork of Fasten Onprem. Validate restarts with an
explicit disposable data directory; never use an existing health-record database
or change `healthit.yaml` for this check.

## Avoid stale native window observations

A native automation handle retained across process exit can capture a blank
window even while the restarted application renders normally. On 2026-10-28,
the retained handle captured a blank frame while a newly acquired handle captured
the dashboard and imported Epic sandbox Patient in the same running process.
The unchanged PR #321 release archive passed the live restart checks.

After each restart:

1. Confirm the previous process exited normally.
2. Launch the executable with the same explicit disposable data directory and
   separate sandbox provider profile. Confirm the startup data-directory message.
3. Acquire a new native app/window handle for that running bundle. Do not reuse a
   handle from the previous process. If the handle was acquired before WebView
   startup completed, reacquire it after startup before capturing evidence.
4. Verify both rendered content and an interaction: open Sources, open Explore,
   and open the imported Patient. A successful database query alone does not
   establish that the desktop UI works.
5. Capture screenshots from the fresh handle, then quit normally and verify exit
   status. Do not query an app handle after quit: automation may relaunch the app
   without the required isolation flags.

If a freshly acquired live handle still shows a blank window, retain startup
diagnostics and investigate it as a new failure. Do not assume every blank frame
is a stale handle, reload repeatedly, or delete WebKit/application storage.

## Verified on 2026-10-28

- Native smoke check: ten checks passed, including fresh database setup, local
  signup/signin, native bindings, callback isolation, and API/SPA separation.
- Chartrove-owned Epic non-production registration: system-browser OAuth with
  PKCE, synthetic Patient preview, and explicit snapshot import passed.
- The imported Patient remained visible through the dashboard, direct source,
  and Explore after restarting the unchanged release build.
- Empty-database and signed-in-account restarts also rendered correctly.
- Normal quits returned exit code 0. Temporary diagnostics were not included in
  product code, and no dependency versions or application identifiers changed.

The earlier blank-restart report was a false failure in native UI observation;
this procedure corrects the validation rather than adding a speculative product
workaround.
