# Replacing Lighthouse for Chartrove production

Assessment date: October 27, 2026. This is an implementation assessment, not a production deployment or authorization to register accounts, accept agreements, purchase services, or access real records. `healthit.yaml` and existing health records are excluded.

Implementation status: the sandbox UI now passes the Angular suite and production build, and its page is present in the native Chartrove package. The checkout was moved outside iCloud and its unavailable dependencies restored. See [BRANDING_VALIDATION.md](BRANDING_VALIDATION.md) for those checks. The full OAuth flow through that Angular UI remains unvalidated; the earlier live protocol evidence used an isolated native harness.

## Recommendation

Start with an independently registered Epic public SMART client and a small, explicit provider catalog owned by this fork. Use the system browser with S256 PKCE and an approved native callback. Keep token exchange, FHIR retrieval, credential storage, and record import in the Go host. Add a hosted callback relay only if the selected provider's registration requirements require it.

This is an engineering recommendation for preserving local record ownership while limiting the first production integration to one vendor. It is not evidence that Epic has approved this fork or that every institution will accept the same configuration.

The restored sandbox UI is a development boundary: one published synthetic Patient, short-lived authorization in memory, and an explicit one-time local import. It still uses upstream's sandbox client and Lighthouse callback. That client ID, callback registration, and sandbox success do not grant production rights.

## What Lighthouse currently supplies

The legacy code combines institution search/catalog metadata, client configuration, a browser redirect/callback relay, and confidential-client token exchange. Native window handling replaces none of the provider registrations behind these services. Upstream [issue #629](https://github.com/fastenhealth/fasten-onprem/issues/629) retired Onprem's direct EHR retrieval; production search returned 404 during validation.

| Approach | Work this fork owns | Main tradeoff |
| --- | --- | --- |
| Direct public SMART integration, Epic first — recommended | Catalog, own client registrations, native browser callback, token lifecycle, FHIR adapters | Strongest independence and local data path; narrower initial coverage and ongoing vendor testing |
| Owned Lighthouse-compatible service | Search/catalog plus callback relay; protected server-side credentials for confidential clients; operations and abuse controls | Can preserve more legacy interfaces, but recreates a service and its credential responsibilities; still needs provider registrations |
| Fasten Connect adapter | New consent/export adapter and secure backend credentials; import exports into the local database | Managed provider coverage, but a separate API and data-processing arrangement rather than a URL substitution |

## Direct Epic path

Epic issues separate production and non-production client IDs through its developer registration process. Registered redirect URIs must match; its documentation describes S256 PKCE and a native-app exception for loopback redirects. Native callback acceptance must be verified for the actual app registration and target institutions, rather than assumed from the standard. See [Epic's current app documentation](https://fhir.epic.com/Documentation?docId=patientfacingfhirapps&section=AutomaticClientDistribution).

Epic publishes a public [R4 Endpoint Bundle](https://open.epic.com/Endpoints/R4), including organization names and FHIR base URLs. It is a useful catalog seed, not a list of app approvals or a substitute for client registration. Import only the public fields needed for discovery into a new, separately owned catalog; preserve source URL, retrieval date, stable identity, and an update/diff process. Do not derive this catalog from the excluded file.

[SMART App Launch](https://hl7.org/fhir/smart-app-launch/STU2.2/app-launch.html) separates registration from discovery. A provider's `.well-known/smart-configuration` supplies OAuth endpoints and capabilities; knowing those endpoints does not register the client. Maintain explicit provider profiles for issuer, FHIR base, client ID, registered callback, scopes, and supported refresh behavior. Reject unexpected endpoint changes pending review.

For production, replace embedded provider login WebViews with the system browser. [RFC 8252](https://www.rfc-editor.org/rfc/rfc8252) specifies external user-agents for native OAuth and describes native callbacks and PKCE. A loopback listener, if accepted, should exist only during authorization, bind to loopback, validate state, and close after one response. An HTTPS app link needs an owned domain and platform association. Neither callback path should put client secrets in the desktop bundle.

## If an owned relay is required

Keep its job narrow. Register an owned HTTPS callback with the provider. Associate a random, expiring, single-use state with a specific provider and approved native return channel. Relay the authorization result without storing records. Preserve PKCE verification in the app. Do not reproduce unrestricted `origin_url` or `dest_url` forwarding from caller input.

Confidential clients require a separate design: credentials remain server-side and the service participates in token exchange. Protect refresh credentials, redact request/response logging, validate issuer and redirect bindings, and define revocation, expiry, and failure recovery. A public endpoint catalog alone cannot replace this functionality.

## Managed-service alternative

Fasten Connect uses [dashboard-generated Public ID / Private Key authentication](https://docs.connect.fastenhealth.com/api-reference/authentication). Its [consent and collection flow](https://docs.connect.fastenhealth.com/guides/patient-consent-data-collection) returns connection identifiers and uses server-side export jobs, events, and download links. The current SourceCredential/token model would need an adapter.

Its [caching documentation](https://docs.connect.fastenhealth.com/guides/caching-strategy) states that retrieved records are cached by the service, with a default 24-hour lifetime. This changes the data path from direct provider-to-local retrieval. Evaluate account eligibility, commercial terms, retention, operating costs, and that privacy tradeoff before selecting it. No account or commercial integration was initiated here.

## Production acceptance milestones

1. Obtain registrations owned by the maintainer, initially non-production; confirm the accepted native callback mechanism.
2. Build the small public catalog/discovery adapter independently of the retired library's bundled definitions.
3. Implement system-browser return, single-use state, PKCE, cancellation, and endpoint/issuer binding in the Go host.
4. Store provider credentials in the platform credential store and keep them out of frontend state, logs, and exported records. Define disconnect and revocation behavior.
5. Add scoped FHIR retrieval, pagination, provenance, partial failure handling, import deduplication, and restart-safe refresh. Exercise synthetic data before any real-record trial.
6. Verify production authorization with the registered client and a specifically authorized test account, followed by signing/notarization and distribution checks.

No production provider support, refresh, bulk synchronization, or migration of purchased-app records is claimed by the sandbox UI restoration.
