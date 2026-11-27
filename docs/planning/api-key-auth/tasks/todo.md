# Gateway authentication implementation

**Goal: create and maintain one valid, gateway-owned core `Policy` resource from
an authorized AIGateway's single `apiKey` instance and referenced Secret.**
[Plan](../plan.md) · [Design](../../../design/gateway-auth/gateway-authentication.md).

This milestone ends at the core Policy API. Proxy creation, actual mounts,
filter-chain attachment, serving readiness and live traffic tests are deferred.
Secret paths and projection requirements remain controller-owned internal data;
the current Policy API does not carry workload mounts. Its YAML acceptance is
checked alongside focused tests of the generated PPE fields.

Task IDs are retained: S10, S13 and S15 have moved to deferred core integration.
No active task depends on those tasks. Keep authored changes around 3–5 files,
splitting before coding where needed; include generated output with its source.

## S01 — Confirm the integration baseline and package contract

- [ ] Locate the Tenant/AIGateway input and reconciliation baseline and the core
  Policy API/controller. Record the Policy generation integration points; core
  workload creation and filter-chain wiring are not prerequisites for this milestone.
- [ ] Define package Secret-reference declarations, the trusted resolved-path
  context and structured policy contribution. Select and verify the pinned Helm
  Go rendering API, with schema validation enabled and no external CLI assumption.
- [ ] Confirm API-key defaults, existing grant compatibility and admission
  lifecycle within the agreed single-instance scope. Record unresolved
  prerequisite work and the supported runtime version before relying on either.

**Verification:** inspect source and official library/runtime contracts; review a
worked flat entry, reference declaration, resolved context and resulting policy.
**Dependencies:** none. **Likely areas:** [the plan](../plan.md) and a short package author
contract alongside the design. **Size:** small investigation/documentation slice.

## S02 — Add the flat auth envelope in the AI Gateway layer

- [ ] Preserve the flat `name`/`authType` envelope and arbitrary nested JSON
  through serialization, CRD storage and read-back. Instance names are unique;
  auth types remain open rather than an enum of built-ins.
- [ ] Resolve Tenant authorization by type, including the explicit handling of
  existing grant values agreed in S01. Keep upstream credential grants separate.
- [ ] Generate the CRD/deepcopy output and use API-server validation for structural
  constraints. Reject more than one configured auth instance before publication
  using a runtime capability check, while keeping the CRD list extensible.

**Verification:** unit serialization tests and `make test-integration` prove
unknown fields survive create/get/update and invalid envelopes/grants are handled
as specified. **Dependencies:** S01 and the AI Gateway API baseline.
**Likely areas:** Tenant/AIGateway types, focused API tests and generated files.
**Size:** medium; split grant compatibility from envelope work if required.

## S03 — Discover auth packages generically

- [ ] Build a startup catalog from default and extension roots under
  `aigateway/governance/auth/<authType>/`, keyed by directory name.
- [ ] Validate metadata, schema, reference declarations and mandatory admission
  manifests; reject duplicate IDs and incomplete packages without overriding defaults.
- [ ] Exercise a second package fixture with different input/reference fields to
  prove discovery needs no auth-specific switch, enum or registry file.

**Verification:** hermetic filesystem tests include projected-volume layouts,
missing artifacts, malformed metadata and collisions. **Dependencies:** S01.
**Likely areas:** catalog module, tests and package fixtures. **Size:** medium.

## Checkpoint C1 — Inputs and generic discovery are defined

- [ ] S01–S03 checks pass on the AI Gateway baseline; the core Policy is the
  agreed output and no workload implementation is required to generate it.

## S04 — Create the API-key chart and schema

- [ ] Add the rendering assets under `templates/aigateway/governance/auth/apiKey/`:
  chart metadata, flat input schema, reference declaration and contribution template.
- [ ] Configure the selected header, optional prefix, controller-supplied file
  path, refresh/staleness and `user`/`groups` identity mapping. The chart neither
  chooses mount paths nor creates the complete policy document.
- [ ] Enforce required Secret name/key, field types, unknown-field rejection and
  interval constraints with the agreed defaults. S07 completes the required
  admission artifacts before this package can be registered.

