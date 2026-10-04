# Contributor verification and contract provenance

Follow [Go](standards/go.md), [schemas](standards/schemas.md), [client API](standards/client-api.md), and [library](standards/library.md) standards. Keep the SDK independent of consumers (LIB-14), account credentials in requests (API-03), and network behavior in transport packages (LIB-01). Customer material belongs in `docs/guides/*.mdx`; this document records maintenance and evidence.

## Generated documentation contract

The API site uses the generated `docs/api-reference.openapi.json` documentation view (SCHEMA-10, LIB-16). `make generate-api` merges the REST resource, public model projection, OAuth, Pub/Sub, and media contracts into this single input. Component and security scheme names are namespaced by source, local references are rewritten, external references are rebased, and each route retains its source origin and authorization. Edit the canonical contracts in `api/` and regenerate this view.

Using one OpenAPI input avoids concurrent navigation metadata writes in the pinned documentation action. Both CI and local action builds must use this generated input. The established schema validator resolves its complete references, regeneration checks include its output, and the rendered site checker requires Devices, Structures, Rooms, Pub/Sub, Auth, and Media navigation along with valid internal links (SCHEMA-11, LIB-17).

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
| Camera stream and image retrieval require different transports and authorization. | SDM exposes generation/extension/stop and image URL/token results; the media client streams authenticated image/clip responses, and callers own body closure and peer connections. | Live media qualification is separate from REST command replay. |


### Catalog source map

The initial catalog review uses the current official trait pages below; this is reference evidence, reviewed on October 3, 2026, not captured behavior. Snapshots and partial updates deliberately share presence-aware trait models with optional fields: the current source evidence does not establish stronger required-field sets for complete snapshots (SCHEMA-07). Their semantics remain distinct: updates patch only present fields. Command inputs and outputs use separate generated contracts with their documented required fields. The source map covers 20 traits; `StructureInfo` and `RoomInfo` have their own resource scope.

