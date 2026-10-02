# md365

AI- and human-friendly CLI for Microsoft 365. Syncs calendars and contacts as local Markdown files and provides live access to mail, Teams, OneDrive, and SharePoint.

## The Problem

If you run an AI agent that needs access to your Microsoft 365 data, you're stuck with the Graph API: OAuth token management, pagination, rate limits, and multi-second roundtrips for every lookup. That's fine for write operations, but for reads — "when's my next meeting?" or "what's Jane's email?" — it's way too much overhead.

## The Solution

md365 syncs your M365 calendars and contacts to local Markdown files with YAML frontmatter. Your agent reads local files. Writes still go through the API.

```bash
# Sync once
md365 sync

# Then search however you want
rg "jane doe" ~/.local/share/md365/
grep -r "team sync" ~/.local/share/md365/*/calendar/
cat ~/.local/share/md365/work/contacts/jane-doe.md
```

No tokens needed for reads. No API calls. Just files.

## How It Works

```
~/.local/share/md365/
├── work/
│   ├── calendar/
│   │   ├── 2026-02-24-team-sync.md
│   │   └── ...
│   └── contacts/
│       ├── jane-doe.md
│       └── ...
└── personal/
    ├── calendar/
    └── contacts/
```

### Calendar Event

```markdown
---
id: AAMkAGEx...
account: work
subject: Team Sync
start: 2026-02-24T16:00:00+01:00
end: 2026-02-24T18:00:00+01:00
location: https://zoom.us/j/123456
organizer: colleague@company.com
attendees:
  - colleague@company.com
  - you@company.com
response: accepted
online_meeting: true
last_modified: 2026-02-18T10:30:00Z
---

# Team Sync

Weekly team synchronization meeting.
```

### Contact

```markdown
---
id: AAMkAGE4...
account: personal
display_name: Jane Doe
emails:
  - jane@example.com
phones:
  - "+49 123 456 789"
company: Acme Corp
job_title: Engineer
---

# Jane Doe

📧 jane@example.com
📱 +49 123 456 789
🏢 Acme Corp — Engineer
```

## Usage

```bash
md365 about                             # Explain the read model, cache, and account conventions
md365 commands --json                   # Inspect command/flag surface
md365 schema --json                     # Versioned command, permission, and safety contract
md365 skill install                     # Install the md365 agent skill

md365 sync                              # Sync all accounts
md365 sync --account work               # Sync one account

md365 cal list                           # Upcoming events (14 days)
md365 cal list --from 2026-02-24 --to 2026-02-28
md365 cal list --search sync

md365 cal create --account work \        # Create event via API
  --subject "Lunch" \
  --start "2026-03-01T12:00" \
  --end "2026-03-01T13:00"

md365 cal delete --account work --id <event-id>

md365 contacts search doe               # Search local contacts

md365 mail list --account work           # Recent mailbox messages
md365 mail list --account work --search bauer
md365 mail search "quarterly forecast" --account work
md365 mail search "project atlas" --account work --top-results
md365 mail list --account work --from-addr colleague@company.com --since 2026-05-01
md365 mail get --account work --id <message-id>
md365 mail attachments --account work --id <message-id>

md365 teams list --account work
md365 teams channels --account work --team-id <team-id>
md365 teams files --account work --team-id <team-id> --channel-id <channel-id>

md365 onedrive list --account work
md365 onedrive list --account work --path "Projects/Current"
md365 onedrive list --account work --item-id <folder-item-id>

md365 files search "Jahresabschluss 2023" --account work

md365 sharepoint libraries --account work --team-id <team-id>
md365 sharepoint list --account work --team-id <team-id>
md365 sharepoint list --account work --site-id <site-id> --path "Projects/Current"
md365 sharepoint list --account work --drive-id <drive-id>

md365 mail send --account work \         # Send mail via API
  --to "colleague@company.com" \
  --subject "Hello" --body "Text"

md365 mail draft --account work \        # Create draft without sending
  --to "colleague@company.com" \
  --subject "Draft" --body "Text"

md365 auth login --account work          # Device code OAuth login
md365 auth status                        # Token status
```

`sharepoint list --team-id` and `--site-id` browse the default document
library. Use `sharepoint libraries` to discover additional libraries such as
`Datenraum`, then pass the returned ID to `sharepoint list --drive-id`.
`files search` searches all visible OneDrive and SharePoint content and returns
stable item, drive, and site IDs for follow-up commands.

`mail search` uses the Microsoft Search API across the signed-in user's own
Exchange Online mailbox, including supported attachment content. Results are
newest-first unless `--top-results` promotes Outlook's most relevant matches.
Use `mail list --search` instead for folder-aware chronological queries,
shared/delegated mailboxes, or Microsoft personal accounts.

### Agent-Friendly Output

Most commands support structured output:

```bash
md365 cal list --account work --json
md365 contacts search doe --ids-only
md365 mail list --account work --count
```

