# PL-0041 Managed Recipe Dashboard

## Goal

Evolve the maintained five-file recipe directory into the single active
Mixture-of-Models management surface for local `vllm-sr serve`: expose recipe
metadata and probes through the Dashboard, validate probes against the live
Router Eval API, and launch exact probe requests in the existing Playground.

## Scope

- Add and validate required `metadata.yaml` files for maintained recipes.
- Change maintained-recipe conformance from four required files to five.
- Make the local serve stack expose fixed sibling recipe assets to Dashboard.
- Add a narrow, process-scoped active Recipe service and read APIs.
- Extend the existing Mixture-of-Models page with Overview and Probes views.
- Add server-side probe pagination, detail loading, and strict route validation.
- Add a typed Playground invocation for probe Run and Edit actions.
- Add focused backend, frontend, CLI, contract, and E2E coverage.

## Non-goals

- No multi-package catalog, upload, installation, activation, or hot switching.
- No multi-tenant package, credential, or provider isolation.
- No new provider-model or credential schema.
- No benchmark scorecard or Evaluation schema change in this plan.
- No generated response during probe validation.
- No parsed-snapshot cache or runtime-hash evaluation receipt in this plan;
  those follow-ups are tracked explicitly as TD047 and TD048.

## Exit Criteria

- Every maintained recipe has schema-valid metadata and passes exact five-file
  conformance.
- `vllm-sr serve --config <recipe>/config.yaml` exposes only that recipe's
  fixed sibling assets to Dashboard without scanning adjacent directories.
- `GET /api/recipe` reports the active recipe descriptor and source health.
- Probe list and detail endpoints handle large manifests with server paging and
  filtering.
- Validate calls the live Router `/api/v1/eval` contract and never simulates a
  successful result.
- Run opens a clean Playground conversation and sends the materialized probe;
  Edit creates an editable draft without sending it.
- Existing entrypoint/recipe editing, provider model management, topology, and
  ordinary Playground behavior remain compatible.
- Applicable harness, CLI, recipe, Dashboard, integration, and local smoke
  gates pass.

## Task List

- [x] MRD-01: Create a clean worktree and branch from current `upstream/main`.
- [x] MRD-02: Add metadata schema, maintained metadata files, and five-file
      conformance.
- [x] MRD-03: Add active recipe path resolution and local container mounts.
- [x] MRD-04: Add Recipe service, descriptor API, and focused backend tests.
- [x] MRD-05: Add paginated probe list/detail and strict Eval validation APIs.
- [x] MRD-06: Refactor the existing MoM page and add Overview and Probes views.
- [x] MRD-07: Add Playground invocation support for probe Run and Edit.
- [x] MRD-08: Add behavior-visible integration/E2E coverage.
- [x] MRD-09: Run the complete validation ladder and fix all regressions.
- [x] MRD-10: Prepare signed-off reviewable commits.

## Next Action

Ready for review.

## Validation

- `make agent-ci-gate`: passed, including `agent-validate`, `agent-lint`,
  `vllm-sr-test`, `recipe-conformance-static`, `test-semantic-router`, and
  `dashboard-check`.
- Recipe conformance: 35 Python tests passed; all 7 maintained recipes,
  58 decisions, 11 entrypoints, and 275 probe variants validated.
- Dashboard: ESLint and type-check passed; 108 frontend test files with
  378 tests passed; backend lint and module checks passed.
- Managed Recipe Playwright coverage: 3 tests passed for Overview/probe
  browsing and Validate, Edit, and Run.
- YAML lint, Markdown lint, and the security AST scanner passed.
- Container-backed CLI integration and live smoke coverage run in the
  pull-request CI environments; all unit, contract, and non-container
  integration coverage passed before submission.

## Operating Rules

- One Router/Dashboard process owns one active recipe directory.
- Production code does not import `tools/agent`; CI and production validation
  consume the same versioned contract from an owned schema seam.
- Keep handlers thin, keep `ChatComponent.tsx` orchestration narrow, and split
  the existing large MoM section before adding new views.
- Never put provider credentials or expanded secret values in metadata or
  Recipe API responses.
- Preserve bare-config compatibility; only a managed Recipe receives metadata,
  probes, and Recipe identity.
- Keep this plan current until the branch is ready for review or the work is
  explicitly blocked.

## Related Docs

- [Agent harness](../README.md)
- [Module boundaries](../module-boundaries.md)
- [Testing strategy](../testing-strategy.md)
- [Feature complete checklist](../feature-complete-checklist.md)
- [Entrypoints and multi-recipe routing](pl-0038-entrypoints-recipes.md)
- [Maintained recipe conformance CI](pl-0040-recipe-conformance-ci.md)
- [Managed Recipe snapshot cache debt](../tech-debt/td-047-managed-recipe-snapshot-cache.md)
- [Recipe validation provenance debt](../tech-debt/td-048-recipe-validation-runtime-provenance.md)
