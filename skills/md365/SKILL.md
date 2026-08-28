---
name: md365
description: Use the md365 CLI for Microsoft 365 calendar, contact, and mail tasks, especially when choosing between local Markdown cache reads and live Graph operations.
---

# md365

Use `md365` for Microsoft 365 calendar, contact, and mail work.

## Operating Model

- Commands use account names from config, not email addresses. Common local names include `private`, `dcg`, `talendos`, and `oms`.
- Calendar and contact read/search commands are cache-first. Use `--no-cache` when the user asks for live/fresh data or when the local cache may be stale.
- Mail list/get reads Microsoft Graph directly today.
- Writes always go through Microsoft Graph: calendar create/delete and mail send.
- Cross-tenant guards use configured account domains. Do not bypass them with `--force` unless the user explicitly asks.

## Output

Prefer `--json` for agent workflows. Successful responses use:

```json
{
  "ok": true,
  "data": {},
  "summary": "",
  "meta": {},
  "breadcrumbs": []
}
```

Errors use:

```json
{
  "ok": false,
  "error": "",
  "code": "",
  "hint": ""
}
```

Use `--ids-only` when a later command only needs IDs, and `--count` when only cardinality matters.

## Useful Commands

```bash
md365 about --json
md365 commands --json
md365 auth status --json
md365 sync --account <name> --json
md365 cal list --account <name> --from 2026-01-01 --to 2026-12-31 --json
md365 cal list --account <name> --no-cache --json
md365 contacts search <query> --account <name> --json
md365 contacts search <query> --account <name> --no-cache --json
md365 mail list --account <name> --search <query> --json
md365 mail get --account <name> --id <message-id> --json
md365 mail attachments --account <name> --id <message-id> --json
```

Follow `breadcrumbs` when present. For example, `mail get --json` includes a `list_attachments` breadcrumb when a message has attachments.