`md365 schema --json` is the authoritative automation contract. It describes
aliases, positional arguments, flags/defaults, delegated Microsoft Graph
permissions, feature bundles, read/write mutability, prompting behavior,
per-command output modes, invocation constraints, and exit statuses. The compatibility policy is documented in
[`docs/command-schema.md`](docs/command-schema.md).

JSON success responses use a stable envelope with `ok`, `data`, optional
`summary`, `meta`, and `breadcrumbs`. Errors use `ok: false`, `error`, `code`,
and optional `hint`.

## Cross-Tenant Guard

If you manage multiple accounts, md365 prevents you from accidentally sending mail or creating events from the wrong one. Configure associated domains per account:

```yaml
accounts:
  work:
    domains:
      - company.com
  personal:
    domains:
      - gmail.com
```

Sending from `personal` to `colleague@company.com` will be blocked with a suggestion to use `--account work`. Override with `--force`.

## Setup

### 1. Add an Account

Interactive setup (guided TUI):
```bash
md365 auth add -i
```

Or non-interactive (AI-friendly):
```bash
md365 auth add --name work --hint you@company.com \
  --scopes "Calendars.ReadWrite,Contacts.ReadWrite,User.Read,Mail.Read,Mail.Send" \
  --domains "company.com" --login
```

Instead of assembling raw Microsoft Graph scopes, select the md365 commands or
feature bundles you intend to use. md365 resolves them to the least-privilege
scope set:

```bash
# Preview permissions without changing configuration or logging in
md365 auth plan --feature mail-manage --command "cal *"

# Configure a new account from feature bundles
md365 auth add --name work --hint you@company.com \
  --feature mail-manage,mail-send,calendar,sync \
  --domains "company.com" --login

# Incrementally request the permission needed by another command
md365 auth login --account work --command "mail send"

# Show token health and available/blocked commands for every account
md365 auth status

# Explain the same capability details for one account
md365 auth explain --account work
```

Available feature bundles currently include `mail-read`, `mail-manage`,
`mail-send`, `calendar-read`, `calendar`, `teams-read`, `files-read`, and
`sync`. Raw `--scope`/`--scopes` flags remain available as an expert escape
hatch.

md365 ships with a built-in public-client app registration — no Microsoft Entra
setup is needed for personal use. A native-app client ID identifies the app; it
is not a secret and does not grant access without user authorization and a
token.

For an organization, prefer an account-specific, single-tenant Microsoft Entra
app registration. Enable public client flows, register a loopback redirect URI
such as `http://localhost`, add only the delegated Microsoft Graph permissions
shown by `md365 auth plan`, and apply the tenant's user/admin consent policy.
Set that registration's application (client) ID as `client_id` on the matching
md365 account and set `tenant` to the Microsoft Entra tenant ID or verified
domain. This limits app-identity, consent, and quota impact to the tenant;
it does not turn the client ID into a credential.

### 2. Login and Sync

```bash
md365 auth login --account work
md365 sync
```

### Auth Flows

Most tenants work with the default **device code flow**. If Microsoft Entra
Conditional Access blocks it, use the **authorization code flow with S256 PKCE**.
md365 also binds the loopback callback to the initiating browser flow with a
fresh OAuth 2.0 `state` value:

```yaml
accounts:
  work:
    tenant: "contoso.onmicrosoft.com" # or the Microsoft Entra tenant GUID
    client_id: "YOUR_SINGLE_TENANT_APPLICATION_CLIENT_ID"
    auth_flow: authcode    # opens browser instead of device code
    hint: you@company.com
    scope: "offline_access Calendars.ReadWrite User.Read"
```

### Configuration

Config lives at `~/.config/md365/config.yaml`:

```yaml
accounts:
  work:
    hint: "you@company.com"
    scope: "offline_access Calendars.ReadWrite Contacts.ReadWrite User.Read Mail.Read Mail.Send"
    domains:
      - company.com
  personal:
    hint: "you@outlook.com"
    scope: "offline_access Calendars.ReadWrite Contacts.ReadWrite User.Read"
    domains:
      - gmail.com
```

## Token Storage

Tokens are stored exclusively in the system keyring (gnome-keyring, macOS Keychain, Windows Credential Manager). A running keyring daemon is required — no file fallback.

The `offline_access` scope enables refresh tokens, so you only need to log in once per account. Tokens refresh automatically on use and remain valid for up to 90 days of inactivity.

## Installation

### Homebrew (macOS & Linux)

```bash
brew install lcorneliussen/md365/md365
```

### AUR (Arch Linux)

```bash
yay -S md365-bin
```

### Go Install

```bash
go install github.com/lcorneliussen/md365@latest
```

### GitHub Releases

Download pre-built binaries for Linux, macOS, and Windows from [Releases](https://github.com/lcorneliussen/md365/releases).

### Build from Source

```bash
git clone https://github.com/lcorneliussen/md365.git
cd md365
go build -o md365 .
```

## Sync Details

- **Events:** Full window sync (past 30 → future 90 days). Remotely deleted events are removed locally.
- **Contacts:** Delta sync via Graph API for incremental updates.
- **Direction:** One-way (remote → local). Local files are a read-only cache.

## License

MIT
