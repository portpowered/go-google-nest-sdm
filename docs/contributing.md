# Contributor verification and contract provenance

Follow [Go](standards/go.md), [schemas](standards/schemas.md), [client API](standards/client-api.md), and [library](standards/library.md) standards. Keep the SDK independent of consumers (LIB-14), account credentials in requests (API-03), and network behavior in transport packages (LIB-01). Customer material belongs in `docs/guides/*.mdx`; this document records maintenance and evidence.

## Evidence classes

The initial catalog is derived from Google's published Device Access REST, trait, command and event documentation. Synthetic fixtures exercise that interpretation offline. They are **not captured traffic** or proof that a live device accepts a request. The comparison is historical design input, not an authoritative contract. No authenticated live endpoint or media connection is qualified by synthetic replay.

- **Documented:** an explicitly cited provider contract or example.
- **Synthetic:** a repository-authored request/response pair or event constructed to test that contract.
- **Captured:** sanitized traffic from an authorized real endpoint, only after review; keep private originals and credentials outside the repository.
- **Local policy:** SDK validation, retry or lifecycle behavior imposed by this library. Identify it separately from provider requirements.

Prefer reviewed captures when they differ from references (LIB-12, SCHEMA-09), record the discrepancy, and update the schemas, generated artifacts, fixtures and guide together. Never add evidence metadata to the API schema (SCHEMA-08).

## Sources and unresolved discrepancies

The current source families are [REST resources](https://developers.google.com/nest/device-access/reference/rest), [device traits](https://developers.google.com/nest/device-access/traits), [events](https://developers.google.com/nest/device-access/api/events), and [authorization](https://developers.google.com/nest/device-access/authorize). Source review and constraints belong with the checked-in catalog inventory; these links alone do not prove every modeled field.

| Question | Current interpretation | Evidence still needed |
| --- | --- | --- |
| Protobuf lists expose pagination; published REST lists expose different fields. | Do not infer REST pagination from protobuf. Implement the documented REST surface. | Authorized REST request or upstream clarification before adding pagination. |
| The event field table describes `resourceGroup` as an object while examples show an array of resource names. | Use the documented example array shape, preserving unknown fields. | Sanitized capture or upstream clarification; label synthetic examples accordingly. |
| Community event models omit envelope metadata or use incorrect event keys and numeric relation enums. | Model the provider's namespaced keys, string relation values and independent event identifiers. | Recheck current official docs; community declarations are historical references. |
| Camera stream and image retrieval require different transports and authorization. | SDM exposes generation/extension/stop and image URL/token results; callers own media retrieval and peer connections. | Live media qualification is separate from REST command replay. |

## Required inventories

Review both the schema registry and **all** production packages. For every resource, trait snapshot, trait update, command input/result, event body/envelope, OAuth exchange and Pub/Sub message/acknowledgement, record: schema file/component, generated Go declaration, pinned generator invocation, public conversion/decoder, and exact wire use. Include empty capability objects, open enums, unknown-field companions, primitive constants, unused exported JSON structs, dependency types and anonymous objects. Never accept a generated-file list as the complete model population (template items 4, 7 and 14).

The outbound inventory must include SDM list/get devices, structures and rooms; execute-command; OAuth exchange/refresh; and Pub/Sub pull/acknowledge/modify-ack-deadline. Query/header names, JSON keys and registry dispatch names require generated bindings. Inventory active dependency traffic independently; keep unused network SDKs out of the dependency graph. Returned media URLs do not constitute library-opened media connections.

## Verification plan

Run `make lint` and `make check` before proposing completion. Keep pinned blocking all-linter checks for every Go module, generation drift checks, schema compilation, catalog integrity, source/model gates, consumer builds, replay, race and non-generated coverage in CI. Report actual output and commit identity in the independent review; never check acceptance items from intended configuration.

The comparison requires positive and negative fixtures for required fields, JSON types, enum values and semantic bounds; absent/zero/false/empty/null distinctions; empty traits; unknown names/fields/enum values; malformed known payload rejection; partial updates; distinct command result shapes; envelope metadata and relation updates; OAuth error bodies; Pub/Sub ack/nack/deadline behavior; media expiry and cancellation. Exercise outbound command validation without network I/O, including fan seconds-string bounds and heat/cool ordering. Do not invent provider bounds absent source evidence.

Replay pairs match method, origin, escaped path, repeated query values, relevant headers and full body before returning a response. Assert status, headers, result and complete consumption; failures never return fallback responses (LIB-05). Store only labeled fixtures under `tests/replay/fixtures`, grouped by transport and behavior. Separate replay, unit, combined and opt-in live coverage (LIB-07). Race checks cover session close/cancellation and concurrent acknowledgement paths as applicable; camera peer-connection tests are outside a REST-only command lifecycle.

## Docs and release

Build customer MDX and schema references using the shared Fumadocs action. Inspect navigation and every rendered page, check internal links, and verify external source/report/release destinations. Coverage/release/reference/Pages badges are configured destinations until actual publication is verified. Enable Pages with GitHub Actions, publish only after review, and tag the SDK and nested CLI module together. Verify separate consumer installation against those published tags.

Two independent reviewers must audit every numbered requirement in [the checklist](checklist.md), independently search for missing wire definitions and examine every tracked documentation file. Record each verdict, final commit, evidence and finding disposition in [the review](review.md). Unresolved findings, unavailable publication checks or absent final-commit CI keep the corresponding items open.
