# Provider connection validation

Checked October 27, 2026 using a disposable database and Epic's published synthetic patient. Neither the purchased application's records nor `healthit.yaml` were used.

## Results

| Boundary | Result |
| --- | --- |
| Production Lighthouse `POST /v1/search` with the desktop app filter | HTTP 404 |
| Sandbox Lighthouse `POST /sandbox/search` | HTTP 200; Epic sandbox returned |
| Sandbox catalog lookups with the returned brand and endpoint UUIDs | HTTP 200 |
| Sandbox `GET /sandbox/connect/<endpoint UUID>` | HTTP 200; public PKCE client metadata |
| Metadata fetch inside the native Wails WebView | Passed, including CORS |
| Native authorization popup through Lighthouse to Epic MyChart | Passed |
| Published synthetic patient login and consent | Passed; demographics only, one hour |
| Lighthouse return to the owned native popup and main-window bridge | Passed |
| Public-client authorization code exchange with S256 PKCE | HTTP 200 |
| Authenticated FHIR Patient read | HTTP 200; matched Epic's published synthetic patient |

The live test used the rebuilt native host and the actual `bridge.mjs`, with an isolated test frontend. It exercised the provider protocol, not the normal Angular provider UI or the backend's bulk-sync and dynamic-registration paths. OAuth tokens stayed in JavaScript memory and were neither printed nor saved. The test session was closed afterward.

The current fork's Medical Sources page offers manual upload; its old provider search and callback processing have been removed. A successful sandbox protocol test therefore does **not** mean provider connection has been restored in the normal application.

## Production blocker

Upstream [issue #629](https://github.com/fastenhealth/fasten-onprem/issues/629), published February 7, 2026, states that Onprem no longer supports direct EHR retrieval. Production search currently returns 404, consistent with that retirement. The surviving sandbox is useful for development but does not provide production availability or authorization.

Production revival needs a supported catalog and OAuth integration: either an authorized integration with the maintained service, or independently registered provider clients and an owned callback service. Changing the native bridge alone cannot restore the retired production infrastructure. Confidential-client exchange, dynamic registration, refresh, bulk synchronization, and other provider families remain unvalidated.

## Reproduce the isolated sandbox protocol check

Build and extract the community app first. Create a fresh directory with `mktemp -d` under the OS temporary directory, and put `testdata/provider-sandbox.html` there as `frontend/index.html`. Copy `../frontend/src/assets/desktop/bridge.mjs` into `frontend/assets/desktop/bridge.mjs` beneath that directory. Launch the extracted executable with:

```sh
fasten-community --unencrypted --data-dir "$test_root/data" --frontend-dir "$test_root/frontend"
```

Here `test_root` must name the newly created disposable directory, never an existing record directory. The harness is hard-coded to Epic sandbox and automatically opens its authorization popup. Use only the patient credentials published in [Epic's sandbox documentation](https://fhir.epic.com/Documentation#patientlist). Select demographics only and the shortest offered consent duration. The main window displays HTTP statuses and a synthetic-patient match, never tokens or patient details. Close the app and remove only that disposable directory after testing. This page is a test utility; it is not included in the normal frontend assets.
