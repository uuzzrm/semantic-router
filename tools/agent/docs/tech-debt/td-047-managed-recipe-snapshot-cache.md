# TD047: Managed Recipe Requests Reparse the Full Source Snapshot

## Status

Open.

## Owner Plan

PL0041 Managed Recipe Dashboard.

## Release Relevance

Post-MVP scalability for large community probe manifests.

## Scope

Dashboard backend active Recipe reads only.

## Summary

Recipe list responses are server-paged and details are lazy, but every Recipe
API request currently rereads and reparses all five source files before slicing
the requested page or resolving one probe.

## Evidence

`dashboard/backend/recipe/service.go` calls `loadSnapshot` from descriptor,
list, detail, run-plan, and validate operations. `loadSnapshot` reads the fixed
files, decodes the complete probe manifest and config projection, and rebuilds
the flattened index and facets.

## Why It Matters

Response size remains bounded, but YAML parsing and index construction are
still O(N) in total probe count for every page, detail, and action. Large
community manifests can therefore increase Dashboard latency and CPU even when
the client requests a small page.

## Desired End State

Keep one immutable, process-scoped parsed snapshot keyed by a safe five-file
fingerprint. Invalidate atomically when any fixed source changes, retain the
current digest precondition for actions, and never cache expanded credentials
or runtime-normalized configuration.

## Exit Criteria

- Repeated list/detail/action requests reuse a parsed snapshot while all five
  files are unchanged.
- A source change atomically invalidates metadata, probe indexes, facets, and
  digests together.
- Concurrency, symlink rejection, size bounds, and stale-action tests cover the
  cache boundary.
