# Automation output and exit contract

md365 exposes a stable automation surface for scripts and agents that work
with Microsoft 365 resources through Microsoft Graph.

## Output modes

- `--json` returns the response envelope (`ok`, `data`, and optional
  `summary`, `meta`, and `breadcrumbs`).
- `--results-only` returns only `data` as valid JSON, regardless of its type.
- `--quiet` retains its legacy compact behavior for backward compatibility;
  string and string-list results are written as unquoted lines.
- `--ids-only` writes one stable resource ID per line.
- `--count` writes only the number of results.

Choose one output mode. `--select` and `--fail-empty` can be composed with
`--json`, `--results-only`, or the backward-compatible `--quiet` mode.

## Projection and empty results

`--select id,subject,from` projects returned objects or each object in a
collection. Dotted paths preserve their object shape when a response contains
nested objects. A requested field
that is not present fails with `usage`; md365 never invents a value.
Projection is rejected for mutating commands so an output-shape error can never
be reported after a Microsoft 365 write has already completed. Use `--dry-run`
to project a supported mutation preview safely.

`--fail-empty` turns an empty collection, map, string, or null result into the
stable `empty_result` error and exit status 11. Without this flag, an empty
result is successful.

## Collection metadata

JSON envelopes for collections include:

- `meta.count`: number of resources in the returned page;
- `meta.has_more`: `true` or `false` when the command can determine it, or
  `null` when the backing operation does not expose that state;
- `meta.total`: present only when the complete result count is known;
- `meta.continuation`: reserved for an opaque, reusable md365 cursor.

Commands with `--limit` use one-resource lookahead, so `has_more` is accurate.
md365 does not expose Microsoft Graph `@odata.nextLink` URLs as public cursors.

## Stable exit statuses

| Status | Code | Meaning |
|---:|---|---|
| 0 | `ok` | Successful command |
| 1 | `usage` / `unknown` | Invalid invocation or unclassified error |
| 2 | `not_found` | Microsoft 365 resource not found |
| 3 | `auth` | Microsoft Entra authentication required or rejected |
| 4 | `forbidden` | Delegated permission or access denied |
| 5 | `rate_limit` | Microsoft Graph throttled the request |
| 6 | `network` | Transport or response-read failure without a Graph error status |
| 7 | `graph` | Other Microsoft Graph error |
| 8 | `policy_denied` | md365 execution policy blocked the command |
| 9 | `conflict` | Graph conflict or failed precondition |
| 10 | `retryable` | Transient Graph response that can be retried |
| 11 | `empty_result` | `--fail-empty` found no results |

Structured Graph errors also include `http_status` when a response status is
available. Error code names already published in the schema 1.x contract are
retained for backward compatibility.
