# Direct SMART Sources UI plan

Implement the user's approved Go-host foundation in Sources with an explicit preview/import boundary. Use the existing Angular 14, Wails runtime, native Git, and dependency locks.

1. Extend the desktop bridge with a typed SMART capability and contract tests. Add a service that lists configured providers, connects, retrieves only the authorized Patient, and cleans up cancelled/stale connections. No tokens enter Angular.
2. Add a focused Sources child component for provider choice, system-browser progress/cancel, escaped Patient preview, explicit import, retry, disconnect, and imported-record navigation. No automatic import. Destroy clears Patient memory and releases the connection.
3. Add optional validated `smart_origin` multipart metadata to Patient snapshot imports. Backend source identity is scoped to FHIR base plus raw Patient ID; repeated completed snapshots reuse the source. Preserve ordinary manual/sandbox behavior and keep different providers separate. Reuse an empty failed source on retry.
4. Add an isolated native UI fixture mode requiring a fresh disposable database, unencrypted testing, and no configured providers. Its local TLS synthetic provider uses a private test HTTP client and controlled browser driver; normal mode retains system-browser authorization and standard TLS verification.
5. Run test-first frontend/backend checks, the full frontend suite, relevant Go suites, production packaging, and native UI preview/import/dedup/cancel/restart checks with fresh synthetic data. Review and commit locally after validation.

Preserve Fasten attribution and identifiers. Never read, hash, scan, copy, migrate, or modify `healthit.yaml` or existing health records. No real provider registration or authorization is performed. Real Epic system-browser interoperability remains gated on Chartrove-owned registration. Do not publish without a separate request.
