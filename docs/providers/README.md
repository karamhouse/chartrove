# Provider and payer compatibility roadmap (planning, not a support claim)

**Snapshot:** 2026-11-05; Chartrove main `9ef8cad98bd7bde91e3a085161b7e1d4cea81cb4`. This document is intentionally separate from the sole Chartrove source writer's active search-mode assignment. No patient data, provider credentials, production access, or live-provider tests were used.

## Purpose

Replace an unmanageable set of inherited Fasten Onprem provider requests and historical connection reports with a **traceable provider/payer coverage program**, without confusing legacy Lighthouse listing with functional Chartrove connectivity. Prioritize the maintainers' own household's providers in a private selection, then reusable EHR integrations and community requests.

The public [issue index](issue-index.csv) records **all 153 open Chartrove issues at the snapshot** and a reviewed primary workstream for each: **38 onboarding/coverage requests**, **25 source-connection or import reports**, **12 adjacent platform/interoperability tasks**, and **78 outside this provider program**. These groupings are planning buckets, not a verdict that bugs are resolved. Some entries in the first two buckets are broad intake or core defects rather than one institution.

## Evidence and coverage limitations

- Chartrove's [provider validation inventory](../../desktop/PROVIDER_VALIDATION.md) confirms disposable local FHIR import and limited synthetic SMART validation; it **does not qualify a production provider connection**.
- The inherited Lighthouse discovery/search, callback relay and confidential-client registration infrastructure is not owned by Chartrove. [Production replacement assessment](../../desktop/LIGHTHOUSE_REPLACEMENT.md) recommends Chartrove-owned direct public SMART registration, initially Epic, and explicitly distinguishes sandbox success from production approval.
- The historical [Fasten Sources fork](https://github.com/andhines/fasten-sources/tree/1bcac8a26c88dadd3267e5557505614b445711b9) contains **39 platform definition YAML names**, inventoried in [platform-seed.csv](platform-seed.csv). The repository's 2024-11-30 data snapshots also reference large `brands.json`, `portals.json` and `endpoints.json` blobs; exact sizes and Git blob IDs are in [catalog-manifest.json](catalog-manifest.json). Those **full files are not ingested** into Chartrove's registry in this PR. The source fork does not advertise an SPDX repository license. Review distribution rights before copying its catalog, definitions or derived bulk tables; keep current documents as provenance and independently authored planning facts.
- The original hosted Lighthouse catalog may have changed after 2024; 2024 platform definitions and issue titles cannot establish a complete or current institution inventory, a registered client, availability, authorization or import compatibility. Do not query privileged/production endpoints to fill this gap.
- The public repository must not reveal which providers or insurers particular family members use. Keep the **P0 household-provider shortlist and personal account details outside this public repository**. Import only provider *metadata* suitable for public distribution after source/license review. Do not copy provider OAuth registrations, secrets, patient identifiers, tokens or personal records.

## Canonical registry design

Maintain **separate entities** rather than flattening a vendor, hospital, insurer and portal into one name:

| Entity | Essential facts | Why distinct |
| --- | --- | --- |
| Platform/adapter | Platform key, product/vendor, protocol, reference source and version | One Epic/Oracle Health/etc adapter can serve many institutions but cannot grant registration everywhere |
| Institution/organization | Stable public identifier, legal/brand name, alias, geography, organization type, provenance | Similar names and mergers demand explicit disambiguation |
| Payer/insurer | Payer name, plan/brand aliases, data scope and patient-access program | Claims/EOB and coverage APIs differ from EHR clinical APIs |
| Portal | Public portal name/domain, institution association, catalog source | One organization may expose multiple portals; portal name is not proof of FHIR support |
| FHIR endpoint | Public R4 base URL, discovery date, CapabilityStatement reference, trust/discovery result | Endpoint publication alone never proves application registration |
| Chartrove registration | Locally configured client ID, allowed redirect, scope and vendor registration evidence **outside the public catalog** | Client metadata and confidential credentials must not leak into a public inventory |
| Verification event | Date, chartrove commit, test type, synthetic/live-authorized scope, sanitized outcome and incident link | Avoid stale "supported" booleans without evidence |
| Historical request | Chartrove issue, upstream Fasten issue, requested institution or failure, affected platform, disposition | Preserve contributor attribution and actionable defects |

Suggested machine-readable institution fields once catalog ingestion is approved: `institution_id,display_name,aliases,organization_type,country,region,platform_key,portal_domain,fhir_base_url,source_name,source_revision,source_record_id,source_checked_at,endpoint_checked_at,chartrove_registration_status,sandbox_test_status,production_validation_status,related_issue_ids`. Fields absent from a historical source must remain unknown, not guessed. Avoid committing private P0 rank or account associations.

**Verification is multidimensional**, not one linear "supported" flag:
1. Historical catalogued (name reported, provenance retained).
2. Endpoint independently discovered and identity matched.
3. Chartrove-owned client registration/callback confirmed for applicable institution.
4. Synthetic authorization and FHIR import tested, with mapped patient/source ownership and failure paths.
5. Separately authorized real-world production validation completed (read, attachment, refresh/reconnect, revoked consent, cleanup).
6. Ongoing health monitored with dated compatibility evidence; failures revert status to unverified/degraded.

## Prioritization

**P0 — privately selected household providers and insurers:** Choose via private configuration or owner-maintained off-repo list; prioritize real care access only after explicit consent and production registration. Record anonymized pass/fail and software defects publicly, never patient facts, user-specific provider relationships or account URLs.

**P1 — shared adapters and interoperability foundations:** Start with independently registered Epic public SMART (existing isolated synthetic work), then other platforms if actual P0 needs or high demonstrated reuse justify them. Handle platform-wide token/callback parsing, institution lookup, profile mismatches, pagination, binary/attachments, permissions and duplicate import safely in generic code.

**P2 — other inherited community institutions:** Retain a catalog/request entry for every historical request and severity report. Consider new registrations when supported by publicly discoverable endpoints, maintainers, and test coverage; no promise of activation or time to delivery.

Keep payer access separate from EHR access, with explicit support for claims/EOB, insurance eligibility and patient-mediated export where applicable. Allow local FHIR/C-CDA document import as a usable fallback to an unavailable direct integration.

## How the 63 inherited source/onboarding records transition

- The [issue index](issue-index.csv) is the lossless baseline for **38 onboarding plus 25 connection/import** titles, plus 12 shared interoperability tasks. Each row retains the original Chartrove issue link and upstream Fasten issue reference. Do not copy entire discussions into this document; original issue bodies, authorship and comments remain authoritative.
- The **38 onboarding** entries are historical *requests*, not 38 working integrations or even 38 unique institutions. A few are broader coverage intake, vendor or API work; the future registry must split or merge only with explicit source evidence.
- The **25 connection/import** entries include cross-cutting code bugs, aggregate status reports, OAuth/portal incidents, sandbox tests and export/attachment failures. Classify each root cause after review as `platform-defect`, `institution-specific`, `payer-specific`, `core-import/serialization`, `retired-Lighthouse-dependency`, or `unreproducible/needs-evidence`. Fixes affecting generic parsers or authorization must retain actionable issue ownership until source tests establish a resolution.
- The **12 adjacent** items remain cross-cutting tasks (FHIR launch, provider search, TEFCA, HIE, international support, locator, etc.); do not silently close them because a coverage entry exists.
- Once an independently authored full catalog and per-request coverage record are available, add links and a reasoned disposition comment to each inherited issue. **Only then** propose issue closures as `not planned / superseded by roadmap`, never `completed` merely because names were imported. Preserve unresolved reproducible bugs as open engineering issues.
- Reconcile count changes against *fresh GitHub state* before mutations; the 153-issue count is a dated snapshot and will naturally change.

## Next planning milestones (no source agent interruption)

1. **Metadata/license admission:** Inspect original sources for the brand/portal/endpoint catalog (and [Fasten Toolbox](https://github.com/fastenhealth/fasten-toolbox)); determine license, permissible fields, freshness and provenance; validate rows independently, with no token/client-secret import.
2. **Full historical source ingestion:** Import the three large catalog artifacts *offline* under bounded memory, preserve source revision and catalog IDs, deduplicate conservatively and generate a **compact** independently maintainable public metadata registry. Do not claim this milestone achieved by this documentation PR.
3. **Reconcile issue-to-entry links:** Resolve ambiguous organization names, aliases and platform changes; map all 38+25 entries and the 12 adjacent work items to an institution/payer/platform or shared incident. Maintain a zero-unmapped exception ledger with explanations.
4. **Owner's private P0 selection:** Capture just the required provider identities privately, not in GitHub. Use those to order Chartrove-owned registration work.
5. **Production evidence gates:** Complete independent sandbox protocol tests, supported provider/client registration, explicit operator approval for any real-record test, bounded record import/reconnect and sanitised rollback verification before labelling anything production-ready.
6. **Issue consolidation:** Propose closure batches only with one-to-one audited mappings, retention of historical attribution, and no suppression of generic defects.

## Responsibility boundaries

This PR is GitHub planning only. The existing Codex/Rex Chartrove session remains sole source-code writer, currently on optional search modes under [#419](https://github.com/karamhouse/chartrove/issues/419). Do not run new source agents, touch its worktrees, alter `healthit.yaml`, change GitOps/deployments, fetch personal health records, or initiate provider registrations. Live Authentik acceptance is independently gated by [#425](https://github.com/karamhouse/chartrove/issues/425).
