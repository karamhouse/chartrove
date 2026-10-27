# Chartrove direct SMART foundation

Chartrove is an independent fork of Fasten Onprem. This host foundation adds public SMART authorization without using Lighthouse, Fasten's client registrations, or bundled provider definitions. It does not change Fasten attribution or existing storage.

## Supported boundary

- Go owns discovery, S256 PKCE, authorization state, token exchange, and Patient retrieval.
- The native host opens the system browser. The temporary callback listens only on `127.0.0.1` at the explicitly registered port and path. It closes after authorization, cancellation, timeout, browser failure, or shutdown.
- Provider endpoints are pinned in an operator-supplied public profile. Discovery must advertise public standalone authorization, authorization-code grants, and S256 without plain PKCE. Discovery endpoints must match the pins. HTTP redirects are not followed.
- Only read-only patient scopes are accepted. The token grant must include Patient read permission. OpenID identity scopes, offline access, confidential clients, and client secrets are unsupported in this increment.
- Credentials stay in Go memory. Connection summaries contain an opaque local ID, provider ID, Patient ID, and expiry; no tokens or verifier. Disconnect forgets credentials and cancels active reads. Expired credentials are rejected and purged when accessed; shutdown clears all connections.
- Patient retrieval is bounded to 2 MiB, verifies resource type and authorized Patient ID, and returns JSON without writing it to a database. It does not use the legacy credential API or its logging path.

The existing Sources screen and synthetic Epic sandbox button are unchanged. The new foundation is exposed through a separate Wails service for the next UI increment. A successful local protocol test is not production approval or a live independent Epic test.

## Explicit registration configuration

Omitting `--smart-provider-config` disables direct connections and performs no catalog-file reads. No default registration, discovery URL, or Fasten fallback is supplied. An explicit file must be a JSON array of profiles. Unknown fields, including client secrets, are rejected.

Create a separate configuration using your own approved registration. This example deliberately uses an invalid example domain and is not a working registration:

```json
[
  {
    "id": "epic-example",
    "display": "Example hospital (Epic)",
    "fhir_base_url": "https://institution.invalid/api/FHIR/R4",
    "issuer": "https://institution.invalid/oauth2",
    "authorization_endpoint": "https://institution.invalid/oauth2/authorize",
    "token_endpoint": "https://institution.invalid/oauth2/token",
    "client_id": "CHARTROVE_OWN_CLIENT_ID",
    "redirect_uri": "http://127.0.0.1:49152/chartrove/callback",
    "scopes": ["launch/patient", "patient/Patient.read"]
  }
]
```

Use the institution's actual approved endpoints and issuer. Discovery requests include `Epic-Client-ID`; Epic can return client-specific metadata. Select scopes compatible with the registration and server (for example, `patient/Patient.r` for SMART v2). Provider and token URLs require HTTPS. Callback ports must be explicit, unprivileged, and available. Only IPv4 literal loopback callbacks are supported in this increment; HTTPS application links and arbitrary redirect ports are not implemented.

For synthetic testing, launch the extracted test app executable with an explicitly fresh data directory:

```sh
/path/to/Chartrove.app/Contents/MacOS/chartrove \
  --data-dir /path/to/fresh-test-directory \
  --unencrypted \
  --smart-provider-config /path/to/chartrove-smart-providers.json
```

The unencrypted option is for disposable synthetic testing. Do not supply an existing data directory to validate this foundation. No configuration is read from or written to `healthit.yaml`.

## Host API

Wails service identifier: `github.com/karamhouse/fasten-desktop/internal/shell.SMARTService`. Existing Fasten module identifiers remain for compatibility.

| Method | Behavior |
| --- | --- |
| `Providers()` | Lists configured public provider summaries; empty by default |
| `Connect(providerID)` | Verifies discovery, opens the system browser, waits up to five minutes, exchanges the code, and returns a credential-free connection summary |
| `Cancel(providerID)` | Cancels the pending authorization for that provider |
| `ReadPatient(connectionID)` | Returns the authorized Patient as a JSON string, without importing it |
| `Disconnect(connectionID)` | Cancels active reads and forgets local credentials; does not claim remote token revocation |

All methods require the trusted main application window. Authentication windows and calls without a window context are rejected. Callers cannot supply arbitrary request URLs or tokens. The browser callback accepts only GET, the registered Host/path, one matching unpredictable state, and one unambiguous code or denial; an issuer parameter, if present, must match the profile. Replays are rejected. Callback bodies never echo codes or errors.

## Validation and remaining work

`go test -race ./internal/smart ./internal/shell ./internal/host -count=1` exercises a real local TLS synthetic provider, including the PKCE challenge/verifier, registration binding, callback validation/replay, token exchange, Patient identity, redirects, denial, cancellation, timeout, expiry, and window restrictions. No real Epic account or existing database is used by those tests. Native `--smoke-test` adds Wails service registration and disabled-provider checks and requires a fresh database and no direct provider profiles.

Before production, obtain Chartrove-owned Epic registrations and verify the accepted callback at intended institutions. Build the provider UI and explicit import path, validate clinical resources and larger payloads separately, then design Keychain persistence, refresh/DCR, remote revocation, and additional vendors. Never embed a shared confidential-client secret. No provider agreements, accounts, production registrations, or real-record trials were initiated by this implementation.

References: [SMART App Launch](https://hl7.org/fhir/smart-app-launch/STU2.2/app-launch.html), [SMART discovery](https://hl7.org/fhir/smart-app-launch/STU2.2/conformance.html), [native OAuth](https://www.rfc-editor.org/rfc/rfc8252.html), [Epic app documentation](https://fhir.epic.com/Documentation?docId=patientfacingfhirapps&section=AutomaticClientDistribution).
