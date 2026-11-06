# Optional agent interoperability for Chartrove — MCP direction

**Status:** proposed architecture, **not implemented, approved for external PHI disclosure, or assigned to the active Codex source task**. 2026-11-05 snapshot. Related existing issue: [#419](https://github.com/karamhouse/chartrove/issues/419). Leave its active optional search-mode work and issue body undisturbed.

## Decision

Let **Chartrove own clinical records, encrypted persistence, search semantics, patient/source authorization and evidence provenance**. Let **the user's chosen agent harness/model own language understanding and conversational presentation**. Offer a small, optional, standards-based Model Context Protocol (MCP) adapter rather than building a dedicated Elasticsearch/Typesense/vector/RAG/model-serving stack inside Chartrove.

A compatible local or remote agent may query only records and contexts for which the authenticated native user holds explicit rights. The adapter does not have independent access to raw databases, encryption keys, provider tokens, OAuth registration secrets, or other users' records.

## Reference implementation

[Medplum's MCP documentation](https://github.com/medplum/medplum/blob/main/packages/docs/docs/ai/mcp.md) intentionally uses a small tool surface driven by FHIR. Its [server implementation](https://github.com/medplum/medplum/blob/main/packages/server/src/mcp/server.ts) supplies a general `fhir-request` tool supporting GET, POST, PUT, PATCH and DELETE, plus compatibility tools. That demonstrates the protocol approach but is **not safe to drop into Chartrove as-is**: Chartrove is not Medplum's FHIR server, and writes or broad FHIR proxy paths are beyond the first Chartrove capability.

Keep Medplum as an architecture/protocol reference, not a deployed clinical repository, service dependency, or authority for Chartrove's per-source permissions. Implement only a narrow, reviewed Chartrove-owned adapter.

## Phase A: proposed read-only tool contract

| Proposed capability | Allowed response | Guardrails |
| --- | --- | --- |
| `search_records` | Bounded authorized metadata hits: resource type, source, opaque ID, title/record time and provenance | Explicit selected patient/scope; current source ownership and VIEW grant; bounded filters, pagination and truncation |
| `get_record` | One authorized selected FHIR resource, or a derived evidence summary, with source/version/citation | Revalidate live authorization, records and provenance at fetch; only allowed resource types; bound size and rate |
| `get_timeline` | Time-ordered authorized encounters/conditions/medications with explicit sources | Preserve uncertainty; no inferred clinical consolidation, no family-wide enumeration |
| `get_observation_series` | Dated numeric measurements with original units, reference ranges and status | Preserve units, source time and status; no unqualified inference of clinical trends |
| `get_document_excerpt` | User-authorized bounded excerpt from a previously reviewed local document | Never automatically fetch external URLs, include full document or trust OCR text as clinically validated |

Tool names and schemas above are a **design proposal**. They are not claims of existing endpoints or features. A lower-level read-only FHIR request can be considered later if it can be strictly constrained by method, path, patient/source scope, filters, auth and maximum output. No arbitrary CRUD, background mutation or raw database interface in V1.

## Identity, transport and security

1. **Local desktop:** Prefer an opt-in MCP `stdio` bridge or equally constrained local IPC paired to an authenticated user session. No direct database bypass. Require explicit patient/source selection and narrow one-session capabilities. Revoke on sign-out, grant changes, expiry or app close.
2. **Self-hosted:** Optional authenticated HTTPS Streamable HTTP with TLS, session binding, scoped authorization, request limits and origin controls. This is not a request to expose a public service by default.
3. **Scope:** A per-request owner/delegate check at every search/result/attachment read. Administrators and SSO groups have no implicit access to household records. Recheck resource existence, retained evidence and revoked/expired grants on each dereference or excerpt.
4. **No hidden disclosure:** Local-first does *not* imply a connected agent's model is local. Before an external model/harness receives record content, show or configure a deliberate data-destination/consent choice, allowed source scope and disclosure risks. Do not silently transmit records to OpenAI/Anthropic/other APIs or plugins.
5. **No dangerous defaults:** Read-only first. Never allow the agent to create, update, delete, reconcile, share, upload or export medical data without a separately designed explicit human approval and audited rights.
6. **Provenance and prompt injection:** Return stable source/resource/record-time references; treat text, PDFs and OCR transcripts as untrusted data, not instructions. Avoid logging PHI, prompt bodies, bearer tokens, query secrets or decrypted databases. Redact operational audit events.
7. **No permanent derived copy by default:** Avoid agent-side history, index services and browser-side global search stores. Apply delete/revocation immediately even if a past search returned a candidate identifier.
8. **Separate permission and transport tests:** Synthetic multiple owners, delegates, same FHIR IDs across sources, revoked/expired access, deleted files/sources, malicious query paths, SSRF, output limit, app shutdown, session resets, and explicit local-versus-cloud consent handling.

## Leverage what already exists

- Chartrove's accepted [source-authorized retrieval](https://github.com/karamhouse/chartrove/pull/416) and [reviewed document search](https://github.com/karamhouse/chartrove/pull/420) are the starting data paths; avoid a second independently trusted search engine.
- Its [synthetic retrieval evaluation](maintenance/retrieval-evaluation.md) and [relevance quality benchmarks](maintenance/retrieval-quality.md) compared encrypted SQLite and PostgreSQL without production data or external models. Recent 4/16/128 KiB synthetic corpus work is merged in [#429](https://github.com/karamhouse/chartrove/pull/429), but this does **not** establish model performance, clinical validity, concurrency capacity, an automatic intent classifier, or production search quality.
- The historical [Fasten Answers AI POC](https://github.com/fastenhealth/fasten-answers-ai) (last source push 2024-10-31) may inform retrieval evaluation design; it does not justify its original FastAPI + Elasticsearch + embedding + llama.cpp deployment.
- Current [#419](https://github.com/karamhouse/chartrove/issues/419) still contains prior security findings from the historical Typesense/RAG PR. Preserve its historical author attribution, exact source/evaluation evidence, existing search-mode acceptance gates and decision record when eventual issue changes are considered.

## Staged acceptance

**A. Contract-only (now):** Review this plan, map each tool to a real Chartrove authorization-aware read API, and publish a synthetic fixture specification. No new code owner/worktree, model service, deployment or permission change.

**B. Local prototype (after current source assignment):** One small adapter, no database dependency or model deployment, synthetic inputs only, end-to-end native permissions and provenance tests, clear tool/output-size caps and deterministic negative tests. Default-off installation.

**C. Opt-in harness validation:** Exercise a chosen MCP-capable local client on disposable synthetic records; compare local vs cloud model disclosure controls without uploading PHI. Document integration instructions independently of a particular harness (Codex/Pi/Hermes/Cline/etc).

**D. Production consideration:** Gate actual user records and any remote exposure behind approved staging, operational identity/recovery acceptance under [#425](https://github.com/karamhouse/chartrove/issues/425), separate privacy/security review, and clear revocation and deletion behavior.

## Explicit non-goals

No bundled LLM, default vector embeddings, managed AI index, full-stack RAG, autonomous clinical recommendation engine, unrestricted FHIR write API, Medplum deployment, external EHR onboarding, or Rust rewrite. A future Rust migration can implement the same audited wire/tool contracts after a functioning production release.

**Coordination:** This proposal lives only in a documentation branch. The existing Chartrove sole-writer Codex session is completing `task/explicit-search-modes`. Do not interrupt that work, modify its branch, reassign #419 source work, or infer approval to run production/real-patient tests.
