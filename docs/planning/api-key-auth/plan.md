# Implementation plan: generate a core Policy for Secret-backed API-key auth

Source design: [Gateway authentication](../../design/gateway-auth/gateway-authentication.md).
Task checklist: [Policy-generation tasks](tasks/todo.md#gateway-authentication-implementation).
Follow-up example: [MaaS HTTP validation](../../design/gateway-auth/mass-auth-example.md).

## Goal

**Given a Tenant-authorized AIGateway with one `apiKey` authentication instance
and a referenced Secret, the controller creates and maintains one valid,
gateway-owned core `Policy` resource (`praxis.io/v1alpha1`).** Its `spec.document`
contains the generated PPE API-key authentication configuration.

This is a control-plane delivery. It is complete when that Policy object is
correctly generated, accepted by the core API and verified by tests. Proxy
creation, filter-chain attachment, actual Secret mounting, serving readiness and
live traffic enforcement are later core integration work and do not block this
milestone. Creating the Policy must not make the AIGateway report serving readiness.

The agreed scope is one configured `apiKey` instance per AIGateway. Its Secret can
contain multiple verifier records for consumers and rotation. Reject multiple
configured instances before publication; keep the API list extensible for later
OR selection. Keep the generic package contract and discovery, with a second
package fixture proving dispatch without auth-specific controller code.

## Prerequisites and output boundary

The inspected checkout registers the core Proxy and Policy APIs, but not the
Tenant/AIGateway layer. The [test structure guide](../../../test/README.md#api-layers)
identifies the `aigateway` development branch as that layer's home. S01 locates
its implementation and records the integration points. Publishing a gateway-owned
Policy still requires those input resources and their reconciliation path;
package/rendering work can start independently.

The core Policy API already accepts a YAML `spec.document` and optional filter
settings. Its current controller checks YAML parsing and records acceptance;
that status does not establish PPE runtime validity or traffic enforcement.
Generated-document tests must also assert the expected plugin settings, identity
mapping, authentication requirement and credential stripping.

The Policy API has no workload-mount fields. The auth controller allocates the
intended file path and supplies it to rendering, retaining reference/path
requirements in an internal plan for future integration. This milestone neither
adds mounts to Policy nor introduces a new binding resource. The generated path
need not exist in a running container for this control-plane artifact to be valid.
Actual projection must later honor the same path contract.

Related boundaries: [API-key mapping #52](https://github.com/praxis-proxy/gateway-controller/issues/52),
[shared policy wiring #51](https://github.com/praxis-proxy/gateway-controller/issues/51),
[AIGateway API #34](https://github.com/praxis-proxy/gateway-controller/issues/34),
[AIGateway reconciliation #35](https://github.com/praxis-proxy/gateway-controller/issues/35)
and [core control plane #11](https://github.com/praxis-proxy/gateway-controller/issues/11).
These identify responsibilities, not current completion claims. The unfinished
core workload path is a future consumer, not a prerequisite for Policy generation.

## Delivery sequence

1. **Contract and inputs — S01–S03:** establish the AI Gateway input baseline,
   define the package and trusted-context contracts, add the flat auth API/Tenant
   grant handling, and discover packages generically.
2. **Default rendering — S04–S06:** create the API-key chart, resolve Secret
   references and allocate intended paths, then render through Helm validation.
3. **Admission and installation — S07–S09:** complete the mandatory VAP/binding,
   test admission/render parity, register admission resources through the
   controller and ship the default package. Failed registration prevents use.
4. **Policy output — S11–S12:** assemble the document and reconcile exactly one
   gateway-owned core Policy from authorized inputs.
5. **Lifecycle and completion — S14, S16:** test changes and errors at the Policy
   boundary and document the Tenant + Secret + AIGateway to Policy workflow.

S10 (core file projection), S13 (listener attachment) and S15 (live gateway tests)
remain explicit deferred integration tasks. Their IDs are retained for tracking;
no current task depends on them. Checkpoints in the checklist assess only the
active control-plane deliverable.

## Contracts to preserve

- Flat auth entries use reserved `name` and `authType`, with package-owned fields
  such as `header`, optional `prefix`, `keySource.secretRef`, `refreshSeconds` and
  `maxStalenessSeconds`. Preserve arbitrary JSON for future types; handle existing
  Tenant grant values explicitly rather than silently changing their casing.
- Discover `aigateway/governance/auth/<authType>/` from default and extension roots
  at startup. Reject duplicate IDs and incomplete packages; use no separate
  registry or auth-specific switch. Default packages are controller-maintained.
- Every package supplies a VAP and enforcing binding. The controller applies them
  once per type with explicit permissions and enables the type only after
  registration/enforcement checks. Helm schema/template validation runs on every
  render; shared cases check equivalence between the two layers.
- Packages declare Secret input locations. The controller checks same-namespace
  access and data-key existence, allocates stable paths and supplies trusted
  context. Templates cannot override that context or create workload bindings.
  Secret contents stay out of rendering, policy documents, status and logs.
- Allocation accounts for namespace, Secret/data key and consuming instance,
  rejects conflicts within the planned files/reserved paths, and records future
  projection requirements. Final workload-wide mount checks are deferred with
  core integration. Verifier Secrets remain distinct from upstream credentials.
- The default emits `identity/api-key` with `provider.kind: file`, the allocated
  path and refresh/staleness settings. Map `user` to `subject.id` and `groups` to
  `subject.roles`. The assembler adds global authentication,
  `require(authenticated)` and caller-header stripping to the same document.
- A route-authorization fixture verifies future composition without template
  placeholders or production Model CEL support. Existing model contributions,
  where available, must not be overwritten by the auth generator.
- Invalid input or lost permission cannot generate a new valid-looking fallback
  Policy with authentication removed. Reconcile owned output and input errors
  consistently; Policy generation and AIGateway serving readiness remain distinct.

## Decisions to close in S01

Define the reference declaration format, reserved context keys and contribution
shape, then select a pinned Helm Go API from official documentation. Confirm the
expected defaults (`X-API-Key`, 5-second refresh, 30-second staleness, staleness at
least refresh) against the targeted PPE contract. No Helm CLI is assumed in the
controller image, and Secret contents are never template inputs.

Record admission propagation/drift/upgrade handling and the disposition of an
existing owned Policy when its Tenant grant or Secret reference becomes invalid.
Tests must distinguish publication of current desired output from the future
runtime's response to Policy removal or changes. Do not claim runtime access
withdrawal in this milestone.

## Verification and definition of done

Unit tests cover JSON preservation, catalog discovery, schema validation,
reference allocation/collisions, rendering and assembly. The generated PPE YAML
must contain the expected plugin/header, file path, mapping, intervals,
`on_error: fail`, global authentication and credential stripping. It must contain
no Secret bytes. Core YAML acceptance alone is not enough to prove those fields.

Integration tests follow [test/README.md](../../../test/README.md) and run the real API
server. They create Tenant, Secret and AIGateway inputs; verify VAP registration
and rejection; exercise Helm validation and reference failures; and read back the
owned Policy from the core API. Check repeat reconciliation, input updates,
ownership conflicts, dependency loss/recovery and cleanup. Run the existing core
Policy controller to observe acceptance without manually writing its status.

Completion requires all of the following:

- A valid, authorized input produces exactly one correctly owned core Policy.
- The Policy passes core API validation and the core controller's current checks;
  focused tests establish the generated PPE configuration's expected structure.
- Invalid input, missing Secret/key, denied grants, failed admission registration
  and unsupported auth combinations do not produce partially generated output.
- Intended paths are deterministic and consistent with the internal projection
  requirements; no actual Pod, mount or Proxy instance is required for this proof.
- Changes reconcile idempotently, Secret contents remain confidential, and Policy
  generation is not reported as a serving-ready AIGateway.
- The documented control-plane example and relevant unit/integration/build/lint
  checks pass with generated outputs included.

Commands verified for this plan are `make test-unit`, `make test-integration`,
`make lint`, `make lint-extra`, `make build`, `make generate` and `make manifests`.
A live gateway e2e suite is not a completion gate for this milestone. `make verify`
and `make test` invoke Git internally and require permission before execution.
No Git commands are run by this planning change.

## Deferred integration and broader providers

- S10, S13 and S15: core Secret projection, Proxy/filter-chain attachment, serving
  readiness, runtime rotation/staleness, grant withdrawal and live traffic tests.
- Multiple auth instances and general OR selection, credential precedence,
  conflicting identities and mixed provider failures.
- Deployed mounted-extension proof and MaaS HTTP validation; generic fixture
  coverage remains active. OIDC and TokenReview packages are follow-up work.
- Production Model CEL/metadata authorization and subscription lookup, ConfigMap
  references and live catalog reload.


---

# Completed work: Proxy basic end-to-end coverage

## Scope

Exercise a real Proxy through the deployed controller and Praxis runtime in a dedicated kind cluster. Existing controller changes are committed; do not modify unrelated local experiments.

## Acceptance criteria

1. Create a fresh namespace and an inline static-response Proxy. Assert its ConfigMap, Deployment, Service and Ready status, then read its response through the Kubernetes Service proxy.
2. Edit the inline response. Wait for actual traffic to change while Deployment pod template, pod UIDs and container restart counts stay unchanged.
3. Change the listener port. Wait for a new pod set and successful response on the updated Service port.

## Decisions

- Use existing Ginkgo/client-go dependencies. Kubernetes Service proxy avoids an extra curl image; it tests Service backend selection and the running runtime, not ClusterIP routing through kube-proxy.
- Define and document an explicit test image setting; pin the CI runtime by verified digest and allow a deliberate local override.
- ConfigMap projection is asynchronous: use bounded eventual checks with sufficient propagation time.
- No skip on missing prerequisites, no silent image fallback. Clean up test namespaces and isolate kubeconfig from existing clusters.

## Verification

Compile the e2e suite; run it on a dedicated cluster; lint with the repository-pinned linter; run unit tests and vet/build. Review assertions against transient reload and rollout observations.

## Results

- Dedicated kind cluster gateway-controller-dev on Kubernetes1.37.0; isolated kubeconfig /tmp/gateway-controller-pr82-e2e.kubeconfig. Original default context remains kind-kserve-integration.
- Full race-enabled e2e suite: 4/4 specs passed, no skips. New Proxy scenario completed in101seconds, including asynchronous projection and rollout.
- Runtime image: Praxis0.5.6 pinned to verified Linux/amd64 manifest digest in Makefile; execution worked under local ARM emulation.
- Repository-pinned golangci-lint2.15.0: zero issues. Compile, gopls and go vet for e2e passed.
- Namespace cleanup succeeded; dedicated controller cluster remains available. No Git commands, commits, pushes or public posts made for this task.