| Catalog trait | Official source |
| --- | --- |
| CameraClipPreview | [camera-clip-preview](https://developers.google.com/nest/device-access/traits/device/camera-clip-preview) |
| CameraEventImage | [camera-event-image](https://developers.google.com/nest/device-access/traits/device/camera-event-image) |
| CameraImage | [camera-image](https://developers.google.com/nest/device-access/traits/device/camera-image) |
| CameraLiveStream | [camera-live-stream](https://developers.google.com/nest/device-access/traits/device/camera-live-stream) |
| CameraMotion | [camera-motion](https://developers.google.com/nest/device-access/traits/device/camera-motion) |
| CameraPerson | [camera-person](https://developers.google.com/nest/device-access/traits/device/camera-person) |
| CameraSound | [camera-sound](https://developers.google.com/nest/device-access/traits/device/camera-sound) |
| Connectivity | [connectivity](https://developers.google.com/nest/device-access/traits/device/connectivity) |
| DoorbellChime | [doorbell-chime](https://developers.google.com/nest/device-access/traits/device/doorbell-chime) |
| Fan | [fan](https://developers.google.com/nest/device-access/traits/device/fan) |
| Humidity | [humidity](https://developers.google.com/nest/device-access/traits/device/humidity) |
| Info | [info](https://developers.google.com/nest/device-access/traits/device/info) |
| Settings | [settings](https://developers.google.com/nest/device-access/traits/device/settings) |
| Temperature | [temperature](https://developers.google.com/nest/device-access/traits/device/temperature) |
| ThermostatEco | [thermostat-eco](https://developers.google.com/nest/device-access/traits/device/thermostat-eco) |
| ThermostatHvac | [thermostat-hvac](https://developers.google.com/nest/device-access/traits/device/thermostat-hvac) |
| ThermostatMode | [thermostat-mode](https://developers.google.com/nest/device-access/traits/device/thermostat-mode) |
| ThermostatTemperatureSetpoint | [thermostat-temperature-setpoint](https://developers.google.com/nest/device-access/traits/device/thermostat-temperature-setpoint) |
| StructureInfo | [structure info](https://developers.google.com/nest/device-access/traits/structure/info) |
| RoomInfo | [room info](https://developers.google.com/nest/device-access/traits/room/info) |

Thirteen command contracts are owned by the Fan (SetTimer), ThermostatEco (SetMode), ThermostatMode (SetMode), ThermostatTemperatureSetpoint (SetHeat, SetCool, SetRange), CameraEventImage (GenerateImage) and CameraLiveStream (Generate/Extend/Stop for RTSP and WebRTC) pages. Their named input/result components are in `api/commands.openapi.yaml`. RTSP URL nesting and generation/extension/stop results are distinct; WebRTC offer/answer SDP and media-session results are distinct. Semantic constraints in prose need tests in addition to generated field types. The [event reference](https://developers.google.com/nest/device-access/api/events) owns envelope metadata and relation/resource update shapes; trait pages own namespaced camera event bodies.

## Required inventories

Review both the schema registry and **all** production packages. For every resource, trait snapshot, trait update, command input/result, event body/envelope, OAuth exchange and Pub/Sub push wrapper, raw message and pull acknowledgement, record: schema file/component, generated Go declaration, pinned generator invocation, public conversion/decoder, and exact wire use. Include empty capability objects, open enums, unknown-field companions, primitive constants, unused exported JSON structs, dependency types and anonymous objects. Never accept a generated-file list as the complete model population (template items 4, 7 and 14).

The outbound inventory must include SDM list/get devices, structures and rooms; execute-command; OAuth exchange/refresh; and Pub/Sub pull/acknowledge/modify-ack-deadline. Query/header names, JSON keys and registry dispatch names require generated bindings. Inventory active dependency traffic independently; keep unused network SDKs out of the dependency graph. Inventory the image/clip HTTPS retrieval routes, generated authorization formats and query keys separately. The media client opens image/clip HTTP responses; it does not open RTSP or WebRTC peer connections.

The PCM consent GET is performed by the caller's browser, using the SDK's schema-bound URL constructor and generated origin, path and query constants. The CLI's fixed OS browser launchers accept that session URL directly. Its `net.ListenConfig` listener receives callbacks only on the caller's registered loopback origin; this is an inbound local endpoint, not an additional provider socket. The source gate inventories both seams and rejects additional unregistered CLI network or process primitives.

## Verification plan

Run `make lint` and `make check` before proposing completion. Keep pinned blocking all-linter checks for every Go module, generation drift checks, schema compilation, catalog integrity, source/model gates, consumer builds, replay, race and non-generated coverage in CI. Report actual output and commit identity in the independent review; never check acceptance items from intended configuration.

`make test-contracts` also runs the Go example gate against canonical schemas. It validates nested component examples, operation request/response examples, parameter examples and AsyncAPI message payloads against their owning schemas without loading network references. Command examples additionally prove all thirteen identifier/parameter bindings; a malformed known command cannot escape through the explicit future-command branch. Examples are sanitized synthetic illustrations of the linked provider references, not device captures (SCHEMA-17/18, LIB-19). Inspect the rendered command variant selector, required nested fields, complete request snippets and event example list before accepting a documentation change.

Interactive authorization replay covers the browser consent URL, callback state, the S256 challenge/verifier relationship, token exchange and Google's required initial device-list call as one flow. Browser launch, the CLI's inbound loopback listener and the SDK's outgoing HTTP transport are independently injectable. Tests cover denial, duplicate and invalid callbacks, cancellation during completion, repeated completion and cleanup. The SDK owns temporary authorization state; callers own callback endpoints and credential persistence.

The [PCM authorization reference](https://developers.google.com/nest/device-access/api/authorization) documents the consent endpoint and state parameter. [Google OAuth](https://developers.google.com/identity/protocols/oauth2/native-app) documents S256 PKCE. The implementation always sends the challenge and its bound verifier and does not fall back to an unprotected exchange. PCM forwarding and provider enforcement of that challenge have not been qualified against a live account; synthetic replay establishes the library's binding and lifecycle, not provider enforcement. The [Device Access walkthrough](https://developers.google.com/nest/device-access/authorize) owns the mandatory initial device-list completion call.

The generated command union uses the pinned `oapi-codegen/runtime` JSON merge helper. Its pinned `go-jsonmerge`, UUID and text dependencies perform local processing; they add no outgoing network edge. The complete model inventory includes the new authorization models and named command envelopes. Private generated union storage is a codec cache, not an additional wire field, and is checked against its owning union schema and codecs.

The comparison requires positive and negative fixtures for required fields, JSON types, enum values and semantic bounds; absent/zero/false/empty/null distinctions; empty traits; unknown names/fields/enum values; malformed known payload rejection; partial updates; distinct command result shapes; envelope metadata and relation updates; OAuth error bodies; stateless wrapped/unwrapped push decoding, application-owned HTTP acknowledgement boundaries, Pub/Sub pull ack/nack/deadline behavior; media expiry and cancellation. Exercise outbound command validation without network I/O, including fan seconds-string bounds and heat/cool ordering. Do not invent provider bounds absent source evidence.

Media synthetic fixture bodies are intentionally dummy bytes, not decodable JPEG or MP4. They verify authenticated paired HTTP contracts and body ownership; they do not establish media decoding or live-device compatibility.

Replay pairs match method, origin, escaped path, repeated query values, relevant headers and full body before returning a response. Assert status, headers, result and complete consumption; failures never return fallback responses (LIB-05). Store only labeled fixtures under `tests/replay/fixtures`, grouped by transport and behavior. Separate replay, unit, combined and opt-in live coverage (LIB-07). Race checks cover session close/cancellation and concurrent acknowledgement paths as applicable; camera peer-connection tests are outside a REST-only command lifecycle.

## Docs and release

Build customer MDX and schema references using the shared Fumadocs action. Inspect navigation and every rendered page, check internal links, and verify external source/report/release destinations. Coverage/release/reference/Pages badges are configured destinations until actual publication is verified. Enable Pages with GitHub Actions, publish only after review, and publish the SDK `v0.1.0` tag first, then publish the nested CLI module's matching `cmd/go-google-nest-sdm/v0.1.0` tag. The CLI requires the published SDK version without a committed local replacement; contributor checks use a temporary modfile for offline verification before publication. Verify separate consumer installation against those published tags.

Two independent reviewers must audit every numbered requirement in [the checklist](checklist.md), independently search for missing wire definitions and examine every tracked documentation file. Record each verdict, final commit, evidence and finding disposition in [the review](review.md). Unresolved findings, unavailable publication checks or absent final-commit CI keep the corresponding items open.





