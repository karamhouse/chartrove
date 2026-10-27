# Direct SMART Connection Foundation Implementation Plan

> Implement natively in this session, following the user's approved Epic-first direction. Use test-first development and independent review before completion.

**Goal:** Add a usable, independently configured Go-host SMART connection boundary without accessing existing records or using Fasten's registrations.

**Architecture:** An in-memory connection manager owns discovery, PKCE, browser authorization, a temporary loopback callback, tokens, and Patient retrieval. A main-window-only Wails service exposes public connection summaries. Explicit JSON provider profiles supply Chartrove-owned client IDs and pinned endpoints; no profile means disabled.

**Tech stack:** Existing Go 1.26/Wails v3 host, Go standard-library HTTP/crypto, TLS synthetic provider tests. No new dependencies.

**Design and constraints:** Choose direct public-client authorization over a hosted broker, as requested. Use the system browser and a registered fixed IPv4-loopback callback; do not assume an EHR accepts arbitrary redirect ports. Only read-only patient scopes are supported. Do not request OpenID identity or offline access in this increment. Tokens remain in Go memory and are forgotten on disconnect or shutdown; expired connections are unusable and purged on access. No Keychain persistence, automatic refresh, clinical-record import, registration, or production activation is included. Preserve Fasten attribution and identifiers; never read or modify healthit.yaml or existing databases.

## Review focus

- Unknown or ambiguous callback state must not consume a pending connection.
- Cancellation, browser-launch failure, and shutdown must close listeners and erase credentials.
- Discovery changes and HTTP redirects must not redirect token-bearing requests.
- Provider response bodies and credentials must not appear in errors or Wails results.
- A provider registration, not sandbox success, gates any real Epic authorization.

## Tasks

1. Add failing configuration and real TLS synthetic-provider tests under `desktop/internal/smart/`. Assert the S256 verifier round trip, state and issuer checks, exact redirect URI, token exchange, authorized Patient retrieval, cancellation/replay, expiry, and safe results/errors.
2. Implement strict profiles and the manager in that package; run its race suite. Keep the listener bound only while authorization is active, with bounded requests and a single accepted callback.
3. Add `desktop/internal/shell/smart.go` and main-window authorization tests. Wire explicit `--smart-provider-config` loading, browser dispatch, service registration, and shutdown in `desktop/cmd/fasten-desktop/main.go`.
4. Document the supported configuration, host API, registration gates, and deferred features in `desktop/DIRECT_SMART.md`. Run the desktop module suite, build the native package using unchanged frontend assets, and run native smoke checks only against a fresh disposable database.
5. Review, fix findings, and commit only this task's files after validation. Publication requires a separate user request.
