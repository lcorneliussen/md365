# md365 command contract

`md365 schema` prints the versioned command contract as deterministic JSON.
`md365 schema --json` wraps that document in md365's stable response envelope.

The `1.x` contract contains:

- every non-hidden Cobra command, alias, positional argument, and flag;
- required/default flag metadata and inherited global flags;
- least-privilege delegated Microsoft Graph permissions and feature bundles;
- central `read`/`write` mutability, invocation-aware prompting behavior,
  dry-run support, and observable effects;
- per-command supported output modes, invocation constraints, and stable
  error/exit-status mappings.

Within a schema major version, fields can be added but existing field meanings
and enum values are not changed incompatibly. A breaking contract change
requires a new major `schema_version`.

The schema is generated from the Cobra command tree, capability catalog, and
execution-policy catalog at runtime. Tests compare that generated surface with
the legacy `commands --json` catalog and fail when a runnable command lacks an
explicit execution policy, preventing command/documentation drift in CI.

Only error codes currently emitted by md365 appear in `exit_statuses`. Reserved
API error constants are not part of the published contract until runtime
classification emits them consistently.

The `policy_denied` status (8) is emitted when `--read-only`, `--no-input`, or
`--dry-run` rejects an invocation. Dry-run output describes the Microsoft 365
workload, Graph operation and resource, method, normalized request fields, and
redactions. It never includes access/refresh tokens and replaces mail/calendar
body content with its length.