**Verification:** focused schema/template cases cover defaults and overrides,
malformed fields and changed supplied paths. **Dependencies:** S01.
**Likely areas:** default chart rendering assets and focused cases. **Size:** medium;
separate schema and template tests if the authoring surface exceeds five files.

## S05 — Resolve verifier Secrets and allocate file paths

- [ ] Resolve package-declared references in the gateway namespace, check Tenant
  access and selected key existence, and return a typed plan without Secret bytes
  in rendering context, diagnostics, status or policy data.
- [ ] Allocate deterministic intended filenames/mount paths and record internal
  projection requirements. Check planned/reserved-path collisions and layout
  requirements; templates consume trusted paths. Actual workload mounts are deferred.
- [ ] Cover the same Secret used by different instances, distinct Secret/data
  keys, changed references and repeated allocation without hardcoded `keySource`
  traversal. Restrict this implementation to `secretRef`.

**Verification:** resolver/allocation unit tests plus API-server reference tests;
assert stable output, isolation and explicit errors before rendering.
**Dependencies:** S01; gateway reference integration also needs S02.
**Likely areas:** reference resolver, allocator and focused tests. **Size:** medium.

## S06 — Render and validate the default through Helm

- [ ] Invoke the generic renderer with the flat entry and controller-resolved
  context; enable Helm schema and template validation before decoding output.
- [ ] Validate the policy contribution and reject workload bindings, global policy
  replacement and attempts to override trusted context. Never fetch Secret
  contents from a template.
- [ ] Render the real API-key chart and a differently shaped package fixture
  through the same renderer. No schema bypass or API-key-specific dispatch exists.

**Verification:** `make test-unit` with schema rejection cases, supplied-path
changes and malformed contributions. Admission/package registration is tested
only after S07 completes the default's required artifacts.
**Dependencies:** S03–S05. **Likely areas:** renderer/contribution modules, tests
and pinned module dependencies. **Size:** medium; separate dependency setup if needed.

## Checkpoint C2 — Secret references become valid plugin configuration

- [ ] S04–S06 checks pass: templates consume allocated paths and no Secret bytes
  or template-generated workload bindings appear in the contribution.

## S07 — Complete the package's VAP and validation parity

- [ ] Supply the default's mandatory VAP and enforcing binding for flat AIGateway
  `apiKey` entries, covering common fields, required nested references, types,
  unknown fields and refresh/staleness defaults and relationships.
- [ ] Scope admission to the intended AIGateway operations/auth type, leaving
  unrelated types and status updates unaffected.
- [ ] Use shared valid/invalid cases for VAP and Helm validation, and document the
  package inputs, defaults, verifier-record shape and identity mapping.

**Verification:** actual API-server admission tests and Helm parity cases agree,
including multiple-entry shapes independently of first-milestone runtime support.
**Dependencies:** S02, S04, S06. **Likely areas:** package admission manifest,
README, parity fixtures and integration tests. **Size:** medium.

## S08 — Register package admission resources through the controller

- [ ] Apply each discovered package's VAP and enforcing binding once per type,
  using explicit RBAC. Validate allowed resource kinds, auth-type scope and names;
  do not take over unrelated cluster policies.
- [ ] Keep a type unusable when installation, permissions or enforcement checks
  fail. Account for API-server propagation instead of treating a write as proof
  that admission is active.
- [ ] Reconcile policy/binding drift and removal using the lifecycle agreed in S01.
  Registration is separate from per-gateway policy rendering.

**Verification:** envtest covers installation, denied permissions, missing/broken
artifacts, enforcement and recovery. **Dependencies:** S03, S07.
**Likely areas:** admission registrar, RBAC and registration tests. **Size:** medium.

## S09 — Ship the default and wire startup registration

- [ ] Include the complete `apiKey` package in the controller image and load the
  catalog/registrar for the AI Gateway controller path.
- [ ] Define and test startup configuration before using it in deployment
  manifests. Keep default and extension roots separate and compatible with
  non-root/read-only operation.
- [ ] Surface catalog/registration failures and pass only usable packages into
  gateway reconciliation. Package changes require a rollout.

**Verification:** configuration tests and image/startup smoke checks show the
bundled default is discovered and its admission resources are registered without
an installed Helm CLI. **Dependencies:** S03, S06–S08 and the AI controller baseline.
**Likely areas:** manager wiring, Dockerfile, deployment config and tests.
**Size:** medium; split image packaging from startup integration if needed.

