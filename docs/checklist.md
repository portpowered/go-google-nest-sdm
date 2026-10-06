# Library acceptance checklist

This checklist applies the current parent [template library standards](standards/template.md) and [repository standards](standards/library.md) to the resource response identity update. Independent code approval and passing checks are recorded separately from pending publication and deployment acceptance. Historical v0.1.0 installation and deployment evidence does not establish acceptance of this update. See [contributor verification](contributing.md) and [independent review](review.md).

- [ ] **1. Consumer independence.** SDK, examples and guides contain no consuming application adapters.
- [ ] **2. Customer documentation.** Authentication, supported operations, typed errors, injection and ownership match the exported API; reference and synthetic behavior are clearly identified.
- [ ] **3. Reports and badges.** README repository values are correct; Go, CI, coverage, release, reference, license and documentation destinations are active and verified.
- [ ] **4. Generated contract coverage.** Every outbound endpoint, method, parameter, header, channel, known nested payload, identifier and production wire model has a schema owner, generated declaration and actual use. Generation drift and negative source-gate controls pass. Dependency traffic is inventoried separately.
- [ ] **5. Blocking checks.** Offline build, all-linter, race, replay and every module gate pass. An independent reviewer confirms passing CI at the exact commit and reviews every narrow exception.
- [ ] **6. Coverage and fixtures.** Paired synthetic fixtures cover supported success and failures. Public, transport, replay and combined non-generated coverage are measured separately; combined coverage is 100%, enforced by CI. Exclusions and uncovered behavior are reported.
- [ ] **7. Package boundaries and model inventory.** Public SDK, generated models and transport packages follow the template. Every model inventory entry is independently traced to schema, generator and conversion; a separate consumer module verifies imports.
- [ ] **8. Functional options.** Constructors provide sensible defaults, validate options and hold no account credentials.
- [ ] **9. Ownership.** Client remains stateless per account; explicit sessions expose their lifetime, cancellation, errors and idempotent close.
- [ ] **10. Transport injection.** Every actual network edge is replaceable offline; tests replay exchanges through that seam.
- [ ] **11. Credential operations.** Exchange and refresh return credentials explicitly; callers own storage and renewal.
- [ ] **12. Published guides.** Customer MDX guides link matching generated references; all rendered pages pass link checks, including site root and generated pages.
- [ ] **13. Editorial review.** Every tracked documentation file and rendered page has been reviewed for audience, duplication, concise copy, evidence and release URLs.
- [ ] **14. Independent review.** Two non-implementing reviewers document separate verdicts and evidence for all 16 items at the final commit in `docs/review.md`; all findings are resolved and affected checks repeated.
- [ ] **15. Paired replay.** Each supported exchange checks the complete request before its response, rejects unexpected and duplicate calls, consumes the expected transcript and proves lifecycle cleanup. Fixtures identify synthetic or captured provenance.
- [ ] **16. Standalone CLI.** Separate CLI module consumes the SDK, covers auth/discovery/control/media/events, keeps secrets out of arguments and ordinary output, supports cancellation and cleanup, passes module/lint/replay checks, and installs from published module tags.

Published v0.1.0 SDK/CLI installation and deployment evidence applies only to the baseline. Record fresh evidence for this update before checking an item. Live-device qualification was not performed; fixtures and coverage remain explicitly synthetic/reference-based.
