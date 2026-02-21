# md365

A Unix-style Markdown client for Microsoft 365. Plain text storage, searchable with standard tools.

## Philosophy

- **Read local, write remote** — calendars and contacts sync as Markdown files; mutations go through Graph API
- **Plain text storage** — YAML frontmatter + Markdown body, one file per item
- **Unix-like** — `rg`, `fzf`, `grep` — use whatever you want to search
- **Cross-tenant guard** — prevents accidentally emailing/scheduling from the wrong account
- **Minimal dependencies** — single Go binary, no runtime needed

## Storage Layout

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

## File Formats

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
given_name: Jane
surname: Doe
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
# Sync all accounts
md365 sync

# Sync specific account
md365 sync --account work

# List upcoming events
md365 cal list
md365 cal list --from 2026-02-24 --to 2026-02-28
md365 cal list --account work --search sync

# Or just use standard Unix tools!
rg "team sync" ~/.local/share/md365/

# Create event (via API, syncs locally)
md365 cal create --account work \
  --subject "Lunch" \
  --start "2026-03-01T12:00" \
  --end "2026-03-01T13:00" \
  --location "Restaurant" \
  --attendees "colleague@company.com"

# Delete event
md365 cal delete --account work --id <event-id>

# Search contacts (from local cache)
md365 contacts search doe

# Send mail
md365 mail send --account work \
  --to "colleague@company.com" \
  --subject "Hello" \
  --body "Message text"
```

## Cross-Tenant Guard

md365 prevents accidentally emailing or scheduling from the wrong account. Configure associated domains per account:

```yaml
accounts:
  work:
    domains:
      - company.com
      - subsidiary.com
  personal:
    domains:
      - gmail.com
      - outlook.com
```

If you try to send from `personal` to `colleague@company.com`, md365 will block it and suggest using `--account work` instead. Override with `--force` if intended.

## Setup

### 1. Register an Azure AD App

1. Go to [Azure Portal](https://portal.azure.com) → App registrations → New registration
2. Name: anything (e.g., "md365")
3. Supported account types: "Accounts in any organizational directory and personal Microsoft accounts"
4. Redirect URI: leave empty (we use device code flow)
5. Under "Authentication": Enable "Allow public client flows"
6. Under "API permissions": Add these **delegated** permissions:
   - `Calendars.ReadWrite`
   - `Contacts.ReadWrite`
   - `User.Read`
   - `Mail.Send` (if you want to send mail)
   - `People.Read` (optional, for people search)

### 2. Configure md365

Create `~/.config/md365/config.yaml`:

```yaml
accounts:
  work:
    client_id: "YOUR_APP_CLIENT_ID"
    hint: "you@company.com"
    scope: "Calendars.ReadWrite Contacts.ReadWrite User.Read Mail.Send"
    domains:
      - company.com
  personal:
    client_id: "YOUR_APP_CLIENT_ID"
    hint: "you@outlook.com"
    scope: "Calendars.ReadWrite Contacts.ReadWrite User.Read"
    domains:
      - outlook.com
      - gmail.com
```

> **Note:** You can use the same `client_id` for all accounts if they're in the same Azure AD app, or different ones per account.

### 3. Authenticate

```bash
md365 auth login --account work
md365 auth login --account personal
md365 auth status
```

### 4. First sync

```bash
md365 sync
```

## Installation

### From source

```bash
git clone https://github.com/lcorneliussen/md365.git
cd md365
go build -o ~/.local/bin/md365 .
```

### Pre-built binaries

Check [Releases](https://github.com/lcorneliussen/md365/releases) for pre-built binaries.

## Sync Strategy

- **Events:** Full window sync (past 30 days → future 90 days). Deleted events are removed locally.
- **Contacts:** Delta sync using Graph API delta links for incremental updates.
- **Direction:** One-way (remote → local). Local files are a read-only cache. Write operations go through CLI commands → API.

## Dependencies

Single Go binary. No runtime dependencies.

Build requires Go 1.21+.

## License

MIT