## Checkpoint C3 — The default package is registered and validated

- [ ] S07–S09 checks pass: the controller installs VAP/binding resources,
  admission enforces them and Helm repeats validation. Installation is not a Tenant grant.

## S11 — Assemble one policy for the API-key instance

- [ ] Combine the identity contribution, global authenticated-caller requirement
  and credential stripping in one complete document with deterministic names.
- [ ] Retain existing model routes and reject conflicts; an auth contribution
  cannot replace controller-owned global enforcement.
- [ ] Compose an independent model-authorization fixture in that same document
  without editing the auth template or implementing the production Model CEL API.

**Verification:** unit assembly tests preserve the auth requirement and mapped
identity contract while adding route/PDP contributions. Assert the generated
PPE settings against the targeted contract; live enforcement is deferred with S15.
**Dependencies:** S01, S06.
**Likely areas:** shared assembler, tests and concrete contribution fixtures.
**Size:** medium. General OR/precedence is not implemented by this task.

## S12 — Generate the gateway-owned Policy from authorized inputs

- [ ] Resolve the gateway's ready Tenant, require an installed and permitted
  `apiKey` type, then resolve the Secret and allocate paths before invoking Helm.
- [ ] Assemble the document and internal projection requirements before writes;
  create/update exactly one gateway-owned `praxis.io/v1alpha1 Policy` with the
  generated YAML in `spec.document`, without adopting user-owned resources.
- [ ] Report denied permission, unavailable package, invalid input or unresolved
  references through the gateway's condition contract with current generation
  and actionable reasons. Reject unsupported combinations before publication;
  creating a Policy must not make the AIGateway report serving readiness.

**Verification:** envtest creates Tenant + Secret + AIGateway, reads back the core
Policy and checks its owner and generated document. Run the core Policy controller
and verify its acceptance independently of AIGateway serving readiness. Cover
repeat/update, reference/permission failures and no partial output; no Proxy
instance or workload is required.
**Dependencies:** S02, S05, S09, S11 and the AIGateway reconciliation baseline.
**Likely areas:** gateway desired-state generation and focused integration tests.
**Size:** medium; S13 is deferred and does not block Policy generation.

## Checkpoint C4 — The controller outputs a valid core Policy

- [ ] S11–S12 checks pass: exactly one owned Policy is read back from the core
  API, passes its current checks and contains the expected PPE configuration.
  Its creation is not reported as a serving-ready AIGateway.

## S14 — Reconcile Policy output when inputs change

- [ ] Watch Tenant grants, Secret references/data, admission resources and owned
  Policies; reconcile affected outputs without an AIGateway spec edit.
- [ ] Define and test the disposition of an existing owned Policy when permission
  or a required reference is lost. Never publish a fallback document with auth
  removed, and do not claim that a status change withdraws runtime access.
- [ ] Handle reference/instance changes, Policy deletion and input loss/recovery
  idempotently. Preserve user-owned resources and keep serving readiness separate
  from successful output generation. Content-only Secret updates must not embed
  verifier data or cause unnecessary Policy spec changes when its inputs are unchanged.

**Verification:** event-mapping tests and envtest cover generation, loss, recovery,
ownership and cleanup at the Policy boundary. Verify unchanged desired Policy
specs remain stable; actual file reload/traffic revocation stays in deferred S15.
**Dependencies:** S08, S12. **Likely areas:** event/lifecycle handling and focused
integration tests. **Size:** medium.

## S16 — Document and verify the Policy-generation workflow

- [ ] Supply Tenant, verifier Secret and AIGateway examples that produce the
  expected core Policy. Explain package defaults, optional prefix and identity mapping.
- [ ] Explain VAP registration, Helm validation, intended file allocation and
  error feedback. State that mounts, filter-chain attachment and runtime enforcement
  await core integration; a generated Policy does not establish gateway readiness.
- [ ] Run the documented control-plane workflow and relevant unit/integration,
  build and lint checks. Include generated artifacts and the verified Policy output.

