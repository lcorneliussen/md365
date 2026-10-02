# md365 command contract

`md365 schema` prints the versioned command contract as deterministic JSON.
`md365 schema --json` wraps that document in md365's stable response envelope.

The `1.x` contract contains:

- every non-hidden Cobra command, alias, positional argument, and flag;
- required/default flag metadata and inherited global flags;
- least-privilege delegated Microsoft Graph permissions and feature bundles;
- central `read`/`write` mutability, prompting behavior, and observable effects;
- per-command supported output modes, invocation constraints, and stable
  error/exit-status mappings.

Within a schema major version, fields can be added but existing field meanings
and enum values are not changed incompatibly. A breaking contract change
requires a new major `schema_version`.

The schema is generated from the Cobra command tree, capability catalog, and
execution-policy catalog at runtime. Tests compare that generated surface with
the legacy `commands --json` catalog and fail when a runnable command lacks an
explicit execution policy, preventing command/documentation drift in CI.
