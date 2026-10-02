---
name: md365
description: Use the md365 CLI for Microsoft 365 calendar, contact, mail, Teams, OneDrive, and SharePoint tasks, especially when choosing between local Markdown cache reads and live Graph operations.
---

# md365

Use `md365` for Microsoft 365 calendar, contact, mail, Teams, OneDrive, and SharePoint work.

## Operating Model

- Commands use account names from config, not email addresses. Common local names include `private`, `dcg`, `talendos`, and `oms`.
- Calendar and contact read/search commands are cache-first. Use `--no-cache` when the user asks for live/fresh data or when the local cache may be stale.
- Mail, Teams, OneDrive, and SharePoint reads use Microsoft Graph directly today.
- Use `mail search` for Microsoft Search across the signed-in user's own
  Exchange Online mailbox and supported attachment content. Use `mail list
  --search` for folder-aware chronological search, shared/delegated mailboxes,
  or Microsoft personal accounts.
- Use `files search` for tenant-wide discovery across visible OneDrive and SharePoint content. A team's default drive is not the same as all of its libraries.
- Writes always go through Microsoft Graph: calendar create/delete and mail send.
- Cross-tenant guards use configured account domains. Do not bypass them with `--force` unless the user explicitly asks.
- md365 is a public native client. A Microsoft Entra application (client) ID is
  not a secret. Prefer an account-specific, single-tenant public-client app
  registration for organizational accounts and configure its Microsoft Entra
  tenant ID or verified domain as `tenant`; authorization code login uses S256
  PKCE plus OAuth `state`, while device code flow remains available for headless
  authentication.

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
md365 mail search <query> --account <name> --json
md365 mail search <query> --account <name> --top-results --json
md365 mail get --account <name> --id <message-id> --json
md365 mail attachments --account <name> --id <message-id> --json
md365 teams list --account <name> --json
md365 teams channels --account <name> --team-id <team-id> --json
md365 teams files --account <name> --team-id <team-id> --channel-id <channel-id> --json
md365 onedrive list --account <name> --path <folder-path> --json
md365 files search "Jahresabschluss 2023" --account <name> --json
md365 sharepoint libraries --account <name> --team-id <team-id> --json
md365 sharepoint list --account <name> --team-id <team-id> --json
md365 sharepoint list --account <name> --drive-id <drive-id> --json
```

Use the returned `id` to descend into a folder with `--item-id`. File results
also include `drive_id`, which identifies the OneDrive or SharePoint document
library that owns the item.

When a file is not in the default team library, run `sharepoint libraries` and
use the returned drive ID with `sharepoint list --drive-id`. Search results
already include the owning `drive_id` and stable item ID.

Follow `breadcrumbs` when present. For example, `mail get --json` includes a `list_attachments` breadcrumb when a message has attachments.