**Verification:** apply the inputs against the integration API server and read back
exactly one owned, accepted Policy with the expected PPE configuration. No running
gateway is required. Use source-verified fields/options and obtain permission for
Git-backed checks. **Dependencies:** S12, S14. **Likely areas:** default README,
control-plane guide and samples. **Size:** medium.

## Checkpoint C5 — The control-plane delivery is complete

- [ ] The documented inputs generate and maintain the accepted Policy, lifecycle
  and error cases pass, and relevant build/lint/unit/integration checks are complete.
  No Proxy resource, actual Secret mount or live traffic proof is required here.

## Deferred core integration

The following tasks consume the generated Policy once the core workload API and
reconciliation path are ready. They are not completion gates for this delivery.
Serving readiness, access withdrawal and runtime validation claims belong here.

### S10 — Project controller-allocated files through the core workload path

- [ ] Add or reuse the minimal typed same-namespace Secret file-projection support
  between the AI Gateway generation path and backing Proxy/workload reconciler.
- [ ] Turn the S05 plan into matching read-only directory mounts and selected data
  keys, without `subPath` or copying Secret bytes into generated configuration.
- [ ] Validate collisions, reject invalid projections and preserve update
  propagation. Keep existing upstream credential/environment handling separate.

**Verification:** core API/renderer tests and envtest inspect generated mounts,
reference changes and matching policy paths. **Dependencies:** S01, S05 and the core
workload baseline. **Likely areas:** core API, projection renderer, validation and
tests, plus generated output. **Size:** split into API/validation and workload
rendering slices if more than five authored files are required.

### S13 — Attach policy and file projections to the shared listener

- [ ] Feed the owned Policy and controller-generated projections through the
  existing gateway-to-Proxy path, with an unconditional policy filter before
  model rewrites and provider credential injection.
- [ ] Coordinate policy/workload publication so the listener is not exposed
  without its required security configuration or verifier files.
- [ ] Follow backing Policy/Proxy readiness for the intended generation; do not
  equate a created resource or an available pod with an enforced configuration.

**Verification:** integration tests assert the filter chain, mounts, ownership,
publication failure/recovery and readiness reasons. **Dependencies:** S10, S12
and the existing gateway/core publication path. **Likely areas:** policy wiring,
projection handoff, readiness and focused tests. **Size:** medium.

### S15 — Prove Secret-backed API-key enforcement on a running gateway

- [ ] Through the deployed controller, declare Tenant + Secret + AIGateway and
  show valid keys pass while missing, invalid and expired keys fail across
  inference/MCP on the shared listener. Verify header stripping and mapped identity.
- [ ] Rotate/remove verifier records and exercise malformed-file reload plus the
  staleness ceiling. Allow for projection delay, use bounded eventual assertions
  and record the supported Praxis/PPE version used.
- [ ] Demonstrate Tenant grant withdrawal and recovery from traffic, and verify
  prerequisite errors do not create an unprotected listener. Clean up the scenario.

**Verification:** `make test-e2e` using the existing isolated Kind setup and a
functional upstream. Envtest or successful rendering is not a substitute. Secret
deletion alone does not prove the old projected file disappeared. **Dependencies:**
S07, S13, S14. **Likely areas:** API-key e2e scenario, runtime fixtures and harness
integration. **Size:** split initial access/header assertions from rotation and
withdrawal scenarios if necessary to keep each review bounded.

## Deferred follow-up work

- [ ] Prove and implement multiple-instance OR selection, header precedence,
  conflicting-identity handling and mixed provider-failure behaviour.
- [ ] Demonstrate a mounted production extension without rebuilding, including
  the [MaaS HTTP validator](../../../design/gateway-auth/mass-auth-example.md).
  Generic discovery/rendering fixture coverage remains in this delivery.
- [ ] Add OIDC and Kubernetes TokenReview packages.
- [ ] Implement production Model CEL and external metadata/subscription
  authorization, including the model-without-an-access-rule decision.
- [ ] Add ConfigMap references and, if required, live catalog reload.


---

## Completed work: Proxy e2e tasks

- [x] Implement basic create/traffic, hot reload and listener rollout scenario.
- [x] Supply verified runtime image, isolated cluster and reproducible CI/local setup.
- [x] Execute the live e2e and resolve any observed failures.
- [x] Run required checks and review changes.
