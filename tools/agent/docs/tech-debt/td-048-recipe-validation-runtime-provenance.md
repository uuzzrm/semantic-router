# TD048: Recipe Validation Does Not Bind Runtime Configuration Provenance

## Status

Open.

## Owner Plan

PL0041 Managed Recipe Dashboard.

## Release Relevance

Post-MVP reproducibility and future MoM evaluation receipts.

## Scope

Dashboard Recipe Validate results and Router configuration identity.

## Summary

Validate binds an action to the selected five-file source digest and checks the
live Router Eval result, but the result does not record the Router source,
generated-runtime, or active configuration hashes.

## Evidence

`dashboard/backend/recipe/service.go` returns `recipe_digest` with validation
results and calls `/api/v1/eval?trace=true`. The Router already exposes
`GET /config/hash` with source, runtime, active hashes, and activation status,
but the Recipe service does not consume it.

## Why It Matters

Local runtime overrides and later config activation can legitimately differ
from the distributable Recipe source. A passing route assertion proves live
behavior at that moment, but is not yet a reproducible receipt tying the result
to one active runtime realization.

## Desired End State

Record explicit source-package and active-runtime provenance around validation,
including environment-bound runtime overrides. Treat unavailable or changing
provenance as unverified rather than claiming a reproducible result, while
preserving compatibility with older Router versions.

## Exit Criteria

- Validation exposes package, source-config, generated-runtime, and active
  Router hashes with a defined verification status.
- Hash observations cannot silently turn a mismatched or mid-activation Router
  into a green verified result.
- The provenance contract composes with future immutable benchmark run
  receipts without putting environment credentials into `metadata.yaml`.
